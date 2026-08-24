package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
