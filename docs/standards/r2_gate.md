# R2 Gate

Operational definition of the R2 cross-provider review from `ai_guidelines.md`. It runs
a chain of reviewer backends as a `pre-push` gate, taking the first one that is actually
available, and reports which one reviewed. R1 (internal Superpowers review) is unchanged;
this document makes R2 concrete and records the repository's R3 wiring. On this
repository, R3 is CodeRabbit (wired via its GitHub app);
its findings are adjudicated in the PR discussion like any reviewer finding.

## Roles

- Author: the model conducting the session (Anthropic Claude family, per `CLAUDE.md`).
  The concrete model varies by session and is recorded per PR.
- Reviewer: whichever backend in the chain actually ran. The requirement is the role,
  not a name: the Reviewer must be a different provider than the Author. The shipped
  default chain is `codex` alone, so a repository that configures nothing behaves as it
  always has.

The Reviewer reads `AGENTS.md` at the repository root for its role and the binding
standards — an agentic backend finds it there itself, and a non-agentic one has it sent
in the request.

## Why a chain

A single hard-wired reviewer means R2 silently stops existing whenever that one vendor's
account is unavailable. A quota that resets days out, an expired key, an offline laptop:
each turns the gate into a no-op that still reports success. The chain exists so an
unavailable reviewer is a fallback rather than a hole.

Falling back is allowed. Falling back quietly is not — a weaker reviewer is not the same
review, so the runner names the backend and model that ran, and that name belongs in the
PR's review-layers record.

## The adapter contract

Each backend is a script at `scripts/reviewers/<name>.sh`. The runner resolves the
settings, exports them, and invokes the adapter. Three exit codes:

| Exit | Meaning | Chain |
|---|---|---|
| `0` | Reviewed; nothing blocking to report | stops |
| `10` | **Unavailable** — not installed, not authenticated, out of quota, endpoint unreachable | advances |
| other | Reviewed; findings, or a failure mid-review | stops (advisory unless blocking) |

The distinction between *unavailable* and *reviewed with findings* cannot be recovered
from outside the adapter, which is why each adapter classifies its own tool's failures.
Where that means matching a vendor's error text, the matching lives in the adapter that
owns that vendor and nowhere else.

The runner passes `R2_BASE`, `R2_BRANCH`, `R2_RESOLVED_MODEL` and `R2_RESOLVED_EFFORT`
in the environment. Under `R2_DRYRUN=1` an adapter prints what it would do and runs
nothing.

## Shipped backends

### `codex`

Codex CLI (OpenAI), agentic: it explores the repository itself. Runs

```sh
codex review --base <base> -c model="<model>" -c model_reasoning_effort="<effort>"
```

Default model `gpt-5.6-terra` with effort `high`. Requires `codex` on `PATH` and an
authenticated session (`codex login`); verify the toolchain with `codex doctor`.

### `gemini`

Gemini CLI (Google), also agentic. Receives a prompt carrying the Reviewer role and
`AGENTS.md`, and reviews the branch against the base. The prompt flag is configurable
(`r2.gemini.promptFlag`, default `--prompt`) because this adapter is exercised against a
stub rather than the real CLI in this repository's tests: an upstream flag change must be
fixable by configuration, not by a framework release.

### `openai`

Any endpoint speaking the OpenAI chat-completions shape. One adapter therefore reaches
Ollama, LM Studio, llama.cpp, vLLM, DeepSeek, Groq, OpenRouter and Together — local and
hosted alike — and it is the only path to a fully local reviewer.

Unlike the agentic backends it sees only what it is sent: the branch diff, plus
`AGENTS.md` for the role and the standards. It cannot explore the repository, so its
review is a different shape, not merely a weaker grade. The request puts the stable
prefix first and the volatile diff last, because providers on this shape bill cached
prompt tokens at a fraction of fresh ones and a pre-push gate re-sends that prefix on
every push.

The budget is total elapsed time, not socket inactivity: a reasoning model sends nothing
while it thinks, so an inactivity timeout never fires and the request runs until something
else drops it. Exceeding the budget is unavailability, so the chain advances rather than
holding the push. A reasoning model's latency is volatile enough that the diff cap makes
the budget reachable, not certain.

Requires Node, which the adapter uses for both the request and the JSON. A diff larger
than `r2.openai.maxDiffBytes` is truncated and the truncation is reported; a response
cut off by the output limit is reported as incomplete. The model's `reasoning_content`,
when present, is never reported as findings.

## Activation (one-time, local)

The hook lives in the versioned `.githooks/` directory, not in `.git/`. Point Git at it:

```sh
git config core.hooksPath .githooks
```

This is a local setting and is not committed. Alternatively run `bash scripts/setup.sh`:
it applies this setting, reports the toolchain state, and creates any missing triage
labels. It is idempotent and safe to re-run.

