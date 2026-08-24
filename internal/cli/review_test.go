package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepo builds a real repository with a branch that adds a file, so the
// review path exercises the same plumbing it will in use.
func gitRepo(t *testing.T, projectBody string) (root string) {
	t.Helper()
	root = t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "t@example.invalid")
	run("config", "user.name", "T")
	run("config", "commit.gpgsign", "false")
	write(t, filepath.Join(root, "seed.txt"), "seed\n")
	if projectBody != "" {
		write(t, filepath.Join(root, ".framework.toml"), projectBody)
	}
	run("add", ".")
	run("commit", "-m", "chore: seed")
	return root
}

func branchWithChange(t *testing.T, root, branch string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("checkout", "-b", branch)
	write(t, filepath.Join(root, "a.txt"), "hello\n")
	run("add", ".")
	run("commit", "-m", "feat: a")
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func reviewEnv(t *testing.T, root string, args ...string) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	return Env{
		Args:        args,
		Stdout:      &out,
		Stderr:      &errOut,
		RepoRoot:    root,
		MachinePath: filepath.Join(t.TempDir(), "config.toml"),
		Getenv:      func(string) string { return "" },
		GitConfig:   func(string) (string, bool) { return "", false },
	}, &out, &errOut
}

const chainProject = `
version = 1

[roles.r2]
backends = ["codex", "fallback"]

[backends.codex]
kind = "cli"
provider = "openai"
command = "definitely-not-installed-codex"
args = ["review", "--base", "{{.Base}}"]
unavailable_patterns = ["usage limit"]

[backends.fallback]
kind = "cli"
provider = "google"
command = "definitely-not-installed-gemini"
`

func TestReviewDryRunDescribesTheWholeChainAndRunsNothing(t *testing.T) {
	root := gitRepo(t, chainProject)
	branchWithChange(t, root, "feat/x")
	e, out, _ := reviewEnv(t, root, "review", "--role", "r2", "--dry-run")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	got := out.String()
	// Both backends, not just the first: the point of a dry run is the fallbacks.
	for _, want := range []string{"codex", "fallback", "--base", "main"} {
		if !strings.Contains(got, want) {
			t.Errorf("dry run output %q lacks %q", got, want)
		}
	}
}

func TestReviewSkipsWhenTheBranchIsItsOwnBase(t *testing.T) {
	// Answered here rather than by a backend: it is a property of the branch,
	// and announcing a review of nothing would be a false entry in the PR.
	root := gitRepo(t, chainProject)
	e, out, _ := reviewEnv(t, root, "review", "--role", "r2")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "nothing to review against itself") {
		t.Errorf("output %q does not explain the skip", out.String())
	}
}

func TestReviewExitsZeroAndNamesEveryBackendWhenNoneIsAvailable(t *testing.T) {
	// A reviewer that never ran is not a finding, so an expired quota or a
	// missing tool must never lock the repository.
	root := gitRepo(t, chainProject)
	branchWithChange(t, root, "feat/x")
	e, out, _ := reviewEnv(t, root, "review", "--role", "r2")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, out.String())
	}
	got := out.String()
	for _, want := range []string{"codex", "fallback", "did not run", "Record the absence"} {
		if !strings.Contains(got, want) {
			t.Errorf("output %q lacks %q", got, want)
		}
	}
}

func TestReviewRejectsAnUnknownRole(t *testing.T) {
	root := gitRepo(t, chainProject)
	e, _, errOut := reviewEnv(t, root, "review", "--role", "r9")
	if code := Run(e); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "r9") {
		t.Errorf("stderr %q does not name the bad role", errOut.String())
	}
}

func TestReviewReportsABackendTheChainNamesButNothingDefines(t *testing.T) {
	// A typo in a chain is a misconfiguration, not an unavailable reviewer.
	// Reporting it as unavailable would let it look like a vendor outage and
	// silently degrade the layer.
	project := "version = 1\n\n[roles.r2]\nbackends = [\"ghost\"]\n"
	root := gitRepo(t, project)
	branchWithChange(t, root, "feat/x")
	e, _, errOut := reviewEnv(t, root, "review", "--role", "r2")
	if code := Run(e); code == 0 {
		t.Error("exit 0 for a chain naming an undefined backend")
	}
	if !strings.Contains(errOut.String(), "ghost") {
		t.Errorf("stderr %q does not name the undefined backend", errOut.String())
	}
}

func TestReviewReportsAnEmptyChangeRatherThanReviewingNothing(t *testing.T) {
	root := gitRepo(t, chainProject)
	run := exec.Command("git", "checkout", "-b", "feat/empty")
	run.Dir = root
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v\n%s", err, out)
	}
	e, out, _ := reviewEnv(t, root, "review", "--role", "r2")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "adds nothing over") {
		t.Errorf("output %q does not report the empty change", out.String())
	}
}

func TestReviewReportsTheCrossProviderStateForR2Only(t *testing.T) {
	root := gitRepo(t, chainProject)
	branchWithChange(t, root, "feat/x")

	// r2 with nothing available still reports; the state line appears only once
	// a backend has reviewed, so here we assert the role gating via r1 instead.
	project := strings.Replace(chainProject, "[roles.r2]", "[roles.r1]", 1)
	root2 := gitRepo(t, project)
	branchWithChange(t, root2, "feat/x")
	e, out, _ := reviewEnv(t, root2, "review", "--role", "r1")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(out.String(), "Cross-provider") {
		t.Errorf("r1 reported a cross-provider state: %q", out.String())
	}
}
