package check

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/LukeSantossz/my-framework/internal/vcs"
)

type Problem struct {
	File    string
	Message string
}

type Result struct {
	Name     string
	Problems []Problem
	Note     string
}

func (r Result) OK() bool { return len(r.Problems) == 0 }

// Where a repository following this framework keeps the documents these gates
// read, relative to its root.
//
// They are defaults rather than constants the gates use directly, for the
// reason agents.DefaultPathPrefix already records for the generated instruction
// files: a repository that vendors this framework as a `.standards` submodule
// keeps the same documents under it, and a gate that can only read
// `docs/standards` is a gate that adopter cannot run at all.
const (
	DefaultStandardsDir = "docs/standards"
	DefaultSpecsDir     = "docs/specs"
	DefaultADRDir       = "docs/adr"
)

type Options struct {
	RepoRoot string

	// The document directories, each absolute or relative to RepoRoot, so a
	// caller can hand a configured value straight through without knowing
	// which of the two it holds. Empty means the default above.
	StandardsDir string
	SpecsDir     string
	ADRDir       string

	Base        string
	ExemptPaths []string
	Repo        *vcs.Repo
}

// Defaults fills the paths a repository following this framework uses and
// resolves the configured ones against its root.
func (o Options) Defaults() Options {
	o.StandardsDir = o.resolve(o.StandardsDir, DefaultStandardsDir)
	o.SpecsDir = o.resolve(o.SpecsDir, DefaultSpecsDir)
	o.ADRDir = o.resolve(o.ADRDir, DefaultADRDir)
	if o.Base == "" {
		o.Base = "main"
	}
	if o.Repo == nil {
		o.Repo = vcs.Open(o.RepoRoot)
	}
	return o
}

