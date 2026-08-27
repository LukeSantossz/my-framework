package check

import (
	"fmt"
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
	return fixtureRepoAt(t, DefaultStandardsDir, DefaultSpecsDir)
}

// fixtureRepoAt builds the same repository with its documents somewhere other
// than the default layout, which is the shape of any adopter that vendors this
// framework rather than copying it.
func fixtureRepoAt(t *testing.T, standardsRel, specsRel string) string {
	t.Helper()
	root := t.TempDir()
	standards := filepath.Join(root, filepath.FromSlash(standardsRel))
	mkdir(t, standards)
	mkdir(t, filepath.Join(root, filepath.FromSlash(specsRel)))
	mkdir(t, filepath.Join(root, "docs", "adr"))
	writeFile(t, filepath.Join(standards, "github.md"), githubDoc)
	writeFile(t, filepath.Join(standards, "spec_method.md"), specMethodDoc)
	writeFile(t, filepath.Join(standards, "INDEX.md"),
		"# Index\n\n- `spec_method.md`: design layer.\n- `github.md`: commits.\n")
	git(t, root, "init", "-b", "main")
	git(t, root, "config", "user.email", "t@example.invalid")
	git(t, root, "config", "user.name", "T")
	git(t, root, "config", "commit.gpgsign", "false")
	// git forks `gc --auto` after a commit and goes on writing into
	// .git/objects after the test body returns, so t.TempDir() fails its own
	// cleanup with "directory not empty" — a flake unrelated to what is
	// asserted here, which took down a release build.
	git(t, root, "config", "gc.auto", "0")
	git(t, root, "config", "maintenance.auto", "false")
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

// specStub is the smallest thing the records gate accepts as a spec: a
// numbered file carrying the header that says what it is.
const specStub = "# SPEC: chore(x): a stub\n"

// byteOrderMark is spelled as a rune rather than pasted in, because pasting it
// would make it as invisible here as it is in the document that carries one —
// and a test whose subject cannot be seen in its own source is one nobody can
// maintain.
var byteOrderMark = string(rune(0xFEFF))

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
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	writeFile(t, filepath.Join(root, "docs", "specs", "0002-b.md"), specStub)
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
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	writeFile(t, filepath.Join(root, "docs", "specs", "0003-c.md"), specStub)
	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("a gap must fail; a superseded record is retired in place, not deleted")
	}
}

func TestRecordsAcceptsAGapAtANumberAnotherBranchHolds(t *testing.T) {
	// Durable numbers are claimed when a spec is written, so two changes open
	// at once means one branch holds 0002 while another claims 0003. The second
	// has a gap in its own tree, has deleted nothing, and could not push: both
	// hooks fail closed.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs(spec): a")

	git(t, root, "checkout", "-b", "other")
	writeFile(t, filepath.Join(root, "docs", "specs", "0002-b.md"), specStub)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs(spec): b")

	git(t, root, "checkout", "main")
	git(t, root, "checkout", "-b", "mine")
	if err := os.Remove(filepath.Join(root, "docs", "specs", "0002-b.md")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "docs", "specs", "0003-c.md"), specStub)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs(spec): c")

	res, err := Records(opts(root))
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if !res.OK() {
		t.Errorf("a number another branch holds is claimed, not deleted: %+v", res.Problems)
	}
}

func TestRecordsStillFailsAGapAtANumberNoRefHolds(t *testing.T) {
	// The narrowing must not turn the check off: a hole nobody is filling is
	// still a hole.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	writeFile(t, filepath.Join(root, "docs", "specs", "0003-c.md"), specStub)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs(spec): a and c")
	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("a gap no ref fills must still fail")
	}
}

func TestRecordsStillFailsAGapANonRecordFileOnAnotherRefWouldExcuse(t *testing.T) {
	// A draft below the archive, or a file that is not a record, must not claim
	// a number: the gate would then accept a gap no record fills.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs(spec): a")

	git(t, root, "checkout", "-b", "other")
	mkdir(t, filepath.Join(root, "docs", "specs", "drafts"))
	writeFile(t, filepath.Join(root, "docs", "specs", "drafts", "0002-b.md"), specStub)
	writeFile(t, filepath.Join(root, "docs", "specs", "0002-not-a-record.txt"), specStub)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs(spec): drafts")

	git(t, root, "checkout", "main")
	git(t, root, "checkout", "-b", "mine")
	writeFile(t, filepath.Join(root, "docs", "specs", "0003-c.md"), specStub)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs(spec): c")

	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("a gap only a draft and a non-record file would fill must still fail")
	}
}

