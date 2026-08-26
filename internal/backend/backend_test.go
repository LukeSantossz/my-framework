package backend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/LukeSantossz/my-framework/internal/report"
)

func req() Request {
	return Request{
		Role: "r2", Base: "main", Head: "feat/x", Diff: "diff --git a/a b/a\n",
		Model: "m", Effort: "high", HeadSHA: "abc123",
	}
}

// --- cli --------------------------------------------------------------------

func TestRunsACLIBackendDeclaredOnlyInConfiguration(t *testing.T) {
	// No compiled adapter exists for "reviewbot"; everything about it comes
	// from configuration, which is what makes a new reviewer a config change.
	var gotName string
	var gotArgs []string
	b := &CLI{
		BackendName: "reviewbot", ProviderName: "acme",
		Command:  "reviewbot",
		Args:     []string{"review", "--base", "{{.Base}}", "-c", "model={{.Model}}", "-c", "effort={{.Effort}}"},
		LookPath: func(string) (string, error) { return "/usr/bin/reviewbot", nil },
		Run: func(_ context.Context, _, name string, args []string) (string, error) {
			gotName, gotArgs = name, args
			return "the base case is wrong", nil
		},
	}
	res, err := b.Review(context.Background(), req())
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if gotName != "reviewbot" {
		t.Errorf("ran %q, want reviewbot", gotName)
	}
	want := []string{"review", "--base", "main", "-c", "model=m", "-c", "effort=high"}
	if strings.Join(gotArgs, " ") != strings.Join(want, " ") {
		t.Errorf("argv = %v, want %v", gotArgs, want)
	}
	if res.Backend != "reviewbot" || res.Provider != "acme" {
		t.Errorf("result does not name the backend: %+v", res)
	}
}

func TestReportsACLIBackendUnavailableWhenItIsNotInstalled(t *testing.T) {
	b := &CLI{
		BackendName: "codex", Command: "codex",
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
	}
	_, err := b.Review(context.Background(), req())
	if !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable", err)
	}
}

func TestClassifiesACLIBackendUnavailableUsingItsConfiguredPatterns(t *testing.T) {
	b := &CLI{
		BackendName: "codex", ProviderName: "openai", Command: "codex",
		Patterns: []string{"usage limit", "quota", "401"},
		LookPath: func(string) (string, error) { return "/usr/bin/codex", nil },
		Run: func(context.Context, string, string, []string) (string, error) {
			return "Error: you have hit your usage limit", errors.New("exit 1")
		},
	}
	_, err := b.Review(context.Background(), req())
	if !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable", err)
	}
}

func TestACLIFailureThatMatchesNoPatternIsAFailureNotAReview(t *testing.T) {
	// A crashed, mis-argumented or killed reviewer did not review. Recording its
	// output as one would stop the chain, name it as the backend that ran, and
	// publish "Reviewed by <it>" on the pull request for a process that produced
	// no review at all.
	b := &CLI{
		BackendName: "codex", ProviderName: "openai", Command: "codex",
		Patterns: []string{"usage limit"},
		LookPath: func(string) (string, error) { return "/usr/bin/codex", nil },
		Run: func(context.Context, string, string, []string) (string, error) {
			return "error: unknown flag --base", errors.New("exit status 2")
		},
	}
	res, err := b.Review(context.Background(), req())
	if !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable so the chain tries the next backend", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("a failed process was recorded as findings: %+v", res.Findings)
	}
	if !strings.Contains(err.Error(), "exit status 2") || !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("reason %q carries neither what failed nor what it printed", err)
	}
}

func TestACLIThatExitsCleanlyWithNoOutputIsNotACompletedReview(t *testing.T) {
	// Zero exit and an empty answer is the shape a killed or misconfigured
	// agentic reviewer takes most often, and it is indistinguishable from a
	// review that found nothing — which is the false negative this framework
	// treats as worst.
	b := &CLI{
		BackendName: "codex", ProviderName: "openai", Command: "codex",
		LookPath: func(string) (string, error) { return "/usr/bin/codex", nil },
		Run: func(context.Context, string, string, []string) (string, error) {
			return "   \n", nil
		},
	}
	_, err := b.Review(context.Background(), req())
	if !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable", err)
	}
	if !strings.Contains(err.Error(), "no output") {
		t.Errorf("reason %q does not say the backend answered with nothing", err)
	}
}

