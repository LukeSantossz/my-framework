package check

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LukeSantossz/my-framework/internal/vcs"
)

// fixtureRepo builds a repository carrying just enough of the standards for the
// checks to read their vocabularies out of documents rather than from code.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "docs", "standards"))
	mkdir(t, filepath.Join(root, "docs", "specs"))
	mkdir(t, filepath.Join(root, "docs", "adr"))
	writeFile(t, filepath.Join(root, "docs", "standards", "github.md"), githubDoc)
	writeFile(t, filepath.Join(root, "docs", "standards", "spec_method.md"), specMethodDoc)
	writeFile(t, filepath.Join(root, "docs", "standards", "INDEX.md"),
		"# Index\n\n- `spec_method.md`: design layer.\n- `github.md`: commits.\n")
	git(t, root, "init", "-b", "main")
	git(t, root, "config", "user.email", "t@example.invalid")
	git(t, root, "config", "user.name", "T")
	git(t, root, "config", "commit.gpgsign", "false")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "chore: seed")
	return root
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func opts(root string) Options {
	return Options{RepoRoot: root, Base: "main", Repo: vcs.Open(root)}
}

const goodSpec = `# SPEC: feat(x): do a thing

## Problem
It is missing.

## Scope
- Includes: the thing
- Does NOT include:
  - the other thing

## Acceptance Criteria

- ` + "`does_the_thing`" + `
`

// --- spec -------------------------------------------------------------------

func TestSpecFailsANonTrivialBranchThatCarriesNoSpec(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	writeFile(t, filepath.Join(root, "code.go"), "package x\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "feat: code")

	res, err := Spec(opts(root))
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if res.OK() {
		t.Fatal("a branch changing code with no spec must fail the Gate")
	}
	if !strings.Contains(res.Problems[0].Message, "docs/specs") {
		t.Errorf("problem %q does not say where a spec belongs", res.Problems[0].Message)
	}
}

func TestSpecPassesWhenTheBranchAddsACompleteSpec(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	writeFile(t, filepath.Join(root, "code.go"), "package x\n")
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-do-a-thing.md"), goodSpec)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "feat: code")

	res, err := Spec(opts(root))
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if !res.OK() {
		t.Errorf("a complete spec must pass: %+v", res.Problems)
	}
}

func TestSpecFailsAnEmptyDoesNotIncludeList(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	body := strings.Replace(goodSpec, "- Does NOT include:\n  - the other thing", "- Does NOT include:", 1)
	writeFile(t, filepath.Join(root, "code.go"), "package x\n")
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-do-a-thing.md"), body)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "feat: code")

	res, _ := Spec(opts(root))
	if res.OK() {
		t.Fatal("an empty Does NOT include list must fail; it is what blocks scope creep")
	}
}

func TestSpecFailsASpecWithNoAcceptanceCriterion(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	body := strings.Replace(goodSpec, "- `does_the_thing`", "To be decided.", 1)
	writeFile(t, filepath.Join(root, "code.go"), "package x\n")
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-do-a-thing.md"), body)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "feat: code")

	res, _ := Spec(opts(root))
	if res.OK() {
		t.Fatal("a spec with no criterion must fail the Gate")
	}
}

func TestSpecExemptsABranchWhoseEveryPathIsExempt(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "docs/typo")
	writeFile(t, filepath.Join(root, "README.md"), "# hi\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "docs: typo")

	o := opts(root)
	o.ExemptPaths = []string{"README.md"}
	res, err := Spec(o)
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if !res.OK() {
		t.Errorf("an exempt-only branch must not require a spec: %+v", res.Problems)
	}
}

// --- commit -----------------------------------------------------------------

func TestCommitDerivesTheTypeVocabularyFromTheDocument(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	writeFile(t, filepath.Join(root, "a.txt"), "a\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "chore: allowed by the fixture table")

	res, err := Commit(opts(root))
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if !res.OK() {
		t.Errorf("a type present in the document must pass: %+v", res.Problems)
	}

	// Adding a type to the document alone must make it valid — no code change.
	doc := githubDoc + "\n"
	doc = strings.Replace(doc, "- chore: build", "- wibble: an invented type.\n- chore: build", 1)
	writeFile(t, filepath.Join(root, "docs", "standards", "github.md"), doc)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "wibble: now valid because the document says so")

	res, _ = Commit(opts(root))
	if !res.OK() {
		t.Errorf("a type added to github.md must be accepted with no code change: %+v", res.Problems)
	}
}

func TestCommitFailsATypeAbsentFromTheTable(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	writeFile(t, filepath.Join(root, "a.txt"), "a\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "wibble: not in the table")

	res, _ := Commit(opts(root))
	if res.OK() {
		t.Fatal("a type absent from the Type Table must fail")
	}
	if !strings.Contains(res.Problems[0].Message, "wibble") {
		t.Errorf("problem %q does not name the bad type", res.Problems[0].Message)
	}
}

func TestCommitFailsAnAttributionTrailer(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	writeFile(t, filepath.Join(root, "a.txt"), "a\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "feat: a", "-m", "Co-Authored-By: Some Model <noreply@example.invalid>")

	res, _ := Commit(opts(root))
	if res.OK() {
		t.Fatal("an attribution trailer must fail; github.md forbids it")
	}
}

func TestCommitFailsASubjectThatIsNotConventional(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	writeFile(t, filepath.Join(root, "a.txt"), "a\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "fixed the thing")

	res, _ := Commit(opts(root))
	if res.OK() {
		t.Fatal("a non-Conventional subject must fail")
	}
}

func TestCommitFailsLoudlyWhenTheTypeTableCannotBeRead(t *testing.T) {
	// Falling back to a compiled-in list would reinstate the parallel
	// vocabulary the standards forbid.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "standards", "github.md"), "# GitHub\n\nnothing here\n")
	git(t, root, "checkout", "-b", "feat/thing")
	if _, err := Commit(opts(root)); err == nil {
		t.Fatal("want a hard error when the document cannot be parsed")
	}
}

// --- branch -----------------------------------------------------------------

func TestBranchAcceptsATypeFromTheTable(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "fix/a-thing")
	res, err := Branch(opts(root))
	if err != nil {
		t.Fatalf("Branch: %v", err)
	}
	if !res.OK() {
		t.Errorf("fix/ must be accepted: %+v", res.Problems)
	}
}

func TestBranchRejectsATypeAbsentFromTheTable(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "wibble/a-thing")
	res, _ := Branch(opts(root))
	if res.OK() {
		t.Fatal("a branch type absent from the table must fail")
	}
}

func TestBranchRejectsANameWithNoType(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "my-branch")
	res, _ := Branch(opts(root))
	if res.OK() {
		t.Fatal("a branch with no type prefix must fail")
	}
}

// --- docs -------------------------------------------------------------------

func TestDocsFailsAStandardMissingFromTheIndex(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "standards", "orphan.md"), "# Orphan\n")
	res, err := Docs(opts(root))
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}
	if res.OK() {
		t.Fatal("an orphan standard must fail")
	}
}

