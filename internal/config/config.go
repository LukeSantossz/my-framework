// Package config resolves the framework's settings across four layers and
// records, for every resolved value, which layer it came from.
//
// The split is by the nature of the data, not by scope: policy lives in a
// committed project file, machine state lives in a per-user file, and the two
// vocabularies do not overlap. A project names providers; only the machine
// defines how to reach them. That is what makes "secrets never enter the
// repository" a rule this loader enforces rather than a convention a person
// remembers. See docs/adr/0006-configuration-split-policy-and-machine.md.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/LukeSantossz/my-framework/internal/usage"
)

// ProjectFileName is the committed policy file at the repository root.
const ProjectFileName = ".framework.toml"

// SchemaVersion is the only project/machine schema this build understands. An
// unknown version is refused rather than best-guessed, because silently reading
// a newer file with older rules is how a policy stops meaning what it says.
const SchemaVersion = 1

// Built-in defaults, as strings because the flat view is the one a human reads
// through `mf config get`. Structured consumers read the decoded files.
const (
	DefaultBase           = "main"
	DefaultMaxDiffBytes   = "30000"
	DefaultTimeoutSeconds = "240"
	DefaultEffort         = "high"
	// The chain r2_gate.md documents as shipped, so a repository that
	// configures nothing behaves exactly as it did before the seam existed.
	DefaultR2Backends = "codex"
)

// Layer identifies where a resolved value came from. Ordered by precedence,
// lowest first, so a later layer overrides an earlier one.
type Layer int

const (
	LayerDefault Layer = iota
	LayerLegacy
	LayerMachine
	LayerProject
	LayerEnv
)

func (l Layer) String() string {
	switch l {
	case LayerDefault:
		return "default"
	case LayerLegacy:
		return "legacy-git-config"
	case LayerMachine:
		return "machine"
	case LayerProject:
		return "project"
	case LayerEnv:
		return "env"
	}
	return "unknown"
}

// Provenance is the answer to "where did this value come from". It travels with
// the value rather than being attached at read time, because a report that
// names the layer a value was read from while showing a value that was since
// transformed is subtly false.
type Provenance struct {
	Layer  Layer
	Source string
}

// Options are the loader's inputs. Env and GitConfig are injected so the tests
// never touch the real environment or the real git configuration.
type Options struct {
	RepoRoot    string
	MachinePath string
	Env         func(string) string
	GitConfig   func(string) (string, bool)
}

// Backend is one named way of performing a role. Endpoint, APIKeyEnv and APIKey
// are declared here so that a project file naming them decodes and is then
// refused with a message that says which key was wrong — rejecting them as
// unknown keys would report a typo instead of a policy violation.
type Backend struct {
	Kind                string   `toml:"kind"`
	Provider            string   `toml:"provider"`
	Command             string   `toml:"command"`
	Args                []string `toml:"args"`
	UnavailablePatterns []string `toml:"unavailable_patterns"`
	Model               string   `toml:"model"`
	Effort              string   `toml:"effort"`

	Endpoint  string `toml:"endpoint"`
	APIKeyEnv string `toml:"api_key_env"`
	APIKey    string `toml:"api_key"`
}

// Provider is how to reach a vendor. It is machine state and has no project
// layer at all.
type Provider struct {
	Kind      string `toml:"kind"`
	Endpoint  string `toml:"endpoint"`
	APIKeyEnv string `toml:"api_key_env"`
	APIKey    string `toml:"api_key"`
}

type Role struct {
	Backends             []string `toml:"backends"`
	RequireCrossProvider bool     `toml:"require_cross_provider"`
}

type Review struct {
	Base           string `toml:"base"`
	Model          string `toml:"model"`
	Effort         string `toml:"effort"`
	MaxDiffBytes   string `toml:"max_diff_bytes"`
	TimeoutSeconds string `toml:"timeout_seconds"`
}

// ProjectFile is the committed policy file. It carries Providers only so that a
// project declaring one can be refused by name.
type ProjectFile struct {
	Version   int                 `toml:"version"`
	Roles     map[string]Role     `toml:"roles"`
	Backends  map[string]Backend  `toml:"backends"`
	Review    Review              `toml:"review"`
	Providers map[string]Provider `toml:"providers"`
	Checks    Checks              `toml:"checks"`
	Agents    map[string]Agent    `toml:"agents"`
}

// Agent is one vendor instruction file to generate. Roles are declared rather
// than derived from the backend chains, because the Author is not a chain: it is
// a per-branch declaration, so there is nothing to derive it from.
type Agent struct {
	File       string   `toml:"file"`
	Roles      []string `toml:"roles"`
	PathPrefix string   `toml:"path_prefix"`
}