func TestNumberingAsksAboutOtherRefsOnlyWhenThereIsAGap(t *testing.T) {
	// Enumerating refs costs a git process per ref, per archive, and almost
	// every archive has no gap at all. A check that asked every time would
	// charge every push for a question it does not ask.
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "specs")
	mkdir(t, dir)
	writeFile(t, filepath.Join(dir, "0001-a.md"), specStub)
	writeFile(t, filepath.Join(dir, "0002-b.md"), specStub)

	asked := 0
	lookup := func() map[int]string {
		asked++
		return nil
	}
	if _, err := numbering("specs", dir, lookup); err != nil {
		t.Fatal(err)
	}
	if asked != 0 {
		t.Errorf("a contiguous archive asked about other refs %d time(s)", asked)
	}

	writeFile(t, filepath.Join(dir, "0004-d.md"), specStub)
	if _, err := numbering("specs", dir, lookup); err != nil {
		t.Fatal(err)
	}
	if asked != 1 {
		t.Errorf("a gap asked %d time(s), want exactly one", asked)
	}
}

func TestRecordsFailsADuplicateHoweverManyRefsHoldIt(t *testing.T) {
	// Duplicates are the part of this rule with no second check behind it.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-b.md"), specStub)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs(spec): two of them")
	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("a reused number must fail whatever the refs hold")
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

func TestRecordsReportsTheSameOrderOnEveryRun(t *testing.T) {
	// Gate output is read as a diff in CI logs, so an unchanged tree must
	// produce a byte-identical report. Two things would otherwise randomise it:
	// the labels are walked in Go map order, and several problems share one
	// File value ("specs"), which an unstable sort is free to reorder.
	root := fixtureRepo(t)
	// Every even number, so each pair leaves a gap and the whole run reports
	// under one File value. There have to be enough of them: Go's sort falls
	// back to insertion sort on short slices, which happens to be stable, and
	// only reorders equal keys once the slice is long enough to be partitioned.
	for n := 2; n <= 40; n += 2 {
		writeFile(t, filepath.Join(root, "docs", "specs", fmt.Sprintf("%04d-a.md", n)), specStub)
	}
	writeFile(t, filepath.Join(root, "docs", "adr", "0002-b.md"), "x")

	first, err := Records(opts(root))
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	sharing := 0
	for _, p := range first.Problems {
		if p.File == "specs" {
			sharing++
		}
	}
	if sharing < 16 {
		t.Fatalf("fixture must produce many problems sharing a File value, got %d: %+v", sharing, first.Problems)
	}
	for run := 0; run < 50; run++ {
		again, err := Records(opts(root))
		if err != nil {
			t.Fatalf("Records: %v", err)
		}
		if len(again.Problems) != len(first.Problems) {
			t.Fatalf("run %d reported %d problems, first run reported %d", run, len(again.Problems), len(first.Problems))
		}
		for i := range again.Problems {
			if again.Problems[i] != first.Problems[i] {
				t.Fatalf("run %d differs at %d: %+v vs %+v", run, i, again.Problems[i], first.Problems[i])
			}
		}
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

func TestDocsResolvesReferencesCaseSensitivelyOnEveryPlatform(t *testing.T) {
	// A reference whose case does not match the file resolves on a
	// case-insensitive filesystem and fails on a case-sensitive one, so the gate
	// passes for the developer and fails in CI. The check has to disagree with
	// the local filesystem to agree with everybody else's.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "standards", "github.md"),
		githubDoc+"\nSee `GITHUB.md` for the type table.\n")
	res, err := Docs(opts(root))
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}
	if res.OK() {
		t.Fatal("a reference differing only in case was accepted")
	}
}

func TestDocsAcceptsAReferenceWhoseCaseMatches(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "standards", "github.md"),
		githubDoc+"\nSee `docs/standards/github.md` for the type table.\n")
	res, err := Docs(opts(root))
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}
	if !res.OK() {
		t.Fatalf("a correctly cased reference was rejected: %+v", res.Problems)
	}
}