func TestAppliesTheWallClockBudgetToACLIBackendToo(t *testing.T) {
	// Without it a hung agentic reviewer holds `mf review`, and the pre-push
	// hook behind it, open forever — while `mf doctor` reports a timeout that
	// was never in force.
	b := &CLI{
		BackendName: "codex", Command: "codex", Budget: 20 * time.Millisecond,
		LookPath: func(string) (string, error) { return "/usr/bin/codex", nil },
		Run: func(ctx context.Context, _, _ string, _ []string) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	_, err := b.Review(context.Background(), req())
	if !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable", err)
	}
	if !strings.Contains(err.Error(), "budget") {
		t.Errorf("error %q does not name the budget", err)
	}
}

func TestSendsTheRolesOwnSystemPromptToAPromptDrivenCLI(t *testing.T) {
	// `roles.explain.backends = ["gemini"]` is shipped configuration. With the
	// review instructions hardcoded into the expansion, `mf explain` paid a
	// backend to answer with findings and then rejected the answer for not
	// being an explainer.
	var gotArgs []string
	b := &CLI{
		BackendName: "gemini", ProviderName: "google", Command: "gemini",
		Args:     []string{"--prompt", "{{.Prompt}}"},
		LookPath: func(string) (string, error) { return "/usr/bin/gemini", nil },
		Run: func(_ context.Context, _, _ string, args []string) (string, error) {
			gotArgs = args
			return "ok", nil
		},
	}
	r := req()
	r.Role = "explain"
	r.System = "Explain this change to a newcomer. Never answer with findings.\n\n"
	if _, err := b.Review(context.Background(), r); err != nil {
		t.Fatalf("Review: %v", err)
	}
	joined := strings.Join(gotArgs, " ")
	if !strings.Contains(joined, "Never answer with findings") {
		t.Errorf("argv %q does not carry the role's own system prompt", joined)
	}
	if strings.Contains(joined, "Report findings only") {
		t.Errorf("argv %q still carries the review instructions to a role that is not a review", joined)
	}
}

func TestRecordsUnparseableCLIOutputAsOneTextualFindingRatherThanAsNone(t *testing.T) {
	b := &CLI{
		BackendName: "codex", ProviderName: "openai", Command: "codex",
		LookPath: func(string) (string, error) { return "/usr/bin/codex", nil },
		Run: func(context.Context, string, string, []string) (string, error) {
			return "Looks fine to me, mostly.", nil
		},
	}
	res, _ := b.Review(context.Background(), req())
	if !res.Unstructured {
		t.Error("prose from a cli backend must be marked unstructured")
	}
	if len(res.Findings) != 1 || res.Findings[0].Category != report.CategoryUnstructured {
		t.Errorf("got %+v, want exactly one unstructured finding", res.Findings)
	}
}

// --- api --------------------------------------------------------------------

func openAIServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestParsesStructuredFindingsFromAnAPIBackend(t *testing.T) {
	srv := openAIServer(t, 200, `{"choices":[{"message":{"content":"{\"findings\":[{\"category\":\"security\",\"severity\":\"blocking\",\"summary\":\"key in log\"}]}"},"finish_reason":"stop"}]}`)
	b := &API{BackendName: "deepseek", ProviderName: "deepseek", Shape: WireOpenAI, Endpoint: srv.URL}
	res, err := b.Review(context.Background(), req())
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Category != report.CategorySecurity {
		t.Fatalf("got %+v", res.Findings)
	}
	if !res.HasBlocking() {
		t.Error("a blocking finding must report as blocking")
	}
}

func TestTreatsAnHTTPErrorFromAnAPIBackendAsUnavailableRatherThanAsFindings(t *testing.T) {
	// A retired model id or an expired key must advance the chain, not be
	// reported as a review that found something.
	srv := openAIServer(t, 401, `{"error":"invalid api key"}`)
	b := &API{BackendName: "deepseek", Shape: WireOpenAI, Endpoint: srv.URL}
	_, err := b.Review(context.Background(), req())
	if !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable", err)
	}
}

func TestNeverReportsReasoningContentAsFindings(t *testing.T) {
	// reasoning_content is a model's private chain of thought. It is not the
	// review, and filing it as findings would put thinking in a PR record.
	body := `{"choices":[{"message":{"content":"{\"findings\":[]}","reasoning_content":"Let me think... maybe the key is leaked."},"finish_reason":"stop"}]}`
	srv := openAIServer(t, 200, body)
	b := &API{BackendName: "deepseek", Shape: WireOpenAI, Endpoint: srv.URL}
	res, err := b.Review(context.Background(), req())
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("reasoning leaked into findings: %+v", res.Findings)
	}
}

