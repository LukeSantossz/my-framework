// Package backend holds the ways a review can actually be performed.
//
// Four kinds, behind one interface. `cli` is declarative, so adding an agentic
// reviewer is a configuration change rather than a release. `api` is compiled
// and speaks three wire shapes. `inproc` runs deterministic checks with no
// model. `in-session` cannot be started at all and contributes an attestation.
//
// The distinction the chain turns on is availability versus verdict, and it
// cannot be recovered from outside the backend that owns the tool. That is why
// each kind classifies its own failures and reports Unavailable itself.
package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/LukeSantossz/my-framework/internal/report"
	"github.com/LukeSantossz/my-framework/internal/usage"
)

// withOverrides applies a backend's own model and effort. A chain can mix a
// hosted reviewer with a local fallback, and neither should inherit the other's
// model name.
func withOverrides(req Request, model, effort string) Request {
	if model != "" {
		req.Model = model
	}
	if effort != "" {
		req.Effort = effort
	}
	return req
}

// Request is what a backend is asked to review.
type Request struct {
	Role         string
	Base, Head   string
	Diff         string
	Truncated    bool
	Instructions string
	Model        string
	Effort       string
	HeadSHA      string

	// System overrides the review system prompt. A role that is not a review —
	// the CRUX explainer — must not be handed instructions to answer with
	// findings, and a backend that received them would produce an explainer
	// shaped like a verdict.
	System string
}

// Backend performs a review, or says it could not.
type Backend interface {
	Name() string
	Provider() string
	Describe(Request) string
	Review(context.Context, Request) (report.Result, error)
}

// Unavailable means the backend did not review: not installed, not
// authenticated, out of quota, unreachable, out of time — or started and failed
// without producing a review. The chain advances. It is deliberately distinct
// from a review that found problems, because that distinction is what decides
// whether a pull request may say this backend reviewed the change.
type Unavailable struct {
	Backend string
	Reason  string
}

func (e *Unavailable) Error() string {
	return fmt.Sprintf("backend %s unavailable: %s", e.Backend, e.Reason)
}

func IsUnavailable(err error) bool {
	var u *Unavailable
	return errors.As(err, &u)
}

// systemFor is the system prompt this request carries, defaulting to the review
// one. Every backend goes through it so a non-review role cannot reach a wire
// shape that hardcoded the review instructions.
func systemFor(req Request) string {
	if req.System != "" {
		return req.System
	}
	return systemPrompt
}

const systemPrompt = "You are the review backend for this repository. Report findings only; " +
	"do not rewrite code. Answer with a JSON object of the form " +
	`{"findings":[{"category":"correctness|invented-symbol|scope-creep|security|convention",` +
	`"severity":"blocking|advisory","file":"...","line":0,"summary":"...","rationale":"..."}]}. ` +
	"Use an empty findings array if you found nothing.\n\n"

// userPrompt puts the volatile diff last. Providers on these shapes bill cached
// prompt tokens at a fraction of fresh ones, and a pre-push gate re-sends the
// stable prefix on every push.
func userPrompt(req Request) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Review the change of %s against %s.", req.Head, req.Base)
	if req.Truncated {
		b.WriteString(" (TRUNCATED: only the first part of the diff is shown.)")
	}
	b.WriteString("\n\n")
	b.WriteString(req.Diff)
	return b.String()
}

// --- cli --------------------------------------------------------------------

// CLI invokes an agentic reviewer that explores the repository itself. Its
// command, arguments, provider identity and unavailability patterns all come
// from configuration, so a new reviewer needs no code.
type CLI struct {
	BackendName  string
	ProviderName string
	Command      string
	Args         []string
	Patterns     []string
	WorkDir      string

	// Budget is the total wall-clock time one review may take. An agentic
	// reviewer explores the repository and can sit silent for minutes, so this
	// is elapsed time rather than socket inactivity, exactly as the api kind
	// treats it. Zero means no deadline, which is only ever right in a test.
	Budget time.Duration

	// Structured says this command answers with the findings schema the role
	// prompt asks for.
	//
	// It is declared rather than detected. Guessing would make one backend
	// behave two ways depending on whether a given answer happened to parse,
	// and the severity of a finding decides whether a push is blocked. A
	// backend that declares it and then answers prose has the prose recorded,
	// exactly as the api kind does: a malformed answer is still an answer, and
	// reading it as a clean review is the false negative this framework treats
	// as the worst outcome.
	Structured bool

	// Model and Effort override the chain-wide values for this backend only.
	Model  string
	Effort string

	// Injected so tests never depend on a vendor CLI being installed.
	LookPath func(string) (string, error)
	Run      func(ctx context.Context, dir, name string, args []string) (string, error)
}

