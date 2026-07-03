# SPEC: chore(scripts): harden guard failure paths and scope the allowlist comment

## Problem
Three deferred PR #5 review follow-ups leave guard edge cases unhandled: the
exec-bit guard passes silently when `git ls-files` itself fails, the reviewer
parity guard dedupes only the setup side of its comparison, and the
`HYPOTHETICAL_REFS` comment does not state that the exemption also covers
references in `INDEX.md`.

## Scope
- Includes:
  - `scripts/test/docs-consistency.test.sh`, `repo_scripts_are_executable`
    guard: run the `git ls-files` listing so its failure is detected and
    reported as a loud FAIL naming the listing failure, instead of an empty
    result that reads as "all executable".
  - `scripts/test/codex-review.test.sh`, `reviewer_defaults_match_across_scripts`
    guard: the runner-side default extraction gains the same `| sort -u`
    dedup as the setup-side extraction, so duplicate occurrences of one
    default literal cannot produce a spurious mismatch.
  - `scripts/test/docs-consistency.sh`, `HYPOTHETICAL_REFS` comment block:
    one line stating the exemption applies to references found in any scanned
    standards file, including `INDEX.md` itself.
- Does NOT include:
  - Any behavior change to the docs-consistency checker beyond the comment.
  - New guards, new checks, or changes to standards documents or the README.
  - The older conditional follow-ups (SHA-pinning actions if Dependabot is
    adopted; CI concurrency if CI grows).

## Acceptance Criteria
- exec_bit_guard_fails_loudly_when_git_listing_fails: with `git` stubbed to
  exit non-zero, the `repo_scripts_are_executable` guard reports FAIL naming
  the listing failure (simulation recorded in the PR Evidence); on the real
  tree it still passes.
- runner_side_extraction_deduped: the runner-side extraction in
  `reviewer_defaults_match_across_scripts` pipes through `sort -u`, and a
  sandbox runner copy containing a duplicated default literal no longer
  produces a mismatch (simulation recorded in the PR Evidence); on the real
  tree the guard still passes.
- allowlist_comment_scopes_index: the `HYPOTHETICAL_REFS` comment block in
  `scripts/test/docs-consistency.sh` states the exemption covers all scanned
  standards including `INDEX.md` (grep-verifiable phrase).
- all_suites_green: `bash scripts/test/docs-consistency.test.sh`,
  `bash scripts/test/docs-consistency.sh`, `bash scripts/test/setup.test.sh`,
  and `bash scripts/test/codex-review.test.sh` all pass on the final tree.
