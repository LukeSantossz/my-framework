package cli

import (
	"fmt"
	"strings"

	"github.com/LukeSantossz/my-framework/internal/check"
	"github.com/LukeSantossz/my-framework/internal/config"
)

// Where this repository keeps the documents the gates read.
//
// Both answers are configuration rather than constants, for the reason
// agents.DefaultPathPrefix already records for the generated instruction files:
// a repository that vendors this framework as a `.standards` submodule keeps
// the same documents under it, and a gate that can only read `docs/standards`
// is a gate that adopter cannot run at all.
//
// The value is returned as configured — relative to the repository root unless
// it is absolute — because every consumer takes the root separately and names
// the path in its own messages, where a resolved absolute path would be noise.
func standardsDir(cfg *config.Config) string {
	return configuredDir(cfg, "paths.standards", check.DefaultStandardsDir)
}

func specsDir(cfg *config.Config) string {
	return configuredDir(cfg, "paths.specs", check.DefaultSpecsDir)
}

// adrDir is configurable for the same reason specsDir is. The records gate
// reads both durable archives, so relocating one and not the other would
// leave a repository able to move its specs and unable to move the decisions
// that supersede them.
// agentsSource is where the document the vendor files are generated from
// lives, as configured. It moves with the standards for a repository that
// vendors them: the source sits inside the submodule beside the documents it
// cites, and generating from a path that only resolves here left `mf agents
// sync` as the one command such a repository could not run.
func agentsSource(cfg *config.Config) string {
	return configuredDir(cfg, "paths.agents_source", config.DefaultAgentsSource)
}

func adrDir(cfg *config.Config) string {
	return configuredDir(cfg, "paths.adr", check.DefaultADRDir)
}

// configuredDir falls back to the layout this framework ships with, so a
// repository that configures nothing behaves exactly as it did before the key
// existed.
func configuredDir(cfg *config.Config, key, fallback string) string {
	if cfg != nil {
		if value, _, ok := cfg.Get(key); ok {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return fallback
}

// runCheck runs the deterministic gates. Nothing here calls a model: judging an
// artifact and judging a process are different tasks, and only the first is
// reliable, so every process rule is checked deterministically or not at all.
func runCheck(env Env, args []string) int {
	var only []string
	// The commit-msg hook is handed the message being written as $1. Without
	// this mode the hook can only read the commits already on the branch, so a
	// subject that breaks the vocabulary is reported one commit after the one
	// the author still has open — the wrong commit, at the wrong moment.
	messageFile := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "spec", "commit", "branch", "docs", "records", "agents", "design":
			only = append(only, args[i])
		case "--message":
			value, ok := optionValue(env, "mf check", args, &i)
			if !ok {
				return 2
			}
			messageFile = value
		default:
			fmt.Fprintf(env.Stderr, "mf check: unknown check %q (expected spec, commit, branch, docs, records, agents or design)\n", args[i])
			return 2
		}
	}
	if messageFile != "" && (len(only) != 1 || only[0] != "commit") {
		fmt.Fprintln(env.Stderr, "mf check: --message belongs to `mf check commit` alone")
		return 2
	}

	cfg, code := load(env)
	if code != 0 {
		return code
	}
	base, _, _ := cfg.Get("review.base")
	opts := check.Options{
		RepoRoot:     env.RepoRoot,
		StandardsDir: standardsDir(cfg),
		SpecsDir:     specsDir(cfg),
		ADRDir:       adrDir(cfg),
		Base:         base,
	}
	if cfg.Project != nil {
		opts.ExemptPaths = cfg.Project.Checks.ExemptPaths
	}

	// The instruction-file drift check lives beside the others so one command
	// answers "is this repository in the state its standards describe". It is
	// separated out here because it reads configuration rather than the
	// standards tree, so it is not one of the check package's gates.
	explicit := len(only) > 0
	checkAgents := !explicit
	checkDesign := !explicit
	var gates []string
	for _, name := range only {
		switch name {
		case "agents":
			checkAgents = true
		case "design":
			checkDesign = true
		default:
			gates = append(gates, name)
		}
	}

	var results []check.Result
	if messageFile != "" {
		res, err := check.CommitMessage(opts, messageFile)
		if err != nil {
			fmt.Fprintf(env.Stderr, "mf check: %v\n", err)
			return 1
		}
		results = []check.Result{res}
		if reportResults(env, results) > 0 {
			return 1
		}
		return 0
	}
	// An empty gate list means "run them all", so asking only for `agents` must
	// skip this call rather than fall through to every gate.
	if !explicit || len(gates) > 0 {
		var err error
		results, err = runSelected(opts, gates)
		if err != nil {
			// A document whose shape no longer matches stops the check rather
			// than letting it run on stale data.
			fmt.Fprintf(env.Stderr, "mf check: %v\n", err)
			return 1
		}
	}

	failed := reportResults(env, results)
	if checkAgents {
		if code := agentsGate(env); code != 0 {
			failed++
		}
	}
	if checkDesign {
		if code := designGate(env, cfg); code != 0 {
			failed++
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

// agentsGate reports instruction-file drift as one more gate line, so `mf check`
// answers the whole question rather than most of it.
func agentsGate(env Env) int {
	cfg, code := load(env)
	if code != 0 {
		return code
	}
	targets := agentTargets(cfg)
	if len(targets) == 0 {
		// Said out loud, because a gate that passes by not running reads
		// exactly like one that ran and found the files in order — and the
		// instruction files promise this gate fails when they drift apart.
		// `mf init` scaffolds both targets, so a repository reaching this
		// line has removed them rather than never had them.
		fmt.Fprintf(env.Stdout, "ok   %-8s no [agents.*] declared; nothing is being compared\n", "agents")
		return 0
	}
	results, err := agentsCheck(env, cfg, targets)
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf check agents: %v\n", err)
		return 1
	}
	drifted := 0
	for _, r := range results {
		if r.Drifted {
			drifted++
		}
	}
	if drifted == 0 {
		fmt.Fprintf(env.Stdout, "ok   %-8s %d file(s) match their source\n", "agents", len(results))
		return 0
	}
	fmt.Fprintf(env.Stdout, "FAIL %-8s %d file(s) drifted from their source\n", "agents", drifted)
	for _, r := range results {
		if r.Drifted {
			fmt.Fprintf(env.Stdout, "       %s: run `mf agents sync`\n", r.File)
		}
	}
	return 1
}

// reportResults renders each gate on one line and answers how many failed. It
// is shared so the single-result paths — a commit message handed to the hook —
// print in exactly the shape the full run does, rather than growing a second
// rendering that drifts from it.
func reportResults(env Env, results []check.Result) int {
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
	return failed
}
