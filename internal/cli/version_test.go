package cli

import (
	"strings"
	"testing"

	"github.com/LukeSantossz/my-framework/internal/version"
)

func TestVersionCommandPrintsTheBuildVersion(t *testing.T) {
	e, out, errOut := env(t, "", "version")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	want := "mf " + version.Version + "\n"
	if got := out.String(); got != want {
		t.Errorf("stdout %q, want %q", got, want)
	}
}

func TestVersionFlagsAreAliasesOfTheCommand(t *testing.T) {
	// The issue behind this reports all three spellings being tried against a
	// binary that answered none of them, so shipping one and leaving the other
	// two on the unknown-command path would close the report without closing
	// the failure.
	e, want, _ := env(t, "", "version")
	if code := Run(e); code != 0 {
		t.Fatalf("`mf version` exited %d", code)
	}
	for _, spelling := range []string{"--version", "-v"} {
		e, out, errOut := env(t, "", spelling)
		if code := Run(e); code != 0 {
			t.Errorf("`mf %s` exited %d: %s", spelling, code, errOut.String())
			continue
		}
		if out.String() != want.String() {
			t.Errorf("`mf %s` printed %q, want %q", spelling, out.String(), want.String())
		}
	}
}

func TestVersionMatchesTheDoctorFirstLine(t *testing.T) {
	// One build must not have two identities. The equality is asserted rather
	// than a literal, so a change to what the string contains keeps the two
	// reports in step instead of splitting them.
	root := gitRepo(t, chainProject)
	e, doctorOut, _ := reviewEnv(t, root, "doctor")
	if code := Run(e); code != 0 {
		t.Fatalf("`mf doctor` exited %d", code)
	}
	first, _, _ := strings.Cut(doctorOut.String(), "\n")

	v, versionOut, _ := reviewEnv(t, root, "version")
	if code := Run(v); code != 0 {
		t.Fatalf("`mf version` exited %d", code)
	}
	if got := strings.TrimSuffix(versionOut.String(), "\n"); got != first {
		t.Errorf("`mf version` printed %q; `mf doctor` opens with %q", got, first)
	}
}

func TestVersionRunsOutsideARepository(t *testing.T) {
	// Asking a binary what it is must not depend on where it is standing: the
	// string comes from the build, never from a tree.
	for _, spelling := range []string{"version", "--version", "-v"} {
		e, out, errOut := outsideARepository(t, spelling)
		if code := Run(e); code != 0 {
			t.Errorf("`mf %s` exited %d outside a repository: %s", spelling, code, errOut.String())
			continue
		}
		if !strings.Contains(out.String(), version.Version) {
			t.Errorf("`mf %s` printed %q, which does not name the build", spelling, out.String())
		}
	}
}

func TestUsageNamesTheVersionCommand(t *testing.T) {
	// A command absent from the banner is one the reader who just hit the
	// unknown-command path still cannot find.
	if !strings.Contains(usageText, "mf version") {
		t.Errorf("the usage banner does not name `mf version`:\n%s", usageText)
	}
}
