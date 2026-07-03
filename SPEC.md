# SPEC: chore: harden scripts and checks with the deferred review follow-ups

## Problem
Seven small hardening items adjudicated as "defer as follow-up" across the
reviews of PR #3 and PR #4 remain open, leaving known-but-unpinned contracts
(empty-string env semantics, duplicated default literals, unchecked config
writes, an untested error path, INDEX-only reference checking, an untested
override, inconsistent test-file exec modes) in the framework's own tooling.

## Design Decision
Close the whole batch in one maintenance cycle, implemented in parallel by
three subagents over disjoint file clusters (codex-review, setup,
docs-consistency), each in an isolated git worktree with its own TDD commits,
merged back into this branch; the cross-cutting exec-mode normalization lands
serially after the merge. No behavior of the R2 gate, the bootstrap's happy
path, or any standard's meaning changes — every item is a test, a guard, an
error check, or a checker extension.

## Alternatives Considered
- One serial fix wave (like previous cycles): rejected — the items are
  pre-adjudicated and independent; the user explicitly asked for parallel
  subagent execution, and disjoint clusters make it conflict-free.
- Extracting shared default literals into a sourced config file instead of a
  guard test: rejected — an unrequested abstraction for two bash scripts; the
  repo's established pattern for drift risk is a guard test (LABEL_SPECS).
- A new repo-hygiene test suite for the exec-mode guard: rejected — a fourth
  suite would require a CI edit for one assertion; the guard lives in
  docs-consistency.test.sh, the closest thing to a repo-invariants suite.

## Scope
- Includes (cluster A — codex-review):
  - Test pinning that an empty-string `CODEX_REVIEW_MODEL`/`CODEX_REVIEW_EFFORT`
    behaves as unset (falls through to git config/default), plus an
    "if non-empty" qualifier on the two env-var lines in
    `docs/standards/codex_review.md`.
  - Guard test pinning the `gpt-5.5`/`high` default literals in
    `scripts/setup.sh` (prompt/summary fallbacks) to those in
    `scripts/codex-review.sh` (resolution fallbacks), extracted mechanically
    from both scripts.
- Includes (cluster B — setup):
  - Error check on the two interactive `git config --local codexreview.*`
    writes: a failed write logs and exits 1 (the hooksPath/label-create
    pattern), tested via a stub `git` that fails only that subcommand.
  - Test pinning the existing exit-1 path when `setup.sh` runs outside a git
    repository.
- Includes (cluster C — docs-consistency):
  - Extend the reverse-reference check to validate `.md` references inside
    every file in the standards dir, not only `INDEX.md` (same token rules and
    resolution roots as today; failure message names the referencing file).
  - Test pinning that the `DOCS_DIR` override is honored (a violation planted
    in an alternate docs dir is detected while the default location is clean).
- Includes (serial, after merge):
  - Normalize the executable bit (100755) on all `scripts/**/*.sh` and
    `.githooks/pre-push` in the git index, plus a guard test in
    `docs-consistency.test.sh` asserting it stays that way.
- Does NOT include:
  - Any change to R2 gate advisory behavior, resolution precedence, or
    defaults.
  - Any change to the non-interactive setup happy path.
  - The larger backlog items (CLAUDE.md conflicts, durable specs, Issue Model,
    README, CRURA, badges).
  - New test suites or CI workflow changes.
  - The user's `prompt.md` (untracked personal file).

## Acceptance Criteria
- review_model_empty_env_treated_as_unset: with `CODEX_REVIEW_MODEL=""` and
  `CODEX_REVIEW_EFFORT=""` exported and repo-local `codexreview.*` set,
  dry-run prints the git-config values (empty env never yields an empty
  `-c model=""`).
- codex_review_doc_qualifies_env_override: the two env-var bullets in
  `codex_review.md` carry the "if non-empty" qualifier (guard grep).
- reviewer_defaults_match_across_scripts: a guard test extracts the model and
  effort default literals from both scripts and fails when they differ.
- setup_fails_when_reviewer_config_write_fails: `--interactive` with a
  non-empty answer and a `git` that fails the `config --local codexreview.*`
  write exits non-zero with a message, and does not report the choice as
  persisted in the summary.
- setup_exits_nonzero_outside_git_repo: running `setup.sh` in a directory
  with no repository exits 1 with the "not inside a git repository" message.
- docs_consistency_detects_refs_in_standards_bodies: a standards file (not
  INDEX.md) referencing a missing `.md` fails the check, naming the
  referencing file; the current tree still passes.
- docs_consistency_honors_docs_dir_override: with `DOCS_DIR` pointing at an
  alternate tree containing a violation, the check fails; unset, the default
  tree passes.
- repo_scripts_are_executable: a guard test lists `scripts/**/*.sh` and
  `.githooks/pre-push` from the git index and fails if any lacks mode 100755.

## Reproducibility
- `bash scripts/test/codex-review.test.sh` — expected: all pass, 0 failed.
- `bash scripts/test/setup.test.sh` — expected: all pass, 0 failed.
- `bash scripts/test/docs-consistency.test.sh && bash scripts/test/docs-consistency.sh`
  — expected: all pass; `all checks passed.`
- Versions: bash (Git for Windows), git ≥ 2.40, gh ≥ 2.40, Codex CLI 0.132.0.
  No randomness involved.

## Risks and Assumptions
- Assumes the three clusters stay strictly disjoint by file; the plan assigns
  files explicitly and the merge is expected to be conflict-free.
- Assumes extending reverse-reference checking to all standards files passes
  on the current tree (the PR #4 reviewer verified every body reference
  resolves today); a false-positive token in prose would surface as a check
  failure and be fixed by tightening the token rules, not by skipping files.
- Assumes `git update-index --chmod=+x` semantics on Windows (mode lives in
  the index, not the filesystem) — the guard reads the index, so it is
  platform-stable.
- Invalidated if a future cycle extracts shared script config; the guard test
  for default literals would then be replaced by the shared source.
