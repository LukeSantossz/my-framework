package explain

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
)

// block is one rendered piece of prose. Splitting the model's text into typed
// blocks is what lets the template escape every one of them in the right
// context: code inside <pre>, prose inside <p>, and nothing inside raw HTML.
type block struct {
	Kind string // "prose", "code" or "callout"
	Text string
}

type section struct {
	ID     string
	Title  string
	Deep   []block
	Blocks []block
	Quiz   bool
}

type page struct {
	Title      string
	Meta       Meta
	Sections   []section
	QuizJSON   string
	HasQuiz    bool
	Attributed string
}

// Render writes the self-contained explainer. Inline CSS and JavaScript, no
// external reference of any kind: a page that reaches outside itself is
// unreadable the moment whatever it points at moves, and these files outlive
// the branch they explain.
func Render(w io.Writer, c Content, m Meta) error {
	title := strings.TrimSpace(c.Title)
	if title == "" {
		title = "CRUX explainer"
	}

	sections := []section{
		{
			ID:     "background",
			Title:  "Background",
			Deep:   blocksOf(c.Background.Deep),
			Blocks: blocksOf(c.Background.Narrow),
		},
		{ID: "intuition", Title: "Intuition", Blocks: blocksOf(c.Intuition)},
		{ID: "code", Title: "Code", Blocks: blocksOf(c.Code)},
		{ID: "quiz", Title: "Quiz", Quiz: true},
	}

	quiz, err := json.Marshal(c.Quiz)
	if err != nil {
		return fmt.Errorf("cannot render the quiz: %w", err)
	}

	return pageTemplate.Execute(w, page{
		Title:      title,
		Meta:       m,
		Sections:   sections,
		QuizJSON:   string(quiz),
		HasQuiz:    len(c.Quiz) > 0,
		Attributed: attribution(m),
	})
}

// attribution names what produced the explainer. A file found months later
// explains a diff nobody can identify unless it says which one, and an
// explainer that does not name its model cannot be weighed against a better one.
func attribution(m Meta) string {
	parts := []string{}
	if m.Head != "" {
		change := m.Head
		if m.Base != "" {
			change += " against " + m.Base
		}
		parts = append(parts, change)
	}
	if m.Backend != "" {
		explained := "explained by " + m.Backend
		if m.Provider != "" {
			explained += " (" + m.Provider + ")"
		}
		if m.Model != "" {
			explained += " using " + m.Model
		}
		parts = append(parts, explained)
	}
	if m.Difficulty != "" {
		parts = append(parts, "quiz at "+string(m.Difficulty)+" difficulty")
	}
	if m.Date != "" {
		parts = append(parts, m.Date)
	}
	return strings.Join(parts, " · ")
}

// blocksOf splits prose into typed blocks. It understands fenced code and
// quoted callouts and nothing else: the prompt asks for plain prose, and a
// fuller Markdown renderer here would be a second place for change-derived
// text to become markup.
func blocksOf(text string) []block {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if strings.TrimSpace(text) == "" {
		return nil
	}

	var out []block
	var buf []string
	kind := "prose"

	flush := func() {
		if len(buf) == 0 {
			return
		}
		body := strings.Join(buf, "\n")
		if kind != "code" {
			body = strings.TrimSpace(body)
		}
		if body != "" {
			out = append(out, block{Kind: kind, Text: body})
		}
		buf = nil
	}

	inCode := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			flush()
			inCode = !inCode
			if inCode {
				kind = "code"
			} else {
				kind = "prose"
			}
			continue
		}
		if inCode {
			buf = append(buf, line)
			continue
		}
		if trimmed == "" {
			flush()
			kind = "prose"
			continue
		}
		if strings.HasPrefix(trimmed, "> ") {
			if kind != "callout" {
				flush()
				kind = "callout"
			}
			buf = append(buf, strings.TrimPrefix(trimmed, "> "))
			continue
		}
		if kind == "callout" {
			flush()
			kind = "prose"
		}
		buf = append(buf, line)
	}
	flush()
	return out
}

