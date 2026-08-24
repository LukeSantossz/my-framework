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

type Options struct {
	RepoRoot     string
	StandardsDir string
	SpecsDir     string
	ADRDir       string
	Base         string
	ExemptPaths  []string
	Repo         *vcs.Repo
}

// Defaults fills the paths a repository following this framework uses.
func (o Options) Defaults() Options {
	if o.StandardsDir == "" {
		o.StandardsDir = filepath.Join(o.RepoRoot, "docs", "standards")
	}
	if o.SpecsDir == "" {
		o.SpecsDir = filepath.Join(o.RepoRoot, "docs", "specs")
	}
	if o.ADRDir == "" {
		o.ADRDir = filepath.Join(o.RepoRoot, "docs", "adr")
	}
	if o.Base == "" {
		o.Base = "main"
	}
	if o.Repo == nil {
		o.Repo = vcs.Open(o.RepoRoot)
	}
	return o
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

	var specs []string
	for _, f := range changed {
		if filepath.ToSlash(filepath.Dir(f)) == "docs/specs" && specFileName.MatchString(filepath.Base(f)) {
			specs = append(specs, f)
		}
	}
	if len(specs) == 0 {
		res.Problems = append(res.Problems, Problem{
			Message: fmt.Sprintf("this branch changes %d non-exempt path(s) but adds no spec under docs/specs/NNNN-<slug>.md", len(changed)),
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
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			n++
		}
	}
	return n
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

func Commit(o Options) (Result, error) {
	o = o.Defaults()
	res := Result{Name: "commit"}

	doc, err := o.read(o.StandardsDir, "github.md")
	if err != nil {
		return res, fmt.Errorf("cannot read github.md: %w", err)
	}
	types, err := ParseTypeTable(doc)
	if err != nil {
		return res, err
	}
	valid := map[string]bool{}
	for _, t := range types {
		valid[t] = true
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
	for _, c := range commits {
		short := c.SHA
		if len(short) > 8 {
			short = short[:8]
		}
		m := conventional.FindStringSubmatch(c.Subject)
		if m == nil {
			res.Problems = append(res.Problems, Problem{File: short,
				Message: fmt.Sprintf("subject %q is not Conventional Commits format", c.Subject)})
		} else if !valid[m[1]] {
			res.Problems = append(res.Problems, Problem{File: short,
				Message: fmt.Sprintf("type %q is absent from the Type Table in github.md (valid: %s)", m[1], strings.Join(types, ", "))})
		}
		if attribution.MatchString(c.Subject + "\n" + c.Body) {
			res.Problems = append(res.Problems, Problem{File: short,
				Message: "carries a co-author or AI-attribution line, which github.md forbids"})
		}
	}
	res.Note = fmt.Sprintf("%d commit(s) checked against %d types read from github.md", len(commits), len(types))
	return res, nil
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
}

var mdRef = regexp.MustCompile(`[A-Za-z0-9_./-]+\.md`)

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
		for _, ref := range mdRef.FindAllString(body, -1) {
			if hypothetical[ref] {
				continue
			}
			if !refResolves(o, ref) {
				res.Problems = append(res.Problems, Problem{File: name, Message: "references a missing file: " + ref})
			}
		}
	}
	return res, nil
}

func refResolves(o Options, ref string) bool {
	if strings.ContainsAny(ref, "/\\") {
		_, err := os.Stat(filepath.Join(o.RepoRoot, filepath.FromSlash(ref)))
		return err == nil
	}
	if _, err := os.Stat(filepath.Join(o.StandardsDir, ref)); err == nil {
		return true
	}
	_, err := os.Stat(filepath.Join(o.RepoRoot, ref))
	return err == nil
}

// --- records ----------------------------------------------------------------

// Records enforces the durability rule: numbers run from 0001 with no gap and
// no duplicate. Contiguity is checked rather than a frozen list, because a
// frozen list needs an edit per new record while contiguity holds for every
// record ever added and still fails the moment one is deleted.
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
	sort.Slice(res.Problems, func(i, j int) bool { return res.Problems[i].File < res.Problems[j].File })
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
