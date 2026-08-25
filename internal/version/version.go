// Package version carries the build's identity.
package version

import "runtime/debug"

// Dev is what a build reports when nothing has given it a released identity.
// It names an unreleased build so a lock file written by one is never mistaken
// for a released adoption.
const Dev = "0.0.0-dev"

// Version is the build's identity, in the form the release tags use (`v0.4.0`).
//
// It is a plain var initialised to a string constant because that is the only
// shape `-ldflags -X` can write to: the release workflow stamps the tag in, so
// a binary copied away from any source tree still reports what it was published
// as. Everything else is resolved once at start-up by initVersion.
var Version = Dev

func init() { Version, _ = resolve(Version, buildInfoVersion) }

// resolve decides which of the two identities a build has is the one to report.
//
// The stamp wins whenever there is one. Below it sits the version the toolchain
// itself recorded, and reading that is what closes the gap the documented
// install path fell into: `go install ...@latest` runs no release workflow, so
// nothing stamps the binary, and every such install used to report 0.0.0-dev
// and write that string into the adopter's `.framework.lock` — the precise
// confusion the default exists to prevent. The toolchain knows the module
// version in exactly that case, so it is asked.
//
// The fallbacks are ordered so that the least inferred answer wins, and the
// last of them is the honest admission rather than a guess.
func resolve(stamped string, fromBuild func() (string, bool)) (string, bool) {
	if stamped != Dev {
		return stamped, true
	}
	if v, ok := moduleVersion(fromBuild); ok {
		return v, true
	}
	return Dev, false
}

// moduleVersion filters the placeholders the toolchain records when it has no
// version to record. `go run` and a test binary both produce one, and copying
// it into a lock file would present "(devel)" as a released adoption — a claim
// that reads as real and can never be matched against a tag.
func moduleVersion(fromBuild func() (string, bool)) (string, bool) {
	v, ok := fromBuild()
	if !ok {
		return "", false
	}
	switch v {
	case "", "(devel)", "devel":
		return "", false
	}
	return v, true
}

// buildInfoVersion is the real source: the module version the linker embedded.
// It is injected into resolve rather than called from it so the decision above
// is testable without building binaries three different ways.
func buildInfoVersion() (string, bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	return info.Main.Version, true
}