func TestDocsTreatsTheDesignFormatNameAsProse(t *testing.T) {
	// `DESIGN.md` is the name of the format design.md adopts, not a file in this
	// repository. It sits beside SPEC.md in the same list for the same reason.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "standards", "github.md"),
		githubDoc+"\nThe identity is written in the `DESIGN.md` format.\n")
	res, err := Docs(opts(root))
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}
	if !res.OK() {
		t.Fatalf("a format name in prose was read as a file reference: %+v", res.Problems)
	}
}

// --- document locations -----------------------------------------------------

func TestGatesReadTheStandardsWhereTheRepositoryKeepsThem(t *testing.T) {
	// The one downstream consumer takes this framework as a `.standards` git
	// submodule, so every document it is checked against sits under that
	// directory. A gate that can only read `docs/standards` cannot run there at
	// all: it stops on the first document it cannot open.
	vendored := ".standards/docs/standards"
	root := fixtureRepoAt(t, vendored, DefaultSpecsDir)
	git(t, root, "checkout", "-b", "fix/a-thing")

	o := opts(root)
	o.StandardsDir = vendored
	results, err := All(o)
	if err != nil {
		t.Fatalf("the gates could not read the vendored standards: %v", err)
	}
	for _, r := range results {
		if !r.OK() {
			t.Errorf("%s failed against a vendored tree: %+v", r.Name, r.Problems)
		}
	}

	// And the directory is what did it: the same repository read at the default
	// location has no standards to read at all.
	if _, err := All(opts(root)); err == nil {
		t.Fatal("the default location resolved documents that are not there")
	}
}

func TestSpecsAreDiscoveredWhereTheRepositoryKeepsThem(t *testing.T) {
	// Where a repository files its specs is its own decision. Discovery pinned
	// to `docs/specs` reports a branch that carries a spec as carrying none,
	// which is a gate failing exactly the change that satisfied it.
	elsewhere := "docs/decisions"
	root := fixtureRepoAt(t, DefaultStandardsDir, elsewhere)
	git(t, root, "checkout", "-b", "feat/thing")
	writeFile(t, filepath.Join(root, "code.go"), "package x\n")
	writeFile(t, filepath.Join(root, filepath.FromSlash(elsewhere), "0001-do-a-thing.md"), goodSpec)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "feat: code")

	o := opts(root)
	o.SpecsDir = elsewhere
	res, err := Spec(o)
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if !res.OK() {
		t.Errorf("a spec in the configured directory was not found: %+v", res.Problems)
	}

	// Unconfigured, the same branch has no spec where the default says to look,
	// and the message says where it looked.
	res, err = Spec(opts(root))
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if res.OK() {
		t.Fatal("a spec outside the configured directory was accepted")
	}
	if !strings.Contains(res.Problems[0].Message, DefaultSpecsDir) {
		t.Errorf("problem %q does not say where a spec was expected", res.Problems[0].Message)
	}
}

func TestAnAbsoluteDocumentDirectoryIsTakenAsGiven(t *testing.T) {
	// A configured path may be either form. Joining an absolute one onto the
	// repository root would produce a path on no filesystem.
	root := fixtureRepo(t)
	o := opts(root)
	o.StandardsDir = filepath.Join(root, "docs", "standards")
	if got := o.Defaults().StandardsDir; got != filepath.Clean(o.StandardsDir) {
		t.Errorf("absolute standards dir resolved to %q, want %q", got, o.StandardsDir)
	}
	if got := opts(root).Defaults().SpecsDir; got != filepath.Join(root, "docs", "specs") {
		t.Errorf("relative specs dir resolved to %q, want it under the repository root", got)
	}
}

