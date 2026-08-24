package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LukeSantossz/my-framework/internal/report"
)

func corpus(t *testing.T, cases map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, manifest := range cases {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "case.toml"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "change.diff"), []byte("diff --git a/x b/x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const plantedCase = `
version = 1
name = "off by one"

[[defects]]
category = "correctness"
file = "counter.go"
terms = ["off by one", "off-by-one", "bound"]
`

const cleanCase = `
version = 1
name = "a mechanical rename"
`

func TestLoadReadsCasesAndTheirDiffs(t *testing.T) {
	dir := corpus(t, map[string]string{"0001-planted": plantedCase, "0002-clean": cleanCase})
	cases, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2", len(cases))
	}
	if cases[0].Dir != "0001-planted" {
		t.Errorf("cases are not in a stable order: %+v", cases)
	}
	if !strings.Contains(cases[0].Diff, "diff --git") {
		t.Error("the diff was not loaded")
	}
	if !cases[1].IsClean() {
		t.Error("a case with no planted defects must read as clean")
	}
}

func TestLoadRefusesACorpusFromAnotherVersion(t *testing.T) {
	// Results across corpus versions measure different things, so lining them
	// up silently would produce a comparison nobody could defend.
	dir := corpus(t, map[string]string{"0001": strings.Replace(plantedCase, "version = 1", "version = 99", 1)})
	if _, err := Load(dir); err == nil {
		t.Fatal("want an error for a corpus this build does not read")
	}
}

func TestLoadRefusesADefectWithNoTerms(t *testing.T) {
	// A plant nothing can match would depress every hit rate forever, and the
	// cause would be invisible in the numbers.
	bad := "version = 1\nname = \"x\"\n\n[[defects]]\ncategory = \"correctness\"\n"
	dir := corpus(t, map[string]string{"0001": bad})
	if _, err := Load(dir); err == nil {
		t.Fatal("want an error for a defect with no terms")
	}
}

func TestLoadRefusesAnEmptyCorpus(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("an empty corpus scores everything perfectly and must be refused")
	}
}

// --- matching ---------------------------------------------------------------

func defects() []Defect {
	return []Defect{{Category: "correctness", File: "counter.go", Terms: []string{"off by one", "bound"}}}
}

func TestAFindingThatNamesTheDefectCountsAsAHit(t *testing.T) {
	res := Match([]report.Finding{
		{Category: report.CategoryCorrectness, File: "counter.go", Summary: "the loop bound is wrong"},
	}, defects())
	if res.Hits != 1 || res.FalsePositives != 0 {
		t.Errorf("res = %+v, want one hit and no false positive", res)
	}
}

func TestTheWrongCategoryIsNotAHit(t *testing.T) {
	// Crediting a finding filed under a different category would let a backend
	// score by describing the right line for the wrong reason.
	res := Match([]report.Finding{
		{Category: report.CategoryConvention, File: "counter.go", Summary: "the loop bound is wrong"},
	}, defects())
	if res.Hits != 0 {
		t.Errorf("res = %+v, want no hit", res)
	}
	if res.FalsePositives != 1 {
		t.Errorf("an unmatched finding must count as a false positive: %+v", res)
	}
}

func TestTheWrongFileIsNotAHit(t *testing.T) {
	res := Match([]report.Finding{
		{Category: report.CategoryCorrectness, File: "other.go", Summary: "off by one"},
	}, defects())
	if res.Hits != 0 || res.FalsePositives != 1 {
		t.Errorf("res = %+v", res)
	}
}

func TestAVagueFindingIsNotGenerouslyCredited(t *testing.T) {
	// The rule is deliberately conservative: a loose one inflates every number
	// and the inflation is invisible.
	res := Match([]report.Finding{
		{Category: report.CategoryCorrectness, File: "counter.go", Summary: "something looks off here"},
	}, defects())
	if res.Hits != 0 {
		t.Errorf("a finding matching no term was credited: %+v", res)
	}
}

func TestATermMatchesInTheRationaleToo(t *testing.T) {
	res := Match([]report.Finding{
		{Category: report.CategoryCorrectness, File: "counter.go", Summary: "look here", Rationale: "this is an off by one"},
	}, defects())
	if res.Hits != 1 {
		t.Errorf("res = %+v", res)
	}
}

func TestOnePlantCannotBeHitTwice(t *testing.T) {
	// Otherwise a backend that repeats itself scores higher than one that says
	// it once.
	res := Match([]report.Finding{
		{Category: report.CategoryCorrectness, File: "counter.go", Summary: "off by one"},
		{Category: report.CategoryCorrectness, File: "counter.go", Summary: "off by one again"},
	}, defects())
	if res.Hits != 1 {
		t.Errorf("hits = %d, want 1", res.Hits)
	}
	if res.FalsePositives != 1 {
		t.Errorf("the duplicate must count as a false positive: %+v", res)
	}
}

func TestACleanCaseTurnsEveryFindingIntoAFalsePositive(t *testing.T) {
	// This is what makes a backend that flags everything score badly.
	res := Match([]report.Finding{
		{Category: report.CategorySecurity, Summary: "looks risky"},
		{Category: report.CategoryConvention, Summary: "naming"},
	}, nil)
	if res.Planted != 0 || res.Hits != 0 || res.FalsePositives != 2 {
		t.Errorf("res = %+v", res)
	}
}

func TestAMissedPlantIsNamedSoItCanBeInvestigated(t *testing.T) {
	res := Match(nil, defects())
	if res.Hits != 0 || len(res.MissedTerms) != 1 {
		t.Errorf("res = %+v", res)
	}
	if !strings.Contains(res.MissedTerms[0], "correctness") {
		t.Errorf("the miss does not say what was missed: %v", res.MissedTerms)
	}
}

// --- report -----------------------------------------------------------------

func TestReportTalliesPerCategory(t *testing.T) {
	r := Report{CorpusVersion: CorpusVersion}
	c := Case{Dir: "0001", Defects: []Defect{
		{Category: "correctness", File: "a.go", Terms: []string{"off by one"}},
		{Category: "security", File: "b.go", Terms: []string{"hardcoded"}},
	}}
	findings := []report.Finding{
		{Category: report.CategoryCorrectness, File: "a.go", Summary: "off by one"},
	}
	r.Accumulate(c, Match(findings, c.Defects), findings)

	if r.ByCategory["correctness"].Hits != 1 {
		t.Errorf("correctness = %+v", r.ByCategory["correctness"])
	}
	if r.ByCategory["security"].Hits != 0 || r.ByCategory["security"].Planted != 1 {
		t.Errorf("security = %+v; a category with no hit must still show its denominator", r.ByCategory["security"])
	}
	hits, planted := r.HitRate()
	if hits != 1 || planted != 2 {
		t.Errorf("hit rate = %d/%d, want 1/2", hits, planted)
	}
}

func TestComparableRefusesResultsFromDifferentCorpora(t *testing.T) {
	a := Report{CorpusVersion: 1}
	b := Report{CorpusVersion: 2}
	if err := Comparable(a, b); err == nil {
		t.Fatal("results from different corpora must not be lined up")
	}
	if err := Comparable(a, Report{CorpusVersion: 1}); err != nil {
		t.Errorf("same-version results must compare: %v", err)
	}
}

func TestTheMatchingRuleIsStatedInFull(t *testing.T) {
	// It decides the score, so a reader must be able to see it without reading
	// the source.
	for _, want := range []string{"category", "file", "case-insensitively", "at most once", "false positive"} {
		if !strings.Contains(MatchingRule, want) {
			t.Errorf("the printed rule omits %q", want)
		}
	}
}

func TestTheShippedCorpusLoadsAndCoversEveryCategory(t *testing.T) {
	// The corpus is content, and content rots. This guards that what ships
	// parses, that every finding category is exercised, and that clean diffs
	// exist so false positives are measurable at all.
	cases, err := Load(filepath.Join("..", "..", "docs", "eval", "corpus"))
	if err != nil {
		t.Fatalf("the shipped corpus does not load: %v", err)
	}
	covered := map[string]bool{}
	clean := 0
	for _, c := range cases {
		if c.IsClean() {
			clean++
		}
		for _, d := range c.Defects {
			covered[d.Category] = true
		}
	}
	for _, want := range []string{"correctness", "invented-symbol", "scope-creep", "security", "convention"} {
		if !covered[want] {
			t.Errorf("the corpus plants nothing in the %q category", want)
		}
	}
	if clean == 0 {
		t.Error("the corpus has no clean diff, so a backend that flags everything cannot score badly")
	}
}
