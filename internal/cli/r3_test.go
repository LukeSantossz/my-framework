package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LukeSantossz/my-framework/internal/forge"
	"github.com/LukeSantossz/my-framework/internal/report"
	"github.com/LukeSantossz/my-framework/internal/role"
	"github.com/LukeSantossz/my-framework/internal/vcs"
)

// fakeForge answers the pull request and records what was posted.
type fakeForge struct {
	pullJSON string
	posted   []string
	patched  []string
	srv      *httptest.Server
}

func newForge(t *testing.T, pullJSON string) *fakeForge {
	t.Helper()
	f := &fakeForge{pullJSON: pullJSON}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/pulls/"):
			fmt.Fprint(w, f.pullJSON)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/comments"):
			fmt.Fprint(w, "[]")
		case r.Method == http.MethodPost:
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			f.posted = append(f.posted, body["body"])
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, "{}")
		case r.Method == http.MethodPatch:
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			f.patched = append(f.patched, body["body"])
			fmt.Fprint(w, "{}")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeForge) env(t *testing.T, root string, args ...string) (Env, *strings.Builder) {
	t.Helper()
	e, _, _ := reviewEnv(t, root, args...)
	var out strings.Builder
	e.Stdout = &out
	e.Getenv = func(name string) string {
		switch name {
		case "GITHUB_REPOSITORY":
			return "o/r"
		case "GITHUB_API_URL":
			return f.srv.URL
		case "GITHUB_TOKEN":
			return "t"
		}
		return ""
	}
	return e, &out
}

const r3Project = `
version = 1

[roles.r3]
backends = ["absent"]

[backends.absent]
kind = "cli"
provider = "someone"
command = "definitely-not-installed-reviewer"
`

func prJSON(fork bool, fullName string) string {
	return fmt.Sprintf(`{"number":7,"title":"feat: a thing","body":"the intent",
	 "base":{"ref":"main","sha":"aaa"},
	 "head":{"ref":"feat/x","sha":"bbb","repo":{"fork":%v,"full_name":%q}}}`, fork, fullName)
}

func TestR3ReportsThatItCannotRunOnAFork(t *testing.T) {
	// Secrets are unavailable to fork workflows by design. Exiting quietly would
	// be indistinguishable from a review that found nothing.
	root := gitRepo(t, r3Project)
	branchWithChange(t, root, "feat/x")
	f := newForge(t, prJSON(true, "someone/r"))
	e, out := f.env(t, root, "review", "--role", "r3", "--pr", "7", "--post")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if !strings.Contains(out.String(), "fork") {
		t.Errorf("output does not explain the fork limitation:\n%s", out.String())
	}
	if len(f.posted) != 0 {
		t.Errorf("something was posted from a fork run: %v", f.posted)
	}
}

func TestR3ExitsZeroAndPostsWhenNoBackendWasAvailable(t *testing.T) {
	root := gitRepo(t, r3Project)
	branchWithChange(t, root, "feat/x")
	f := newForge(t, prJSON(false, "o/r"))
	e, out := f.env(t, root, "review", "--role", "r3", "--pr", "7", "--post")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, out.String())
	}
	if len(f.posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(f.posted))
	}
	if !strings.Contains(f.posted[0], "did not run") {
		t.Errorf("the comment does not report the absence:\n%s", f.posted[0])
	}
	if !strings.Contains(f.posted[0], forge.Marker) {
		t.Error("the comment carries no marker, so a re-run would append rather than replace")
	}
}

func TestR3RejectsANonNumericPullRequest(t *testing.T) {
	root := gitRepo(t, r3Project)
	e, _, errOut := reviewEnv(t, root, "review", "--role", "r3", "--pr", "seven")
	if code := Run(e); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "seven") {
		t.Errorf("stderr %q does not name the bad value", errOut.String())
	}
}

func TestPostWithoutAPullRequestIsRefused(t *testing.T) {
	root := gitRepo(t, r3Project)
	branchWithChange(t, root, "feat/x")
	e, _, errOut := reviewEnv(t, root, "review", "--role", "r3", "--post")
	if code := Run(e); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "--pr") {
		t.Errorf("stderr %q does not say what is missing", errOut.String())
	}
}

func TestR3FailsOnAForgeErrorBecauseThatIsMisconfiguration(t *testing.T) {
	root := gitRepo(t, r3Project)
	branchWithChange(t, root, "feat/x")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	e, _, errOut := reviewEnv(t, root, "review", "--role", "r3", "--pr", "7")
	e.Getenv = func(name string) string {
		switch name {
		case "GITHUB_REPOSITORY":
			return "o/r"
		case "GITHUB_API_URL":
			return srv.URL
		}
		return ""
	}
	if code := Run(e); code == 0 {
		t.Error("exit 0 despite being unable to read the pull request")
	}
	if !strings.Contains(errOut.String(), "401") {
		t.Errorf("stderr %q does not carry the cause", errOut.String())
	}
}

