package design

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const standard = `# Design Standard

Prose the parser must step over.

<!-- mf:design tokens -->` + "```" + `toml
[color.canvas]
role  = "the page ground"
light = "#faf8f4"
dark  = "#1a1815"

[color.ink]
role  = "primary text"
light = "#1c1a17"
dark  = "#f2eee6"

[color.correct]
role  = "a quiz answer that was right"
semantic = true
light = "#3f6b4a"
dark  = "#7fae89"

[typeface]
prose = "ui-serif, Georgia, serif"

[scale]
radius-lg = "6px"

[source]
name = "some/entry"
read = "2026-08-24"
fingerprints = ["d474b848022c7e18"]
` + "```" + `

More prose.
`

func parsed(t *testing.T) Palette {
	t.Helper()
	p, err := Parse(standard)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return p
}

func TestParsesEveryDeclaredTokenOutOfTheStandard(t *testing.T) {
	// The vocabulary lives in the document that owns it, per
	// docs/adr/0009-checks-derive-vocabularies-from-standards.md. A second copy
	// in Go would drift from the standard and agree with itself while doing it.
	p := parsed(t)
	if len(p.Colors) != 3 {
		t.Fatalf("got %d colours, want 3: %+v", len(p.Colors), p.Colors)
	}
	canvas, ok := p.Colors["canvas"]
	if !ok {
		t.Fatal("the canvas role was not parsed")
	}
	if canvas.Light != "#faf8f4" || canvas.Dark != "#1a1815" || canvas.Role == "" {
		t.Errorf("canvas = %+v", canvas)
	}
	if p.Typefaces["prose"] == "" {
		t.Error("the prose typeface was not parsed")
	}
	if p.Scale["radius-lg"] != "6px" {
		t.Errorf("scale = %+v", p.Scale)
	}
	if len(p.Source.Fingerprints) != 1 || p.Source.Read == "" {
		t.Errorf("source = %+v; without it the derivation cannot be checked", p.Source)
	}
}

func TestFailsHardWhenTheStandardCannotBeParsedRatherThanReportingAnEmptyPalette(t *testing.T) {
	// An empty vocabulary makes every surface pass, which is the failure mode a
	// gate must never have: it would report success precisely when it had
	// stopped checking.
	for name, body := range map[string]string{
		"no marker":      "# Design\n\n```toml\n[color.ink]\nlight = \"#000000\"\n```\n",
		"no fence":       "<!-- mf:design tokens -->\n\nnothing follows\n",
		"broken toml":    "<!-- mf:design tokens -->\n```toml\n[color.ink\nlight =\n```\n",
		"no colours":     "<!-- mf:design tokens -->\n```toml\n[scale]\nradius-lg = \"6px\"\n```\n",
		"empty document": "",
	} {
		if _, err := Parse(body); err == nil {
			t.Errorf("%s: parsed without error, so the palette would silently be empty", name)
		}
	}
}

func TestCarriesAValueForBothPolaritiesOfEveryColourRole(t *testing.T) {
	// A role defined in one polarity only is a page that looks decided in one
	// theme and accidental in the other.
	body := strings.Replace(standard, `dark  = "#1a1815"`, `dark  = ""`, 1)
	if _, err := Parse(body); err == nil {
		t.Fatal("a colour with no dark value was accepted")
	}
}

// --- neutrality -------------------------------------------------------------

func TestDeclaresNoChromaticAccent(t *testing.T) {
	// The rule that carries the project's own premise: a tool whose providers
	// are configuration must not wear one vendor's accent.
	if v := parsed(t).CheckNeutrality(); len(v) != 0 {
		t.Errorf("the shipped palette is not neutral: %+v", v)
	}
}

func TestASemanticColourIsExemptFromNeutralityAndANonSemanticOneIsNot(t *testing.T) {
	// `correct` is chromatic on purpose and declared semantic; the same value
	// under a non-semantic role is an accent by another name.
	p := parsed(t)
	green := p.Colors["correct"]
	if Chroma(green.Light) <= NeutralLimit {
		t.Fatalf("the fixture's semantic colour is not chromatic enough to test the rule: %d", Chroma(green.Light))
	}
	p.Colors["brand"] = Token{Name: "brand", Light: green.Light, Dark: green.Dark}
	violations := p.CheckNeutrality()
	// Both polarities are reported. A token fixed in one theme and left
	// chromatic in the other is still an accent half the time, so naming only
	// the first would send someone back for a second round.
	if len(violations) != 2 {
		t.Fatalf("a chromatic non-semantic token was allowed: %+v", violations)
	}
	reported := violations[0].Value + " " + violations[1].Value
	for _, want := range []string{green.Light, green.Dark, "brand"} {
		if !strings.Contains(reported, want) {
			t.Errorf("the violations omit %q: %+v", want, violations)
		}
	}
	if strings.Contains(reported, "correct") {
		t.Error("the semantic token was flagged; reporting a result is not branding")
	}
}

func TestChromaMeasuresDistanceFromNeutralAndNotDarkness(t *testing.T) {
	if Chroma("#000000") != 0 || Chroma("#ffffff") != 0 {
		t.Error("a pure grey was reported as chromatic")
	}
	if Chroma("#1a1815") > NeutralLimit {
		t.Error("a warm near-black was reported as an accent; warmth is the identity here")
	}
	if Chroma("#2f5d9e") <= NeutralLimit {
		t.Error("a blue accent was reported as neutral")
	}
}

// --- derivation -------------------------------------------------------------

