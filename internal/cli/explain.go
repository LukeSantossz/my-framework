package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LukeSantossz/my-framework/internal/backend"
	"github.com/LukeSantossz/my-framework/internal/config"
	"github.com/LukeSantossz/my-framework/internal/explain"
	"github.com/LukeSantossz/my-framework/internal/report"
	"github.com/LukeSantossz/my-framework/internal/role"
	"github.com/LukeSantossz/my-framework/internal/vcs"
)

// ExplainRole is the role whose chain answers `mf explain`. It is a role like
// any other, so which model explains a change is configuration rather than a
// second, separate setting nobody remembers exists.
const ExplainRole = "explain"

// runExplain generates the CRUX explainer for the current change.
//
// It never blocks and never reports a verdict. crux_method.md and
// docs/adr/0003-crux-explainers-are-transient.md both make it an aid feeding
// R1 and CRURA, and an aid that can fail a run is a gate wearing another name.
// Every path here returns zero.
func runExplain(env Env, args []string) int {
	base := ""
	difficultyFlag := ""
	dir := ""
	dryRun := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			dryRun = true
		case "--base":
			value, ok := optionValue(env, "mf explain", args, &i)
			if !ok {
				return 2
			}
			base = value
		case "--difficulty":
			value, ok := optionValue(env, "mf explain", args, &i)
			if !ok {
				return 2
			}
			difficultyFlag = value
		case "--dir":
			value, ok := optionValue(env, "mf explain", args, &i)
			if !ok {
				return 2
			}
			dir = value
		default:
			fmt.Fprintf(env.Stderr, "mf explain: unknown option %q\n", args[i])
			return 2
		}
	}

	// A mistyped flag is the caller's error, not a degraded explainer, so it is
	// the one thing here that does not exit zero.
	difficulty, err := explain.ParseDifficulty(difficultyFlag)
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf explain: %v\n", err)
		return 2
	}

	cfg, code := load(env)
	if code != 0 {
		return code
	}
	if base == "" {
		base, _, _ = cfg.Get("review.base")
	}
	if dir == "" {
		dir = explainDir(env, cfg)
	}
	// Checked before anything runs. Discovering that the destination is refused
	// after a model has already been paid for the answer wastes the run and the
	// quota, and a wrong destination is the caller's error rather than a
	// degraded explainer.
	if err := explain.CheckDestination(dir, env.RepoRoot); err != nil {
		fmt.Fprintf(env.Stderr, "mf explain: %v\n", err)
		return 2
	}

	repo := vcs.Open(env.RepoRoot)
	head, err := repo.CurrentBranch()
	if err != nil {
		return absent(env, fmt.Sprintf("cannot determine the current branch: %v", err))
	}
	if head == base {
		return absent(env, fmt.Sprintf("on base branch %q; there is no change to explain", base))
	}

	chain, buildErr := buildChain(env, cfg, ExplainRole)
	if buildErr != nil {
		return absent(env, buildErr.Error())
	}
	if len(chain) == 0 {
		return absent(env, fmt.Sprintf("no backend configured for role %q; add one to .framework.toml", ExplainRole))
	}

	if dryRun {
		// Shows which model would explain and where the file would land,
		// without spending a token. Both are configuration, and configuration
		// nobody can inspect is configuration nobody trusts.
		fmt.Fprintf(env.Stdout, "[explain] %s against %s at %s difficulty\n", head, base, difficulty)
		fmt.Fprintf(env.Stdout, "  destination: %s\n", filepath.Join(dir, explain.FileName(env.today(), head)))
		for _, line := range describeExplainChain(env, cfg, chain) {
			fmt.Fprintf(env.Stdout, "  %s\n", line)
		}
		return 0
	}

	diff, err := repo.Diff(base, head, intValue(cfg, "review.max_diff_bytes", 30000))
	if err != nil {
		return absent(env, err.Error())
	}
	if diff.Empty {
		return absent(env, fmt.Sprintf("%q adds nothing over %q; there is nothing to explain", head, base))
	}

	req := backend.Request{
		Role: ExplainRole, Base: base, Head: head,
		Diff: diff.Text, Truncated: diff.Truncated,
		System: explain.Prompt(difficulty),
		Model:  stringValue(cfg, "review.model", ""),
		Effort: stringValue(cfg, "review.effort", config.DefaultEffort),
	}

	runner := &role.Runner{Role: ExplainRole, Chain: chain}
	out, err := runner.Run(context.Background(), req)
	if err != nil {
		return absent(env, err.Error())
	}
	for _, s := range out.Skipped {
		fmt.Fprintf(env.Stdout, "[explain] skipped %s: %s\n", s.Backend, s.Reason)
	}
	if !out.Ran {
		// The method's own fallback: no explainer is produced, the reviewer
		// reads the diff directly, and the absence is stated rather than
		// passing for an explainer nobody opened.
		return absent(env, "no configured backend was available")
	}

	// An explainer costs a model call like a review does. Recording it keeps
	// `mf usage` a total rather than a total of one command.
	if store := usageStore(env); store.Path != "" {
		_ = store.Add(out.Result.Usage)
	}

	content, parseErr := explain.Parse(report.Text(out.Result))
	if parseErr != nil {
		return absent(env, fmt.Sprintf("%s answered, but not with an explainer: %v", out.Result.Backend, parseErr))
	}

	path, writeErr := explain.Write(dir, env.RepoRoot, content, explain.Meta{
		Head: head, Base: base,
		Backend:  out.Result.Backend,
		Provider: out.Result.Provider,
		Model:    out.Result.Model,
		Date:     env.today(),
		// crux_method.md passes the prose through the `humanizer` skill before
		// the final render. Nothing here does, so the explainer says so on its
		// own face rather than letting the missing step be silent.
		Humanized:  false,
		Difficulty: difficulty,
	})
	if writeErr != nil {
		return absent(env, writeErr.Error())
	}

	fmt.Fprintf(env.Stdout, "[explain] %s\n", path)
	fmt.Fprintf(env.Stdout, "[explain] %s explained %s against %s at %s difficulty\n",
		out.Result.Backend, head, base, difficulty)
	if diff.Truncated {
		fmt.Fprintln(env.Stdout, "[explain] the diff was truncated, so the explainer covers only part of the change")
	}
	fmt.Fprintln(env.Stdout, "[explain] advisory: this is an aid to R1 and CRURA, not a review layer and not a gate.")
	fmt.Fprintln(env.Stdout, "[explain] nothing verifies its claims; read it against the diff, not instead of it.")
	return 0
}

