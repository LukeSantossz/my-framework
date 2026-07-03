# SPEC: chore(review): add Codex CLI pre-push gate for R2 cross-provider review

## Problem
The standards name a "Codex pre-commit gate" as the R2 cross-provider reviewer, but
nothing in the repository invokes Codex or gives it the project's conventions, so the
R2 layer never actually runs.

## Design Decision
Wire Codex CLI as the R2 layer through versioned pieces: (1) a project `AGENTS.md` at
the repo root that points Codex at `docs/standards/INDEX.md`, so its review applies the
same conventions Claude follows; (2) a versioned `pre-push` git hook under `.githooks/`
(activated locally via `core.hooksPath`) that runs a small runner script; (3) a runner
`scripts/codex-review.sh` that builds and invokes `codex review --base main` with the
reviewer model pinned to `gpt-5.5` at `model_reasoning_effort=high`, with its guard logic
kept testable; (4) `docs/standards/codex_review.md` documenting when the gate runs, the
exact command, how to record the result in the PR Self-Review Checklist, and the bypass
path, plus a pointer from `ai_guidelines.md` R2 and the INDEX files.

## Alternatives Considered
- Documented manual command only, no hook: rejected — relies on the developer remembering
  to run it; the user chose enforcement at push time.
- Editing `.git/hooks/pre-push` directly: rejected — `.git/` is not versioned, so the gate
  could not be shared or reviewed. `.githooks/` plus `core.hooksPath` keeps it in the tree.
- Inheriting the reviewer model from `~/.codex/config.toml`: rejected — the user chose to
  pin `gpt-5.5`/high for reproducible, explicitly cross-provider reviews.
- Hard-blocking the push on any Codex finding by default: rejected — `codex review` is not
  guaranteed to expose a reliable pass/fail exit code, and `ai_guidelines.md` treats R2
  findings as advisory-but-must-address; the hook surfaces the review and a blocking mode
  is opt-in.

## Scope
- Includes:
  - `AGENTS.md` at the repo root pointing Codex to the standards and its reviewer duties.
  - `.githooks/pre-push` hook plus `scripts/codex-review.sh` runner.
  - `docs/standards/codex_review.md`, a pointer from `ai_guidelines.md` R2, and INDEX entries.
  - A one-time local activation step (`git config core.hooksPath .githooks`), documented.
- Does NOT include:
  - Changing the Author model or any `CLAUDE.md`/Claude behavior.
  - Wiring R3 automated PR review (e.g. CodeRabbit).
  - CI integration or running Codex inside GitHub Actions.
  - Committing or pushing anything (per the standing no-auto-commit instruction).
  - Modifying the user's global `~/.codex/config.toml`.

## Acceptance Criteria
Phrased as test outcomes; each becomes a failing test first.

- skips_review_when_codex_binary_absent: the runner exits 0 and prints a clear
  "Codex not installed, skipping R2" message when `codex` is not on PATH.
- skips_review_when_pushing_base_branch: the runner exits 0 without invoking Codex when the
  current branch is the base branch (nothing to review against itself).
- builds_review_command_with_pinned_model: in dry-run mode the runner prints exactly
  `codex review --base main -c model="gpt-5.5" -c model_reasoning_effort="high"`.
- bypass_env_skips_gate: setting `SKIP_CODEX_REVIEW=1` makes the runner exit 0 without
  invoking Codex.
- agents_file_points_to_standards: `AGENTS.md` references `docs/standards/INDEX.md` and the
  precedence order in `code_conventions.md`.

## Reproducibility
- Codex: codex-cli 0.140.0 (`codex --version`). Reviewer model `gpt-5.5`,
  `model_reasoning_effort=high`.
- Activate locally: `git config core.hooksPath .githooks`.
- Run the gate manually: `bash scripts/codex-review.sh`; print the command without running
  it: `CODEX_REVIEW_DRYRUN=1 bash scripts/codex-review.sh`.
- Tests: `bash scripts/test/codex-review.test.sh`, exercising the criteria above with a
  `codex` stub placed first on PATH.
- Platform: Windows 11, Git Bash (POSIX sh).

## Risks and Assumptions
- Assumption: `codex review --base <branch>` runs non-interactively and does not block on
  stdin (per `codex review --help`). If it prompts, the hook would hang and must gain a
  timeout.
- Assumption: `codex review` exit code is not a reliable pass/fail signal; treated as
  advisory. If testing shows a stable non-zero-on-findings contract, blocking mode could
  become the default.
- Assumption: pre-push hooks run under the user's Git on Windows via Git Bash (POSIX sh).
- Risk: Codex auth or quota failure at push time. Mitigated by treating runner failure as
  non-blocking (exit 0 with a warning) unless blocking mode is enabled.
- Assumption: the base branch is `main` (matches this repo); parameterized via
  `CODEX_REVIEW_BASE`, defaulting to `main`.
