package backend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestACLIFailureThatMatchesNoPatternIsAReviewNotAnAbsence(t *testing.T) {
	// A pattern that stops matching must stop the chain and name this backend
	// rather than silently pretending a review happened somewhere else.
	b := &CLI{
		BackendName: "codex", ProviderName: "openai", Command: "codex",
		Patterns: []string{"usage limit"},
		LookPath: func(string) (string, error) { return "/usr/bin/codex", nil },
		Run: func(context.Context, string, string, []string) (string, error) {
			return "found a real problem in handler.go", errors.New("exit 1")
		},
	}
	res, err := b.Review(context.Background(), req())
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(res.Findings))
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
