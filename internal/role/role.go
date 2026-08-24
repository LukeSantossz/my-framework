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
func (r *Runner) state(reviewerProvider string) (CrossProviderState, string) {
	if r.Declaration == nil {
		return StateUnknown, "no Author Declaration for this branch; run `mf author declare` while authoring"
	}
	if strings.EqualFold(r.Declaration.Provider, reviewerProvider) {
		return StateUnknown, fmt.Sprintf(
			"the Reviewer's provider (%s) is the Author's; R2 requires a different provider",
			reviewerProvider)
	}
	if r.Fingerprint != "" {
		return StateVerified, fmt.Sprintf("author %s (corroborated), reviewer %s", r.Declaration.Provider, reviewerProvider)
	}
	return StateDeclared, fmt.Sprintf("author %s (declared, not corroborated), reviewer %s", r.Declaration.Provider, reviewerProvider)
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
