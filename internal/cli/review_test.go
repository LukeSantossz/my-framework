package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepo builds a real repository with a branch that adds a file, so the
// review path exercises the same plumbing it will in use.
func gitRepo(t *testing.T, projectBody string) (root string) {
	t.Helper()
	root = t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "t@example.invalid")
	run("config", "user.name", "T")
	run("config", "commit.gpgsign", "false")
	// git forks `gc --auto` after a commit, and it goes on writing into
	// .git/objects after the test body has returned. t.TempDir() then fails its
	// own cleanup with "directory not empty" — a flake that has nothing to do
	// with what the test asserts, and that took down a release build.
	run("config", "gc.auto", "0")
	run("config", "maintenance.auto", "false")
	write(t, filepath.Join(root, "seed.txt"), "seed\n")
	if projectBody != "" {
		write(t, filepath.Join(root, ".framework.toml"), projectBody)
	}
	run("add", ".")
	run("commit", "-m", "chore: seed")
	return root
}

func branchWithChange(t *testing.T, root, branch string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("checkout", "-b", branch)
	write(t, filepath.Join(root, "a.txt"), "hello\n")
	run("add", ".")
	run("commit", "-m", "feat: a")
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func reviewEnv(t *testing.T, root string, args ...string) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	return Env{
		Args:        args,
		Stdout:      &out,
		Stderr:      &errOut,
		RepoRoot:    root,
		MachinePath: filepath.Join(t.TempDir(), "config.toml"),
		Getenv:      func(string) string { return "" },
		GitConfig:   func(string) (string, bool) { return "", false },
	}, &out, &errOut
}

const chainProject = `
version = 1

[roles.r2]
backends = ["codex", "fallback"]

[backends.codex]
kind = "cli"
provider = "openai"
command = "definitely-not-installed-codex"
args = ["review", "--base", "{{.Base}}"]
unavailable_patterns = ["usage limit"]

[backends.fallback]
kind = "cli"
provider = "google"
command = "definitely-not-installed-gemini"
`

func TestReviewDryRunDescribesTheWholeChainAndRunsNothing(t *testing.T) {
	root := gitRepo(t, chainProject)
	branchWithChange(t, root, "feat/x")
	e, out, _ := reviewEnv(t, root, "review", "--role", "r2", "--dry-run")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	got := out.String()
	// Both backends, not just the first: the point of a dry run is the fallbacks.
	for _, want := range []string{"codex", "fallback", "--base", "main"} {
		if !strings.Contains(got, want) {
			t.Errorf("dry run output %q lacks %q", got, want)
		}
	}
}

func TestReviewSkipsWhenTheBranchIsItsOwnBase(t *testing.T) {
	// Answered here rather than by a backend: it is a property of the branch,
	// and announcing a review of nothing would be a false entry in the PR.
	root := gitRepo(t, chainProject)
	e, out, _ := reviewEnv(t, root, "review", "--role", "r2")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "nothing to review against itself") {
		t.Errorf("output %q does not explain the skip", out.String())
	}
}

func TestReviewExitsZeroAndNamesEveryBackendWhenNoneIsAvailable(t *testing.T) {
	// A reviewer that never ran is not a finding, so an expired quota or a
	// missing tool must never lock the repository.
	root := gitRepo(t, chainProject)
	branchWithChange(t, root, "feat/x")
	e, out, _ := reviewEnv(t, root, "review", "--role", "r2")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, out.String())
	}
	got := out.String()
	for _, want := range []string{"codex", "fallback", "did not run", "Record the absence"} {
		if !strings.Contains(got, want) {
			t.Errorf("output %q lacks %q", got, want)
		}
	}
}

func TestReviewRejectsAnUnknownRole(t *testing.T) {
	root := gitRepo(t, chainProject)
	e, _, errOut := reviewEnv(t, root, "review", "--role", "r9")
	if code := Run(e); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "r9") {
		t.Errorf("stderr %q does not name the bad role", errOut.String())
	}
}

