package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scriptedReviewer answers every case with the same findings object, so the
// score is a property of the corpus and the matcher rather than of a model.
func scriptedReviewer(t *testing.T, findingsJSON string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%q},"finish_reason":"stop"}]}`, findingsJSON)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func evalRepo(t *testing.T, endpoint string) (root, machine string) {
	t.Helper()
	project := "version = 1\n\n[roles.r2]\nbackends = [\"scripted\"]\n\n" +
		"[backends.scripted]\nkind = \"api\"\nprovider = \"local\"\nmodel = \"fake-1\"\n"
	root = gitRepo(t, project)

	// One planted case and one clean case, so both halves of the metric are
	// exercised.
	planted := filepath.Join(root, "docs", "eval", "corpus", "0001-planted")
	clean := filepath.Join(root, "docs", "eval", "corpus", "0002-clean")
	for _, d := range []string{planted, clean} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "change.diff"), []byte("diff --git a/a.go b/a.go\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(planted, "case.toml"),
		"version = 1\nname = \"planted\"\n\n[[defects]]\ncategory = \"correctness\"\nfile = \"a.go\"\nterms = [\"off by one\"]\n")
	write(t, filepath.Join(clean, "case.toml"), "version = 1\nname = \"clean\"\n")

	machine = filepath.Join(t.TempDir(), "config.toml")
	write(t, machine, fmt.Sprintf("version = 1\n\n[providers.local]\nkind = \"openai-compatible\"\nendpoint = %q\n", endpoint))
	return root, machine
}

func TestEvalReportsHitRateAndFalsePositivesSeparately(t *testing.T) {
	// One finding that matches the plant. Against the clean case the same
	// answer is a false positive, which is exactly what the clean diff is for.
	srv := scriptedReviewer(t, `{"findings":[{"category":"correctness","file":"a.go","summary":"off by one in the bound"}]}`)
	root, machine := evalRepo(t, srv.URL)

	e, out, errOut := reviewEnv(t, root, "eval")
	e.MachinePath = machine
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	got := out.String()
	if !strings.Contains(got, "hit rate         1/1") {
		t.Errorf("hit rate not reported as a fraction with its denominator:\n%s", got)
	}
	if !strings.Contains(got, "false positives  1") {
		t.Errorf("the clean case did not produce a false positive:\n%s", got)
	}
	if !strings.Contains(got, "correctness      1/1") {
		t.Errorf("no per-category breakdown:\n%s", got)
	}
}

func TestEvalPrintsTheMatchingRuleAndTheProvenanceOfTheNumbers(t *testing.T) {
	srv := scriptedReviewer(t, `{"findings":[]}`)
	root, machine := evalRepo(t, srv.URL)
	e, out, _ := reviewEnv(t, root, "eval")
	e.MachinePath = machine
	Run(e)
	got := out.String()
	for _, want := range []string{"Matching rule", "at most once", "corpus", "date ", "fake-1", "not comparable to an independent evaluation"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
}

func TestEvalScoresABackendThatFlagsEverythingBadly(t *testing.T) {
	// A reviewer people stop reading is not a good reviewer, and a single
	// combined score would hide that.
	srv := scriptedReviewer(t, `{"findings":[
	 {"category":"security","summary":"looks risky"},
	 {"category":"convention","summary":"naming"},
	 {"category":"scope-creep","summary":"too big"}]}`)
	root, machine := evalRepo(t, srv.URL)
	e, out, _ := reviewEnv(t, root, "eval")
	e.MachinePath = machine
	Run(e)
	got := out.String()
	if !strings.Contains(got, "hit rate         0/1") {
		t.Errorf("a backend that found nothing planted scored a hit:\n%s", got)
	}
	if !strings.Contains(got, "false positives  6") {
		t.Errorf("three findings across two cases must be six false positives:\n%s", got)
	}
}

func TestEvalFailsRatherThanScoringWhenTheBackendIsUnavailable(t *testing.T) {
	// A zero here would be indistinguishable from a backend that found nothing.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	root, machine := evalRepo(t, srv.URL)
	e, out, errOut := reviewEnv(t, root, "eval")
	e.MachinePath = machine
	if code := Run(e); code == 0 {
		t.Errorf("exit 0 with an unreachable backend:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "scripted") {
		t.Errorf("stderr %q does not name the backend that could not be measured", errOut.String())
	}
}

func TestEvalRejectsAnUnknownOption(t *testing.T) {
	root, machine := evalRepo(t, "http://unused")
	e, _, errOut := reviewEnv(t, root, "eval", "--depth", "9")
	e.MachinePath = machine
	if code := Run(e); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "--depth") {
		t.Errorf("stderr %q does not name the bad option", errOut.String())
	}
}

func TestEvalRejectsAnOptionWhoseValueIsMissing(t *testing.T) {
	// Silently ignoring it would measure the default role and print the figure
	// under a heading the caller never asked for.
	root, machine := evalRepo(t, "http://unused")
	for _, args := range [][]string{
		{"eval", "--role"},
		{"eval", "--backend"},
	} {
		e, _, errOut := reviewEnv(t, root, args...)
		e.MachinePath = machine
		if code := Run(e); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
		if !strings.Contains(errOut.String(), "expects a value") {
			t.Errorf("%v: stderr %q does not say the value is missing", args, errOut.String())
		}
	}
}