func TestDocsFailsADanglingReference(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "standards", "github.md"), githubDoc+"\nSee `nowhere.md`.\n")
	res, _ := Docs(opts(root))
	if res.OK() {
		t.Fatal("a reference to a missing file must fail")
	}
}

func TestDocsFailsRetiredWording(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "standards", "github.md"), githubDoc+"\nThe Self-Review Checklist applies.\n")
	res, _ := Docs(opts(root))
	if res.OK() {
		t.Fatal("retired wording must fail")
	}
}

// --- records ----------------------------------------------------------------

func TestRecordsAcceptsContiguousNumbering(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), "x")
	writeFile(t, filepath.Join(root, "docs", "specs", "0002-b.md"), "x")
	res, err := Records(opts(root))
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if !res.OK() {
		t.Errorf("contiguous numbering must pass: %+v", res.Problems)
	}
}

func TestRecordsFailsAGapLeftByADeletedRecord(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), "x")
	writeFile(t, filepath.Join(root, "docs", "specs", "0003-c.md"), "x")
	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("a gap must fail; a superseded record is retired in place, not deleted")
	}
}

func TestRecordsFailsNumberingThatDoesNotStartAtOne(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "adr", "0002-b.md"), "x")
	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("numbering that does not start at 0001 must fail")
	}
}

// --- all --------------------------------------------------------------------

func TestAllRunsEveryCheck(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "fix/a-thing")
	results, err := All(opts(root))
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	names := map[string]bool{}
	for _, r := range results {
		names[r.Name] = true
	}
	for _, want := range []string{"spec", "commit", "branch", "docs", "records"} {
		if !names[want] {
			t.Errorf("All did not run the %q check", want)
		}
	}
}
