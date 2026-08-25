// Package report carries what a review found and how it is shown.
//
// The categories are not invented here: they are the five AGENTS.md already
// enumerates for the Reviewer role, so the vocabulary has one home.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/LukeSantossz/my-framework/internal/jsonx"
	"github.com/LukeSantossz/my-framework/internal/usage"
)

type Category string

const (
	CategoryCorrectness    Category = "correctness"
	CategoryInventedSymbol Category = "invented-symbol"
	CategoryScopeCreep     Category = "scope-creep"
	CategorySecurity       Category = "security"
	CategoryConvention     Category = "convention"

	// CategoryUnstructured marks prose from a backend that cannot express
	// per-finding structure. It is deliberately not one of the five: presenting
	// it as a category would let an unparsed blob pass for a classified finding.
	CategoryUnstructured Category = "unstructured"
)

// Categories returns the five categories a structured review may use.
func Categories() []Category {
	return []Category{
		CategoryCorrectness,
		CategoryInventedSymbol,
		CategoryScopeCreep,
		CategorySecurity,
		CategoryConvention,
	}
}

type Severity string

const (
	SeverityAdvisory Severity = "advisory"
	SeverityBlocking Severity = "blocking"
)

type Finding struct {
	Category  Category `json:"category"`
	Severity  Severity `json:"severity"`
	File      string   `json:"file"`
	Line      int      `json:"line"`
	Summary   string   `json:"summary"`
	Rationale string   `json:"rationale"`
}

// Result is one backend's review of one change.
type Result struct {
	Backend  string
	Provider string
	Model    string
	Findings []Finding

	// Truncated says the diff sent was cut short; Incomplete says the answer
	// was. Both make the review partial, and a partial review that reads as a
	// whole one is the failure this pair exists to prevent.
	Truncated  bool
	Incomplete bool

	// Unstructured says the findings are prose the backend could not classify.
	Unstructured bool

	// Usage is what this review consumed, in disjoint buckets.
	Usage usage.Usage
}

func (r Result) HasBlocking() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityBlocking {
			return true
		}
	}
	return false
}

type findingsEnvelope struct {
	Findings []Finding `json:"findings"`
}

// ParseFindings reads the structured shape out of a model's answer. Models wrap
// the object in prose or a fenced block, so the object is located rather than
// required to be the whole message.
func ParseFindings(body string) ([]Finding, error) {
	raw, ok := jsonx.Object(body, "findings")
	if !ok {
		return nil, fmt.Errorf("no findings object in the answer")
	}
	var env findingsEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil, fmt.Errorf("findings object is not valid JSON: %w", err)
	}
	valid := map[Category]bool{}
	for _, c := range Categories() {
		valid[c] = true
	}
	for i := range env.Findings {
		f := &env.Findings[i]
		if !valid[f.Category] {
			// Coercing an unknown category into a known one would file a
			// finding under a heading its author did not choose.
			return nil, fmt.Errorf("unknown finding category %q", f.Category)
		}
		if f.Severity == "" {
			// ai_guidelines.md makes a finding advisory unless stated
			// otherwise, so an omission must not escalate.
			f.Severity = SeverityAdvisory
		}
	}
	if env.Findings == nil {
		env.Findings = []Finding{}
	}
	return env.Findings, nil
}

// Text returns a backend's answer as prose. An unstructured result carries the
// whole answer in a single finding, which is the right shape for a review and
// the wrong one for a caller that asked for something other than findings.
func Text(r Result) string {
	parts := make([]string, 0, len(r.Findings))
	for _, f := range r.Findings {
		parts = append(parts, f.Summary)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// Unstructured records a backend's prose as one finding. Reporting nothing
// would be indistinguishable from a clean review, which is the false negative
// this framework treats as worst.
func Unstructured(backend, provider, model, output string) Result {
	summary := strings.TrimSpace(output)
	if summary == "" {
		summary = "the backend produced no output; this is not a clean review"
	}
	return Result{
		Backend:      backend,
		Provider:     provider,
		Model:        model,
		Unstructured: true,
		Findings: []Finding{{
			Category: CategoryUnstructured,
			Severity: SeverityAdvisory,
			Summary:  summary,
		}},
	}
}

// Render writes the human-facing report. It always names the backend, provider
// and model: falling back to a weaker reviewer is allowed, falling back quietly
// is not.
func Render(w io.Writer, r Result) {
	fmt.Fprintf(w, "Reviewed by: %s / %s / %s\n", nonEmpty(r.Backend), nonEmpty(r.Provider), nonEmpty(r.Model))
	if r.Truncated {
		fmt.Fprintln(w, "  note: the diff was truncated; this review is partial.")
	}
	if r.Incomplete {
		fmt.Fprintln(w, "  note: the answer was cut off by the output limit; this review is incomplete.")
	}
	if r.Unstructured {
		fmt.Fprintln(w, "  note: this backend cannot report per-finding structure; its output is recorded verbatim.")
	}
	// Printed before the findings and before the early return, so a clean review
	// still accounts for what it cost.
	fmt.Fprintf(w, "  usage: %s\n", r.Usage)
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "  no findings reported.")
		return
	}
	for _, f := range r.Findings {
		location := ""
		if f.File != "" {
			location = " " + f.File
			if f.Line > 0 {
				location = fmt.Sprintf("%s:%d", location, f.Line)
			}
		}
		fmt.Fprintf(w, "  [%s/%s]%s %s\n", f.Category, f.Severity, location, f.Summary)
		if f.Rationale != "" {
			fmt.Fprintf(w, "      %s\n", f.Rationale)
		}
	}
}

func nonEmpty(s string) string {
	if s == "" {
		return "<unset>"
	}
	return s
}
