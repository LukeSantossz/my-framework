# SPEC 0009 benchmark assets

Evidence for [`../0009-switch-r2-reviewer-to-gpt-5-6-terra.md`](../0009-switch-r2-reviewer-to-gpt-5-6-terra.md):
the benchmark that justified switching the R2 reviewer default from `gpt-5.5` to
`gpt-5.6-terra`.

## Files

- `run_benchmark.sh` — the harness. Reviews five diffs (three seeded defects it
  regenerates on each run, two real commits) with each of `gpt-5.5`,
  `gpt-5.6-terra`, and `gpt-5.6-sol` at `model_reasoning_effort=high`, and writes a
  `manifest.csv`. Requires an authenticated Codex CLI (verified on 0.144.1).
- `manifest.csv` — the recorded run: one row per (model, diff) cell with its
  latency and exit code. The seeded rows carry `SEEDED` in the `sha` column because
  the harness recreates those commits on each run, so their SHAs are not stable; the
  two real diffs are the fixed commits `c3a891b` and `5ff245c`.

## Reproduce

```sh
bash docs/specs/0009-assets/run_benchmark.sh   # writes manifest + per-cell logs to $BENCH_OUT
```

Per-cell review transcripts (the reviewers' verdicts) are written to `$BENCH_OUT`
alongside the manifest; they were not committed. Scoring (recall on the seeded
defects, precision on the clean real diffs) is done by reading each cell's verdict.

## Result

All three models reached full recall (3/3 seeded defects) and full precision (2/2
clean real diffs). Terra matched the more expensive Sol tier, so it was adopted as
the cheaper default. Sample is n=5, one run per cell — directional, not
statistically significant. See the spec for the full table and caveats.
