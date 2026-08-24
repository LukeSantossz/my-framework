# SPEC: feat(eval): measure each backend against a corpus of diffs with planted defects

## Problem

Every change to a review prompt, a model choice or a backend chain is currently justified
by impression, because nothing measures whether a configuration finds the defects it is
supposed to find.

## Design Decision

Build a corpus of diffs carrying known, deliberately planted defects, run a configured
role over it, and report hit rate per backend.

The metric is **hit rate** — of the planted defects, how many were found — with false
positives reported separately rather than folded into a single score. For a gate whose
output a human triages, those two numbers drive different decisions: a low hit rate means
the gate misses things, a high false-positive rate means people stop reading it.

Every request is sent at `temperature = 0`. Without it a score varies between runs and
stops being comparable, which would make the corpus decorative. Every result records the
model id, the date and the corpus version, because a number without those cannot be
compared to another number.

Defects are planted across the five categories `AGENTS.md` enumerates, so the corpus can
show that a backend is strong on correctness and blind to scope creep — the shape of
answer that changes a chain's order.

Grading is by planted-defect matching, not by a model judging a model. A judge would
introduce self-bias, length bias and position bias into the very measurement meant to
detect quality, and the ground truth here is already known because the defects were
planted deliberately. Where a finding's text must be matched to a planted defect, the
matching rule is explicit and reported, so a generous rule cannot inflate a score
silently.

Results are never presented as vendor benchmarks. They measure this framework's prompts
and this corpus, and self-reported numbers are not comparable to independent evaluation.

## Alternatives Considered

- **Use a model as judge for grading.** Rejected. The biases a judge carries are exactly
  the ones the measurement exists to expose, and ground truth is already available.
- **Report one combined quality score.** Rejected. It merges two failure modes that call
  for opposite responses, and it lets a backend that flags everything look strong.
- **Harvest defects from real fixed bugs in the history.** Rejected for this slice.
  It is more realistic and much better evidence, but the diffs are entangled with
  unrelated changes and the "defect" is defined by whatever the fix touched, which is not
  a clean label. Worth revisiting once the planted corpus establishes the harness.
- **Run the corpus in CI on every push.** Rejected. It costs real tokens per run for a
  number that changes only when a prompt, model or chain changes; it runs on demand and
  before a configuration change lands.
- **Score with a rubric from 1 to 100.** Rejected. Fine-grained scores from a model are
  not reproducible; the corpus avoids the problem entirely by counting known defects.

## Scope

- Includes:
  - The corpus format: a diff, a manifest of planted defects with category and location,
    and a corpus version.
  - An initial corpus covering all five finding categories, including clean diffs with no
    planted defect at all so false positives are measurable.
  - `mf eval --role <role> [--backend <name>]`, reporting hit rate and false-positive
    count per backend and per category.
  - The explicit finding-to-defect matching rule, printed with the results.
  - Enforced `temperature = 0` and recorded model id, date and corpus version.
  - Tests of the harness itself against fake backends with known outputs, written first.

- Does NOT include:
  - A model judging outputs.
  - Publishing results as a comparison between vendors.
  - Defects harvested from real history.
  - Running in CI.
  - Any threshold that blocks a configuration change. The numbers inform a person.
  - Measuring latency or cost, which `0023` reports separately.

## Acceptance Criteria

- `reports_hit_rate_per_backend_over_the_planted_defect_corpus`
- `reports_false_positives_separately_from_hit_rate`
- `reports_hit_rate_broken_down_by_finding_category`
- `sends_temperature_zero_on_every_evaluation_request`
- `records_the_model_id_the_date_and_the_corpus_version_with_every_result`
- `includes_clean_diffs_so_a_backend_that_flags_everything_scores_badly`
- `prints_the_finding_to_defect_matching_rule_alongside_the_results`
- `fails_rather_than_scoring_when_a_backend_was_unavailable`
- `produces_the_same_result_twice_against_a_fake_backend_with_fixed_output`
- `refuses_to_compare_results_carrying_different_corpus_versions`

## Reproducibility

- `go test ./...` — exercises the harness against fake backends only.
- `mf eval --role r2` — reaches a real provider, so it is run by hand. Every reported
  number carries its model id, date, corpus version and the matching rule, and a result
  missing any of those is not a result.

## Risks and Assumptions

- **Planted defects are not real defects.** They are what someone thought a defect looks
  like, so a backend can score well on the corpus and miss what actually breaks. The
  corpus measures sensitivity to known shapes and nothing more, and reading it as review
  quality would be the mistake it exists to prevent.
- **A corpus becomes a target.** Once prompts are tuned against it, the score stops
  measuring general capability and starts measuring fit to these examples, which is the
  benchmark-saturation failure in miniature.
- **The matching rule decides the score.** A loose rule counts a vague finding as a hit
  and inflates every number; printing the rule is the only defence, and it depends on
  someone reading it.
- **Small corpora produce unstable numbers.** With few defects per category, one
  difference moves the rate visibly, and a difference that size will be over-read.
- **Model behaviour drifts behind stable ids.** A result is valid for the date it carries
  and no longer, so comparing across months compares two different models with one name.
- **Evaluation costs real tokens and is therefore run rarely,** which means the evidence
  arrives late — usually after a configuration change has already shipped.
