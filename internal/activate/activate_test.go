package activate

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LukeSantossz/my-framework/internal/version"
)

// isolateGitConfig points this process's git at an empty global configuration
// and switches the system one off.
//
// Not tidiness: `git init` copies init.templateDir when the developer
// configures one, so a machine with a template carrying hooks fails the
// shadowing test for a reason that test does not test — and a machine with a
// global core.hooksPath decides the wiring assertions here instead of the code
// under test.
//
// A test staging its own global configuration calls withGlobalHooksPath after
// repo, so that its scratch file is the one in effect.
func isolateGitConfig(t *testing.T) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", file)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func repo(t *testing.T, withHooksDir bool) string {
	t.Helper()
	isolateGitConfig(t)
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
	run("config", "user.email", "t@example.invalid")
	run("config", "user.name", "T")
	if withHooksDir {
		if err := os.MkdirAll(filepath.Join(root, HooksDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, HooksDir, "pre-push"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// --- hooks ------------------------------------------------------------------

func TestHooksStatusReportsAnUnwiredRepository(t *testing.T) {
	state := HooksStatus(repo(t, true))
	if state.Canonical {
		t.Error("a repository nobody wired must not report canonical")
	}
	if !state.Present {
		t.Error("the versioned hooks directory exists and must be reported present")
	}
}

func TestInstallHooksPointsAtTheVersionedDirectory(t *testing.T) {
	root := repo(t, true)
	if err := InstallHooks(root); err != nil {
		t.Fatalf("InstallHooks: %v", err)
	}
	state := HooksStatus(root)
	if !state.Canonical {
		t.Errorf("hooks path = %q, want %q", state.Path, HooksDir)
	}
}

func TestInstallHooksIsIdempotent(t *testing.T) {
	root := repo(t, true)
	if err := InstallHooks(root); err != nil {
		t.Fatal(err)
	}
	if err := InstallHooks(root); err != nil {
		t.Fatalf("second InstallHooks: %v", err)
	}
	if !HooksStatus(root).Canonical {
		t.Error("a second install unwired the repository")
	}
}

func TestHooksStatusReportsAPathPointingSomewhereElse(t *testing.T) {
	// A repository wired to someone else's directory is not activated, and
	// reporting it as fine is how the Gap reopens quietly.
	root := repo(t, true)
	cmd := exec.Command("git", "config", "--local", "core.hooksPath", "some/other/dir")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}
	state := HooksStatus(root)
	if state.Canonical {
		t.Error("a divergent hooks path must not report canonical")
	}
	if state.Path != "some/other/dir" {
		t.Errorf("path = %q, want the divergent value reported back", state.Path)
	}
}

func TestInstallHooksRefusesWhenThereIsNoVersionedDirectory(t *testing.T) {
	if err := InstallHooks(repo(t, false)); err == nil {
		t.Fatal("wiring a hooks path at a directory that does not exist must fail")
	}
}

func TestUninstallHooksRemovesTheSettingOnly(t *testing.T) {
	root := repo(t, true)
	if err := InstallHooks(root); err != nil {
		t.Fatal(err)
	}
	if err := UninstallHooks(root); err != nil {
		t.Fatalf("UninstallHooks: %v", err)
	}
	if HooksStatus(root).Canonical {
		t.Error("the setting survived uninstall")
	}
	if _, err := os.Stat(filepath.Join(root, HooksDir)); err != nil {
		t.Error("uninstall deleted the versioned directory; it only owns the setting")
	}
}

// setHooksPath writes the repository's own core.hooksPath, the way another tool
// that owns the hooks directory would.
func setHooksPath(t *testing.T, root, value string) {
	t.Helper()
	cmd := exec.Command("git", "config", "--local", "core.hooksPath", value)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}
}

// withGlobalHooksPath points this process's git at a scratch global
// configuration carrying core.hooksPath, so a test can reproduce the state that
// makes every repository on a machine claim to be wired.
func withGlobalHooksPath(t *testing.T, value string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "gitconfig")
	body := "[core]\n\thooksPath = " + value + "\n"
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", file)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func TestInstallHooksRefusesToReplaceAPathItDoesNotOwn(t *testing.T) {
	// A repository using Husky has core.hooksPath pointing at .husky. Wiring
	// over it destroys that project's gate, and silently: git reports no error
	// for a value that was simply replaced.
	root := repo(t, true)
	setHooksPath(t, root, ".husky")
	err := InstallHooks(root)
	if err == nil {
		t.Fatal("wiring over a hooks path someone else set must fail")
	}
	if !strings.Contains(err.Error(), ".husky") {
		t.Errorf("error %q does not name the value it refused to replace", err)
	}
	if got := HooksStatus(root).Path; got != ".husky" {
		t.Errorf("hooks path = %q; the refusal did not leave the value alone", got)
	}
}

