package cli

import (
	"fmt"

	"github.com/LukeSantossz/my-framework/internal/agents"
	"github.com/LukeSantossz/my-framework/internal/config"
)

func agentTargets(cfg *config.Config) []agents.Target {
	if cfg.Project == nil {
		return nil
	}
	var targets []agents.Target
	for name, a := range cfg.Project.Agents {
		targets = append(targets, agents.Target{
			Name: name, File: a.File, Roles: a.Roles, PathPrefix: a.PathPrefix,
		})
	}
	// Sorted so a run's output and its writes are the same every time.
	for i := 0; i < len(targets); i++ {
		for j := i + 1; j < len(targets); j++ {
			if targets[j].Name < targets[i].Name {
				targets[i], targets[j] = targets[j], targets[i]
			}
		}
	}
	return targets
}

func runAgents(env Env, args []string) int {
	action := "check"
	if len(args) > 0 {
		action = args[0]
	}
	if action != "sync" && action != "check" {
		fmt.Fprintf(env.Stderr, "mf agents: unknown action %q (expected sync or check)\n", action)
		return 2
	}

	cfg, code := load(env)
	if code != 0 {
		return code
	}
	targets := agentTargets(cfg)
	if len(targets) == 0 {
		fmt.Fprintln(env.Stdout, "no [agents.*] declared; nothing to generate")
		return 0
	}
	opts := agents.Options{RepoRoot: env.RepoRoot, Targets: targets, SourcePath: agentsSource(cfg), OverlayPath: agentsOverlay(cfg)}

	if action == "sync" {
		results, err := agents.Sync(opts)
		if err != nil {
			fmt.Fprintf(env.Stderr, "mf agents sync: %v\n", err)
			return 1
		}
		for _, r := range results {
			if r.Changed {
				fmt.Fprintf(env.Stdout, "+ %-14s %s\n", r.Target, r.File)
				continue
			}
			fmt.Fprintf(env.Stdout, "  %-14s %s (already current)\n", r.Target, r.File)
		}
		return 0
	}

	results, err := agents.Check(opts)
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf agents check: %v\n", err)
		return 1
	}
	drifted := 0
	for _, r := range results {
		if r.Drifted {
			drifted++
			fmt.Fprintf(env.Stdout, "DRIFT %-14s %s\n", r.Target, r.File)
			continue
		}
		fmt.Fprintf(env.Stdout, "ok    %-14s %s\n", r.Target, r.File)
	}
	if drifted > 0 {
		// Without this failing, the generated files are a convention people
		// bypass by editing the output — the original duplication with extra
		// steps.
		fmt.Fprintf(env.Stdout, "\n%d file(s) differ from %s. Run `mf agents sync`.\n", drifted, agentsSource(cfg))
		return 1
	}
	return 0
}

func agentsCheck(env Env, cfg *config.Config, targets []agents.Target) ([]agents.Result, error) {
	return agents.Check(agents.Options{RepoRoot: env.RepoRoot, Targets: targets, SourcePath: agentsSource(cfg), OverlayPath: agentsOverlay(cfg)})
}
