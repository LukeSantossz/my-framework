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
	"path"
	"path/filepath"
	"sort"
	"strconv"
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

	// Where the documents each gate reads live, as this repository arranges
	// them. They are defaults rather than constants because an adopter that
	// consumes these standards as a submodule keeps them somewhere else, and a
	// gate that can only read one hardcoded directory is a gate that adopter
	// cannot run at all.
	DefaultStandardsDir = "docs/standards"
	DefaultSpecsDir     = "docs/specs"
	DefaultADRDir       = "docs/adr"
	DefaultAgentsSource = "docs/agents/instructions.md"
	DefaultAgentsFile   = "AGENTS.md"
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
	Structured          bool     `toml:"structured"`

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

	// Blocking says a finding this role classes as blocking may stop whatever
	// invoked the review — in practice the pre-push hook. It is per role
	// because the roles do not carry the same weight: R3 reviews with the least
	// context and posts to a pull request, so a repository can hold its own
	// pushes to a blocking R2 without making CI's advisory layer fail a build.
	//
	// It replaces `R2_BLOCKING`, which lived only in the deleted shell runner.
	// A setting reached through the cascade is one `mf config get` can explain
	// and any layer can answer, and it needs no second name: the cascade
	// already resolves `MF_ROLES_R2_BLOCKING` for it, which is the per-run form
	// the environment variable used to provide.
	Blocking bool `toml:"blocking"`
}

type Review struct {
	Base           string `toml:"base"`
	Model          string `toml:"model"`
	Effort         string `toml:"effort"`
	MaxDiffBytes   string `toml:"max_diff_bytes"`
	TimeoutSeconds string `toml:"timeout_seconds"`
}

