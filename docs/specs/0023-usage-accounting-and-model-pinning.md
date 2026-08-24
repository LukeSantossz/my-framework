# SPEC: feat(usage): account tokens in disjoint buckets and pin the models that resolved

## Problem

The framework spends tokens on every push and records none of it, and it names default
model ids that nothing verifies still exist, so cost is discovered on a bill and a
retired model is discovered as a failing gate.

## Design Decision

Record what each review actually consumed, and record which model actually answered.

Usage is counted in disjoint buckets — input, output, cache-read, cache-write, reasoning
— because a single total cannot distinguish a cheap cached prefix from an expensive fresh
one, and the gate's whole prompt design depends on that difference.

Vendors do not report usage in one shape. Layouts and terminology differ between
providers, and some paths return none at all. So parsing is per-backend, and a backend
that returns nothing yields an **estimate**, marked as an estimate everywhere it appears.
Reporting zero as a measured value would be a fabricated number, which the standards
forbid outright.

`mf models pin` records the model ids that actually resolved, with the date, into
`.framework.lock`. `mf doctor` warns when a configured id no longer resolves. This
answers a failure mode with a known shape: a vendor retires or silently upgrades a model,
and the gate degrades without anything saying so.

Monetary cost is optional and never bundled. Prices change constantly and a table shipped
in a binary is wrong within weeks, so `mf` computes money only from a price table the
user supplies, and reports tokens otherwise. A number the framework cannot defend is not
printed.

Accounting never fails a review. If usage cannot be determined the review still stands;
the accounting says it does not know.

## Alternatives Considered

- **Report a single token total.** Rejected. It hides the cached-prefix effect the gate's
  message ordering exists to exploit, so it could not show whether that design works.
- **Ship a price table per vendor.** Rejected. It ages faster than releases, and a stale
  price presented as cost is worse than no cost at all.
- **Estimate silently when a backend returns no usage.** Rejected. An estimate that looks
  like a measurement corrupts every aggregate built on it later, including the eval
  slice's comparisons.
- **Pin models by locking configuration to a resolved id.** Rejected. It would refuse a
  model the user deliberately changed; the lock records what happened, and `doctor`
  reports divergence, leaving the decision with a person.
- **Fail the review when usage is unavailable.** Rejected. Accounting is observation, and
  losing a review because the observation failed inverts their importance.

## Scope

- Includes:
  - `internal/usage`: the disjoint bucket type, per-backend parsing, the estimator, and
    the estimated/measured distinction carried through every aggregate.
  - Usage on the `mf review` report, per run.
  - Cumulative usage in `mf doctor`, with a resettable counter.
  - `mf models pin` and the `doctor` warning on a model id that no longer resolves.
  - Optional monetary reporting from a user-supplied price table.
  - Tests against fake backends returning each vendor's usage shape, and none, written
    first.

- Does NOT include:
  - A price table for any vendor.
  - Budget enforcement or refusing a review over a threshold.
  - Usage for `cli` backends beyond what their tool prints, which is usually nothing.
  - Per-user or per-team aggregation, or sending usage anywhere.
  - Failing anything when usage is unknown.

## Acceptance Criteria

- `reports_token_usage_in_disjoint_buckets`
- `parses_the_usage_shape_of_each_supported_api_backend`
- `marks_usage_as_estimated_when_the_backend_returned_none`
- `never_reports_zero_as_a_measured_value`
- `keeps_the_estimated_marking_through_a_cumulative_aggregate`
- `reports_no_usage_at_all_for_a_cli_backend_that_prints_none`
- `completes_the_review_when_usage_cannot_be_determined`
- `pins_the_model_ids_that_resolved_with_the_date`
- `warns_when_a_configured_model_id_no_longer_resolves`
- `computes_money_only_from_a_user_supplied_price_table`
- `reports_tokens_and_no_money_when_no_price_table_exists`

## Reproducibility

- `go test ./...`
- `mf review --role r2` against a local scriptable endpoint, whose returned usage is
  fixed by the test script.
- `mf models pin` requires reaching a real provider and is therefore run by hand, with
  the date and the provider recorded alongside the result.

## Risks and Assumptions

- **Per-backend parsing is a maintenance line, not a solved problem.** Each vendor's
  shape can change, and a parser that silently stops matching degrades to the estimator
  — which is the safe direction, but it means quality erodes without an alarm.
- **The estimator's accuracy is unknown and unclaimed.** A character-count heuristic is
  not a token count, and it must never be compared against a measured value as if the two
  were the same unit.
- **`models pin` needs a live provider.** It cannot run in CI without a key, so the pin
  will be refreshed rarely and by hand, and a stale pin is a warning nobody triggered.
- **A model id that resolves is not a model that behaves the same.** A vendor can change
  a model behind a stable id, and nothing here detects that; only the eval slice could.
- **Cumulative usage lives on a machine, not in the repository.** It is per-clone and per-
  user, so it answers "what did I spend" and never "what did this project cost".
