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

// --- list-valued keys and the machine role layer ------------------------------

func TestMigrateProducesAConfigurationThatLoads(t *testing.T) {
	// The regression this pair of fixes exists for: `mf config migrate` wrote a
	// machine file the loader then refused, so the command that was supposed to
	// take over the deprecated keys left every later command unable to start.
	opts := fixture(t, "version = 1\n", "")
	opts.GitConfig = func(key string) (string, bool) {
		switch key {
		case "r2.backends":
			return "codex,openai", true
		case "r2.openai.endpoint":
			return "https://api.deepseek.com", true
		case "r2.openai.apiKeyEnv":
			return "DEEPSEEK_API_KEY", true
		}
		return "", false
	}
	moved, err := Migrate(opts)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(moved) == 0 {
		t.Fatal("nothing migrated, so the regression cannot be observed")
	}
	cfg, err := Load(opts)
	if err != nil {
		t.Fatalf("the migrated configuration does not load: %v\n%s", err,
			readFile(t, opts.MachinePath))
	}
	assertValue(t, cfg, "roles.r2.backends", "codex,openai", LayerMachine)
}

func TestSetWritesAListValuedKeyAsAnArray(t *testing.T) {
	// A chain is a list. Writing it as one string decodes as a type error, and
	// the file is refused rather than misread — which is better than the
	// alternative, but only visible once something tries to load it.
	opts := fixture(t, "version = 1\n", "version = 1\n")
	if err := Set(opts, "roles.r2.backends", "codex, gemini ,deepseek", TargetMachine); err != nil {
		t.Fatalf("Set: %v", err)
	}
	body := readFile(t, opts.MachinePath)
	if !strings.Contains(body, `backends = ["codex", "gemini", "deepseek"]`) {
		t.Errorf("the chain was not written as an array:\n%s", body)
	}
	assertValue(t, mustLoad(t, opts), "roles.r2.backends", "codex,gemini,deepseek", LayerMachine)
}

func TestSetReplacesAnExistingArrayRatherThanAppendingBesideIt(t *testing.T) {
	opts := fixture(t, "version = 1\n", "version = 1\n\n[roles.r2]\nbackends = [\"codex\"]\n")
	if err := Set(opts, "roles.r2.backends", "gemini", TargetMachine); err != nil {
		t.Fatalf("Set: %v", err)
	}
	body := readFile(t, opts.MachinePath)
	if strings.Contains(body, `"codex"`) {
		t.Errorf("the old chain survived beside the new one:\n%s", body)
	}
	assertValue(t, mustLoad(t, opts), "roles.r2.backends", "gemini", LayerMachine)
}

func TestAProjectChainOutranksAMachineChain(t *testing.T) {
	// The cascade is unchanged by the machine layer gaining roles: a committed
	// policy still wins, so a machine cannot quietly review a repository with a
	// chain that repository did not choose.
	project := "version = 1\n\n[roles.r2]\nbackends = [\"codex\", \"gemini\"]\n"
	machine := "version = 1\n\n[roles.r2]\nbackends = [\"openai\"]\n"
	assertValue(t, mustLoad(t, fixture(t, project, machine)),
		"roles.r2.backends", "codex,gemini", LayerProject)
}

func TestAMachineChainAppliesWhenTheProjectDeclaresNone(t *testing.T) {
	machine := "version = 1\n\n[roles.r3]\nbackends = [\"coderabbit\"]\n"
	assertValue(t, mustLoad(t, fixture(t, "version = 1\n", machine)),
		"roles.r3.backends", "coderabbit", LayerMachine)
}

// --- the write guard and the load guard answer the same question --------------

func TestEveryKeyTheLoaderRefusesInALayerIsRefusedBeforeItIsWrittenThere(t *testing.T) {
	// The failure this covers: a write that succeeds into the wrong file leaves
	// a repository nobody can load until someone hand-edits it, which turns what
	// should have been one clean error into a broken clone for everybody.
	cases := []struct {
		key, value string
		target     Target
	}{
		{"backends.foo.api_key_env", "GITHUB_TOKEN", TargetProject},
		{"backends.foo.endpoint", "http://localhost:11434/v1", TargetProject},
		{"providers.foo.endpoint", "http://localhost:11434/v1", TargetProject},
		{"explain.dir", "/tmp/explain", TargetProject},
		{"fingerprints.CLAUDECODE", "anthropic", TargetProject},
		{"paths.standards", "docs/standards", TargetMachine},
		{"checks.exempt_paths", "README.md", TargetMachine},
		{"agents.claude.file", "CLAUDE.md", TargetMachine},
	}
	for _, c := range cases {
		opts := fixture(t, "version = 1\n", "version = 1\n")
		err := Set(opts, c.key, c.value, c.target)
		if err == nil {
			t.Errorf("Set(%s) into the %s layer succeeded; the loader refuses it there", c.key, c.target)
			continue
		}
		if !strings.Contains(err.Error(), c.key) {
			t.Errorf("Set(%s) error %q does not name the key", c.key, err)
		}
		// And the refusal must have written nothing.
		if _, loadErr := Load(opts); loadErr != nil {
			t.Errorf("Set(%s) left a configuration that no longer loads: %v", c.key, loadErr)
		}
	}
}

