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
:root { color-scheme: light dark; --ink: #17181c; --paper: #fbfbf9; --muted: #63656d;
        --rule: #dcdcd6; --accent: #2f5d9e; --callout: #f0f2f6; }
@media (prefers-color-scheme: dark) {
  :root { --ink: #e7e7e4; --paper: #16171a; --muted: #9a9ca4; --rule: #2e3036;
          --accent: #8fb4e8; --callout: #202329; }
}
* { box-sizing: border-box; }
body { margin: 0 auto; padding: 2rem 1.25rem 6rem; max-width: 46rem; background: var(--paper);
       color: var(--ink); font: 16px/1.65 ui-serif, Georgia, "Times New Roman", serif; }
h1, h2, h3 { font-family: ui-sans-serif, system-ui, "Segoe UI", Arial, sans-serif; line-height: 1.25; }
h1 { font-size: 1.7rem; margin: 0 0 .4rem; }
h2 { font-size: 1.25rem; margin: 3rem 0 .75rem; padding-top: .75rem; border-top: 1px solid var(--rule); }
.meta, .degraded { color: var(--muted); font-size: .85rem; margin: .2rem 0;
                   font-family: ui-sans-serif, system-ui, sans-serif; }
.degraded { color: var(--accent); }
nav ol { padding-left: 1.2rem; margin: 1.5rem 0 0; font-family: ui-sans-serif, system-ui, sans-serif; }
nav a { color: var(--accent); }
pre { background: var(--callout); border: 1px solid var(--rule); border-radius: 6px;
      padding: .8rem 1rem; overflow-x: auto; white-space: pre; }
pre, code { font-family: ui-monospace, "Cascadia Mono", Consolas, monospace; font-size: .85rem; }
.callout { background: var(--callout); border-left: 3px solid var(--accent); border-radius: 0 6px 6px 0;
           padding: .7rem 1rem; margin: 1.2rem 0; white-space: pre-wrap; }
details { margin: 1rem 0; }
summary { cursor: pointer; font-family: ui-sans-serif, system-ui, sans-serif; color: var(--accent); }
.q { border: 1px solid var(--rule); border-radius: 8px; padding: 1rem 1.1rem; margin: 1.2rem 0; }
.q h3 { margin: 0 0 .6rem; font-size: 1rem; }
.opt { display: block; width: 100%; text-align: left; margin: .3rem 0; padding: .5rem .7rem;
       border: 1px solid var(--rule); border-radius: 6px; background: transparent; color: inherit;
       font: inherit; cursor: pointer; }
.opt:hover { border-color: var(--accent); }
.opt.right { border-color: #2e7d4f; }
.opt.wrong { border-color: #a33; }
.fix { margin-top: .8rem; padding: .8rem 1rem; background: var(--callout); border-radius: 6px;
       white-space: pre-wrap; }
.skip { margin-top: .6rem; font: inherit; font-size: .85rem; background: transparent; color: var(--accent);
        border: 1px solid var(--rule); border-radius: 6px; padding: .35rem .7rem; cursor: pointer; }
.score { font-family: ui-sans-serif, system-ui, sans-serif; color: var(--muted); }
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