func TestAVendoredStandardResolvesAReferenceAgainstTheTreeItCameFrom(t *testing.T) {
	// The documents cross-reference each other by their path inside the corpus
	// they belong to. Vendored as a submodule, that corpus has its own root, and
	// a resolver that only knows the repository root reports every
	// cross-reference in the corpus as a missing file — which is the whole
	// corpus failing a gate over where it was mounted.
	vendored := ".standards/docs/standards"
	root := fixtureRepoAt(t, vendored, DefaultSpecsDir)
	// One reference by its path inside the corpus, and one bare name of a file
	// that sits at the corpus root rather than at this repository's.
	writeFile(t, filepath.Join(root, ".standards", "CONTEXT.md"), "# The vendored corpus\n")
	writeFile(t, filepath.Join(root, filepath.FromSlash(vendored), "github.md"),
		githubDoc+"\nThe Gate is in `docs/standards/spec_method.md`, the domain in `CONTEXT.md`.\n")

	o := opts(root)
	o.StandardsDir = vendored
	res, err := Docs(o)
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}
	if !res.OK() {
		t.Errorf("a reference inside the vendored corpus was reported as missing: %+v", res.Problems)
	}
}

func TestAReferenceToAFileNeitherRootHasIsStillReported(t *testing.T) {
	// The second root widens where a reference may resolve, never whether it
	// has to resolve at all.
	vendored := ".standards/docs/standards"
	root := fixtureRepoAt(t, vendored, DefaultSpecsDir)
	writeFile(t, filepath.Join(root, filepath.FromSlash(vendored), "github.md"),
		githubDoc+"\nSee `docs/standards/nothing_here.md`.\n")

	o := opts(root)
	o.StandardsDir = vendored
	res, _ := Docs(o)
	if res.OK() {
		t.Fatal("a reference to a file in neither tree was accepted")
	}
}

// --- merge commits ----------------------------------------------------------

