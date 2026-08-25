// Package role walks a role's backend chain until one actually reviews, and
// reports which one did.
//
// Falling back is allowed; falling back quietly is not. A weaker reviewer is
// not the same review, so the outcome always names the backend, provider and
// model that ran, and names every backend it skipped on the way.
package role

import (
	"context"
	"fmt"
	"strings"

	"github.com/LukeSantossz/my-framework/internal/backend"
	"github.com/LukeSantossz/my-framework/internal/report"
	"github.com/LukeSantossz/my-framework/internal/vcs"
)

// CrossProviderState says how strongly R2's provider requirement is
// established. Three values, not two: collapsing them would report "nobody
// recorded it" as "satisfied", and an unverified claim presented as an enforced
// one is worse than no claim, because the PR reader trusts it.
type CrossProviderState string

const (
	// StateVerified and StateDeclared both describe the Author's side of the
	// claim, and only that side: whether an independent signal corroborated the
	// declaration. Neither says anything about the Reviewer's provider, which
	// is a label in configuration that nothing here can check — see state.
	StateVerified CrossProviderState = "verified"
	StateDeclared CrossProviderState = "declared"
	StateUnknown  CrossProviderState = "unknown"
	StateNA       CrossProviderState = "not-applicable"
)

// Satisfies reports whether this state can satisfy the cross-provider rule.
func (s CrossProviderState) Satisfies() bool {
	return s == StateVerified || s == StateDeclared
}

// Skip records a backend the chain passed over and why.
type Skip struct {
	Backend string
	Reason  string
}

// Outcome is what the chain did.
type Outcome struct {
	Role    string
	Ran     bool
	Result  report.Result
	Skipped []Skip

	CrossProvider     CrossProviderState
	CrossProviderNote string
}

// Runner walks one role's chain.
type Runner struct {
	Role  string
	Chain []backend.Backend

	// RequireCrossProvider is true only for the role that carries the rule.
	// Computing the state elsewhere would invite a reader to believe R1 or R3
	// enforces something it does not.
	RequireCrossProvider bool

	// Declaration is the Author record for this change, absent when nobody
	// wrote one. Fingerprint is an independent signal of which agent is in the
	// chair, empty when there is none.
	Declaration *vcs.Declaration
	Fingerprint string
}

// ContradictionError is raised when an independent signal disagrees with the
// Author Declaration. It is loud on purpose: silently preferring one over the
// other would decide, on the user's behalf, which of two conflicting claims
// about provenance is true.
type ContradictionError struct {
	Declared string
	Detected string
}

func (e *ContradictionError) Error() string {
	return fmt.Sprintf(
		"the Author Declaration says provider %q but this session looks like %q; resolve the contradiction rather than letting one silently win",
		e.Declared, e.Detected)
}

// Run walks the chain. A backend that reports unavailable advances it; the
// first backend that reviews stops it.
func (r *Runner) Run(ctx context.Context, req backend.Request) (Outcome, error) {
	out := Outcome{Role: r.Role, CrossProvider: StateNA}

	if r.RequireCrossProvider {
		if err := r.checkContradiction(); err != nil {
			return out, err
		}
	}

	for _, b := range r.Chain {
		result, err := b.Review(ctx, req)
		if err != nil {
			if backend.IsUnavailable(err) {
				out.Skipped = append(out.Skipped, Skip{Backend: b.Name(), Reason: unavailableReason(err)})
				continue
			}
			return out, err
		}
		out.Ran = true
		out.Result = result
		if r.RequireCrossProvider {
			out.CrossProvider, out.CrossProviderNote = r.state(b.Provider())
		}
		return out, nil
	}

	// Every backend was unavailable. That is not a finding, so it must never
	// block; it is recorded so the absence reaches the PR instead of passing
	// for a review that happened.
	if r.RequireCrossProvider {
		out.CrossProvider = StateUnknown
		out.CrossProviderNote = "no backend reviewed, so nothing can be said about the Reviewer's provider"
	}
	return out, nil
}

func (r *Runner) checkContradiction() error {
	if r.Declaration == nil || r.Fingerprint == "" {
		return nil
	}
	if !strings.EqualFold(r.Declaration.Provider, r.Fingerprint) {
		return &ContradictionError{Declared: r.Declaration.Provider, Detected: r.Fingerprint}
	}
	return nil
}

// state resolves how strongly the cross-provider requirement holds for the
// backend that actually reviewed.
//
// Both sides of the comparison are names somebody typed into configuration, and
// only the Author's side has anything behind it: the environment fingerprint
// corroborates it independently. The Reviewer's side has nothing. A backend
// declared `provider = "openai"` whose machine layer points that provider at
// another vendor's endpoint still reports "openai" here, and this package
// cannot see the endpoint at all.
//
// Detecting the vendor behind an arbitrary OpenAI-compatible URL was considered
// and rejected — issue #16 — because that shape is exactly what Ollama, vLLM,
// DeepSeek, Groq and a dozen others speak, and no reliable signal distinguishes
// them. So the honest move is not to guess but to say what was corroborated and
// what was not, in the line meant for the pull request: on the principle the
// chain already holds, an unverified claim presented as an enforced one is worse
// than no claim, because the reader trusts it.
func (r *Runner) state(reviewerProvider string) (CrossProviderState, string) {
	// Repeated in every satisfying state rather than written once at the end of
	// the run, because this note is what a reader copies into the PR's
	// review-layers record, and half of it copied is the half that overclaims.
	reviewer := fmt.Sprintf(
		"reviewer %s (a configured label; nothing here checked it against the endpoint that backend reached)",
		reviewerProvider)

	if r.Declaration == nil {
		return StateUnknown, "no Author Declaration for this branch; run `mf author declare` while authoring"
	}
	if strings.EqualFold(r.Declaration.Provider, reviewerProvider) {
		return StateUnknown, fmt.Sprintf(
			"the Reviewer's provider (%s) is the Author's; R2 requires a different provider",
			reviewerProvider)
	}
	if r.Fingerprint != "" {
		return StateVerified, fmt.Sprintf(
			"author %s (corroborated by an environment fingerprint), %s", r.Declaration.Provider, reviewer)
	}
	return StateDeclared, fmt.Sprintf(
		"author %s (declared, not corroborated), %s", r.Declaration.Provider, reviewer)
}

func unavailableReason(err error) string {
	var u *backend.Unavailable
	if ok := asUnavailable(err, &u); ok {
		return u.Reason
	}
	return err.Error()
}

// Describe renders what the chain would do, running nothing. It describes every
// backend rather than stopping at the first: the point is to show the fallbacks.
func (r *Runner) Describe(req backend.Request) []string {
	lines := make([]string, 0, len(r.Chain))
	for _, b := range r.Chain {
		lines = append(lines, fmt.Sprintf("%s [%s]: %s", b.Name(), b.Provider(), b.Describe(req)))
	}
	return lines
}