func TestFailsADeclaredTokenMatchingARecordedSourceFingerprint(t *testing.T) {
	// This is what makes "derived, not copied" a property rather than a claim.
	p := parsed(t)
	if v := p.CheckDerivation(); len(v) != 0 {
		t.Fatalf("the shipped palette already collides with the source: %+v", v)
	}
	// "#f7f5f0" is the value behind the fixture's single fingerprint.
	p.Colors["ink"] = Token{Name: "ink", Light: "#f7f5f0", Dark: "#1a1815"}
	violations := p.CheckDerivation()
	if len(violations) != 1 {
		t.Fatalf("a copied source value passed: %+v", violations)
	}
	if !strings.Contains(violations[0].Value, "#f7f5f0") || !strings.Contains(violations[0].Reason, "source") {
		t.Errorf("the violation does not name the value or why it is one: %+v", violations[0])
	}
}

func TestFingerprintIsCaseAndWhitespaceInsensitive(t *testing.T) {
	// Otherwise `#F7F5F0` walks straight past the check that exists to catch it.
	if Fingerprint("  #F7F5F0 ") != Fingerprint("#f7f5f0") {
		t.Error("the same value fingerprinted differently depending on how it was written")
	}
}

// --- the surface ------------------------------------------------------------

func TestFailsASurfaceUsingAColourTheStandardDoesNotDeclare(t *testing.T) {
	p := parsed(t)
	css := "body { background: #faf8f4; color: #1c1a17; }\n.cta { border: 1px solid #2f5d9e; }\n"
	violations := p.CheckSurface("render.go", css)
	if len(violations) != 1 {
		t.Fatalf("violations = %+v, want exactly the undeclared colour", violations)
	}
	if violations[0].Value != "#2f5d9e" {
		t.Errorf("the wrong value was reported: %+v", violations[0])
	}
	if violations[0].File != "render.go" || violations[0].Line != 2 {
		t.Errorf("the violation does not say where it is: %+v", violations[0])
	}
}

func TestAnRgbLiteralIsCaughtToo(t *testing.T) {
	// Writing the same colour a different way must not be a way around the gate.
	violations := parsed(t).CheckSurface("x.css", ".a { color: rgb(47, 93, 158); }")
	if len(violations) != 1 {
		t.Errorf("an rgb() literal passed unnoticed: %+v", violations)
	}
}

func TestADeclaredColourIsAcceptedInAnyCase(t *testing.T) {
	if v := parsed(t).CheckSurface("x.css", ".a { color: #FAF8F4; }"); len(v) != 0 {
		t.Errorf("a declared colour was rejected for its spelling: %+v", v)
	}
}

// --- the shipped standard ---------------------------------------------------

func TestTheShippedStandardLoadsAndGovernsTheExplainer(t *testing.T) {
	// The standard is content, and content rots. This guards that what ships
	// parses, is neutral, does not reuse the source, and actually matches the
	// one surface it binds.
	p, err := Load(filepath.Join("..", ".."), "")
	if err != nil {
		t.Fatalf("the shipped standard does not load: %v", err)
	}
	if v := p.CheckNeutrality(); len(v) != 0 {
		t.Errorf("the shipped palette declares a chromatic accent: %+v", v)
	}
	if v := p.CheckDerivation(); len(v) != 0 {
		t.Errorf("the shipped palette reuses a source value: %+v", v)
	}
	for _, want := range []string{"canvas", "canvas-soft", "hairline", "ink", "body", "mute"} {
		if _, ok := p.Colors[want]; !ok {
			t.Errorf("the standard no longer declares the %q role", want)
		}
	}
	if p.Source.Read == "" || p.Source.Name == "" {
		t.Error("the standard does not record what it was derived from or when")
	}
}

func TestParsesAStandardCheckedOutWithWindowsLineEndings(t *testing.T) {
	// A standard is a Markdown file in a git repository, and git hands a Windows
	// checkout CRLF by default. The TOML decoder refuses a carriage return as a
	// control character, so a parser that does not normalise first fails on the
	// same document it just parsed on another machine.
	crlf := strings.ReplaceAll(standard, "\n", "\r\n")
	p, err := Parse(crlf)
	if err != nil {
		t.Fatalf("a CRLF checkout does not parse: %v", err)
	}
	if len(p.Colors) != 3 || p.Colors["canvas"].Light != "#faf8f4" {
		t.Errorf("the CRLF parse lost content: %+v", p.Colors)
	}
}

// --- where the standard lives -----------------------------------------------

func TestTheStandardIsReadWhereTheRepositoryKeepsIt(t *testing.T) {
	// A repository that vendors this framework as a `.standards` submodule
	// keeps design.md under it. Reading only `docs/standards` fails that
	// repository on a document it has.
	root := t.TempDir()
	vendored := ".standards/docs/standards"
	dir := filepath.Join(root, filepath.FromSlash(vendored))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, StandardFileName), []byte(standard), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Load(root, vendored)
	if err != nil {
		t.Fatalf("the vendored standard did not load: %v", err)
	}
	if len(p.Colors) != 3 {
		t.Errorf("got %d colours, want the 3 the document declares", len(p.Colors))
	}

	// A violation has to point at the document that has to change, which is the
	// one that was read rather than the one at the default location.
	v := p.CheckSurface("x.css", ".a { color: #2f5d9e; }")
	if len(v) != 1 {
		t.Fatalf("got %d violations, want 1: %+v", len(v), v)
	}
	if !strings.Contains(v[0].Reason, vendored+"/"+StandardFileName) {
		t.Errorf("the violation names %q rather than the document it was read from", v[0].Reason)
	}

	// And the directory is what did it.
	if _, err := Load(root, ""); err == nil {
		t.Error("the default location loaded a standard that is not there")
	}
}
