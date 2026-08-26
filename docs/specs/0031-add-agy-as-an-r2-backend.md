# SPEC: feat(config): add agy as an R2 backend the Developer already runs

## Problem

R2 has had no reachable backend on this machine: `codex` is out of quota and
`gemini` is not installed, so every push records that the cross-provider layer
did not run.

## Scope

- Includes: a `[backends.agy]` block declaring Antigravity's CLI as a
  prompt-driven `cli` backend, pinned to a Gemini model so the provider label
  states the vendor that actually reviews; its placement in the R2 chain after
  `codex`; and its unavailability patterns.
- Does NOT include: parsing the findings JSON agy returns — the `cli` kind
  records output verbatim, so agy's severities are recorded as prose and cannot
  block. That is a separate decision, stated in Risks; any change to `codex`'s
  or `gemini`'s configuration; a machine-layer route, since agy authenticates
  itself and needs none.

## Acceptance Criteria

- `doctor_reports_agy_as_found_on_a_machine_that_has_it`
- `a_review_through_agy_names_agy_as_the_backend_that_ran`
- `agy_reports_a_planted_defect_in_the_diff_it_is_sent`
- `the_chain_advances_past_agy_when_it_is_not_installed`

## Risks and Assumptions

- Assumption: the Developer keeps agy authenticated. It carries no credential of
  its own here, so a logged-out CLI reads as unavailable and the chain advances.
- Assumption: pinning a Gemini model is what makes the cross-provider claim true
  against an Anthropic Author. Changing the model to a Claude one would keep the
  rule satisfied by name while breaking it in substance.
- Risk: agy answers with exactly the findings schema this harness asks for, and
  the `cli` kind throws that structure away — two findings it classed blocking
  were recorded as one advisory prose blob, verified. Its review is therefore
  advisory only, and cannot block a push however severe. Closing that needs a
  `cli` backend to be able to declare that it answers with the schema.
- What would invalidate this spec: agy ceasing to accept `--print`, or its model
  ids ceasing to carry the effort suffix, which is why no `--effort` is passed.
