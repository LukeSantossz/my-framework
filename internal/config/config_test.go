package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture builds a repository root and a machine config path from the given
// file bodies. An empty body means the file is not written at all, which is a
// distinct case from an empty file.
func fixture(t *testing.T, projectBody, machineBody string) Options {
	t.Helper()
	root := t.TempDir()
	machineDir := t.TempDir()
	machinePath := filepath.Join(machineDir, "config.toml")

	if projectBody != "" {
		if err := os.WriteFile(filepath.Join(root, ProjectFileName), []byte(projectBody), 0o644); err != nil {
			t.Fatalf("writing project file: %v", err)
		}
	}
	if machineBody != "" {
		if err := os.WriteFile(machinePath, []byte(machineBody), 0o644); err != nil {
			t.Fatalf("writing machine file: %v", err)
		}
	}
	return Options{
		RepoRoot:    root,
		MachinePath: machinePath,
		Env:         func(string) string { return "" },
		GitConfig:   func(string) (string, bool) { return "", false },
	}
}

func mustLoad(t *testing.T, opts Options) *Config {
	t.Helper()
	cfg, err := Load(opts)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	return cfg
}

func assertValue(t *testing.T, cfg *Config, key, want string, wantLayer Layer) {
	t.Helper()
	got, prov, ok := cfg.Get(key)
	if !ok {
		t.Fatalf("Get(%q): not found", key)
	}
	if got != want {
		t.Errorf("Get(%q) = %q, want %q", key, got, want)
	}
	if prov.Layer != wantLayer {
		t.Errorf("Get(%q) layer = %s, want %s", key, prov.Layer, wantLayer)
	}
}

const minimalMachine = `
version = 1

[providers.deepseek]
kind = "openai-compatible"
endpoint = "https://api.deepseek.com/v1"
api_key_env = "DEEPSEEK_API_KEY"
`

// --- Resolution -------------------------------------------------------------

func TestResolvesAPolicyValueFromEnvOverProjectOverMachineOverLegacyOverDefault(t *testing.T) {
	project := `
version = 1
[review]
base = "develop"
`
	machine := minimalMachine + `
[review]
base = "machine-base"
`
	opts := fixture(t, project, machine)
	opts.GitConfig = func(key string) (string, bool) {
		if key == "r2.base" {
			return "legacy-base", true
		}
		return "", false
	}

	// All four layers present: env wins.
	withEnv := opts
	withEnv.Env = func(name string) string {
		if name == "MF_REVIEW_BASE" {
			return "env-base"
		}
		return ""
	}
	assertValue(t, mustLoad(t, withEnv), "review.base", "env-base", LayerEnv)

	// No env: project wins.
	assertValue(t, mustLoad(t, opts), "review.base", "develop", LayerProject)

	// No project: machine wins over legacy.
	noProject := fixture(t, "", machine)
	noProject.GitConfig = opts.GitConfig
	assertValue(t, mustLoad(t, noProject), "review.base", "machine-base", LayerMachine)

	// No project, no machine review section: legacy wins over default.
	legacyOnly := fixture(t, "", minimalMachine)
	legacyOnly.GitConfig = opts.GitConfig
	assertValue(t, mustLoad(t, legacyOnly), "review.base", "legacy-base", LayerLegacy)

	// Nothing at all: built-in default.
	bare := fixture(t, "", minimalMachine)
	assertValue(t, mustLoad(t, bare), "review.base", DefaultBase, LayerDefault)
}

func TestTreatsAnEmptyEnvironmentOverrideAsUnsetRatherThanAsAnEmptyValue(t *testing.T) {
	project := `
version = 1
[review]
base = "develop"
`
	opts := fixture(t, project, minimalMachine)
	opts.Env = func(name string) string {
		if name == "MF_REVIEW_BASE" {
			return ""
		}
		return ""
	}
	assertValue(t, mustLoad(t, opts), "review.base", "develop", LayerProject)
}

func TestReadsALegacyGitConfigKeyWhenNoProjectOrMachineValueExists(t *testing.T) {
	opts := fixture(t, "", minimalMachine)
	opts.GitConfig = func(key string) (string, bool) {
		if key == "r2.backends" {
			return "codex,gemini", true
		}
		return "", false
	}
	assertValue(t, mustLoad(t, opts), "roles.r2.backends", "codex,gemini", LayerLegacy)
}

func TestReturnsTheBuiltInDefaultWhenNoLayerSuppliesAValue(t *testing.T) {
	cfg := mustLoad(t, fixture(t, "", minimalMachine))
	assertValue(t, cfg, "review.base", DefaultBase, LayerDefault)
	assertValue(t, cfg, "review.max_diff_bytes", DefaultMaxDiffBytes, LayerDefault)
	assertValue(t, cfg, "review.timeout_seconds", DefaultTimeoutSeconds, LayerDefault)
}

// --- The policy / machine split --------------------------------------------

