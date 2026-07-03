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
- R, Review: review changed files locally in the editor, applying the
  Self-Review section of `ai_guidelines.md`. Make atomic commits for related
  changes.
- U, Upload: run git push with clear, descriptive commit messages
  (Conventional Commits per `github.md`). The push triggers the R2
  cross-provider gate (`codex_review.md`).
- RA, Review Again: open a Pull Request and review everything in the Files
  Changed tab before requesting review, backed by the PR Review Checklist of
  `github.md`. Fix overlooked details (logs, bad names).

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
