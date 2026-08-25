package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const designStandard = `# Design Standard

<!-- mf:design tokens -->` + "```" + `toml
[color.canvas]
role  = "the page ground"
light = "#faf8f4"
dark  = "#1a1815"

[color.ink]
role  = "primary text"
light = "#1c1a17"
dark  = "#f2eee6"

[source]
name = "some/entry"
read = "2026-08-24"
fingerprints = ["d474b848022c7e18"]
` + "```" + `
`

// designRepo builds a repository carrying the standard, a project file naming
// one surface, and that surface's content.
func designRepo(t *testing.T, surface string) string {
	t.Helper()
	root := t.TempDir()
	standards := filepath.Join(root, "docs", "standards")
	if err := os.MkdirAll(standards, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(standards, "design.md"), designStandard)
	write(t, filepath.Join(root, "surface.css"), surface)
	write(t, filepath.Join(root, ".framework.toml"),
		"version = 1\n\n[checks]\ndesign_surfaces = [\"surface.css\"]\n")
	return root
}

func TestCheckDesignPassesOnASurfaceUsingOnlyDeclaredTokens(t *testing.T) {
	root := designRepo(t, "body { background: #faf8f4; color: #1c1a17; }\n")
	e, out, _ := reviewEnv(t, root, "check", "design")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "ok   design") {
		t.Errorf("the gate did not pass: %q", got)
	}
	// What was checked is reported, so `ok` cannot be confused with `checked
	// nothing`.
	if !strings.Contains(got, "1 surface(s)") {
		t.Errorf("the gate does not say what it checked: %q", got)
	}
}

func TestCheckDesignFailsASurfaceUsingAnUndeclaredColourAndNamesIt(t *testing.T) {
	root := designRepo(t, "body { background: #faf8f4; }\n.cta { color: #2f5d9e; }\n")
	e, out, _ := reviewEnv(t, root, "check", "design")
	if code := Run(e); code == 0 {
		t.Fatalf("an undeclared colour passed: %s", out.String())
	}
	got := out.String()
	for _, want := range []string{"#2f5d9e", "surface.css", "design.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("the failure does not name %q: %q", want, got)
		}
	}
}

func TestCheckDesignFailsWhenTheStandardCannotBeParsed(t *testing.T) {
	// An unparseable standard must stop the gate rather than leave an empty
	// palette accepting every colour — success reported exactly when the gate
	// stopped checking.
	root := designRepo(t, "body { color: #1c1a17; }")
	write(t, filepath.Join(root, "docs", "standards", "design.md"), "# Design Standard\n\nno token block\n")
	e, out, _ := reviewEnv(t, root, "check", "design")
	if code := Run(e); code == 0 {
		t.Fatalf("an unparseable standard passed: %s", out.String())
	}
	if !strings.Contains(out.String(), "FAIL design") {
		t.Errorf("the failure was not reported as the design gate: %q", out.String())
	}
}

func TestCheckDesignSaysSoWhenNoSurfaceIsDeclared(t *testing.T) {
	// An adopter with no rendered surface is conformant. Saying `ok` without
	// saying that nothing was checked is what would not be.
	root := designRepo(t, "body {}")
	write(t, filepath.Join(root, ".framework.toml"), "version = 1\n")
	e, out, _ := reviewEnv(t, root, "check", "design")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "no surfaces declared") {
		t.Errorf("the gate reported ok without saying it checked nothing: %q", out.String())
	}
}

func TestCheckDesignRunsWithNoModelAndNoNetwork(t *testing.T) {
	// Every deterministic gate holds this property; this one asserts it by
	// leaving the process without a PATH to reach any tool with.
	t.Setenv("PATH", "")
	root := designRepo(t, "body { background: #faf8f4; }")
	e, _, _ := reviewEnv(t, root, "check", "design")
	if code := Run(e); code != 0 {
		t.Fatal("the design gate needed something outside the process")
	}
}

func TestCheckRunsTheDesignGateAmongTheOthers(t *testing.T) {
	// `mf check` has to answer the whole question, not most of it.
	root := gitRepo(t, "version = 1\n")
	standards := filepath.Join(root, "docs", "standards")
	if err := os.MkdirAll(standards, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(standards, "design.md"), designStandard)
	e, out, _ := reviewEnv(t, root, "check", "design")
	Run(e)
	if !strings.Contains(out.String(), "design") {
		t.Errorf("the design gate produced no line: %q", out.String())
	}
}

func TestCheckDesignSaysSoWhenTheStandardIsNotThere(t *testing.T) {
	// An adopter whose vendored standards corpus carries no design.md could
	// not run `mf check` at all: the gate reported the missing file as a
	// failure of the repository rather than as the absence of its own input.
	root := designRepo(t, "body {}")
	if err := os.Remove(filepath.Join(root, "docs", "standards", "design.md")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, ".framework.toml"), "version = 1\n")

	e, out, _ := reviewEnv(t, root, "check", "design")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d with no standard and nothing declared: %s", code, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "design.md") || !strings.Contains(got, "ok   design") {
		t.Errorf("the gate did not report its input as absent: %q", got)
	}
}

func TestCheckDesignFailsWhenSurfacesAreDeclaredAndTheStandardIsNot(t *testing.T) {
	// The other half: a repository that declares surfaces has said it renders
	// something the identity governs, so an absent standard is a contradiction
	// rather than an adopter who never had one. docs/adr/0011 calls this gate
	// binding, and "absent means pass" would retire it by deleting a file.
	root := designRepo(t, "body { background: #faf8f4; }\n")
	if err := os.Remove(filepath.Join(root, "docs", "standards", "design.md")); err != nil {
		t.Fatal(err)
	}

	e, out, _ := reviewEnv(t, root, "check", "design")
	if code := Run(e); code == 0 {
		t.Fatalf("deleting the standard turned the gate off: %s", out.String())
	}
	got := out.String()
	if !strings.Contains(got, "FAIL design") || !strings.Contains(got, "design.md") {
		t.Errorf("the failure does not name the gate and the missing document: %q", got)
	}
}