func TestReportsAnAnswerCutOffByTheOutputLimitAsIncomplete(t *testing.T) {
	srv := openAIServer(t, 200, `{"choices":[{"message":{"content":"{\"findings\":[]}"},"finish_reason":"length"}]}`)
	b := &API{BackendName: "deepseek", Shape: WireOpenAI, Endpoint: srv.URL}
	res, _ := b.Review(context.Background(), req())
	if !res.Incomplete {
		t.Error("a length-terminated answer must report as incomplete")
	}
}

func TestCarriesDiffTruncationIntoTheResult(t *testing.T) {
	srv := openAIServer(t, 200, `{"choices":[{"message":{"content":"{\"findings\":[]}"},"finish_reason":"stop"}]}`)
	b := &API{BackendName: "deepseek", Shape: WireOpenAI, Endpoint: srv.URL}
	r := req()
	r.Truncated = true
	res, _ := b.Review(context.Background(), r)
	if !res.Truncated {
		t.Error("a truncated diff must be reported as a partial review")
	}
}

func TestTreatsExceedingTheWallClockBudgetAsUnavailability(t *testing.T) {
	// The budget is total elapsed time, not socket inactivity: a reasoning
	// model sends nothing while it thinks, so an inactivity timeout never fires.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`)
	}))
	defer srv.Close()
	b := &API{BackendName: "slow", Shape: WireOpenAI, Endpoint: srv.URL, Budget: 30 * time.Millisecond}
	_, err := b.Review(context.Background(), req())
	if !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable", err)
	}
	if !strings.Contains(err.Error(), "budget") {
		t.Errorf("error %q does not name the budget", err)
	}
}

func TestAnAPIBackendWithNoEndpointOrModelIsUnavailable(t *testing.T) {
	if _, err := (&API{BackendName: "x", Shape: WireOpenAI}).Review(context.Background(), req()); !IsUnavailable(err) {
		t.Errorf("missing endpoint: err = %v, want Unavailable", err)
	}
	r := req()
	r.Model = ""
	if _, err := (&API{BackendName: "x", Shape: WireOpenAI, Endpoint: "http://x"}).Review(context.Background(), r); !IsUnavailable(err) {
		t.Errorf("missing model: err = %v, want Unavailable", err)
	}
}

