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

## The Artifact: `SPEC.md`

One spec per feature, adjustment, or refactor that is non-trivial. Skip it only
for changes too small to have a design (a typo, a one-line fix). When in doubt,
write the spec; it is cheaper than the rework it prevents.

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

## The Spec Gate

The Gate is the human checkpoint between design and implementation. A spec passes
the Gate only when all of the following hold:

- Problem is stated in one sentence.
- Scope is filled, including a non-empty "Does NOT include" list.
- At least one Acceptance Criterion exists and is verifiable.

A spec missing any of these is not ready; the agent must not proceed to the Plan.
The author approves the spec at the Gate before implementation starts.

## Where It Sits in the Pipeline

1. Brainstorm refines requirements into a draft spec.
2. The draft is shown in chunks short enough to read and digest.
3. The author approves at the Spec Gate.
4. The Plan turns each Acceptance Criterion into a failing test, then implementation.

Naming, code rules, commits, and review continue to follow `code_conventions.md`,
`var_method.md`, `github.md`, and `ai_guidelines.md` from this point on.
