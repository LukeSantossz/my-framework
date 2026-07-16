# SPEC Method

The design layer that runs before any code. Counterpart to the Brainstorm and
Plan phases of the Superpowers orchestrator; the artifact it produces (`SPEC.md`)
is what the Spec Gate approves before implementation begins.

This is the layer the rest of the standards assumed but never defined. `github.md`
has an Issue Model for task tracking and a README with an Engineering Decisions
section, but neither forces design decisions, discarded alternatives, or
verifiable acceptance criteria *before* code is written. SPEC Method fills that gap.

## Why It Exists

- A coding agent left without a spec starts writing code immediately and drifts.
  The brainstorm-then-spec discipline resolves architectural decisions while they
  are still cheap to change.
- Discarded alternatives recorded before implementation prove trade-off reasoning
  at the point it matters, not retroactively in the README.
- Verifiable acceptance criteria written up front become the failing tests in the Plan.

## The Artifact: `docs/specs/NNNN-<slug>.md`

One spec per feature, adjustment, or refactor that is non-trivial, authored
directly under `docs/specs/NNNN-<slug>.md` — numbered sequentially, the next
free number, `slug` a short kebab-case phrase drawn from the spec's own title.
Skip it only for changes too small to have a design (a typo, a one-line fix);
a change too small for a full spec but not skippable uses the Spec-lite tier
below instead. When in doubt, write the spec; it is cheaper than the rework
it prevents.

```markdown
# SPEC: <title in Conventional Commits format>

## Problem
One sentence. What is broken or missing, from the user's or system's point of view.

## Design Decision
The chosen approach in two to four sentences.

## Alternatives Considered
Minimum two, each with the reason it was rejected. Same rigor as the README
Engineering Decisions section, but recorded before coding, not after.

## Scope
- Includes: <list>
- Does NOT include: <list>   # mandatory; this is what blocks scope creep

## Acceptance Criteria
Verifiable, phrased as test outcomes (returns_empty_list_when_no_matches).
Each criterion becomes a test in the Plan.

## Reproducibility
The exact command to run, the seed if randomness is involved, and the relevant
versions. A result that does not reproduce is not a result.

## Risks and Assumptions
Assumptions declared in one line each (per `ai_guidelines.md` Declare Assumptions)
and what would invalidate this spec.
```

## Durable Numbers Are Never Reused

A number, once assigned to a spec under `docs/specs/` or to an ADR under
`docs/adr/`, is never reused. The number is part of every reference to that
record — in a README row, in another spec, in a commit message, in a review
thread — so handing it to a different record silently rewrites what all of those
references mean. A record that is superseded or withdrawn is marked Retired in
place: it keeps its number and its file, and its text says what superseded it.
It is not deleted. Deleting a durable record and reusing its number makes every
existing reference to that number ambiguous, because the same citation resolves
to a different decision depending on when it was written, and nothing in the
text tells the reader which one was meant.

In this repository the rule is enforced rather than trusted, by two checks in
the docs-consistency self-test. The first reads git history: every spec and ADR
ever committed must still be present, unless it is listed in the guard as
deliberately retired with a stated reason, which makes retiring a record a
conscious act that leaves a trace. The second requires the numbers to run from
`0001` with no gap and no duplicate. The history check is the load-bearing one:
contiguity alone cannot see the deletion of the highest-numbered record, which
leaves the rest contiguous and is the shape the incident behind this rule
actually had.

An adopting repository copies this standard but runs only `docs-consistency.sh`
and drops the self-test suite (see the README), so it inherits the rule without
those checks and carries it on discipline until it wires an equivalent. The
rule is the never-overwritten rule of `docs/adr/0002-durable-spec-archive.md`
made explicit for the case of deleting a record and reusing its number.

## Spec-lite

A lighter tier for a change that needs no Design Decision worth recording —
there is no real trade-off to weigh, so there is nothing for Alternatives
Considered to hold. A spec-lite spec is still authored under
`docs/specs/NNNN-<slug>.md` and keeps exactly the three Gate-checked sections:

```markdown
# SPEC: <title in Conventional Commits format>

## Problem
One sentence. What is broken or missing, from the user's or system's point of view.

## Scope
- Includes: <list>
- Does NOT include: <list>   # mandatory; this is what blocks scope creep

## Acceptance Criteria
Verifiable, phrased as test outcomes (returns_empty_list_when_no_matches).
Each criterion becomes a test in the Plan.
```

If, while drafting or at the Gate, an Alternatives Considered turns out to be
needed after all, the spec is full-tier: add the Design Decision, Alternatives
Considered, Reproducibility, and Risks and Assumptions sections before it
passes. The Spec Gate criteria below are unchanged for both tiers.

## The Spec Gate

The Gate is the human checkpoint between design and implementation. A spec (of
either tier) passes the Gate only when all of the following hold:

- Problem is stated in one sentence.
- Scope is filled, including a non-empty "Does NOT include" list.
- At least one Acceptance Criterion exists and is verifiable.

A spec missing any of these is not ready; the agent must not proceed to the Plan.
The Developer approves the spec at the Gate before implementation starts.

At the Gate the Developer also promotes any Design Decision that is hard to reverse,
surprising without context, and the result of a real trade-off into an Architecture
Decision Record under `docs/adr/`. The SPEC's Alternatives Considered is durable —
it is archived under `docs/specs/` alongside the rest of the approved spec — but the
ADR stays the curated home for decision rationale: an ADR records one decision for an
outside reader, while the spec archive preserves each change's gate-approved intent,
scope, and acceptance criteria as a whole. The README Engineering Decisions later
links the ADR rather than restating it. Later promotion is allowed when a decision's
significance only emerges during implementation. See
`docs/adr/0001-decision-records-flow.md` and `docs/adr/0002-durable-spec-archive.md`.

## Where It Sits in the Pipeline

1. Brainstorm refines requirements into a draft spec.
2. The draft is shown in chunks short enough to read and digest.
3. The Developer approves at the Spec Gate.
4. The approved spec is committed to `docs/specs/NNNN-<slug>.md`; it is not a
   working copy overwritten by the next change.
5. The Plan turns each Acceptance Criterion into a failing test, then implementation.

Naming, code rules, commits, and review continue to follow `code_conventions.md`,
`var_method.md`, `github.md`, and `ai_guidelines.md` from this point on.
