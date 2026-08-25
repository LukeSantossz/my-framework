package version

import "testing"

func TestAStampedVersionOutranksAnythingInferred(t *testing.T) {
	// A release binary carries the tag it was published under. Nothing read
	// back out of the build may replace that claim, because the tag is the only
	// one an artifact copied away from its source can still be checked against.
	got, ok := resolve("v1.2.3", func() (string, bool) { return "v9.9.9", true })
	if !ok || got != "v1.2.3" {
		t.Errorf("resolve = %q, %v; the -ldflags stamp must win", got, ok)
	}
}

func TestAnUnstampedBuildTakesTheVersionTheToolchainRecorded(t *testing.T) {
	// This is the whole of H7: `go install ...@latest` produces a binary no
	// release workflow stamped, and reporting 0.0.0-dev from it writes
	// "0.0.0-dev" into every adopter's lock file.
	got, ok := resolve(Dev, func() (string, bool) { return "v0.4.0", true })
	if !ok || got != "v0.4.0" {
		t.Errorf("resolve = %q, %v; an unstamped build must take the module version", got, ok)
	}
}

func TestABuildWithNoIdentityStaysHonestlyUnreleased(t *testing.T) {
	// `go run` and a test binary have no module version. Inventing one would be
	// worse than the default: the default says "unreleased" and is true.
	if got, ok := resolve(Dev, func() (string, bool) { return "", false }); ok || got != Dev {
		t.Errorf("resolve = %q, %v; want the unreleased default", got, ok)
	}
}

func TestModuleVersionRejectsTheDevelopmentPlaceholder(t *testing.T) {
	// `(devel)` is what the toolchain records when it has no version to record.
	// Copied into the lock it would read as a released adoption named "(devel)".
	for _, placeholder := range []string{"", "(devel)", "devel"} {
		if got, ok := moduleVersion(func() (string, bool) { return placeholder, true }); ok {
			t.Errorf("moduleVersion(%q) = %q, %v; a placeholder is not a version", placeholder, got, ok)
		}
	}
}
