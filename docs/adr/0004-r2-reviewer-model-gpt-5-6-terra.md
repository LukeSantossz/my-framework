# R2 reviewer model: gpt-5.5 → gpt-5.6-terra

The R2 cross-provider gate pinned its default reviewer to `gpt-5.5`. OpenAI's GPT-5.6
launch (GA 2026-07-09) introduced the Sol/Terra/Luna tiers, and a five-diff benchmark
— three seeded defects across the reviewer's mandated categories plus two real, clean
commits — run at `model_reasoning_effort=high` against `gpt-5.5`, `gpt-5.6-terra`, and
`gpt-5.6-sol` showed all three reaching full recall and full precision. We switch the
default to `gpt-5.6-terra`: it matched the more expensive `gpt-5.6-sol` tier on
detection at half the token price ($2.50/$15 vs $5/$30 per 1M), so the cheaper tier is
the correct recurring-gate default. Only the default literal changes; the resolution
chain (env → git config → default) and the `high` effort are untouched. The benchmark
spec that carried the full run, `docs/specs/0009-switch-r2-reviewer-to-gpt-5-6-terra.md`,
was deleted by PR #10 and is not in the tree; it is recoverable from history at
`git show 4164120^:docs/specs/0009-switch-r2-reviewer-to-gpt-5-6-terra.md`, and the
numbers recorded below are the surviving curated record of it.

## Status

Accepted.

## Considered Options

- **`gpt-5.6-terra` (chosen)**: matched `gpt-5.5` and `gpt-5.6-sol` on recall and
  precision in the benchmark, with better severity calibration and lower latency than
  `gpt-5.5`, at half Sol's price — the right cost/detection point for a gate that runs
  on every push.
- **Stay on `gpt-5.5`**: rejected — Terra was equal-or-better on every measured axis
  and gives API-key adopters a defined GPT-5.6 price point.
- **`gpt-5.6-sol`**: rejected — it caught nothing Terra missed (Terra caught more), so
  its 2× price buys no additional detection here.
- **`gpt-5.6` (alias)**: rejected — the bare alias resolves to the Sol tier; pinning
  Terra needs the explicit `gpt-5.6-terra` id.

## Consequences

- Every clone that runs the R2 gate reviews with `gpt-5.6-terra` by default; adopters
  on API-key auth pay Terra's $2.50/$15 per-1M rate. The per-run and per-repo overrides
  (`CODEX_REVIEW_MODEL`, `git config codexreview.model`) are unchanged.
- The benchmark saturated recall (all tiers 3/3), so it establishes that Terra is not
  worse than Sol for this gate, not that it is universally better; a harder defect set
  could reopen the Terra-vs-Sol question.
- `codex_review.md`, `CONTEXT.md`, `scripts/codex-review.sh`, and `scripts/setup.sh`
  state `gpt-5.6-terra` as the default; the parity guard pins the runner and setup
  literals equal. The historical specs that cite `gpt-5.5` remain as durable record of
  the prior default: the archived specs `0001`, `0004`, and `0005` cite `gpt-5.5` as the
  default of their moment, and this ADR is the record of the current one. A reader who
  finds `gpt-5.5` in the archive is reading history, not the live default.

## Numbering history

This decision was originally recorded as ADR `0003` and spec `0009` at commit `3cc2ba0`.
PR #10 (commit `4164120`) deleted both records while leaving the `gpt-5.6-terra` default
live in the scripts, which left the durable archive stating a default the code no longer
ran. The numbers `0003` and `0009` were subsequently reused for the unrelated CRUX
records.

This ADR restores the decision at `0004` rather than reclaiming `0003`, because the CRUX
ADR now at `0003` is already referenced by the README and by the test suite, and moving it
would break those references to buy nothing. Durable numbers are not reused: once a number
identifies a record, it identifies that record permanently, and a retired record is marked
Retired in place rather than deleted.
