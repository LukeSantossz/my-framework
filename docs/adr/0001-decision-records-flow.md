# Decision records flow: SPEC → ADR → README

Trade-off rationale was specified in three places with overlapping definitions — the
SPEC's Alternatives Considered (before code), the README's Engineering Decisions (after
code), and ADRs — with no defined relationship, and in practice nothing durable: the
root SPEC.md is overwritten each change and no README exists. We make ADRs the single
durable home for a decision's rationale, promoted from a SPEC's Design Decision at the
Spec Gate; the SPEC's Alternatives Considered stays transient, and the README's
Engineering Decisions becomes a curated index that links ADRs rather than restating them.

## Status

Accepted.

## Considered Options

- **Promotion pipeline (chosen)**: SPEC (transient) → ADR (durable) → README (index linking ADRs).
- **README as the durable home**: keep the before/after twin, ADRs optional. Rejected —
  the README does not exist and a single product-facing table is too coarse for
  per-decision rationale and supersession.
- **Collapse to ADR-only**, dropping the Engineering Decisions table. Rejected — it
  redefines `github.md`'s README Model and removes the curated outside-reader view.

## Consequences

- ADRs are promoted at the Spec Gate against the three criteria (hard to reverse,
  surprising without context, real trade-off); late promotion is allowed when
  significance emerges during implementation.
- The root SPEC.md stays transient; its Alternatives Considered is not a durable record.
- README Engineering Decisions rows link the ADR that holds the rationale; the minimum-3
  guidance applies once that many decisions are recorded.
- `spec_method.md`, `github.md`, and `INDEX.md` state these rules.
