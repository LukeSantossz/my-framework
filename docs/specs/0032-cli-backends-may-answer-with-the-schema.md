# SPEC: feat(backend): let a cli backend declare it answers with the schema

## Problem

The `cli` kind records every answer as prose because "an agentic CLI cannot be
asked for a schema", which is no longer true: agy answers with exactly the
findings shape the role prompt asks for. Discarding that structure means such a
review can never block a push however severe its findings, and `mf eval` scores
it zero, since every finding carries the category `unstructured` rather than the
one the backend reported.

## Scope

- Includes: a `backends.<name>.structured` key; `CLI.Review` parsing the answer
  through the same `report.ParseFindings` the `api` kind uses when the key is
  set; falling back to recording the prose when a declared backend answers
  something else; and setting the key on `agy`.
- Does NOT include: detecting the shape instead of declaring it; passing agy's
  `--json-schema` flag, since it answers correctly without one; changing
  `codex` or `gemini`, neither of which was verified to answer this way.

## Acceptance Criteria

- `a_cli_that_answers_with_the_schema_is_read_as_findings`
- `a_blocking_severity_from_a_cli_backend_survives_to_the_result`
- `a_declared_backend_answering_prose_has_the_prose_recorded`
- `a_cli_that_declares_nothing_is_recorded_verbatim_as_before`

## Risks and Assumptions

- Assumption: declaring is better than detecting. Detection would make one
  backend behave two ways depending on whether a given answer happened to
  parse, and the severity of a finding decides whether a push is blocked.
- Risk: a backend that stops answering in the shape it declared degrades to
  prose rather than failing, so the change is invisible until someone reads the
  review. That is the same trade the api kind already makes, for the same
  reason: a malformed answer is still an answer, and reading it as a clean
  review is the worse failure.