## Configuration

Settings resolve through git's own scope cascade, so a machine-wide default is possible
and a repository can still override it — the authority order `code_conventions.md`
states:

```
environment  >  git config --local  >  git config --global  >  built-in default
```

| Key | Meaning | Default |
|---|---|---|
| `r2.backends` | Ordered, comma-separated chain | `codex` |
| `r2.base` | Branch to review against | `main` |
| `r2.model`, `r2.effort` | Applied to every backend | per backend |
| `r2.<backend>.model`, `r2.<backend>.effort` | Override for one backend | — |
| `r2.openai.endpoint` | Base URL, without `/chat/completions` | — |
| `r2.openai.apiKeyEnv` | **Name** of the environment variable holding the key | — |
| `r2.openai.maxDiffBytes` | Diff size limit before truncation | `30000` |
| `r2.openai.timeoutSeconds` | Total wall-clock budget for one review | `240` |
| `r2.gemini.promptFlag` | Prompt flag for the Gemini CLI | `--prompt` |

Write the machine-global layer with:

```sh
bash scripts/setup.sh --reviewer
```

`--reviewer` writes `--global`; `--interactive` writes `--local`. Each flag owns one
scope, so it is always clear which layer an answer landed in.

**Secrets are never stored here.** `r2.openai.apiKeyEnv` holds the *name* of the
variable carrying the key, and `setup.sh --reviewer` refuses a value that is not a valid
environment variable name. `git config --list` output ends up in bug reports and
screenshots; a key does not belong in it. A `localhost` endpoint needs no key at all.

## Environment variables

- `SKIP_R2_REVIEW=1`: skip the gate for this push.
- `R2_BLOCKING=1`: block the push when the reviewing backend exits non-zero.
- `R2_BASE=<branch>`: base branch to review against (default `main`).
- `R2_BACKENDS=<list>`: the chain for this run.
- `R2_MODEL=<model>` / `R2_EFFORT=<effort>`: applied to whichever backend runs.
- `R2_DRYRUN=1`: print what each backend would do, without running any.
- `R2_REVIEWERS_DIR=<path>`: override the adapter directory (testing).
- `CODEX_BIN`, `GEMINI_BIN`, `NODE_BIN`: override a binary (testing).

Legacy names from before the seam keep working, so a clone configured earlier resolves
exactly as it did:

- `SKIP_CODEX_REVIEW=1`, `CODEX_REVIEW_BLOCKING=1`, `CODEX_REVIEW_BASE=<branch>`, `CODEX_REVIEW_DRYRUN=1`.
- `CODEX_REVIEW_MODEL=<model>`: reviewer model for this run, for the `codex` backend only (highest precedence, if non-empty; an empty value is treated as unset).
- `CODEX_REVIEW_EFFORT=<effort>`: reviewer reasoning effort for this run, for the `codex` backend only (highest precedence, if non-empty; an empty value is treated as unset).
- `git config codexreview.model` / `codexreview.effort`: the repo-local persisted keys, for the `codex` backend only.

Those last two keys stay repo-local by design: promoting them to global scope now would
make a machine-wide value appear in repositories that never asked for it.

## Behavior

- On the base branch (nothing to review against itself): skipped, push proceeds.
- A backend that is unavailable: the chain advances to the next one.
- No backend available: R2 did not run, the push proceeds, and the run says so — even
  under `R2_BLOCKING`, because a reviewer that never ran is not a finding and an expired
  quota must not lock the repository.
- A backend that reviewed and exited non-zero: advisory message, push proceeds — unless
  blocking mode is on.

The review is **advisory** by default, matching `ai_guidelines.md` ("A Reviewer finding
is advisory, not binding, but an unresolved finding must be addressed or justified,
never silently dropped").

## Bypass

Use `SKIP_R2_REVIEW=1 git push`, or Git's own `git push --no-verify`. A bypass means R2
did not run; record that in the PR per the next section.

## Recording in the PR

In the PR Review Checklist (`github.md`), name the backend that reviewed and the
concrete models on both sides (for example: Author `<author-model>`, Reviewer
`codex / <reviewer-model>`), including any override in effect. The runner prints this as
`Reviewed by: <backend> / <model>` for exactly that purpose. If R2 did not run — no
backend available, skipped, or bypassed — note it and why, as the checklist requires.

Naming the backend is not bookkeeping: a chain that fell through to a weaker reviewer
produced a weaker review, and the human adjudicating the PR is the one who decides what
that is worth.

## Manual run

```sh
bash scripts/r2-review.sh                 # run the gate now
R2_DRYRUN=1 bash scripts/r2-review.sh     # print the chain, run nothing
```

Tests for the runner, the chain and the adapters: `bash scripts/test/r2-review.test.sh`.
