package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LukeSantossz/my-framework/internal/backend"
	"github.com/LukeSantossz/my-framework/internal/config"
	"github.com/LukeSantossz/my-framework/internal/forge"
	"github.com/LukeSantossz/my-framework/internal/report"
	"github.com/LukeSantossz/my-framework/internal/role"
	"github.com/LukeSantossz/my-framework/internal/style"
	"github.com/LukeSantossz/my-framework/internal/vcs"
)

// runReview executes one role's chain against the current branch.
//
// It exits zero on findings by default. Every layer is advisory per
// ai_guidelines.md, and a reviewer that never ran is not a finding at all — an
// expired quota must not lock a repository. A role whose `blocking` flag some
// layer turned on is the one exception, and only for findings the reviewer
// itself classed as blocking: see blockedBy below.
func runReview(env Env, args []string) int {
	roleName := "r2"
	base := ""
	dryRun := false
	prNumber := 0
	post := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--role":
			value, ok := optionValue(env, "mf review", args, &i)
			if !ok {
				return 2
			}
			roleName = value
		case "--base":
			value, ok := optionValue(env, "mf review", args, &i)
			if !ok {
				return 2
			}
			base = value
		case "--dry-run":
			dryRun = true
		case "--post":
			post = true
		case "--pr":
			value, ok := optionValue(env, "mf review", args, &i)
			if !ok {
				return 2
			}
			n, convErr := strconv.Atoi(value)
			if convErr != nil || n <= 0 {
				fmt.Fprintf(env.Stderr, "mf review: --pr expects a pull request number, got %q\n", value)
				return 2
			}
			prNumber = n
		default:
			fmt.Fprintf(env.Stderr, "mf review: unknown option %q\n", args[i])
			return 2
		}
	}
	switch roleName {
	case "r1", "r2", "r3":
	default:
		fmt.Fprintf(env.Stderr, "mf review: unknown role %q (expected r1, r2 or r3)\n", roleName)
		return 2
	}
	// Checked before anything runs. Validating it later would make the error
	// depend on whether a backend happened to be available, so the same wrong
	// command would sometimes be caught and sometimes pass in silence.
	if post && prNumber == 0 {
		fmt.Fprintln(env.Stderr, "mf review: --post needs --pr <number>; there is nowhere to post without one")
		return 2
	}

	cfg, code := load(env)
	if code != 0 {
		return code
	}
	if base == "" {
		base, _, _ = cfg.Get("review.base")
	}

	repo := vcs.Open(env.RepoRoot)
	head, err := repo.CurrentBranch()
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf review: cannot determine the current branch: %v\n", err)
		return 1
	}

	// R3's pull request context: the intent, which is the only thing it has that
	// R2 does not.
	var pullBody string
	var client *forge.Client
	headSHA := ""
	if prNumber > 0 {
		client = forgeClient(env)
		if client == nil {
			fmt.Fprintln(env.Stderr, "mf review: --pr needs GITHUB_REPOSITORY to name the repository")
			return 2
		}
		pr, prErr := client.PullRequest(prNumber)
		if prErr != nil {
			// Misconfiguration is the one thing this command fails on.
			fmt.Fprintf(env.Stderr, "mf review: %v\n", prErr)
			return 1
		}
		if pr.IsFork {
			// Secrets are unavailable to fork workflows by design. Saying so is
			// the honest outcome; exiting zero without a word would look like a
			// review that found nothing.
			fmt.Fprintf(env.Stdout, "[%s] pull request #%d comes from a fork, where the credentials this review needs are unavailable by design. R3 did not run.\n", roleName, prNumber)
			return 0
		}
		if pr.BaseRef != "" {
			base = pr.BaseRef
		}
		if pr.HeadRef != "" {
			head = pr.HeadRef
		}
		// The forge knows the commit under review; this clone may not even have
		// the branch. An attestation names a change, so where a real head SHA
		// exists it is the one to carry.
		headSHA = pr.HeadSHA
		pullBody = pullContext(pr, repo, base, head)
	}

	// Nothing to review when the branch is its own base. Answering this here
	// rather than in a backend keeps a false entry out of the PR's review
	// record: it is a property of the branch, not of any reviewer.
	if head == base {
		fmt.Fprintf(env.Stdout, "[%s] on base branch %q; nothing to review against itself.\n", roleName, base)
		return 0
	}

	chain, buildErr := buildChain(env, cfg, roleName)
	if buildErr != nil {
		fmt.Fprintln(env.Stderr, buildErr)
		return 1
	}

	// The same review is a conversation when it is printed to a terminal and a
	// pull request artifact when it is posted, so what the output becomes —
	// not which command produced it — decides whether terse style may apply.
	// token_economy.md §3 forbids it for the second, and this is where the
	// framework can enforce that rather than trust it.
	artifact := style.Conversation
	if post {
		artifact = style.PullRequest
	}
	instructions := readInstructions(env.RepoRoot, agentsFile(cfg)) + pullBody
	styleNote := "full prose"
	if styled, styleErr := style.Compose(instructions, artifact); styleErr == nil {
		instructions = styled
		styleNote = "terse"
	}

	if headSHA == "" {
		headSHA = tipCommit(repo, base, head)
	}
	maxBytes := intValue(cfg, "review.max_diff_bytes", 30000)
	req := backend.Request{
		Role: roleName, Base: base, Head: head,
		Model:        stringValue(cfg, "review.model", ""),
		Effort:       stringValue(cfg, "review.effort", config.DefaultEffort),
		Instructions: instructions,
		HeadSHA:      headSHA,
	}

	runner := &role.Runner{
		Role:  roleName,
		Chain: chain,
		// Read rather than assumed from the role's name. R2 is still the only
		// role that ships with the requirement, but the key decoded, resolved
		// and appeared in `mf config list` while nothing read it, so a project
		// that moved the rule onto R3, or off R2, was silently ignored.
		RequireCrossProvider: boolValue(cfg, "roles."+roleName+".require_cross_provider", roleName == "r2"),
	}
	if decl, ok := repo.AuthorDeclaration(head); ok {
		runner.Declaration = &decl
	}
	runner.Fingerprint = detectFingerprint(cfg, env.Getenv)

	if dryRun {
		// Describe the whole chain rather than stopping at the first backend:
		// the point is to show what would happen, fallbacks included.
		fmt.Fprintf(env.Stdout, "[%s] %s against %s\n", roleName, head, base)
		// Where the terse boundary becomes visible: the same chain, described
		// twice, differs only in what its output is going to become.
		fmt.Fprintf(env.Stdout, "  prompt style: %s (%s)\n", styleNote, artifact)
		for _, line := range runner.Describe(req) {
			fmt.Fprintf(env.Stdout, "  %s\n", line)
		}
		if len(chain) == 0 {
			fmt.Fprintf(env.Stdout, "  (no backends configured for role %s)\n", roleName)
		}
		return 0
	}

	diff, err := repo.Diff(base, head, maxBytes)
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf review: %v\n", err)
		return 1
	}
	if diff.Empty {
		fmt.Fprintf(env.Stdout, "[%s] %q adds nothing over %q; nothing to review.\n", roleName, head, base)
		return 0
	}
	req.Diff, req.Truncated = diff.Text, diff.Truncated

	out, err := runner.Run(context.Background(), req)
	if err != nil {
		fmt.Fprintf(env.Stderr, "mf review: %v\n", err)
		return 1
	}

	for _, s := range out.Skipped {
		fmt.Fprintf(env.Stdout, "[%s] skipped %s: %s\n", roleName, s.Backend, s.Reason)
	}
	if !out.Ran {
		// Not a finding, so it never blocks; recorded so the absence reaches the
		// PR instead of passing for a review that happened.
		fmt.Fprintf(env.Stdout, "[%s] did not run: no configured backend was available. Record the absence in the PR.\n", roleName)
		if post && client != nil {
			postComment(env, client, prNumber, out)
		}
		return 0
	}

	report.Render(env.Stdout, out.Result)
	if store := usageStore(env); store.Path != "" {
		// Accounting never fails a review: a total that could not be written is
		// a lost observation, not a lost review.
		_ = store.Add(out.Result.Usage)
	}
	if out.CrossProvider != role.StateNA {
		fmt.Fprintf(env.Stdout, "Cross-provider: %s (%s)\n", out.CrossProvider, out.CrossProviderNote)
		if !out.CrossProvider.Satisfies() {
			fmt.Fprintf(env.Stdout, "R2 is NOT satisfied by this run; note it in the PR.\n")
		}
	}
	if post && client != nil {
		postComment(env, client, prNumber, out)
	}
	return blockedBy(env, cfg, roleName, out)
}