func TestConfigSetWritesABackendIntoTheMachineLayer(t *testing.T) {
	// The four-command recipe in the header of .framework.toml, which used to
	// exit zero four times and change nothing.
	opts := fixture(t, "version = 1\n", "version = 1\n")
	for _, kv := range [][2]string{
		{"providers.deepseek.endpoint", "https://api.deepseek.com/v1"},
		{"providers.deepseek.api_key_env", "DEEPSEEK_API_KEY"},
		{"providers.deepseek.kind", "openai-compatible"},
		{"backends.deepseek.kind", "api"},
		{"backends.deepseek.provider", "deepseek"},
		{"backends.deepseek.model", "deepseek-v4"},
		{"roles.r2.backends", "deepseek"},
	} {
		if err := Set(opts, kv[0], kv[1], TargetMachine); err != nil {
			t.Fatalf("Set(%s): %v", kv[0], err)
		}
	}
	cfg := mustLoad(t, opts)
	assertValue(t, cfg, "roles.r2.backends", "deepseek", LayerMachine)
	spec, layer, ok := cfg.Backend("deepseek")
	if !ok || layer != LayerMachine || spec.Kind != "api" || spec.Provider != "deepseek" {
		t.Fatalf("Backend(deepseek) = %+v, %s, %v; want the machine's api backend", spec, layer, ok)
	}
	if problems := cfg.Validate(); len(problems) > 0 {
		t.Errorf("the recipe produced a configuration validate rejects: %v", problems)
	}
}

func TestConfigSetRefusesACommittedCommandTheLoaderWouldRefuse(t *testing.T) {
	// `mf config set backends.x.command sh --project` wrote the value and left
	// the repository unloadable for everybody who clones it — the exact failure
	// the layer guards exist to prevent, applied to the one field that decides
	// what code runs on a contributor's machine.
	for _, command := range []string{"sh", "./scripts/review.sh", "node"} {
		opts := fixture(t, "version = 1\n", "version = 1\n")
		err := Set(opts, "backends.x.command", command, TargetProject)
		assertRefused(t, err, "backends.x.command")
		if _, loadErr := Load(opts); loadErr != nil {
			t.Errorf("the refusal of %q still left a configuration that does not load: %v", command, loadErr)
		}
	}
}

func TestConfigSetAllowsACommandOnTheMachineWhereTheLoaderDoes(t *testing.T) {
	// The committed-command rule guards a contributor against the repository.
	// A user's own machine file cannot do that to them, and the loader says so
	// too: refusing here would put a real route out of reach for no gain.
	opts := fixture(t, "version = 1\n", "version = 1\n")
	if err := Set(opts, "backends.x.command", "sh", TargetMachine); err != nil {
		t.Errorf("Set(backends.x.command) into the machine layer: %v", err)
	}
}

func TestConfigSetRefusesAPathThatLeavesTheRepository(t *testing.T) {
	// Caught at write time for the same reason as the layer guards: the loader
	// would refuse the file afterwards, and a refused file is everyone's problem
	// rather than this command's.
	opts := fixture(t, "version = 1\n", "version = 1\n")
	err := Set(opts, "paths.standards", "../shared/standards", TargetProject)
	assertRefused(t, err, "repository")
}

func TestSetWritesIntoASectionWhoseHeaderCarriesAComment(t *testing.T) {
	// The header was recognised only when the whole trimmed line was
	// `[section]`, so a comment after it appended a second table — and a file
	// that defines one twice does not load, for everyone who clones it, while
	// the command that wrote it reported success.
	dir := t.TempDir()
	path := filepath.Join(dir, ProjectFileName)
	body := `version = 1

[roles.r2]  # the reviewer chain
backends = ["codex"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{RepoRoot: dir, MachinePath: filepath.Join(t.TempDir(), "config.toml"),
		Env: func(string) string { return "" }, GitConfig: func(string) (string, bool) { return "", false }}
	if err := Set(opts, "roles.r2.backends", "gemini", TargetProject); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(got), "[roles.r2]"); n != 1 {
		t.Errorf("the file declares [roles.r2] %d times:\n%s", n, got)
	}
	if !strings.Contains(string(got), `backends = ["gemini"]`) {
		t.Errorf("the value was not replaced in place:\n%s", got)
	}
	// The proof that matters: it still loads.
	cfg, err := Load(opts)
	if err != nil {
		t.Fatalf("the file this command wrote no longer loads: %v", err)
	}
	if v, _, _ := cfg.Get("roles.r2.backends"); v != "gemini" {
		t.Errorf("roles.r2.backends resolved to %q", v)
	}
}

func TestSetReplacesAQuotedKeyRatherThanDuplicatingIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ProjectFileName)
	body := "version = 1\n\n[review]\n\"base\" = \"main\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{RepoRoot: dir, MachinePath: filepath.Join(t.TempDir(), "config.toml"),
		Env: func(string) string { return "" }, GitConfig: func(string) (string, bool) { return "", false }}
	if err := Set(opts, "review.base", "trunk", TargetProject); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(got), "base") != 1 {
		t.Errorf("the key was duplicated rather than replaced:\n%s", got)
	}
}

func TestSetWritesIntoASectionWhoseHeaderSpacesTheDots(t *testing.T) {
	// Raised by R3: TOML allows space around the separators, so `[roles . r2]`
	// names the table `mf config set roles.r2.backends` is looking for. Missing
	// it appended a duplicate, which is the same unloadable file by a different
	// route.
	dir := t.TempDir()
	path := filepath.Join(dir, ProjectFileName)
	if err := os.WriteFile(path, []byte("version = 1\n\n[roles . r2]\nbackends = [\"codex\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{RepoRoot: dir, MachinePath: filepath.Join(t.TempDir(), "config.toml"),
		Env: func(string) string { return "" }, GitConfig: func(string) (string, bool) { return "", false }}
	if err := Set(opts, "roles.r2.backends", "gemini", TargetProject); err != nil {
		t.Fatalf("Set: %v", err)
	}
	cfg, err := Load(opts)
	if err != nil {
		t.Fatalf("the file this command wrote no longer loads: %v", err)
	}
	if v, _, _ := cfg.Get("roles.r2.backends"); v != "gemini" {
		t.Errorf("roles.r2.backends resolved to %q", v)
	}
}