// --- comment rendering ------------------------------------------------------

func TestCommentNamesTheBackendProviderModelAndCategoryCounts(t *testing.T) {
	out := role.Outcome{
		Ran: true,
		Result: report.Result{
			Backend: "deepseek", Provider: "deepseek", Model: "deepseek-v4",
			Findings: []report.Finding{
				{Category: report.CategorySecurity, Severity: report.SeverityBlocking, File: "a.go", Line: 4, Summary: "key in log"},
				{Category: report.CategoryConvention, Severity: report.SeverityAdvisory, Summary: "naming"},
			},
		},
	}
	body := renderComment(out)
	for _, want := range []string{"deepseek", "deepseek-v4", "security: 1", "convention: 1", "2 finding(s)", "a.go:4", "advisory"} {
		if !strings.Contains(body, want) {
			t.Errorf("comment lacks %q:\n%s", want, body)
		}
	}
}

func TestCommentStatesACleanReviewExplicitly(t *testing.T) {
	body := renderComment(role.Outcome{Ran: true, Result: report.Result{Backend: "codex", Provider: "openai", Model: "m"}})
	if !strings.Contains(body, "No findings reported") {
		t.Errorf("a clean review must say so:\n%s", body)
	}
}

func TestCommentReportsSkippedBackendsSoAFallbackIsVisible(t *testing.T) {
	out := role.Outcome{
		Ran:     true,
		Result:  report.Result{Backend: "gemini", Provider: "google", Model: "m"},
		Skipped: []role.Skip{{Backend: "codex", Reason: "out of quota"}},
	}
	body := renderComment(out)
	if !strings.Contains(body, "codex") || !strings.Contains(body, "out of quota") {
		t.Errorf("a fallback that is not named passes for the original:\n%s", body)
	}
}

func TestCommentMarksATruncatedOrIncompleteReviewAsPartial(t *testing.T) {
	body := renderComment(role.Outcome{Ran: true, Result: report.Result{
		Backend: "d", Truncated: true, Incomplete: true,
	}})
	if !strings.Contains(body, "truncated") || !strings.Contains(body, "incomplete") {
		t.Errorf("a partial review must read as partial:\n%s", body)
	}
}

// --- linked specs -----------------------------------------------------------

// specBranch commits a spec at rel on a new branch, and returns the branch name.
func specBranch(t *testing.T, root, rel string) string {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("checkout", "-b", "feat/x")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(rel))), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, filepath.FromSlash(rel)), "# SPEC: feat: a thing\n\n## Scope\nthe scope R3 judges creep against.\n")
	write(t, filepath.Join(root, "a.txt"), "hello\n")
	run("add", ".")
	run("commit", "-m", "feat: a")
	return "feat/x"
}

func TestTheLinkedSpecIsReadWhereTheRepositoryKeepsIts(t *testing.T) {
	// Scope creep is one of the five categories, and it cannot be judged without
	// the scope. A reader pinned to `docs/specs` sends R3 into a repository that
	// files its specs elsewhere with no scope at all — and reports nothing about
	// having lost it.
	root := gitRepo(t, "version = 1\n\n[paths]\nspecs = \"docs/decisions\"\n")
	head := specBranch(t, root, "docs/decisions/0001-a-thing.md")

	if got := repoSpecsDir(root); got != "docs/decisions" {
		t.Fatalf("repoSpecsDir = %q, want the configured directory", got)
	}
	specs := linkedSpecs(vcs.Open(root), "main", head, repoSpecsDir(root))
	if len(specs) != 1 {
		t.Fatalf("read %d specs, want the one this branch adds: %+v", len(specs), specs)
	}
	if !strings.Contains(specs[0].body, "the scope R3 judges creep against") {
		t.Errorf("the spec was located but not read: %q", specs[0].body)
	}
}

func TestTheLinkedSpecIsReadFromTheDefaultDirectoryWhenNoneIsConfigured(t *testing.T) {
	root := gitRepo(t, "version = 1\n")
	head := specBranch(t, root, "docs/specs/0001-a-thing.md")

	specs := linkedSpecs(vcs.Open(root), "main", head, repoSpecsDir(root))
	if len(specs) != 1 {
		t.Fatalf("read %d specs, want the one this branch adds: %+v", len(specs), specs)
	}
}
