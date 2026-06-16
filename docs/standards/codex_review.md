# Codex Review (R2 Gate)

Operational definition of the R2 cross-provider review from `ai_guidelines.md`. It wires
Codex CLI as the Reviewer model (provider different from the Author) and runs it as a
`pre-push` gate. R1 (internal Superpowers review) and R3 (automated PR review) are
unchanged; this document only makes R2 concrete.

## Roles

- Author: Claude (Anthropic), per `CLAUDE.md`.
- Reviewer: Codex CLI with `model=gpt-5.5`, `model_reasoning_effort=high` (OpenAI). The
  reviewer model is a different provider than the Author, which is what satisfies R2.

Codex reads `AGENTS.md` at the repo root for its role and the binding standards.

## Activation (one-time, local)

The hook lives in the versioned `.githooks/` directory, not in `.git/`. Point Git at it:

```sh
git config core.hooksPath .githooks
```

This is a local setting and is not committed. Requires `codex` on `PATH` and an
authenticated session (`codex login`). Verify the toolchain with `codex doctor`.

## What runs

On `git push`, `.githooks/pre-push` calls `scripts/codex-review.sh`, which runs:

```sh
codex review --base main -c model="gpt-5.5" -c model_reasoning_effort="high"
```

It reviews the current branch against `main`. The review is **advisory**: findings are
printed but the push is not blocked by default, matching `ai_guidelines.md` ("A Reviewer
finding is advisory, not binding, but an unresolved finding must be addressed or
justified, never silently dropped").

## Behavior

- On the base branch (nothing to review against itself): skipped, push proceeds.
- Codex not installed: skipped with a message, push proceeds (R2 did not run).
- Codex exits non-zero (findings, auth, or quota): advisory message, push proceeds —
  unless blocking mode is on.

## Environment variables

- `SKIP_CODEX_REVIEW=1`: skip the gate for this push.
- `CODEX_REVIEW_BLOCKING=1`: block the push when `codex review` exits non-zero.
- `CODEX_REVIEW_BASE=<branch>`: base branch to review against (default `main`).
- `CODEX_REVIEW_DRYRUN=1`: print the command without running Codex.
- `CODEX_BIN=<path>`: override the Codex binary (testing).

## Bypass

Use `SKIP_CODEX_REVIEW=1 git push`, or Git's own `git push --no-verify`. A bypass means R2
did not run; record that in the PR per the next section.

## Recording in the PR

In the PR Review Checklist (`github.md`), name the models for the review layers:
Author `claude-opus-4-8`, Reviewer `codex / gpt-5.5`. If R2 did not run (Codex absent,
skipped, or bypassed), note it and why, as the checklist requires.

## Manual run

```sh
bash scripts/codex-review.sh                 # run the gate now
CODEX_REVIEW_DRYRUN=1 bash scripts/codex-review.sh   # print the command only
```

Tests for the runner's guard logic: `bash scripts/test/codex-review.test.sh`.