func TestUninstallHooksIsIdempotent(t *testing.T) {
	// A teardown script and a CI cleanup step both run this more than once, and
	// on machines where it was never installed. `git config --unset` exits 5
	// for an absent key, which used to surface as raw plumbing naming no remedy.
	root := repo(t, true)
	if err := InstallHooks(root); err != nil {
		t.Fatal(err)
	}
	if err := UninstallHooks(root); err != nil {
		t.Fatalf("first UninstallHooks: %v", err)
	}
	if err := UninstallHooks(root); err != nil {
		t.Fatalf("second UninstallHooks: %v", err)
	}
	if err := UninstallHooks(repo(t, true)); err != nil {
		t.Fatalf("UninstallHooks on a repository nothing ever wired: %v", err)
	}
}

func TestUninstallHooksLeavesAPathItDoesNotOwn(t *testing.T) {
	root := repo(t, true)
	setHooksPath(t, root, ".husky")
	err := UninstallHooks(root)
	if err == nil {
		t.Fatal("removing a hooks path mf never set must fail rather than destroy it")
	}
	if !strings.Contains(err.Error(), ".husky") {
		t.Errorf("error %q does not name the value it refused to remove", err)
	}
	if got := HooksStatus(root).Path; got != ".husky" {
		t.Errorf("hooks path = %q; uninstall destroyed a value it did not set", got)
	}
}

func TestHooksStatusSeparatesThisRepositorysSettingFromAnInheritedOne(t *testing.T) {
	// A global core.hooksPath applies to every repository on the machine. It is
	// still a real setting, so it is reported — but reporting it as this
	// repository's own is what made every clone claim to be activated.
	root := repo(t, false)
	withGlobalHooksPath(t, HooksDir)
	state := HooksStatus(root)
	if state.Path != HooksDir {
		t.Errorf("path = %q, want the inherited value reported", state.Path)
	}
	if state.Local {
		t.Error("a global setting was reported as this repository's own")
	}
	if state.Present {
		t.Error("there is no versioned directory here; present must be false")
	}
}

func TestInitIsNotFooledByAGlobalHooksPath(t *testing.T) {
	root := repo(t, true)
	withGlobalHooksPath(t, HooksDir)
	steps, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !HooksStatus(root).Local {
		t.Errorf("init declined to wire this repository, believing a global setting had done it: %+v", steps)
	}
}

func TestInitLeavesAHooksPathSomeoneElseSet(t *testing.T) {
	root := repo(t, true)
	setHooksPath(t, root, ".husky")
	steps, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := HooksStatus(root).Path; got != ".husky" {
		t.Errorf("hooks path = %q; init overwrote another tool's wiring", got)
	}
	hooks := stepNamed(t, steps, "hooks")
	if hooks.Changed {
		t.Error("init reported a change it must not have made")
	}
	if !strings.Contains(hooks.Message, ".husky") {
		t.Errorf("message %q does not name what it left alone", hooks.Message)
	}
}

func TestInitSaysWhenItTakesOverAHooksPathSetOutsideTheRepository(t *testing.T) {
	// A global core.hooksPath is not this repository's decision, so wiring here
	// is right — but git honours one hooks path, and the tool that set the
	// global one loses its gate in this repository the moment mf writes a local
	// value. That happened, and the step reported only the new path.
	root := repo(t, true)
	withGlobalHooksPath(t, ".husky")
	steps, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !HooksStatus(root).Canonical {
		t.Fatalf("init left this repository unwired: %+v", steps)
	}
	hooks := stepNamed(t, steps, "hooks")
	if !hooks.Changed {
		t.Error("init wired the repository and reported no change")
	}
	if !strings.Contains(hooks.Message, ".husky") {
		t.Errorf("message %q does not name the path that no longer runs here", hooks.Message)
	}
}

