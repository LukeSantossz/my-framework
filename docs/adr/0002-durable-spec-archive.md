# Durable spec archive under `docs/specs/`

ADR 0001 made the root `SPEC.md` the transient working copy, overwritten by the next
change, with rationale promoted to an ADR the only durable trace. In practice this
discarded more than rationale: every cycle's gate-approved Problem, Scope, and
Acceptance Criteria were lost too, recoverable only through git archaeology. We make
specs durable: an approved spec is authored directly under `docs/specs/NNNN-<slug>.md`
(numbered like ADRs) and stays there, never overwritten. This does not collapse the
SPEC and the ADR into one artifact — the ADR remains the curated home for decision
rationale. Audience and lifetime differ: an ADR records one decision, curated for an
outside reader; the spec archive preserves each change's whole gate-approved intent,
scope, and acceptance criteria, as a chronological record rather than a curated one.
This amends ADR 0001's transiency claim.

## Status

Accepted.

## Considered Options

- **Durable spec archive, ADR stays the curated decision home (chosen)**: specs move
  to `docs/specs/NNNN-<slug>.md` and are archived, not overwritten; ADRs keep recording
  the promoted, curated rationale, referenced rather than restated.
- **Keep authoring at the root `SPEC.md` and archive a copy at merge**: rejected — it
  creates dual maintenance when adjudication amends the spec mid-PR, and no cheap guard
  can force the copy to stay equal to the original.
- **Keep specs transient and rely on ADRs alone** (the ADR 0001 status quo): rejected —
  an ADR records one curated decision, not the approved scope and acceptance criteria
  of a change; gate-approved intent was lost every cycle.
- **Collapse SPEC and ADR into one durable artifact**: rejected — the two serve
  different readers on different lifetimes; an outside reader wants the curated "why"
  of one decision, not every change's full gate record.

## Consequences

- The root `SPEC.md` is retired; specs are authored directly under
  `docs/specs/NNNN-<slug>.md`, numbered sequentially.
- ADR 0001's "the root SPEC.md stays transient" consequence is amended: the spec is
  now durable, but the ADR remains the curated, promoted home for decision rationale
  that the README Engineering Decisions indexes.
- A spec's Alternatives Considered persists in the archive as a historical record; it
  is still promoted to an ADR at the Spec Gate when the decision is hard to reverse,
  surprising without context, and a real trade-off.
- `spec_method.md`, `docs/standards/INDEX.md`, and `CONTEXT.md` state these rules.
