package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDoctorReportsAndChangesNothing(t *testing.T) {
	root := gitRepo(t, chainProject)
	before := HooksStatusPathForTest(root)
	e, out, _ := reviewEnv(t, root, "doctor")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if after := HooksStatusPathForTest(root); after != before {
		t.Errorf("doctor changed the hooks path from %q to %q; a diagnostic that repairs hides the drift", before, after)
	}
	got := out.String()
	for _, want := range []string{"activation", "roles", "cross-provider", "credentials", "standards"} {
		if !strings.Contains(got, want) {
			t.Errorf("doctor output lacks the %q section:\n%s", want, got)
		}
	}
}

func TestDoctorNamesTheBackendAndModelEachRoleResolvesTo(t *testing.T) {
	root := gitRepo(t, chainProject)
	e, out, _ := reviewEnv(t, root, "doctor")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := out.String()
	for _, want := range []string{"codex", "fallback", "kind=cli", "provider=openai"} {
		if !strings.Contains(got, want) {
			t.Errorf("doctor output lacks %q:\n%s", want, got)
		}
	}
}

func TestDoctorSaysTheAuthorIsUndeclared(t *testing.T) {
	root := gitRepo(t, chainProject)
	branchWithChange(t, root, "feat/x")
	e, out, _ := reviewEnv(t, root, "doctor")
	Run(e)
	if !strings.Contains(out.String(), "not declared") {
		t.Errorf("doctor did not report the missing declaration:\n%s", out.String())
	}
}