func TestShadowedLocalHooksNamesWhatCoreHooksPathSilences(t *testing.T) {
	// Setting core.hooksPath at all makes git ignore .git/hooks entirely, so a
	// hook a person installed there stops firing. Nothing said so anywhere.
	root := repo(t, true)
	dir := filepath.Join(root, ".git", "hooks")
	if err := os.WriteFile(filepath.Join(dir, "pre-commit"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := ShadowedLocalHooks(root)
	if len(got) != 1 || got[0] != "pre-commit" {
		t.Errorf("ShadowedLocalHooks = %v, want just the enabled hook (the .sample files never ran)", got)
	}
}

func stepNamed(t *testing.T, steps []Step, name string) Step {
	t.Helper()
	for _, s := range steps {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no %q step in %+v", name, steps)
	return Step{}
}

// --- lock -------------------------------------------------------------------

func TestLockRoundTrips(t *testing.T) {
	root := repo(t, true)
	if _, ok := ReadLock(root); ok {
		t.Fatal("a repository with no lock must report none")
	}
	if _, err := WriteLock(root, "1.2.3"); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}
	lock, ok := ReadLock(root)
	if !ok || lock.FrameworkVersion != "1.2.3" {
		t.Errorf("lock = %+v, ok = %v", lock, ok)
	}
}

// --- init -------------------------------------------------------------------

func TestInitWritesTheProjectFileTheHooksPathAndTheLock(t *testing.T) {
	root := repo(t, true)
	steps, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	for _, name := range []string{"project file", "standards", "agent source", "hook files", "hooks", "lock"} {
		if !stepNamed(t, steps, name).Changed {
			t.Errorf("step %q reported no change on a fresh repository", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".framework.toml")); err != nil {
		t.Error("no project file written")
	}
	if !HooksStatus(root).Canonical {
		t.Error("hooks not wired")
	}
	if lock, ok := ReadLock(root); !ok || lock.FrameworkVersion != "1.2.3" {
		t.Error("lock not recorded")
	}
}

func TestInitIsIdempotent(t *testing.T) {
	root := repo(t, true)
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	steps, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}
	for _, s := range steps {
		if s.Changed {
			t.Errorf("step %q changed something on a second run: %s", s.Name, s.Message)
		}
	}
}

func TestInitNeverOverwritesAProjectFileSomeoneWrote(t *testing.T) {
	// Scaffolding over a person's policy is data loss disguised as convenience.
	root := repo(t, true)
	mine := "version = 1\n# mine\n"
	if err := os.WriteFile(filepath.Join(root, ".framework.toml"), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".framework.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != mine {
		t.Errorf("init overwrote an existing project file:\n%s", got)
	}
}

func TestInitScaffoldIsValidConfiguration(t *testing.T) {
	// A scaffold that will not load is worse than none: the first thing a new
	// adopter runs would fail on the file the tool just wrote.
	root := repo(t, true)
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".framework.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "version = 1") {
		t.Error("the scaffold carries no schema version")
	}
	// Assignments, not prose: the scaffold's comments name these keys precisely
	// in order to forbid them, and a test that cannot tell the rule from a
	// violation of it would force the explanation out of the file.
	for _, forbidden := range []string{"api_key =", "api_key_env =", "endpoint ="} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("the scaffold assigns machine state in a committed file: %s", forbidden)
		}
	}
}

func TestInitGivesARepositoryWithNoHooksTheOnesThisBuildCarries(t *testing.T) {
	// A repository with no .githooks had nothing to wire, so `mf init` reported
	// success and left the push ungated. The hooks ship in the binary for the
	// same reason the standards do: there is nowhere else an adopter can get
	// them from.
	root := repo(t, false)
	steps, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !stepNamed(t, steps, "hook files").Changed {
		t.Error("init wrote no hooks into a repository that had none")
	}
	if _, err := os.Stat(filepath.Join(root, HooksDir, "pre-push")); err != nil {
		t.Errorf("no pre-push hook written: %v", err)
	}
	if !stepNamed(t, steps, "hooks").Changed || !HooksStatus(root).Canonical {
		t.Error("init wrote the hooks and then did not wire them")
	}
}

func TestInitLeavesTheShippedHooksExecutableOnEveryRun(t *testing.T) {
	// The executable bit was set only on the run that wrote the files, so a
	// hook that lost it — a checkout that does not carry modes, a copy through
	// a zip — stayed unrunnable through every later activation. git then fails
	// the push with an exec error rather than reporting a missing gate, which
	// is the harder failure to read of the two.
	if runtime.GOOS == "windows" {
		t.Skip("the filesystem carries no executable bit here")
	}
	root := repo(t, false)
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(root, HooksDir, "pre-push")
	if err := os.Chmod(hook, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(hook)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("pre-push mode is %v; git runs a hook by executing it", info.Mode().Perm())
	}
}

