package upgrade

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	framework "github.com/LukeSantossz/my-framework"
	"github.com/LukeSantossz/my-framework/internal/activate"
)

// seed writes the standards this build shipped with into dir, so a test starts
// from a tree that matches and then introduces exactly one difference. The
// content comes from the embed rather than from literals here, because a
// literal copy of a standard is a second source that drifts from the first and
// would make these tests agree with themselves while the code disagrees with
// the build.
func seed(t *testing.T, dir string, transform func(string) string) []string {
	t.Helper()
	var names []string
	err := fs.WalkDir(framework.Standards, framework.StandardsPrefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, readErr := framework.Standards.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel := strings.TrimPrefix(path, framework.StandardsPrefix+"/")
		out := filepath.Join(dir, filepath.FromSlash(rel))
		if mkErr := os.MkdirAll(filepath.Dir(out), 0o755); mkErr != nil {
			return mkErr
		}
		text := string(body)
		if transform != nil {
			text = transform(text)
		}
		names = append(names, rel)
		return os.WriteFile(out, []byte(text), 0o644)
	})
	if err != nil {
		t.Fatalf("seeding the embedded standards: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("the build embeds no standards, so nothing here proves anything")
	}
	return names
}

func statuses(rep Report) map[string]string {
	out := map[string]string{}
	for _, c := range rep.Changes {
		out[c.File] = c.Status
	}
	return out
}

func TestATreeThatMatchesTheBuildReportsNoChange(t *testing.T) {
	root := t.TempDir()
	seed(t, filepath.Join(root, "docs", "standards"), nil)

	rep, err := Compare(root, "", "0.0.0-dev")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(rep.Changes) != 0 {
		t.Fatalf("a tree copied from the build was reported as changed: %+v", rep.Changes)
	}
	if !strings.Contains(rep.Summary(), "match this build") {
		t.Errorf("summary %q does not say the standards match", rep.Summary())
	}
}

func TestStandardsThatAreAbsentAreReportedAsMissingRatherThanAsDiffering(t *testing.T) {
	// `mf doctor` prints the summary line and nothing else, so a repository with
	// no standards at all being told its files "differ" hides every real
	// difference behind phantom ones.
	root := t.TempDir()

	rep, err := Compare(root, "", "0.0.0-dev")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(rep.Changes) == 0 {
		t.Fatal("a repository with no standards must report every embedded file")
	}
	for _, c := range rep.Changes {
		if c.Status != StatusMissing {
			t.Errorf("%s reported as %q, want %q", c.File, c.Status, StatusMissing)
		}
	}
	summary := rep.Summary()
	if !strings.Contains(summary, "missing") {
		t.Errorf("summary %q does not say the standards are missing", summary)
	}
	if strings.Contains(summary, "differ") {
		t.Errorf("summary %q calls absent standards differing", summary)
	}
}

func TestSummaryCountsMissingAndDifferingSeparately(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "standards")
	names := seed(t, dir, nil)
	if len(names) < 2 {
		t.Skip("the build embeds fewer than two standards")
	}
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(names[0]))); err != nil {
		t.Fatal(err)
	}
	edited := filepath.Join(dir, filepath.FromSlash(names[1]))
	if err := os.WriteFile(edited, []byte("# rewritten locally\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Compare(root, "", "0.0.0-dev")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	got := statuses(rep)
	if got[names[0]] != StatusMissing {
		t.Errorf("%s reported as %q, want %q", names[0], got[names[0]], StatusMissing)
	}
	if got[names[1]] != StatusDiffers {
		t.Errorf("%s reported as %q, want %q", names[1], got[names[1]], StatusDiffers)
	}
	summary := rep.Summary()
	if !strings.Contains(summary, "1 missing") || !strings.Contains(summary, "1 differing") {
		t.Errorf("summary %q does not count the two cases separately", summary)
	}
}

func TestAWindowsCheckoutMatchesTheBuildItWasCopiedFrom(t *testing.T) {
	// git hands a Windows checkout CRLF by default, and a byte comparison would
	// report every standard as edited on the machine that edited none of them.
	root := t.TempDir()
	// The embed itself carries whatever the checkout that built it had, which on
	// this platform is already CRLF, so the copy is flattened before being
	// rewritten — otherwise the fixture writes CR CR LF and tests nothing.
	seed(t, filepath.Join(root, "docs", "standards"), func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
	})

	rep, err := Compare(root, "", "0.0.0-dev")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(rep.Changes) != 0 {
		t.Errorf("a CRLF checkout was reported as diverging: %+v", rep.Changes)
	}
}