// Checks configures the deterministic gates. ExemptPaths is what decides
// triviality for the spec check: a crude, readable list rather than a heuristic
// or a model, because a gate nobody can predict is a gate people route around,
// and an exemption that lives in a committed file shows up in review when
// someone widens it.
type Checks struct {
	ExemptPaths []string `toml:"exempt_paths"`
}

// Explain configures the CRUX explainer. Dir is machine state and has no
// project layer: the artifact is written outside version control by decision,
// so a committed file naming where it goes would be naming a path that only
// exists on one machine.
type Explain struct {
	Dir string `toml:"dir"`
}

// MachineFile is the per-user file. It is the only place a provider is defined.
type MachineFile struct {
	Version   int                 `toml:"version"`
	Providers map[string]Provider `toml:"providers"`
	Review    Review              `toml:"review"`
	Explain   Explain             `toml:"explain"`

	// Fingerprints maps an environment variable name to the provider whose
	// agent sets it, and is how a session can corroborate an Author
	// Declaration. It is machine state because which agent runs here is a
	// property of this machine, and it ships empty on purpose: guessing a
	// vendor's variable names would be inventing environment variables, which
	// ai_guidelines.md forbids. Until an adopter fills it in, the
	// cross-provider state is `declared` at best.
	Fingerprints map[string]string `toml:"fingerprints"`

	// Prices is the user-supplied cost table. None ships: prices change faster
	// than releases, and a stale price presented as cost is worse than no cost
	// at all. Without it, usage is reported in tokens and nothing else.
	Prices map[string]usage.Price `toml:"prices"`
}

// Problem is one validation failure. Every problem found is reported, because a
// loader that stops at the first one turns fixing a config into a guessing loop.
type Problem struct {
	File    string
	Key     string
	Message string
}

type ValidationError struct {
	Problems []Problem
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Problems))
	for _, p := range e.Problems {
		parts = append(parts, fmt.Sprintf("%s: %s: %s", p.File, p.Key, p.Message))
	}
	return "invalid configuration:\n  " + strings.Join(parts, "\n  ")
}

var validKinds = map[string]bool{
	"cli":        true,
	"api":        true,
	"inproc":     true,
	"in-session": true,
	"external":   true,
}

type entry struct {
	value string
	prov  Provenance
}

// Config is the resolved configuration: a flat key space for humans, plus the
// decoded files for consumers that need structure.
type Config struct {
	Project *ProjectFile
	Machine *MachineFile

	entries map[string]entry
}

// Get returns a resolved value and the layer it came from.
func (c *Config) Get(key string) (string, Provenance, bool) {
	e, ok := c.entries[key]
	if !ok {
		return "", Provenance{}, false
	}
	return e.value, e.prov, true
}

// Keys returns every resolved key, defaults included, sorted. Defaults are part
// of the resolved table rather than hidden from it: a value in effect that the
// report omits is exactly the surprise provenance exists to prevent.
func (c *Config) Keys() []string {
	keys := make([]string, 0, len(c.entries))
	for k := range c.entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// legacyKeys maps the deprecated git-config keys onto the new key space. The
// layer is read but never written; `mf config migrate` is what moves it.
var legacyKeys = map[string]string{
	"r2.base":                  "review.base",
	"r2.backends":              "roles.r2.backends",
	"r2.model":                 "review.model",
	"r2.effort":                "review.effort",
	"r2.openai.endpoint":       "providers.openai.endpoint",
	"r2.openai.apiKeyEnv":      "providers.openai.api_key_env",
	"r2.openai.maxDiffBytes":   "review.max_diff_bytes",
	"r2.openai.timeoutSeconds": "review.timeout_seconds",
}

// envName maps a dotted key onto its environment variable.
func envName(key string) string {
	return "MF_" + strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(key))
}