func TestDoctorReportsAConfiguredKeyVariableThatIsUnset(t *testing.T) {
	root := gitRepo(t, "version = 1\n")
	machine := filepath.Join(t.TempDir(), "config.toml")
	body := "version = 1\n\n[providers.deepseek]\nendpoint = \"https://x/v1\"\napi_key_env = \"DEEPSEEK_API_KEY\"\n"
	if err := os.WriteFile(machine, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	e, out, _ := reviewEnv(t, root, "doctor")
	e.MachinePath = machine
	Run(e)
	if !strings.Contains(out.String(), "is unset") {
		t.Errorf("doctor did not report the unset key variable:\n%s", out.String())
	}
}

func TestInitThenDoctorReportsTheAdoptedVersion(t *testing.T) {
	root := gitRepo(t, "")
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	e, out, _ := reviewEnv(t, root, "init")
	if code := Run(e); code != 0 {
		t.Fatalf("init exit %d: %s", code, out.String())
	}
	e2, out2, _ := reviewEnv(t, root, "doctor")
	Run(e2)
	if strings.Contains(out2.String(), "no .framework.lock") {
		t.Errorf("doctor still reports no lock after init:\n%s", out2.String())
	}
}

func TestAuthorDeclareRecordsWhatDoctorThenReports(t *testing.T) {
	root := gitRepo(t, chainProject)
	branchWithChange(t, root, "feat/x")
	e, _, errOut := reviewEnv(t, root, "author", "declare", "--provider", "anthropic", "--model", "claude-opus-5")
	if code := Run(e); code != 0 {
		t.Fatalf("author declare exit %d: %s", code, errOut.String())
	}
	e2, out2, _ := reviewEnv(t, root, "doctor")
	Run(e2)
	if !strings.Contains(out2.String(), "anthropic") {
		t.Errorf("doctor does not report the declaration:\n%s", out2.String())
	}
}

func TestAuthorDeclareRequiresAProvider(t *testing.T) {
	root := gitRepo(t, chainProject)
	e, _, errOut := reviewEnv(t, root, "author", "declare", "--model", "x")
	if code := Run(e); code == 0 {
		t.Error("exit 0 with no provider; the provider is the claim R2 is checked against")
	}
	if !strings.Contains(errOut.String(), "--provider") {
		t.Errorf("stderr %q does not say what is missing", errOut.String())
	}
}

func TestHooksInstallAndStatus(t *testing.T) {
	root := gitRepo(t, "")
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	e, _, errOut := reviewEnv(t, root, "hooks", "install")
	if code := Run(e); code != 0 {
		t.Fatalf("hooks install exit %d: %s", code, errOut.String())
	}
	e2, out2, _ := reviewEnv(t, root, "hooks", "status")
	Run(e2)
	if !strings.Contains(out2.String(), "canonical:  true") {
		t.Errorf("status does not report the wiring:\n%s", out2.String())
	}
}

func TestUpgradeReportsDifferencesAndAppliesNothing(t *testing.T) {
	root := gitRepo(t, "")
	standards := filepath.Join(root, "docs", "standards")
	if err := os.MkdirAll(standards, 0o755); err != nil {
		t.Fatal(err)
	}
	// A local file the build does not carry, and a build file edited locally.
	if err := os.WriteFile(filepath.Join(standards, "local_only.md"), []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, out, _ := reviewEnv(t, root, "upgrade")
	if code := Run(e); code != 0 {
		t.Fatalf("upgrade exit %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "local_only.md") {
		t.Errorf("upgrade did not report the local-only file:\n%s", got)
	}
	if !strings.Contains(got, "Nothing was applied") {
		t.Errorf("upgrade did not state that it applied nothing:\n%s", got)
	}
	// It must not have written anything into the tree.
	if _, err := os.Stat(filepath.Join(standards, "INDEX.md")); err == nil {
		t.Error("upgrade wrote standards into the repository; it only reports")
	}
}

func TestAgentsSyncThenCheckPasses(t *testing.T) {
	project := "version = 1\n\n[agents.claude]\nfile = \"CLAUDE.md\"\nroles = [\"shared\"]\n"
	root := gitRepo(t, project)
	if err := os.MkdirAll(filepath.Join(root, "docs", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "# Instructions\n\nPreamble.\n\n<!-- mf:role shared -->\n## Binding\n\nRead the standards.\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "agents", "instructions.md"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	e, _, errOut := reviewEnv(t, root, "agents", "check")
	if code := Run(e); code == 0 {
		t.Errorf("check passed before any sync: %s", errOut.String())
	}

	e2, _, errOut2 := reviewEnv(t, root, "agents", "sync")
	if code := Run(e2); code != 0 {
		t.Fatalf("sync exit %d: %s", code, errOut2.String())
	}

	e3, out3, _ := reviewEnv(t, root, "agents", "check")
	if code := Run(e3); code != 0 {
		t.Errorf("check failed right after sync: %s", out3.String())
	}
}

func TestCheckFailsOnInstructionFileDrift(t *testing.T) {
	// The generated files are only a single source if editing the output is
	// caught; otherwise they are the old duplication with extra steps.
	project := "version = 1\n\n[agents.claude]\nfile = \"CLAUDE.md\"\nroles = [\"shared\"]\n"
	root := gitRepo(t, project)
	if err := os.MkdirAll(filepath.Join(root, "docs", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "# Instructions\n\n<!-- mf:role shared -->\n## Binding\n\nRead the standards.\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "agents", "instructions.md"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	e, _, _ := reviewEnv(t, root, "agents", "sync")
	Run(e)
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# hand edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e2, out2, _ := reviewEnv(t, root, "check", "agents")
	if code := Run(e2); code == 0 {
		t.Errorf("mf check passed with a drifted instruction file:\n%s", out2.String())
	}
	if !strings.Contains(out2.String(), "mf agents sync") {
		t.Errorf("output does not say how to fix it:\n%s", out2.String())
	}
}

func TestModelsPinRecordsTheDateAndDoctorReportsDrift(t *testing.T) {
	project := "version = 1\n\n[backends.codex]\nkind = \"cli\"\nprovider = \"openai\"\ncommand = \"codex\"\nmodel = \"m1\"\n"
	root := gitRepo(t, project)

	e, out, _ := reviewEnv(t, root, "models", "pin")
	e.Now = func() time.Time { return time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC) }
	if code := Run(e); code != 0 {
		t.Fatalf("pin exit %d", code)
	}
	if !strings.Contains(out.String(), "2026-08-24") {
		t.Errorf("a pin with no date cannot be compared to anything later:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "unverified") {
		t.Errorf("pinning must not imply a vendor confirmed the id:\n%s", out.String())
	}

	// Change the configured model; the pin must now read as drift.
	if err := os.WriteFile(filepath.Join(root, ".framework.toml"),
		[]byte(strings.Replace(project, "m1", "m2", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	e2, out2, _ := reviewEnv(t, root, "models", "list")
	if code := Run(e2); code != 0 {
		t.Fatalf("list exit %d", code)
	}
	if !strings.Contains(out2.String(), "DRIFT") {
		t.Errorf("a changed model id did not report as drift:\n%s", out2.String())
	}

	e3, out3, _ := reviewEnv(t, root, "doctor")
	Run(e3)
	if !strings.Contains(out3.String(), "m1") || !strings.Contains(out3.String(), "m2") {
		t.Errorf("doctor does not report the pin against the configuration:\n%s", out3.String())
	}
}

func TestUsageShowsAndResets(t *testing.T) {
	root := gitRepo(t, "version = 1\n")
	e, out, _ := reviewEnv(t, root, "usage")
	if code := Run(e); code != 0 {
		t.Fatalf("usage exit %d", code)
	}
	if !strings.Contains(out.String(), "runs:  0") {
		t.Errorf("a fresh total should report no runs:\n%s", out.String())
	}
	e2, out2, _ := reviewEnv(t, root, "usage", "reset")
	if code := Run(e2); code != 0 {
		t.Fatalf("reset exit %d", code)
	}
	if !strings.Contains(out2.String(), "reset") {
		t.Errorf("reset said nothing:\n%s", out2.String())
	}
}

func TestUsageRejectsAnUnknownAction(t *testing.T) {
	root := gitRepo(t, "version = 1\n")
	e, _, errOut := reviewEnv(t, root, "usage", "spend")
	if code := Run(e); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "spend") {
		t.Errorf("stderr %q does not name the bad action", errOut.String())
	}
}

// withIsolatedGitConfig points this process's git at an empty global
// configuration and switches the system one off, so a hooks assertion is
// decided by the code under test rather than by whatever the developer running
// it has configured for themselves.
func withIsolatedGitConfig(t *testing.T) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", file)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// withGlobalHooksPath points this process's git at a scratch global
// configuration carrying core.hooksPath.
func withGlobalHooksPath(t *testing.T, value string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(file, []byte("[core]\n\thooksPath = "+value+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", file)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func TestDoctorReportsHooksAsUnwiredWhenOnlyAGlobalHooksPathIsSet(t *testing.T) {
	// A global core.hooksPath = .githooks applies to every repository on the
	// machine. In one that has no .githooks directory there is no hook to run,
	// and doctor reported it as wired because it read the setting before it read
	// the repository.
	withGlobalHooksPath(t, ".githooks")
	root := gitRepo(t, "version = 1\n")
	e, out, _ := reviewEnv(t, root, "doctor")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := out.String()
	if strings.Contains(got, "wired to .githooks") {
		t.Errorf("doctor claims a gate that does not exist:\n%s", got)
	}
	if !strings.Contains(got, "no .githooks directory") {
		t.Errorf("doctor does not say the directory is missing:\n%s", got)
	}
}

func TestDoctorSaysAWiringCameFromOutsideTheRepository(t *testing.T) {
	// The directory is here and the hooks do run, so this is not "unwired" — but
	// nothing in the repository says so, and the next clone inherits none of it.
	withGlobalHooksPath(t, ".githooks")
	root := gitRepo(t, "version = 1\n")
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	e, out, _ := reviewEnv(t, root, "doctor")
	Run(e)
	if !strings.Contains(out.String(), "outside this repository") {
		t.Errorf("doctor does not say the wiring is not this repository's own:\n%s", out.String())
	}
}

func TestDoctorNamesTheLocalHooksThatCoreHooksPathSilences(t *testing.T) {
	// Setting core.hooksPath replaces .git/hooks rather than adding to it, so a
	// hook already installed there stops firing. Nothing mentioned that anywhere.
	withIsolatedGitConfig(t)
	root := gitRepo(t, "version = 1\n")
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "pre-commit"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	e, _, errOut := reviewEnv(t, root, "hooks", "install")
	if code := Run(e); code != 0 {
		t.Fatalf("hooks install exit %d: %s", code, errOut.String())
	}
	e2, out2, _ := reviewEnv(t, root, "doctor")
	Run(e2)
	if !strings.Contains(out2.String(), "pre-commit") {
		t.Errorf("doctor does not name the hook that stopped firing:\n%s", out2.String())
	}
}

func TestDoctorComparesTheAdoptedVersionAgainstTheBuildRunning(t *testing.T) {
	// The two were printed side by side and never compared, which left the
	// reader to notice they disagree — the only question the pair answers.
	root := gitRepo(t, "version = 1\n")
	lock := "version = 1\nframework_version = \"v9.9.9\"\n"
	if err := os.WriteFile(filepath.Join(root, ".framework.lock"), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	e, out, _ := reviewEnv(t, root, "doctor")
	Run(e)
	got := out.String()
	if !strings.Contains(got, "v9.9.9") {
		t.Fatalf("doctor does not report the adopted version:\n%s", got)
	}
	if !strings.Contains(got, "not the build running") {
		t.Errorf("doctor prints both versions without comparing them:\n%s", got)
	}
}

func TestInitScaffoldsAnAdoptableRepository(t *testing.T) {
	// The whole of E5: `mf init` reported success while leaving an adopter with
	// no standards, no instruction source and no generated vendor files, so
	// every gate afterwards read an empty tree.
	root := gitRepo(t, "")
	e, out, errOut := reviewEnv(t, root, "init")
	if code := Run(e); code != 0 {
		t.Fatalf("init exit %d: %s", code, errOut.String())
	}
	for _, rel := range []string{
		".framework.toml", ".framework.lock",
		"docs/standards/INDEX.md", "docs/agents/instructions.md", ".githooks/pre-push",
		"CLAUDE.md", "AGENTS.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("init left %s missing:\n%s", rel, out.String())
		}
	}
	// And what it wrote has to satisfy the gate that compares them.
	e2, out2, _ := reviewEnv(t, root, "check", "agents")
	if code := Run(e2); code != 0 {
		t.Errorf("`mf check agents` fails on a freshly scaffolded repository:\n%s", out2.String())
	}
	if strings.Contains(out2.String(), "no [agents.*] declared") {
		t.Errorf("the agents gate is permanently green on a fresh adoption:\n%s", out2.String())
	}
}

func TestInitLeavesAVendorFileSomeoneWrote(t *testing.T) {
	root := gitRepo(t, "")
	mine := "# our own instructions\n"
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	e, out, _ := reviewEnv(t, root, "init")
	if code := Run(e); code != 0 {
		t.Fatalf("init exit %d: %s", code, out.String())
	}
	got, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil || string(got) != mine {
		t.Errorf("init generated over a file someone wrote: %q", got)
	}
	if !strings.Contains(out.String(), "mf agents sync") {
		t.Errorf("init does not say how to generate the file it left alone:\n%s", out.String())
	}
}

func TestDoctorReportsNoPinsAsSomethingToDo(t *testing.T) {
	root := gitRepo(t, "version = 1\n")
	e, out, _ := reviewEnv(t, root, "doctor")
	Run(e)
	if !strings.Contains(out.String(), "no models pinned") {
		t.Errorf("doctor does not mention that nothing is pinned:\n%s", out.String())
	}
}

func TestInitWithoutProviderFlagsWritesNoMachineState(t *testing.T) {
	// The default is unchanged: init scaffolds policy, and reaching outside the
	// repository is something a person asks for rather than something an
	// activation step does to them.
	root := gitRepo(t, "")
	machine := filepath.Join(t.TempDir(), "config.toml")
	e, _, _ := initEnv(t, root, machine, "init")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(machine); !os.IsNotExist(err) {
		t.Errorf("init wrote a machine file nobody asked for: %v", err)
	}
}

func TestInitRecordsTheProviderTheAdopterChose(t *testing.T) {
	// Which provider reviews is the adopter's choice, made here, rather than a
	// vendor this framework picked for them. The route is machine state and the
	// chain that names it is policy, so one command writes both halves into the
	// layer each belongs in.
	root := gitRepo(t, "")
	machine := filepath.Join(t.TempDir(), "config.toml")
	e, out, errOut := initEnv(t, root, machine,
		"init", "--provider", "acme", "--endpoint", "https://api.acme.test/v1",
		"--api-key-env", "ACME_API_KEY", "--model", "acme-2")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s%s", code, out.String(), errOut.String())
	}

	body, err := os.ReadFile(machine)
	if err != nil {
		t.Fatalf("reading the machine file: %v", err)
	}
	for _, want := range []string{"https://api.acme.test/v1", "ACME_API_KEY", "acme-2"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the machine file does not carry %q:\n%s", want, body)
		}
	}
	if strings.Contains(string(body), "api.deepseek.com") {
		t.Error("the machine file carries a vendor the adopter did not name")
	}

	project, err := os.ReadFile(filepath.Join(root, ".framework.toml"))
	if err != nil {
		t.Fatalf("reading the project file: %v", err)
	}
	if !strings.Contains(string(project), `"acme"`) {
		t.Errorf("the scaffold does not name the chosen backend in a chain:\n%s", project)
	}
	if strings.Contains(string(project), "api.acme.test") {
		t.Error("the committed file carries a route; only a machine may define one")
	}
}

func TestInitRefusesAProviderItCannotReach(t *testing.T) {
	// Half a route is worse than none: the backend would resolve, be named in a
	// chain, and report itself unavailable on every run for a reason nothing
	// states.
	root := gitRepo(t, "")
	machine := filepath.Join(t.TempDir(), "config.toml")
	e, _, errOut := initEnv(t, root, machine, "init", "--provider", "acme")
	if code := Run(e); code == 0 {
		t.Fatal("init accepted a provider with no endpoint")
	}
	if !strings.Contains(errOut.String(), "--endpoint") {
		t.Errorf("the refusal does not name what is missing: %q", errOut.String())
	}
}

// initEnv is reviewEnv with the machine file named, because these tests assert
// on what init does and does not write there.
func initEnv(t *testing.T, root, machine string, args ...string) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	return Env{
		Args:        args,
		Stdout:      &out,
		Stderr:      &errOut,
		RepoRoot:    root,
		MachinePath: machine,
		Getenv:      func(string) string { return "" },
		GitConfig:   func(string) (string, bool) { return "", false },
	}, &out, &errOut
}

func TestInitMaterialisesTheAgentSourceWhereTheConfigurationNamesIt(t *testing.T) {
	// init wrote the source to the shipped layout and then generated from the
	// configured one, so a repository that relocates it got no instruction
	// files at all — and a success message, because nothing read the failure
	// the generation step had already reported.
	root := gitRepo(t, "version = 1\n\n[paths]\nagents_source = \".standards/docs/agents/instructions.md\"\n\n[agents.claude]\nfile = \"CLAUDE.md\"\nroles = [\"shared\", \"author\"]\n")
	e, out, errOut := initEnv(t, root, filepath.Join(t.TempDir(), "config.toml"), "init")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s%s", code, out.String(), errOut.String())
	}

	if _, err := os.Stat(filepath.Join(root, ".standards", "docs", "agents", "instructions.md")); err != nil {
		t.Errorf("the source was not written where the configuration names it: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("no instruction file was generated: %v", err)
	}
	if !strings.Contains(string(body), ".standards/docs/agents/instructions.md") {
		t.Error("the generated header does not name the source it came from")
	}
}

func TestInitRefusesASourceFilenameItCannotMaterialise(t *testing.T) {
	// The directory is the adopter's to choose; the filename is what this build
	// carries. Accepting a different one made init write `instructions.md` and
	// then generate from a name that was never written, one level below where
	// the same defect had already been fixed.
	root := gitRepo(t, "version = 1\n\n[paths]\nagents_source = \"docs/agents/rules.md\"\n\n[agents.claude]\nfile = \"CLAUDE.md\"\nroles = [\"shared\"]\n")
	e, _, errOut := initEnv(t, root, filepath.Join(t.TempDir(), "config.toml"), "init")
	if code := Run(e); code == 0 {
		t.Fatal("init accepted a source filename it does not ship")
	}
	if !strings.Contains(errOut.String(), "rules.md") || !strings.Contains(errOut.String(), "instructions.md") {
		t.Errorf("the refusal names neither the given nor the expected filename: %q", errOut.String())
	}
}

// read is the counterpart to write, for an assertion that needs the file body.
func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

// vendorSubmodule declares a submodule and, when populated, gives it a corpus.
func vendorSubmodule(t *testing.T, root, sub string, populated bool) {
	t.Helper()
	write(t, filepath.Join(root, ".gitmodules"),
		"[submodule \""+sub+"\"]\n\tpath = "+sub+"\n\turl = https://example.invalid/mf.git\n")
	dir := filepath.Join(root, filepath.FromSlash(sub))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if !populated {
		return
	}
	mk := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, path, text)
	}
	mk(filepath.Join(dir, "docs", "standards", "INDEX.md"), "# Development Standards Index\n")
	mk(filepath.Join(dir, "docs", "agents", "instructions.md"),
		"# Agent Instructions\n\nRead `docs/standards/INDEX.md`.\n\n<!-- mf:role shared -->\n## Binding\n\nRead `docs/standards/code_conventions.md`.\n\n<!-- mf:role author -->\n## Author\n\nWrite the change.\n\n<!-- mf:role reviewer -->\n## Reviewer\n\nReport findings.\n")
}

func TestInitAdoptsTheLayoutASubmoduleAlreadySupplies(t *testing.T) {
	// It wrote a second corpus under docs/standards and generated instruction
	// files pointing at it, so the repository ended up with two standards trees
	// and every gate reading the one nothing maintains.
	root := gitRepo(t, "")
	vendorSubmodule(t, root, ".standards", true)
	e, out, errOut := initEnv(t, root, filepath.Join(t.TempDir(), "config.toml"), "init")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s%s", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "standards", "INDEX.md")); err == nil {
		t.Error("init wrote a second corpus beside the one the submodule supplies")
	}
	body := read(t, filepath.Join(root, ".framework.toml"))
	if !strings.Contains(body, "standards = \".standards/docs/standards\"") {
		t.Error("the policy file does not name the submodule the gates must read")
	}
	claude := read(t, filepath.Join(root, "CLAUDE.md"))
	if !strings.Contains(claude, ".standards/docs/standards/INDEX.md") {
		t.Error("the generated instruction file points at a directory that does not resolve here")
	}
}

func TestInitRefusesADeclaredSubmoduleItCannotRead(t *testing.T) {
	// The state every known consumer is in. Nothing here can tell whether that
	// submodule supplies the corpus, and guessing writes the one thing a
	// repository must never be given twice.
	root := gitRepo(t, "")
	vendorSubmodule(t, root, ".standards", false)
	e, _, errOut := initEnv(t, root, filepath.Join(t.TempDir(), "config.toml"), "init")
	if code := Run(e); code == 0 {
		t.Fatal("init adopted a repository whose declared submodule it could not read")
	}
	said := errOut.String()
	for _, want := range []string{".standards", "submodule update --init", "--standards"} {
		if !strings.Contains(said, want) {
			t.Errorf("the refusal does not name %q: %q", want, said)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".framework.toml")); err == nil {
		t.Error("a refusal wrote the policy file anyway")
	}
}

func TestInitHonoursAnExplicitStandardsDirectoryOverDetection(t *testing.T) {
	root := gitRepo(t, "")
	vendorSubmodule(t, root, ".standards", false)
	e, out, errOut := initEnv(t, root, filepath.Join(t.TempDir(), "config.toml"),
		"init", "--standards", "policy/std")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s%s", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(filepath.Join(root, "policy", "std", "INDEX.md")); err != nil {
		t.Errorf("the named directory was not used: %v", err)
	}
}

func TestInitIgnoresASubmoduleThatCarriesSomethingElse(t *testing.T) {
	// A repository whose only submodule is a dependency adopts exactly as one
	// with no submodule at all.
	root := gitRepo(t, "")
	vendorSubmodule(t, root, "vendor/lib", false)
	write(t, filepath.Join(root, "vendor", "lib", "go.mod"), "module x\n")
	e, out, errOut := initEnv(t, root, filepath.Join(t.TempDir(), "config.toml"), "init")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s%s", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "standards", "INDEX.md")); err != nil {
		t.Errorf("an unrelated submodule changed how the standards are written: %v", err)
	}
}