// mergeMainInto advances main and merges it back into branch, which is the
// shape of every long-lived branch that keeps up with its base.
func mergeMainInto(t *testing.T, root, branch string) {
	t.Helper()
	git(t, root, "checkout", "main")
	writeFile(t, filepath.Join(root, "on-main.txt"), "main moved\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "chore: move main on")
	git(t, root, "checkout", branch)
	git(t, root, "merge", "--no-ff", "--no-edit", "main")
}

func TestCommitSkipsTheSubjectAMergeGenerates(t *testing.T) {
	// A merge subject is written by git or by the forge, and the Type Table
	// governs what an author writes. Over this repository's own history fifteen
	// `Merge pull request #N from ...` subjects fail the Conventional Commits
	// shape, and any branch that merges its base back in carries one.
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	writeFile(t, filepath.Join(root, "a.txt"), "a\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "feat: a")
	mergeMainInto(t, root, "feat/thing")

	res, err := Commit(opts(root))
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if !res.OK() {
		t.Fatalf("a merge subject nobody authored must not fail the gate: %+v", res.Problems)
	}
	if !strings.Contains(res.Note, "merge") {
		t.Errorf("note %q does not say a merge was skipped", res.Note)
	}
}

func TestCommitStillChecksWhatAMergeBringsAlong(t *testing.T) {
	// Skipping the merge must not skip the branch: those commits are the ones
	// an author did write.
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	writeFile(t, filepath.Join(root, "a.txt"), "a\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "wibble: not in the table")
	mergeMainInto(t, root, "feat/thing")

	res, _ := Commit(opts(root))
	if res.OK() {
		t.Fatal("a bad subject on a branch carrying a merge went unreported")
	}
}

// --- one message ------------------------------------------------------------

func messageFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	writeFile(t, path, body)
	return path
}

func TestCommitMessageReadsTheSameVocabularyTheBranchModeDoes(t *testing.T) {
	// docs/adr/0009: the vocabulary comes from the standard, never from a list
	// in the binary — which is proven by adding a type to the document alone.
	root := fixtureRepo(t)
	res, err := CommitMessage(opts(root), messageFile(t, "chore: allowed by the fixture table\n"))
	if err != nil {
		t.Fatalf("CommitMessage: %v", err)
	}
	if !res.OK() {
		t.Errorf("a type present in the document must pass: %+v", res.Problems)
	}

	doc := strings.Replace(githubDoc, "- chore: build", "- wibble: an invented type.\n- chore: build", 1)
	writeFile(t, filepath.Join(root, "docs", "standards", "github.md"), doc)
	res, err = CommitMessage(opts(root), messageFile(t, "wibble: now valid because the document says so\n"))
	if err != nil {
		t.Fatalf("CommitMessage: %v", err)
	}
	if !res.OK() {
		t.Errorf("a type added to github.md must be accepted with no code change: %+v", res.Problems)
	}
}

func TestCommitMessageRejectsATypeAbsentFromTheTable(t *testing.T) {
	root := fixtureRepo(t)
	res, _ := CommitMessage(opts(root), messageFile(t, "wibble: not in the table\n"))
	if res.OK() {
		t.Fatal("a type absent from the Type Table must fail")
	}
	if !strings.Contains(res.Problems[0].Message, "wibble") {
		t.Errorf("problem %q does not name the bad type", res.Problems[0].Message)
	}
}

func TestCommitMessageRejectsASubjectThatIsNotConventional(t *testing.T) {
	root := fixtureRepo(t)
	res, _ := CommitMessage(opts(root), messageFile(t, "fixed the thing\n"))
	if res.OK() {
		t.Fatal("a non-Conventional subject must fail")
	}
}

func TestCommitMessageRejectsAnAttributionTrailer(t *testing.T) {
	root := fixtureRepo(t)
	res, _ := CommitMessage(opts(root),
		messageFile(t, "feat: a\n\nCo-Authored-By: Some Model <noreply@example.invalid>\n"))
	if res.OK() {
		t.Fatal("an attribution trailer must fail; github.md forbids it")
	}
}

func TestCommitMessageIgnoresWhatGitStripsBeforeRecordingIt(t *testing.T) {
	// The hook is handed the file under the editor's cursor, which still
	// carries git's own comment lines and, under `commit --verbose`, the whole
	// diff below the scissors. Reading either as the message would report a
	// comment as the subject and every `#` line as an unconventional one.
	root := fixtureRepo(t)
	body := "# Please enter the commit message for your changes.\n" +
		"\n" +
		"feat(x): a real subject\n" +
		"\n" +
		"A body that explains why.\n" +
		"# On branch feat/thing\n" +
		"# ------------------------ >8 ------------------------\n" +
		"diff --git a/a.txt b/a.txt\n" +
		"+Co-Authored-By: a line that lives in the diff, not the message\n"
	res, err := CommitMessage(opts(root), messageFile(t, body))
	if err != nil {
		t.Fatalf("CommitMessage: %v", err)
	}
	if !res.OK() {
		t.Errorf("git's own comments and the verbose diff must not be read as the message: %+v", res.Problems)
	}
}

func TestCommitMessageSkipsTheSubjectGitWritesForAMerge(t *testing.T) {
	// git runs this hook for `git merge` too, handing it a MERGE_MSG it wrote
	// itself. Rejecting that would make the hook block every merge over a
	// subject no author typed — the branch-mode defect, moved to commit time.
	root := fixtureRepo(t)
	for _, subject := range []string{
		"Merge branch 'main' into feat/thing",
		"Merge pull request #15 from LukeSantossz/feat/thing",
		"Merge remote-tracking branch 'origin/main'",
	} {
		res, err := CommitMessage(opts(root), messageFile(t, subject+"\n"))
		if err != nil {
			t.Fatalf("CommitMessage(%q): %v", subject, err)
		}
		if !res.OK() {
			t.Errorf("%q was rejected: %+v", subject, res.Problems)
		}
	}
}

func TestCommitMessageRejectsAMessageWithNothingInIt(t *testing.T) {
	root := fixtureRepo(t)
	res, _ := CommitMessage(opts(root), messageFile(t, "# only a comment\n\n"))
	if res.OK() {
		t.Fatal("a message with no subject must fail rather than pass as conventional")
	}
}

func TestCommitMessageFailsLoudlyWhenTheFileIsNotThere(t *testing.T) {
	// A hook that cannot read the message it was handed has not checked it.
	root := fixtureRepo(t)
	if _, err := CommitMessage(opts(root), filepath.Join(root, "no-such-file")); err == nil {
		t.Fatal("want an error for a message file that does not exist")
	}
}

// --- the durable archive ------------------------------------------------------

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

// archiveDoc is what a repository's own decision record looks like once it
// carries the pins and the closed deletion list the gate reads.
func archiveDoc(block string) string {
	return "# Durable spec archive\n\n" + ArchiveMarker + "\n" + "```" + "toml\n" + block + "```" + "\n"
}

func writeArchive(t *testing.T, root, block string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "docs", "adr", "0001-durable-spec-archive.md"), archiveDoc(block))
}

