package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const explainProject = `
version = 1

[roles.explain]
backends = ["explainer"]

[backends.explainer]
kind = "cli"
provider = "google"
command = "definitely-not-installed-gemini"
model = "explainer-model-9"
`

func explainEnv(t *testing.T, root string, outside string, args ...string) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	return Env{
		Args:        args,
		Stdout:      &out,
		Stderr:      &errOut,
		RepoRoot:    root,
		MachinePath: filepath.Join(t.TempDir(), "config.toml"),
		Now:         func() time.Time { return time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC) },
		Getenv: func(name string) string {
			if name == "MF_EXPLAIN_DIR" {
				return outside
			}
			return ""
		},
		GitConfig: func(string) (string, bool) { return "", false },
	}, &out, &errOut
}

func TestExplainResolvesItsModelThroughTheRoleConfiguration(t *testing.T) {
	// Which model explains is configuration like every other role, so it comes
	// from the explain chain rather than from a setting of its own.
	root := gitRepo(t, explainProject)
	branchWithChange(t, root, "feat/role-runner")
	outside := t.TempDir()
	e, out, _ := explainEnv(t, root, outside, "explain", "--dir", outside, "--dry-run")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	got := out.String()
	for _, want := range []string{"explainer", "google", "explainer-model-9", "medium"} {
		if !strings.Contains(got, want) {
			t.Errorf("dry run output %q lacks %q", got, want)
		}
	}
}

func TestExplainWritesOutsideVersionControlAndSaysWhere(t *testing.T) {
	root := gitRepo(t, explainProject)
	branchWithChange(t, root, "feat/role-runner")
	outside := t.TempDir()
	e, out, _ := explainEnv(t, root, outside, "explain", "--dir", outside, "--dry-run")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "2026-08-24-feat-role-runner.html") {
		t.Errorf("the destination is not the date-prefixed artifact: %q", got)
	}
	if !strings.Contains(got, outside) {
		t.Errorf("the destination is not the configured directory: %q", got)
	}
}

func TestExplainNeverBlocksOrReportsAVerdict(t *testing.T) {
	// crux_method.md and ADR 0003 both make CRUX an aid feeding R1 and CRURA.
	// An aid that can fail a run is a gate wearing another name.
	root := gitRepo(t, explainProject)
	branchWithChange(t, root, "feat/x")
	e, out, _ := explainEnv(t, root, t.TempDir(), "explain", "--dir", t.TempDir())
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "no explainer produced") {
		t.Errorf("the absence was not stated: %q", got)
	}
	if !strings.Contains(got, "note the absent CRUX aid in the pull request") {
		t.Errorf("the fallback does not mirror the R2 absent-reviewer note: %q", got)
	}
	for _, verdict := range []string{"blocking", "FAIL", "finding"} {
		if strings.Contains(got, verdict) {
			t.Errorf("the explainer reported a verdict (%q): %q", verdict, got)
		}
	}
}

func TestExplainWithNoConfiguredChainSaysSoAndStillExitsZero(t *testing.T) {
	root := gitRepo(t, "version = 1\n")
	branchWithChange(t, root, "feat/x")
	e, out, _ := explainEnv(t, root, t.TempDir(), "explain", "--dir", t.TempDir())
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if !strings.Contains(out.String(), "no backend configured") {
		t.Errorf("output %q does not name the missing configuration", out.String())
	}
}

func TestExplainRefusesADestinationInsideTheRepository(t *testing.T) {
	// A committed explainer becomes the durable per-change record the method
	// deliberately does not create.
	root := gitRepo(t, explainProject)
	branchWithChange(t, root, "feat/x")
	inside := filepath.Join(root, "docs", "crux")
	e, _, errOut := explainEnv(t, root, inside, "explain", "--dir", inside)
	if code := Run(e); code == 0 {
		t.Fatal("a destination inside the repository was accepted")
	}
	if !strings.Contains(errOut.String(), "outside version control") {
		t.Errorf("stderr %q does not say why the destination was refused", errOut.String())
	}
	// Refused before anything ran, so nothing was created and no quota spent.
	if _, err := os.Stat(inside); err == nil {
		t.Error("the refused destination was created anyway")
	}
}

func TestExplainRejectsAnUnknownDifficulty(t *testing.T) {
	root := gitRepo(t, explainProject)
	e, _, errOut := explainEnv(t, root, t.TempDir(), "explain", "--difficulty", "brutal")
	if code := Run(e); code == 0 {
		t.Fatal("an unknown difficulty was accepted")
	}
	if !strings.Contains(errOut.String(), "brutal") {
		t.Errorf("stderr %q does not name the rejected value", errOut.String())
	}
}

func TestExplainRejectsAnOptionWhoseValueIsMissing(t *testing.T) {
	// `mf explain --dir "$OUT"` with an unset $OUT would write the explainer
	// somewhere the caller never named, and a mistyped command line is the
	// caller's error rather than a degraded explainer.
	root := gitRepo(t, explainProject)
	for _, args := range [][]string{
		{"explain", "--base"},
		{"explain", "--difficulty"},
		{"explain", "--dir"},
	} {
		e, _, errOut := explainEnv(t, root, t.TempDir(), args...)
		if code := Run(e); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
		if !strings.Contains(errOut.String(), "expects a value") {
			t.Errorf("%v: stderr %q does not say the value is missing", args, errOut.String())
		}
	}
}