func TestReviewReportsABackendTheChainNamesButNothingDefines(t *testing.T) {
	// A typo in a chain is a misconfiguration, not an unavailable reviewer.
	// Reporting it as unavailable would let it look like a vendor outage and
	// silently degrade the layer.
	project := "version = 1\n\n[roles.r2]\nbackends = [\"ghost\"]\n"
	root := gitRepo(t, project)
	branchWithChange(t, root, "feat/x")
	e, _, errOut := reviewEnv(t, root, "review", "--role", "r2")
	if code := Run(e); code == 0 {
		t.Error("exit 0 for a chain naming an undefined backend")
	}
	if !strings.Contains(errOut.String(), "ghost") {
		t.Errorf("stderr %q does not name the undefined backend", errOut.String())
	}
}

func TestReviewReportsAnEmptyChangeRatherThanReviewingNothing(t *testing.T) {
	root := gitRepo(t, chainProject)
	run := exec.Command("git", "checkout", "-b", "feat/empty")
	run.Dir = root
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v\n%s", err, out)
	}
	e, out, _ := reviewEnv(t, root, "review", "--role", "r2")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "adds nothing over") {
		t.Errorf("output %q does not report the empty change", out.String())
	}
}

func TestReviewReportsTheCrossProviderStateForR2Only(t *testing.T) {
	root := gitRepo(t, chainProject)
	branchWithChange(t, root, "feat/x")

	// r2 with nothing available still reports; the state line appears only once
	// a backend has reviewed, so here we assert the role gating via r1 instead.
	project := strings.Replace(chainProject, "[roles.r2]", "[roles.r1]", 1)
	root2 := gitRepo(t, project)
	branchWithChange(t, root2, "feat/x")
	e, out, _ := reviewEnv(t, root2, "review", "--role", "r1")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(out.String(), "Cross-provider") {
		t.Errorf("r1 reported a cross-provider state: %q", out.String())
	}
}

// gitIn runs one git command in a test repository and returns what it printed.
func gitIn(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

const inSessionProject = `
version = 1

[roles.r1]
backends = ["superpowers"]

[backends.superpowers]
kind = "in-session"
provider = "anthropic"
`

func TestReviewIsSatisfiedByAnAttestationRecordedForThisChange(t *testing.T) {
	// superpowers is the whole of R1's shipped chain. Until an attestation can
	// be both recorded and read, R1 can only ever report that it did not run.
	root := gitRepo(t, inSessionProject)
	branchWithChange(t, root, "feat/x")

	e, out, _ := reviewEnv(t, root, "review", "--role", "r1")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "did not run") {
		t.Fatalf("output %q does not report the absence", out.String())
	}
	if !strings.Contains(out.String(), "mf.attestation.r1") {
		t.Errorf("output %q names no way to record an attestation", out.String())
	}

	gitIn(t, root, "config", "--local", "mf.attestation.r1", gitIn(t, root, "rev-parse", "HEAD"))

	e, out, _ = reviewEnv(t, root, "review", "--role", "r1")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "Reviewed by: superpowers") {
		t.Errorf("output %q does not name the attesting backend as the one that reviewed", out.String())
	}
}

func TestAnAttestationForAnEarlierCommitDoesNotCoverTheChangeBeingPushed(t *testing.T) {
	// An attestation names a change, not a branch: a session that reviewed an
	// earlier tip has not seen what is being pushed now.
	root := gitRepo(t, inSessionProject)
	branchWithChange(t, root, "feat/x")
	gitIn(t, root, "config", "--local", "mf.attestation.r1", gitIn(t, root, "rev-parse", "HEAD"))

	write(t, filepath.Join(root, "b.txt"), "more\n")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-m", "feat: b")

	e, out, _ := reviewEnv(t, root, "review", "--role", "r1")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "did not run") {
		t.Errorf("output %q accepted a stale attestation", out.String())
	}
}