func TestRecordsFailsASpecMissingTheHeaderThatSaysWhatItIs(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), "Some prose with no header.\n")
	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("a spec without the `# SPEC:` header must fail")
	}
	if !strings.Contains(res.Problems[0].File, "0001-a.md") {
		t.Errorf("problem %+v does not name the file", res.Problems[0])
	}
}

func TestRecordsAcceptsASpecWhoseHeaderCarriesAByteOrderMark(t *testing.T) {
	// A BOM is invisible in the editor that wrote it, so the gate rejected a
	// file whose first line the author could see was exactly right, and named
	// the header as the fault. The mark is transit damage, not content.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), byteOrderMark+specStub)
	res, err := Records(opts(root))
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if !res.OK() {
		t.Errorf("a spec carrying a byte-order mark must pass: %+v", res.Problems)
	}
}

func TestRecordsFailsAStrayDocumentInTheSpecsDirectory(t *testing.T) {
	// The specs directory is the archive. A file in it that is not a spec is
	// either a spec that lost its header or something that does not belong.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	writeFile(t, filepath.Join(root, "docs", "specs", "notes.md"), "scratch\n")
	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("a stray document in the archive must fail")
	}
}

func TestRecordsFailsARecordThatHistorySaysWasDeleted(t *testing.T) {
	// Contiguity cannot see this: deleting the highest-numbered record leaves
	// 0001..N-1 contiguous and clean, which is the exact shape of the incident
	// the rule was written after.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	writeFile(t, filepath.Join(root, "docs", "specs", "0002-b.md"), specStub)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "docs: two records")
	if err := os.Remove(filepath.Join(root, "docs", "specs", "0002-b.md")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs: delete the highest record")

	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("a record deleted rather than retired in place must fail")
	}
	found := false
	for _, p := range res.Problems {
		if strings.Contains(p.File, "0002-b.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("no problem names the deleted record: %+v", res.Problems)
	}
}

func TestRecordsAcceptsARecordThatWasRenamedRatherThanRemoved(t *testing.T) {
	// git detects renames by default, so the commit that renames a record
	// reports it as R and the new name never appears as an addition. Reading
	// additions alone left the old name missing from the tree, and the gate
	// reported a deletion for a record still sitting there — naming the
	// deletion archive as the remedy for something nobody deleted.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "docs: one record")
	git(t, root, "mv", "docs/specs/0001-a.md", "docs/specs/0001-a-clearer-slug.md")
	git(t, root, "commit", "-m", "docs: rename the record")

	res, err := Records(opts(root))
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if !res.OK() {
		t.Errorf("a renamed record is still in the tree and must not be reported deleted: %+v", res.Problems)
	}
}

func TestRecordsStillFailsARecordDeletedAfterBeingRenamed(t *testing.T) {
	// The rename chain is followed to its end, not treated as an excuse: a
	// record renamed and then deleted is as gone as one deleted outright, and
	// resolving renames must not become a way to leave the archive.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "docs: one record")
	git(t, root, "mv", "docs/specs/0001-a.md", "docs/specs/0001-a-clearer-slug.md")
	git(t, root, "commit", "-m", "docs: rename the record")
	git(t, root, "rm", "-q", "docs/specs/0001-a-clearer-slug.md")
	git(t, root, "commit", "-m", "docs: remove it after all")

	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("a record deleted at the end of a rename chain must still fail")
	}
}

