package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LukeSantossz/my-framework/internal/check"
)

// vendoredStandardsRepo builds a repository whose standards sit where a
// consumer that takes this framework as a `.standards` submodule keeps them,
// rather than at the default location.
func vendoredStandardsRepo(t *testing.T, project string) string {
	t.Helper()
	root := gitRepo(t, project)
	standards := filepath.Join(root, ".standards", "docs", "standards")
	if err := os.MkdirAll(standards, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(standards, "INDEX.md"), "# Index\n\n- `github.md`: commits.\n")
	write(t, filepath.Join(standards, "github.md"), "# GitHub\n\nThe conventions this repository follows.\n")
	return root
}

func TestCheckReadsStandardsFromADirectoryOutsideDocsStandards(t *testing.T) {
	// The one downstream consumer of this framework has no `docs/standards` at
	// all: it takes these documents as a `.standards` submodule. Until the
	// location was configuration, `mf check` stopped on the first document it
	// could not open, which made the harness unusable in the one place it was
	// meant to be used.
	root := vendoredStandardsRepo(t,
		"version = 1\n\n[paths]\nstandards = \".standards/docs/standards\"\n")

	e, out, errOut := reviewEnv(t, root, "check", "docs")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d against a vendored standards tree: %s%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "ok   docs") {
		t.Errorf("the docs gate did not pass: %q", out.String())
	}
}

func TestCheckWithoutTheConfiguredDirectoryLooksWhereTheDefaultSays(t *testing.T) {
	// The other half of the same claim: the standards are read from the
	// configured directory, not found by searching, so the same repository
	// without the key fails on the documents it does not have there.
	root := vendoredStandardsRepo(t, "version = 1\n")

	e, _, errOut := reviewEnv(t, root, "check", "docs")
	if code := Run(e); code == 0 {
		t.Fatal("the gate passed against a standards tree at neither location")
	}
	if !strings.Contains(errOut.String(), "INDEX.md") {
		t.Errorf("stderr %q does not name the document it could not read", errOut.String())
	}
}

func TestTheDocumentLocationsFallBackToTheShippedLayout(t *testing.T) {
	// A resolver that answered empty would hand every gate the repository root,
	// so the fallback is asserted rather than assumed.
	if got := standardsDir(nil); got != check.DefaultStandardsDir {
		t.Errorf("standardsDir(nil) = %q, want %q", got, check.DefaultStandardsDir)
	}
	if got := specsDir(nil); got != check.DefaultSpecsDir {
		t.Errorf("specsDir(nil) = %q, want %q", got, check.DefaultSpecsDir)
	}

	root := gitRepo(t, "version = 1\n")
	e, _, _ := reviewEnv(t, root, "check")
	cfg, code := load(e)
	if code != 0 {
		t.Fatal("the fixture's configuration does not load")
	}
	if got := standardsDir(cfg); got != check.DefaultStandardsDir {
		t.Errorf("a repository configuring no paths resolved %q, want %q", got, check.DefaultStandardsDir)
	}
	if got := specsDir(cfg); got != check.DefaultSpecsDir {
		t.Errorf("a repository configuring no paths resolved %q, want %q", got, check.DefaultSpecsDir)
	}
}

// --- one commit message -------------------------------------------------------

// typeTableRepo builds a repository whose github.md carries a Type Table, which
// is where both commit modes read their vocabulary from.
func typeTableRepo(t *testing.T) string {
	t.Helper()
	root := gitRepo(t, "version = 1\n")
	standards := filepath.Join(root, "docs", "standards")
	if err := os.MkdirAll(standards, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(standards, "github.md"),
		"# GitHub\n\n### Type Table\n\n- feat: new feature.\n- fix: bug fix.\n- chore: tooling.\n")
	return root
}

func messagePath(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	write(t, path, body)
	return path
}

func TestCheckCommitAcceptsAMessageFileTheTableAllows(t *testing.T) {
	// The commit-msg hook is handed the message being written as $1. Without
	// this mode the hook can only check the commits already on the branch, so
	// a bad subject is caught one commit after the one that has to change.
	root := typeTableRepo(t)
	e, out, errOut := reviewEnv(t, root, "check", "commit", "--message", messagePath(t, "feat(x): a subject\n"))
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "ok   commit") {
		t.Errorf("the gate did not report itself as the commit gate: %q", out.String())
	}
}

func TestCheckCommitRejectsAMessageFileTheTableDoesNot(t *testing.T) {
	root := typeTableRepo(t)
	path := messagePath(t, "wibble: not in the table\n")
	e, out, _ := reviewEnv(t, root, "check", "commit", "--message", path)
	if code := Run(e); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "FAIL commit") || !strings.Contains(got, "wibble") {
		t.Errorf("the failure does not name the gate and the bad type: %q", got)
	}
}

func TestCheckMessageBelongsToTheCommitGateAlone(t *testing.T) {
	// It validates one message, so pairing it with the gates that read a
	// branch would silently check something other than what was asked.
	root := typeTableRepo(t)
	path := messagePath(t, "feat(x): a subject\n")
	for _, args := range [][]string{
		{"check", "--message", path},
		{"check", "docs", "--message", path},
		{"check", "commit", "docs", "--message", path},
	} {
		e, _, errOut := reviewEnv(t, root, args...)
		if code := Run(e); code != 2 {
			t.Errorf("%v: exit %d, want 2 (usage)", args, code)
		}
		if !strings.Contains(errOut.String(), "--message") {
			t.Errorf("%v: stderr %q does not name the flag", args, errOut.String())
		}
	}
}

func TestCheckMessageWithNoPathIsAUsageError(t *testing.T) {
	root := typeTableRepo(t)
	e, _, errOut := reviewEnv(t, root, "check", "commit", "--message")
	if code := Run(e); code != 2 {
		t.Errorf("exit %d, want 2 (usage)", code)
	}
	if !strings.Contains(errOut.String(), "--message") {
		t.Errorf("stderr %q does not name the flag", errOut.String())
	}
}

func TestAgentsGateSaysNothingIsBeingComparedWhenNoTargetIsDeclared(t *testing.T) {
	// A gate that passes by not running reads exactly like one that ran and
	// found the files in order. CLAUDE.md promises this gate "fails when they
	// have drifted apart", so the line has to say when there is nothing it
	// could fail on.
	root := gitRepo(t, "version = 1\n")
	e, out, _ := reviewEnv(t, root, "check", "agents")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "nothing is being compared") {
		t.Errorf("the line does not say the gate had nothing to check: %q", got)
	}
}
