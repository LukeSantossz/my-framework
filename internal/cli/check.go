package cli

import (
	"fmt"
	"path/filepath"

	"github.com/LukeSantossz/my-framework/internal/check"
)

// runCheck runs the deterministic gates. Nothing here calls a model: judging an
// artifact and judging a process are different tasks, and only the first is
// reliable, so every process rule is checked deterministically or not at all.
func runCheck(env Env, args []string) int {
	var only []string
	for _, a := range args {
		switch a {
		case "spec", "commit", "branch", "docs", "records":
			only = append(only, a)
		default:
			fmt.Fprintf(env.Stderr, "mf check: unknown check %q (expected spec, commit, branch, docs or records)\n", a)
			return 2
		}
	}

	cfg, code := load(env)
	if code != 0 {
		return code
	}
	base, _, _ := cfg.Get("review.base")
	opts := check.Options{
		RepoRoot:     env.RepoRoot,
		StandardsDir: filepath.Join(env.RepoRoot, "docs", "standards"),
		Base:         base,
	}
	if cfg.Project != nil {
		opts.ExemptPaths = cfg.Project.Checks.ExemptPaths
	}

	results, err := runSelected(opts, only)
	if err != nil {
		// A document whose shape no longer matches stops the check rather than
		// letting it run on stale data.
		fmt.Fprintf(env.Stderr, "mf check: %v\n", err)
		return 1
	}

	failed := 0
	for _, r := range results {
		if r.OK() {
			note := r.Note
			if note == "" {
				note = "passed"
			}
			fmt.Fprintf(env.Stdout, "ok   %-8s %s\n", r.Name, note)
			continue
		}
		failed++
		fmt.Fprintf(env.Stdout, "FAIL %-8s %d problem(s)\n", r.Name, len(r.Problems))
		for _, p := range r.Problems {
			if p.File != "" {
				fmt.Fprintf(env.Stdout, "       %s: %s\n", p.File, p.Message)
				continue
			}
			fmt.Fprintf(env.Stdout, "       %s\n", p.Message)
		}
	}
	if failed > 0 {
		return 1
	}
	return 0
}

func runSelected(opts check.Options, only []string) ([]check.Result, error) {
	if len(only) == 0 {
		return check.All(opts)
	}
	byName := map[string]func(check.Options) (check.Result, error){
		"spec": check.Spec, "commit": check.Commit, "branch": check.Branch,
		"docs": check.Docs, "records": check.Records,
	}
	var results []check.Result
	for _, name := range only {
		r, err := byName[name](opts)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	return results, nil
}