func TestInitLeavesTheModeOfAFileItDoesNotShipAlone(t *testing.T) {
	// The versioned directory is the repository's, not this framework's: an
	// adopter keeps their own scripts and notes beside the shipped hooks, and a
	// blanket chmod over the directory would decide the mode of every one.
	if runtime.GOOS == "windows" {
		t.Skip("the filesystem carries no executable bit here")
	}
	root := repo(t, true)
	theirs := filepath.Join(root, HooksDir, "README.md")
	if err := os.WriteFile(theirs, []byte("# how our hooks work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(theirs)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 != 0 {
		t.Errorf("a file this build does not ship was made executable: %v", info.Mode().Perm())
	}
}

func TestInitNeverOverwritesAHookSomeoneWrote(t *testing.T) {
	root := repo(t, true)
	mine := "#!/bin/sh\n# ours\n"
	if err := os.WriteFile(filepath.Join(root, HooksDir, "pre-push"), []byte(mine), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, HooksDir, "pre-push"))
	if err != nil || string(got) != mine {
		t.Errorf("init replaced a hook the repository already had: %q", got)
	}
}

func TestPinModelsRecordsWhatTheConfigurationResolvedTo(t *testing.T) {
	root := repo(t, true)
	if _, err := WriteLock(root, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	lock, err := PinModels(root, map[string]string{"codex": "gpt-5.6-terra", "empty": ""}, "2026-08-24")
	if err != nil {
		t.Fatalf("PinModels: %v", err)
	}
	if lock.Models["codex"].Model != "gpt-5.6-terra" {
		t.Errorf("pin = %+v", lock.Models["codex"])
	}
	if lock.Models["codex"].PinnedOn != "2026-08-24" {
		t.Error("a pin with no date cannot be compared against anything later")
	}
	if _, present := lock.Models["empty"]; present {
		t.Error("a backend with no resolved model was pinned to nothing")
	}
	// The adopted version must survive a write that only touches models.
	reread, ok := ReadLock(root)
	if !ok || reread.FrameworkVersion != "1.2.3" {
		t.Errorf("writing a pin dropped the adopted version: %+v", reread)
	}
	if reread.Models["codex"].Model != "gpt-5.6-terra" {
		t.Errorf("the pin did not survive the round trip: %+v", reread.Models)
	}
}

func TestWriteLockPreservesPinsAlreadyRecorded(t *testing.T) {
	root := repo(t, true)
	if _, err := PinModels(root, map[string]string{"codex": "m1"}, "2026-08-24"); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteLock(root, "2.0.0"); err != nil {
		t.Fatal(err)
	}
	lock, _ := ReadLock(root)
	if lock.Models["codex"].Model != "m1" {
		t.Error("recording a version dropped the pins; a command that writes one field must not drop the others")
	}
}

func TestWriteLockDoesNotLetAnUnreleasedBuildDowngradeARecordedAdoption(t *testing.T) {
	// A repository that adopted v0.4.0 records that. Building `mf` from source
	// and running `mf init` used to rewrite it to the unreleased default, which
	// is the same silent overwrite this package exists to refuse everywhere else.
	root := repo(t, true)
	if _, err := WriteLock(root, "v0.4.0"); err != nil {
		t.Fatal(err)
	}
	recorded, err := WriteLock(root, version.Dev)
	if err != nil {
		t.Fatalf("WriteLock: %v", err)
	}
	if recorded {
		t.Error("an unreleased build claimed to have recorded an adoption")
	}
	if lock, _ := ReadLock(root); lock.FrameworkVersion != "v0.4.0" {
		t.Errorf("framework_version = %q; a source build overwrote a released adoption", lock.FrameworkVersion)
	}
}

func TestWriteLockRecordsAReleasedVersionOverAnother(t *testing.T) {
	// Upgrading is the whole point of the record, so a build that has an
	// identity of its own still writes it.
	root := repo(t, true)
	if _, err := WriteLock(root, "v0.4.0"); err != nil {
		t.Fatal(err)
	}
	recorded, err := WriteLock(root, "v0.5.0")
	if err != nil || !recorded {
		t.Fatalf("WriteLock = %v, %v", recorded, err)
	}
	if lock, _ := ReadLock(root); lock.FrameworkVersion != "v0.5.0" {
		t.Errorf("framework_version = %q, want the new adoption", lock.FrameworkVersion)
	}
}

func TestComparePinsReportsAPinNothingCanCheckAnyMore(t *testing.T) {
	// A pin for a backend no configuration layer resolves cannot be compared
	// against anything. Skipping it silently made `mf doctor` count it among
	// the pins "all matching the configuration" — a pin that is unchecked
	// reported as a pin that checks out.
	lock := Lock{Models: map[string]PinnedModel{"gone": {Model: "x", PinnedOn: "2026-08-24"}}}
	drift := ComparePins(lock, map[string]string{})
	if len(drift) != 1 {
		t.Fatalf("drift = %+v, want the unresolvable pin reported", drift)
	}
	if drift[0].Configured != NotConfigured {
		t.Errorf("configured = %q, want %q", drift[0].Configured, NotConfigured)
	}
}

func TestComparePinsReportsOnlyRealDisagreement(t *testing.T) {
	lock := Lock{Models: map[string]PinnedModel{
		"codex":  {Model: "gpt-5.6-terra", PinnedOn: "2026-08-24"},
		"gemini": {Model: "g-1", PinnedOn: "2026-08-24"},
	}}
	drift := ComparePins(lock, map[string]string{"codex": "gpt-5.6-terra", "gemini": "g-2"})
	if len(drift) != 1 {
		t.Fatalf("drift = %+v, want exactly the changed one", drift)
	}
	if drift[0].Backend != "gemini" || drift[0].Pinned != "g-1" || drift[0].Configured != "g-2" {
		t.Errorf("drift = %+v", drift[0])
	}
}

// --- scaffold ---------------------------------------------------------------

func TestInitMaterialisesTheStandardsThisBuildCarries(t *testing.T) {
	// `mf init` reported success while leaving an adopter with no standards at
	// all: the binary embedded them only so `mf upgrade` could compare, and
	// nothing ever installed one. Every gate then read an empty tree.
	root := repo(t, true)
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "standards", "INDEX.md")); err != nil {
		t.Errorf("no standards materialised: %v", err)
	}
}