func TestReviewRejectsAFlagWhoseValueIsMissing(t *testing.T) {
	// `mf review --role r3 --pr "$PR" --post` with an unset $PR used to review
	// locally, post nothing and exit 0 — which in CI reads as "R3 ran and found
	// nothing".
	root := gitRepo(t, chainProject)
	branchWithChange(t, root, "feat/x")
	for _, args := range [][]string{
		{"review", "--role"},
		{"review", "--base"},
		{"review", "--dry-run", "--role"},
		{"review", "--role", "r3", "--pr"},
	} {
		e, _, errOut := reviewEnv(t, root, args...)
		if code := Run(e); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
		if !strings.Contains(errOut.String(), "expects a value") {
			t.Errorf("%v: stderr %q does not say the value is missing", args, errOut.String())
		}
	}
}

// --- what `mf init` scaffolds has to be a chain that works -------------------

// scaffoldProject is what `mf init` writes: every chain declared and empty,
// because a fresh repository has configured no reviewer yet.
const scaffoldProject = `
version = 1

[roles.r1]
backends = []

[roles.r2]
backends = []
require_cross_provider = true

[roles.r3]
backends = []
`

func TestReviewOnTheInitScaffoldReportsNoChainRatherThanAnUnknownBackend(t *testing.T) {
	// The documented adoption path, `mf init` then `mf review`. It used to fail
	// naming "codex" — a backend the adopter never typed, in a file they could
	// not find it in — because the built-in chain survived a project file that
	// had declared the chain empty on purpose.
	root := gitRepo(t, scaffoldProject)
	branchWithChange(t, root, "feat/x")

	e, out, errOut := reviewEnv(t, root, "review", "--role", "r2", "--dry-run")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "no backends configured") {
		t.Errorf("dry run %q does not report the empty chain", out.String())
	}
	if strings.Contains(errOut.String(), "codex") {
		t.Errorf("stderr %q still names the built-in chain the project erased", errOut.String())
	}

	e, out, errOut = reviewEnv(t, root, "review", "--role", "r2")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "did not run") {
		t.Errorf("output %q does not record the absence", out.String())
	}
}

func TestReviewCompletesAProjectChainWithABackendOnlyTheMachineDefines(t *testing.T) {
	// docs/adr/0006's split, working: the project names the reviewer it wants
	// and the machine says how it is reached from here, with no endpoint and no
	// command ever entering the committed file.
	root := gitRepo(t, "version = 1\n\n[roles.r2]\nbackends = [\"local\"]\n")
	branchWithChange(t, root, "feat/x")

	e, out, errOut := reviewEnv(t, root, "review", "--role", "r2", "--dry-run")
	machine := filepath.Join(t.TempDir(), "config.toml")
	write(t, machine, "version = 1\n\n[backends.local]\nkind = \"cli\"\nprovider = \"local\"\ncommand = \"definitely-not-installed-reviewer\"\n")
	e.MachinePath = machine
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "local") {
		t.Errorf("dry run %q does not describe the machine's backend", out.String())
	}
}

// --- the cross-provider requirement is read from the configuration -----------

// attestingProject builds a chain of one in-session backend for a role, so a
// review actually happens and the cross-provider line can be observed.
func attestingProject(role, requirement string) string {
	return "version = 1\n\n[roles." + role + "]\nbackends = [\"superpowers\"]\n" +
		"require_cross_provider = " + requirement + "\n\n" +
		"[backends.superpowers]\nkind = \"in-session\"\nprovider = \"anthropic\"\n"
}

func TestReviewAppliesTheCrossProviderRequirementTheConfigurationDeclares(t *testing.T) {
	// `roles.<role>.require_cross_provider` decoded, resolved and showed up in
	// `mf config list`, and was read by nothing: the requirement was hardcoded
	// to the role's name, so setting it did nothing in either direction.
	root := gitRepo(t, attestingProject("r1", "true"))
	branchWithChange(t, root, "feat/x")
	gitIn(t, root, "config", "--local", "mf.attestation.r1", gitIn(t, root, "rev-parse", "HEAD"))

	e, out, _ := reviewEnv(t, root, "review", "--role", "r1")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "Cross-provider:") {
		t.Errorf("output %q ignores a requirement the project declared on r1", out.String())
	}
}

