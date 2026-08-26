package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/LukeSantossz/my-framework/internal/activate"
	"github.com/LukeSantossz/my-framework/internal/agents"
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

	// The standards comparison is computed here rather than beside the section
	// that prints it, because the activation report needs one of its answers:
	// whether the version this repository adopted is the version running.
	rep, repErr := upgrade.Compare(env.RepoRoot, standardsDir(cfg), version.Version)

	// Activation.
	fmt.Fprintln(env.Stdout, "activation")
	state := activate.HooksStatus(env.RepoRoot)
	// Whether the versioned directory exists is asked before what the setting
	// says, because it is the fact that decides whether any hook can run at all.
	// Asked the other way round, a core.hooksPath inherited from a user's global
	// configuration made every repository on the machine report a gate it did
	// not have — and `mf init` then declined to wire anything, believing the job
	// done.
	switch {
	case !state.Present && state.Canonical:
		fmt.Fprintf(env.Stdout, "  hooks      core.hooksPath points at %s, but there is no %s directory in this repository — no hook runs%s\n",
			activate.HooksDir, activate.HooksDir, inheritedNote(state))
	case !state.Present:
		fmt.Fprintf(env.Stdout, "  hooks      no %s directory in this repository, so there is no push gate\n", activate.HooksDir)
	case state.Canonical && !state.Local:
		fmt.Fprintf(env.Stdout, "  hooks      wired to %s by a setting outside this repository; no clone inherits it — run `mf hooks install`\n",
			activate.HooksDir)
	case state.Canonical:
		fmt.Fprintf(env.Stdout, "  hooks      wired to %s\n", activate.HooksDir)
	case state.Path != "":
		fmt.Fprintf(env.Stdout, "  hooks      core.hooksPath points at %q, not %s — the gate does not run%s\n",
			state.Path, activate.HooksDir, inheritedNote(state))
	default:
		fmt.Fprintf(env.Stdout, "  hooks      not wired; run `mf hooks install`\n")
	}
	// A core.hooksPath replaces .git/hooks rather than adding to it. Nothing
	// said so anywhere, and the hooks it silences were installed on purpose.
	if state.Path != "" {
		if shadowed := activate.ShadowedLocalHooks(env.RepoRoot); len(shadowed) > 0 {
			fmt.Fprintf(env.Stdout, "  git hooks  core.hooksPath replaces .git/hooks, so %s no longer runs\n",
				strings.Join(shadowed, ", "))
		}
	}
	switch {
	case rep.LockedVersion == "":
		fmt.Fprintf(env.Stdout, "  adopted    no %s; run `mf init`\n", activate.LockFileName)
	case rep.VersionMismatch():
		fmt.Fprintf(env.Stdout, "  adopted    framework %s, which is not the build running (%s) — `mf upgrade` compares them\n",
			rep.LockedVersion, rep.RunningVersion)
	default:
		fmt.Fprintf(env.Stdout, "  adopted    framework %s\n", rep.LockedVersion)
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
	if repErr == nil {
		fmt.Fprintf(env.Stdout, "  %s\n", rep.Summary())
	} else {
		fmt.Fprintf(env.Stdout, "  could not compare: %v\n", repErr)
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

// inheritedNote names the scope a hooks path came from when it is not this
// repository's. A value nobody can find in the repository is one people look
// for in the wrong file.
func inheritedNote(state activate.HooksState) string {
	if state.Local || state.Path == "" {
		return ""
	}
	return " (set outside this repository, so it applies to every repository on this machine)"
}

// chosenProvider is the reviewer an adopter names at activation time.
//
// Which provider reviews their code is theirs to pick, so nothing here ships a
// default one: without these flags init writes no machine state at all, and
// with them it writes each half into the layer that owns it — the route on the
// machine, the chain that names it in committed policy (docs/adr/0006).
type chosenProvider struct {
	Name      string
	Endpoint  string
	APIKeyEnv string
	Model     string
	Kind      string
}

func (c chosenProvider) named() bool { return c.Name != "" }

func runInit(env Env, args []string) int {
	var chosen chosenProvider
	// Where the standards belong, when the adopter names it. It settles a
	// question init otherwise answers from what it can see, so it is read
	// before anything is written and skips the detection entirely.
	var namedStandards string
	for i := 0; i < len(args); i++ {
		var into *string
		switch args[i] {
		case "--provider":
			into = &chosen.Name
		case "--endpoint":
			into = &chosen.Endpoint
		case "--api-key-env":
			into = &chosen.APIKeyEnv
		case "--model":
			into = &chosen.Model
		case "--kind":
			into = &chosen.Kind
		case "--standards":
			into = &namedStandards
		default:
			fmt.Fprintf(env.Stderr, "mf init: unknown option %q\n", args[i])
			return 2
		}
		value, ok := optionValue(env, "mf init", args, &i)
		if !ok {
			return 2
		}
		*into = value
	}
	if code := checkChosen(env, chosen); code != 0 {
		return code
	}

	// Loaded before anything is written, for one value: where this repository
	// keeps its standards. A repository that already configures a corpus — the
	// submodule case — must not be handed a second one under docs/standards.
	cfg, code := load(env)
	if code != 0 {
		return code
	}

	// init materialises the shipped source and then generates from the
	// configured one, so the two have to name the same file. Only the directory
	// is free: the basename is what this build carries, and writing one name
	// while reading another is the defect this guard exists to stop, one level
	// below the directory it was already fixed at.
	source := filepath.ToSlash(agentsSource(cfg))
	if base := path.Base(source); base != path.Base(agents.SourcePath) {
		fmt.Fprintf(env.Stderr,
			"mf init: paths.agents_source names %q, and this build carries %q; the directory is yours to choose, the filename is not\n",
			base, path.Base(agents.SourcePath))
		return 1
	}
	// An environment override lasts one run, and the scaffold this run writes
	// does not record it. Materialising there would put the source somewhere
	// the next command, without the variable set, cannot find. Activation is a
	// durable act; a per-run value is not a place to perform it.
	if _, prov, ok := cfg.Get("paths.agents_source"); ok && prov.Layer == config.LayerEnv {
		fmt.Fprintln(env.Stderr,
			"mf init: paths.agents_source is set for this run only; set it in "+config.ProjectFileName+" so the next command finds the source too")
		return 1
	}
	standards, source, submodule, code := resolveStandards(env, cfg, namedStandards)
	if code != 0 {
		return code
	}
	sourceDir := path.Dir(filepath.ToSlash(source))

	steps, err := activate.Init(activate.InitOptions{
		RepoRoot:           env.RepoRoot,
		FrameworkVersion:   version.Version,
		StandardsDir:       standards,
		StandardsSubmodule: submodule,
		R2Backend:          chosen.Name,
		AgentsSourceDir:    sourceDir,
	})
	if chosen.named() && err == nil {
		// After the scaffold, so a machine write cannot leave a repository
		// half-activated if it fails.
		step, writeErr := recordProvider(env, chosen)
		steps = append(steps, step)
		if writeErr != nil {
			err = writeErr
		}
	}
	if err == nil {
		generated := generateAgentFiles(env)
		steps = append(steps, generated...)
		// A step that reported a failure is a failure, whatever the ones before
		// it did. Exiting zero here left an adopter with no instruction files and
		// a success message, which is the shape of activation this framework
		// treats as the worst outcome.
		for _, g := range generated {
			if strings.HasPrefix(g.Message, "not generated") || strings.HasPrefix(g.Message, "cannot read") {
				err = errors.New(g.Message)
			}
		}
	}
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
	fmt.Fprintln(env.Stdout, "\nThen `mf doctor` reports what still has no route: a role with an empty chain")
	fmt.Fprintf(env.Stdout, "reports that it did not run, and %s says how to give it one.\n", config.ProjectFileName)
	return 0
}

// generateAgentFiles writes the vendor instruction files the freshly scaffolded
// configuration declares.
//
// It is done here rather than inside activate because the targets are
// configuration, and it generates only files that are absent: `mf agents sync`
// is the command whose job is to overwrite them, and doing that as a side
// effect of `mf init` would replace a CLAUDE.md a person wrote with one derived
// from a source they have never seen. What it leaves alone it names, with the
// command that would regenerate it.
//
// The configuration is re-read rather than reused, because the scaffold this
// run may have just written is the file that declares the targets.
func generateAgentFiles(env Env) []activate.Step {
	cfg, err := config.Load(env.configOptions())
	if err != nil {
		return []activate.Step{{Name: "agent files", Message: "cannot read the configuration that declares them: " + err.Error()}}
	}
	targets := agentTargets(cfg)
	if len(targets) == 0 {
		return nil
	}
	var pending []agents.Target
	var kept []string
	for _, t := range targets {
		if _, statErr := os.Stat(filepath.Join(env.RepoRoot, filepath.FromSlash(t.File))); statErr == nil {
			kept = append(kept, t.File)
			continue
		}
		pending = append(pending, t)
	}

	var steps []activate.Step
	if len(pending) > 0 {
		results, syncErr := agents.Sync(agents.Options{RepoRoot: env.RepoRoot, Targets: pending, SourcePath: agentsSource(cfg)})
		if syncErr != nil {
			return append(steps, activate.Step{Name: "agent files", Message: "not generated: " + syncErr.Error()})
		}
		var written []string
		for _, r := range results {
			written = append(written, r.File)
		}
		steps = append(steps, activate.Step{Name: "agent files", Changed: true, Message: "generated " + strings.Join(written, ", ") + " from " + agentsSource(cfg)})
	}
	if len(kept) > 0 {
		steps = append(steps, activate.Step{Name: "agent files", Message: "left untouched: " + strings.Join(kept, ", ") +
			" — run `mf agents sync` to generate over what is already there"})
	}
	return steps
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
		// Where the value came from is printed beside it: a path set outside the
		// repository applies to every repository on this machine and travels
		// with none of them, which is a different state from this repository
		// having chosen it.
		fmt.Fprintf(env.Stdout, "set here:   %v\n", state.Local)
		fmt.Fprintf(env.Stdout, "canonical:  %v\n", state.Canonical)
		fmt.Fprintf(env.Stdout, "directory:  present=%v\n", state.Present)
		return 0
	case "install":
		if err := activate.InstallHooks(env.RepoRoot); err != nil {
			fmt.Fprintf(env.Stderr, "mf hooks install: %v\n", err)
			return 1
		}
		fmt.Fprintf(env.Stdout, "core.hooksPath -> %s\n", activate.HooksDir)
		if shadowed := activate.ShadowedLocalHooks(env.RepoRoot); len(shadowed) > 0 {
			fmt.Fprintf(env.Stdout, "core.hooksPath replaces .git/hooks rather than adding to it, so %s no longer runs.\n",
				strings.Join(shadowed, ", "))
		}
		return 0
	case "uninstall":
		state := activate.HooksStatus(env.RepoRoot)
		if err := activate.UninstallHooks(env.RepoRoot); err != nil {
			fmt.Fprintf(env.Stderr, "mf hooks uninstall: %v\n", err)
			return 1
		}
		if !state.Local {
			fmt.Fprintln(env.Stdout, "core.hooksPath was not set by this repository; nothing to remove")
			return 0
		}
		fmt.Fprintln(env.Stdout, "core.hooksPath removed; the versioned directory is untouched")
		if after := activate.HooksStatus(env.RepoRoot); after.Path != "" {
			fmt.Fprintf(env.Stdout, "A setting outside this repository still points at %q.\n", after.Path)
		}
		return 0
	}
	fmt.Fprintf(env.Stderr, "mf hooks: unknown action %q (expected install, uninstall or status)\n", action)
	return 2
}

func runUpgrade(env Env) int {
	// The configuration is loaded for one value: where this repository keeps
	// its standards. A repository whose configuration does not load is one
	// whose standards location is unknown, and comparing the default location
	// anyway would report an adopter's whole corpus as missing.
	cfg, code := load(env)
	if code != 0 {
		return code
	}
	rep, err := upgrade.Compare(env.RepoRoot, standardsDir(cfg), version.Version)
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

// checkChosen refuses half a route.
//
// A provider named without a way to reach it resolves, gets named in a chain,
// and reports itself unavailable on every run for a reason nothing states —
// worse than not having been configured at all, because it looks configured.
func checkChosen(env Env, c chosenProvider) int {
	if !c.named() {
		for flag, value := range map[string]string{
			"--endpoint": c.Endpoint, "--api-key-env": c.APIKeyEnv,
			"--model": c.Model, "--kind": c.Kind,
		} {
			if value != "" {
				fmt.Fprintf(env.Stderr, "mf init: %s describes a provider, so --provider has to name one\n", flag)
				return 2
			}
		}
		return 0
	}
	for _, missing := range []struct{ flag, value string }{
		{"--endpoint", c.Endpoint},
		{"--api-key-env", c.APIKeyEnv},
	} {
		if missing.value == "" {
			fmt.Fprintf(env.Stderr, "mf init: --provider %s needs %s; a provider with no route is a backend that reports itself unavailable on every run\n", c.Name, missing.flag)
			return 2
		}
	}
	return 0
}

// recordProvider writes the adopter's chosen route into the machine layer.
//
// Every value goes through config.Set rather than being formatted here, so the
// refusals that guard a committed file — a credential instead of a variable
// name, a key that belongs in the other layer — guard this path too.
func recordProvider(env Env, c chosenProvider) (activate.Step, error) {
	kind := c.Kind
	if kind == "" {
		kind = "openai-compatible"
	}
	writes := []struct{ key, value string }{
		{"providers." + c.Name + ".endpoint", c.Endpoint},
		{"providers." + c.Name + ".api_key_env", c.APIKeyEnv},
		{"providers." + c.Name + ".kind", kind},
		{"backends." + c.Name + ".kind", "api"},
		{"backends." + c.Name + ".provider", c.Name},
		{"backends." + c.Name + ".model", c.Model},
	}
	for _, w := range writes {
		if w.value == "" {
			continue
		}
		if err := config.Set(env.configOptions(), w.key, w.value, config.TargetMachine); err != nil {
			return activate.Step{Name: "provider", Message: err.Error()}, err
		}
	}
	return activate.Step{
		Name:    "provider",
		Changed: true,
		Message: fmt.Sprintf("recorded %s on this machine; %s names it in the R2 chain", c.Name, config.ProjectFileName),
	}, nil
}

// resolveStandards decides where this repository's standards live, before init
// writes anything that depends on the answer.
//
// The default is only a default. A repository that vendors the corpus as a
// submodule already has one, and materialising the shipped layout beside it
// gives that repository two corpora — the gates read one, `mf agents sync`
// generates references into it, and the submodule the adopter actually updates
// is the other. The existing guard cannot catch this: it asks whether the
// *configured* directory is inside a submodule, and a repository being adopted
// has configured nothing yet.
//
// So the question is settled from evidence. A checked-out submodule carrying a
// corpus is the answer. A declared submodule that is not checked out is not
// evidence of anything, and is refused rather than guessed at, because the
// wrong guess is the one that cannot be undone by re-running the command.
func resolveStandards(env Env, cfg *config.Config, named string) (dir, source, sub string, code int) {
	dir, source = standardsDir(cfg), agentsSource(cfg)
	if named != "" {
		return named, source, "", 0
	}
	// A repository that already configures a corpus has answered the question
	// itself, in the layer that outranks the default.
	if _, prov, ok := cfg.Get("paths.standards"); ok && prov.Layer != config.LayerDefault {
		return dir, source, "", 0
	}
	vendored, ok := activate.VendoredStandards(env.RepoRoot)
	if !ok {
		return dir, source, "", 0
	}
	if !vendored.Populated {
		fmt.Fprintf(env.Stderr,
			"mf init: %s is declared as a submodule and is not checked out, so this cannot tell whether it supplies the standards.\n"+
				"  If it does:      git submodule update --init %s, then run `mf init` again\n"+
				"  If it does not:  mf init --standards <dir> names where the standards belong\n"+
				"Nothing was written.\n",
			vendored.Dir, vendored.Dir)
		return "", "", "", 1
	}
	if !vendored.AgentSource {
		// The corpus is there and the source beside it is not, which is what a
		// pin older than the source looks like. Materialising one here would
		// put an untracked file in that submodule; materialising it outside
		// would generate the instruction files from a copy the adopter never
		// updates. Both are worse than saying so before anything is written.
		fmt.Fprintf(env.Stderr,
			"mf init: %s supplies the standards but carries no %s, so there is nothing to"+
				" generate the instruction files from.\n"+
				"  Update it:  git -C %s submodule update --remote %s\n"+
				"  Or:         mf init --standards <dir> to keep the standards somewhere this repository owns\n"+
				"Nothing was written.\n",
			vendored.Dir, config.DefaultAgentsSource, env.RepoRoot, vendored.Dir)
		return "", "", "", 1
	}
	return vendored.Dir + "/" + config.DefaultStandardsDir,
		vendored.Dir + "/" + config.DefaultAgentsSource,
		vendored.Dir, 0
}
