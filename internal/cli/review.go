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
	"github.com/LukeSantossz/my-framework/internal/vcs"
)

// runReview executes one role's chain against the current branch.
//
// It exits zero on findings. Every layer is advisory per ai_guidelines.md, and
// a reviewer that never ran is not a finding at all — an expired quota must not
// lock a repository.
func runReview(env Env, args []string) int {
	roleName := "r2"
	base := ""
	dryRun := false
	prNumber := 0
	post := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--role":
			if i+1 < len(args) {
				i++
				roleName = args[i]
			}
		case "--base":
			if i+1 < len(args) {
				i++
				base = args[i]
			}
		case "--dry-run":
			dryRun = true
		case "--post":
			post = true
		case "--pr":
			if i+1 < len(args) {
				i++
				n, convErr := strconv.Atoi(args[i])
				if convErr != nil || n <= 0 {
					fmt.Fprintf(env.Stderr, "mf review: --pr expects a pull request number, got %q\n", args[i])
					return 2
				}
				prNumber = n
			}
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

	maxBytes := intValue(cfg, "review.max_diff_bytes", 30000)
	req := backend.Request{
		Role: roleName, Base: base, Head: head,
		Model:        stringValue(cfg, "review.model", ""),
		Effort:       stringValue(cfg, "review.effort", config.DefaultEffort),
		Instructions: readInstructions(env.RepoRoot) + pullBody,
	}

	runner := &role.Runner{
		Role:                 roleName,
		Chain:                chain,
		RequireCrossProvider: roleName == "r2",
	}
	if decl, ok := repo.AuthorDeclaration(head); ok {
		runner.Declaration = &decl
	}
	runner.Fingerprint = detectFingerprint(cfg, env.Getenv)

	if dryRun {
		// Describe the whole chain rather than stopping at the first backend:
		// the point is to show what would happen, fallbacks included.
		fmt.Fprintf(env.Stdout, "[%s] %s against %s\n", roleName, head, base)
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
	// Findings never fail the run. Every layer is advisory, and a blocking R3
	// would make the reviewer with the least context the strictest gate.
	return 0
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
	var spec config.Backend
	if cfg.Project != nil {
		if b, ok := cfg.Project.Backends[name]; ok {
			spec = b
		}
	}
	if spec.Kind == "" {
		// A backend the chain names but nothing defines is a misconfiguration,
		// not an unavailable reviewer: reporting it as unavailable would let a
		// typo look like a vendor outage.
		return nil, fmt.Errorf("mf review: role chain names backend %q, which no configuration defines", name)
	}

	switch spec.Kind {
	case "cli":
		return &backend.CLI{
			BackendName: name, ProviderName: spec.Provider,
			Command: spec.Command, Args: spec.Args, Patterns: spec.UnavailablePatterns,
			WorkDir: env.RepoRoot,
			Model:   spec.Model, Effort: spec.Effort,
		}, nil

	case "api":
		var provider config.Provider
		if cfg.Machine != nil {
			provider = cfg.Machine.Providers[spec.Provider]
		}
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
			Budget:   time.Duration(intValue(cfg, "review.timeout_seconds", 240)) * time.Second,
			Model:    spec.Model, Effort: spec.Effort,
		}, nil

	case "in-session":
		return &backend.InSession{BackendName: name, ProviderName: spec.Provider}, nil

	case "inproc":
		return &backend.InProc{BackendName: name}, nil

	case "external":
		return &backend.External{BackendName: name, ProviderName: spec.Provider}, nil
	}
	return nil, fmt.Errorf("mf review: backend %q has unknown kind %q", name, spec.Kind)
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
// backend finds AGENTS.md itself; a non-agentic one has it sent.
func readInstructions(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

func stringValue(cfg *config.Config, key, fallback string) string {
	if v, _, ok := cfg.Get(key); ok && v != "" {
		return v
	}
	return fallback
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
