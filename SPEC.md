# SPEC: docs: add domain glossary and align standards

## Problem
The framework's domain language is overloaded and scattered across the standards
("author" means both the writing model and the human approver; "Self-Review", "Gate",
and "Plan" each name two different things), with no central glossary, and trade-off
rationale has three overlapping homes (SPEC Alternatives Considered, README Engineering
Decisions, ADRs) with no defined relationship — so in practice no decision rationale is
durably recorded.

## Design Decision
Capture the resolved vocabulary in a root `CONTEXT.md` glossary (the source of truth for
terms), align the standards docs to it, and define a single decision-records flow
SPEC -> ADR -> README in which a SPEC's Design Decision is promoted to a durable ADR at
the Spec Gate while Alternatives Considered stays transient and the README Engineering
Decisions indexes the ADR. Record that flow as the first ADR. Also land the agent-skills
configuration (`CLAUDE.md` `## Agent skills` + `docs/agents/`) the engineering skills
require.

## Alternatives Considered
- Keep the vocabulary implicit in each standard, no central glossary: rejected — the
  overloaded terms had already produced a real contradiction in `spec_method.md`, and
  nothing would prevent future drift.
- Make the README Engineering Decisions the durable home for rationale and keep ADRs
  optional: rejected — no README exists and a single product-facing table is too coarse
  for per-decision rationale and supersession (recorded in `docs/adr/0001`).

## Scope
- Includes:
  - `CONTEXT.md`: the domain glossary.
  - Terminology alignment in `spec_method.md`, `github.md`, `crura_method.md`,
    `codex_review.md`, `token_economy.md` (Developer; PR Review Checklist).
  - The decision-records flow in `spec_method.md`, `github.md`, `INDEX.md`, recorded as
    `docs/adr/0001-decision-records-flow.md`.
  - Agent-skills configuration: `## Agent skills` in `CLAUDE.md` and
    `docs/agents/{issue-tracker,triage-labels,domain}.md`.
- Does NOT include:
  - Any behavior change to the Author/Reviewer models or the review scripts.
  - Writing the README itself or its Engineering Decisions table.
  - Retroactive ADRs for already-merged work (e.g. the Codex R2 gate).
  - Creating tracker issues or wiring R3 automated PR review.

## Acceptance Criteria
Phrased as verifiable documentation-consistency outcomes.

- glossary_cross_references_resolve: every term referenced inside a `CONTEXT.md`
  definition is itself defined, and no `_Avoid_` synonym is used as a definition.
- deprecated_wording_absent: `grep -rEn "Self-Review Checklist|author approves"
  docs/standards` returns no matches.
- decision_flow_consistent: `INDEX.md`, `spec_method.md`, `CONTEXT.md`, and
  `docs/adr/0001-decision-records-flow.md` all name the Design Decision (not Alternatives
  Considered) as the artifact promoted to the ADR.
- agent_skills_documented: `CLAUDE.md` has an `## Agent skills` section pointing at the
  three `docs/agents/*.md` files, and those files exist.

## Reproducibility
- Verify on branch `docs/domain-glossary` at the PR head.
- Commands: the three `grep`/read checks above; `ls docs/agents docs/adr`.
- Platform: Windows 11, Git Bash (POSIX sh).

## Risks and Assumptions
- Assumption: this SPEC is written to bring an already-implemented documentation change
  into compliance (retroactive); the Spec Gate review here is the Developer's approval of
  the change as a whole.
- Risk: the root `SPEC.md` is transient and overwrites the prior (merged) Codex-gate
  spec; that content remains in git history (PR #1).
- Assumption: no code changes, so the "tests" are the consistency checks above rather
  than executable unit tests.