func TestReviewLetsAProjectTurnTheCrossProviderRequirementOff(t *testing.T) {
	root := gitRepo(t, attestingProject("r2", "false"))
	branchWithChange(t, root, "feat/x")
	gitIn(t, root, "config", "--local", "mf.attestation.r2", gitIn(t, root, "rev-parse", "HEAD"))

	e, out, _ := reviewEnv(t, root, "review", "--role", "r2")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if strings.Contains(out.String(), "Cross-provider:") {
		t.Errorf("output %q still enforces a requirement the project switched off", out.String())
	}
}

func TestReviewSendsTheInstructionsFileTheConfigurationNames(t *testing.T) {
	// The adopter that keeps its Reviewer instructions somewhere other than
	// AGENTS.md: the prompt-driven backend is handed whatever `paths.agents_file`
	// names, and nothing else.
	project := `
version = 1

[paths]
agents_file = "REVIEWER.md"

[roles.r2]
backends = ["prompted"]

[backends.prompted]
kind = "cli"
provider = "acme"
command = "definitely-not-installed-reviewer"
args = ["--prompt", "{{.Prompt}}"]
`
	root := gitRepo(t, project)
	write(t, filepath.Join(root, "REVIEWER.md"), "MARKER-FROM-THE-CONFIGURED-FILE\n")
	write(t, filepath.Join(root, "AGENTS.md"), "MARKER-FROM-THE-DEFAULT-FILE\n")
	branchWithChange(t, root, "feat/x")

	e, out, _ := reviewEnv(t, root, "review", "--role", "r2", "--dry-run")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "MARKER-FROM-THE-CONFIGURED-FILE") {
		t.Errorf("the prompt %q does not carry the configured instructions file", out.String())
	}
	if strings.Contains(out.String(), "MARKER-FROM-THE-DEFAULT-FILE") {
		t.Errorf("the prompt %q still carries AGENTS.md, which the configuration replaced", out.String())
	}
}

func TestCheckReportsEachGateAndExitsNonZeroOnAFailure(t *testing.T) {
	root := gitRepo(t, "version = 1\n")
	// No standards tree at all: the checks must fail loudly rather than pass by
	// finding nothing to read.
	e, _, errOut := reviewEnv(t, root, "check", "docs")
	if code := Run(e); code == 0 {
		t.Error("exit 0 with no standards to check against")
	}
	if !strings.Contains(errOut.String(), "INDEX.md") {
		t.Errorf("stderr %q does not say what could not be read", errOut.String())
	}
}

func TestCheckRejectsAnUnknownGateName(t *testing.T) {
	root := gitRepo(t, "version = 1\n")
	e, _, errOut := reviewEnv(t, root, "check", "vibes")
	if code := Run(e); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "vibes") {
		t.Errorf("stderr %q does not name the unknown gate", errOut.String())
	}
	// The message doubles as the only listing of what the gates are, so a gate
	// added without appearing here is one nobody can discover.
	for _, gate := range []string{"spec", "commit", "branch", "docs", "records", "agents", "design"} {
		if !strings.Contains(errOut.String(), gate) {
			t.Errorf("stderr %q does not offer the %q gate", errOut.String(), gate)
		}
	}
}

// --- the blocking mode -------------------------------------------------------
//
// `R2_BLOCKING=1` was documented by r2_gate.md and honoured only by the shell
// runner that is now deleted; `mf review` read nothing and returned 0 on every
// path, so installing the binary silently retired a gate the standard still
// promised. These tests pin the replacement, which is the same switch reached
// through the configuration cascade instead of a name of its own.

