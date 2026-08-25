package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LukeSantossz/my-framework/internal/backend"
	"github.com/LukeSantossz/my-framework/internal/config"
	"github.com/LukeSantossz/my-framework/internal/eval"
	"github.com/LukeSantossz/my-framework/internal/report"
)

// CorpusDir is where the planted-defect cases live.
const CorpusDir = "docs/eval/corpus"

// runEval measures backends against the corpus.
//
// It reaches real providers, so it is never wired into a gate or CI: the number
// changes only when a prompt, a model or a chain changes, and paying tokens on
// every push for a figure nobody reads is the cost this deliberately avoids.
func runEval(env Env, args []string) int {
	roleName := "r2"
	only := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--role":
			value, ok := optionValue(env, "mf eval", args, &i)
			if !ok {
				return 2
			}
			roleName = value
		case "--backend":
			value, ok := optionValue(env, "mf eval", args, &i)
			if !ok {
				return 2
			}
			only = value
		default:
			fmt.Fprintf(env.Stderr, "mf eval: unknown option %q\n", args[i])
			return 2
		}
	}

	cfg, code := load(env)
	if code != 0 {
		return code
	}
	cases, err := eval.Load(filepath.Join(env.RepoRoot, filepath.FromSlash(CorpusDir)))
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf eval: %v\n", err)
		return 1
	}

	names := evalBackends(cfg, roleName, only)
	if len(names) == 0 {
		fmt.Fprintf(env.Stderr, "mf eval: no backend to measure for role %s\n", roleName)
		return 1
	}

	fmt.Fprintf(env.Stdout, "corpus %s v%d — %d case(s), %d planted defect(s)\n",
		CorpusDir, eval.CorpusVersion, len(cases), plantedTotal(cases))
	fmt.Fprintf(env.Stdout, "date %s\n\n", env.today())

	failed := false
	for _, name := range names {
		rep, runErr := evalBackend(env, cfg, name, roleName, cases)
		if runErr != nil {
			// A backend that could not be reached has no score. Recording a zero
			// would be indistinguishable from one that found nothing.
			fmt.Fprintf(env.Stderr, "mf eval: %s: %v\n", name, runErr)
			failed = true
			continue
		}
		printReport(env, rep)
	}

	// The rule decides the score, so it is printed with the results rather than
	// left in the source for a reader to go and find.
	fmt.Fprintf(env.Stdout, "\nMatching rule\n%s\n", indent(eval.MatchingRule))
	fmt.Fprintln(env.Stdout, "\nThese numbers measure this corpus and these prompts. They are self-reported")
	fmt.Fprintln(env.Stdout, "and are not comparable to an independent evaluation, nor to a run against a")
	fmt.Fprintln(env.Stdout, "different corpus version.")

	if failed {
		return 1
	}
	return 0
}

func plantedTotal(cases []eval.Case) int {
	n := 0
	for _, c := range cases {
		n += len(c.Defects)
	}
	return n
}

func evalBackends(cfg *config.Config, roleName, only string) []string {
	if only != "" {
		return []string{only}
	}
	names, _, _ := cfg.Get("roles." + roleName + ".backends")
	var out []string
	for _, n := range strings.Split(names, ",") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func evalBackend(env Env, cfg *config.Config, name, roleName string, cases []eval.Case) (eval.Report, error) {
	b, err := buildBackend(env, cfg, name)
	if err != nil {
		return eval.Report{}, err
	}
	model, _, _ := cfg.Get("backends." + name + ".model")
	if model == "" {
		model, _, _ = cfg.Get("review.model")
	}
	rep := eval.Report{
		Backend: name, Provider: b.Provider(), Model: model,
		Date: env.today(), CorpusVersion: eval.CorpusVersion,
	}

	for _, c := range cases {
		req := backend.Request{
			Role: roleName, Base: "main", Head: "eval/" + c.Dir,
			Diff:         c.Diff,
			Instructions: readInstructions(env.RepoRoot, agentsFile(cfg)),
			Model:        model,
			Effort:       stringValue(cfg, "review.effort", config.DefaultEffort),
		}
		res, reviewErr := b.Review(context.Background(), req)
		if reviewErr != nil {
			return eval.Report{}, fmt.Errorf("case %s: %w", c.Dir, reviewErr)
		}
		findings := structuredOnly(res)
		rep.Accumulate(c, eval.Match(findings, c.Defects), findings)
	}
	return rep, nil
}

// structuredOnly drops prose a backend could not classify. An unstructured blob
// cannot be matched against a planted category, and counting it as a false
// positive would punish a backend for a limitation the corpus cannot measure.
func structuredOnly(res report.Result) []report.Finding {
	if res.Unstructured {
		return nil
	}
	return res.Findings
}

func printReport(env Env, rep eval.Report) {
	hits, planted := rep.HitRate()
	fmt.Fprintf(env.Stdout, "%s / %s / %s\n", rep.Backend, orUnset(rep.Provider), orUnset(rep.Model))
	fmt.Fprintf(env.Stdout, "  hit rate         %d/%d planted defect(s) found\n", hits, planted)
	fmt.Fprintf(env.Stdout, "  false positives  %d finding(s) matched no plant\n", rep.TotalFalsePositives())
	for _, category := range rep.SortedCategories() {
		score := rep.ByCategory[category]
		fmt.Fprintf(env.Stdout, "  %-16s %d/%d\n", category, score.Hits, score.Planted)
	}
	for _, c := range rep.Cases {
		if c.Hits == c.Planted && c.FalsePositives == 0 {
			continue
		}
		fmt.Fprintf(env.Stdout, "  case %-26s %d/%d hit, %d false positive(s)\n",
			c.Case, c.Hits, c.Planted, c.FalsePositives)
	}
	fmt.Fprintln(env.Stdout)
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return strings.Join(lines, "\n")
}
