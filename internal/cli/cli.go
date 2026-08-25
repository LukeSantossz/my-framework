// Package cli implements the mf commands. Logic lives here rather than in main
// so every command is exercised through the same entry point the user runs.
package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LukeSantossz/my-framework/internal/config"
)

// Env carries the process facts a command depends on, so a test can supply them
// instead of the real repository, environment and git configuration.
type Env struct {
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	RepoRoot    string
	MachinePath string
	Getenv      func(string) string
	GitConfig   func(string) (string, bool)

	// DiscoverRepoRoot answers "which repository am I in", and is injected for
	// the same reason the rest is: the interesting answer is "none", and a test
	// that had to run outside every repository on the machine could not produce
	// it.
	DiscoverRepoRoot func() string

	// Now is injected so a test can assert on a recorded date.
	Now func() time.Time
}

const usageText = `mf — development standards harness

Usage:
  mf config get <key>              print one resolved value and the layer it came from
  mf config list [--provenance]    print every resolved value
  mf config set <key> <value> [--machine|--project]
  mf config validate               load the configuration and report every problem
  mf config migrate                take over the deprecated r2.* git-config keys

  mf review --role <r1|r2|r3> [--base <ref>] [--dry-run]
                     [--pr <number>] [--post]
                                   walk the role's backend chain and report
                                   which backend actually reviewed

  mf check [spec|commit|branch|docs|records|agents|design]
                                   run the deterministic gates; no model is called

  mf doctor                        report what resolves, what is wired, what is missing
  mf init                          scaffold policy, wire hooks, record the version
  mf hooks install|uninstall|status
  mf upgrade                       compare standards against this build; applies nothing
  mf author declare --provider <name> [--model <id>]
  mf agents sync|check              generate the vendor instruction files from one source
  mf models pin|list                record and compare the model ids in use
  mf usage [show|reset]             tokens spent, in disjoint buckets
  mf eval [--role r2] [--backend n]  measure a backend against planted defects

  mf explain [--base <ref>] [--difficulty easy|medium|hard] [--dir <path>]
             [--dry-run]
                                   generate the CRUX explainer for this change,
                                   outside version control; advisory, never a gate
  mf statusline render|apply|refresh|revert
                                   the status line contract; apply edits the
                                   agent's own configuration, and revert
                                   restores what the last apply replaced
`

// Run dispatches a command and returns the process exit code.
func Run(env Env) int {
	if env.Getenv == nil {
		env.Getenv = os.Getenv
	}
	if env.GitConfig == nil {
		env.GitConfig = gitConfigLookup
	}
	if env.DiscoverRepoRoot == nil {
		env.DiscoverRepoRoot = discoverRepoRoot
	}
	if env.RepoRoot == "" {
		env.RepoRoot = env.DiscoverRepoRoot()
	}
	if env.MachinePath == "" {
		env.MachinePath = MachineConfigPath(env.Getenv)
	}

	if len(env.Args) == 0 {
		fmt.Fprint(env.Stdout, usageText)
		return 0
	}

	if env.RepoRoot == "" && requiresRepository(env.Args) {
		fmt.Fprintf(env.Stderr,
			"mf %s: not inside a git repository (`git rev-parse --show-toplevel` found none).\n"+
				"This command works on the repository it governs, and with no root to resolve\n"+
				"against it would act on whatever directory you happen to be in.\n"+
				"Run it from inside a repository, or `git init` first.\n", env.Args[0])
		return 1
	}

	switch env.Args[0] {
	case "config":
		return runConfig(env, env.Args[1:])
	case "review":
		return runReview(env, env.Args[1:])
	case "check":
		return runCheck(env, env.Args[1:])
	case "doctor":
		return runDoctor(env)
	case "init":
		return runInit(env)
	case "hooks":
		return runHooks(env, env.Args[1:])
	case "upgrade":
		return runUpgrade(env)
	case "author":
		return runAuthor(env, env.Args[1:])
	case "agents":
		return runAgents(env, env.Args[1:])
	case "models":
		return runModels(env, env.Args[1:])
	case "usage":
		return runUsage(env, env.Args[1:])
	case "eval":
		return runEval(env, env.Args[1:])
	case "explain":
		return runExplain(env, env.Args[1:])
	case "statusline":
		return runStatusline(env, env.Args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(env.Stdout, usageText)
		return 0
	default:
		fmt.Fprintf(env.Stderr, "mf: unknown command %q\n\n%s", env.Args[0], usageText)
		return 2
	}
}

// MachineConfigPath resolves the per-user file. os.UserConfigDir is %AppData%
// on Windows and ~/.config elsewhere, so naming one path in documentation would
// be wrong on the other platform; MF_CONFIG_HOME exists so a test and a
// confused user both have one answer.
func MachineConfigPath(getenv func(string) string) string {
	if home := getenv("MF_CONFIG_HOME"); home != "" {
		return filepath.Join(home, "config.toml")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "framework", "config.toml")
}

// requiresRepository decides whether a command may run with no repository root.
//
// The rule, stated once: a command needs a repository unless its whole subject
// is this machine. Everything else resolves a path against the root — the
// policy file, the standards, the lock, the diff a review reads — and an empty
// root makes every one of those relative, so the path resolves against whatever
// directory the process happens to be in. `mf init` in a plain directory
// scaffolded a policy file and a lock there and reported success; `mf doctor`
// afterwards printed a blank `repository:` line and carried on. Refusing is the
// only honest answer, because the command cannot tell which repository it was
// meant for.
//
// The exceptions are the three whose state lives outside any repository: the
// status line writes to the agent's own configuration, the usage total answers
// "what did I spend" rather than "what did this project cost", and machine
// configuration is what a person sets up before cloning anything.
func requiresRepository(args []string) bool {
	switch args[0] {
	case "help", "-h", "--help", "statusline", "usage":
		return false
	case "config":
		return configReadsTheRepository(args[1:])
	}
	return true
}

// configReadsTheRepository separates the machine-only configuration operations
// from the rest. `mf config set --machine` and `mf config migrate` write the
// per-user file and never open the project one; every other form resolves the
// cascade, and the project layer is a layer of it.
func configReadsTheRepository(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "migrate":
		return false
	case "set":
		for _, a := range args[1:] {
			if a == "--machine" {
				return false
			}
		}
	}
	return true
}

func discoverRepoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitConfigLookup(key string) (string, bool) {
	out, err := exec.Command("git", "config", "--get", key).Output()
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(string(out))
	return value, value != ""
}

func (e Env) configOptions() config.Options {
	return config.Options{
		RepoRoot:    e.RepoRoot,
		MachinePath: e.MachinePath,
		Env:         e.Getenv,
		GitConfig:   e.GitConfig,
	}
}

func runConfig(env Env, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(env.Stderr, usageText)
		return 2
	}
	switch args[0] {
	case "get":
		return configGet(env, args[1:])
	case "list":
		return configList(env, args[1:])
	case "set":
		return configSet(env, args[1:])
	case "validate":
		return configValidate(env)
	case "migrate":
		return configMigrate(env)
	default:
		fmt.Fprintf(env.Stderr, "mf config: unknown subcommand %q\n", args[0])
		return 2
	}
}

func load(env Env) (*config.Config, int) {
	cfg, err := config.Load(env.configOptions())
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return nil, 1
	}
	return cfg, 0
}

func configGet(env Env, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(env.Stderr, "mf config get: expected exactly one key")
		return 2
	}
	cfg, code := load(env)
	if code != 0 {
		return code
	}
	value, prov, ok := cfg.Get(args[0])
	if !ok {
		fmt.Fprintf(env.Stderr, "mf config get: no such key %q\n", args[0])
		return 1
	}
	// The layer is printed with the value, never behind a flag: a second place
	// to look is the cost this whole design accepted, and provenance is what
	// pays for it.
	fmt.Fprintf(env.Stdout, "%s\t[%s: %s]\n", displayValue(value), prov.Layer, prov.Source)
	return 0
}

// displayValue renders a resolved value for a reader. A value a layer emptied
// on purpose — a role chain a project switched off — is a real answer and the
// one most likely to be under investigation, so it is named rather than printed
// as nothing, which reads as a broken line rather than as an empty chain.
func displayValue(value string) string {
	if value == "" {
		return "(empty)"
	}
	return value
}

func configList(env Env, args []string) int {
	showProvenance := false
	for _, a := range args {
		if a == "--provenance" {
			showProvenance = true
		}
	}
	cfg, code := load(env)
	if code != 0 {
		return code
	}
	for _, key := range cfg.Keys() {
		value, prov, _ := cfg.Get(key)
		if showProvenance {
			fmt.Fprintf(env.Stdout, "%s = %s\t[%s: %s]\n", key, displayValue(value), prov.Layer, prov.Source)
			continue
		}
		fmt.Fprintf(env.Stdout, "%s = %s\n", key, displayValue(value))
	}
	return 0
}

func configSet(env Env, args []string) int {
	target := config.TargetProject
	var positional []string
	for _, a := range args {
		switch a {
		case "--machine":
			target = config.TargetMachine
		case "--project":
			target = config.TargetProject
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 2 {
		fmt.Fprintln(env.Stderr, "mf config set: expected <key> <value>")
		return 2
	}
	if err := config.Set(env.configOptions(), positional[0], positional[1], target); err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	fmt.Fprintf(env.Stdout, "%s set in the %s layer\n", positional[0], target)
	return 0
}

// configValidate answers both halves of "report every problem".
//
// Loading refuses a file that is wrong on its own terms. What it deliberately
// does not refuse is a configuration that is merely incomplete on this machine
// — an api backend whose provider has no endpoint here is a fresh clone, not a
// broken file, and refusing to load would leave that clone unable to run even
// `mf config list`. The gap is real all the same, and this is the command whose
// usage promises to name it.
func configValidate(env Env) int {
	cfg, code := load(env)
	if code != 0 {
		return code
	}
	problems := cfg.Validate()
	if len(problems) == 0 {
		fmt.Fprintf(env.Stdout, "configuration is valid (%d resolved keys)\n", len(cfg.Keys()))
		return 0
	}
	for _, p := range problems {
		fmt.Fprintf(env.Stderr, "%s: %s: %s\n", p.File, p.Key, p.Message)
	}
	// These never stop the tool from running — the chain reports an unreachable
	// backend as unavailable and moves on — so the exit code says the report is
	// non-empty, not that anything is now refused.
	fmt.Fprintf(env.Stderr, "\n%d problem(s); the configuration loads, but the run will not reach everything it names.\n", len(problems))
	return 1
}

func configMigrate(env Env) int {
	moved, err := config.Migrate(env.configOptions())
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	if len(moved) == 0 {
		fmt.Fprintln(env.Stdout, "no deprecated r2.* keys to migrate")
		return 0
	}
	for _, key := range moved {
		fmt.Fprintf(env.Stdout, "migrated %s\n", key)
	}
	// Nothing is deleted. The machine layer already outranks the legacy one, so
	// the migrated values are in effect; removing the originals stays a human
	// decision because a command that quietly drops configuration is not one a
	// user can undo.
	fmt.Fprintln(env.Stdout, "\nThe originals are untouched and still readable. To remove them:")
	for _, cmd := range config.RemovalCommands(moved) {
		fmt.Fprintf(env.Stdout, "  %s\n", cmd)
	}
	return 0
}