func TestStandardsAreComparedWhereTheRepositoryKeepsThem(t *testing.T) {
	// The one downstream consumer of this framework vendors it as a `.standards`
	// submodule, so its documents are nowhere near `docs/standards`. A
	// comparison that can only look there reports that adopter's whole corpus as
	// missing.
	root := t.TempDir()
	vendored := filepath.FromSlash(".standards/docs/standards")
	seed(t, filepath.Join(root, vendored), nil)

	rep, err := Compare(root, vendored, "0.0.0-dev")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(rep.Changes) != 0 {
		t.Errorf("the vendored tree was not compared where it lives: %+v", rep.Changes)
	}

	// And the default is still the default: pointing nowhere finds nothing.
	rep, err = Compare(root, "", "0.0.0-dev")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(rep.Changes) == 0 {
		t.Error("the configured directory was read even though none was configured")
	}
}

func TestAStandardInASubdirectoryIsNotInvisible(t *testing.T) {
	// Reading the local tree one level deep while flattening the embedded one
	// made a nested document compare as though it were not there at all, which
	// is the one answer a comparison must never give.
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "standards")
	seed(t, dir, nil)
	nested := filepath.Join(dir, "appendix")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "notes.md"), []byte("# local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Compare(root, "", "0.0.0-dev")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if got := statuses(rep)["appendix/notes.md"]; got != StatusLocalOnly {
		t.Errorf("a nested local standard was reported as %q, want %q: %+v", got, StatusLocalOnly, rep.Changes)
	}
}

func TestAnAdoptedVersionThatIsNotTheRunningOneIsReportedAsAMismatch(t *testing.T) {
	// Printing the two versions side by side leaves the reader to notice they
	// disagree. Whether they disagree is the question the two facts exist to
	// answer.
	root := t.TempDir()
	seed(t, filepath.Join(root, "docs", "standards"), nil)
	if _, err := activate.WriteLock(root, "v0.1.0"); err != nil {
		t.Fatal(err)
	}

	rep, err := Compare(root, "", "0.0.0-dev")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if !rep.VersionMismatch() {
		t.Fatalf("a lock recording %q against a %q build is not a mismatch", rep.LockedVersion, rep.RunningVersion)
	}
	summary := rep.Summary()
	if !strings.Contains(summary, "v0.1.0") || !strings.Contains(summary, "0.0.0-dev") {
		t.Errorf("summary %q does not name both versions", summary)
	}
}

func TestAMatchingLockIsNotAMismatch(t *testing.T) {
	root := t.TempDir()
	seed(t, filepath.Join(root, "docs", "standards"), nil)
	if _, err := activate.WriteLock(root, "0.0.0-dev"); err != nil {
		t.Fatal(err)
	}

	rep, err := Compare(root, "", "0.0.0-dev")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if rep.VersionMismatch() {
		t.Error("a lock recording the running version was reported as a mismatch")
	}
	if strings.Contains(rep.Summary(), "adopted") {
		t.Errorf("summary %q raises a version that agrees with the build", rep.Summary())
	}
}

func TestARepositoryWithNoLockSaysSoRatherThanClaimingAMismatch(t *testing.T) {
	root := t.TempDir()
	seed(t, filepath.Join(root, "docs", "standards"), nil)

	rep, err := Compare(root, "", "0.0.0-dev")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if rep.Note == "" || !strings.Contains(rep.Note, activate.LockFileName) {
		t.Errorf("note %q does not report the absent lock", rep.Note)
	}
	if rep.VersionMismatch() {
		t.Error("an unrecorded adoption was reported as disagreeing with the build")
	}
}