// blockedBy decides whether this review stops its caller, and says so where the
// caller's user will see it.
//
// The default is 0 on everything: every layer is advisory, and a blocking R3
// would make the reviewer with the least context the strictest gate in the
// pipeline. `roles.<role>.blocking` is what a repository, a machine or a single
// run turns on to get the behaviour `R2_BLOCKING=1` used to buy from the shell
// runner — with two limits the shell also had, and for the same reasons.
//
// The first is that only a finding the reviewer classed as blocking counts.
// Severity is the reviewer's own judgement, and treating every advisory note as
// a stop sign would make the flag unusable and push people to `--no-verify`.
// Prose from a backend that declared no schema is advisory by construction, so
// a chain of those can raise findings and still never block; that is the honest
// reading, because nothing in that answer claimed a severity. A `cli` backend
// that declares `structured` is asked for the schema and is judged on what it
// answered, like any other.
//
// The second is that a chain that never ran never blocks. r2_gate.md is
// explicit about it and it is what keeps the gate from being a lock: an expired
// quota is not a finding, and a repository that cannot be pushed to because a
// vendor is down is worse than one pushed to without a review.
func blockedBy(env Env, cfg *config.Config, roleName string, out role.Outcome) int {
	if !boolValue(cfg, "roles."+roleName+".blocking", false) {
		return 0
	}
	if !out.Result.HasBlocking() {
		return 0
	}
	// The exit code is all a hook can read, so the reason is printed: several
	// different failures stop a push, and the one thing the reader needs to
	// know is which of them this was.
	fmt.Fprintf(env.Stdout,
		"[%s] blocking mode is on (roles.%s.blocking) and this review carries a blocking finding; stopping here.\n"+
			"Address it, or justify it in the pull request and re-run without the flag.\n",
		roleName, roleName)
	return 1
}