// describeExplainChain names each backend and the model the configuration
// resolves for it. The explainer is a role like any other, so the answer comes
// from the role's chain rather than from a setting of its own.
func describeExplainChain(env Env, cfg *config.Config, chain []backend.Backend) []string {
	lines := make([]string, 0, len(chain))
	for _, b := range chain {
		model, _, _ := cfg.Get("backends." + b.Name() + ".model")
		if model == "" {
			model, _, _ = cfg.Get("review.model")
		}
		lines = append(lines, fmt.Sprintf("%s [%s]: model=%s", b.Name(), b.Provider(), orUnset(model)))
	}
	return lines
}

// absent reports why no explainer was produced and exits zero. The method's
// fallbacks degrade deliberately, never silently — and an aid that fails a run
// is a gate wearing another name.
func absent(env Env, reason string) int {
	fmt.Fprintf(env.Stdout, "[explain] no explainer produced: %s\n", reason)
	fmt.Fprintln(env.Stdout, "[explain] read the diff directly and note the absent CRUX aid in the pull request.")
	return 0
}

// explainDir resolves where the artifact goes. The default is the user cache
// directory: outside version control by construction, and already the place a
// machine keeps files it may delete.
func explainDir(env Env, cfg *config.Config) string {
	if dir, _, ok := cfg.Get("explain.dir"); ok && dir != "" {
		return dir
	}
	if home := env.Getenv("MF_CONFIG_HOME"); home != "" {
		return filepath.Join(home, "explain")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "framework", "explain")
	}
	return filepath.Join(cache, "framework", "explain")
}
