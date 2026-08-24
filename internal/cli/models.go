package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/LukeSantossz/my-framework/internal/activate"
	"github.com/LukeSantossz/my-framework/internal/config"
	"github.com/LukeSantossz/my-framework/internal/usage"
)

// resolvedModels maps each declared backend to the model id the configuration
// currently resolves for it.
func resolvedModels(cfg *config.Config) map[string]string {
	resolved := map[string]string{}
	if cfg.Project == nil {
		return resolved
	}
	chainWide, _, _ := cfg.Get("review.model")
	for name, b := range cfg.Project.Backends {
		model := b.Model
		if model == "" {
			model = chainWide
		}
		resolved[name] = model
	}
	return resolved
}

func (e Env) today() string {
	if e.Now != nil {
		return e.Now().Format("2006-01-02")
	}
	return time.Now().Format("2006-01-02")
}

func runModels(env Env, args []string) int {
	action := "list"
	if len(args) > 0 {
		action = args[0]
	}
	cfg, code := load(env)
	if code != 0 {
		return code
	}
	resolved := resolvedModels(cfg)

	switch action {
	case "pin":
		lock, err := activate.PinModels(env.RepoRoot, resolved, env.today())
		if err != nil {
			fmt.Fprintf(env.Stderr, "mf models pin: %v\n", err)
			return 1
		}
		for _, name := range sortedKeys(lock.Models) {
			fmt.Fprintf(env.Stdout, "pinned %-14s %s (%s)\n", name, lock.Models[name].Model, lock.Models[name].PinnedOn)
		}
		// What is recorded is what the configuration resolves to, not what a
		// provider confirmed. Verifying an id against a live vendor costs tokens
		// and a key, so it is never done as a side effect of pinning.
		fmt.Fprintln(env.Stdout, "\nThese are the ids the configuration resolves to. Nothing was sent to a provider,")
		fmt.Fprintln(env.Stdout, "so a pinned id being accepted by its vendor remains unverified.")
		return 0

	case "list":
		lock, ok := activate.ReadLock(env.RepoRoot)
		if !ok || len(lock.Models) == 0 {
			fmt.Fprintln(env.Stdout, "no models pinned; run `mf models pin`")
			return 0
		}
		drift := activate.ComparePins(lock, resolved)
		drifted := map[string]activate.ModelDrift{}
		for _, d := range drift {
			drifted[d.Backend] = d
		}
		for _, name := range sortedKeys(lock.Models) {
			pin := lock.Models[name]
			if d, ok := drifted[name]; ok {
				fmt.Fprintf(env.Stdout, "DRIFT %-14s pinned %s (%s), configured %s\n", name, d.Pinned, d.PinnedOn, d.Configured)
				continue
			}
			fmt.Fprintf(env.Stdout, "ok    %-14s %s (%s)\n", name, pin.Model, pin.PinnedOn)
		}
		return 0
	}
	fmt.Fprintf(env.Stderr, "mf models: unknown action %q (expected pin or list)\n", action)
	return 2
}

func sortedKeys(m map[string]activate.PinnedModel) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// usageStore lives beside the machine configuration rather than in the
// repository: it answers "what did I spend", never "what did this project cost".
func usageStore(env Env) usage.Store {
	dir := filepath.Dir(env.MachinePath)
	if dir == "" || dir == "." {
		return usage.Store{}
	}
	return usage.Store{Path: filepath.Join(dir, "usage.json")}
}

func runUsage(env Env, args []string) int {
	store := usageStore(env)
	if store.Path == "" {
		fmt.Fprintln(env.Stderr, "mf usage: no machine configuration directory to keep a total in")
		return 1
	}
	if len(args) > 0 && args[0] == "reset" {
		if err := store.Reset(); err != nil {
			fmt.Fprintf(env.Stderr, "mf usage reset: %v\n", err)
			return 1
		}
		fmt.Fprintln(env.Stdout, "usage total reset")
		return 0
	}
	if len(args) > 0 && args[0] != "show" {
		fmt.Fprintf(env.Stderr, "mf usage: unknown action %q (expected show or reset)\n", args[0])
		return 2
	}
	totals := store.Read()
	fmt.Fprintf(env.Stdout, "runs:  %d\n", totals.Runs)
	fmt.Fprintf(env.Stdout, "usage: %s\n", totals.Usage)

	cfg, code := load(env)
	if code == 0 {
		if line, ok := costLine(cfg, totals.Usage); ok {
			fmt.Fprintln(env.Stdout, line)
		}
	}
	return 0
}

// costLine renders money only when the user supplied a price for the model. A
// price table ages faster than releases, so none ships and a figure the tool
// cannot defend is not printed.
func costLine(cfg *config.Config, u usage.Usage) (string, bool) {
	if cfg.Machine == nil || len(cfg.Machine.Prices) == 0 || !u.Known {
		return "", false
	}
	model, _, _ := cfg.Get("review.model")
	table := usage.Table{}
	for name, p := range cfg.Machine.Prices {
		table[name] = p
	}
	cost, ok := table.Cost(model, u)
	if !ok {
		return fmt.Sprintf("cost:  no price configured for %q", model), true
	}
	suffix := ""
	if u.Estimated {
		suffix = " (from an ESTIMATED token count)"
	}
	return fmt.Sprintf("cost:  %.4f for %s%s", cost, model, suffix), true
}

func modelDriftLines(env Env, cfg *config.Config) []string {
	lock, ok := activate.ReadLock(env.RepoRoot)
	if !ok || len(lock.Models) == 0 {
		return []string{"no models pinned; run `mf models pin` so a vendor changing an id is visible"}
	}
	drift := activate.ComparePins(lock, resolvedModels(cfg))
	if len(drift) == 0 {
		return []string{fmt.Sprintf("%d model(s) pinned, all matching the configuration", len(lock.Models))}
	}
	lines := make([]string, 0, len(drift))
	for _, d := range drift {
		lines = append(lines, fmt.Sprintf("%s: pinned %s on %s, configuration now says %s",
			d.Backend, d.Pinned, d.PinnedOn, d.Configured))
	}
	return lines
}