func TestRecordsAcceptsADeletionTheArchiveAccountsFor(t *testing.T) {
	// Two records were removed before the rule existed and cannot be restored.
	// The archive names them and names what carries their decision today.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	writeFile(t, filepath.Join(root, "docs", "specs", "0002-b.md"), specStub)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "docs: two records")
	if err := os.Remove(filepath.Join(root, "docs", "specs", "0002-b.md")); err != nil {
		t.Fatal(err)
	}
	writeArchive(t, root, "[deleted]\n\"docs/specs/0002-b.md\" = \"docs/adr/0001-durable-spec-archive.md\"\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs: account for the deletion")

	res, err := Records(opts(root))
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if !res.OK() {
		t.Errorf("an accounted deletion must pass: %+v", res.Problems)
	}
}

func TestRecordsRefusesADeletionAccountedToARecordThatIsNotThere(t *testing.T) {
	// The entry has to point at where the decision went, or it is a way of
	// waving any deletion through by adding a line.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), specStub)
	writeFile(t, filepath.Join(root, "docs", "specs", "0002-b.md"), specStub)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "docs: two records")
	if err := os.Remove(filepath.Join(root, "docs", "specs", "0002-b.md")); err != nil {
		t.Fatal(err)
	}
	writeArchive(t, root, "[deleted]\n\"docs/specs/0002-b.md\" = \"docs/adr/0099-nowhere.md\"\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs: account for the deletion badly")

	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("a deletion accounted to a record nothing carries must fail")
	}
}

// pinnedArchive commits a root SPEC.md, then archives it under docs/specs and
// records the pin, which is how the backfilled records came to exist.
func pinnedArchive(t *testing.T, root, archived string) string {
	t.Helper()
	writeFile(t, filepath.Join(root, "SPEC.md"), specStub)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "docs: the working spec")
	source := gitOutput(t, root, "rev-parse", "HEAD")

	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a.md"), archived)
	if err := os.Remove(filepath.Join(root, "SPEC.md")); err != nil {
		t.Fatal(err)
	}
	writeArchive(t, root, "[extracted]\n\"0001-a.md\" = \""+source+":SPEC.md\"\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "docs: archive it")
	return source
}

func TestRecordsAcceptsAnArchivePinWhoseBlobStillMatches(t *testing.T) {
	root := fixtureRepo(t)
	pinnedArchive(t, root, specStub)
	res, err := Records(opts(root))
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if !res.OK() {
		t.Errorf("a verbatim archived record must pass: %+v", res.Problems)
	}
}

func TestRecordsFailsAnArchivePinWhoseBlobNoLongerMatches(t *testing.T) {
	// The archive is verbatim history. Blob to blob, so a line-ending
	// conversion cannot make a faithful copy look edited, and an edit cannot
	// hide behind one.
	root := fixtureRepo(t)
	pinnedArchive(t, root, specStub+"\nA paragraph the extraction never had.\n")
	res, _ := Records(opts(root))
	if res.OK() {
		t.Fatal("an edited archive record must fail its pin")
	}
	if !strings.Contains(res.Problems[0].Message, "extract") {
		t.Errorf("problem %+v does not say what the pin is about", res.Problems[0])
	}
}

func TestRecordsFailsLoudlyWhenTheArchiveBlockCannotBeRead(t *testing.T) {
	// Same reason the vocabulary parsers are hard errors: a block that stops
	// parsing must not quietly become an empty one, which would turn every pin
	// and every accounted deletion off at once.
	root := fixtureRepo(t)
	writeFile(t, filepath.Join(root, "docs", "adr", "0001-durable-spec-archive.md"),
		"# Durable spec archive\n\n"+ArchiveMarker+"\n\nno fenced block here\n")
	if _, err := Records(opts(root)); err == nil {
		t.Fatal("want a hard error when the archive block cannot be read")
	}
}

