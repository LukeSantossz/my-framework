package activate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repo(t *testing.T, withHooksDir bool) string {
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

// --- lock -------------------------------------------------------------------

func TestLockRoundTrips(t *testing.T) {
	root := repo(t, true)
	if _, ok := ReadLock(root); ok {
		t.Fatal("a repository with no lock must report none")
	}
	if err := WriteLock(root, "1.2.3"); err != nil {
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
	if len(steps) != 3 {
		t.Fatalf("got %d steps, want 3: %+v", len(steps), steps)
	}
	for _, s := range steps {
		if !s.Changed {
			t.Errorf("step %q reported no change on a fresh repository", s.Name)
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

func TestInitRecordsWhenThereIsNoHooksDirectoryRatherThanFailing(t *testing.T) {
	root := repo(t, false)
	steps, err := Init(InitOptions{RepoRoot: root, FrameworkVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	var hooks Step
	for _, s := range steps {
		if s.Name == "hooks" {
			hooks = s
		}
	}
	if hooks.Changed {
		t.Error("init wired a hooks path with no directory to point at")
	}
	if !strings.Contains(hooks.Message, "nothing to wire") {
		t.Errorf("message %q does not say why", hooks.Message)
	}
}

func TestPinModelsRecordsWhatTheConfigurationResolvedTo(t *testing.T) {
	root := repo(t, true)
	if err := WriteLock(root, "1.2.3"); err != nil {
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
	if err := WriteLock(root, "2.0.0"); err != nil {
		t.Fatal(err)
	}
	lock, _ := ReadLock(root)
	if lock.Models["codex"].Model != "m1" {
		t.Error("recording a version dropped the pins; a command that writes one field must not drop the others")
	}
}

func TestComparePinsReportsOnlyRealDisagreement(t *testing.T) {
	lock := Lock{Models: map[string]PinnedModel{
		"codex":  {Model: "gpt-5.6-terra", PinnedOn: "2026-08-24"},
		"gemini": {Model: "g-1", PinnedOn: "2026-08-24"},
		"gone":   {Model: "x", PinnedOn: "2026-08-24"},
	}}
	drift := ComparePins(lock, map[string]string{"codex": "gpt-5.6-terra", "gemini": "g-2"})
	if len(drift) != 1 {
		t.Fatalf("drift = %+v, want exactly the changed one", drift)
	}
	if drift[0].Backend != "gemini" || drift[0].Pinned != "g-1" || drift[0].Configured != "g-2" {
		t.Errorf("drift = %+v", drift[0])
	}
}