func TestSendsTemperatureZeroOnTheOpenAIShape(t *testing.T) {
	// Without it the same diff yields different findings between runs and
	// nothing can be measured against the result.
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		body = string(buf)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`)
	}))
	defer srv.Close()
	b := &API{BackendName: "d", Shape: WireOpenAI, Endpoint: srv.URL}
	if _, err := b.Review(context.Background(), req()); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if !strings.Contains(body, `"temperature":0`) {
		t.Errorf("request body %q does not pin temperature to 0", body)
	}
}

func TestPutsTheVolatileDiffAfterTheStablePrefix(t *testing.T) {
	// Providers bill cached prompt tokens at a fraction of fresh ones, and a
	// pre-push gate re-sends the prefix on every push.
	r := req()
	r.Instructions = "BINDING STANDARDS"
	prompt := userPrompt(r)
	if !strings.HasSuffix(strings.TrimSpace(prompt), strings.TrimSpace(r.Diff)) {
		t.Errorf("the diff is not last in the user prompt:\n%s", prompt)
	}
}

func TestSupportsTheAnthropicAndGoogleWireShapes(t *testing.T) {
	anthropic := openAIServer(t, 200, `{"content":[{"text":"{\"findings\":[{\"category\":\"convention\",\"summary\":\"naming\"}]}"}],"stop_reason":"end_turn"}`)
	res, err := (&API{BackendName: "claude", Shape: WireAnthropic, Endpoint: anthropic.URL}).Review(context.Background(), req())
	if err != nil {
		t.Fatalf("anthropic: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Errorf("anthropic: got %+v", res.Findings)
	}

	google := openAIServer(t, 200, `{"candidates":[{"content":{"parts":[{"text":"{\"findings\":[{\"category\":\"correctness\",\"summary\":\"off by one\"}]}"}]},"finishReason":"STOP"}]}`)
	res, err = (&API{BackendName: "gemini", Shape: WireGoogle, Endpoint: google.URL}).Review(context.Background(), req())
	if err != nil {
		t.Fatalf("google: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Errorf("google: got %+v", res.Findings)
	}
}

// --- in-session -------------------------------------------------------------

func TestReportsAnInSessionBackendUnavailableWhenNoAttestationExists(t *testing.T) {
	b := &InSession{BackendName: "superpowers", ProviderName: "anthropic"}
	_, err := b.Review(context.Background(), req())
	if !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable", err)
	}
	if !strings.Contains(err.Error(), "subprocess") {
		t.Errorf("error %q does not explain why it cannot simply be run", err)
	}
}

func TestAnInSessionBackendIsSatisfiedByAnAttestationForThisChange(t *testing.T) {
	b := &InSession{
		BackendName: "superpowers", ProviderName: "anthropic",
		HasAttestation: func(role, sha string) bool { return role == "r2" && sha == "abc123" },
	}
	res, err := b.Review(context.Background(), req())
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if res.Backend != "superpowers" {
		t.Errorf("result does not name the backend: %+v", res)
	}
}

func TestAnInSessionBackendSaysHowAnAttestationIsRecorded(t *testing.T) {
	// An absence nobody can act on is a dead end rather than a fallback: this
	// backend is the whole of R1's shipped chain, so a reader who is told only
	// that it did not run has no way to make it run.
	b := &InSession{
		BackendName:  "superpowers",
		HowToAttest:  func(role string) string { return "record " + role + " with git config" },
		ProviderName: "anthropic",
	}
	_, err := b.Review(context.Background(), req())
	if !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable", err)
	}
	if !strings.Contains(err.Error(), "record r2 with git config") {
		t.Errorf("reason %q leaves the user with no way to fill the absence", err)
	}
}

func TestAnAttestationForAnotherChangeDoesNotSatisfyThisOne(t *testing.T) {
	b := &InSession{
		BackendName:    "superpowers",
		HasAttestation: func(_, sha string) bool { return sha == "an-older-sha" },
	}
	if _, err := b.Review(context.Background(), req()); !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable", err)
	}
}

// --- inproc -----------------------------------------------------------------

func TestAnInProcBackendWithNoChecksIsUnavailableRatherThanSilentlyClean(t *testing.T) {
	if _, err := (&InProc{BackendName: "checks"}).Review(context.Background(), req()); !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable", err)
	}
}

func TestAnInProcBackendReportsWhatItsChecksFound(t *testing.T) {
	b := &InProc{BackendName: "checks", Checks: []Check{
		func(Request) ([]report.Finding, error) {
			return []report.Finding{{Category: report.CategoryConvention, Severity: report.SeverityAdvisory, Summary: "naming"}}, nil
		},
	}}
	res, err := b.Review(context.Background(), req())
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Errorf("got %+v", res.Findings)
	}
}

func TestExpandsThePromptForACLIThatCannotReadTheRepositoryItself(t *testing.T) {
	// A codex-style reviewer finds AGENTS.md on disk; a gemini-style one is
	// handed a prompt. Without {{.Prompt}} the declarative form cannot express
	// the second, and the shipped gemini adapter could not be replaced.
	var gotArgs []string
	b := &CLI{
		BackendName: "gemini", ProviderName: "google", Command: "gemini",
		Args:     []string{"-m", "{{.Model}}", "--prompt", "{{.Prompt}}"},
		LookPath: func(string) (string, error) { return "/usr/bin/gemini", nil },
		Run: func(_ context.Context, _, _ string, args []string) (string, error) {
			gotArgs = args
			return "ok", nil
		},
	}
	r := req()
	r.Instructions = "BINDING STANDARDS"
	if _, err := b.Review(context.Background(), r); err != nil {
		t.Fatalf("Review: %v", err)
	}
	joined := strings.Join(gotArgs, " ")
	if !strings.Contains(joined, "BINDING STANDARDS") {
		t.Errorf("argv %q does not carry the role instructions", joined)
	}
	if !strings.Contains(joined, r.Diff) {
		t.Errorf("argv %q does not carry the diff", joined)
	}
}

func TestAPerBackendModelOverridesTheChainWideOne(t *testing.T) {
	// A chain can mix a hosted reviewer with a local fallback, and neither
	// should inherit the other's model name. Losing this in the port would send
	// an empty model to a vendor that then guesses or refuses.
	var gotArgs []string
	b := &CLI{
		BackendName: "codex", ProviderName: "openai", Command: "codex",
		Model: "gpt-5.6-terra", Effort: "low",
		Args:     []string{"-c", "model={{.Model}}", "-c", "effort={{.Effort}}"},
		LookPath: func(string) (string, error) { return "/usr/bin/codex", nil },
		Run: func(_ context.Context, _, _ string, args []string) (string, error) {
			gotArgs = args
			return "ok", nil
		},
	}
	r := req()
	r.Model, r.Effort = "chain-wide-model", "high"
	if _, err := b.Review(context.Background(), r); err != nil {
		t.Fatalf("Review: %v", err)
	}
	joined := strings.Join(gotArgs, " ")
	if !strings.Contains(joined, "model=gpt-5.6-terra") {
		t.Errorf("argv %q did not take the per-backend model", joined)
	}
	if !strings.Contains(joined, "effort=low") {
		t.Errorf("argv %q did not take the per-backend effort", joined)
	}
}

func TestAnAPIBackendTakesItsOwnModelWhenSet(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		body = string(buf)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`)
	}))
	defer srv.Close()
	b := &API{BackendName: "d", Shape: WireOpenAI, Endpoint: srv.URL, Model: "deepseek-v4-flash"}
	r := req()
	r.Model = "chain-wide-model"
	if _, err := b.Review(context.Background(), r); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if !strings.Contains(body, "deepseek-v4-flash") {
		t.Errorf("request %q did not take the per-backend model", body)
	}
}