func (c *CLI) Name() string     { return c.BackendName }
func (c *CLI) Provider() string { return c.ProviderName }

// expand substitutes the named fields and nothing else. It is deliberately not
// a template language: the moment it grows conditionals it is a program living
// in a config file, harder to debug than the adapter script it replaced.
func expand(arg string, req Request) string {
	return strings.NewReplacer(
		"{{.Base}}", req.Base,
		"{{.Head}}", req.Head,
		"{{.Model}}", req.Model,
		"{{.Effort}}", req.Effort,
		// A reviewer that explores the repository finds AGENTS.md itself; one
		// that is handed a prompt needs the role and the diff sent to it.
		// Without this the declarative form could not express the second, and a
		// prompt-driven CLI would still need a hand-written adapter.
		//
		// The system prompt comes from systemFor rather than from the review
		// constant, so a role that is not a review — the CRUX explainer, whose
		// shipped chain is a prompt-driven cli backend — cannot be handed
		// instructions to answer with findings and then be judged for doing so.
		"{{.Prompt}}", systemFor(req)+req.Instructions+"\n\n"+userPrompt(req),
	).Replace(arg)
}

func (c *CLI) argv(req Request) []string {
	req = withOverrides(req, c.Model, c.Effort)
	out := make([]string, 0, len(c.Args))
	for _, a := range c.Args {
		out = append(out, expand(a, req))
	}
	return out
}

func (c *CLI) Describe(req Request) string {
	return c.Command + " " + strings.Join(c.argv(req), " ")
}

// Review runs the configured command and records what it printed.
//
// Every way of not reviewing is reported as unavailability rather than as an
// error that stops the chain, and the reason says which way it was. The choice
// is deliberate: a reviewer that crashed produced no review, exactly like one
// that was never installed, so the property worth preserving is that the chain
// still tries the next backend and the run still names what happened. Failing
// hard instead would let a mis-argumented backend at the head of a chain lock a
// repository out of R2 entirely, and this framework never lets a reviewer that
// did not run stand in the way of a push.
//
// What must never happen is the third option: recording a failed or silent
// process as a review. That marks the chain as having reviewed, stops it, and
// publishes "Reviewed by <this backend>" on the pull request for output no
// reviewer produced.
func (c *CLI) Review(ctx context.Context, req Request) (report.Result, error) {
	// Applied once, here, so the argv the command is given and the model the
	// result records are the same value. argv applied them and the result did
	// not, so a backend pinning its own model reviewed with it and reported
	// `<unset>` — a review whose record names a model it did not use.
	req = withOverrides(req, c.Model, c.Effort)

	lookPath := c.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if _, err := lookPath(c.Command); err != nil {
		return report.Result{}, &Unavailable{Backend: c.BackendName, Reason: c.Command + " is not installed"}
	}

	if c.Budget > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Budget)
		defer cancel()
	}

	run := c.Run
	if run == nil {
		run = runCommand
	}
	out, err := run(ctx, c.WorkDir, c.Command, c.argv(req))
	if err != nil {
		if c.Budget > 0 && ctx.Err() == context.DeadlineExceeded {
			return report.Result{}, &Unavailable{Backend: c.BackendName,
				Reason: fmt.Sprintf("no answer within the %s budget", c.Budget)}
		}
		// Matching a vendor's error text is confined to the backend that owns
		// that vendor, because the distinction between an unavailable tool and
		// a failed one cannot be recovered from outside it.
		if matchesAny(out, c.Patterns) {
			return report.Result{}, &Unavailable{Backend: c.BackendName, Reason: "quota, authentication, or network"}
		}
		return report.Result{}, &Unavailable{Backend: c.BackendName,
			Reason: fmt.Sprintf("%s failed (%v): %s", c.Command, err, snippet(out))}
	}
	if strings.TrimSpace(out) == "" {
		// A clean exit with nothing to show is what a killed or misconfigured
		// agentic reviewer looks like most often, and it is indistinguishable
		// from a review that found nothing — the false negative this framework
		// treats as the worst outcome.
		return report.Result{}, &Unavailable{Backend: c.BackendName,
			Reason: c.Command + " exited successfully but produced no output, so nothing was reviewed"}
	}
	// A CLI that has not declared the schema has its output recorded verbatim
	if c.Structured {
		if findings, parseErr := report.ParseFindings(out); parseErr == nil {
			return report.Result{
				Backend:   c.BackendName,
				Provider:  c.ProviderName,
				Model:     req.Model,
				Truncated: req.Truncated,
				Findings:  findings,
			}, nil
		}
	}
	// A partial review that loses the flag reads as a complete one, so the
	// prose path carries it exactly as the structured path and the api kind do.
	unstructured := report.Unstructured(c.BackendName, c.ProviderName, req.Model, out)
	unstructured.Truncated = req.Truncated
	return unstructured, nil
}

