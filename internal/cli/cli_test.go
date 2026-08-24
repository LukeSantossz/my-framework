package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(t *testing.T, projectBody string, args ...string) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	machine := filepath.Join(t.TempDir(), "config.toml")
	if projectBody != "" {
		if err := os.WriteFile(filepath.Join(root, ".framework.toml"), []byte(projectBody), 0o644); err != nil {
			t.Fatalf("writing project file: %v", err)
		}
	}
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

func TestConfigGetPrintsTheValueAndItsLayer(t *testing.T) {
	e, out, _ := env(t, "version = 1\n\n[review]\nbase = \"develop\"\n", "config", "get", "review.base")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, stdout=%q", code, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "develop") {
		t.Errorf("output %q lacks the value", got)
	}
	if !strings.Contains(got, "project") {
		t.Errorf("output %q lacks the layer; provenance is what pays for a second place to look", got)
	}
}

func TestConfigGetReportsAnUnknownKey(t *testing.T) {
	e, _, errOut := env(t, "version = 1\n", "config", "get", "review.nope")
	if code := Run(e); code == 0 {
		t.Error("exit 0 for an unknown key")
	}
	if !strings.Contains(errOut.String(), "review.nope") {
		t.Errorf("stderr %q does not name the key", errOut.String())
	}
}

func TestConfigListWithProvenanceNamesALayerForEveryKey(t *testing.T) {
	e, out, _ := env(t, "version = 1\n", "config", "list", "--provenance")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("no output")
	}
	for _, line := range lines {
		if !strings.Contains(line, "[") || !strings.Contains(line, "]") {
			t.Errorf("line %q carries no provenance", line)
		}
	}
}

func TestConfigListWithoutProvenanceOmitsIt(t *testing.T) {
	e, out, _ := env(t, "version = 1\n", "config", "list")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(out.String(), "[default:") {
		t.Error("provenance leaked into the plain listing")
	}
}

func TestConfigSetRefusesAMachineOnlyKeyInTheProjectLayer(t *testing.T) {
	e, _, errOut := env(t, "version = 1\n", "config", "set", "providers.x.endpoint", "http://localhost:1/v1")
	if code := Run(e); code == 0 {
		t.Error("exit 0 while writing machine state into a committed file")
	}
	if !strings.Contains(errOut.String(), "machine state") {
		t.Errorf("stderr %q does not explain the refusal", errOut.String())
	}
}

func TestConfigValidateReportsEveryProblem(t *testing.T) {
	project := "version = 1\n\n[backends.one]\nkind = \"telepathy\"\n\n[backends.two]\nkind = \"api\"\nprovider = \"deepseek\"\napi_key = \"sk-nope\"\n"
	e, _, errOut := env(t, project, "config", "validate")
	if code := Run(e); code == 0 {
		t.Error("exit 0 on an invalid configuration")
	}
	got := errOut.String()
	for _, want := range []string{"telepathy", "api_key"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr %q does not report %q", got, want)
		}
	}
}

func TestConfigMigrateReportsWhatItTookOverAndHowToRemoveTheOriginals(t *testing.T) {
	e, out, _ := env(t, "", "config", "migrate")
	e.GitConfig = func(key string) (string, bool) {
		if key == "r2.base" {
			return "develop", true
		}
		return "", false
	}
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "r2.base") {
		t.Errorf("output %q does not name the migrated key", got)
	}
	if !strings.Contains(got, "git config --global --unset r2.base") {
		t.Errorf("output %q does not show how to remove the original", got)
	}
}

func TestConfigMigrateSaysSoWhenThereIsNothingToDo(t *testing.T) {
	e, out, _ := env(t, "", "config", "migrate")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "no deprecated") {
		t.Errorf("output %q does not say there was nothing to migrate", out.String())
	}
}

func TestUnknownCommandExitsTwoAndPrintsUsage(t *testing.T) {
	e, _, errOut := env(t, "", "frobnicate")
	if code := Run(e); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "Usage:") {
		t.Errorf("stderr %q lacks usage", errOut.String())
	}
}

func TestMachineConfigPathHonoursTheOverride(t *testing.T) {
	got := MachineConfigPath(func(name string) string {
		if name == "MF_CONFIG_HOME" {
			return filepath.Join("some", "where")
		}
		return ""
	})
	want := filepath.Join("some", "where", "config.toml")
	if got != want {
		t.Errorf("MachineConfigPath = %q, want %q", got, want)
	}
}
