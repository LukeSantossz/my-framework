// Package eval measures a review backend against diffs carrying known,
// deliberately planted defects.
//
// The metric is hit rate — of the planted defects, how many were found — with
// false positives reported separately rather than folded into one score. For a
// gate whose output a human triages, those two numbers drive opposite
// decisions: a low hit rate means the gate misses things, a high false-positive
// rate means people stop reading it.
//
// Grading is by matching against the planted defects, never by a model judging
// a model. The ground truth is already known because the defects were planted,
// and a judge would import self-bias, length bias and position bias into the
// very measurement meant to detect quality.
package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/LukeSantossz/my-framework/internal/report"
)

// CorpusVersion changes whenever the cases change. Results carrying different
// versions are not comparable and are refused rather than silently averaged.
const CorpusVersion = 1

// MatchingRule is printed with every result. A loose rule counts a vague
// finding as a hit and inflates every number, so stating it is the only defence
// a reader has.
const MatchingRule = `A finding counts as a hit for a planted defect when all of these hold:
  1. the finding's category equals the defect's category;
  2. if the defect names a file, the finding names the same file;
  3. the finding's summary or rationale contains at least one of the defect's
     terms, compared case-insensitively.
Each planted defect can be hit at most once. A finding matching no planted
defect is counted as a false positive, never as a hit.`

// Defect is one deliberately planted problem.
type Defect struct {
	Category string   `toml:"category"`
	File     string   `toml:"file"`
	Line     int      `toml:"line"`
	Terms    []string `toml:"terms"`
}

// Case is one diff and what was planted in it. A case with no defects is a
// clean diff, and it is what makes false positives measurable: a backend that
// flags everything must score badly somewhere.
type Case struct {
	Name    string   `toml:"name"`
	Version int      `toml:"version"`
	Defects []Defect `toml:"defects"`

	Dir  string `toml:"-"`
	Diff string `toml:"-"`
}

func (c Case) IsClean() bool { return len(c.Defects) == 0 }

// Load reads every case in a corpus directory.
func Load(dir string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read the corpus at %s: %w", dir, err)
	}
	var cases []Case
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		caseDir := filepath.Join(dir, e.Name())
		manifest, err := os.ReadFile(filepath.Join(caseDir, "case.toml"))
		if err != nil {
			continue
		}
		var c Case
		if _, err := toml.Decode(string(manifest), &c); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if c.Version != CorpusVersion {
			return nil, fmt.Errorf("%s: corpus version %d, this build reads %d; results across versions are not comparable",
				e.Name(), c.Version, CorpusVersion)
		}
		diff, err := os.ReadFile(filepath.Join(caseDir, "change.diff"))
		if err != nil {
			return nil, fmt.Errorf("%s: no change.diff beside its manifest", e.Name())
		}
		for _, d := range c.Defects {
			if len(d.Terms) == 0 {
				return nil, fmt.Errorf("%s: a planted defect with no terms can never be matched", e.Name())
			}
		}
		c.Dir = e.Name()
		c.Diff = string(diff)
		cases = append(cases, c)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].Dir < cases[j].Dir })
	if len(cases) == 0 {
		return nil, fmt.Errorf("no cases found in %s", dir)
	}
	return cases, nil
}

// CaseResult is what one backend scored on one case.
type CaseResult struct {
	Case           string
	Planted        int
	Hits           int
	FalsePositives int
	MissedTerms    []string
}

// Match applies the rule above. It is deliberately conservative: a finding that
// does not clearly correspond to a plant counts against the backend rather than
// being generously credited.
func Match(findings []report.Finding, defects []Defect) CaseResult {
	res := CaseResult{Planted: len(defects)}
	used := make([]bool, len(findings))

	for _, d := range defects {
		matched := false
		for i, f := range findings {
			if used[i] {
				continue
			}
			if !defectMatches(d, f) {
				continue
			}
			used[i] = true
			matched = true
			res.Hits++
			break
		}
		if !matched {
			res.MissedTerms = append(res.MissedTerms, d.Category+":"+strings.Join(d.Terms, "|"))
		}
	}
	for i := range findings {
		if !used[i] {
			res.FalsePositives++
		}
	}
	return res
}

func defectMatches(d Defect, f report.Finding) bool {
	if string(f.Category) != d.Category {
		return false
	}
	if d.File != "" && !strings.EqualFold(filepath.ToSlash(f.File), filepath.ToSlash(d.File)) {
		return false
	}
	haystack := strings.ToLower(f.Summary + " " + f.Rationale)
	for _, term := range d.Terms {
		if strings.Contains(haystack, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

// Report is one backend's whole run. Every field that makes a number
// comparable — the model, the date, the corpus version — is carried with it: a
// number without those cannot be compared to another number.
type Report struct {
	Backend       string
	Provider      string
	Model         string
	Date          string
	CorpusVersion int

	Cases      []CaseResult
	ByCategory map[string]CategoryScore
}

type CategoryScore struct {
	Planted int
	Hits    int
}

func (r Report) TotalPlanted() int {
	n := 0
	for _, c := range r.Cases {
		n += c.Planted
	}
	return n
}

func (r Report) TotalHits() int {
	n := 0
	for _, c := range r.Cases {
		n += c.Hits
	}
	return n
}

func (r Report) TotalFalsePositives() int {
	n := 0
	for _, c := range r.Cases {
		n += c.FalsePositives
	}
	return n
}

// HitRate is reported as a fraction with its denominator, because a percentage
// over a handful of defects invites being over-read.
func (r Report) HitRate() (hits, planted int) {
	return r.TotalHits(), r.TotalPlanted()
}

// Comparable refuses to line up results from different corpora.
func Comparable(a, b Report) error {
	if a.CorpusVersion != b.CorpusVersion {
		return fmt.Errorf("corpus versions differ (%d and %d); these results measure different things",
			a.CorpusVersion, b.CorpusVersion)
	}
	return nil
}

// Accumulate folds one case's outcome into the per-category tally.
func (r *Report) Accumulate(c Case, result CaseResult, findings []report.Finding) {
	if r.ByCategory == nil {
		r.ByCategory = map[string]CategoryScore{}
	}
	r.Cases = append(r.Cases, result)

	remaining := append([]report.Finding(nil), findings...)
	for _, d := range c.Defects {
		score := r.ByCategory[d.Category]
		score.Planted++
		for i, f := range remaining {
			if defectMatches(d, f) {
				score.Hits++
				remaining = append(remaining[:i], remaining[i+1:]...)
				break
			}
		}
		r.ByCategory[d.Category] = score
	}
}

// SortedCategories keeps the printed order stable between runs.
func (r Report) SortedCategories() []string {
	out := make([]string, 0, len(r.ByCategory))
	for c := range r.ByCategory {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