func TestDocsIgnoresAMarkdownReferenceInsideAURL(t *testing.T) {
	// The gate runs in the pre-push hook, so a standard that cites an upstream
	// document by URL stopped the push, naming a path that was never a path.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "INDEX.md"), "# Index\n\n- `code_conventions.md`: rules.\n")
	writeFile(t, filepath.Join(dir, "code_conventions.md"),
		"# Conventions\n\nSee https://github.com/conventional-commits/spec/blob/main/SPECIFICATION.md for the upstream text.\n")

	res, err := Docs(Options{StandardsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range res.Problems {
		if strings.Contains(p.Message, "SPECIFICATION.md") {
			t.Errorf("a URL was read as a repository path: %s", p.Message)
		}
	}
}

func TestCountCriteriaCountsAnOrderedList(t *testing.T) {
	// The Gate rejected such a spec with "states no criterion", which is the
	// one message that cannot tell the author what to change.
	if n := countCriteria("1. returns_empty_list_when_no_matches\n2. refuses_an_absolute_path\n"); n != 2 {
		t.Errorf("counted %d criteria in an ordered list, want 2", n)
	}
	if n := countCriteria("1) first\n2) second\n3) third\n"); n != 3 {
		t.Errorf("counted %d criteria in a paren-numbered list, want 3", n)
	}
	if n := countCriteria("Prose about the change.\n"); n != 0 {
		t.Errorf("counted %d criteria in prose, want 0", n)
	}
	if n := countCriteria("- a\n- b\n"); n != 2 {
		t.Errorf("counted %d criteria in a bullet list, want 2", n)
	}
}

func TestSpecFailsAnEmptyDoesNotIncludeAboveTheIncludesLine(t *testing.T) {
	// The list is what blocks scope creep, so it is satisfied by content that
	// belongs to it — not by whatever line happens to follow it in the section.
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	body := strings.Replace(goodSpec,
		"- Includes: the thing\n- Does NOT include:\n  - the other thing",
		"- Does NOT include:\n- Includes: the thing", 1)
	writeFile(t, filepath.Join(root, "code.go"), "package x\n")
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-do-a-thing.md"), body)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "feat: code")

	res, _ := Spec(opts(root))
	if res.OK() {
		t.Fatal("an empty Does NOT include list passed because another Scope item followed it")
	}
}

func TestSpecExemptsNothingWhenThePatternTrimsToNothing(t *testing.T) {
	// The prefix branch exists so `docs/specs/*` can match across a separator,
	// which filepath.Match cannot do. With `*` the prefix is empty, every path
	// begins with it, and the Spec Gate was switched off for every change by a
	// one-character value that does not look like a total disable in a diff.
	//
	// The changed file sits one directory down, which is where the two branches
	// differ: `*` matched it only through the prefix branch, never through the
	// glob, which does not cross a separator.
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	mkdir(t, filepath.Join(root, "internal"))
	writeFile(t, filepath.Join(root, "internal", "code.go"), "package x\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "feat: code")

	o := opts(root)
	o.ExemptPaths = []string{"*"}
	res, err := Spec(o)
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if res.OK() {
		t.Fatal("`*` exempted a change that carries no spec")
	}
}

func TestSpecStillExemptsADirectoryPrefixPattern(t *testing.T) {
	// What the prefix branch is for, and what this repository's own policy
	// depends on: `docs/specs/*` must reach a file one separator down.
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "docs/record")
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-a-record.md"), specStub)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "docs: a record")

	o := opts(root)
	o.ExemptPaths = []string{"docs/specs/*"}
	res, err := Spec(o)
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if !res.OK() {
		t.Errorf("a directory prefix pattern stopped matching: %+v", res.Problems)
	}
}

func TestSpecFailsAnEmptyDoesNotIncludeWhoseSectionListsWhatItDoesInclude(t *testing.T) {
	// The next scope item ends the search rather than being stepped over:
	// skipping only its heading left the items beneath it — which say what the
	// change does include — standing in for the list that says what it leaves
	// out.
	root := fixtureRepo(t)
	git(t, root, "checkout", "-b", "feat/thing")
	body := strings.Replace(goodSpec,
		"- Includes: the thing\n- Does NOT include:\n  - the other thing",
		"- Does NOT include:\n- Includes:\n  - the thing", 1)
	writeFile(t, filepath.Join(root, "code.go"), "package x\n")
	writeFile(t, filepath.Join(root, "docs", "specs", "0001-do-a-thing.md"), body)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "feat: code")

	res, _ := Spec(opts(root))
	if res.OK() {
		t.Fatal("an empty Does NOT include list was satisfied by the Includes list below it")
	}
}
