package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const source = `# Agent Instructions

Read ` + "`docs/standards/INDEX.md`" + ` before doing anything.

<!-- mf:role shared -->
## Standards are binding

Follow ` + "`docs/standards/code_conventions.md`" + ` and its precedence order.

<!-- mf:role author -->
## Your role as Author

Specify before building.

<!-- mf:role reviewer -->
## Your role as Reviewer (R2)

You review; you do not rewrite.
`

func mustParse(t *testing.T) Source {
	t.Helper()
	src, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return src
}

func TestParseSplitsTheSourceOnItsRoleMarkers(t *testing.T) {
	src := mustParse(t)
	if len(src.Sections) != 3 {
		t.Fatalf("got %d sections, want 3: %+v", len(src.Sections), src.Sections)
	}
	want := []string{"shared", "author", "reviewer"}
	for i, r := range want {
		if src.Sections[i].Role != r {
			t.Errorf("section %d role = %q, want %q", i, src.Sections[i].Role, r)
		}
	}
	if !strings.Contains(src.Preamble, "INDEX.md") {
		t.Errorf("preamble lost: %q", src.Preamble)
	}
}

func TestParseFailsWhenTheSourceCarriesNoMarkers(t *testing.T) {
	if _, err := Parse("# Just a document\n\nNo markers.\n"); err == nil {
		t.Fatal("a source with no markers cannot be assigned to any vendor and must fail")
	}
}