func postComment(env Env, client *forge.Client, prNumber int, out role.Outcome) {
	action, err := client.UpsertComment(prNumber, renderComment(out))
	if err != nil {
		// Posting is reporting, so failing to post must not turn an advisory
		// review into a failed build.
		fmt.Fprintf(env.Stderr, "mf review: could not post the comment: %v\n", err)
		return
	}
	fmt.Fprintf(env.Stdout, "comment %s on #%d\n", action, prNumber)
}

func buildChain(env Env, cfg *config.Config, roleName string) ([]backend.Backend, error) {
	names, _, _ := cfg.Get("roles." + roleName + ".backends")
	var chain []backend.Backend
	for _, name := range strings.Split(names, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		b, err := buildBackend(env, cfg, name)
		if err != nil {
			return nil, err
		}
		chain = append(chain, b)
	}
	return chain, nil
}

func buildBackend(env Env, cfg *config.Config, name string) (backend.Backend, error) {
	// The merged view, so a chain a project declares can be completed by a
	// backend only this machine defines — which is the whole point of the split:
	// the project names the reviewers it wants, and how any of them is reached
	// from here never enters the committed file.
	spec, _, ok := cfg.Backend(name)
	if !ok || spec.Kind == "" {
		// A backend the chain names but nothing defines is a misconfiguration,
		// not an unavailable reviewer: reporting it as unavailable would let a
		// typo look like a vendor outage.
		return nil, fmt.Errorf("mf review: role chain names backend %q, which no configuration defines", name)
	}

	// One budget for every kind that can hang. `review.timeout_seconds` is a
	// property of the review, not of the wire shape behind it, and applying it
	// to only some backends left `mf doctor` reporting a timeout that an
	// agentic reviewer never observed.
	budget := time.Duration(intValue(cfg, "review.timeout_seconds", 240)) * time.Second

	switch spec.Kind {
	case "cli":
		return &backend.CLI{
			BackendName: name, ProviderName: spec.Provider,
			Command: spec.Command, Args: spec.Args, Patterns: spec.UnavailablePatterns,
			WorkDir: env.RepoRoot,
			Budget:  budget,
			Model:   spec.Model, Effort: spec.Effort,
			Structured: spec.Structured,
		}, nil

	case "api":
		provider := cfg.Provider(spec.Provider)
		key := ""
		if provider.APIKeyEnv != "" {
			key = env.Getenv(provider.APIKeyEnv)
		}
		shape := backend.WireShape(provider.Kind)
		if shape == "" {
			shape = backend.WireOpenAI
		}
		return &backend.API{
			BackendName: name, ProviderName: spec.Provider,
			Shape:    shape,
			Endpoint: provider.Endpoint,
			APIKey:   key,
			Budget:   budget,
			Model:    spec.Model, Effort: spec.Effort,
		}, nil

	case "in-session":
		repo := vcs.Open(env.RepoRoot)
		return &backend.InSession{
			BackendName: name, ProviderName: spec.Provider,
			HasAttestation: func(role, headSHA string) bool { return hasAttestation(repo, role, headSHA) },
			HowToAttest:    howToAttest,
		}, nil

	case "inproc":
		return &backend.InProc{BackendName: name}, nil

	case "external":
		return &backend.External{BackendName: name, ProviderName: spec.Provider}, nil
	}
	return nil, fmt.Errorf("mf review: backend %q has unknown kind %q", name, spec.Kind)
}

