# SPEC 0009 benchmark — reviewer outputs

The actual verdict each model returned for each of the 15 (model, diff) cells,
paired with the seed patches in [`seeds.md`](seeds.md). Together these are the review
outputs backing the 3/3 recall and 2/2 precision recorded in
[`manifest.csv`](manifest.csv). Each entry is condensed to the review's summary and
headline finding; the full tool-trace output was not retained. `caught` = flagged the
planted defect; `clean` = returned no actionable finding on the clean diff.

## seed1 — correctness (inverted guard) — expected: caught

- **gpt-5.5** — caught (P2): "Fix inverted base-branch guard … when running from any feature branch … the review never runs where it is supposed to."
- **gpt-5.6-terra** — caught (P1): "Reverse the base-branch condition … this is the opposite of the stated guard and prevents reviews for every branch that actually needs one."
- **gpt-5.6-sol** — caught (P1): "The branch guard is inverted … effectively disables review whenever there are changes to review."

## seed2 — invented symbol (`--format=json`, `git rev-parse --branch-name`) — expected: caught

- **gpt-5.5** — caught (P2 ×2): "computes the output filename from an unsupported Git invocation and calls Codex with an unsupported option … `git rev-parse --branch-name` … `codex review` has no `--format` option."
- **gpt-5.6-terra** — caught (P1 ×2): "uses unsupported Git and Codex CLI options, and it can report success after failure" (also raised the benchmark commit's own test-first / commit-type violations).
- **gpt-5.6-sol** — caught (P1 + P2): "`codex review` does not support `--format=json` … `git rev-parse` has no `--branch-name` option."

## seed3 — security (eval command injection) — expected: caught

- **gpt-5.5** — caught (P1): "introduces command injection through `eval` on caller-controlled input, so the patch is not safe as written."
- **gpt-5.6-terra** — caught (P1): "allows arbitrary command execution through its caller-provided hook argument … Remove caller-controlled eval dispatch."
- **gpt-5.6-sol** — caught (P1): "passes caller-controlled input to `eval`, introducing direct shell command injection."

## real c3a891b (exec-bit guard hardening) — expected: clean

- **gpt-5.5** — clean: "No introduced correctness, security, scope, or convention issues were found in the reviewed commit."
- **gpt-5.6-terra** — clean: "correctly turns a failed `git ls-files` invocation into a recorded failing guard while preserving the existing executable-mode validation path."
- **gpt-5.6-sol** — clean: "correctly preserves the exit status of `git ls-files`, explicitly reports listing failures … No actionable defects were found."

## real 5ff245c (`sort -u` dedupe) — expected: clean

- **gpt-5.5** — clean: "No actionable … issues were identified … deduplicating runner-side default extraction in the parity guard."
- **gpt-5.6-terra** — clean: "applies the same unique-value normalization as the setup-side extraction … No correctness, scope, security, or convention issues were identified."
- **gpt-5.6-sol** — clean: "correctly deduplicates runner-side extracted defaults with `sort -u` … No functional regressions or actionable defects were found."

## Tally

Every model: 3/3 seeded defects caught, 2/2 clean diffs reported clean. Terra matched
the more expensive Sol tier, so Terra was adopted as the cheaper default.
