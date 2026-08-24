package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/LukeSantossz/my-framework/internal/activate"
	"github.com/LukeSantossz/my-framework/internal/config"
	"github.com/LukeSantossz/my-framework/internal/style"
	"github.com/LukeSantossz/my-framework/internal/upgrade"
	"github.com/LukeSantossz/my-framework/internal/vcs"
	"github.com/LukeSantossz/my-framework/internal/version"
)

// runDoctor reports; it never repairs. A diagnostic that fixes as it reads
// makes the second run disagree with the first and hides the drift that
// mattered.
func runDoctor(env Env) int {
	cfg, code := load(env)
	if code != 0 {
		return code
	}
	repo := vcs.Open(env.RepoRoot)

	fmt.Fprintf(env.Stdout, "mf %s\n", version.Version)
	fmt.Fprintf(env.Stdout, "repository: %s\n\n", env.RepoRoot)

	// Activation.
	fmt.Fprintln(env.Stdout, "activation")
	state := activate.HooksStatus(env.RepoRoot)
	switch {
	case state.Canonical:
		fmt.Fprintf(env.Stdout, "  hooks      wired to %s\n", activate.HooksDir)
	case state.Path != "":
		fmt.Fprintf(env.Stdout, "  hooks      core.hooksPath points at %q, not %s — the gate does not run\n", state.Path, activate.HooksDir)
	case !state.Present:
		fmt.Fprintf(env.Stdout, "  hooks      no %s directory in this repository\n", activate.HooksDir)
	default:
		fmt.Fprintf(env.Stdout, "  hooks      not wired; run `mf hooks install`\n")
	}
	if lock, ok := activate.ReadLock(env.RepoRoot); ok && lock.FrameworkVersion != "" {
		fmt.Fprintf(env.Stdout, "  adopted    framework %s\n", lock.FrameworkVersion)
	} else {
		fmt.Fprintf(env.Stdout, "  adopted    no %s; run `mf init`\n", activate.LockFileName)
	}

	// Roles.
	fmt.Fprintln(env.Stdout, "\nroles")
	// The explainer is listed beside the review layers because it is a role in
	// the same configuration. Reporting only the three that review would leave
	// the one role whose chain is easiest to misconfigure invisible.
	for _, roleName := range []string{"r1", "r2", "r3", ExplainRole} {
		names, prov, _ := cfg.Get("roles." + roleName + ".backends")
		if strings.TrimSpace(names) == "" {
			fmt.Fprintf(env.Stdout, "  %-10s no chain configured  [%s]\n", roleName, prov.Layer)
			continue
		}
		fmt.Fprintf(env.Stdout, "  %-10s %s  [%s]\n", roleName, names, prov.Layer)
		for _, name := range strings.Split(names, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			fmt.Fprintf(env.Stdout, "               %s\n", describeBackend(env, cfg, name))
		}
	}

	// The cross-provider claim, and how strongly it can be made.
	fmt.Fprintln(env.Stdout, "\ncross-provider")
	head, _ := repo.CurrentBranch()
	if decl, ok := repo.AuthorDeclaration(head); ok {
		fmt.Fprintf(env.Stdout, "  author     %s / %s (declared on %s)\n", decl.Provider, orUnset(decl.Model), head)
	} else {
		fmt.Fprintf(env.Stdout, "  author     not declared on %s; the state can be no better than `unknown`\n", head)
		fmt.Fprintf(env.Stdout, "             run `mf author declare --provider <name> --model <id>`\n")
	}
	if fp := cfg.Fingerprints(); len(fp) == 0 {
		fmt.Fprintln(env.Stdout, "  corroborate no fingerprints configured, so `verified` is unreachable and `declared` is the best case")
	} else {
		for envVar, provider := range fp {
			set := "unset"
			if env.Getenv(envVar) != "" {
				set = "set"
			}
			fmt.Fprintf(env.Stdout, "  corroborate $%s -> %s (%s)\n", envVar, provider, set)
		}
	}

	// Credentials: the configuration holds a variable name, so the only useful
	// question is whether that variable actually carries anything here.
	fmt.Fprintln(env.Stdout, "\ncredentials")
	printed := false
	if cfg.Machine != nil {
		for name, p := range cfg.Machine.Providers {
			if p.APIKeyEnv == "" {
				continue
			}
			printed = true
			if env.Getenv(p.APIKeyEnv) == "" {
				fmt.Fprintf(env.Stdout, "  %-10s $%s is unset; this provider will report unavailable\n", name, p.APIKeyEnv)
				continue
			}
			fmt.Fprintf(env.Stdout, "  %-10s $%s is set\n", name, p.APIKeyEnv)
		}
	}
	if !printed {
		fmt.Fprintln(env.Stdout, "  none configured")
	}

	// Spend, and whether a pinned model still matches the configuration.
	fmt.Fprintln(env.Stdout, "\nusage")
	if store := usageStore(env); store.Path != "" {
		totals := store.Read()
		fmt.Fprintf(env.Stdout, "  total      %d run(s): %s\n", totals.Runs, totals.Usage)
		if line, ok := costLine(cfg, totals.Usage); ok {
			fmt.Fprintf(env.Stdout, "  %s\n", line)
		}
	}
	for _, line := range modelDriftLines(env, cfg) {
		fmt.Fprintf(env.Stdout, "  models     %s\n", line)
	}

	// What the Token Economy names, against what exists here. `caveman-compress`
	// is listed precisely because it does not exist: naming a capability the
	// framework lacks is honest only while something says plainly it is absent.
	fmt.Fprintln(env.Stdout, "\ntoken economy")
	for _, c := range style.Capabilities() {
		state := "NOT IMPLEMENTED"
		if c.Implemented {
			state = "implemented"
		}
		fmt.Fprintf(env.Stdout, "  %-18s %-15s %s\n", c.Name, state, c.Note)
	}

	// Standards drift.
	fmt.Fprintln(env.Stdout, "\nstandards")
	if rep, err := upgrade.Compare(env.RepoRoot, version.Version); err == nil {
		fmt.Fprintf(env.Stdout, "  %s\n", rep.Summary())
	} else {
		fmt.Fprintf(env.Stdout, "  could not compare: %v\n", err)
	}

	// Reports only. Nothing above prevents work, so nothing above fails.
	return 0
}

