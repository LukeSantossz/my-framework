package cli

import (
	"fmt"

	"github.com/LukeSantossz/my-framework/internal/version"
)

// versionLine is the one place the build's identity is composed.
//
// `mf doctor` opens with it and `mf version` is it, so the two cannot drift
// into reporting different identities for the same binary — which is the only
// way a second place to ask could be worse than none.
func versionLine() string { return "mf " + version.Version }

// runVersion answers "what is this binary", and nothing else.
//
// It exists because every install path ends in that question and the three
// obvious spellings of it all reached the unknown-command path. `mf doctor`
// answered it, but on its way to resolving the configuration, probing every
// backend for reachability and reading the standards off disk: the wrong amount
// of work for a CI step or a script that wants a string, and work that fails
// for reasons having nothing to do with the version.
func runVersion(env Env) int {
	fmt.Fprintln(env.Stdout, versionLine())
	return 0
}