func TestInitHonoursAConfiguredStandardsDirectory(t *testing.T) {
	root := repo(t, true)
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3", StandardsDir: "policy/std"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "policy", "std", "INDEX.md")); err != nil {
		t.Errorf("the configured directory was not used: %v", err)
	}
}

func TestInitWritesNoStandardsIntoASubmodule(t *testing.T) {
	// The one known downstream consumer vendors this framework as a `.standards`
	// submodule. Copies written there are untracked files inside somebody else's
	// checkout, and they land precisely when the submodule has not been
	// initialised — the state in which the directory looks empty.
	root := repo(t, true)
	gitmodules := "[submodule \".standards\"]\n\tpath = .standards\n\turl = https://example.invalid/mf.git\n"
	if err := os.WriteFile(filepath.Join(root, ".gitmodules"), []byte(gitmodules), 0o644); err != nil {
		t.Fatal(err)
	}
	steps, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3", StandardsDir: ".standards/docs/standards"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".standards")); err == nil {
		t.Error("init wrote into the submodule that supplies the standards")
	}
	standards := stepNamed(t, steps, "standards")
	if standards.Changed || !strings.Contains(standards.Message, "submodule") {
		t.Errorf("step %+v does not say why nothing was written", standards)
	}
}

func TestInitNeverOverwritesAStandardSomeoneEdited(t *testing.T) {
	// Adopting these documents means editing them. Rewriting one on the next
	// `mf init` is the data loss `mf upgrade` refuses to commit for the same
	// reason.
	root := repo(t, true)
	dir := filepath.Join(root, "docs", "standards")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mine := "# our own index\n"
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil || string(got) != mine {
		t.Errorf("init overwrote an edited standard: %q", got)
	}
}

func TestInitWritesTheSourceTheInstructionFilesAreGeneratedFrom(t *testing.T) {
	// Without it `mf agents sync` has nothing to read, so the [agents.*] the
	// scaffold declares would make `mf check agents` fail on a fresh adoption
	// instead of gating anything.
	root := repo(t, true)
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(AgentSourcePath)))
	if err != nil {
		t.Fatalf("no agent instruction source: %v", err)
	}
	if !strings.Contains(string(body), "mf:role") {
		t.Error("the source carries no role markers, so nothing can be assigned to a vendor")
	}
}

func TestInitScaffoldTellsAnAdopterHowToReachAProvider(t *testing.T) {
	// The scaffold said "only a machine defines how to reach them" and then
	// left the reader with no way to find out how.
	root := repo(t, true)
	if _, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".framework.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--machine", "[paths]", "[agents.", "endpoint"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the scaffold never mentions %q:\n%s", want, body)
		}
	}
}
