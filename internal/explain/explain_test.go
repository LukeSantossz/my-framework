package explain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sample() Content {
	return Content{
		Title: "the role runner",
		Background: Background{
			Deep:   "A review backend is a program that reads a diff.",
			Narrow: "This change adds a chain of them.",
		},
		Intuition: "Walk the chain until one answers.\n\nThe first answer wins.",
		Code:      "The runner lives in internal/role.\n\n```go\nfor _, b := range chain {}\n```",
		Quiz: []Question{
			{Question: "What stops the chain?", Options: []string{"a finding", "the first backend that reviews"}, Answer: 1, Remediation: "Unavailable advances it; a review stops it."},
		},
	}
}

func TestProducesTheFourSectionsTheMethodNames(t *testing.T) {
	// crux_method.md names them in order: Background, Intuition, Code, Quiz.
	var b strings.Builder
	if err := Render(&b, sample(), Meta{Head: "feat/x", Base: "main", Date: "2026-08-24"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	html := b.String()

	last := -1
	for _, section := range Sections {
		anchor := `id="` + strings.ToLower(section) + `"`
		at := strings.Index(html, anchor)
		if at < 0 {
			t.Fatalf("the %s section is missing", section)
		}
		if at < last {
			t.Errorf("the %s section is out of order", section)
		}
		last = at
		if !strings.Contains(html, `href="#`+strings.ToLower(section)+`"`) {
			t.Errorf("the table of contents does not link the %s section", section)
		}
	}
	// Background carries a deep, skippable version for beginners and then the
	// narrow background relevant to the change; one is not the other.
	for _, want := range []string{"A review backend is a program", "This change adds a chain"} {
		if !strings.Contains(html, want) {
			t.Errorf("the background lost %q", want)
		}
	}
}

func TestTheArtifactIsSelfContained(t *testing.T) {
	// The method's contract: a single self-contained HTML file with inline CSS
	// and JavaScript. A remote reference would make an explainer written today
	// unreadable when the host it points at changes.
	var b strings.Builder
	if err := Render(&b, sample(), Meta{}); err != nil {
		t.Fatal(err)
	}
	html := b.String()
	for _, forbidden := range []string{"<link ", "src=\"http", "@import", "https://"} {
		if strings.Contains(html, forbidden) {
			t.Errorf("the explainer reaches outside itself: %q", forbidden)
		}
	}
	for _, want := range []string{"<style>", "<script>"} {
		if !strings.Contains(html, want) {
			t.Errorf("the explainer is missing its inline %s", want)
		}
	}
}

func TestChangeDerivedTextCannotInjectMarkupOrScripts(t *testing.T) {
	// Everything in an explainer comes from a diff under review, which is
	// exactly the text that must not be trusted. Escaping is the reason this
	// package renders the page rather than asking a model for HTML.
	hostile := Content{
		Title:     `</title><script>alert("title")</script>`,
		Intuition: `<img src=x onerror="alert(1)">`,
		Code:      "</pre><script>alert('code')</script>",
		Quiz: []Question{{
			Question: `"></div><script>alert('quiz')</script>`,
			Options:  []string{"a", "b"},
			Answer:   0,
		}},
	}
	var b strings.Builder
	if err := Render(&b, hostile, Meta{Head: `<script>alert('head')</script>`}); err != nil {
		t.Fatal(err)
	}
	html := b.String()
	for _, injection := range []string{"<script>alert", "onerror=\"alert", "</pre><script"} {
		if strings.Contains(html, injection) {
			t.Errorf("change-derived text reached the page unescaped: %q found in\n%s", injection, html)
		}
	}
}

func TestTheQuizCarriesRemediationForAWrongAnswer(t *testing.T) {
	// crux_method.md: a wrong answer reveals a deeper explanation before
	// advancing, with a control that lets the Developer skip it.
	var b strings.Builder
	if err := Render(&b, sample(), Meta{}); err != nil {
		t.Fatal(err)
	}
	html := b.String()
	if !strings.Contains(html, "Unavailable advances it") {
		t.Error("the remediation text did not reach the page")
	}
	if !strings.Contains(strings.ToLower(html), "skip") {
		t.Error("there is no control to skip the remediation and proceed")
	}
}

// --- where it is written ----------------------------------------------------

func TestWritesTheExplainerOutsideVersionControl(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	path, err := Write(outside, repo, sample(), Meta{Head: "feat/role-runner", Date: "2026-08-24"})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.HasPrefix(filepath.Base(path), "2026-08-24-") {
		// The date prefix is what keeps the files time-sorted outside version
		// control, where nothing else orders them.
		t.Errorf("the filename is not date-prefixed: %s", filepath.Base(path))
	}
	if !strings.HasSuffix(path, ".html") {
		t.Errorf("the artifact is not an HTML file: %s", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("nothing was written: %v", err)
	}
	if rel, relErr := filepath.Rel(repo, path); relErr == nil && !strings.HasPrefix(rel, "..") {
		t.Errorf("the explainer landed inside the repository at %s", path)
	}
}

func TestRefusesToWriteTheExplainerIntoTheRepository(t *testing.T) {
	// ADR 0003 settled that CRUX explainers are transient. One committed by
	// accident becomes a durable per-change record the method deliberately does
	// not create, and it would age against the code with nothing updating it.
	repo := t.TempDir()
	for _, inside := range []string{repo, filepath.Join(repo, "docs", "crux")} {
		if _, err := Write(inside, repo, sample(), Meta{Date: "2026-08-24"}); err == nil {
			t.Errorf("%s: an explainer was written inside the repository", inside)
		}
	}
	// The refusal must not have left a partial file behind.
	if entries, _ := os.ReadDir(repo); len(entries) != 0 {
		t.Errorf("the refused write left %d entry/entries in the repository", len(entries))
	}
}

func TestASlugIsDerivedFromTheBranchWithoutLettingItEscapeTheDirectory(t *testing.T) {
	// A branch name is user input and reaches a filename. `feat/../../etc` must
	// become a name, not a path.
	if got := slug("feat/../../etc/passwd"); strings.ContainsAny(got, `/\.`) {
		t.Errorf("slug = %q; it still carries path syntax", got)
	}
	if got := slug(""); got == "" {
		t.Error("an empty branch produced an empty filename")
	}
}

// --- the prompt -------------------------------------------------------------

func TestThePromptAsksForTheFourSectionsAndTheChosenDifficulty(t *testing.T) {
	prompt := Prompt(Hard)
	want := append(append([]string{}, Sections...), "hard",
		fmt.Sprintf("%d multiple-choice questions", QuizQuestions))
	for _, w := range want {
		if !strings.Contains(strings.ToLower(prompt), strings.ToLower(w)) {
			t.Errorf("the prompt omits %q", w)
		}
	}
}

func TestDifficultyDefaultsToMediumAndRefusesAnUnknownValue(t *testing.T) {
	if d, err := ParseDifficulty(""); err != nil || d != Medium {
		t.Errorf("ParseDifficulty(\"\") = %q, %v; the method's default is medium", d, err)
	}
	if _, err := ParseDifficulty("brutal"); err == nil {
		t.Error("an unknown difficulty was accepted")
	}
}

func TestParseReadsTheEnvelopeOutOfAChattyAnswer(t *testing.T) {
	answer := "Sure, here it is:\n```json\n" + `{"title":"t","background":{"deep":"d","narrow":"n"},` +
		`"intuition":"i","code":"c","quiz":[{"question":"q","options":["a","b"],"answer":1,"remediation":"r"}]}` +
		"\n```\nHope that helps."
	c, err := Parse(answer)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Intuition != "i" || len(c.Quiz) != 1 || c.Quiz[0].Answer != 1 {
		t.Errorf("content = %+v", c)
	}
}

func TestParseRefusesAnAnswerWithNoEnvelope(t *testing.T) {
	// A model that answered in prose has not produced the artifact. Rendering
	// the prose into the four headings would present one section's text as all
	// four.
	if _, err := Parse("I would explain this change as follows: it is fine."); err == nil {
		t.Fatal("prose with no envelope was accepted as an explainer")
	}
}

func TestParseRefusesAQuizAnswerOutsideItsOptions(t *testing.T) {
	// An out-of-range index makes every answer wrong, which reads to the
	// Developer as a broken understanding rather than a broken explainer.
	answer := `{"title":"t","intuition":"i","code":"c","quiz":[{"question":"q","options":["a","b"],"answer":7}]}`
	if _, err := Parse(answer); err == nil {
		t.Fatal("a quiz answer outside its options was accepted")
	}
}