// The page. Every interpolation goes through html/template, so change-derived
// text is escaped for the context it lands in — element body, attribute or
// title — rather than trusted to be harmless.
var pageTemplate = template.Must(template.New("crux").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
/* Every colour below is a role declared in docs/standards/design.md, and
   mf check design fails this file if one appears that is not. There is no
   accent: emphasis is weight, rule and underline. Both polarities are defined
   together, neither as the other's afterthought. */
:root { color-scheme: light dark;
        --canvas: #faf8f4; --canvas-soft: #f0ece4; --hairline: #ddd6ca;
        --ink: #1c1a17; --body: #423d36; --mute: #6f675c;
        --correct: #3f6b4a; --incorrect: #9b3b32; }
@media (prefers-color-scheme: dark) {
  :root { --canvas: #1a1815; --canvas-soft: #262320; --hairline: #35302a;
          --ink: #f2eee6; --body: #cfc7b9; --mute: #948b7d;
          --correct: #7fae89; --incorrect: #d98d84; }
}
* { box-sizing: border-box; }
body { margin: 0 auto; padding: 32px 16px 96px; max-width: 46rem; background: var(--canvas);
       color: var(--body); font: 16px/1.65 ui-serif, Georgia, "Times New Roman", serif; }
h1, h2, h3 { font-family: ui-sans-serif, system-ui, "Segoe UI", Arial, sans-serif;
             line-height: 1.25; color: var(--ink); }
h1 { font-size: 1.7rem; margin: 0 0 4px; }
h2 { font-size: 1.25rem; margin: 48px 0 12px; padding-top: 12px; border-top: 1px solid var(--hairline); }
.meta, .degraded { color: var(--mute); font-size: .85rem; margin: 2px 0;
                   font-family: ui-sans-serif, system-ui, sans-serif; }
.degraded { font-style: italic; }
nav ol { padding-left: 24px; margin: 24px 0 0; font-family: ui-sans-serif, system-ui, sans-serif; }
nav a { color: var(--ink); text-decoration: underline; text-underline-offset: 3px; }
pre { background: var(--canvas-soft); border: 1px solid var(--hairline); border-radius: 6px;
      padding: 12px 16px; overflow-x: auto; white-space: pre; color: var(--ink); }
pre, code { font-family: ui-monospace, "Cascadia Mono", Consolas, monospace; font-size: .85rem; }
.callout { background: var(--canvas-soft); border-left: 2px solid var(--hairline);
           border-radius: 0 6px 6px 0; padding: 12px 16px; margin: 24px 0; white-space: pre-wrap; }
details { margin: 16px 0; }
summary { cursor: pointer; font-family: ui-sans-serif, system-ui, sans-serif; color: var(--ink); }
.q { border: 1px solid var(--hairline); border-radius: 6px; padding: 16px; margin: 24px 0; }
.q h3 { margin: 0 0 8px; font-size: 1rem; }
.opt { display: block; width: 100%; text-align: left; margin: 4px 0; padding: 8px 12px;
       border: 1px solid var(--hairline); border-radius: 4px; background: transparent; color: inherit;
       font: inherit; cursor: pointer; }
.opt:hover { border-color: var(--ink); }
/* Colour is never the only signal here: the mark says the same thing, so the
   state survives a reader who cannot separate the two hues. */
.opt.right { border-color: var(--correct); color: var(--ink); }
.opt.wrong { border-color: var(--incorrect); color: var(--ink); }
.mark { font-family: ui-monospace, "Cascadia Mono", Consolas, monospace; }
.fix { margin-top: 12px; padding: 12px 16px; background: var(--canvas-soft); border-radius: 6px;
       white-space: pre-wrap; }
.skip { margin-top: 8px; font: inherit; font-size: .85rem; background: transparent; color: var(--ink);
        border: 1px solid var(--hairline); border-radius: 3px; padding: 4px 12px; cursor: pointer; }
.score { font-family: ui-sans-serif, system-ui, sans-serif; color: var(--mute); }
</style>
</head>
<body>
<header>
<h1>{{.Title}}</h1>
{{if .Attributed}}<p class="meta">{{.Attributed}}</p>{{end}}
<p class="meta">A CRUX explainer. Advisory only: it is not a review layer and it blocks nothing.</p>
{{if not .Meta.Humanized}}<p class="degraded">The anti-slop pass did not run here, so this prose had no humanizer step. Flagged rather than left silent.</p>{{end}}
</header>

<nav>
<ol>
{{range .Sections}}<li><a href="#{{.ID}}">{{.Title}}</a></li>
{{end}}</ol>
</nav>

{{range .Sections}}
<section id="{{.ID}}">
<h2>{{.Title}}</h2>
{{if .Deep}}<details><summary>Deep background — skip this if you already have it</summary>
{{template "blocks" .Deep}}</details>
{{end}}{{template "blocks" .Blocks}}
{{if .Quiz}}<div id="quiz-root" data-quiz="{{$.QuizJSON}}"></div>
{{if not $.HasQuiz}}<p class="meta">No quiz was produced for this change.</p>{{end}}
{{end}}</section>
{{end}}

<script>
(function () {
  var root = document.getElementById("quiz-root");
  if (!root) { return; }
  var questions = [];
  try { questions = JSON.parse(root.dataset.quiz) || []; } catch (e) { questions = []; }
  var answered = 0, correct = 0;

  questions.forEach(function (q, index) {
    var card = document.createElement("div");
    card.className = "q";

    var heading = document.createElement("h3");
    heading.textContent = (index + 1) + ". " + (q.question || "");
    card.appendChild(heading);

    var fix = document.createElement("div");
    fix.className = "fix";
    fix.hidden = true;

    (q.options || []).forEach(function (option, choice) {
      var button = document.createElement("button");
      button.className = "opt";
      button.type = "button";
      button.textContent = option;
      button.addEventListener("click", function () {
        if (card.dataset.done) { return; }
        card.dataset.done = "1";
        answered = answered + 1;
        var right = choice === q.answer;
        if (right) { correct = correct + 1; }
        Array.prototype.forEach.call(card.querySelectorAll(".opt"), function (el, i) {
          el.className = "opt" + (i === q.answer ? " right" : (i === choice ? " wrong" : ""));
          // The mark carries the same meaning as the border colour, so the
          // answer is legible without separating the two hues.
          if (i === q.answer || i === choice) {
            var mark = document.createElement("span");
            mark.className = "mark";
            mark.textContent = (i === q.answer ? "[ok] " : "[x] ");
            el.insertBefore(mark, el.firstChild);
          }
        });
        if (!right && q.remediation) {
          fix.textContent = q.remediation;
          fix.hidden = false;
          var skip = document.createElement("button");
          skip.className = "skip";
          skip.type = "button";
          skip.textContent = "Skip this and continue";
          skip.addEventListener("click", function () { fix.hidden = true; });
          fix.appendChild(document.createElement("br"));
          fix.appendChild(skip);
        }
        score.textContent = correct + " of " + answered + " answered correctly.";
      });
      card.appendChild(button);
    });

    card.appendChild(fix);
    root.appendChild(card);
  });

  var score = document.createElement("p");
  score.className = "score";
  score.textContent = questions.length ? "Nothing answered yet." : "";
  root.appendChild(score);
})();
</script>
</body>
</html>
`))

var _ = template.Must(pageTemplate.New("blocks").Parse(
	`{{range .}}{{if eq .Kind "code"}}<pre><code>{{.Text}}</code></pre>
{{else if eq .Kind "callout"}}<aside class="callout">{{.Text}}</aside>
{{else}}<p>{{.Text}}</p>
{{end}}{{end}}`))