// reviewingEndpoint stands in for a reviewing provider. It answers in the
// OpenAI wire shape with the findings object the report parser looks for.
//
// An `api` backend is the shape used here because it is the one that always
// answers this way. A `cli` backend can too, once it declares `structured` —
// the prompt then asks for the schema and its severities survive — but a cli
// that declares nothing has its prose recorded verbatim, which carries no
// severity and can never block.
func reviewingEndpoint(t *testing.T, severity string) *httptest.Server {
	t.Helper()
	body := fmt.Sprintf(`{"findings":[{"category":"correctness","severity":%q,`+
		`"file":"a.txt","line":1,"summary":"the change drops a bounds check",`+
		`"rationale":"an out-of-range index is reachable"}]}`, severity)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// reviewingChain wires a repository whose R2 chain is one api backend pointed
// at the test endpoint: the project names the reviewer, the machine says how it
// is reached, which is the split docs/adr/0006 requires.
func reviewingChain(t *testing.T, severity string, environment map[string]string) (Env, *bytes.Buffer) {
	t.Helper()
	root := gitRepo(t, "version = 1\n\n[roles.r2]\nbackends = [\"probe\"]\n\n"+
		"[backends.probe]\nkind = \"api\"\nprovider = \"probe\"\nmodel = \"probe-1\"\n")
	branchWithChange(t, root, "feat/x")
	srv := reviewingEndpoint(t, severity)

	e, out, _ := reviewEnv(t, root, "review", "--role", "r2")
	machine := filepath.Join(t.TempDir(), "config.toml")
	write(t, machine, "version = 1\n\n[providers.probe]\nkind = \"openai-compatible\"\nendpoint = \""+srv.URL+"\"\n")
	e.MachinePath = machine
	e.Getenv = func(name string) string { return environment[name] }
	return e, out
}

func TestABlockingFindingIsAdvisoryUntilTheBlockingModeIsOn(t *testing.T) {
	// The shipped answer, unchanged: every layer is advisory per
	// ai_guidelines.md, and a finding is addressed or justified in the pull
	// request rather than by the push failing.
	e, out := reviewingChain(t, "blocking", nil)
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "bounds check") {
		t.Errorf("output %q does not carry the finding", out.String())
	}
}

func TestTheBlockingModeStopsTheCallerOnABlockingFinding(t *testing.T) {
	e, out := reviewingChain(t, "blocking", map[string]string{"MF_ROLES_R2_BLOCKING": "1"})
	if code := Run(e); code == 0 {
		t.Fatalf("exit 0 under the blocking mode with a blocking finding: %s", out.String())
	}
	// The exit code alone reaches a hook; the reader needs to be told which of
	// the several things that can stop a push actually did.
	if !strings.Contains(out.String(), "blocking finding") {
		t.Errorf("output %q does not say why the run failed", out.String())
	}
}

func TestTheBlockingModeIgnoresAnAdvisoryFinding(t *testing.T) {
	// Blocking mode is not "fail on any finding". Severity is the reviewer's
	// own judgement, and collapsing the two would make every advisory note a
	// stop sign.
	e, out := reviewingChain(t, "advisory", map[string]string{"MF_ROLES_R2_BLOCKING": "1"})
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, out.String())
	}
}

func TestTheBlockingModeStillExitsZeroWhenNoBackendWasAvailable(t *testing.T) {
	// r2_gate.md is explicit about this and it is the rule that keeps the gate
	// from being a lock: a reviewer that never ran is not a finding, and an
	// expired quota must not stop a repository from being pushed to.
	root := gitRepo(t, chainProject)
	branchWithChange(t, root, "feat/x")
	e, out, _ := reviewEnv(t, root, "review", "--role", "r2")
	e.Getenv = func(name string) string {
		if name == "MF_ROLES_R2_BLOCKING" {
			return "1"
		}
		return ""
	}
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "did not run") {
		t.Errorf("output %q does not record the absence", out.String())
	}
}

func TestTheBlockingModeIsPerRole(t *testing.T) {
	// R3 reviews with the least context and posts to a pull request; turning R2
	// blocking on a developer's machine must not make CI's advisory layer fail
	// a build.
	e, out := reviewingChain(t, "blocking", map[string]string{"MF_ROLES_R3_BLOCKING": "1"})
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, out.String())
	}
}
