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

func TestConfigValidateReportsAProblemOnlyTheCascadeCanSee(t *testing.T) {
	// The command's usage says it reports every problem, and this is the class
	// it could not see: the file is correct on its own terms, so the loader
	// accepts it — deliberately, so a fresh clone still runs — and only the
	// finished cascade can say the chain has no route to its reviewer.
	project := "version = 1\n\n[roles.r2]\nbackends = [\"local\"]\n\n[backends.local]\nkind = \"api\"\nprovider = \"nowhere\"\n"
	e, _, errOut := env(t, project, "config", "validate")
	if code := Run(e); code == 0 {
		t.Error("exit 0 for a chain whose only backend has no endpoint to reach")
	}
	got := errOut.String()
	for _, want := range []string{"local", "nowhere", "endpoint"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr %q does not report %q", got, want)
		}
	}
}

func TestConfigListRendersADeliberatelyEmptyValueAsEmpty(t *testing.T) {
	// An erased chain is a real answer, and the one a reader is most likely to
	// be tracing. Printing nothing at all would read as a broken line.
	project := "version = 1\n\n[roles.r2]\nbackends = []\n"
	e, out, _ := env(t, project, "config", "get", "roles.r2.backends")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "(empty)") {
		t.Errorf("output %q does not render the empty value", got)
	}
	if !strings.Contains(got, "project") {
		t.Errorf("output %q does not name the layer that emptied it", got)
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

// outsideARepository builds the environment a command sees when
// `git rev-parse --show-toplevel` finds nothing: an empty repository root.
func outsideARepository(t *testing.T, args ...string) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	return Env{
		Args:             args,
		Stdout:           &out,
		Stderr:           &errOut,
		MachinePath:      filepath.Join(t.TempDir(), "config.toml"),
		Getenv:           func(string) string { return "" },
		GitConfig:        func(string) (string, bool) { return "", false },
		DiscoverRepoRoot: func() string { return "" },
	}, &out, &errOut
}

func TestInitOutsideAGitRepositoryRefusesRatherThanScaffoldingIntoTheWorkingDirectory(t *testing.T) {
	// An empty repository root joins to a relative path, which resolves against
	// whatever directory the process happens to be in. `mf init` scaffolded a
	// policy file and a lock there and reported success.
	e, out, errOut := outsideARepository(t, "init")
	if code := Run(e); code == 0 {
		t.Fatalf("exit 0 outside a repository; stdout:\n%s", out.String())
	}
	if out.Len() != 0 {
		t.Errorf("a refused command still reported work:\n%s", out.String())
	}
	got := errOut.String()
	for _, want := range []string{"mf init", "git repository"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr %q does not contain %q", got, want)
		}
	}
}

func TestEveryCommandThatReadsTheRepositoryRefusesOutsideOne(t *testing.T) {
	// The rule is stated once, in requiresRepository, and this is what holds it
	// to the whole command table: a command added later that reads the
	// repository has to be listed or it silently resolves against the process
	// working directory.
	for _, args := range [][]string{
		{"init"}, {"doctor"}, {"check"}, {"hooks", "status"}, {"upgrade"},
		{"agents", "check"}, {"models", "list"}, {"config", "list"},
		{"author", "declare", "--provider", "x"},
		// The three that read a diff or a document out of the repository, and
		// resolve every one of those paths against the root.
		{"review"}, {"eval"}, {"explain", "x"},
	} {
		e, _, errOut := outsideARepository(t, args...)
		if code := Run(e); code == 0 {
			t.Errorf("`mf %s` exited 0 outside a repository", strings.Join(args, " "))
		}
		if !strings.Contains(errOut.String(), "git repository") {
			t.Errorf("`mf %s` stderr %q does not say what is missing", strings.Join(args, " "), errOut.String())
		}
	}
}

func TestTheCommandsWhoseSubjectIsThisMachineRunOutsideARepository(t *testing.T) {
	// Refusing these would be the opposite mistake: setting up a machine before
	// cloning anything is the case the machine layer exists for.
	e, out, errOut := outsideARepository(t, "config", "set", "review.effort", "medium", "--machine")
	if code := Run(e); code != 0 {
		t.Errorf("machine write refused outside a repository: %s", errOut.String())
	}
	if !strings.Contains(out.String(), "machine") {
		t.Errorf("output %q does not name the layer written", out.String())
	}
	e2, _, errOut2 := outsideARepository(t, "usage")
	if code := Run(e2); code != 0 {
		t.Errorf("`mf usage` refused outside a repository: %s", errOut2.String())
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