func TestRenderGivesAVendorOnlyTheRolesItPlays(t *testing.T) {
	// The whole point: an authoring session should not carry reviewer
	// obligations in its context, and a reviewer should not be told to specify
	// before building.
	src := mustParse(t)
	out, err := Render(src, Target{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared", "author"}}, SourcePath)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "Standards are binding") || !strings.Contains(out, "role as Author") {
		t.Errorf("output lacks a role it should carry:\n%s", out)
	}
	if strings.Contains(out, "role as Reviewer") {
		t.Errorf("output carries a role this vendor does not play:\n%s", out)
	}
}

func TestRenderAlwaysCarriesThePreamble(t *testing.T) {
	src := mustParse(t)
	for _, roles := range [][]string{{"shared"}, {"reviewer"}} {
		out, err := Render(src, Target{Name: "x", File: "X.md", Roles: roles}, SourcePath)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if !strings.Contains(out, "INDEX.md") {
			t.Errorf("preamble missing for roles %v:\n%s", roles, out)
		}
	}
}

func TestRenderWritesAHeaderNamingTheSourceAndTheCommand(t *testing.T) {
	src := mustParse(t)
	out, _ := Render(src, Target{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared"}}, SourcePath)
	for _, want := range []string{SourcePath, "mf agents sync", "Do not edit"} {
		if !strings.Contains(out, want) {
			t.Errorf("header lacks %q:\n%s", want, out)
		}
	}
}

func TestRenderAppliesThePathPrefix(t *testing.T) {
	// A submodule consumer's CLAUDE.md must point into `.standards/`, or every
	// reference in the generated file resolves to nothing there.
	src := mustParse(t)
	out, err := Render(src, Target{
		Name: "claude", File: "CLAUDE.md", Roles: []string{"shared"},
		PathPrefix: ".standards/docs/standards",
	}, SourcePath)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, ".standards/docs/standards/code_conventions.md") {
		t.Errorf("path prefix not applied:\n%s", out)
	}
	if strings.Contains(out, "`docs/standards/code_conventions.md`") {
		t.Errorf("an unprefixed reference survived:\n%s", out)
	}
}

func TestRenderRefusesARoleTheSourceDoesNotDeclare(t *testing.T) {
	// A typo would otherwise produce a file quietly missing the obligations it
	// was supposed to carry.
	src := mustParse(t)
	_, err := Render(src, Target{Name: "x", File: "X.md", Roles: []string{"revewer"}}, SourcePath)
	if err == nil {
		t.Fatal("want an error for an undeclared role")
	}
	if !strings.Contains(err.Error(), "revewer") {
		t.Errorf("error %q does not name the bad role", err)
	}
}

// --- sync and check ---------------------------------------------------------

func fixture(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(SourcePath)), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return Options{
		RepoRoot: root,
		Targets: []Target{
			{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared", "author"}},
			{Name: "codex", File: "AGENTS.md", Roles: []string{"shared", "reviewer"}},
		},
	}
}

func TestSyncWritesEveryTarget(t *testing.T) {
	o := fixture(t)
	results, err := Sync(o)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	for _, r := range results {
		if !r.Changed {
			t.Errorf("%s reported no change on a first sync", r.File)
		}
		if _, err := os.Stat(filepath.Join(o.RepoRoot, r.File)); err != nil {
			t.Errorf("%s was not written", r.File)
		}
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	o := fixture(t)
	if _, err := Sync(o); err != nil {
		t.Fatal(err)
	}
	results, err := Sync(o)
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	for _, r := range results {
		if r.Changed {
			t.Errorf("%s rewritten on a second sync", r.File)
		}
	}
}

func TestCheckPassesImmediatelyAfterSync(t *testing.T) {
	o := fixture(t)
	if _, err := Sync(o); err != nil {
		t.Fatal(err)
	}
	results, err := Check(o)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, r := range results {
		if r.Drifted {
			t.Errorf("%s reported drift right after sync", r.File)
		}
	}
}

func TestCheckFailsWhenAGeneratedFileWasEditedByHand(t *testing.T) {
	// Without this the generated files are a convention people bypass by
	// editing the output, which is the original problem with extra steps.
	o := fixture(t)
	if _, err := Sync(o); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(o.RepoRoot, "CLAUDE.md")
	if err := os.WriteFile(path, []byte("# I edited this by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	results, _ := Check(o)
	drifted := false
	for _, r := range results {
		if r.File == "CLAUDE.md" && r.Drifted {
			drifted = true
		}
	}
	if !drifted {
		t.Error("a hand-edited output did not report drift")
	}
}

func TestCheckFailsWhenTheSourceChangedAndSyncWasNotRun(t *testing.T) {
	o := fixture(t)
	if _, err := Sync(o); err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(source, "Specify before building.", "Specify before building, always.", 1)
	if err := os.WriteFile(filepath.Join(o.RepoRoot, filepath.FromSlash(SourcePath)), []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	results, _ := Check(o)
	drifted := false
	for _, r := range results {
		if r.File == "CLAUDE.md" && r.Drifted {
			drifted = true
		}
	}
	if !drifted {
		t.Error("a changed source with no re-sync did not report drift")
	}
}

func TestCheckReportsAMissingOutputAsDrift(t *testing.T) {
	o := fixture(t)
	results, err := Check(o)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, r := range results {
		if !r.Drifted {
			t.Errorf("%s exists nowhere yet and must report drift", r.File)
		}
	}
}

func TestAddingAVendorNeedsNoCodeChange(t *testing.T) {
	o := fixture(t)
	o.Targets = append(o.Targets, Target{Name: "gemini", File: "GEMINI.md", Roles: []string{"shared", "reviewer"}})
	results, err := Sync(o)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	body, err := os.ReadFile(filepath.Join(o.RepoRoot, "GEMINI.md"))
	if err != nil {
		t.Fatalf("GEMINI.md not written: %v", err)
	}
	if !strings.Contains(string(body), "role as Reviewer") {
		t.Errorf("the new vendor did not get its roles:\n%s", body)
	}
}

func TestReadsTheSourceWhereTheRepositoryKeepsIt(t *testing.T) {
	// The submodule consumer again. Its standards moved behind `paths.standards`
	// and its records behind `paths.specs`, but the document those files are
	// generated from stayed hardcoded, so `mf agents sync` was the one command
	// that still could not find its input there.
	root := t.TempDir()
	vendored := filepath.Join(root, ".standards", "docs", "agents")
	if err := os.MkdirAll(vendored, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendored, "instructions.md"),
		[]byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Sync(Options{
		RepoRoot:   root,
		SourcePath: ".standards/docs/agents/instructions.md",
		Targets:    []Target{{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared"}}},
	})
	if err != nil {
		t.Fatalf("Sync from a relocated source: %v", err)
	}
	if len(results) != 1 || !results[0].Changed {
		t.Fatalf("nothing was written: %+v", results)
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Errorf("the vendor file was not generated: %v", err)
	}
}

func TestTheSourcePathFallsBackToTheShippedLayout(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "instructions.md"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(Options{
		RepoRoot: root,
		Targets:  []Target{{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared"}}},
	}); err != nil {
		t.Fatalf("Sync with no configured source: %v", err)
	}
}

func TestTheGeneratedHeaderNamesTheSourceItWasGeneratedFrom(t *testing.T) {
	// The header tells the reader which file to edit instead of this one. A
	// constant there sends a submodule consumer to a path that does not exist
	// in their repository — the same defect as reading from it, on the way out.
	root := t.TempDir()
	vendored := filepath.Join(root, ".standards", "docs", "agents")
	if err := os.MkdirAll(vendored, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendored, "instructions.md"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	const relocated = ".standards/docs/agents/instructions.md"

	if _, err := Sync(Options{
		RepoRoot:   root,
		SourcePath: relocated,
		Targets:    []Target{{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared"}}},
	}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), relocated) {
		t.Errorf("the header does not name the source it came from:\n%s", firstLines(string(body), 3))
	}
	if strings.Contains(string(body), "from "+SourcePath) {
		t.Error("the header names the shipped layout, which this repository does not use")
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// vendoredSource is a source that names its sibling skill documents, the way
// the shipped one does. The consumer-owned paths are in it on purpose: a
// rewrite that catches them would send the Spec Gate's reader into the
// framework's archive instead of this repository's.
const vendoredSource = "# Agent Instructions\n" +
	"\n" +
	"Read `docs/standards/INDEX.md` before doing anything.\n" +
	"\n" +
	"<!-- mf:role shared -->\n" +
	"## Standards are binding\n" +
	"\n" +
	"The approved spec under `docs/specs/` is the source of truth.\n" +
	"\n" +
	"<!-- mf:role author -->\n" +
	"## Your role as Author\n" +
	"\n" +
	"- **Issue tracker**: see `docs/agents/issue-tracker.md`.\n" +
	"- **Domain docs**: one `CONTEXT.md` plus `docs/adr/`. See `docs/agents/domain.md`.\n"

const vendoredPath = ".standards/docs/agents/instructions.md"

func mustParseVendored(t *testing.T) Source {
	t.Helper()
	src, err := Parse(vendoredSource)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return src
}

func TestRenderRewritesASkillReferenceToWhereTheVendoredSourceKeepsIt(t *testing.T) {
	// The skill documents ship beside the instruction source, so a repository
	// that vendors the corpus has them at `.standards/docs/agents/`. Left
	// unrewritten, the generated file sends every session to a path that does
	// not exist there, and nothing reports it: the output matches the source it
	// was generated from, which is all `mf check agents` can see.
	src := mustParseVendored(t)
	out, err := Render(src, Target{
		Name: "claude", File: "CLAUDE.md", Roles: []string{"shared", "author"},
		PathPrefix: ".standards/docs/standards",
	}, vendoredPath)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		".standards/docs/agents/issue-tracker.md",
		".standards/docs/agents/domain.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the generated file does not point at %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "`docs/agents/issue-tracker.md`") {
		t.Errorf("an unprefixed skill reference survived:\n%s", out)
	}
}

func TestRenderLeavesTheHeaderAloneWhenTheSourceIsVendored(t *testing.T) {
	// The header names the configured source, which in this layout already
	// begins with the prefix. Rewriting the finished string doubles it.
	src := mustParseVendored(t)
	out, err := Render(src, Target{
		Name: "claude", File: "CLAUDE.md", Roles: []string{"shared"},
		PathPrefix: ".standards/docs/standards",
	}, vendoredPath)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, vendoredPath) {
		t.Errorf("the header no longer names the source it was generated from:\n%s", out)
	}
	if strings.Contains(out, ".standards/.standards/") {
		t.Errorf("the rewrite was applied to the header:\n%s", out)
	}
}

func TestRenderDoesNotRewriteTheConsumerOwnedSpecAndADRPaths(t *testing.T) {
	// `docs/specs` and `docs/adr` belong to the adopting repository. A rewrite
	// that swept them in would point the Spec Gate's reader at the framework's
	// own archive.
	src := mustParseVendored(t)
	out, err := Render(src, Target{
		Name: "claude", File: "CLAUDE.md", Roles: []string{"shared", "author"},
		PathPrefix: ".standards/docs/standards",
	}, vendoredPath)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"`docs/specs/`", "`docs/adr/`"} {
		if !strings.Contains(out, want) {
			t.Errorf("%s was rewritten; it is this repository's, not the submodule's:\n%s", want, out)
		}
	}
}
