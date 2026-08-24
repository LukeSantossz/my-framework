# Author provenance is a property of the change, not of the push

R2 is valid only when the Reviewer's provider differs from the Author's, and until now
nothing established what the Author's provider was — the gate trusted it. The obvious
fix, detecting which agent is in the chair when the gate runs, is wrong by construction:
the gate runs at push time, and a push carries commits that may come from several
sessions, several agents, or a person typing directly, so there is no single Author to
detect at that moment. Provider identity is a property of the change, so it is recorded
when the change is made: `mf author declare` writes the Author's provider and model per
branch, and the R2 gate reads that record. Environment fingerprints are kept, but only as
a cross-check — a detected provider that contradicts the declaration is a loud error,
never a silent preference.

The report has three states rather than two. `verified` when a fingerprint agrees and
differs from the Reviewer's provider; `declared` when only the branch record asserts it;
`unknown` when there is no signal at all. `unknown` does not satisfy R2. It reports the
requirement as unverified in the line meant for the Pull Request, on the principle the
gate already holds: falling back is allowed, falling back quietly is not.

## Status

Accepted.

## Considered Options

- **Declaration at authoring time, detection as a cross-check, three-state report
  (chosen)**: it asks the question at the only moment the answer exists, and it
  distinguishes an enforced claim from an asserted one instead of collapsing both into
  "satisfied".
- **Declaration alone, with no detection**: rejected — simpler, and free of heuristics
  that age with every vendor release, but a stale declaration is worse than no
  declaration, because it reads as verified and nothing in the system can contradict it.
- **Detection at push time**: rejected — a push has no single Author, so the question has
  no well-defined answer at that moment.
- **Reduce the rule to a report with no gate**: rejected — honest about what a machine can
  guarantee, but it reduces the one rule that distinguishes R2 from every other review
  layer to a convention nothing enforces.

## Consequences

- Most branches will report `declared` rather than `verified`. Fingerprints age with each
  vendor release and exist only for the agents that expose one, so `verified` is the
  exception rather than the norm.
- A branch whose declaration was never written reports `unknown`, which is correct and
  will be common until `mf init` wires the declaration into the authoring flow. A
  framework that reports `unknown` most of the time has an adoption problem, not a
  correctness problem, and the two must not be read as the same defect.
- The PR review-layers record gains the state alongside the backend and model, so the
  human adjudicating the PR can tell an enforced cross-provider claim from an asserted
  one — which is what that record exists to support.
- `CONTEXT.md`, `docs/standards/ai_guidelines.md` and `docs/standards/r2_gate.md` change
  to describe the Author as a recorded property of the change rather than an assumed
  session fact.