// Paths tells the gates where this repository keeps the documents they read.
//
// It has no machine layer. Where a repository keeps its standards is a fact
// about the repository, identical on every clone, so a machine able to redirect
// it could make the same commit pass a gate on one machine and fail it on
// another — which is the drift these gates exist to catch. Every value is
// resolved against the repository root and may not leave it.
type Paths struct {
	Standards    string `toml:"standards"`
	Specs        string `toml:"specs"`
	ADR          string `toml:"adr"`
	AgentsSource string `toml:"agents_source"`
	// AgentsOverlay is this repository's own instruction sections, appended to
	// the generated vendor files. Empty is a repository that declared none.
	AgentsOverlay string `toml:"agents_overlay"`
	AgentsFile    string `toml:"agents_file"`
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
	Paths     Paths               `toml:"paths"`
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

	// DesignSurfaces are the files the design standard governs. The standard
	// owns the vocabulary; which files render it is this project's own fact, so
	// an adopter points it at theirs rather than inheriting ours.
	DesignSurfaces []string `toml:"design_surfaces"`
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

	// Roles let a machine supply a chain for a role the project leaves empty —
	// a locally reachable reviewer, say, that only exists here. It is modelled
	// because the deprecated `r2.backends` git-config key was machine state and
	// migration has to be lossless; a layer that cannot hold what it is handed
	// writes a file the loader then refuses. A project that declares the same
	// role still wins, so a machine cannot review a repository with a chain
	// that repository did not choose.
	Roles map[string]Role `toml:"roles"`

	// Backends are how a chain gets completed without editing committed policy,
	// which is what docs/adr/0006 decided and this loader did not implement: a
	// project names providers, and only a machine defines how to reach them.
	// Without this, naming a reviewer at all meant committing it, so a machine
	// with a local model and CI with a secret had no way to supply one — R3
	// spent a runner on every pull request to report that it did not run.
	//
	// A project definition of the same name wins, whole: see Config.Backend.
	// The committed-command rule does not apply here. Its subject is code that
	// arrives with a repository and runs on whoever clones it; a machine file is
	// its owner's own, and refusing them a reviewer they wrote themselves would
	// protect nobody from anything.
	Backends map[string]Backend `toml:"backends"`

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

// writer applies one decoded file into the resolved table.
//
// It carries that file's metadata because the value alone cannot tell which of
// two different statements a layer made: `backends = []` and no `backends` key
// at all both decode to an empty slice, and only the first may override a lower
// layer. A project that declares an empty chain has erased it deliberately; a
// project that never mentions the role has said nothing about it. Presence
// therefore travels beside the value instead of being inferred from it — which
// is what an empty string used as the sentinel for "unset" made impossible, and
// why `mf init`'s scaffold could not switch off the built-in chain.
type writer struct {
	cfg    *Config
	md     toml.MetaData
	layer  Layer
	source string
}

// set records a value the file actually contained, empty or not. The flat key
// and the TOML path are passed separately because the first is what a human
// reads and the second is how the document is addressed — a section name may
// itself contain a dot, and only the parts survive that.
func (w writer) set(key, value string, tomlPath ...string) {
	if !w.md.IsDefined(tomlPath...) {
		return
	}
	w.cfg.entries[key] = entry{value: value, prov: Provenance{Layer: w.layer, Source: w.source}}
}

func (w writer) review(r Review) {
	w.set("review.base", r.Base, "review", "base")
	w.set("review.model", r.Model, "review", "model")
	w.set("review.effort", r.Effort, "review", "effort")
	w.set("review.max_diff_bytes", r.MaxDiffBytes, "review", "max_diff_bytes")
	w.set("review.timeout_seconds", r.TimeoutSeconds, "review", "timeout_seconds")
}

func (w writer) roles(roles map[string]Role) {
	for name, role := range roles {
		prefix := "roles." + name + "."
		w.set(prefix+"backends", strings.Join(role.Backends, ","), "roles", name, "backends")
		w.set(prefix+"require_cross_provider", strconv.FormatBool(role.RequireCrossProvider),
			"roles", name, "require_cross_provider")
		w.set(prefix+"blocking", strconv.FormatBool(role.Blocking), "roles", name, "blocking")
	}
}

func (w writer) backends(backends map[string]Backend) {
	for name, b := range backends {
		prefix := "backends." + name + "."
		// A backend resolves as a whole definition, so the layer that defines a
		// name takes all of it and the entries a lower layer wrote for that name
		// go. Leaving them to show through the gaps would report a backend that
		// is half one file and half another, which is one nobody wrote.
		w.cfg.forget(prefix)
		w.set(prefix+"kind", b.Kind, "backends", name, "kind")
		w.set(prefix+"provider", b.Provider, "backends", name, "provider")
		w.set(prefix+"command", b.Command, "backends", name, "command")
		w.set(prefix+"args", strings.Join(b.Args, ","), "backends", name, "args")
		w.set(prefix+"unavailable_patterns", strings.Join(b.UnavailablePatterns, ","),
			"backends", name, "unavailable_patterns")
		w.set(prefix+"model", b.Model, "backends", name, "model")
		w.set(prefix+"effort", b.Effort, "backends", name, "effort")
		if b.Structured {
			w.set(prefix+"structured", "true", "backends", name, "structured")
		}
	}
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
	project, projectMeta, projectProblems := decodeProject(projectPath)
	problems = append(problems, projectProblems...)
	cfg.Project = project

	machine, machineMeta, machineProblems := decodeMachine(opts.MachinePath)
	problems = append(problems, machineProblems...)
	cfg.Machine = machine

	problems = append(problems, validateStatic(project, projectMeta, machine)...)
	if len(problems) > 0 {
		return nil, &ValidationError{Problems: problems}
	}

	// Lowest precedence first; a later layer overwrites what an earlier one
	// resolved, including with an empty value it declared on purpose.
	cfg.applyDefaults()
	cfg.applyLegacy(opts.GitConfig)
	if machine != nil {
		cfg.applyMachine(machine, machineMeta, opts.MachinePath)
	}
	if project != nil {
		cfg.applyProject(project, projectMeta, projectPath)
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
	//
	// Config.Validate reports it instead, which is what `mf config validate` is
	// for: saying that a route is missing is a different act from refusing to
	// let the tool start.
	return cfg, nil
}

// set records a value from a layer that has no notion of presence — a built-in
// default, or a git-config key that is absent exactly when it is empty. An
// empty value is dropped here for that reason and no other; a layer that can
// distinguish "declared empty" from "silent" writes through a writer instead.
func (c *Config) set(key, value string, layer Layer, source string) {
	if value == "" {
		return
	}
	c.entries[key] = entry{value: value, prov: Provenance{Layer: layer, Source: source}}
}

// forget drops every resolved key under a prefix, so a layer can replace a
// whole named object rather than merging into what a lower layer left behind.
func (c *Config) forget(prefix string) {
	for key := range c.entries {
		if strings.HasPrefix(key, prefix) {
			delete(c.entries, key)
		}
	}
}

func (c *Config) applyDefaults() {
	for key, value := range map[string]string{
		"review.base":            DefaultBase,
		"review.max_diff_bytes":  DefaultMaxDiffBytes,
		"review.timeout_seconds": DefaultTimeoutSeconds,
		"review.effort":          DefaultEffort,
		"roles.r2.backends":      DefaultR2Backends,
		"paths.standards":        DefaultStandardsDir,
		"paths.specs":            DefaultSpecsDir,
		"paths.adr":              DefaultADRDir,
		"paths.agents_source":    DefaultAgentsSource,
		"paths.agents_file":      DefaultAgentsFile,
	} {
		c.set(key, value, LayerDefault, "built-in default")
	}
	// R2 is the only role that carries the cross-provider rule, and it is the
	// default rather than a hardcoded test on the role's name so that a project
	// can move or drop the requirement. Every role gets a resolvable key,
	// because a role whose flag no layer wrote is one no override can reach.
	for _, name := range []string{"r1", "r2", "r3", "explain"} {
		c.set("roles."+name+".require_cross_provider", strconv.FormatBool(name == "r2"),
			LayerDefault, "built-in default (only R2 carries the rule)")
		// Advisory for every role until a layer says otherwise, which is what
		// ai_guidelines.md states. The key is written for roles no file
		// mentions for the same reason the flag above is: a key no layer wrote
		// is one no environment override can land on, and a switch that cannot
		// be reached is how `R2_BLOCKING` came to be documented and dead.
		c.set("roles."+name+".blocking", "false",
			LayerDefault, "built-in default (every review layer is advisory)")
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
	// No model is imposed: which one reviews is the backend's business, and a
	// default here would override every backend that names its own. The key is
	// still declared, for the reason the empty chains above are — a key no
	// layer wrote is one no environment override can land on, and
	// `MF_REVIEW_MODEL` was documented and dead for exactly that reason.
	c.entries["review.model"] = entry{value: "", prov: Provenance{Layer: LayerDefault, Source: "built-in default (each backend names its own)"}}
	// There is no default overlay: a repository either has its own instruction
	// sections or it does not. The key is declared here rather than in the table
	// above because `set` drops an empty value, and a key no layer wrote is a key
	// `MF_PATHS_AGENTS_OVERLAY` cannot land on.
	c.entries["paths.agents_overlay"] = entry{value: "", prov: Provenance{Layer: LayerDefault, Source: "built-in default (no overlay)"}}
}

func (c *Config) applyLegacy(gitConfig func(string) (string, bool)) {
	for gitKey, key := range legacyKeys {
		if value, ok := gitConfig(gitKey); ok && value != "" {
			c.set(key, value, LayerLegacy, "git config "+gitKey)
		}
	}
}

func (c *Config) applyMachine(m *MachineFile, md toml.MetaData, source string) {
	w := writer{cfg: c, md: md, layer: LayerMachine, source: source}
	w.review(m.Review)
	w.roles(m.Roles)
	w.backends(m.Backends)
	for name, p := range m.Providers {
		prefix := "providers." + name + "."
		w.set(prefix+"kind", p.Kind, "providers", name, "kind")
		w.set(prefix+"endpoint", p.Endpoint, "providers", name, "endpoint")
		w.set(prefix+"api_key_env", p.APIKeyEnv, "providers", name, "api_key_env")
	}
	w.set("explain.dir", m.Explain.Dir, "explain", "dir")
	for envVar, provider := range m.Fingerprints {
		w.set("fingerprints."+envVar, provider, "fingerprints", envVar)
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

func (c *Config) applyProject(p *ProjectFile, md toml.MetaData, source string) {
	w := writer{cfg: c, md: md, layer: LayerProject, source: source}
	w.review(p.Review)
	w.roles(p.Roles)
	w.backends(p.Backends)
	w.set("paths.standards", p.Paths.Standards, "paths", "standards")
	w.set("paths.specs", p.Paths.Specs, "paths", "specs")
	w.set("paths.adr", p.Paths.ADR, "paths", "adr")
	w.set("paths.agents_source", p.Paths.AgentsSource, "paths", "agents_source")
	w.set("paths.agents_overlay", p.Paths.AgentsOverlay, "paths", "agents_overlay")
	w.set("paths.agents_file", p.Paths.AgentsFile, "paths", "agents_file")
}

// Backend returns one named backend's whole definition and the layer it came
// from.
//
// A project definition shadows a machine one of the same name entirely, rather
// than field by field. Two reasons, and both point the same way: policy
// outranks machine state throughout docs/adr/0006, and a machine that could
// redefine a name a committed chain already uses could substitute the reviewer
// that repository chose. Merging the two instead would produce a backend that
// is half of each — a definition nobody wrote and nobody can predict.
//
// A machine backend therefore completes a chain by *adding* a name, which is
// exactly what the split intends: the project says which reviewers it wants,
// the machine says how any of them is reached from here.
// BackendNames lists every backend any layer defines, in a stable order. It is
// the enumerating counterpart to Backend: a caller that must visit all of them
// — comparing pins, validating a chain — cannot reach a machine backend by
// walking the project file, and walking one layer was how a machine-defined
// backend came to be silently exempt from the pin comparison.
func (c *Config) BackendNames() []string {
	seen := map[string]bool{}
	if c.Project != nil {
		for name := range c.Project.Backends {
			seen[name] = true
		}
	}
	if c.Machine != nil {
		for name := range c.Machine.Backends {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *Config) Backend(name string) (Backend, Layer, bool) {
	var (
		b     Backend
		layer Layer
		found bool
	)
	if c.Project != nil {
		b, found = c.Project.Backends[name]
		layer = LayerProject
	}
	if !found && c.Machine != nil {
		b, found = c.Machine.Backends[name]
		layer = LayerMachine
	}
	if !found {
		return Backend{}, LayerDefault, false
	}
	return c.withResolvedFields(name, b), layer, true
}

// withResolvedFields lays the cascade over a decoded backend.
//
// The definition is read whole from one file, but the flat entries carry a
// layer the files do not: the environment. Without this, `mf config get
// backends.codex.model` reported an override while `mf review` went on sending
// the committed value — one question with two answers, and the reader checked,
// was told it took, and was wrong.
//
// Only the scalar fields participate. A list is a definition rather than a
// setting, and `MF_BACKENDS_CODEX_ARGS` splitting one on commas would silently
// mangle an argument that contains one. The route fields are absent for the
// reason validateStatic refuses them in a committed file: where a backend is
// reached is a provider's business, not a per-run override.
func (c *Config) withResolvedFields(name string, b Backend) Backend {
	prefix := "backends." + name + "."
	for key, into := range map[string]*string{
		"kind":     &b.Kind,
		"provider": &b.Provider,
		"command":  &b.Command,
		"model":    &b.Model,
		"effort":   &b.Effort,
	} {
		if value, _, ok := c.Get(prefix + key); ok && value != "" {
			*into = value
		}
	}
	return b
}

// Provider resolves a provider's route through the cascade, the way Backend
// resolves a backend's.
//
// `mf review` read `Machine.Providers[name]` — the raw decoded file — while
// `mf config get providers.<name>.endpoint` answered from the resolved table.
// So an override that redirected a provider to a local endpoint for one run
// reported as taken and changed nothing: the diff went to the configured host
// anyway. That is the same defect withResolvedFields exists to have fixed for
// backends, and it is worse here, because the value it silently ignores is
// where a change is sent.
func (c *Config) Provider(name string) Provider {
	var p Provider
	if c.Machine != nil {
		p = c.Machine.Providers[name]
	}
	prefix := "providers." + name + "."
	for key, into := range map[string]*string{
		"kind":        &p.Kind,
		"endpoint":    &p.Endpoint,
		"api_key_env": &p.APIKeyEnv,
	} {
		if value, _, ok := c.Get(prefix + key); ok && value != "" {
			*into = value
		}
	}
	return p
}

// Validate answers what only the finished cascade can answer, and is separate
// from Load on purpose.
//
// Load refuses a file that is wrong on its own terms. What it must not refuse
// is a configuration that is merely incomplete *here*: an unconfigured provider
// and a misspelled one are indistinguishable from inside the file, and a fresh
// clone has to be able to run `mf config list` before anyone has set up a
// single endpoint. Reporting the gap is a different act from refusing to start,
// and this is where `mf config validate` performs it.
//
// Only backends a role chain actually names are examined. A definition nothing
// reaches for costs nobody anything, and may well be there for another machine.
func (c *Config) Validate() []Problem {
	var problems []Problem
	for _, roleName := range c.roleNames() {
		names, prov, _ := c.Get("roles." + roleName + ".backends")
		for _, name := range splitList(names) {
			spec, _, ok := c.Backend(name)
			if !ok {
				problems = append(problems, Problem{
					File: prov.Source, Key: "roles." + roleName + ".backends",
					Message: fmt.Sprintf("names backend %q, which no configuration layer defines", name),
				})
				continue
			}
			problems = append(problems, c.routeProblems(name, spec)...)
		}
	}
	return problems
}

// routeProblems reports whether a backend can actually be reached from here.
// Only an api backend needs a route at all: a cli backend's provider is an
// identity used for the cross-provider check, an external one runs where this
// tool cannot see it, and an in-session one is a claim about the session.
func (c *Config) routeProblems(name string, spec Backend) []Problem {
	if spec.Kind != "api" {
		return nil
	}
	if spec.Provider == "" {
		return []Problem{{
			File: "backends." + name, Key: "backends." + name + ".provider",
			Message: "an api backend must name the provider it reaches",
		}}
	}
	endpoint, _, _ := c.Get("providers." + spec.Provider + ".endpoint")
	if strings.TrimSpace(endpoint) != "" {
		return nil
	}
	return []Problem{{
		File: "machine config", Key: "providers." + spec.Provider + ".endpoint",
		Message: fmt.Sprintf("backend %q reaches provider %q, which no machine configuration gives an endpoint; "+
			"the chain will report it unavailable and move on", name, spec.Provider),
	}}
}

// roleNames lists every role that has a resolved chain, sorted, so a report
// over them reads the same way twice.
func (c *Config) roleNames() []string {
	var names []string
	for key := range c.entries {
		if strings.HasPrefix(key, "roles.") && strings.HasSuffix(key, ".backends") {
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(key, "roles."), ".backends"))
		}
	}
	sort.Strings(names)
	return names
}

// splitList reads a chain out of its flat, comma-joined form.
func splitList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
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

// decodeProject returns the decoded file and the metadata saying which keys it
// actually contained. The metadata is not an implementation detail of decoding:
// it is the only record of the difference between a key declared empty and a
// key never written, and the cascade needs that difference.
func decodeProject(file string) (*ProjectFile, toml.MetaData, []Problem) {
	var md toml.MetaData
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, md, nil
		}
		return nil, md, []Problem{{File: file, Key: "", Message: err.Error()}}
	}
	var f ProjectFile
	md, err = toml.Decode(string(data), &f)
	if err != nil {
		return nil, md, []Problem{{File: ProjectFileName, Key: "", Message: err.Error()}}
	}
	return &f, md, undecodedProblems(ProjectFileName, md)
}

func decodeMachine(file string) (*MachineFile, toml.MetaData, []Problem) {
	var md toml.MetaData
	if file == "" {
		return nil, md, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, md, nil
		}
		return nil, md, []Problem{{File: file, Key: "", Message: err.Error()}}
	}
	var f MachineFile
	md, err = toml.Decode(string(data), &f)
	if err != nil {
		return nil, md, []Problem{{File: file, Key: "", Message: err.Error()}}
	}
	return &f, md, undecodedProblems(filepath.Base(file), md)
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
// a resolved value belongs in Config.Validate, which runs after the cascade.
func validateStatic(project *ProjectFile, projectMeta toml.MetaData, machine *MachineFile) []Problem {
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
			if msg := commandProblem(b.Command); msg != "" {
				problems = append(problems, Problem{File: ProjectFileName, Key: key + ".command", Message: msg})
			}
			// Reachability is deliberately not checked here. Only an api backend
			// needs a route at all — a cli backend's provider is an identity
			// used for the cross-provider check — and whether that route exists
			// depends on the whole cascade, so Config.Validate answers it.
		}
		problems = append(problems, pathProblems(project.Paths, projectMeta)...)
		// `agents.<name>.file` is a path this committed file chooses and every
		// clone honours, so it is contained the way `paths.*` is. Without it
		// `mf agents sync` wrote outside the repository on whoever ran `mf
		// init`, from a value the repository itself supplied.
		for name, a := range project.Agents {
			if a.File == "" {
				continue
			}
			if msg := pathProblem(a.File); msg != "" {
				problems = append(problems, Problem{File: ProjectFileName,
					Key: "agents." + name + ".file", Message: msg})
			}
		}
	}

	if machine != nil {
		if machine.Version != 0 && machine.Version != SchemaVersion {
			problems = append(problems, Problem{File: "machine config", Key: "version",
				Message: fmt.Sprintf("unsupported schema version %d; this build understands %d", machine.Version, SchemaVersion)})
		}
		problems = append(problems, machineBackendProblems(machine.Backends)...)
	}

	return problems
}

// machineBackendProblems checks a machine's own backend definitions.
//
// Two rules from the project file do not carry over. A route may not be written
// here either — not because it is a secret, but because a provider already owns
// it, and a second home for the same fact is one nothing reads. And the
// committed-command rule is absent by design: it exists to stop a repository
// running its own code on whoever clones it, which is not a thing a user's own
// file can do to them.
func machineBackendProblems(backends map[string]Backend) []Problem {
	names := make([]string, 0, len(backends))
	for name := range backends {
		names = append(names, name)
	}
	sort.Strings(names)

	var problems []Problem
	for _, name := range names {
		b := backends[name]
		key := "backends." + name
		if b.Endpoint != "" {
			problems = append(problems, Problem{File: "machine config", Key: key + ".endpoint",
				Message: fmt.Sprintf("a backend selects a provider; the route belongs to providers.%s.endpoint", orName(b.Provider))})
		}
		if b.APIKeyEnv != "" {
			problems = append(problems, Problem{File: "machine config", Key: key + ".api_key_env",
				Message: fmt.Sprintf("a backend selects a provider; the credential reference belongs to providers.%s.api_key_env", orName(b.Provider))})
		}
		if b.APIKey != "" {
			problems = append(problems, Problem{File: "machine config", Key: key + ".api_key",
				Message: "a credential must never appear in configuration, committed or not"})
		}
		if b.Kind == "" {
			problems = append(problems, Problem{File: "machine config", Key: key + ".kind",
				Message: "a backend must declare a kind"})
		} else if !validKinds[b.Kind] {
			problems = append(problems, Problem{File: "machine config", Key: key + ".kind",
				Message: fmt.Sprintf("unknown backend kind %q", b.Kind)})
		}
	}
	return problems
}

func orName(provider string) string {
	if provider == "" {
		return "<provider>"
	}
	return provider
}

// pathProblems checks the configured document locations. Presence decides which
// of them are checked at all: an absent key takes the built-in default, while a
// key written empty is a statement, and the only one this table cannot honour.
func pathProblems(p Paths, md toml.MetaData) []Problem {
	var problems []Problem
	for _, cfgPath := range []struct {
		key, leaf, value string
	}{
		{"paths.standards", "standards", p.Standards},
		{"paths.specs", "specs", p.Specs},
		{"paths.adr", "adr", p.ADR},
		{"paths.agents_source", "agents_source", p.AgentsSource},
		{"paths.agents_overlay", "agents_overlay", p.AgentsOverlay},
		{"paths.agents_file", "agents_file", p.AgentsFile},
	} {
		if !md.IsDefined("paths", cfgPath.leaf) {
			continue
		}
		if msg := pathProblem(cfgPath.value); msg != "" {
			problems = append(problems, Problem{File: ProjectFileName, Key: cfgPath.key, Message: msg})
		}
	}
	return problems
}

// pathProblem refuses a configured location a gate could not safely resolve
// against the repository root, and returns "" for one it can.
//
// Every consumer joins these onto the root, so an empty value silently means
// the whole repository and an escaping one means somebody else's files. This
// file is committed, which is what makes the second more than a footgun: the
// path a gate reads would then be chosen by the repository, on every machine
// that clones it.
func pathProblem(value string) string {
	if strings.TrimSpace(value) == "" {
		return "a path may not be empty; remove the key to take the built-in default"
	}
	if absoluteAnywhere(value) {
		return fmt.Sprintf("%q is absolute; a configured path is resolved against the repository root", value)
	}
	if cleaned := path.Clean(filepath.ToSlash(value)); cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Sprintf("%q leaves the repository root; a gate reads only what the repository itself ships", value)
	}
	return ""
}

