package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo builds a real git repository on disk. The plumbing is the subject, so
// faking git here would test the fake.
func newRepo(t *testing.T) *Repo {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.invalid")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "chore: seed")
	return Open(root)
}

func commitOnBranch(t *testing.T, r *Repo, branch, file, body string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = r.Root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	if branch != "" {
		run("checkout", "-b", branch)
	}
	if err := os.WriteFile(filepath.Join(r.Root, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "feat: "+file)
}

func TestResolvesReportsWhetherARefExists(t *testing.T) {
	r := newRepo(t)
	if !r.Resolves("main") {
		t.Error("main must resolve")
	}
	if r.Resolves("no-such-ref") {
		t.Error("a missing ref must not resolve")
	}
}

func TestCurrentBranchNamesTheCheckedOutBranch(t *testing.T) {
	r := newRepo(t)
	commitOnBranch(t, r, "feat/thing", "a.txt", "a\n")
	got, err := r.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != "feat/thing" {
		t.Errorf("CurrentBranch = %q, want %q", got, "feat/thing")
	}
}

func TestDiffReturnsTheChangeOfTheBranchAgainstItsBase(t *testing.T) {
	r := newRepo(t)
	commitOnBranch(t, r, "feat/thing", "a.txt", "hello\n")
	d, err := r.Diff("main", "feat/thing", 1<<20)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Empty {
		t.Fatal("diff reported empty for a branch that adds a file")
	}
	if !strings.Contains(d.Text, "a.txt") || !strings.Contains(d.Text, "hello") {
		t.Errorf("diff does not carry the change:\n%s", d.Text)
	}
	if d.Truncated {
		t.Error("a small diff must not report truncation")
	}
}

func TestDiffReportsAnEmptyChangeAsEmptyRatherThanAsAnError(t *testing.T) {
	r := newRepo(t)
	d, err := r.Diff("main", "main", 1<<20)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !d.Empty {
		t.Error("a branch against itself must report empty")
	}
}

func TestDiffTruncatesAtTheCapAndSaysSo(t *testing.T) {
	r := newRepo(t)
	commitOnBranch(t, r, "feat/big", "big.txt", strings.Repeat("x\n", 5000))
	d, err := r.Diff("main", "feat/big", 200)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !d.Truncated {
		t.Error("a diff past the cap must report truncation")
	}
	if len(d.Text) > 200 {
		t.Errorf("diff is %d bytes, want at most the 200-byte cap", len(d.Text))
	}
}

func TestDiffRefusesARefThatDoesNotResolve(t *testing.T) {
	// An unresolvable ref yields an empty diff from git, and reading that as
	// "nothing to review" would let a backend report a review it never made.
	r := newRepo(t)
	commitOnBranch(t, r, "feat/thing", "a.txt", "a\n")
	_, err := r.Diff("no-such-base", "feat/thing", 1<<20)
	if err == nil {
		t.Fatal("want an error for a base that does not resolve")
	}
	if !strings.Contains(err.Error(), "no-such-base") {
		t.Errorf("error %q does not name the ref", err)
	}
}

func TestChangedFilesListsWhatTheBranchTouched(t *testing.T) {
	r := newRepo(t)
	commitOnBranch(t, r, "feat/thing", "a.txt", "a\n")
	files, err := r.ChangedFiles("main", "feat/thing")
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	if len(files) != 1 || files[0] != "a.txt" {
		t.Errorf("ChangedFiles = %v, want [a.txt]", files)
	}
}

// --- the Author Declaration -------------------------------------------------

func TestAuthorDeclarationIsAbsentUntilItIsWritten(t *testing.T) {
	r := newRepo(t)
	if _, ok := r.AuthorDeclaration("main"); ok {
		t.Error("a branch nobody declared must report no declaration")
	}
}

func TestAuthorDeclarationRoundTripsPerBranch(t *testing.T) {
	r := newRepo(t)
	commitOnBranch(t, r, "feat/thing", "a.txt", "a\n")
	want := Declaration{Provider: "anthropic", Model: "claude-opus-5"}
	if err := r.SetAuthorDeclaration("feat/thing", want); err != nil {
		t.Fatalf("SetAuthorDeclaration: %v", err)
	}
	got, ok := r.AuthorDeclaration("feat/thing")
	if !ok {
		t.Fatal("declaration not found after writing it")
	}
	if got != want {
		t.Errorf("declaration = %+v, want %+v", got, want)
	}
	// It is per branch, not per repository: a push carries commits from
	// possibly several branches and several sessions.
	if _, ok := r.AuthorDeclaration("main"); ok {
		t.Error("the declaration leaked onto another branch")
	}
}

func TestCommitsListsWhatTheBranchAddsOverItsBase(t *testing.T) {
	r := newRepo(t)
	commitOnBranch(t, r, "feat/thing", "a.txt", "a\n")
	commitOnBranch(t, r, "", "b.txt", "b\n")
	commits, err := r.Commits("main", "feat/thing")
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2: %+v", len(commits), commits)
	}
	if commits[0].Subject != "feat: a.txt" {
		t.Errorf("first subject = %q; --reverse must put the oldest first", commits[0].Subject)
	}
}

