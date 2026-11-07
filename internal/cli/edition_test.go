package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const acmeProject = `version = 1

[backends.acme]
kind = "cli"
provider = "acme"
command = "acme-cli"
`

// editionSource lays out every input the sec-free edition is built from.
func editionSource(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"LICENSE": "MIT License\n",
		"docs/agents/instructions.md": "# Agent Instructions\n\n<!-- mf:role shared -->\n## Shared\n\n" +
			"<!-- mf:role author -->\n## Author\n\n<!-- mf:role reviewer -->\n## Reviewer\n",
		"docs/editions/secfree/README.md":          "# sec-free\n",
		"docs/editions/secfree/isolated_review.md": "# Isolated-Context Review\n",
	}
	for _, name := range []string{"INDEX.md", "spec_method.md", "code_conventions.md", "var_method.md",
		"github.md", "crura_method.md", "ai_guidelines.md", "token_economy.md"} {
		files["docs/standards/"+name] = "# " + name + "\n"
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, path, body)
	}
}

func TestEditionBuildWritesTheSecfreeTree(t *testing.T) {
	root := gitRepo(t, acmeProject)
	editionSource(t, root)
	out := filepath.Join(t.TempDir(), "secfree")

	e, stdout, stderr := reviewEnv(t, root, "edition", "build", "secfree", "--out", out)
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(out, "CLAUDE.md")); err != nil {
		t.Errorf("CLAUDE.md was not written: %v", err)
	}
	if !strings.Contains(stdout.String(), "docs/standards/isolated_review.md") {
		t.Errorf("the run does not list what it wrote: %q", stdout.String())
	}
}

func TestEditionBuildForbidsEveryBackendThePolicyDeclares(t *testing.T) {
	root := gitRepo(t, acmeProject)
	editionSource(t, root)
	write(t, filepath.Join(root, "docs", "standards", "github.md"), "# github\n\nReview with acme-cli.\n")

	e, stdout, stderr := reviewEnv(t, root, "edition", "build", "secfree", "--out", filepath.Join(t.TempDir(), "o"))
	if code := Run(e); code != 1 {
		t.Fatalf("exit %d, want 1: %s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "acme-cli") {
		t.Errorf("stderr does not name the backend command: %q", stderr.String())
	}
}

func TestEditionBuildRejectsAnUnknownEditionOrAMissingOut(t *testing.T) {
	root := gitRepo(t, acmeProject)
	for _, args := range [][]string{
		{"edition", "build", "corporate", "--out", "x"},
		{"edition", "build", "secfree"},
		{"edition", "publish", "secfree", "--out", "x"},
		{"edition"},
	} {
		e, _, _ := reviewEnv(t, root, args...)
		if code := Run(e); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}

func TestCheckEditionSaysSoWhenThereIsNoEditionSource(t *testing.T) {
	// An adopting repository carries no edition source. The gate passes there,
	// and says it compared nothing rather than reading like a build that ran.
	root := gitRepo(t, acmeProject)
	e, stdout, stderr := reviewEnv(t, root, "check", "edition")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok   edition") || !strings.Contains(stdout.String(), "nothing is built") {
		t.Errorf("stdout %q does not say the gate had nothing to build", stdout.String())
	}
}

func TestCheckEditionFailsOnMainWhenTheEditionWouldNotBuild(t *testing.T) {
	root := gitRepo(t, acmeProject)
	editionSource(t, root)
	write(t, filepath.Join(root, "docs", "standards", "github.md"), "# github\n\nRun `mf check` first.\n")

	e, stdout, _ := reviewEnv(t, root, "check", "edition")
	if code := Run(e); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAIL edition") || !strings.Contains(stdout.String(), "github.md:3") {
		t.Errorf("stdout %q does not report the failing line", stdout.String())
	}
}

func TestCheckEditionPassesWhenTheEditionBuilds(t *testing.T) {
	root := gitRepo(t, acmeProject)
	editionSource(t, root)
	e, stdout, stderr := reviewEnv(t, root, "check", "edition")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok   edition") {
		t.Errorf("stdout %q", stdout.String())
	}
}
