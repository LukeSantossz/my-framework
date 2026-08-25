package role

import (
	"context"
	"strings"
	"testing"

	"github.com/LukeSantossz/my-framework/internal/backend"
	"github.com/LukeSantossz/my-framework/internal/report"
	"github.com/LukeSantossz/my-framework/internal/vcs"
)

// fake is a backend whose behaviour the test states outright.
type fake struct {
	name, provider string
	unavailable    string
	findings       []report.Finding
	ran            *bool
}

func (f *fake) Name() string                    { return f.name }
func (f *fake) Provider() string                { return f.provider }
func (f *fake) Describe(backend.Request) string { return "fake " + f.name }

func (f *fake) Review(context.Context, backend.Request) (report.Result, error) {
	if f.ran != nil {
		*f.ran = true
	}
	if f.unavailable != "" {
		return report.Result{}, &backend.Unavailable{Backend: f.name, Reason: f.unavailable}
	}
	return report.Result{Backend: f.name, Provider: f.provider, Model: "m", Findings: f.findings}, nil
}

func request() backend.Request {
	return backend.Request{Role: "r2", Base: "main", Head: "feat/x", Diff: "d"}
}

func TestAdvancesTheChainWhenABackendReportsUnavailable(t *testing.T) {
	r := &Runner{Role: "r2", Chain: []backend.Backend{
		&fake{name: "codex", provider: "openai", unavailable: "out of quota"},
		&fake{name: "gemini", provider: "google"},
	}}
	out, err := r.Run(context.Background(), request())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !out.Ran {
		t.Fatal("the chain did not review")
	}
	if out.Result.Backend != "gemini" {
		t.Errorf("reviewed by %q, want gemini", out.Result.Backend)
	}
	if len(out.Skipped) != 1 || out.Skipped[0].Backend != "codex" {
		t.Errorf("skipped = %+v, want codex named with its reason", out.Skipped)
	}
	if !strings.Contains(out.Skipped[0].Reason, "quota") {
		t.Errorf("skip reason %q does not carry why", out.Skipped[0].Reason)
	}
}

func TestStopsTheChainAtTheFirstBackendThatReviews(t *testing.T) {
	secondRan := false
	r := &Runner{Role: "r2", Chain: []backend.Backend{
		&fake{name: "codex", provider: "openai"},
		&fake{name: "gemini", provider: "google", ran: &secondRan},
	}}
	out, _ := r.Run(context.Background(), request())
	if out.Result.Backend != "codex" {
		t.Errorf("reviewed by %q, want codex", out.Result.Backend)
	}
	if secondRan {
		t.Error("the chain kept going after a backend reviewed")
	}
}

func TestReportsTheBackendProviderAndModelThatActuallyReviewed(t *testing.T) {
	r := &Runner{Role: "r2", Chain: []backend.Backend{&fake{name: "gemini", provider: "google"}}}
	out, _ := r.Run(context.Background(), request())
	if out.Result.Backend != "gemini" || out.Result.Provider != "google" || out.Result.Model != "m" {
		t.Errorf("result = %+v; a fallback that is not named is a fallback that passes for the original", out.Result)
	}
}

func TestNamesEveryBackendTriedWhenNoneWasAvailable(t *testing.T) {
	r := &Runner{Role: "r2", Chain: []backend.Backend{
		&fake{name: "codex", provider: "openai", unavailable: "not installed"},
		&fake{name: "gemini", provider: "google", unavailable: "no key"},
	}}
	out, err := r.Run(context.Background(), request())
	if err != nil {
		t.Fatalf("Run: %v — a chain with nothing available is not an error", err)
	}
	if out.Ran {
		t.Error("Ran must be false when nothing reviewed")
	}
	if len(out.Skipped) != 2 {
		t.Fatalf("skipped %d, want both named: %+v", len(out.Skipped), out.Skipped)
	}
}

