# CRURA Method

Human review discipline. Counterpart to the Self-Review section of `ai_guidelines.md`; feeds the PR Review Checklist in `github.md`.

## Benefits

- Reviews the same code at least 3 times.
- Reduces the chance of forgetting what you did; forces understanding the solution.
- Avoids trivial feedback from reviewers.

## Stages

- C, Change: write the feature, adjustment, or refactor with intent.
- R, Review: review changed files locally in the editor. Make atomic commits for related changes.
- U, Upload: run git push. Use clear, descriptive commit messages.
- RA, Review Again: open a Pull Request, review everything in the Files Changed tab before requesting review. Fix overlooked details (logs, bad names).
