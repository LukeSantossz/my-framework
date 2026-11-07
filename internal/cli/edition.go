package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LukeSantossz/my-framework/internal/config"
	"github.com/LukeSantossz/my-framework/internal/edition/secfree"
)

// runEdition builds a published edition of the framework. There is one today,
// the sec-free edition of spec 0052; naming it on the command line keeps the
// command honest about which tree it writes.
func runEdition(env Env, args []string) int {
	if len(args) < 2 || args[0] != "build" || args[1] != "secfree" {
		fmt.Fprintln(env.Stderr, "mf edition: expected `mf edition build secfree --out <dir>`")
		return 2
	}
	out := ""
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--out":
			value, ok := optionValue(env, "mf edition build", args, &i)
			if !ok {
				return 2
			}
			out = value
		default:
			fmt.Fprintf(env.Stderr, "mf edition build: unknown argument %q\n", args[i])
			return 2
		}
	}
	if out == "" {
		fmt.Fprintln(env.Stderr, "mf edition build: --out <dir> is required; the edition is written only where you name")
		return 2
	}

	cfg, code := load(env)
	if code != 0 {
		return code
	}
	written, err := secfree.Build(editionOptions(env, cfg, out))
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf edition build: %v\n", err)
		return 1
	}
	for _, name := range written {
		fmt.Fprintf(env.Stdout, "+ %s\n", name)
	}
	fmt.Fprintf(env.Stdout, "\nwrote the sec-free edition (%d file(s)) to %s\n", len(written), out)
	return 0
}

func editionOptions(env Env, cfg *config.Config, out string) secfree.Options {
	return secfree.Options{
		RepoRoot:     env.RepoRoot,
		StandardsDir: standardsDir(cfg),
		AgentsSource: agentsSource(cfg),
		Out:          out,
		Forbidden:    declaredBackendNames(cfg),
	}
}

// declaredBackendNames is every backend name and command the committed policy
// declares. The project layer alone: the edition is cut from the repository,
// and a build that forbade different names on different machines would pass
// on one and fail on the next for the same tree.
func declaredBackendNames(cfg *config.Config) []string {
	if cfg == nil || cfg.Project == nil {
		return nil
	}
	var names []string
	for name, b := range cfg.Project.Backends {
		names = append(names, name)
		if b.Command != "" {
			names = append(names, b.Command)
		}
	}
	sort.Strings(names)
	return names
}

// editionGate builds the sec-free edition into a scratch directory, so a
// standard that gains an unmarked mention of the binary fails on main, in CI,
// rather than at the release that would publish it.
func editionGate(env Env, cfg *config.Config) int {
	source := filepath.Join(env.RepoRoot, filepath.FromSlash(secfree.EditionDir))
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		// Said out loud for the reason the agents gate says it: a gate that
		// passes by not running must not read like one that ran. An adopting
		// repository carries no edition source, and has nothing to build.
		fmt.Fprintf(env.Stdout, "ok   %-8s no %s in this repository; nothing is built\n", "edition", secfree.EditionDir)
		return 0
	} else if err != nil {
		fmt.Fprintf(env.Stderr, "mf check edition: %v\n", err)
		return 1
	}
	scratch, err := os.MkdirTemp("", "mf-edition-")
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf check edition: %v\n", err)
		return 1
	}
	defer func() {
		if err := os.RemoveAll(scratch); err != nil {
			fmt.Fprintf(env.Stderr, "mf check edition: could not remove the scratch build %s: %v\n", scratch, err)
		}
	}()

	written, err := secfree.Build(editionOptions(env, cfg, filepath.Join(scratch, "secfree")))
	if err != nil {
		fmt.Fprintf(env.Stdout, "FAIL %-8s the sec-free edition would not build\n", "edition")
		for _, line := range strings.Split(err.Error(), "\n") {
			fmt.Fprintf(env.Stdout, "       %s\n", strings.TrimSpace(line))
		}
		return 1
	}
	fmt.Fprintf(env.Stdout, "ok   %-8s the sec-free edition builds (%d file(s))\n", "edition", len(written))
	return 0
}
