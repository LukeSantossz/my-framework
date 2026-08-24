// Package design reads the visual identity out of docs/standards/design.md and
// checks the surfaces this framework renders against it.
//
// The vocabulary is parsed from the standard rather than restated in Go, for the
// reason recorded in docs/adr/0009-deterministic-checks-derive-from-standards.md:
// a second copy drifts from the document that owns it and agrees with itself
// while doing so. A standard this package cannot parse is a hard error, never an
// empty palette — an empty palette makes every surface pass, which is a gate
// reporting success exactly when it stopped checking.
package design

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// StandardFile is where the identity lives.
const StandardFile = "docs/standards/design.md"

// Marker precedes the fenced block carrying the tokens. It is an HTML comment
// for the same reason the instruction source uses one: it is invisible in a
// rendered document and unambiguous to a parser.
const Marker = "<!-- mf:design tokens -->"

// NeutralLimit is how far a non-semantic colour may sit from grey, measured as
// the spread between its strongest and weakest channel. Warmth is the identity
// here, so the limit is not zero; an accent is what it exists to catch.
const NeutralLimit = 32

// Token is one colour role in both polarities. Neither is the default with the
// other as an afterthought, so both are required.
type Token struct {
	Name     string
	Role     string `toml:"role"`
	Light    string `toml:"light"`
	Dark     string `toml:"dark"`
	Semantic bool   `toml:"semantic"`
}

// Source records what the identity was derived from, and the fingerprints that
// let the derivation be checked without reproducing the source's values.
type Source struct {
	Name         string   `toml:"name"`
	Read         string   `toml:"read"`
	Algorithm    string   `toml:"algorithm"`
	Fingerprints []string `toml:"fingerprints"`
}

// Palette is the parsed standard.
type Palette struct {
	Colors    map[string]Token
	Typefaces map[string]string
	Scale     map[string]string
	Source    Source
}

// Violation is one thing the standard forbids, and where it is.
type Violation struct {
	File   string
	Line   int
	Value  string
	Reason string
}

func (v Violation) String() string {
	where := v.File
	if v.Line > 0 {
		where = fmt.Sprintf("%s:%d", v.File, v.Line)
	}
	return fmt.Sprintf("%s: %s: %s", where, v.Value, v.Reason)
}

type document struct {
	Color     map[string]Token  `toml:"color"`
	Typeface  map[string]string `toml:"typeface"`
	Scale     map[string]string `toml:"scale"`
	SourceRef Source            `toml:"source"`
}

// Load reads the standard from a repository.
func Load(repoRoot string) (Palette, error) {
	path := filepath.Join(repoRoot, filepath.FromSlash(StandardFile))
	body, err := os.ReadFile(path)
	if err != nil {
		return Palette{}, fmt.Errorf("cannot read %s: %w", StandardFile, err)
	}
	p, err := Parse(string(body))
	if err != nil {
		return Palette{}, fmt.Errorf("%s: %w", StandardFile, err)
	}
	return p, nil
}

// Parse extracts the token block. Every failure is an error: this is the one
// place where being lenient would turn the gate off without saying so.
func Parse(body string) (Palette, error) {
	at := strings.Index(body, Marker)
	if at < 0 {
		return Palette{}, fmt.Errorf("no %s marker; the token block cannot be located", Marker)
	}
	block, err := fencedBlock(body[at+len(Marker):])
	if err != nil {
		return Palette{}, err
	}

	var doc document
	if _, err := toml.Decode(block, &doc); err != nil {
		return Palette{}, fmt.Errorf("the token block is not valid TOML: %w", err)
	}
	if len(doc.Color) == 0 {
		return Palette{}, fmt.Errorf("the token block declares no colours")
	}

	p := Palette{
		Colors:    make(map[string]Token, len(doc.Color)),
		Typefaces: doc.Typeface,
		Scale:     doc.Scale,
		Source:    doc.SourceRef,
	}
	for name, token := range doc.Color {
		if token.Light == "" || token.Dark == "" {
			return Palette{}, fmt.Errorf("colour %q is missing a value for %s; a role defined in one polarity looks decided in one theme and accidental in the other",
				name, missingPolarity(token))
		}
		token.Name = name
		p.Colors[name] = token
	}
	return p, nil
}

func missingPolarity(t Token) string {
	if t.Light == "" && t.Dark == "" {
		return "either polarity"
	}
	if t.Light == "" {
		return "light"
	}
	return "dark"
}

func fencedBlock(after string) (string, error) {
	lines := strings.Split(after, "\n")
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			start = i + 1
			break
		}
		return "", fmt.Errorf("the %s marker is not followed by a fenced block", Marker)
	}
	if start < 0 {
		return "", fmt.Errorf("the %s marker is not followed by a fenced block", Marker)
	}
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			return strings.Join(lines[start:i], "\n"), nil
		}
	}
	return "", fmt.Errorf("the token block is never closed")
}

// --- neutrality -------------------------------------------------------------

// Chroma is how far a colour sits from grey: the spread between its strongest
// and weakest channel. It measures colourfulness rather than darkness, so a
// warm near-black and a warm near-white both read as neutral.
func Chroma(value string) int {
	r, g, b, ok := rgb(value)
	if !ok {
		return 0
	}
	high, low := r, r
	for _, c := range []int{g, b} {
		if c > high {
			high = c
		}
		if c < low {
			low = c
		}
	}
	return high - low
}