func (o Options) resolve(dir, fallback string) string {
	if dir == "" {
		dir = fallback
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return filepath.Join(o.RepoRoot, filepath.FromSlash(dir))
}

// specsPrefix is where specs live as git names them: a slash path relative to
// the repository root. The changed-file list arrives from git in that form, so
// an absolute directory compared against it would match nothing and every
// branch would be reported as carrying no spec.
func (o Options) specsPrefix() string { return o.prefix(o.SpecsDir) }

// prefix is any configured directory in the form git names paths in, which is
// the form every answer git gives arrives in and every pathspec it takes has to
// be written in.
func (o Options) prefix(dir string) string {
	rel, err := filepath.Rel(o.RepoRoot, dir)
	if err != nil {
		return filepath.ToSlash(dir)
	}
	return filepath.ToSlash(rel)
}

func (o Options) read(dir, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	return string(data), err
}

var specFileName = regexp.MustCompile(`^\d{4}-[a-z0-9-]+\.md$`)

// --- spec -------------------------------------------------------------------

// Spec enforces the Spec Gate: a non-trivial change carries a spec, and that
// spec has the sections the Gate checks, a non-empty "Does NOT include", and at
// least one acceptance criterion.
func Spec(o Options) (Result, error) {
	o = o.Defaults()
	res := Result{Name: "spec"}

	doc, err := o.read(o.StandardsDir, "spec_method.md")
	if err != nil {
		return res, fmt.Errorf("cannot read spec_method.md: %w", err)
	}
	sections, err := ParseGateSections(doc)
	if err != nil {
		return res, err
	}

	head, err := o.Repo.CurrentBranch()
	if err != nil {
		return res, err
	}
	if head == o.Base {
		res.Note = "on the base branch; nothing to gate"
		return res, nil
	}
	changed, err := o.Repo.ChangedFiles(o.Base, head)
	if err != nil {
		return res, err
	}
	if len(changed) == 0 {
		res.Note = "the branch adds nothing over its base"
		return res, nil
	}

	// Triviality is an explicit path list, not a heuristic and not a model. A
	// gate nobody can predict is a gate people route around, and the exemption
	// is visible in a committed file so widening it shows up in review.
	if allExempt(changed, o.ExemptPaths) {
		res.Note = "every changed path is exempt; no spec required"
		return res, nil
	}

	specsIn := o.specsPrefix()
	var specs []string
	for _, f := range changed {
		if filepath.ToSlash(filepath.Dir(f)) == specsIn && specFileName.MatchString(filepath.Base(f)) {
			specs = append(specs, f)
		}
	}
	if len(specs) == 0 {
		res.Problems = append(res.Problems, Problem{
			Message: fmt.Sprintf("this branch changes %d non-exempt path(s) but adds no spec under %s/NNNN-<slug>.md", len(changed), specsIn),
		})
		return res, nil
	}

	for _, rel := range specs {
		body, err := os.ReadFile(filepath.Join(o.RepoRoot, rel))
		if err != nil {
			res.Problems = append(res.Problems, Problem{File: rel, Message: "cannot read the spec: " + err.Error()})
			continue
		}
		res.Problems = append(res.Problems, gateProblems(rel, string(body), sections)...)
	}
	return res, nil
}

func gateProblems(rel, body string, sections []string) []Problem {
	var problems []Problem
	for _, section := range sections {
		content, ok := sectionBody(body, "## "+section)
		if !ok {
			problems = append(problems, Problem{File: rel, Message: fmt.Sprintf("missing the %q section the Spec Gate checks", section)})
			continue
		}
		if strings.TrimSpace(content) == "" {
			problems = append(problems, Problem{File: rel, Message: fmt.Sprintf("the %q section is empty", section)})
			continue
		}
		switch section {
		case "Scope":
			if !hasDoesNotInclude(content) {
				problems = append(problems, Problem{File: rel,
					Message: `the Scope section has no non-empty "Does NOT include" list; that list is what blocks scope creep`})
			}
		case "Acceptance Criteria":
			if countCriteria(content) == 0 {
				problems = append(problems, Problem{File: rel,
					Message: "the Acceptance Criteria section states no criterion; the Gate requires at least one that is verifiable"})
			}
		}
	}
	return problems
}

func hasDoesNotInclude(scope string) bool {
	lines := strings.Split(scope, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "Does NOT include") {
			continue
		}
		// Content may follow on the same line after the colon, or on the lines
		// beneath it before the section ends.
		if _, after, found := strings.Cut(line, "Does NOT include"); found {
			if strings.TrimSpace(strings.TrimLeft(after, ":")) != "" {
				return true
			}
		}
		for _, rest := range lines[i+1:] {
			if strings.TrimSpace(rest) != "" {
				return true
			}
		}
	}
	return false
}

func countCriteria(section string) int {
	n := 0
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || orderedItem(line) {
			n++
		}
	}
	return n
}

// orderedItem reports whether a line opens an ordered list item. The Gate asks
// for criteria, not for a bullet character: a spec that numbers them stated
// them, and rejecting it with "states no criterion" is the one message that
// cannot tell its author what to change.
func orderedItem(line string) bool {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i+1 >= len(line) {
		return false
	}
	if line[i] != '.' && line[i] != ')' {
		return false
	}
	return line[i+1] == ' ' || line[i+1] == '\t'
}

// citedByURL collects the markdown names that appear inside a URL, so the
// reference scan can tell a citation from a path this repository must ship.
func citedByURL(body string) map[string]bool {
	cited := map[string]bool{}
	for _, u := range urlRef.FindAllString(body, -1) {
		for _, ref := range mdRef.FindAllString(u, -1) {
			cited[ref] = true
		}
	}
	return cited
}