// optionValue reads the value that follows a flag, advancing the index past it,
// and reports the flag as an error when there is nothing there.
//
// Dropping the value silently is worse than any wrong value, because the
// command then does something plausible: `mf review --role r3 --pr "$PR"
// --post` with an unset $PR reviewed the local branch, posted nothing and
// exited zero, which in a CI log reads exactly like R3 having run and found
// nothing.
func optionValue(env Env, command string, args []string, i *int) (string, bool) {
	if *i+1 >= len(args) {
		fmt.Fprintf(env.Stderr, "%s: %s expects a value\n", command, args[*i])
		return "", false
	}
	*i++
	return args[*i], true
}

// attestationKey is where a session records that it reviewed a change. It is
// repository-local git config for the same reason the Author Declaration is: it
// records what happened on this clone, and a value that travelled with the
// branch would assert something about sessions it never saw.
func attestationKey(role string) string {
	return "mf.attestation." + role
}

// hasAttestation reports whether a session recorded a review of this exact
// change. The commit is compared rather than the branch: an attestation for an
// earlier tip has not seen what is being pushed now, and a per-branch record
// would quietly cover every commit added after it.
func hasAttestation(repo *vcs.Repo, role, headSHA string) bool {
	if headSHA == "" {
		return false
	}
	recorded, err := repo.ConfigGet(attestationKey(role))
	return err == nil && recorded == headSHA
}

// howToAttest is what an in-session backend prints when no attestation exists.
// Writing the record is a plain git command rather than a subcommand of this
// tool, which is the same place the Author Declaration started from.
func howToAttest(role string) string {
	return fmt.Sprintf("record one from the session that reviewed: "+
		"git config --local %s $(git rev-parse HEAD)", attestationKey(role))
}

// tipCommit resolves the commit the review is about. It is the change's newest
// commit rather than the branch name, because that is what an attestation has
// to name to mean anything.
func tipCommit(repo *vcs.Repo, base, head string) string {
	commits, err := repo.Commits(base, head)
	if err != nil || len(commits) == 0 {
		return ""
	}
	return commits[len(commits)-1].SHA
}

// detectFingerprint corroborates an Author Declaration from the environment.
// The table is machine configuration and ships empty, so this normally returns
// nothing and the state is `declared` at best — which is the honest answer, not
// a defect.
func detectFingerprint(cfg *config.Config, getenv func(string) string) string {
	for envVar, provider := range cfg.Fingerprints() {
		if getenv(envVar) != "" {
			return provider
		}
	}
	return ""
}

// readInstructions loads the Reviewer's binding role description. An agentic
// backend finds the file itself; a non-agentic one has it sent.
//
// Which file that is comes from the configuration, because an adopter's vendor
// instruction files are their own: `[agents]` already lets a repository decide
// what it generates, and a reviewer sent a file that repository does not keep
// would be reviewing against instructions nobody wrote.
func readInstructions(root, name string) string {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return ""
	}
	return string(data)
}

// agentsFile resolves which instruction file the Reviewer is handed.
func agentsFile(cfg *config.Config) string {
	return stringValue(cfg, "paths.agents_file", config.DefaultAgentsFile)
}

func stringValue(cfg *config.Config, key, fallback string) string {
	if v, _, ok := cfg.Get(key); ok && v != "" {
		return v
	}
	return fallback
}

// boolValue reads a configured flag. A value that is not a boolean falls back
// rather than failing the run: the flags this reads decide how strong a claim a
// review may make, and a typo must not be able to stop the review happening.
func boolValue(cfg *config.Config, key string, fallback bool) bool {
	v, _, ok := cfg.Get(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return b
}

func intValue(cfg *config.Config, key string, fallback int) int {
	v, _, ok := cfg.Get(key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