func TestANonUnavailableErrorStopsTheChain(t *testing.T) {
	// A backend that failed mid-review has reviewed. Advancing past it would
	// report a different, possibly weaker, reviewer as the one that ran.
	boom := &brokenBackend{}
	second := false
	r := &Runner{Role: "r2", Chain: []backend.Backend{boom, &fake{name: "gemini", provider: "google", ran: &second}}}
	if _, err := r.Run(context.Background(), request()); err == nil {
		t.Fatal("want the error to surface")
	}
	if second {
		t.Error("the chain advanced past a backend that failed mid-review")
	}
}

type brokenBackend struct{}

func (b *brokenBackend) Name() string                    { return "broken" }
func (b *brokenBackend) Provider() string                { return "x" }
func (b *brokenBackend) Describe(backend.Request) string { return "broken" }
func (b *brokenBackend) Review(context.Context, backend.Request) (report.Result, error) {
	return report.Result{}, errBroken
}

var errBroken = &customError{"the reviewer crashed halfway"}

type customError struct{ msg string }

func (e *customError) Error() string { return e.msg }

// --- the cross-provider state ----------------------------------------------

func TestComputesTheStateOnlyForTheRoleThatCarriesTheRule(t *testing.T) {
	r := &Runner{Role: "r1", Chain: []backend.Backend{&fake{name: "cheap", provider: "openai"}},
		Declaration: &vcs.Declaration{Provider: "anthropic"}}
	out, _ := r.Run(context.Background(), request())
	if out.CrossProvider != StateNA {
		t.Errorf("state = %q for r1; computing it elsewhere invites a reader to think R1 enforces it", out.CrossProvider)
	}
}

func TestReportsVerifiedWhenAFingerprintCorroboratesADifferingProvider(t *testing.T) {
	r := &Runner{Role: "r2", RequireCrossProvider: true,
		Chain:       []backend.Backend{&fake{name: "codex", provider: "openai"}},
		Declaration: &vcs.Declaration{Provider: "anthropic", Model: "claude-opus-5"},
		Fingerprint: "anthropic"}
	out, err := r.Run(context.Background(), request())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.CrossProvider != StateVerified {
		t.Errorf("state = %q, want verified", out.CrossProvider)
	}
	if !out.CrossProvider.Satisfies() {
		t.Error("verified must satisfy the rule")
	}
}

func TestReportsDeclaredWhenOnlyTheBranchRecordAssertsIt(t *testing.T) {
	r := &Runner{Role: "r2", RequireCrossProvider: true,
		Chain:       []backend.Backend{&fake{name: "codex", provider: "openai"}},
		Declaration: &vcs.Declaration{Provider: "anthropic"}}
	out, _ := r.Run(context.Background(), request())
	if out.CrossProvider != StateDeclared {
		t.Errorf("state = %q, want declared", out.CrossProvider)
	}
	if !strings.Contains(out.CrossProviderNote, "not corroborated") {
		t.Errorf("note %q must say the claim is unverified", out.CrossProviderNote)
	}
}

func TestSaysWhichSideOfTheClaimWasCorroboratedAndWhichWasNot(t *testing.T) {
	// Both provider names are labels written by hand in configuration. On this
	// machine `providers.openai.endpoint` points at another vendor entirely, so
	// a reviewer labelled "openai" reaches DeepSeek and the check still reports
	// "openai". Nothing here can see that, so the note must not let a pull
	// request reader read the state as independence somebody established.
	for _, tc := range []struct {
		name        string
		fingerprint string
		want        CrossProviderState
	}{
		{"corroborated author", "anthropic", StateVerified},
		{"declared author", "", StateDeclared},
	} {
		r := &Runner{Role: "r2", RequireCrossProvider: true,
			Chain:       []backend.Backend{&fake{name: "codex", provider: "openai"}},
			Declaration: &vcs.Declaration{Provider: "anthropic"},
			Fingerprint: tc.fingerprint}
		out, err := r.Run(context.Background(), request())
		if err != nil {
			t.Fatalf("%s: Run: %v", tc.name, err)
		}
		if out.CrossProvider != tc.want {
			t.Fatalf("%s: state = %q, want %q", tc.name, out.CrossProvider, tc.want)
		}
		if !strings.Contains(out.CrossProviderNote, "label") {
			t.Errorf("%s: note %q does not say the Reviewer's provider is an unchecked label", tc.name, out.CrossProviderNote)
		}
		if !strings.Contains(out.CrossProviderNote, "endpoint") {
			t.Errorf("%s: note %q does not say what was never checked against that label", tc.name, out.CrossProviderNote)
		}
	}
}