// CheckNeutrality enforces the rule that carries this project's own premise: a
// tool whose provider is configuration must not wear a vendor's accent colour.
// A colour declared semantic is exempt, because reporting a result is not
// branding — but it may never be a state's only signal, which is prose in the
// standard rather than something this can measure.
func (p Palette) CheckNeutrality() []Violation {
	var out []Violation
	for _, name := range p.sortedColors() {
		token := p.Colors[name]
		if token.Semantic {
			continue
		}
		for polarity, value := range map[string]string{"light": token.Light, "dark": token.Dark} {
			if c := Chroma(value); c > NeutralLimit {
				out = append(out, Violation{
					File:  StandardFile,
					Value: fmt.Sprintf("color.%s.%s = %s", name, polarity, value),
					Reason: fmt.Sprintf("chroma %d exceeds the neutral limit of %d and is not declared semantic; this palette has no accent colour",
						c, NeutralLimit),
				})
			}
		}
	}
	sortViolations(out)
	return out
}

// --- derivation -------------------------------------------------------------

// Fingerprint is the one-way digest the standard records for a source value.
// Values are normalised first, or `#F7F5F0` walks straight past the check that
// exists to catch it.
func Fingerprint(value string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(sum[:])[:16]
}

// CheckDerivation refuses any declared value matching a recorded fingerprint of
// the source's identity-carrying values.
//
// It proves exactly one thing: the source's literal colours and typefaces are
// not in this document. It cannot prove independence of design — direction is
// not a value, and a restraint cannot be fingerprinted.
func (p Palette) CheckDerivation() []Violation {
	if len(p.Source.Fingerprints) == 0 {
		return []Violation{{
			File:   StandardFile,
			Value:  "source.fingerprints",
			Reason: "no fingerprints recorded, so a derived identity cannot be distinguished from a copied one",
		}}
	}
	recorded := make(map[string]bool, len(p.Source.Fingerprints))
	for _, f := range p.Source.Fingerprints {
		recorded[strings.ToLower(strings.TrimSpace(f))] = true
	}

	var out []Violation
	check := func(label, value string) {
		if value == "" {
			return
		}
		if recorded[Fingerprint(value)] {
			out = append(out, Violation{
				File:   StandardFile,
				Value:  fmt.Sprintf("%s = %s", label, value),
				Reason: "matches a recorded fingerprint of the source entry; the identity is derived, never copied",
			})
		}
	}
	for _, name := range p.sortedColors() {
		token := p.Colors[name]
		check("color."+name+".light", token.Light)
		check("color."+name+".dark", token.Dark)
	}
	for _, role := range sortedKeys(p.Typefaces) {
		// A stack is checked family by family: reusing one face inside a longer
		// list is still reusing it.
		for _, family := range strings.Split(p.Typefaces[role], ",") {
			check("typeface."+role, strings.Trim(strings.TrimSpace(family), `"'`))
		}
	}
	sortViolations(out)
	return out
}

// --- surfaces ---------------------------------------------------------------

// colourLiteral finds the two forms this repository writes colours in. A named
// CSS colour or an oklch() call would pass unnoticed, which is why the standard
// forbids those forms rather than this pretending to catch them.
var colourLiteral = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|rgba?\([^)]*\)`)

// CheckSurface reports every colour in a rendered surface that the standard does
// not declare. A colour nobody declared is a colour nobody decided.
func (p Palette) CheckSurface(file, body string) []Violation {
	declared := map[string]bool{}
	for _, token := range p.Colors {
		declared[normalizeColour(token.Light)] = true
		declared[normalizeColour(token.Dark)] = true
	}

	var out []Violation
	for i, line := range strings.Split(body, "\n") {
		for _, match := range colourLiteral.FindAllString(line, -1) {
			if declared[normalizeColour(match)] {
				continue
			}
			out = append(out, Violation{
				File: file, Line: i + 1, Value: match,
				Reason: "not declared in " + StandardFile,
			})
		}
	}
	return out
}

// normalizeColour reduces the shapes a colour can be written in to one, so that
// writing it differently is not a way around the gate.
func normalizeColour(value string) string {
	r, g, b, ok := rgb(value)
	if !ok {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func rgb(value string) (int, int, int, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(value, "#") {
		digits := value[1:]
		if len(digits) == 3 {
			digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
		}
		if len(digits) != 6 && len(digits) != 8 {
			return 0, 0, 0, false
		}
		n, err := strconv.ParseUint(digits[:6], 16, 32)
		if err != nil {
			return 0, 0, 0, false
		}
		return int(n >> 16 & 0xff), int(n >> 8 & 0xff), int(n & 0xff), true
	}
	if !strings.HasPrefix(value, "rgb") {
		return 0, 0, 0, false
	}
	open := strings.Index(value, "(")
	closing := strings.Index(value, ")")
	if open < 0 || closing < open {
		return 0, 0, 0, false
	}
	parts := strings.FieldsFunc(value[open+1:closing], func(r rune) bool {
		return r == ',' || r == ' ' || r == '/'
	})
	if len(parts) < 3 {
		return 0, 0, 0, false
	}
	var channels [3]int
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil {
			return 0, 0, 0, false
		}
		channels[i] = n
	}
	return channels[0], channels[1], channels[2], true
}

func (p Palette) sortedColors() []string { return sortedKeys(p.Colors) }

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortViolations(v []Violation) {
	sort.Slice(v, func(i, j int) bool {
		if v[i].File != v[j].File {
			return v[i].File < v[j].File
		}
		if v[i].Line != v[j].Line {
			return v[i].Line < v[j].Line
		}
		return v[i].Value < v[j].Value
	})
}