func TestRefusesAProjectFileThatDeclaresAProviderEndpoint(t *testing.T) {
	project := `
version = 1
[providers.deepseek]
endpoint = "https://evil.example/v1"
`
	_, err := Load(fixture(t, project, minimalMachine))
	assertRefused(t, err, "providers")
}

func TestRefusesAProjectFileThatDeclaresAnAPIKeyVariableName(t *testing.T) {
	project := `
version = 1
[backends.local]
kind = "api"
provider = "deepseek"
api_key_env = "SOME_KEY"
`
	_, err := Load(fixture(t, project, minimalMachine))
	assertRefused(t, err, "api_key_env")
}

func TestRefusesAProjectFileThatDeclaresALiteralAPIKey(t *testing.T) {
	project := `
version = 1
[backends.local]
kind = "api"
provider = "deepseek"
api_key = "sk-not-a-real-key"
`
	_, err := Load(fixture(t, project, minimalMachine))
	assertRefused(t, err, "api_key")
}

func TestRefusesAProjectFileThatDeclaresABackendEndpoint(t *testing.T) {
	project := `
version = 1
[backends.local]
kind = "api"
provider = "deepseek"
endpoint = "http://localhost:11434/v1"
`
	_, err := Load(fixture(t, project, minimalMachine))
	assertRefused(t, err, "endpoint")
}

func TestResolvesABackendProviderNameFromTheProjectAndItsEndpointFromTheMachine(t *testing.T) {
	project := `
version = 1
[backends.local]
kind = "api"
provider = "deepseek"
model = "deepseek-v4-flash"
`
	cfg := mustLoad(t, fixture(t, project, minimalMachine))
	assertValue(t, cfg, "backends.local.provider", "deepseek", LayerProject)
	assertValue(t, cfg, "providers.deepseek.endpoint", "https://api.deepseek.com/v1", LayerMachine)
	assertValue(t, cfg, "providers.deepseek.api_key_env", "DEEPSEEK_API_KEY", LayerMachine)
}

func TestAnAPIBackendWithNoConfiguredProviderStillLoads(t *testing.T) {
	// Whether a provider is reachable here is a property of this machine, not
	// of the file's correctness, and a provider nobody has configured yet is
	// indistinguishable from a misspelled one. Refusing the load would stop a
	// fresh clone from running any command at all; the backend reports itself
	// unavailable at run time instead, and the chain advances.
	project := `
version = 1
[backends.local]
kind = "api"
provider = "nowhere"
`
	cfg := mustLoad(t, fixture(t, project, minimalMachine))
	assertValue(t, cfg, "backends.local.provider", "nowhere", LayerProject)
	if endpoint, _, ok := cfg.Get("providers.nowhere.endpoint"); ok && endpoint != "" {
		t.Errorf("an unconfigured provider resolved to an endpoint: %q", endpoint)
	}
}

func TestAllowsACLIBackendWhoseProviderNoMachineFileDefines(t *testing.T) {
	// A cli backend's provider is an identity used for the cross-provider
	// check, not a route to reach. Requiring a machine entry for it would
	// break the shipped default chain on a machine that configured nothing.
	project := `
version = 1
[backends.codex]
kind = "cli"
provider = "openai"
command = "codex"
`
	cfg := mustLoad(t, fixture(t, project, minimalMachine))
	assertValue(t, cfg, "backends.codex.provider", "openai", LayerProject)
}

// --- Provenance -------------------------------------------------------------

func TestConfigGetNamesTheLayerTheValueResolvedFrom(t *testing.T) {
	project := `
version = 1
[review]
base = "develop"
`
	cfg := mustLoad(t, fixture(t, project, minimalMachine))
	_, prov, ok := cfg.Get("review.base")
	if !ok {
		t.Fatal("review.base not found")
	}
	if prov.Layer != LayerProject {
		t.Errorf("layer = %s, want %s", prov.Layer, LayerProject)
	}
	if !strings.Contains(prov.Source, ProjectFileName) {
		t.Errorf("source = %q, want it to name %q", prov.Source, ProjectFileName)
	}
}

func TestConfigListWithProvenancePrintsEveryResolvedValueWithItsLayer(t *testing.T) {
	project := `
version = 1
[review]
base = "develop"
`
	cfg := mustLoad(t, fixture(t, project, minimalMachine))
	keys := cfg.Keys()
	if len(keys) == 0 {
		t.Fatal("Keys() returned nothing")
	}
	for _, k := range keys {
		if _, prov, ok := cfg.Get(k); !ok || prov.Source == "" {
			t.Errorf("key %q has no provenance source", k)
		}
	}
	// Defaults are part of the resolved table, not hidden from it.
	found := false
	for _, k := range keys {
		if k == "review.timeout_seconds" {
			found = true
		}
	}
	if !found {
		t.Error("Keys() omitted a defaulted key; the resolved table must show defaults too")
	}
}