func TestCommitsSurvivesABodyWithBlankLines(t *testing.T) {
	// A trailer-carrying message has blank lines in it, and a naive split would
	// read each paragraph as another commit.
	r := newRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = r.Root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("checkout", "-b", "feat/multiline")
	if err := os.WriteFile(filepath.Join(r.Root, "c.txt"), []byte("c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "feat: c", "-m", "first para\n\nsecond para")
	commits, err := r.Commits("main", "feat/multiline")
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("got %d commits, want 1: %+v", len(commits), commits)
	}
	if !strings.Contains(commits[0].Body, "second para") {
		t.Errorf("body lost: %q", commits[0].Body)
	}
}

// --- configuration ----------------------------------------------------------

func TestConfigGetAnswersAnUnsetKeyWithNoError(t *testing.T) {
	// `git config --get` exits 1 for a key nobody set, which arrived here as a
	// command failure. Both readers document absence as a normal answer, and
	// every caller asks about an optional setting, so an unset key that came
	// back as an error made the two disagree with their own documentation.
	r := newRepo(t)
	for _, read := range []struct {
		name string
		fn   func(string) (string, error)
	}{
		{"ConfigGet", r.ConfigGet},
		{"ConfigGetLocal", r.ConfigGetLocal},
	} {
		value, err := read.fn("mftest.nobodySetThis")
		if err != nil {
			t.Errorf("%s on an unset key returned %v; absence is documented as a normal answer", read.name, err)
		}
		if value != "" {
			t.Errorf("%s on an unset key returned %q", read.name, value)
		}
	}
}

func TestConfigGetStillReportsAFailureThatIsNotAbsence(t *testing.T) {
	// Only git's "no such key" status becomes an empty answer. A directory that
	// is not a repository has to keep failing, or the absence contract would
	// report every setting as unset wherever git cannot run at all.
	r := Open(filepath.Join(t.TempDir(), "not-a-repository"))
	if _, err := r.ConfigGet("core.hooksPath"); err == nil {
		t.Error("reading configuration where there is no repository must fail rather than report absence")
	}
}

func TestConfigGetReadsAValueThisRepositorySet(t *testing.T) {
	r := newRepo(t)
	if err := r.ConfigSetLocal("mftest.key", "value"); err != nil {
		t.Fatalf("ConfigSetLocal: %v", err)
	}
	for _, read := range []struct {
		name string
		fn   func(string) (string, error)
	}{
		{"ConfigGet", r.ConfigGet},
		{"ConfigGetLocal", r.ConfigGetLocal},
	} {
		value, err := read.fn("mftest.key")
		if err != nil || value != "value" {
			t.Errorf("%s = %q, %v; want the value that was written", read.name, value, err)
		}
	}
}

// --- history ----------------------------------------------------------------

func TestPathsEverAddedRemembersAPathTheTreeNoLongerHas(t *testing.T) {
	r := newRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = r.Root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	if err := os.MkdirAll(filepath.Join(r.Root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Root, "docs", "a.md"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "docs: a")
	run("rm", "-q", "docs/a.md")
	run("commit", "-m", "docs: remove a")

	paths, err := r.PathsEverAdded("docs")
	if err != nil {
		t.Fatalf("PathsEverAdded: %v", err)
	}
	if len(paths) != 1 || paths[0] != "docs/a.md" {
		t.Errorf("PathsEverAdded = %v, want the path history added and the tree lost", paths)
	}
}

func TestRenamedPathsSaysWhereAnAddedPathWent(t *testing.T) {
	// This is the companion PathsEverAdded needs to be read correctly: git
	// reports the renaming commit as R, so the new name never appears as an
	// addition and the old one looks deleted with nothing saying where it went.
	r := newRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = r.Root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	if err := os.MkdirAll(filepath.Join(r.Root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Root, "docs", "a.md"), []byte("a long enough body to match\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "docs: a")
	run("mv", "docs/a.md", "docs/b.md")
	run("commit", "-m", "docs: rename a to b")
	run("mv", "docs/b.md", "docs/c.md")
	run("commit", "-m", "docs: rename b to c")

	renames, err := r.RenamedPaths("docs")
	if err != nil {
		t.Fatalf("RenamedPaths: %v", err)
	}
	if renames["docs/a.md"] != "docs/b.md" || renames["docs/b.md"] != "docs/c.md" {
		t.Errorf("RenamedPaths = %v, want each step of the chain", renames)
	}
}