func allExempt(changed, exempt []string) bool {
	if len(exempt) == 0 {
		return false
	}
	for _, f := range changed {
		matched := false
		for _, pattern := range exempt {
			if ok, _ := filepath.Match(pattern, filepath.ToSlash(f)); ok {
				matched = true
				break
			}
			if strings.HasPrefix(filepath.ToSlash(f), strings.TrimSuffix(pattern, "*")) && strings.HasSuffix(pattern, "*") {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// --- commit -----------------------------------------------------------------

var conventional = regexp.MustCompile(`^([a-z]+)(\([^)]+\))?!?: .+`)

// attribution matches the credit lines github.md forbids. A commit message
// describes the change and its intent, not that a model produced it.
var attribution = regexp.MustCompile(`(?im)^\s*co-authored-by:|generated with \[?claude|🤖 generated with`)

// generatedMerge matches the subjects git and the forges write when they join
// two histories: `Merge branch 'main' into feat/x`, `Merge pull request #15
// from owner/branch`, and the tag, commit and octopus variants of the same.
//
// It is not a vocabulary, so docs/adr/0009 does not put it in a standard: these
// strings are git's and GitHub's, not this project's, and github.md has nothing
// to say about them. It is only consulted where a parent count is unavailable —
// see CommitMessage.
var generatedMerge = regexp.MustCompile(`^Merge (branch|branches|remote-tracking branch|tag|commit|pull request) `)

// Commit checks the subjects on this branch against the Type Table.
//
// Merge commits are skipped. github.md's Type Table governs the subject an
// author writes, and a merge subject is generated — by git for `git merge`, by
// the forge for a pull request. Over this repository's own history fifteen
// `Merge pull request #N from ...` subjects fail the Conventional Commits
// shape, and every branch that merges its base back in carries one, so
// checking them would fail pull requests over text nobody typed and cannot
// rewrite. The parent count is what identifies them, rather than the wording:
// it is what git records, so it cannot be spoofed by a subject and cannot miss
// a merge whose subject was edited.
func Commit(o Options) (Result, error) {
	o = o.Defaults()
	res := Result{Name: "commit"}

	types, valid, err := o.commitVocabulary()
	if err != nil {
		return res, err
	}

	head, err := o.Repo.CurrentBranch()
	if err != nil {
		return res, err
	}
	if head == o.Base {
		res.Note = "on the base branch; nothing to check"
		return res, nil
	}
	commits, err := o.Repo.Commits(o.Base, head)
	if err != nil {
		return res, err
	}
	merges := 0
	for _, c := range commits {
		if c.Merge() {
			merges++
			continue
		}
		short := c.SHA
		if len(short) > 8 {
			short = short[:8]
		}
		res.Problems = append(res.Problems, messageProblems(short, c.Subject, c.Body, types, valid)...)
	}
	res.Note = fmt.Sprintf("%d commit(s) checked against %d types read from github.md", len(commits)-merges, len(types))
	if merges > 0 {
		res.Note += fmt.Sprintf("; %d merge commit(s) skipped, whose subjects no author wrote", merges)
	}
	return res, nil
}

// CommitMessage checks the single message in a file, which is what the
// commit-msg hook is handed as its argument.
//
// It exists because the branch mode above answers a different question at a
// later moment: it reads the commits already recorded, so a subject the Type
// Table rejects is reported one commit after the one that has to change, and
// the author has to rewrite history to fix what they were about to type. The
// vocabulary is the same and is read from the same document — per
// docs/adr/0009 there is one Type Table and the binary carries no copy of it —
// so the two modes cannot disagree about what is valid.
//
// git runs the commit-msg hook for `git merge` as well as for `git commit`,
// handing it the MERGE_MSG it wrote itself, so a generated merge subject is
// skipped here too. The parent count Commit uses is not available: the file
// holds a message, and the commit it will become does not exist yet. The
// wording is the only evidence there is, which is a weaker test — a subject
// that begins "Merge branch " is taken at its word — and it is the right side
// to be wrong on, since the alternative is a hook that blocks every merge.
func CommitMessage(o Options, path string) (Result, error) {
	o = o.Defaults()
	res := Result{Name: "commit"}

	types, valid, err := o.commitVocabulary()
	if err != nil {
		return res, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return res, fmt.Errorf("cannot read the commit message: %w", err)
	}

	where := filepath.Base(path)
	subject, body := splitMessage(string(raw))
	switch {
	case subject == "":
		res.Problems = append(res.Problems, Problem{File: where,
			Message: "the message is empty once git's own comment lines are removed, so there is no subject to check"})
	case generatedMerge.MatchString(subject):
		res.Note = fmt.Sprintf("%s carries a merge subject git generated; nothing an author wrote to check", where)
		return res, nil
	default:
		res.Problems = append(res.Problems, messageProblems(where, subject, body, types, valid)...)
	}
	res.Note = fmt.Sprintf("%s checked against %d types read from github.md", where, len(types))
	return res, nil
}

// commitVocabulary reads the Type Table both modes judge a subject against,
// returning it in both the forms they need: ordered, so a failure can list what
// is allowed in the document's own order, and indexed, so a lookup is one map
// read.
func (o Options) commitVocabulary() ([]string, map[string]bool, error) {
	doc, err := o.read(o.StandardsDir, "github.md")
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read github.md: %w", err)
	}
	types, err := ParseTypeTable(doc)
	if err != nil {
		return nil, nil, err
	}
	valid := make(map[string]bool, len(types))
	for _, t := range types {
		valid[t] = true
	}
	return types, valid, nil
}

// messageProblems is every rule github.md states about one message, applied
// once. Both modes call it so that a subject rejected at commit time is
// rejected in the same words at push time; two copies of these three rules
// would be two chances to word them differently and one chance to fix only one.
func messageProblems(where, subject, body string, types []string, valid map[string]bool) []Problem {
	var problems []Problem
	m := conventional.FindStringSubmatch(subject)
	if m == nil {
		problems = append(problems, Problem{File: where,
			Message: fmt.Sprintf("subject %q is not Conventional Commits format", subject)})
	} else if !valid[m[1]] {
		problems = append(problems, Problem{File: where,
			Message: fmt.Sprintf("type %q is absent from the Type Table in github.md (valid: %s)", m[1], strings.Join(types, ", "))})
	}
	if attribution.MatchString(subject + "\n" + body) {
		problems = append(problems, Problem{File: where,
			Message: "carries a co-author or AI-attribution line, which github.md forbids"})
	}
	return problems
}

// scissors is the line `git commit --verbose` puts above the diff it appends
// for the author to read. Everything below it is stripped before the message is
// recorded.
const scissors = "# ------------------------ >8 ------------------------"

// splitMessage reduces the file the hook is handed to the message git will
// actually record: comment lines and the verbose diff removed, then split into
// subject and body.
//
// Doing this is not optional. The file arrives before git's own cleanup, so its
// first line is usually the "Please enter the commit message" comment, and
// under `commit --verbose` it ends with a diff of the change. Checking the raw
// text would report a comment as the subject and would find a forbidden
// attribution line in any diff that happens to add one.
func splitMessage(raw string) (subject, body string) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	if at := strings.Index(raw, scissors); at >= 0 {
		raw = raw[:at]
	}
	var kept []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if len(kept) == 0 && strings.TrimSpace(line) == "" {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		return "", ""
	}
	return strings.TrimSpace(kept[0]), strings.Join(kept[1:], "\n")
}

// --- branch -----------------------------------------------------------------

func Branch(o Options) (Result, error) {
	o = o.Defaults()
	res := Result{Name: "branch"}

	doc, err := o.read(o.StandardsDir, "github.md")
	if err != nil {
		return res, fmt.Errorf("cannot read github.md: %w", err)
	}
	types, err := ParseTypeTable(doc)
	if err != nil {
		return res, err
	}
	head, err := o.Repo.CurrentBranch()
	if err != nil {
		return res, err
	}
	if head == o.Base {
		res.Note = "on the base branch"
		return res, nil
	}
	prefix, rest, found := strings.Cut(head, "/")
	if !found || rest == "" {
		res.Problems = append(res.Problems, Problem{File: head,
			Message: "branch name is not type/short-description"})
		return res, nil
	}
	for _, t := range types {
		if t == prefix {
			return res, nil
		}
	}
	res.Problems = append(res.Problems, Problem{File: head,
		Message: fmt.Sprintf("branch type %q is absent from the Type Table in github.md", prefix)})
	return res, nil
}

// --- docs -------------------------------------------------------------------

var deprecated = regexp.MustCompile(`Self-Review Checklist|author approves|only makes R2 concrete`)

// hypothetical names appear in prose rather than as links; each is listed with
// the reason it does not resolve to a file.
var hypothetical = map[string]bool{
	"CONTRIBUTING.md": true, // named by the README template as optional
	"CLAUDE.full.md":  true, // an example name in token_economy.md
	"SPEC.md":         true, // the artifact's generic name in prose
	// The name of the design-document format design.md adopts, not a file here.
	// On a case-insensitive filesystem it resolves to design.md and looks fine,
	// which is how it reached a release tag before anything noticed.
	"DESIGN.md": true,
}

var mdRef = regexp.MustCompile(`[A-Za-z0-9_./-]+\.md`)

// urlRef matches a markdown name that is part of a URL. The standards cite
// upstream documents, and a citation is not a claim that this repository ships
// the file: reading one as a path failed the gate — inside the pre-push hook —
// naming something that was never a path.
var urlRef = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s)]*\.md`)

func Docs(o Options) (Result, error) {
	o = o.Defaults()
	res := Result{Name: "docs"}

	index, err := o.read(o.StandardsDir, "INDEX.md")
	if err != nil {
		return res, fmt.Errorf("cannot read INDEX.md: %w", err)
	}
	listed := map[string]bool{}
	for _, ref := range mdRef.FindAllString(index, -1) {
		listed[filepath.Base(ref)] = true
	}

	entries, err := os.ReadDir(o.StandardsDir)
	if err != nil {
		return res, err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || name == "INDEX.md" {
			continue
		}
		if !listed[name] {
			res.Problems = append(res.Problems, Problem{File: name, Message: "not listed in INDEX.md; an orphan standard is one nobody is told to read"})
		}
		body, err := o.read(o.StandardsDir, name)
		if err != nil {
			continue
		}
		if m := deprecated.FindString(body); m != "" {
			res.Problems = append(res.Problems, Problem{File: name, Message: fmt.Sprintf("carries retired wording %q", m)})
		}
		cited := citedByURL(body)
		for _, ref := range mdRef.FindAllString(body, -1) {
			if hypothetical[ref] || cited[ref] {
				continue
			}
			if !refResolves(o, ref) {
				res.Problems = append(res.Problems, Problem{File: name, Message: "references a missing file: " + ref})
			}
		}
	}
	return res, nil
}

// refResolves reports whether a reference written in a standard names a file
// that is there. A bare name may also be a sibling standard, which is why the
// standards directory is one of the places it is looked for; a reference
// carrying a path is written against a tree's root, so only the roots answer it.
func refResolves(o Options, ref string) bool {
	rel := filepath.FromSlash(ref)
	var bases []string
	if !strings.ContainsAny(ref, "/\\") {
		bases = append(bases, o.StandardsDir)
	}
	bases = append(bases, o.RepoRoot)
	if corpus := o.corpusRoot(); corpus != filepath.Clean(o.RepoRoot) {
		bases = append(bases, corpus)
	}
	for _, base := range bases {
		if existsExactly(base, rel) {
			return true
		}
	}
	return false
}

// corpusRoot is the root the documents' own cross-references are written
// against. A standard naming `docs/standards/spec_method.md` means its sibling,
// and where that sibling is depends on where the corpus was mounted: at the
// repository root when the documents live in the repository, and at the
// submodule when they are vendored into one. Resolving only against the
// repository root reports every cross-reference in a vendored corpus as a
// missing file — the whole corpus failing a gate over where it was checked out.
//
// It is derived from the standards directory rather than configured, because
// there is nothing here for an adopter to decide: the corpus is laid out the
// way this framework lays it out, and the only question is where that layout
// begins.
func (o Options) corpusRoot() string {
	suffix := filepath.FromSlash(DefaultStandardsDir)
	if trimmed := strings.TrimSuffix(o.StandardsDir, suffix); trimmed != o.StandardsDir {
		return filepath.Clean(trimmed)
	}
	return filepath.Clean(o.RepoRoot)
}

// existsExactly reports whether rel exists under base with the case it was
// written in.
//
// os.Stat alone answers a different question on Windows and macOS, where the
// filesystem matches names case-insensitively: a reference to DESIGN.md
// resolves to design.md, the gate passes for the developer, and the same commit
// fails on the case-sensitive filesystem CI runs on. The check has to disagree
// with the local filesystem in order to agree with everybody else's.
func existsExactly(base, rel string) bool {
	if base == "" {
		return false
	}
	dir := base
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}
		found := false
		for _, e := range entries {
			if e.Name() != part {
				continue
			}
			// Only the last component may be a file; every earlier one has to be
			// a directory, or the path does not describe what it claims to.
			if i < len(parts)-1 && !e.IsDir() {
				return false
			}
			found = true
			break
		}
		if !found {
			return false
		}
		dir = filepath.Join(dir, part)
	}
	return true
}

// --- records ----------------------------------------------------------------

// Records enforces the durability rule: numbers run from 0001 with no gap and
// no duplicate, every document in the archive says it is one, and a record once
// committed is still there.
//
// Contiguity is checked rather than a frozen list, because a frozen list needs
// an edit per new record while contiguity holds for every record ever added and
// still fails the moment one is deleted. It is not enough on its own, which is
// what archive.go's three guards are for.
func Records(o Options) (Result, error) {
	o = o.Defaults()
	res := Result{Name: "records"}
	for label, dir := range map[string]string{"specs": o.SpecsDir, "adr": o.ADRDir} {
		problems, err := numbering(label, dir)
		if err != nil {
			return res, err
		}
		res.Problems = append(res.Problems, problems...)
	}
	headers, err := specHeaders(o)
	if err != nil {
		return res, err
	}
	res.Problems = append(res.Problems, headers...)

	a, err := loadArchive(o)
	if err != nil {
		return res, err
	}
	res.Problems = append(res.Problems, archivePins(o, a)...)
	deleted, err := deletedRecords(o, a)
	if err != nil {
		return res, err
	}
	res.Problems = append(res.Problems, deleted...)
	if len(a.Extracted) > 0 {
		res.Note = fmt.Sprintf("%d archive pin(s) checked against the extractions %s records", len(a.Extracted), a.Source)
	}

	// Order by File and then by Message, a total ordering over the problems
	// this gate can emit. Ordering on File alone is not enough: numbering
	// finds several problems under the same File value ("specs"), the labels
	// are walked in Go map order, and sort.Slice is not stable, so equal keys
	// could come out in either order and the gate's output would diff between
	// runs on an unchanged tree.
	sort.Slice(res.Problems, func(i, j int) bool {
		if res.Problems[i].File != res.Problems[j].File {
			return res.Problems[i].File < res.Problems[j].File
		}
		return res.Problems[i].Message < res.Problems[j].Message
	})
	return res, nil
}

func numbering(label, dir string) ([]Problem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	seen := map[string]string{}
	var numbers []int
	var problems []Problem
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !specFileName.MatchString(name) {
			continue
		}
		num := name[:4]
		if prev, dup := seen[num]; dup {
			problems = append(problems, Problem{File: label + "/" + name,
				Message: fmt.Sprintf("number %s is already used by %s; a durable number is never reused", num, prev)})
			continue
		}
		seen[num] = name
		n := 0
		fmt.Sscanf(num, "%d", &n)
		numbers = append(numbers, n)
	}
	if len(numbers) == 0 {
		return problems, nil
	}
	sort.Ints(numbers)
	if numbers[0] != 1 {
		problems = append(problems, Problem{File: label, Message: fmt.Sprintf("numbering starts at %04d, not 0001", numbers[0])})
	}
	for i := 1; i < len(numbers); i++ {
		if numbers[i] != numbers[i-1]+1 {
			problems = append(problems, Problem{File: label,
				Message: fmt.Sprintf("gap between %04d and %04d; a record was deleted rather than retired in place", numbers[i-1], numbers[i])})
		}
	}
	return problems, nil
}

// --- all --------------------------------------------------------------------

func All(o Options) ([]Result, error) {
	var results []Result
	for _, fn := range []func(Options) (Result, error){Spec, Commit, Branch, Docs, Records} {
		r, err := fn(o)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	return results, nil
}