func TestAnExternalBackendIsRecordedButNeverRunHere(t *testing.T) {
	// The framework knows configuration says a reviewer is wired; it did not
	// observe a review. Reporting that as a review would be the one claim in the
	// chain with nothing behind it.
	b := &External{BackendName: "coderabbit", ProviderName: "coderabbit"}
	_, err := b.Review(context.Background(), req())
	if !IsUnavailable(err) {
		t.Fatalf("err = %v, want Unavailable", err)
	}
	if !strings.Contains(err.Error(), "outside this tool") {
		t.Errorf("error %q does not explain why nothing ran", err)
	}
	if !strings.Contains(b.Describe(req()), "recorded but never run here") {
		t.Errorf("describe does not state the weaker claim: %q", b.Describe(req()))
	}
}

func TestReportsMeasuredUsageWhenTheEndpointReturnsIt(t *testing.T) {
	body := `{"choices":[{"message":{"content":"{\"findings\":[]}"},"finish_reason":"stop"}],
	 "usage":{"prompt_tokens":1000,"completion_tokens":200,"prompt_tokens_details":{"cached_tokens":700}}}`
	srv := openAIServer(t, 200, body)
	b := &API{BackendName: "d", Shape: WireOpenAI, Endpoint: srv.URL}
	res, err := b.Review(context.Background(), req())
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if !res.Usage.Known || res.Usage.Estimated {
		t.Fatalf("usage = %+v, want measured", res.Usage)
	}
	if res.Usage.CacheReadTokens != 700 || res.Usage.InputTokens != 300 {
		t.Errorf("buckets not disjoint: %+v", res.Usage)
	}
}

func TestFallsBackToAnEstimateWhenTheEndpointReportsNoUsage(t *testing.T) {
	// The estimate is marked as one everywhere it appears. Reporting zero as a
	// measured value would be a fabricated number.
	srv := openAIServer(t, 200, `{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`)
	b := &API{BackendName: "d", Shape: WireOpenAI, Endpoint: srv.URL}
	res, _ := b.Review(context.Background(), req())
	if !res.Usage.Known {
		t.Fatal("no usage at all; an estimate was expected")
	}
	if !res.Usage.Estimated {
		t.Error("the fallback figure is not marked as an estimate")
	}
}

func TestUsageSurvivesAnUnparseableAnswer(t *testing.T) {
	body := `{"choices":[{"message":{"content":"I could not produce JSON"}}],
	 "usage":{"prompt_tokens":10,"completion_tokens":5}}`
	srv := openAIServer(t, 200, body)
	b := &API{BackendName: "d", Shape: WireOpenAI, Endpoint: srv.URL}
	res, _ := b.Review(context.Background(), req())
	if !res.Unstructured {
		t.Fatal("expected the prose path")
	}
	if !res.Usage.Known {
		t.Error("usage was lost on the prose path; the call still cost what it cost")
	}
}

func TestDescribeNamesTheModelTheReviewWouldActuallyUse(t *testing.T) {
	// A dry run exists to show what a real run would do. Review applies the
	// per-backend override before it sends anything, so a Describe that skips
	// it reports model="" — the exact value Review treats as "no model
	// configured", which is the one answer a reader would act on.
	a := &API{
		BackendName: "acme",
		Endpoint:    "https://example.invalid/v1",
		Shape:       "openai",
		Model:       "acme-2",
	}
	got := a.Describe(Request{Base: "main", Head: "HEAD"})
	if !strings.Contains(got, `model="acme-2"`) {
		t.Errorf("Describe does not name the model the review would use: %s", got)
	}
}