// absoluteAnywhere reports whether a path is absolute on any platform. A drive
// letter is absolute only on Windows and a leading slash only elsewhere, but
// this file travels between them, so both are refused on both.
//
// A drive prefix is a *letter* followed by a colon, so the first position is
// checked for one. Reading any second byte of ':' as a drive refused values
// like `9:15-notes/specs`, which no platform resolves outside the repository:
// the rule is here to catch `C:\standards`, not every colon.
func absoluteAnywhere(value string) bool {
	if strings.HasPrefix(filepath.ToSlash(value), "/") {
		return true
	}
	if len(value) >= 2 && value[1] == ':' {
		// Lowercasing by hand rather than through strings: a drive letter is
		// ASCII, and unicode folding would admit letters no filesystem names a
		// drive with.
		if c := value[0] | 0x20; c >= 'a' && c <= 'z' {
			return true
		}
	}
	return filepath.IsAbs(value)
}

// interpreters are the programs whose arguments are a program. A project file
// controls `args` as well as `command`, so naming one of these makes the
// committed file the source of the code that runs.
//
// The list is curated, not exhaustive, and cannot be: enough programs can be
// persuaded to run another that completing it is not a goal a denylist reaches.
// It covers the shells, the general-purpose interpreters, the exec wrappers and
// the package and build runners that execute a file the repository ships.
var interpreters = map[string]bool{
	// shells
	"sh": true, "bash": true, "zsh": true, "dash": true, "ash": true, "ksh": true,
	"csh": true, "tcsh": true, "fish": true, "busybox": true,
	"cmd": true, "powershell": true, "pwsh": true, "wsl": true,
	// exec wrappers: they run whatever they are handed
	"env": true, "xargs": true, "nohup": true, "timeout": true, "nice": true,
	"stdbuf": true, "sudo": true, "doas": true, "su": true, "ssh": true,
	// general-purpose interpreters
	"python": true, "python2": true, "python3": true, "py": true,
	"ruby": true, "perl": true, "node": true, "deno": true, "bun": true,
	"php": true, "lua": true, "tclsh": true, "rscript": true,
	"awk": true, "gawk": true, "mawk": true,
	"osascript": true, "wscript": true, "cscript": true, "mshta": true,
	"rundll32": true, "regsvr32": true,
	// package and build runners: they execute what the repository ships
	"npm": true, "npx": true, "pnpm": true, "yarn": true, "bunx": true,
	"uv": true, "uvx": true, "pipx": true, "poetry": true,
	"make": true, "cmake": true, "rake": true, "gulp": true, "grunt": true,
	"go": true, "cargo": true, "dotnet": true, "java": true,
	"mvn": true, "gradle": true, "ant": true,
	"docker": true, "podman": true, "nerdctl": true,
}

