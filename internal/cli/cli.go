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

	"github.com/LukeSantossz/my-framework/internal/config"
)

// Env carries the process facts a command depends on, so a test can supply them
// instead of the real repository, environment and git configuration.
type Env struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer

	RepoRoot    string
	MachinePath string
	Getenv      func(string) string
	GitConfig   func(string) (string, bool)
}

const usage = `mf — development standards harness

Usage:
  mf config get <key>              print one resolved value and the layer it came from
  mf config list [--provenance]    print every resolved value
  mf config set <key> <value> [--machine|--project]
  mf config validate               load the configuration and report every problem
  mf config migrate                take over the deprecated r2.* git-config keys

  mf review --role <r1|r2|r3> [--base <ref>] [--dry-run]
                                   walk the role's backend chain and report
                                   which backend actually reviewed
`

// Run dispatches a command and returns the process exit code.
func Run(env Env) int {
	if env.Getenv == nil {
		env.Getenv = os.Getenv
	}
	if env.GitConfig == nil {
		env.GitConfig = gitConfigLookup
	}
	if env.RepoRoot == "" {
		env.RepoRoot = discoverRepoRoot()
	}
	if env.MachinePath == "" {
		env.MachinePath = MachineConfigPath(env.Getenv)
	}

	if len(env.Args) == 0 {
		fmt.Fprint(env.Stdout, usage)
		return 0
	}

	switch env.Args[0] {
	case "config":
		return runConfig(env, env.Args[1:])
	case "review":
		return runReview(env, env.Args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(env.Stdout, usage)
		return 0
	default:
		fmt.Fprintf(env.Stderr, "mf: unknown command %q\n\n%s", env.Args[0], usage)
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
		fmt.Fprint(env.Stderr, usage)
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
	fmt.Fprintf(env.Stdout, "%s\t[%s: %s]\n", value, prov.Layer, prov.Source)
	return 0
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
			fmt.Fprintf(env.Stdout, "%s = %s\t[%s: %s]\n", key, value, prov.Layer, prov.Source)
			continue
		}
		fmt.Fprintf(env.Stdout, "%s = %s\n", key, value)
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

func configValidate(env Env) int {
	cfg, code := load(env)
	if code != 0 {
		return code
	}
	fmt.Fprintf(env.Stdout, "configuration is valid (%d resolved keys)\n", len(cfg.Keys()))
	return 0
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