// TestAKilledCommandCannotHoldItsOutputPipeOpenForever guards the WaitDelay
// that makes the review budget mean anything.
//
// The failure it guards against was observed, not theorised: a push under a
// 240-second budget was still running after ten minutes, and a review under a
// deliberately short 15-second budget ran until an external timeout killed it
// at 121 seconds. The cause is that Stdout is an io.Writer, so Go copies
// through an OS pipe and Wait blocks until every writer closes it, while
// CommandContext kills only the process it started. On Windows `codex` is an
// npm shim: a batch file that spawns node, so the kill reaches the shim and
// node goes on holding the pipe. With the delay set the same review returns in
// 23 seconds and the chain advances.
//
// This asserts the delay is configured rather than reproducing the orphaned
// grandchild, which needs a process tree this suite cannot build portably. It
// is a regression guard for a one-line removal, and it is honest about being
// only that: the behaviour itself was verified by hand, as recorded above.
func TestAKilledCommandCannotHoldItsOutputPipeOpenForever(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if _, err := runWith(context.Background(), cmd); err != nil {
		t.Fatalf("running a trivial command: %v", err)
	}
	if cmd.WaitDelay <= 0 {
		t.Error("no WaitDelay: a killed command can hold its output pipe open past the budget")
	}
}

func TestACLIThatAnswersWithTheSchemaIsReadAsFindings(t *testing.T) {
	// The kind's comment said an agentic CLI cannot be asked for a schema. Some
	// can, and one that answers with the findings shape had it recorded as
	// prose: its severities were discarded, so it could never block, and
	// `mf eval` scored it zero because every finding carried the category
	// `unstructured` rather than the one it reported.
	answer := "```json\n" + `{"findings":[{"category":"correctness","severity":"blocking",` +
		`"file":"x.go","line":2,"summary":"out of range","rationale":"len(s) is not an index"}]}` + "\n```"
	c := &CLI{
		BackendName: "agy", ProviderName: "google", Structured: true,
		LookPath: func(string) (string, error) { return "agy", nil },
		Run:      func(context.Context, string, string, []string) (string, error) { return answer, nil },
	}
	res, err := c.Review(context.Background(), req())
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if res.Unstructured {
		t.Fatal("the answer was recorded as prose")
	}
	if len(res.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(res.Findings))
	}
	if got := string(res.Findings[0].Category); got != "correctness" {
		t.Errorf("category %q, want correctness", got)
	}
	if !res.HasBlocking() {
		t.Error("a blocking severity did not survive, so this review could never block a push")
	}
}

func TestACLIThatPromisesTheSchemaAndAnswersProseIsStillRecorded(t *testing.T) {
	// A backend that stops answering in the shape it declared must not be read
	// as a clean review. The prose is kept, exactly as the api kind keeps it.
	c := &CLI{
		BackendName: "agy", ProviderName: "google", Structured: true,
		LookPath: func(string) (string, error) { return "agy", nil },
		Run: func(context.Context, string, string, []string) (string, error) {
			return "I had a look and it seems fine.", nil
		},
	}
	res, err := c.Review(context.Background(), req())
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if !res.Unstructured {
		t.Error("prose from a backend that promised the schema was not marked unstructured")
	}
	if len(res.Findings) != 1 || !strings.Contains(res.Findings[0].Summary, "seems fine") {
		t.Errorf("the prose was lost: %+v", res.Findings)
	}
}

func TestTheResultNamesTheModelTheCommandWasGiven(t *testing.T) {
	// argv applied the backend's own model and the result did not, so a backend
	// pinning one reviewed with it and recorded `<unset>`: a review whose record
	// names a model it did not use.
	c := &CLI{
		BackendName: "agy", ProviderName: "google", Model: "gemini-3.1-pro-high",
		LookPath: func(string) (string, error) { return "agy", nil },
		Run:      func(context.Context, string, string, []string) (string, error) { return "looks fine", nil },
	}
	res, err := c.Review(context.Background(), Request{Base: "main", Head: "HEAD", Truncated: true})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if res.Model != "gemini-3.1-pro-high" {
		t.Errorf("Model = %q, want the one the command was given", res.Model)
	}
	if !res.Truncated {
		t.Error("a partial review lost its Truncated flag and reads as complete")
	}
}