// Load resolves the configuration. It returns a *ValidationError carrying every
// problem found when the files are readable but wrong.
func Load(opts Options) (*Config, error) {
	if opts.Env == nil {
		opts.Env = os.Getenv
	}
	if opts.GitConfig == nil {
		opts.GitConfig = func(string) (string, bool) { return "", false }
	}

	cfg := &Config{entries: map[string]entry{}}
	var problems []Problem

	projectPath := filepath.Join(opts.RepoRoot, ProjectFileName)
	project, projectProblems := decodeProject(projectPath)
	problems = append(problems, projectProblems...)
	cfg.Project = project

	machine, machineProblems := decodeMachine(opts.MachinePath)
	problems = append(problems, machineProblems...)
	cfg.Machine = machine

	problems = append(problems, validateStatic(project, machine)...)
	if len(problems) > 0 {
		return nil, &ValidationError{Problems: problems}
	}

	// Lowest precedence first; set overwrites, so the last writer wins.
	cfg.applyDefaults()
	cfg.applyLegacy(opts.GitConfig)
	if machine != nil {
		cfg.applyMachine(machine, opts.MachinePath)
	}
	if project != nil {
		cfg.applyProject(project, projectPath)
	}
	cfg.applyEnv(opts.Env)

	// Reachability is deliberately not a load error.
	//
	// Whether an api backend's provider has an endpoint here is a property of
	// this machine, not of the configuration's correctness, and the two are
	// indistinguishable from inside the file: a provider nobody configured yet
	// and a misspelled one look identical. Refusing to load would mean a fresh
	// clone could not even run `mf config list` until someone set up every
	// endpoint the committed policy mentions.
	//
	// The graceful path already exists and is the one this framework chose
	// everywhere else: an api backend with no endpoint reports itself
	// unavailable, the chain advances, and the run names what it skipped.
	return cfg, nil
}

func (c *Config) set(key, value string, layer Layer, source string) {
	if value == "" {
		return
	}
	c.entries[key] = entry{value: value, prov: Provenance{Layer: layer, Source: source}}
}

func (c *Config) applyDefaults() {
	for key, value := range map[string]string{
		"review.base":            DefaultBase,
		"review.max_diff_bytes":  DefaultMaxDiffBytes,
		"review.timeout_seconds": DefaultTimeoutSeconds,
		"review.effort":          DefaultEffort,
		"roles.r2.backends":      DefaultR2Backends,
	} {
		c.set(key, value, LayerDefault, "built-in default")
	}
	// Roles with no shipped chain still need a resolvable key, so that an
	// environment override lands on them and so that the resolved table shows
	// them as empty rather than omitting them.
	for _, key := range []string{"roles.r1.backends", "roles.r3.backends", "roles.explain.backends"} {
		c.entries[key] = entry{value: "", prov: Provenance{Layer: LayerDefault, Source: "built-in default (empty chain)"}}
	}
	// The explainer's destination resolves to the user cache directory when
	// nothing sets it. The path is left empty here rather than computed,
	// because os.UserCacheDir differs per platform and a report that named one
	// of them would be wrong on the other.
	c.entries["explain.dir"] = entry{value: "", prov: Provenance{Layer: LayerDefault, Source: "built-in default (the user cache directory)"}}
}

func (c *Config) applyLegacy(gitConfig func(string) (string, bool)) {
	for gitKey, key := range legacyKeys {
		if value, ok := gitConfig(gitKey); ok && value != "" {
			c.set(key, value, LayerLegacy, "git config "+gitKey)
		}
	}
}

func (c *Config) applyReview(r Review, layer Layer, source string) {
	c.set("review.base", r.Base, layer, source)
	c.set("review.model", r.Model, layer, source)
	c.set("review.effort", r.Effort, layer, source)
	c.set("review.max_diff_bytes", r.MaxDiffBytes, layer, source)
	c.set("review.timeout_seconds", r.TimeoutSeconds, layer, source)
}

func (c *Config) applyMachine(m *MachineFile, source string) {
	c.applyReview(m.Review, LayerMachine, source)
	for name, p := range m.Providers {
		prefix := "providers." + name + "."
		c.set(prefix+"kind", p.Kind, LayerMachine, source)
		c.set(prefix+"endpoint", p.Endpoint, LayerMachine, source)
		c.set(prefix+"api_key_env", p.APIKeyEnv, LayerMachine, source)
	}
	c.set("explain.dir", m.Explain.Dir, LayerMachine, source)
	for envVar, provider := range m.Fingerprints {
		c.set("fingerprints."+envVar, provider, LayerMachine, source)
	}
}

// Fingerprints returns the environment-variable-to-provider map used to
// corroborate an Author Declaration. Empty unless a machine declared one.
func (c *Config) Fingerprints() map[string]string {
	if c.Machine == nil {
		return nil
	}
	return c.Machine.Fingerprints
}