func describeBackend(env Env, cfg *config.Config, name string) string {
	b, err := buildBackend(env, cfg, name)
	if err != nil {
		return fmt.Sprintf("%-12s not defined in any configuration layer", name)
	}
	kind, _, _ := cfg.Get("backends." + name + ".kind")
	model, _, _ := cfg.Get("backends." + name + ".model")
	if model == "" {
		model, _, _ = cfg.Get("review.model")
	}
	detail := ""
	if command, _, ok := cfg.Get("backends." + name + ".command"); ok && command != "" {
		if _, lookErr := exec.LookPath(command); lookErr != nil {
			detail = fmt.Sprintf("  (%s not on PATH)", command)
		} else {
			detail = fmt.Sprintf("  (%s found)", command)
		}
	}
	return fmt.Sprintf("%-12s kind=%s provider=%s model=%s%s", name, kind, b.Provider(), orUnset(model), detail)
}

func orUnset(s string) string {
	if s == "" {
		return "<unset>"
	}
	return s
}

func runInit(env Env) int {
	steps, err := activate.Init(activate.InitOptions{
		RepoRoot:         env.RepoRoot,
		FrameworkVersion: version.Version,
	})
	for _, s := range steps {
		marker := "  "
		if s.Changed {
			marker = "+ "
		}
		fmt.Fprintf(env.Stdout, "%s%-14s %s\n", marker, s.Name, s.Message)
	}
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf init: %v\n", err)
		return 1
	}
	fmt.Fprintln(env.Stdout, "\nNext: declare the Author for this branch so R2 can say more than `unknown`:")
	fmt.Fprintln(env.Stdout, "  mf author declare --provider <name> --model <id>")
	return 0
}

func runHooks(env Env, args []string) int {
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}
	switch action {
	case "status":
		state := activate.HooksStatus(env.RepoRoot)
		fmt.Fprintf(env.Stdout, "hooks path: %s\n", orUnset(state.Path))
		fmt.Fprintf(env.Stdout, "canonical:  %v\n", state.Canonical)
		fmt.Fprintf(env.Stdout, "directory:  present=%v\n", state.Present)
		return 0
	case "install":
		if err := activate.InstallHooks(env.RepoRoot); err != nil {
			fmt.Fprintf(env.Stderr, "mf hooks install: %v\n", err)
			return 1
		}
		fmt.Fprintf(env.Stdout, "core.hooksPath -> %s\n", activate.HooksDir)
		return 0
	case "uninstall":
		if err := activate.UninstallHooks(env.RepoRoot); err != nil {
			fmt.Fprintf(env.Stderr, "mf hooks uninstall: %v\n", err)
			return 1
		}
		fmt.Fprintln(env.Stdout, "core.hooksPath removed; the versioned directory is untouched")
		return 0
	}
	fmt.Fprintf(env.Stderr, "mf hooks: unknown action %q (expected install, uninstall or status)\n", action)
	return 2
}

func runUpgrade(env Env) int {
	rep, err := upgrade.Compare(env.RepoRoot, version.Version)
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf upgrade: %v\n", err)
		return 1
	}
	if rep.Note != "" {
		fmt.Fprintf(env.Stdout, "%s\n", rep.Note)
	}
	if rep.LockedVersion != "" {
		fmt.Fprintf(env.Stdout, "adopted %s, running %s\n", rep.LockedVersion, rep.RunningVersion)
	}
	fmt.Fprintf(env.Stdout, "%s\n", rep.Summary())
	for _, c := range rep.Changes {
		fmt.Fprintf(env.Stdout, "  %-28s %s\n", c.File, c.Status)
	}
	if len(rep.Changes) > 0 {
		// Applying a release over an adopter's standards destroys the local
		// intent that made adopting them worthwhile.
		fmt.Fprintln(env.Stdout, "\nNothing was applied. Your standards are your content; merge what you want.")
	}
	return 0
}

func runAuthor(env Env, args []string) int {
	if len(args) == 0 || args[0] != "declare" {
		fmt.Fprintln(env.Stderr, "mf author: expected `declare`")
		return 2
	}
	provider, model := "", ""
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--provider":
			if i+1 < len(rest) {
				i++
				provider = rest[i]
			}
		case "--model":
			if i+1 < len(rest) {
				i++
				model = rest[i]
			}
		default:
			fmt.Fprintf(env.Stderr, "mf author declare: unknown option %q\n", rest[i])
			return 2
		}
	}
	if provider == "" {
		fmt.Fprintln(env.Stderr, "mf author declare: --provider is required; it is the claim R2 is checked against")
		return 2
	}
	repo := vcs.Open(env.RepoRoot)
	head, err := repo.CurrentBranch()
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf author declare: %v\n", err)
		return 1
	}
	// Recorded per branch and while the change is being authored, because a
	// push carries commits that may come from several sessions and has no
	// single Author to detect.
	if err := repo.SetAuthorDeclaration(head, vcs.Declaration{Provider: provider, Model: model}); err != nil {
		fmt.Fprintf(env.Stderr, "mf author declare: %v\n", err)
		return 1
	}
	fmt.Fprintf(env.Stdout, "declared %s / %s as the Author of %s\n", provider, orUnset(model), head)
	return 0
}
