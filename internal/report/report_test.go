package report

import (
	"bytes"
	"strings"
	"testing"
)

func TestParsesTheFiveCategoriesAgentsMdEnumerates(t *testing.T) {
	body := `{"findings":[
	  {"category":"correctness","severity":"blocking","file":"a.go","line":12,"summary":"off by one"},
	  {"category":"invented-symbol","severity":"advisory","summary":"no such flag"},
	  {"category":"scope-creep","severity":"advisory","summary":"unrelated rename"},
	  {"category":"security","severity":"blocking","summary":"hardcoded secret"},
	  {"category":"convention","severity":"advisory","summary":"naming"}
	]}`
	findings, err := ParseFindings(body)
	if err != nil {
		t.Fatalf("ParseFindings: %v", err)
	}
	if len(findings) != 5 {
		t.Fatalf("parsed %d findings, want 5", len(findings))
	}
	want := []Category{CategoryCorrectness, CategoryInventedSymbol, CategoryScopeCreep, CategorySecurity, CategoryConvention}
	for i, c := range want {
		if findings[i].Category != c {
			t.Errorf("finding %d category = %q, want %q", i, findings[i].Category, c)
		}
	}
	if findings[0].Line != 12 || findings[0].File != "a.go" {
		t.Errorf("location lost: %+v", findings[0])
	}
}

func TestParsesFindingsWrappedInProse(t *testing.T) {
	// Models put the object inside an explanation or a fenced block. Refusing
	// that would send a real review down the unstructured path for a reason
	// that has nothing to do with the review.
	body := "Here is what I found:\n```json\n{\"findings\":[{\"category\":\"security\",\"summary\":\"key in log\"}]}\n```\nHope that helps."
	findings, err := ParseFindings(body)
	if err != nil {
		t.Fatalf("ParseFindings: %v", err)
	}
	if len(findings) != 1 || findings[0].Category != CategorySecurity {
		t.Fatalf("got %+v", findings)
	}
}

func TestRejectsAnUnknownCategoryRatherThanInventingOne(t *testing.T) {
	_, err := ParseFindings(`{"findings":[{"category":"vibes","summary":"hmm"}]}`)
	if err == nil {
		t.Fatal("want an error for an unknown category")
	}
	if !strings.Contains(err.Error(), "vibes") {
		t.Errorf("error %q does not name the offending category", err)
	}
}

func TestDefaultsMissingSeverityToAdvisory(t *testing.T) {
	// ai_guidelines.md makes every finding advisory unless stated otherwise, so
	// an omitted severity must not silently become blocking.
	findings, err := ParseFindings(`{"findings":[{"category":"convention","summary":"naming"}]}`)
	if err != nil {
		t.Fatalf("ParseFindings: %v", err)
	}
	if findings[0].Severity != SeverityAdvisory {
		t.Errorf("severity = %q, want %q", findings[0].Severity, SeverityAdvisory)
	}
}

func TestParsesAnEmptyFindingsListAsACleanReview(t *testing.T) {
	findings, err := ParseFindings(`{"findings":[]}`)
	if err != nil {
		t.Fatalf("ParseFindings: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("got %d findings, want none", len(findings))
	}
}

func TestFailsOnTextThatCarriesNoFindingsObject(t *testing.T) {
	if _, err := ParseFindings("I could not review this."); err == nil {
		t.Fatal("want an error when there is no findings object to parse")
	}
}

func TestUnstructuredRecordsProseAsOneFindingRatherThanNone(t *testing.T) {
	// Silence would read as a clean review. A backend that cannot express
	// per-finding structure must still be distinguishable from one that found
	// nothing.
	r := Unstructured("codex", "openai", "gpt-5.6-terra", "The base case is wrong.")
	if len(r.Findings) != 1 {
		t.Fatalf("got %d findings, want exactly 1", len(r.Findings))
	}
	if r.Findings[0].Category != CategoryUnstructured {
		t.Errorf("category = %q, want %q", r.Findings[0].Category, CategoryUnstructured)
	}
	if !r.Unstructured {
		t.Error("result must be marked unstructured")
	}
	if !strings.Contains(r.Findings[0].Summary, "base case") {
		t.Errorf("prose lost: %q", r.Findings[0].Summary)
	}
}

func TestUnstructuredWithNoOutputIsStillNotACleanReview(t *testing.T) {
	r := Unstructured("codex", "openai", "m", "   \n ")
	if len(r.Findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(r.Findings))
	}
	if !strings.Contains(strings.ToLower(r.Findings[0].Summary), "no output") {
		t.Errorf("summary %q must say the backend produced nothing", r.Findings[0].Summary)
	}
}

func TestCategoryUnstructuredIsNotOneOfTheFive(t *testing.T) {
	for _, c := range Categories() {
		if c == CategoryUnstructured {
			t.Fatal("the unstructured marker must not be presented as one of the five categories")
		}
	}
	if len(Categories()) != 5 {
		t.Errorf("Categories() has %d entries, want the 5 AGENTS.md enumerates", len(Categories()))
	}
}

func TestRenderNamesTheBackendProviderAndModel(t *testing.T) {
	var buf bytes.Buffer
	Render(&buf, Result{
		Backend: "codex", Provider: "openai", Model: "gpt-5.6-terra",
		Findings: []Finding{{Category: CategorySecurity, Severity: SeverityBlocking, Summary: "key in log"}},
	})
	got := buf.String()
	for _, want := range []string{"codex", "openai", "gpt-5.6-terra", "security", "key in log"} {
		if !strings.Contains(got, want) {
			t.Errorf("render %q lacks %q", got, want)
		}
	}
}

func TestRenderReportsTruncationAndIncompleteness(t *testing.T) {
	var buf bytes.Buffer
	Render(&buf, Result{Backend: "openai", Truncated: true, Incomplete: true})
	got := buf.String()
	if !strings.Contains(got, "truncated") {
		t.Errorf("render %q does not report the truncated diff", got)
	}
	if !strings.Contains(got, "incomplete") {
		t.Errorf("render %q does not report the cut-off review", got)
	}
}

func TestRenderSaysSoWhenThereAreNoFindings(t *testing.T) {
	var buf bytes.Buffer
	Render(&buf, Result{Backend: "codex", Provider: "openai", Model: "m"})
	if !strings.Contains(buf.String(), "no findings") {
		t.Errorf("render %q must state a clean review explicitly", buf.String())
	}
}

func TestHasBlockingIsTrueOnlyForABlockingFinding(t *testing.T) {
	clean := Result{Findings: []Finding{{Category: CategoryConvention, Severity: SeverityAdvisory}}}
	if clean.HasBlocking() {
		t.Error("advisory findings must not report as blocking")
	}
	blocking := Result{Findings: []Finding{{Category: CategorySecurity, Severity: SeverityBlocking}}}
	if !blocking.HasBlocking() {
		t.Error("a blocking finding must report as blocking")
	}
}
