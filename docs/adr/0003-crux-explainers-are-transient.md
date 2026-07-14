# CRUX explainers are transient; ADRs stay the durable record

The CRUX Method (`docs/standards/crux_method.md`) produces an explanation of an
implemented change for review. The reference format writes that explanation to a
date-prefixed HTML file kept out of version control. We keep it that way: a CRUX
explainer is a transient review aid, not a durable record. The durable "what and
why" of a change stays where it already lives — the SPEC archive for gate-approved
intent and the ADR for curated rationale. CRUX adds no new versioned per-change
artifact.

## Status

Accepted.

## Considered Options

- **Transient explainer, ADR stays the durable record (chosen)**: the explainer is
  a review aid generated outside version control; durable rationale remains in the
  ADR, indexed by the README Engineering Decisions.
- **Persist a durable per-change explanation record in the repository**: rejected —
  explainers are reviews of already implemented code and need no persistence; a
  second durable record would duplicate the ADR and bloat the archive, the same
  disproportion the spec-archive curation removed.
- **Keep no record and rely on the explainer alone**: rejected — a throwaway HTML
  file is not traceable; "we shipped knowing what we did" needs the durable ADR,
  not a dated file a reviewer may discard.

## Consequences

- The CRUX explainer is generated outside version control and is not committed.
- ADRs remain the durable home for decision rationale; the SPEC archive remains the
  durable home for gate-approved intent, scope, and acceptance criteria.
- The README Engineering Decisions table indexes this ADR.