func (c *Config) applyProject(p *ProjectFile, source string) {
	c.applyReview(p.Review, LayerProject, source)
	for name, role := range p.Roles {
		prefix := "roles." + name + "."
		c.set(prefix+"backends", strings.Join(role.Backends, ","), LayerProject, source)
		if role.RequireCrossProvider {
			c.set(prefix+"require_cross_provider", "true", LayerProject, source)
		}
	}
	for name, b := range p.Backends {
		prefix := "backends." + name + "."
		c.set(prefix+"kind", b.Kind, LayerProject, source)
		c.set(prefix+"provider", b.Provider, LayerProject, source)
		c.set(prefix+"command", b.Command, LayerProject, source)
		c.set(prefix+"args", strings.Join(b.Args, ","), LayerProject, source)
		c.set(prefix+"unavailable_patterns", strings.Join(b.UnavailablePatterns, ","), LayerProject, source)
		c.set(prefix+"model", b.Model, LayerProject, source)
		c.set(prefix+"effort", b.Effort, LayerProject, source)
	}
}

// applyEnv overrides any key already resolved. An empty variable is treated as
// unset, so `MF_REVIEW_BASE= mf review` falls through to the persisted value
// rather than resolving to the empty string.
func (c *Config) applyEnv(env func(string) string) {
	for key := range c.entries {
		name := envName(key)
		if value := env(name); value != "" {
			c.entries[key] = entry{value: value, prov: Provenance{Layer: LayerEnv, Source: "$" + name}}
		}
	}
}

func decodeProject(path string) (*ProjectFile, []Problem) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []Problem{{File: path, Key: "", Message: err.Error()}}
	}
	var f ProjectFile
	md, err := toml.Decode(string(data), &f)
	if err != nil {
		return nil, []Problem{{File: ProjectFileName, Key: "", Message: err.Error()}}
	}
	return &f, undecodedProblems(ProjectFileName, md)
}

func decodeMachine(path string) (*MachineFile, []Problem) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []Problem{{File: path, Key: "", Message: err.Error()}}
	}
	var f MachineFile
	md, err := toml.Decode(string(data), &f)
	if err != nil {
		return nil, []Problem{{File: path, Key: "", Message: err.Error()}}
	}
	return &f, undecodedProblems(filepath.Base(path), md)
}

// undecodedProblems turns unknown keys into errors. A misspelled key that is
// silently ignored is a setting the user believes is in effect and is not.
func undecodedProblems(file string, md toml.MetaData) []Problem {
	var problems []Problem
	for _, key := range md.Undecoded() {
		problems = append(problems, Problem{
			File:    file,
			Key:     key.String(),
			Message: "unknown key",
		})
	}
	return problems
}

// validateStatic checks what a file says on its own. Anything that depends on
// a resolved value belongs in validateResolved, which runs after the cascade.
func validateStatic(project *ProjectFile, machine *MachineFile) []Problem {
	var problems []Problem

	if project != nil {
		if project.Version != SchemaVersion {
			problems = append(problems, Problem{
				File: ProjectFileName, Key: "version",
				Message: fmt.Sprintf("unsupported schema version %d; this build understands %d", project.Version, SchemaVersion),
			})
		}
		// The split, enforced. A project that can carry reachability can carry
		// a credential, so the key is refused outright rather than overridden.
		for name := range project.Providers {
			problems = append(problems, Problem{
				File: ProjectFileName, Key: "providers." + name,
				Message: "a project file may not define providers; endpoints and credentials are machine state",
			})
		}
		for name, b := range project.Backends {
			key := "backends." + name
			if b.Endpoint != "" {
				problems = append(problems, Problem{File: ProjectFileName, Key: key + ".endpoint",
					Message: "endpoint is machine state and may not appear in a committed file"})
			}
			if b.APIKeyEnv != "" {
				problems = append(problems, Problem{File: ProjectFileName, Key: key + ".api_key_env",
					Message: "api_key_env is machine state and may not appear in a committed file"})
			}
			if b.APIKey != "" {
				problems = append(problems, Problem{File: ProjectFileName, Key: key + ".api_key",
					Message: "a credential must never appear in configuration, committed or not"})
			}
			if b.Kind == "" {
				problems = append(problems, Problem{File: ProjectFileName, Key: key + ".kind",
					Message: "a backend must declare a kind"})
			} else if !validKinds[b.Kind] {
				problems = append(problems, Problem{File: ProjectFileName, Key: key + ".kind",
					Message: fmt.Sprintf("unknown backend kind %q", b.Kind)})
			}
			// Reachability is deliberately not checked here. Only an api backend
			// needs a route at all — a cli backend's provider is an identity
			// used for the cross-provider check — and whether that route exists
			// depends on the whole cascade, so validateResolved answers it.
		}
	}

	if machine != nil && machine.Version != 0 && machine.Version != SchemaVersion {
		problems = append(problems, Problem{File: "machine config", Key: "version",
			Message: fmt.Sprintf("unsupported schema version %d; this build understands %d", machine.Version, SchemaVersion)})
	}

	return problems
}
