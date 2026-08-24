package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func TestConfigSetWritesTheProjectLayerByDefault(t *testing.T) {
	opts := fixture(t, "version = 1\n\n[review]\nbase = \"main\"\n", minimalMachine)
	if err := Set(opts, "review.base", "develop", TargetProject); err != nil {
		t.Fatalf("Set: %v", err)
	}
	body := readFile(t, filepath.Join(opts.RepoRoot, ProjectFileName))
	if !strings.Contains(body, `base = "develop"`) {
		t.Errorf("project file did not take the new value:\n%s", body)
	}
	assertValue(t, mustLoad(t, opts), "review.base", "develop", LayerProject)
}

func TestConfigSetPreservesTheRestOfTheFile(t *testing.T) {
	// A policy file is hand-edited, so a write that reformats it or drops its
	// comments costs the reader more than the setting is worth.
	project := "# why this base\nversion = 1\n\n[review]\nbase = \"main\"\neffort = \"high\"\n"
	opts := fixture(t, project, minimalMachine)
	if err := Set(opts, "review.base", "develop", TargetProject); err != nil {
		t.Fatalf("Set: %v", err)
	}
	body := readFile(t, filepath.Join(opts.RepoRoot, ProjectFileName))
	for _, want := range []string{"# why this base", `effort = "high"`, `base = "develop"`} {
		if !strings.Contains(body, want) {
			t.Errorf("lost %q from the file:\n%s", want, body)
		}
	}
}

func TestConfigSetAddsAKeyToAnExistingSection(t *testing.T) {
	opts := fixture(t, "version = 1\n\n[review]\nbase = \"main\"\n", minimalMachine)
	if err := Set(opts, "review.effort", "low", TargetProject); err != nil {
		t.Fatalf("Set: %v", err)
	}
	assertValue(t, mustLoad(t, opts), "review.effort", "low", LayerProject)
}

func TestConfigSetCreatesAMissingSection(t *testing.T) {
	opts := fixture(t, "version = 1\n", minimalMachine)
	if err := Set(opts, "review.base", "develop", TargetProject); err != nil {
		t.Fatalf("Set: %v", err)
	}
	assertValue(t, mustLoad(t, opts), "review.base", "develop", LayerProject)
}

func TestConfigSetCreatesAMissingProjectFile(t *testing.T) {
	opts := fixture(t, "", minimalMachine)
	if err := Set(opts, "review.base", "develop", TargetProject); err != nil {
		t.Fatalf("Set: %v", err)
	}
	body := readFile(t, filepath.Join(opts.RepoRoot, ProjectFileName))
	if !strings.Contains(body, "version = 1") {
		t.Errorf("a created project file must carry its schema version:\n%s", body)
	}
	assertValue(t, mustLoad(t, opts), "review.base", "develop", LayerProject)
}

func TestConfigSetWritesTheMachineLayerOnRequest(t *testing.T) {
	opts := fixture(t, "version = 1\n", minimalMachine)
	if err := Set(opts, "providers.deepseek.endpoint", "http://localhost:11434/v1", TargetMachine); err != nil {
		t.Fatalf("Set: %v", err)
	}
	assertValue(t, mustLoad(t, opts), "providers.deepseek.endpoint", "http://localhost:11434/v1", LayerMachine)
}

func TestConfigSetRefusesToWriteAMachineOnlyKeyIntoTheProjectFile(t *testing.T) {
	opts := fixture(t, "version = 1\n", minimalMachine)
	err := Set(opts, "providers.deepseek.endpoint", "http://localhost:11434/v1", TargetProject)
	assertRefused(t, err, "machine")
}

func TestConfigSetRefusesACredentialOutright(t *testing.T) {
	opts := fixture(t, "version = 1\n", minimalMachine)
	err := Set(opts, "providers.deepseek.api_key", "sk-nope", TargetMachine)
	assertRefused(t, err, "api_key")
}

func TestConfigSetRefusesAnApiKeyEnvThatIsNotAVariableName(t *testing.T) {
	// The setting holds the NAME of the variable, never the key. The shapes
	// that fail this check are exactly the shapes a real credential has.
	opts := fixture(t, "version = 1\n", minimalMachine)
	err := Set(opts, "providers.deepseek.api_key_env", "sk-abc123XYZ-not-a-name", TargetMachine)
	assertRefused(t, err, "variable name")
}

// --- migrate ----------------------------------------------------------------

func fakeGit(pairs map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := pairs[key]
		return v, ok
	}
}

func TestMigrateMovesLegacyGitConfigKeysIntoTheMachineFile(t *testing.T) {
	opts := fixture(t, "", "")
	opts.GitConfig = fakeGit(map[string]string{
		"r2.openai.endpoint":  "https://api.deepseek.com/v1",
		"r2.openai.apiKeyEnv": "DEEPSEEK_API_KEY",
	})
	moved, err := Migrate(opts)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(moved) != 2 {
		t.Fatalf("moved %d keys, want 2: %v", len(moved), moved)
	}
	cfg := mustLoad(t, opts)
	assertValue(t, cfg, "providers.openai.endpoint", "https://api.deepseek.com/v1", LayerMachine)
	assertValue(t, cfg, "providers.openai.api_key_env", "DEEPSEEK_API_KEY", LayerMachine)
}

func TestMigrateLeavesTheLegacyKeysReadableSoTheChangeIsReversible(t *testing.T) {
	// Migration is not destructive: it takes over responsibility for the value
	// and reports how to remove the source, rather than deleting a machine's
	// configuration as a side effect of running a command.
	opts := fixture(t, "", "")
	opts.GitConfig = fakeGit(map[string]string{"r2.base": "develop"})
	if _, err := Migrate(opts); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if v, ok := opts.GitConfig("r2.base"); !ok || v != "develop" {
		t.Error("Migrate removed the legacy key; it must leave the source intact")
	}
	// The machine layer outranks the legacy layer, so the migrated value wins.
	assertValue(t, mustLoad(t, opts), "review.base", "develop", LayerMachine)
}

func TestMigrateIsIdempotent(t *testing.T) {
	opts := fixture(t, "", "")
	opts.GitConfig = fakeGit(map[string]string{"r2.base": "develop"})
	if _, err := Migrate(opts); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	first := readFile(t, opts.MachinePath)
	if _, err := Migrate(opts); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if second := readFile(t, opts.MachinePath); second != first {
		t.Errorf("second run changed the file:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestMigrateReportsNothingWhenThereIsNoLegacyConfiguration(t *testing.T) {
	opts := fixture(t, "", minimalMachine)
	moved, err := Migrate(opts)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(moved) != 0 {
		t.Errorf("moved %v, want nothing", moved)
	}
}