func TestNamesTheLegacyLayerDistinctlyFromTheMachineLayer(t *testing.T) {
	if LayerLegacy == LayerMachine {
		t.Fatal("legacy and machine must be distinct layers")
	}
	if LayerLegacy.String() == LayerMachine.String() {
		t.Errorf("legacy and machine render identically as %q", LayerLegacy.String())
	}
	opts := fixture(t, "", minimalMachine)
	opts.GitConfig = func(key string) (string, bool) {
		if key == "r2.base" {
			return "legacy-base", true
		}
		return "", false
	}
	cfg := mustLoad(t, opts)
	_, prov, _ := cfg.Get("review.base")
	if !strings.Contains(prov.Source, "r2.base") {
		t.Errorf("legacy source = %q, want it to name the git key", prov.Source)
	}
}

// --- Validation -------------------------------------------------------------

func TestConfigValidateRefusesAnUnknownSchemaVersion(t *testing.T) {
	project := `
version = 99
`
	_, err := Load(fixture(t, project, minimalMachine))
	assertRefused(t, err, "version")
}

func TestConfigValidateRefusesABackendOfAnUnknownKind(t *testing.T) {
	project := `
version = 1
[backends.weird]
kind = "telepathy"
provider = "openai"
`
	_, err := Load(fixture(t, project, minimalMachine))
	assertRefused(t, err, "telepathy")
}

func TestConfigValidateReportsEveryErrorItFoundRatherThanOnlyTheFirst(t *testing.T) {
	project := `
version = 1
[backends.one]
kind = "telepathy"
provider = "openai"

[backends.two]
kind = "api"
provider = "deepseek"
api_key = "sk-nope"
`
	_, err := Load(fixture(t, project, minimalMachine))
	var verr *ValidationError
	if !asValidationError(err, &verr) {
		t.Fatalf("want *ValidationError, got %T (%v)", err, err)
	}
	if len(verr.Problems) < 2 {
		t.Errorf("reported %d problems, want at least 2: %v", len(verr.Problems), verr.Problems)
	}
}

func TestLoadsARepositoryWithNoProjectFileUsingMachineAndDefaultLayersOnly(t *testing.T) {
	cfg := mustLoad(t, fixture(t, "", minimalMachine))
	assertValue(t, cfg, "review.base", DefaultBase, LayerDefault)
	assertValue(t, cfg, "providers.deepseek.endpoint", "https://api.deepseek.com/v1", LayerMachine)
}

func TestLoadsWithNoMachineFileAtAll(t *testing.T) {
	project := `
version = 1
[backends.codex]
kind = "cli"
provider = "openai"
command = "codex"
`
	cfg := mustLoad(t, fixture(t, project, ""))
	assertValue(t, cfg, "backends.codex.kind", "cli", LayerProject)
}

func TestRefusesAnUnknownKeyInTheProjectFile(t *testing.T) {
	project := `
version = 1
[review]
bse = "typo"
`
	_, err := Load(fixture(t, project, minimalMachine))
	assertRefused(t, err, "bse")
}

// --- helpers ----------------------------------------------------------------

func assertRefused(t *testing.T, err error, wantSubstring string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want an error naming %q, got nil", wantSubstring)
	}
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Errorf("error %q does not name %q", err.Error(), wantSubstring)
	}
}

func asValidationError(err error, target **ValidationError) bool {
	return errors.As(err, target)
}

func TestAppliesAnEnvOverrideForAKeyNoFileDefines(t *testing.T) {
	// An env override that lands on a key no layer happened to write must still
	// take effect. Resolving only over keys some file mentioned would make the
	// override silently conditional on unrelated configuration.
	opts := fixture(t, "", "")
	opts.Env = func(name string) string {
		if name == "MF_ROLES_R2_BACKENDS" {
			return "gemini,openai"
		}
		return ""
	}
	assertValue(t, mustLoad(t, opts), "roles.r2.backends", "gemini,openai", LayerEnv)
}

func TestShipsTheDocumentedDefaultR2Chain(t *testing.T) {
	// r2_gate.md states the shipped default chain is codex alone, so a
	// repository that configures nothing behaves as it always has.
	assertValue(t, mustLoad(t, fixture(t, "", "")), "roles.r2.backends", DefaultR2Backends, LayerDefault)
}

func TestAcceptsAnAPIBackendWhoseProviderEndpointComesFromTheLegacyLayer(t *testing.T) {
	// A clone configured before the TOML layers existed has its endpoint in
	// git config. Validating only against the decoded machine file would refuse
	// a configuration that resolves perfectly well, which is precisely the
	// upgrade break the deprecated layer exists to prevent.
	project := `
version = 1
[backends.legacy]
kind = "api"
provider = "openai"
`
	opts := fixture(t, project, "")
	opts.GitConfig = func(key string) (string, bool) {
		if key == "r2.openai.endpoint" {
			return "https://api.deepseek.com/v1", true
		}
		return "", false
	}
	cfg := mustLoad(t, opts)
	assertValue(t, cfg, "providers.openai.endpoint", "https://api.deepseek.com/v1", LayerLegacy)
}
