# SPEC: chore(scripts): switch the R2 reviewer default to gpt-5.6-terra

## Problem
The R2 cross-provider reviewer default is pinned to `gpt-5.5`; OpenAI's GPT-5.6
launch (GA 2026-07-09; tiers Sol/Terra/Luna) makes `gpt-5.6-terra` available at
equivalent review quality for half the token price of the `gpt-5.6-sol` tier.

## Design Decision
Adopt `gpt-5.6-terra` as the runner's default reviewer model, leaving the
resolution chain (env → git config → default) and the `high` reasoning effort
unchanged. The choice is grounded in a five-diff benchmark — three seeded defects
across the reviewer's mandated categories plus two real, clean commits — run at
`model_reasoning_effort=high` against `gpt-5.5`, `gpt-5.6-terra`, and `gpt-5.6-sol`.
All three reached full recall and full precision, so the cheaper Terra tier matches
the more expensive Sol tier for this gate; only the single default literal moves.

## Alternatives Considered
- **Stay on `gpt-5.5`**: rejected — Terra matched its recall and precision with
  better severity calibration (it flagged the seeded inverted-guard defect as P1,
  not P2) and lower latency in the sample, and it gives API-key adopters a defined
  GPT-5.6 price point.
- **Adopt `gpt-5.6-sol`**: rejected — Sol caught nothing Terra missed (Terra caught
  more, flagging two real commit-process violations), so its 2× price
  ($5/$30 vs $2.50/$15 per 1M) buys no additional detection for a recurring gate.
- **Adopt `gpt-5.6` (the API alias of `gpt-5.6-sol`)**: rejected — the bare alias
  resolves to the Sol tier; pinning Terra requires the explicit `gpt-5.6-terra` id,
  which the local Codex CLI (0.144.1) accepts.

## Scope
- Includes:
  - `scripts/codex-review.sh`: the `config_model` fallback default
    `gpt-5.5` → `gpt-5.6-terra`.
  - `scripts/setup.sh`: both `current_model` prompt/resolution defaults
    `gpt-5.5` → `gpt-5.6-terra`.
  - `scripts/test/codex-review.test.sh` and `scripts/test/setup.test.sh`: the
    pinned default literal used by the dry-run and parity guards.
  - `docs/standards/codex_review.md` and `CONTEXT.md`: the stated default model
    and the worked example.
  - `README.md`: the Engineering Decisions row indexing ADR 0003, and correcting
    the stale `Codex CLI` prerequisite version to the `0.144.1` the gate is now
    verified against (folded in at the Developer's in-task request; see the R2
    adjudication note below).
  - `docs/specs/0009-assets/`: the benchmark methodology record and per-cell
    scoring manifest, so the reported numbers are auditable.
  - `docs/specs/0009-*.md` and `docs/adr/0003-*.md`: the decision record.
- Does NOT include:
  - Any change to the resolution precedence (env → git config → default) or to the
    `high` reasoning effort.
  - Making the gate blocking, or any other behavior change.
  - The historical specs 0001/0004/0005/0006/0007 that cite `gpt-5.5` or the older
    `Codex CLI 0.132.0`/`0.140.0` as then-current (durable archive; not rewritten).

## Acceptance Criteria
- review_model_default_when_unset_is_terra: with no env or git-config override,
  `CODEX_REVIEW_DRYRUN=1 bash scripts/codex-review.sh` prints
  `codex review --base main -c model="gpt-5.6-terra" -c model_reasoning_effort="high"`.
- setup_interactive_default_prompt_is_terra: `scripts/setup.sh --interactive` with
  empty answers reports `reviewer=gpt-5.6-terra` in its summary.
- reviewer_defaults_match_across_scripts_holds: the runner-side and setup-side
  default literals remain equal (the parity guard passes) at `gpt-5.6-terra`.
- readme_codex_prerequisite_is_current: `README.md` states the `Codex CLI`
  prerequisite as `0.144.1`, and no living (non-archive) file leaves a `0.132.0`
  prerequisite.
- all_suites_green: `bash scripts/test/codex-review.test.sh`,
  `bash scripts/test/setup.test.sh`, `bash scripts/test/docs-consistency.test.sh`,
  and `bash scripts/test/docs-consistency.sh` pass on the final tree.

## Reproducibility
Benchmark command, per cell:
`codex review --commit <SHA> -c model="<model>" -c model_reasoning_effort="high"`
for each of `{gpt-5.5, gpt-5.6-terra, gpt-5.6-sol}` over five diffs — three seeded
defects (an inverted base-branch guard; the non-existent flags
`codex review --format=json` and `git rev-parse --branch-name`; `eval` on an
unsanitized argument), each a single-file commit, and two real commits `c3a891b`
and `5ff245c`. Codex CLI 0.144.1; auth mode ChatGPT, so per-token pricing is not
billed locally ($2.50/$15 and $5/$30 per 1M are OpenAI's published Terra/Sol rates,
supplied by the Developer). The per-cell scoring — expected outcome, reviewer
verdict, and latency — is tracked in `docs/specs/0009-assets/manifest.csv`, with the
methodology in `docs/specs/0009-assets/README.md`; per-cell review transcripts were
generated locally. Observed result (n=5, one run per cell — directional, not
statistically significant):

| Model | Seeded recall | Real-clean precision | Latency (5 cells) |
|---|---|---|---|
| gpt-5.5 | 3/3 | 2/2 | 524s |
| gpt-5.6-terra | 3/3 | 2/2 | 410s |
| gpt-5.6-sol | 3/3 | 2/2 | 310s |

All three model ids were validated live against CLI 0.144.1 before the run.

## Risks and Assumptions
- Assumption: `gpt-5.6-terra` is a stable, generally-available model id accepted by
  `codex review -c model=` (verified live against CLI 0.144.1 on 2026-07-10).
- Assumption: the GPT-5.6 pricing and the Sol/Terra/Luna tiers are as supplied by
  the Developer; the local ChatGPT-auth benchmark measures quality and latency, not
  dollar cost.
- What would invalidate this spec: OpenAI renaming or retiring the `gpt-5.6-terra`
  id, or a harder defect set showing Sol catches defects Terra misses — the
  benchmark saturated recall and cannot rank the tiers above the floor it
  established.

## R2 Adjudication
The R2 gate (`gpt-5.6-terra`) reviewed this branch across two passes; every finding
was addressed or justified, never silently dropped.

First pass — two P2 findings, both accepted:
- README prerequisite flagged as scope creep — the version line was in this spec's
  "Does NOT include" list. Resolved by folding the Developer-approved correction
  into Scope > Includes, not by reverting it.
- Benchmark not reproducible from the repository — the evidence lived in an untracked
  scratch path and the seeded commit SHAs were ephemeral. Resolved by recording the
  benchmark under `docs/specs/0009-assets/`.

Second pass — on the newly tracked benchmark assets:
- Robustness findings on the committed harness (it force-deleted a same-named branch,
  could clobber untracked paths, and did not fail on a failed seed commit), plus a
  request for an auditable scoring record. Resolved by removing the harness rather
  than hardening a fragile research script inside a standards repo, and recording the
  benchmark as a methodology note plus a per-cell scoring manifest (expected outcome,
  reviewer verdict, latency).
- Claim that the spec was committed after the test and implementation commits:
  rejected as verified false — the spec was added in `3cc2ba0`, before the test
  (`83db7b6`) and implementation (`914b760`) commits.