// snippet bounds what a failing command's output contributes to a reason, and
// flattens it to one line. The whole of it can be a stack trace, and this text
// is read in a terminal skip line and inside a markdown list item on the pull
// request, neither of which survives an embedded newline.
func snippet(out string) string {
	flat := strings.Join(strings.Fields(out), " ")
	if flat == "" {
		return "it printed nothing"
	}
	if len(flat) > 300 {
		return flat[:300] + "…"
	}
	return flat
}

func matchesAny(text string, patterns []string) bool {
	lower := strings.ToLower(text)
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if re, err := regexp.Compile("(?i)" + p); err == nil {
			if re.MatchString(text) {
				return true
			}
			continue
		}
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func runCommand(ctx context.Context, dir, name string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return runWith(ctx, cmd)
}

// runWith collects what a command printed, and gives up on it when the budget
// does.
//
// The WaitDelay is what makes the deadline mean anything. Stdout is an
// io.Writer rather than a file, so Go copies through an OS pipe, and Wait
// blocks until every writer closes it. CommandContext kills the process it
// started and nothing else, so an agentic reviewer that left a child behind
// keeps that pipe open and Wait never returns — a 240-second budget held a
// push for ten minutes that way, which is the failure the budget exists to
// prevent. With the delay set, Wait closes the pipe itself and returns what
// was collected up to that point.
func runWith(ctx context.Context, cmd *exec.Cmd) (string, error) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.WaitDelay = waitDelay
	err := cmd.Run()
	return buf.String(), err
}

// waitDelay is how long a killed command may go on holding its output pipe
// before the pipe is closed out from under it. Short, because by the time it
// applies the command has already been killed and the answer is already late.
const waitDelay = 2 * time.Second

// --- api --------------------------------------------------------------------

// WireShape is the request/response format an endpoint speaks.
type WireShape string

const (
	WireOpenAI    WireShape = "openai-compatible"
	WireAnthropic WireShape = "anthropic"
	WireGoogle    WireShape = "google"
)

// API sends the diff to an HTTP endpoint. Unlike an agentic backend it sees
// only what it is sent, so its review is a different shape rather than merely a
// weaker grade.
type API struct {
	BackendName  string
	ProviderName string
	Shape        WireShape
	Endpoint     string
	APIKey       string
	Budget       time.Duration
	Client       *http.Client

	// Model and Effort override the chain-wide values for this backend only.
	Model  string
	Effort string
}

func (a *API) Name() string     { return a.BackendName }
func (a *API) Provider() string { return a.ProviderName }

func (a *API) Describe(req Request) string {
	// The overrides are applied here for the same reason Review applies them:
	// a dry run that reports a different model from the one the real run would
	// send is worse than no dry run, and the value it reported without this was
	// the empty string Review itself reads as "no model configured".
	req = withOverrides(req, a.Model, a.Effort)
	return fmt.Sprintf("POST %s (%s) model=%q, diff of %s vs %s, budget %s",
		a.Endpoint, a.Shape, req.Model, req.Head, req.Base, a.Budget)
}

