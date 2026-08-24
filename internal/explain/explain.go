// Package explain generates the CRUX explainer described in
// docs/standards/crux_method.md.
//
// A single self-contained HTML file — Background, Intuition, Code, Quiz —
// written outside version control and advisory throughout. It is never a review
// layer and never a gate: docs/adr/0003-crux-explainers-are-transient.md
// settled that it is an aid, and having an implementation does not change what
// it is.
//
// The page is rendered here rather than asked of a model. Everything in an
// explainer derives from a diff under review, which is exactly the text that
// must not be trusted with markup; building the HTML in code means the escaping
// is a property of the renderer instead of a request the model may ignore.
package explain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LukeSantossz/my-framework/internal/jsonx"
)

// Sections are the four the method names, in the order it names them.
var Sections = []string{"Background", "Intuition", "Code", "Quiz"}

// Difficulty is the quiz's level. The method defines three and defaults to
// medium.
type Difficulty string

const (
	Easy   Difficulty = "easy"
	Medium Difficulty = "medium"
	Hard   Difficulty = "hard"
)

// QuizQuestions is how many the method asks for.
const QuizQuestions = 5

// ParseDifficulty resolves the requested level. An unknown one is refused
// rather than rounded to the default: silently downgrading "brutal" to medium
// answers a question the Developer did not ask.
func ParseDifficulty(s string) (Difficulty, error) {
	switch Difficulty(strings.ToLower(strings.TrimSpace(s))) {
	case "":
		return Medium, nil
	case Easy:
		return Easy, nil
	case Medium:
		return Medium, nil
	case Hard:
		return Hard, nil
	}
	return "", fmt.Errorf("unknown quiz difficulty %q (expected easy, medium or hard)", s)
}

// Question is one multiple-choice item and the deeper explanation a wrong
// answer reveals.
type Question struct {
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Answer      int      `json:"answer"`
	Remediation string   `json:"remediation"`
}

// Background is the method's two-part opening: a deep version a beginner can
// read and anyone else can skip, then the narrow part this change needs.
type Background struct {
	Deep   string `json:"deep"`
	Narrow string `json:"narrow"`
}

// Content is the explainer's substance, as the model returns it.
type Content struct {
	Title      string     `json:"title"`
	Background Background `json:"background"`
	Intuition  string     `json:"intuition"`
	Code       string     `json:"code"`
	Quiz       []Question `json:"quiz"`
}

// Meta is what the explainer says about itself: which change, explained by
// what, on which day. Without it a file found months later explains a diff
// nobody can identify.
type Meta struct {
	Head, Base string
	Backend    string
	Provider   string
	Model      string
	Date       string
	Difficulty Difficulty

	// Humanized records whether the anti-slop pass ran. crux_method.md degrades
	// deliberately, never silently: an explainer produced without it is still
	// produced, and the missing step is flagged.
	Humanized bool
}

// Prompt is what the explaining backend is asked for. It requests an envelope
// rather than HTML, because HTML from a model is markup this package would have
// to trust.
func Prompt(d Difficulty) string {
	return fmt.Sprintf(`You are producing a CRUX explainer for a change under review. Explain what the
change does and why, for a reader who has not seen this codebase.

Answer with a single JSON object and nothing else:

{"title": "...",
 "background": {"deep": "...", "narrow": "..."},
 "intuition": "...",
 "code": "...",
 "quiz": [{"question": "...", "options": ["...", "..."], "answer": 0, "remediation": "..."}]}

The four sections are Background, Intuition, Code and Quiz, in that order:

- background.deep: the field's own background, for a beginner. Skippable by a
  reader who already has it.
- background.narrow: only the background this specific change needs.
- intuition: the essence of the change, with concrete toy-data examples.
- code: a high-level, grouped walkthrough of the changes. Group by what the code
  does, not file by file.
- quiz: exactly %d multiple-choice questions at %s difficulty, testing real
  understanding rather than gotchas. "answer" is the zero-based index of the
  correct option. "remediation" explains the concept more deeply, and is shown
  when the reader answers wrongly.

Write plain prose. Use Markdown fenced code blocks for code and nothing else:
no HTML, no tables, no images. Explain only what the diff shows — if something
is unclear from the diff, say so rather than inventing behaviour, because an
explainer that confidently explains something the code does not do is worse than
no explainer.`, QuizQuestions, d)
}

// Parse recovers the envelope from the answer and refuses anything that is not
// one. Prose rendered into the four headings would present one section's text
// as all four.
func Parse(answer string) (Content, error) {
	raw, ok := jsonx.Object(answer, "intuition")
	if !ok {
		return Content{}, fmt.Errorf("the answer carries no explainer object; the backend replied in prose")
	}
	var c Content
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return Content{}, fmt.Errorf("the explainer object is not valid JSON: %w", err)
	}
	for i, q := range c.Quiz {
		if len(q.Options) < 2 {
			return Content{}, fmt.Errorf("quiz question %d offers fewer than two options", i+1)
		}
		if q.Answer < 0 || q.Answer >= len(q.Options) {
			// An out-of-range index makes every answer wrong, which reads to
			// the Developer as a broken understanding rather than a broken
			// explainer.
			return Content{}, fmt.Errorf("quiz question %d marks option %d correct, but there are %d options",
				i+1, q.Answer, len(q.Options))
		}
	}
	return c, nil
}

// --- writing ----------------------------------------------------------------

// Write renders the explainer into dir and returns the path.
//
// It refuses a destination inside the repository. The artifact is transient by
// decision: one committed by accident becomes the durable per-change record the
// method deliberately does not create, and it would then age against the code
// with nothing updating it.
func Write(dir, repoRoot string, c Content, m Meta) (string, error) {
	if err := CheckDestination(dir, repoRoot); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create %s: %w", dir, err)
	}
	path := filepath.Join(dir, FileName(m.Date, m.Head))
	file, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("cannot write %s: %w", path, err)
	}
	defer file.Close()
	if err := Render(file, c, m); err != nil {
		return "", err
	}
	return path, nil
}

// CheckDestination answers whether an explainer may be written to dir. It is
// separate from Write so a caller can ask before it spends anything: finding
// out that the destination is refused after a model has been paid for the
// answer wastes the run and the quota.
func CheckDestination(dir, repoRoot string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("no destination directory for the explainer")
	}
	if repoRoot == "" {
		return nil
	}
	inside, err := isInside(dir, repoRoot)
	if err != nil {
		return err
	}
	if inside {
		return fmt.Errorf("%s is inside the repository at %s; CRUX explainers are written outside version control (docs/adr/0003-crux-explainers-are-transient.md)",
			dir, repoRoot)
	}
	return nil
}

// FileName is date-prefixed so the files stay time-sorted in a directory that
// has nothing else to order them by.
func FileName(date, head string) string {
	if date == "" {
		date = "0000-00-00"
	}
	return date + "-" + slug(head) + ".html"
}

// slug turns a branch name into a filename component. A branch name is user
// input on its way to a path, so everything outside the allowed set is replaced
// rather than escaped: `feat/../../etc` must become a name, not a traversal.
func slug(s string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "change"
	}
	if len(out) > 60 {
		out = strings.Trim(out[:60], "-")
	}
	return out
}

func isInside(dir, repoRoot string) (bool, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false, fmt.Errorf("cannot resolve %s: %w", dir, err)
	}
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return false, fmt.Errorf("cannot resolve %s: %w", repoRoot, err)
	}
	rel, err := filepath.Rel(absRoot, absDir)
	if err != nil {
		// Different volumes: not inside, and not an error either.
		return false, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, nil
	}
	return true, nil
}