// commandProblem refuses a committed `command` that would let a repository run
// its own code on a contributor's machine, and returns "" for one that would not.
//
// This is the trust boundary the project file asserts. A cli backend's command
// and args go to exec.CommandContext verbatim, and `mf review` runs from the
// pre-push hook, so a repository shipping the wrong two lines executes them on
// anyone who clones it and pushes — before any human has read a diff.
//
// The rule is that a committed file may *select* a tool the contributor already
// installed and may never *introduce* code. A path names a file the repository
// itself ships; an interpreter turns `args`, which the same file controls, into
// the program. What is left is a bare name resolved from PATH, which the
// contributor chose to install.
//
// Argument splitting is already safe: args is a pre-tokenized TOML array
// executed without a shell, so an expanded `{{.Prompt}}` cannot become extra
// argv entries. And this narrows the hole rather than closing it — a committed
// backend still runs a real program on a contributor's machine, so the honest
// claim for the project file is reviewable policy, not a sandbox.
func commandProblem(command string) string {
	if command == "" {
		// A cli backend with no command is unavailable at run time and says so
		// there; every other kind has no command at all.
		return ""
	}
	if strings.ContainsAny(command, `/\`) || command == ".." {
		return fmt.Sprintf("%q names a path: a committed file may select a tool the contributor already installed, "+
			"never point at a file this repository ships", command)
	}
	if strings.ContainsAny(command, " \t\r\n\"'`$;|&<>()*?") {
		return fmt.Sprintf("%q is not a bare program name; a committed command is executed verbatim, "+
			"so it may name only a program to resolve on PATH", command)
	}
	name := strings.ToLower(command)
	for _, ext := range []string{".exe", ".bat", ".cmd", ".com", ".ps1"} {
		name = strings.TrimSuffix(name, ext)
	}
	if interpreters[name] {
		return fmt.Sprintf("%q runs whatever its arguments say, and this file controls those arguments: "+
			"a committed backend may select an installed tool, never supply the code it executes", command)
	}
	return ""
}
