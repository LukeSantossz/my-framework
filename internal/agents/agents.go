// Package agents generates each vendor's instruction file from one source.
//
// The files are not the same content in different formats: they carry different
// roles. What an agent must do when it authors a change and what it must do when
// it reviews one are different obligations, and every vendor file was
// hand-maintaining its own copy of whichever it needed. So the source is marked
// by role, and each vendor file receives only the roles that vendor plays.
//
// The source is a separate file rather than AGENTS.md, because AGENTS.md is
// itself an output: it is the file an agentic reviewer finds at the repository
// root, and a file cannot be both the source and one of the things generated
// from it.
//
// The outputs stay committed. A cloned repository must present CLAUDE.md to a
// session that starts before anyone runs a command, so generating at read time
// would mean a standard that never activates — the exact Gap this framework
// exists to close.
package agents

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SourcePath is where the marked-up instructions live.
const SourcePath = "docs/agents/instructions.md"

// DefaultPathPrefix is where a repository keeps its standards. A submodule
// consumer overrides it, because its CLAUDE.md must point into `.standards/`.
const DefaultPathPrefix = "docs/standards"

// DefaultAgentDir is where the skill documents the source references live in the
// shipped layout. A submodule consumer's generated file must point inside the
// submodule instead, or the skills it names resolve to nothing there.
const DefaultAgentDir = "docs/agents"

var roleMarker = regexp.MustCompile(`(?m)^<!--\s*mf:role\s+([a-z-]+)\s*-->\s*$`)

// Section is one role-tagged part of the source.
type Section struct {
	Role string
	Body string
}

// Source is the parsed instruction source: a preamble every file carries, then
// the role-tagged sections in the order they were written.
type Source struct {
	Preamble string
	Sections []Section
}

// Parse splits the source on its role markers.
func Parse(body string) (Source, error) {
	locs := roleMarker.FindAllStringSubmatchIndex(body, -1)
	if len(locs) == 0 {
		return Source{}, fmt.Errorf("no `<!-- mf:role ... -->` markers in the source; without them nothing can be assigned to a vendor")
	}
	src := Source{Preamble: strings.TrimSpace(body[:locs[0][0]])}
	for i, loc := range locs {
		role := body[loc[2]:loc[3]]
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		src.Sections = append(src.Sections, Section{
			Role: role,
			Body: strings.TrimSpace(body[loc[1]:end]),
		})
	}
	return src, nil
}

// Roles lists every role the source declares, sorted.
func (s Source) Roles() []string {
	seen := map[string]bool{}
	var out []string
	for _, sec := range s.Sections {
		if !seen[sec.Role] {
			seen[sec.Role] = true
			out = append(out, sec.Role)
		}
	}
	sort.Strings(out)
	return out
}

// Target is one vendor file to generate.
type Target struct {
	Name       string
	File       string
	Roles      []string
	PathPrefix string
}

// Render composes one vendor file. Unknown roles are an error rather than an
// empty section, because a typo would otherwise produce a file quietly missing
// the obligations it was supposed to carry.
func Render(src Source, t Target, sourcePath string) (string, error) {
	declared := map[string]bool{}
	for _, r := range src.Roles() {
		declared[r] = true
	}
	for _, r := range t.Roles {
		if !declared[r] {
			return "", fmt.Errorf("agent %q asks for role %q, which the source does not declare (it has: %s)",
				t.Name, r, strings.Join(src.Roles(), ", "))
		}
	}
	wanted := map[string]bool{}
	for _, r := range t.Roles {
		wanted[r] = true
	}

	prefix := t.PathPrefix
	if prefix == "" {
		prefix = DefaultPathPrefix
	}

	var b strings.Builder
	if src.Preamble != "" {
		b.WriteString("\n")
		b.WriteString(src.Preamble)
		b.WriteString("\n")
	}
	for _, sec := range src.Sections {
		if !wanted[sec.Role] {
			continue
		}
		b.WriteString("\n")
		b.WriteString(sec.Body)
		b.WriteString("\n")
	}
	// The rewrites reach the body alone. The header names the configured source,
	// which in a vendored layout already carries the prefix, and rewriting the
	// finished string would double it.
	body := b.String()
	if prefix != DefaultPathPrefix {
		body = strings.ReplaceAll(body, DefaultPathPrefix+"/", prefix+"/")
	}
	if dir := agentDocDir(sourcePath); dir != DefaultAgentDir {
		body = strings.ReplaceAll(body, DefaultAgentDir+"/", dir+"/")
	}
	return header(t.Name, sourcePath) + body, nil
}

