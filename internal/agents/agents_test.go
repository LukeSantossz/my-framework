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

// --- overlay ----------------------------------------------------------------

// overlaySource is the repository's own instructions. Its paths are this
// repository's, not the submodule's, so nothing here may be rewritten.
const overlaySource = "<!-- mf:role author -->\n" +
	"## This project\n" +
	"\n" +
	"The toolchain is pinned in `docs/standards/toolchain.md`; another SDK rewrites the lockfile.\n" +
	"\n" +
	"<!-- mf:role reviewer -->\n" +
	"## Reviewing here\n" +
	"\n" +
	"Classification collapses six causes into one null.\n"

func mustParseOverlay(t *testing.T) Source {
	t.Helper()
	src := mustParse(t)
	overlay, err := Parse(overlaySource)
	if err != nil {
		t.Fatalf("Parse overlay: %v", err)
	}
	src.Overlay = overlay.Sections
	return src
}

func TestAnOverlaySectionReachesTheVendorFileForARoleItPlays(t *testing.T) {
	// A repository that vendors the corpus has nowhere else to state an
	// obligation no shipped document can guess: editing the generated file is
	// what `mf check agents` reports as drift.
	src := mustParseOverlay(t)
	out, err := Render(src, Target{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared", "author"}}, SourcePath)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "## This project") {
		t.Errorf("the overlay's author section is missing:\n%s", out)
	}
	if strings.Index(out, "## This project") < strings.Index(out, "## Your role as Author") {
		t.Errorf("the overlay reads before the framework section it refines:\n%s", out)
	}
}

func TestAnOverlaySectionForARoleTheVendorDoesNotPlayIsLeftOut(t *testing.T) {
	src := mustParseOverlay(t)
	out, err := Render(src, Target{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared", "author"}}, SourcePath)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "## Reviewing here") {
		t.Errorf("the reviewer overlay reached a file that does not play the role:\n%s", out)
	}
}

func TestOverlayTextIsNotPathRewritten(t *testing.T) {
	// The overlay is the repository's text about its own layout. Its paths are
	// already correct, so a rewrite could only damage them.
	src := mustParseOverlay(t)
	out, err := Render(src, Target{
		Name: "claude", File: "CLAUDE.md", Roles: []string{"shared", "author"},
		PathPrefix: ".standards/docs/standards",
	}, vendoredPath)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "`docs/standards/toolchain.md`") {
		t.Errorf("the overlay's own path was rewritten:\n%s", out)
	}
}

func TestTheOverlayMayDeclareARoleTheSourceDoesNot(t *testing.T) {
	src := mustParse(t)
	overlay, err := Parse("<!-- mf:role project -->\n## Project\n\nLocal only.\n")
	if err != nil {
		t.Fatal(err)
	}
	src.Overlay = overlay.Sections
	out, err := Render(src, Target{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared", "project"}}, SourcePath)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "Local only.") {
		t.Errorf("a role only the overlay declares was refused or dropped:\n%s", out)
	}
}

// overlayFixture is the sync fixture with a repository-owned overlay beside the
// source.
func overlayFixture(t *testing.T) Options {
	t.Helper()
	o := fixture(t)
	o.OverlayPath = "docs/agents/project.md"
	if err := os.WriteFile(filepath.Join(o.RepoRoot, filepath.FromSlash(o.OverlayPath)), []byte(overlaySource), 0o644); err != nil {
		t.Fatal(err)
	}
	return o
}

func TestSyncWritesTheOverlayIntoEveryVendorFile(t *testing.T) {
	o := overlayFixture(t)
	if _, err := Sync(o); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	claude, err := os.ReadFile(filepath.Join(o.RepoRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(claude), "## This project") {
		t.Errorf("CLAUDE.md carries no overlay:\n%s", claude)
	}
	codex, err := os.ReadFile(filepath.Join(o.RepoRoot, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(codex), "## Reviewing here") {
		t.Errorf("AGENTS.md carries no overlay:\n%s", codex)
	}
}

func TestAConfiguredOverlayThatCannotBeReadFailsSyncAndCheck(t *testing.T) {
	// A silently dropped overlay is a file that looks complete and has lost
	// exactly the obligations nothing else carries.
	o := fixture(t)
	o.OverlayPath = "docs/agents/project.md"
	if _, err := Sync(o); err == nil {
		t.Error("Sync accepted a configured overlay it could not read")
	}
	if _, err := Check(o); err == nil {
		t.Error("Check accepted a configured overlay it could not read")
	}
}

func TestCheckReportsDriftWhenTheOverlayChangedAndSyncWasNotRun(t *testing.T) {
	o := overlayFixture(t)
	if _, err := Sync(o); err != nil {
		t.Fatal(err)
	}
	edited := overlaySource + "\nOne more obligation.\n"
	if err := os.WriteFile(filepath.Join(o.RepoRoot, filepath.FromSlash(o.OverlayPath)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := Check(o)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	drifted := false
	for _, r := range results {
		if r.Drifted {
			drifted = true
		}
	}
	if !drifted {
		t.Error("an edited overlay was not reported as drift")
	}
}

func TestNoOverlayConfiguredGeneratesTheSameFile(t *testing.T) {
	// The key is opt-in: a repository that declares none must generate exactly
	// what it generated before the key existed.
	src := mustParse(t)
	want, err := Render(src, Target{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared", "author"}}, SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	src.Overlay = nil
	got, err := Render(src, Target{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared", "author"}}, SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("an empty overlay changed the output:\n%s", got)
	}
}

func TestTheHeaderNamesBothSourcesWhenAnOverlayWasUsed(t *testing.T) {
	// With an overlay there are two sources, and in the layout this matters for
	// the first is inside a submodule the reader does not own. A header naming
	// only that one tells a contributor to edit the single file they must not
	// touch.
	src := mustParseOverlay(t)
	src.OverlayPath = "docs/agents/project.md"
	out, err := Render(src, Target{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared", "author"}}, vendoredPath)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	head := out[:strings.Index(out, "-->")]
	for _, want := range []string{vendoredPath, "docs/agents/project.md"} {
		if !strings.Contains(head, want) {
			t.Errorf("the header does not name %s:\n%s", want, head)
		}
	}
	if !strings.Contains(head, "belongs to") {
		t.Errorf("the header still says to edit \"the source\", which of the two is not said:\n%s", head)
	}
}

func TestTheHeaderIsUnchangedWhenNoOverlayIsConfigured(t *testing.T) {
	// A repository that declares no overlay must not see its generated files
	// churn.
	src := mustParse(t)
	out, err := Render(src, Target{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared"}}, SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	want := "<!-- Generated for claude from " + SourcePath + " by `mf agents sync`.\n" +
		"     Do not edit this file: edit the source and re-run. `mf check agents`\n" +
		"     fails when a generated file and its source have drifted apart. -->\n"
	if !strings.HasPrefix(out, want) {
		t.Errorf("the header changed for a file generated without an overlay:\n%s", out[:len(want)+40])
	}
}

func TestSyncNamesTheOverlayItRead(t *testing.T) {
	// The path has to reach the header from the loader, not from a test that
	// sets the field by hand.
	o := overlayFixture(t)
	if _, err := Sync(o); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	generated, err := os.ReadFile(filepath.Join(o.RepoRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), o.OverlayPath) {
		t.Errorf("the generated header does not name the overlay it was given:\n%s", generated)
	}
}

func TestRenderRefusesAPathThatWouldCloseTheHeaderComment(t *testing.T) {
	// Both paths are written into an HTML comment. One carrying `-->` closes it
	// early and spills the rest of the header into the document as content.
	src := mustParseOverlay(t)
	src.OverlayPath = "docs/agents/oops-->.md"
	if _, err := Render(src, Target{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared"}}, SourcePath); err == nil {
		t.Error("an overlay path containing `-->` was accepted")
	} else if !strings.Contains(err.Error(), "agents_overlay") {
		t.Errorf("the error does not name the key at fault: %v", err)
	}

	clean := mustParse(t)
	if _, err := Render(clean, Target{Name: "claude", File: "CLAUDE.md", Roles: []string{"shared"}}, "docs/agents/-->.md"); err == nil {
		t.Error("a source path containing `-->` was accepted")
	}
}
