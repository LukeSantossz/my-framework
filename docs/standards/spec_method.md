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

`docs/specs/` is the default location, not a constant: a repository that keeps
its documents elsewhere sets `paths.specs` (and `paths.adr` for the decision
records), and every gate reads them from there. The layout is what is fixed, not
the prefix.

The title line must be the first line of the file. It is what identifies the
document as a spec, and `mf check records` fails a file in the archive that does
not open with it. There is no Status field while a spec stands; one is added
only when it is retired, per Durable Numbers Are Never Reused below.

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

A number, once assigned to a spec under the specs directory or to an ADR under
the ADR directory, is never reused. The number is part of every reference to
that record — in a README row, in another spec, in a commit message, in a review
thread — so handing it to a different record silently rewrites what all of those
references mean. A record that is superseded or withdrawn is marked in place: it
keeps its number and its file, and gains a line saying what became of it. It is
not deleted. Deleting a durable record and reusing its number makes every
existing reference to that number ambiguous, because the same citation resolves
to a different decision depending on when it was written, and nothing in the
text tells the reader which one was meant.

### How a retired record is marked

An ADR carries a `## Status` section from the day it is written, so retiring one
is editing a section that already exists: its `Accepted.` becomes the retirement
line below. A spec carries no such section, and deliberately does not gain one:
a spec that is in force says so by existing, and a field that every spec must
carry saying `Active` is a field that rots into a lie the first time someone
forgets it. So for a spec the marking is **added at retirement and absent before
it**:

- **Heading**: `## Status`.
- **Location**: immediately below the `# SPEC:` title, before `## Problem`. The
  title line stays first — it is what identifies the file as a spec, and
  `mf check records` reads it there — and the status is the next thing a reader
  meets, because a retired record read as a live one is the failure this marking
  exists to prevent.
- **Content**: one line, in one of two forms.

  ```markdown
  ## Status

  Retired — superseded by spec NNNN (<title>). <One sentence on what changed.>
  ```

  ```markdown
  ## Status

  Withdrawn — <one sentence on why it was never implemented>.
  ```

Nothing else in the file is edited. Its Problem, Design Decision, Alternatives
Considered and Acceptance Criteria stay exactly as they were approved: the
archive's value is that it holds what was decided at the time, and rewriting a
retired spec to agree with its successor destroys the only evidence of the
change of mind. The successor is what carries the new intent.

A record that supersedes another says so too, in its own Design Decision, so the
link is readable from both ends. One-way marking leaves a reader who arrives at
the successor unable to tell that anything was replaced.

### The rule is checked, not trusted

`mf check records` enforces it, in three ways, because no one of them sees what
the others do:

- **Contiguity.** The numbers in each archive must run from `0001` with no gap
  and no duplicate.
- **History.** Every spec and ADR ever committed must still be present. This is
  the load-bearing one: contiguity alone cannot see the deletion of the
  highest-numbered record, which leaves the rest contiguous and is the shape the
  incident behind this rule actually had. A repository may account for records
  removed before the rule existed, in a recorded list that makes each removal a
  conscious act leaving a trace; a repository that records nothing gets the
  strict reading, where every deletion fails.
- **Headers.** Each file in the specs archive must open with `# SPEC:`. A
  directory of files is not an archive unless each file says what it is.

An adopting repository inherits the rule and the gate together, because both
travel in the binary rather than in a script the adopter has to copy.

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
Decision Record in the ADR directory. The SPEC's Alternatives Considered is durable —
it is archived in the specs directory alongside the rest of the approved spec — but the
ADR stays the curated home for decision rationale: an ADR records one decision for an
outside reader, while the spec archive preserves each change's gate-approved intent,
scope, and acceptance criteria as a whole. The README Engineering Decisions later
links the ADR rather than restating it. Later promotion is allowed when a decision's
significance only emerges during implementation.

Three tests decide the promotion, and all three must hold: the decision is hard to
reverse, it is surprising to a reader without the context, and a real alternative was
rejected for a stated reason. A decision failing any of them stays in the spec, where
it is still archived and still readable; promoting everything would make the ADR
directory a second changelog and leave the curated index worth nothing.

## Where It Sits in the Pipeline

1. Brainstorm refines requirements into a draft spec.
2. The draft is shown in chunks short enough to read and digest.
3. The Developer approves at the Spec Gate.
4. The approved spec is committed to `docs/specs/NNNN-<slug>.md`; it is not a
   working copy overwritten by the next change.
5. The Plan turns each Acceptance Criterion into a failing test, then implementation.

Naming, code rules, commits, and review continue to follow `code_conventions.md`,
`var_method.md`, `github.md`, and `ai_guidelines.md` from this point on.
