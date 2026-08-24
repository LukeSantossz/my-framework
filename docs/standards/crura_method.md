# CRURA Method

Human review discipline. Counterpart to the Self-Review section of
`ai_guidelines.md`; feeds the PR Review Checklist in `github.md`.

## Benefits

- The same change is inspected at every stage boundary: locally at R, on the
  platform at RA, and finally by the human arbiter over the recorded review
  layers.
- Reduces the chance of forgetting what you did; forces understanding the
  solution.
- Avoids trivial feedback from reviewers.

## Stages

- C, Change: write the feature, adjustment, or refactor with intent, against
  a SPEC approved at the Spec Gate (`spec_method.md`) when the change is
  non-trivial.
- R, Review: read the changed files locally in the editor, applying the
  Self-Review section of `ai_guidelines.md`.
  This read is **triggered rather than unconditional** —
  it is owed when one of the Triggers below fires, or
  when the change is drawn as part of the Untriggered Sample. Make atomic
  commits for related changes either way.
- U, Upload: run git push with clear, descriptive commit messages
  (Conventional Commits per `github.md`). The push triggers the R2
  cross-provider gate (`r2_gate.md`).
- RA, Review Again: open a Pull Request. This stage does two different jobs,
  and only one of them is conditional.
  - Adjudicate: read what the review layers recorded — findings, their
    resolutions, and any recorded absence — and decide whether to merge.
    Here, **adjudication and the merge decision are unconditional**. They run on
    every change, because the Developer is accountable for what ships and
    accountability without a mandatory decision point is a claim nobody can
    exercise.
  - Re-read: read the diff line by line in the Files Changed tab, fixing
    overlooked details (logs, bad names). Like R, this read is triggered
    rather than unconditional.

  Both are backed by the PR Review Checklist of `github.md`.

## Why the reads are conditional

Verifying model output at a fixed rate does not survive its own success. Where
the automated layers are good enough that intervention is rare, sustained
vigilance is not achievable; where they are poor enough to keep a reader
vigilant, the reading costs more than it returns. There is no quality level at
which a fixed-rate read is simultaneously effective and worth its price, so
spending human attention that way manufactures alert fatigue and buys little.

The always-on human obligation is not removed. It is **moved to the Spec Gate**
(`spec_method.md`), where the Developer decides the problem, the scope and the
acceptance criteria before any code exists. That is a decision made against an
independent basis for judgment, rather than a verification made against nothing,
and it is the point in the process where human attention is worth the most.

## Triggers

A read is owed when any of these holds. The list is enumerated rather than left
to judgment: a trigger set that is not written down degrades into "never", and
the re-scope becomes a deletion.

- A finding of category `security`, at any severity.
- A finding marked `blocking`.
- Any deterministic check failing.
- The change touches a path the project declares high-risk.
- R2's cross-provider state is `unknown` (`r2_gate.md`).
- No review layer ran at all, every backend having been unavailable.
- The change is drawn as part of the Untriggered Sample below.

The list references finding categories and check names that later work will
change, so it is revisited whenever the review runtime gains a category. A stale
trigger list fails silently — it simply stops firing.

## Instrumentation

Re-scoping the reads is a bet, not a proven result, so it ships with the means
to test it. Every read performed under this method records its outcome in the
PR, in the fixed vocabulary of the PR Review Checklist: whether it found a
defect the automated layers missed, and how many.

Recording only the reads a Trigger fired on would be a broken measurement. It
observes exactly the population already suspected, so the evidence it produces
can confirm the trigger set and **never correct it** — a missing trigger hides
precisely in the changes nobody looked at. The mitigation is a
**periodic sample of untriggered changes**:
the Developer sets the period and draws changes no
Trigger selected, reads them, and records the outcome the same way.

The sample is what makes the evidence able to falsify this decision instead of
merely ratifying it. Its period is deliberately not fixed here, because no data
exists yet to set one; the cost of that is real, since an unset period is
indistinguishable from a period of never and nothing detects the difference.

Both the reads and the sample depend on the Developer honestly recording what
was found. Nothing can know what a person noticed, so this is an attestation
rather than a measurement, and the accumulated record must be read as the weaker
thing it is.

## Composition with the Review Layers

CRURA is the human thread through the machine layers of `ai_guidelines.md`
Review Composition. R1 (internal review), R2 (cross-provider review), and R3
(automated PR review) record their results — or, when a layer legitimately
did not run (per `ai_guidelines.md` Review Composition), its recorded absence
and the reason. The human review consumes those records — verifying each
layer either ran or has its absence noted, reading findings and their
adjudications — rather than repeating the layers' work. The human is the
final arbiter: an unresolved layer finding must be addressed or justified
before merge, and the merge decision itself is always human.