func TestReportsUnknownAndDoesNotSatisfyR2WhenNothingRecordedTheAuthor(t *testing.T) {
	r := &Runner{Role: "r2", RequireCrossProvider: true,
		Chain: []backend.Backend{&fake{name: "codex", provider: "openai"}}}
	out, _ := r.Run(context.Background(), request())
	if out.CrossProvider != StateUnknown {
		t.Errorf("state = %q, want unknown", out.CrossProvider)
	}
	if out.CrossProvider.Satisfies() {
		t.Error("unknown must not satisfy R2; collapsing it into satisfied is the assumption this exists to remove")
	}
	if !strings.Contains(out.CrossProviderNote, "author declare") {
		t.Errorf("note %q does not tell the user how to fix it", out.CrossProviderNote)
	}
}

func TestDoesNotSatisfyR2WhenTheReviewerSharesTheAuthorsProvider(t *testing.T) {
	r := &Runner{Role: "r2", RequireCrossProvider: true,
		Chain:       []backend.Backend{&fake{name: "codex", provider: "openai"}},
		Declaration: &vcs.Declaration{Provider: "openai"}}
	out, _ := r.Run(context.Background(), request())
	if out.CrossProvider.Satisfies() {
		t.Errorf("state = %q; a same-provider reviewer cannot satisfy R2", out.CrossProvider)
	}
	if !strings.Contains(out.CrossProviderNote, "different provider") {
		t.Errorf("note %q does not explain the refusal", out.CrossProviderNote)
	}
}

func TestFailsLoudlyWhenADetectedProviderContradictsTheDeclaration(t *testing.T) {
	ran := false
	r := &Runner{Role: "r2", RequireCrossProvider: true,
		Chain:       []backend.Backend{&fake{name: "codex", provider: "openai", ran: &ran}},
		Declaration: &vcs.Declaration{Provider: "anthropic"},
		Fingerprint: "google"}
	_, err := r.Run(context.Background(), request())
	if err == nil {
		t.Fatal("a contradiction must not be resolved silently")
	}
	if !strings.Contains(err.Error(), "anthropic") || !strings.Contains(err.Error(), "google") {
		t.Errorf("error %q must name both claims", err)
	}
	if ran {
		t.Error("the chain ran despite an unresolved contradiction about who authored the change")
	}
}

func TestStateIsUnknownWhenNoBackendReviewedAtAll(t *testing.T) {
	r := &Runner{Role: "r2", RequireCrossProvider: true,
		Chain:       []backend.Backend{&fake{name: "codex", provider: "openai", unavailable: "no quota"}},
		Declaration: &vcs.Declaration{Provider: "anthropic"}}
	out, _ := r.Run(context.Background(), request())
	if out.CrossProvider != StateUnknown {
		t.Errorf("state = %q, want unknown when nothing reviewed", out.CrossProvider)
	}
}

// --- describe ---------------------------------------------------------------

func TestDescribeCoversTheWholeChainAndRunsNothing(t *testing.T) {
	ranFirst := false
	r := &Runner{Role: "r2", Chain: []backend.Backend{
		&fake{name: "codex", provider: "openai", ran: &ranFirst},
		&fake{name: "gemini", provider: "google"},
	}}
	lines := r.Describe(request())
	if len(lines) != 2 {
		t.Errorf("described %d backends, want the whole chain including fallbacks", len(lines))
	}
	if ranFirst {
		t.Error("Describe ran a backend")
	}
}