// agentDocDir is where the skill documents the source names actually live. They
// ship beside the source — the framework writes `docs/agents/domain.md` next to
// `docs/agents/instructions.md`, and a submodule delivers the whole directory —
// so the source a repository configured is what says where they are. Deriving
// it beats a second key: two ways to state one fact is two ways to disagree.
func agentDocDir(sourcePath string) string {
	return path.Dir(filepath.ToSlash(sourcePath))
}

// header names the document this file was generated from, so the reader is
// sent to the file they must actually edit. It takes the path rather than
// reading the constant: a repository that vendors the source keeps it inside
// the submodule, and naming the shipped layout there points at nothing.
func header(name, sourcePath string) string {
	return fmt.Sprintf(
		"<!-- Generated for %s from %s by `mf agents sync`.\n"+
			"     Do not edit this file: edit the source and re-run. `mf check agents`\n"+
			"     fails when a generated file and its source have drifted apart. -->\n",
		name, sourcePath)
}

// Result is what sync or check found for one target.
type Result struct {
	Target  string
	File    string
	Changed bool
	Drifted bool
}

type Options struct {
	RepoRoot string
	Targets  []Target

	// SourcePath is where the marked-up instructions live, as configured.
	// Empty takes the layout this framework ships with. It is a parameter for
	// the reason the standards directory became one: a repository that vendors
	// these documents as a submodule keeps the source inside it, and generating
	// from a path that only resolves here left `mf agents sync` as the one
	// command such a repository could not run.
	SourcePath string
}

// sourcePath resolves the configured source, or the shipped layout.
func (o Options) sourcePath() string {
	if o.SourcePath != "" {
		return o.SourcePath
	}
	return SourcePath
}

func load(o Options) (Source, error) {
	src := o.sourcePath()
	body, err := os.ReadFile(filepath.Join(o.RepoRoot, filepath.FromSlash(src)))
	if err != nil {
		return Source{}, fmt.Errorf("cannot read %s: %w", src, err)
	}
	return Parse(string(body))
}

// Sync writes every target, leaving one already correct untouched.
func Sync(o Options) ([]Result, error) {
	src, err := load(o)
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, t := range o.Targets {
		rendered, err := Render(src, t, o.sourcePath())
		if err != nil {
			return results, err
		}
		path := filepath.Join(o.RepoRoot, filepath.FromSlash(t.File))
		existing, readErr := os.ReadFile(path)
		if readErr == nil && normalize(string(existing)) == normalize(rendered) {
			results = append(results, Result{Target: t.Name, File: t.File})
			continue
		}
		// A vendor whose instructions live under a directory of its own — the
		// `.github/` layouts — got "cannot find the path" and no file, because
		// nothing created it. The path is already contained to the repository
		// by the loader, so this creates only inside it.
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return results, err
		}
		if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
			return results, err
		}
		results = append(results, Result{Target: t.Name, File: t.File, Changed: true})
	}
	return results, nil
}

// Check reports drift without writing. It is what keeps the generated files
// from becoming a convention people bypass by editing the output.
func Check(o Options) ([]Result, error) {
	src, err := load(o)
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, t := range o.Targets {
		rendered, err := Render(src, t, o.sourcePath())
		if err != nil {
			return results, err
		}
		path := filepath.Join(o.RepoRoot, filepath.FromSlash(t.File))
		existing, readErr := os.ReadFile(path)
		drifted := readErr != nil || normalize(string(existing)) != normalize(rendered)
		results = append(results, Result{Target: t.Name, File: t.File, Drifted: drifted})
	}
	return results, nil
}

// normalize absorbs the line-ending difference a Windows checkout introduces,
// so a clone is not reported as drifted from the source it matches.
func normalize(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}
