# SPEC 0009 benchmark record

Evidence for [`../0009-switch-r2-reviewer-to-gpt-5-6-terra.md`](../0009-switch-r2-reviewer-to-gpt-5-6-terra.md):
the benchmark that justified switching the R2 reviewer default from `gpt-5.5` to
`gpt-5.6-terra`. This is a methodology and results record, not runnable code — the
one-off harness was intentionally not committed (a branch-juggling research script
does not belong in a standards repo; see the spec's R2 Adjudication).

## What was measured

Five diffs were reviewed with each of `gpt-5.5`, `gpt-5.6-terra`, and `gpt-5.6-sol`
via `codex review --commit <SHA> -c model="<model>" -c model_reasoning_effort="high"`
(Codex CLI 0.144.1). Three diffs each carried one **intentional seeded defect**; two
were real, clean pre-merged commits, used to check for false positives.

| Diff | Kind | Planted defect (expected finding) |
|---|---|---|
| seed1 | correctness | a base-branch guard whose condition is inverted, so it skips feature branches and runs only on the base |
| seed2 | invented symbol | `codex review --format=json` and `git rev-parse --branch-name` — neither flag exists |
| seed3 | security | `eval` on an unsanitized positional argument (command injection) |
| `c3a891b` | real-clean | none (exec-bit guard hardening) |
| `5ff245c` | real-clean | none (`sort -u` dedupe) |

Each seeded diff was a single-file commit; the two real diffs are the named commits.

## Result (auditable scoring)

`manifest.csv` records one row per (model, diff) cell: the expected outcome, the
reviewer's actual verdict (`caught` the seeded defect / reported the clean diff as
`clean`), whether that matched (`pass`), and latency. Summary:

| Model | Seeded recall | Real-clean precision | Latency (5 cells) |
|---|---|---|---|
| gpt-5.5 | 3/3 | 2/2 | 524s |
| gpt-5.6-terra | 3/3 | 2/2 | 410s |
| gpt-5.6-sol | 3/3 | 2/2 | 310s |

All three models caught every seeded defect and reported every clean diff as clean.
Terra matched the more expensive Sol tier, so it was adopted as the cheaper default.

## Caveats

Sample is n=5, one run per cell — directional, not statistically significant. All
three tiers saturated recall, so the benchmark establishes that Terra is not worse
than Sol for this gate, not that it is universally better. Auth mode was ChatGPT, so
per-token dollar cost was not billed locally; the $2.50/$15 and $5/$30 per-1M rates
are OpenAI's published Terra/Sol prices supplied by the Developer. Each cell's actual
reviewer verdict is committed in [`verdicts.md`](verdicts.md); `manifest.csv` is the
machine-readable scoring record. The full tool-trace transcripts were not retained,
and because an LLM reviewer is stochastic the exact verdicts and latencies are not
byte-for-byte reproducible.

## Reproduce

The exact seeded file contents are in [`seeds.md`](seeds.md). Commit each as a single
new file under `scripts/bench/`, then, for each of the three models and each of the
five commit SHAs (the three seeded commits plus the real `c3a891b` and `5ff245c`):

```sh
codex review --commit <SHA> -c model="<model>" -c model_reasoning_effort="high"
```

Score each cell by whether the reviewer flagged the planted defect (seeded diffs) or
returned no actionable finding (real diffs).
