# SPEC: fix(harness): close what a full-codebase audit found

## Problem

A full audit of the tree — every package read against the standards rather than
against the test suite, which is green — found fourteen defects the gates cannot
see, three of which let a review be recorded that never happened, send a diff to
an endpoint the configuration says was overridden, or write a file outside the
repository from a committed policy file.

## Design Decision

Fix them in one change, because they are one finding: every gate and every test
passes over all of them, so each is evidence about the same blind spot rather
than an independent bug. Each fix takes the shape the codebase has already
chosen for its class — the resolved cascade for a configured route, the
`pathProblem` containment rule for a configured path, `Unavailable` for a
backend that answered nothing — so none of them introduces a rule the tree does
not already keep somewhere else. Where a fix cannot be made safe, the surface is
refused rather than left looking wired.

## Alternatives Considered

- **One change per defect.** Fourteen branches, fourteen reviews. Rejected: they
  share a cause and most share a test file, so splitting them multiplies the
  review cost of the batch without making any single fix easier to judge, and
  the framework's own precedent for an audit sweep is `docs/specs/0027`.
- **Fix only the three that can produce a false review.** Rejected: the rest are
  the same class of defect one severity down, and leaving them means the next
  audit finds the same list. The cheap ones are cheap now and expensive as a
  second pass.
- **Fix the empty-answer case by retrying the backend.** Rejected: a retry
  charges tokens twice for an answer the vendor already declined to give, and a
  content filter or a safety block returns empty deterministically. Reporting
  unavailability lets the chain fall through, which is what the chain is for.

## Scope

- Includes:
  - `api` backends resolve their provider's endpoint, key variable and kind
    through the configuration cascade rather than the raw machine file.
  - An `api` answer whose content is empty is unavailability, not a review.
  - `agents.<name>.file` is contained to the repository and its directory is
    created before the file is written.
  - The docs gate ignores a markdown reference inside a URL.
  - The Spec Gate counts an ordered-list acceptance criterion.
  - `mf config set` refuses a project file whose section header it cannot parse,
    rather than appending a duplicate table that makes the file unloadable.
  - `mf author declare` clears a model the new declaration does not carry, and
    parses its flags the way every other command does.
  - The status line refresh lock is released.
  - A transcript the status line could not finish reading degrades to the
    placeholder rather than to a stale figure.
  - `mf init` reports a failed generation as a returned error rather than by
    matching its own message text.
  - `mf eval` and `mf explain` record what they spent.
  - `mf eval` reports a corpus case it skipped.
  - The GitHub client carries a timeout.
  - `review.model` is resolvable, so its documented environment form lands.
- Does NOT include: the `inproc` backend kind, which is a documented stub with
  its own scope in `docs/specs/0019`; the `anthropic` wire shape's missing
  `temperature` and fixed `max_tokens`, which change what a reviewer returns and
  belong with a measurement; the cost line's single-model assumption, which
  needs a usage store keyed by model; the design gate's colour regular
  expression, which `docs/standards/design.md` already scopes.

## Acceptance Criteria

- `resolves_an_api_backends_endpoint_through_the_cascade_not_the_machine_file`
- `records_an_empty_api_answer_as_unavailable`
- `refuses_an_agent_file_that_leaves_the_repository`
- `creates_the_directory_an_agent_file_names`
- `ignores_a_markdown_reference_inside_a_url`
- `counts_an_ordered_list_acceptance_criterion`
- `refuses_to_set_a_key_under_a_section_header_it_cannot_parse`
- `clears_the_model_when_a_declaration_does_not_carry_one`
- `reports_a_missing_value_for_every_author_declare_flag`
- `releases_the_refresh_lock`
- `reports_no_context_when_the_transcript_could_not_be_read`
- `fails_init_when_the_instruction_files_were_not_generated`
- `records_the_tokens_eval_and_explain_spend`
- `names_a_corpus_case_it_could_not_load`
- `bounds_a_github_request_in_time`
- `resolves_review_model_so_its_environment_form_lands`

## Reproducibility

`go test ./...` on Go 1.26.7. Each criterion is a test in the package that owns
the defect; the three that need a server use the fake transports already in
`internal/backend` and `internal/forge`.

## Risks and Assumptions

- Assumption: no adopter depends on `mf config set` appending a duplicate table,
  because the result does not load. Invalidated only if a file already carries
  one, which the refusal reports rather than compounds.
- Assumption: an empty `api` answer is always unavailability. A model that
  legitimately returns no text has nothing a reviewer could record either way,
  so the two are indistinguishable and the weaker claim is the honest one.
- Risk: containing `agents.<name>.file` refuses a layout an adopter already
  committed. The refusal names the key and the value, and a path inside the
  repository is the only one `mf agents check` could compare anyway.