func (a *API) Review(ctx context.Context, req Request) (report.Result, error) {
	req = withOverrides(req, a.Model, a.Effort)
	if a.Endpoint == "" {
		return report.Result{}, &Unavailable{Backend: a.BackendName, Reason: "no endpoint configured"}
	}
	if req.Model == "" {
		return report.Result{}, &Unavailable{Backend: a.BackendName, Reason: "no model configured"}
	}
	// The budget is total elapsed time, not socket inactivity: a reasoning
	// model sends nothing while it thinks, so an inactivity timeout never fires
	// and the request runs until something else drops it.
	budget := a.Budget
	if budget <= 0 {
		budget = 240 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	url, body, headers, err := a.buildRequest(req)
	if err != nil {
		return report.Result{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return report.Result{}, &Unavailable{Backend: a.BackendName, Reason: err.Error()}
	}
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return report.Result{}, &Unavailable{Backend: a.BackendName,
				Reason: fmt.Sprintf("no answer within the %s budget", budget)}
		}
		return report.Result{}, &Unavailable{Backend: a.BackendName, Reason: "endpoint unreachable: " + err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	// An HTTP error is this backend being unavailable, not a review with
	// findings: a retired model id or an expired key must advance the chain.
	if resp.StatusCode != http.StatusOK {
		snippet := string(raw)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return report.Result{}, &Unavailable{Backend: a.BackendName,
			Reason: fmt.Sprintf("endpoint returned HTTP %d: %s", resp.StatusCode, snippet)}
	}

	content, incomplete, err := a.readAnswer(raw)
	if err != nil {
		return report.Result{}, &Unavailable{Backend: a.BackendName, Reason: err.Error()}
	}
	if strings.TrimSpace(content) == "" {
		// HTTP 200 with nothing in it: a content filter, a safety block, or a
		// truncation before the first token. Indistinguishable from a review
		// that found nothing, so it is reported as the weaker claim — the same
		// rule the cli kind keeps, for the same reason. Recording it stopped
		// the chain and put a reviewer's name on a change nothing had read.
		return report.Result{}, &Unavailable{Backend: a.BackendName,
			Reason: "the endpoint answered with no content, so nothing was reviewed"}
	}

	// Accounting never fails a review: if the figure cannot be determined the
	// review still stands and the accounting says it does not know.
	consumed := a.readUsage(raw)
	if !consumed.Known {
		consumed = usage.Estimate(len(systemFor(req))+len(req.Instructions)+len(req.Diff), len(content))
	}

	result := report.Result{
		Backend:    a.BackendName,
		Provider:   a.ProviderName,
		Model:      req.Model,
		Truncated:  req.Truncated,
		Incomplete: incomplete,
		Usage:      consumed,
	}
	findings, parseErr := report.ParseFindings(content)
	if parseErr != nil {
		// A malformed answer is still an answer: the backend reviewed. Recording
		// the prose keeps it from reading as a clean review.
		unstructured := report.Unstructured(a.BackendName, a.ProviderName, req.Model, content)
		unstructured.Truncated = req.Truncated
		unstructured.Incomplete = incomplete
		unstructured.Usage = consumed
		return unstructured, nil
	}
	result.Findings = findings
	return result, nil
}

func (a *API) buildRequest(req Request) (string, []byte, map[string]string, error) {
	base := strings.TrimRight(a.Endpoint, "/")
	headers := map[string]string{"Content-Type": "application/json"}
	system := systemFor(req) + req.Instructions

	switch a.Shape {
	case WireAnthropic:
		if a.APIKey != "" {
			headers["x-api-key"] = a.APIKey
			headers["anthropic-version"] = "2023-06-01"
		}
		body, err := json.Marshal(map[string]any{
			"model":      req.Model,
			"max_tokens": 4096,
			"system":     system,
			"messages":   []any{map[string]string{"role": "user", "content": userPrompt(req)}},
		})
		return base + "/v1/messages", body, headers, err

	case WireGoogle:
		url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", base, req.Model)
		if a.APIKey != "" {
			headers["x-goog-api-key"] = a.APIKey
		}
		body, err := json.Marshal(map[string]any{
			"systemInstruction": map[string]any{"parts": []any{map[string]string{"text": system}}},
			"contents":          []any{map[string]any{"parts": []any{map[string]string{"text": userPrompt(req)}}}},
			"generationConfig":  map[string]any{"temperature": 0},
		})
		return url, body, headers, err

	default: // WireOpenAI
		if a.APIKey != "" {
			headers["Authorization"] = "Bearer " + a.APIKey
		}
		body, err := json.Marshal(map[string]any{
			"model": req.Model,
			"messages": []any{
				map[string]string{"role": "system", "content": system},
				map[string]string{"role": "user", "content": userPrompt(req)},
			},
			// temperature 0 so a review is comparable between runs; without it
			// the same diff yields different findings and nothing can be
			// measured against it.
			"temperature": 0,
		})
		return base + "/chat/completions", body, headers, err
	}
}

// readAnswer extracts the answer text. reasoning_content and its equivalents
// carry a model's private chain of thought; it is not the review and must never
// be reported as findings.
func (a *API) readAnswer(raw []byte) (string, bool, error) {
	switch a.Shape {
	case WireAnthropic:
		var parsed struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			StopReason string `json:"stop_reason"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return "", false, fmt.Errorf("response was not JSON")
		}
		var b strings.Builder
		for _, c := range parsed.Content {
			b.WriteString(c.Text)
		}
		return b.String(), parsed.StopReason == "max_tokens", nil

	case WireGoogle:
		var parsed struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return "", false, fmt.Errorf("response was not JSON")
		}
		if len(parsed.Candidates) == 0 {
			return "", false, fmt.Errorf("response carried no candidate")
		}
		var b strings.Builder
		for _, p := range parsed.Candidates[0].Content.Parts {
			b.WriteString(p.Text)
		}
		return b.String(), parsed.Candidates[0].FinishReason == "MAX_TOKENS", nil

	default:
		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return "", false, fmt.Errorf("response was not JSON")
		}
		if len(parsed.Choices) == 0 {
			return "", false, fmt.Errorf("response carried no choice")
		}
		return parsed.Choices[0].Message.Content, parsed.Choices[0].FinishReason == "length", nil
	}
}

// --- in-session -------------------------------------------------------------

// InSession is a reviewer that runs inside a coding-agent session. It cannot be
// started as a subprocess — the session is already running and the skill
// executes within it — so its participation is an attestation rather than an
// execution, and an absent session is unavailability so the chain advances.
type InSession struct {
	BackendName    string
	ProviderName   string
	HasAttestation func(role, headSHA string) bool

	// HowToAttest renders the command that records an attestation for a role.
	// The store belongs to the caller, not to this backend, but an absence
	// nobody is told how to fill is a dead end rather than a fallback — and
	// this backend is the whole of R1's shipped chain.
	HowToAttest func(role string) string
}

func (s *InSession) Name() string     { return s.BackendName }
func (s *InSession) Provider() string { return s.ProviderName }

func (s *InSession) Describe(req Request) string {
	return fmt.Sprintf("%s (in-session): satisfied by an attestation for %s, never by a subprocess", s.BackendName, req.Role)
}

func (s *InSession) Review(_ context.Context, req Request) (report.Result, error) {
	if s.HasAttestation == nil || !s.HasAttestation(req.Role, req.HeadSHA) {
		reason := "no in-session attestation for this change; it cannot be started as a subprocess"
		if s.HowToAttest != nil {
			reason += ". " + s.HowToAttest(req.Role)
		}
		return report.Result{}, &Unavailable{Backend: s.BackendName, Reason: reason}
	}
	return report.Result{
		Backend:  s.BackendName,
		Provider: s.ProviderName,
		Model:    req.Model,
	}, nil
}

// --- inproc -----------------------------------------------------------------

// Check is a deterministic review with no model behind it.
type Check func(Request) ([]report.Finding, error)

// InProc runs registered deterministic checks. What it runs is supplied by the
// checks slice; with none registered it is honestly unavailable rather than
// silently clean.
type InProc struct {
	BackendName string
	Checks      []Check
}

func (p *InProc) Name() string     { return p.BackendName }
func (p *InProc) Provider() string { return "none" }

func (p *InProc) Describe(Request) string {
	return fmt.Sprintf("%s (in-process): %d deterministic checks", p.BackendName, len(p.Checks))
}

func (p *InProc) Review(_ context.Context, req Request) (report.Result, error) {
	if len(p.Checks) == 0 {
		return report.Result{}, &Unavailable{Backend: p.BackendName, Reason: "no in-process checks registered"}
	}
	result := report.Result{Backend: p.BackendName, Provider: "none"}
	for _, check := range p.Checks {
		findings, err := check(req)
		if err != nil {
			return report.Result{}, &Unavailable{Backend: p.BackendName, Reason: err.Error()}
		}
		result.Findings = append(result.Findings, findings...)
	}
	return result, nil
}

// --- external ---------------------------------------------------------------

// External is a reviewer that runs outside this tool — a forge app wired to the
// repository, for instance. It is declared so the review-layers record can name
// it, and it is always unavailable to this chain, because the framework observed
// no review: it only knows that configuration says one is wired.
//
// That is a weaker claim than every other backend makes, and it reads as such.
type External struct {
	BackendName  string
	ProviderName string
}

func (e *External) Name() string     { return e.BackendName }
func (e *External) Provider() string { return e.ProviderName }

func (e *External) Describe(Request) string {
	return fmt.Sprintf("%s (external): declared as wired, executed elsewhere, recorded but never run here", e.BackendName)
}

func (e *External) Review(context.Context, Request) (report.Result, error) {
	return report.Result{}, &Unavailable{Backend: e.BackendName,
		Reason: "declared as external: it runs outside this tool, so no review was observed here"}
}

// readUsage parses the usage object for this backend's wire shape. Layouts and
// terminology differ between vendors, which is why this is per-shape rather
// than one parser trying to satisfy all of them.
func (a *API) readUsage(raw []byte) usage.Usage {
	switch a.Shape {
	case WireAnthropic:
		return usage.ParseAnthropic(raw)
	case WireGoogle:
		return usage.ParseGoogle(raw)
	default:
		return usage.ParseOpenAI(raw)
	}
}
