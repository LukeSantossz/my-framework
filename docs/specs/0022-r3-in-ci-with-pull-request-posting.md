# SPEC: feat(review): run R3 in CI against a configurable provider and post its findings

## Problem

R3 is a GitHub app wired outside the repository, so the layer exists only for whoever
installed that app, its provider cannot be chosen, and nothing in the framework can run
it.

## Design Decision

Make R3 the third configuration of the role runner, executed by a workflow on pull
requests, with findings posted back as one comment.

The workflow runs `mf review --role r3 --pr <n> --post`. Provider and model come from the
repository's configuration; the key comes from a repository secret, referenced by the
variable name the configuration already stores. Nothing new is invented — R3 differs from
R2 only in when it runs and what it sees.

R3 sees more than R2 does: the pull request title, body and linked spec, alongside the
diff. That is the point of running it there. Reviewing intent against implementation is
only possible where the intent is written down, and the pull request is where it is.

Findings post as a single comment, replaced on re-run rather than appended, because a
gate that runs on every push and comments every time trains people to ignore it. The
comment names the backend, provider, model and the count by category, so a reader can
tell a strong review from a fallback.

CodeRabbit becomes one option among several — a backend of kind `external`: declared in
configuration, executed by its own app, and recorded in the review-layers line so a human
sees that R3 ran and by what. The framework stops depending on it and stops pretending it
is the definition of the layer.

CI never fails on findings. R3 is advisory like every other layer, and a review layer
that blocks merges on advisory output would be a blocking gate wearing the wrong name.
The workflow fails only when it is misconfigured.

## Alternatives Considered

- **Post each finding as an inline review comment.** Rejected for this slice. It is more
  useful when the findings carry reliable line numbers, and only `api` backends produce
  those; a `cli` backend's prose would have nowhere to land. Worth revisiting once the
  eval slice measures how often line numbers are right.
- **Fail the build on findings.** Rejected. The standards make every layer advisory, and
  a blocking R3 would make the weakest-context reviewer the strictest gate.
- **Append a comment per run.** Rejected. Comment spam is how a review bot becomes
  invisible.
- **Keep R3 as CodeRabbit and only document it.** Rejected. That is today's state: a layer
  the framework cannot run, configure or replace.
- **Reuse the pre-push gate in CI instead of a distinct role.** Rejected. It would review
  the branch against its base without the pull request's intent, discarding the only
  context that distinguishes R3.

## Scope

- Includes:
  - The `r3` role wiring: pull request context assembly (title, body, linked spec, diff).
  - `mf review --role r3 --pr <n> [--post]`.
  - Comment rendering and replace-on-rerun posting through the GitHub API.
  - The `external` backend kind, declared and recorded but executed elsewhere.
  - `.github/workflows/review.yml`, with the secret referenced by configured name.
  - Tests against a fake API, written first.

- Does NOT include:
  - Inline per-line review comments.
  - Failing CI on findings.
  - Any forge other than GitHub.
  - Running R3 on forks, where secrets are unavailable by design; the workflow reports
    that it cannot run rather than appearing to pass.
  - Adjudicating findings. A human still resolves or justifies each one.

## Acceptance Criteria

- `assembles_the_pull_request_title_body_and_linked_spec_alongside_the_diff`
- `posts_findings_as_one_comment_naming_backend_provider_model_and_category_counts`
- `replaces_its_previous_comment_instead_of_appending_a_new_one`
- `exits_zero_when_findings_exist_so_ci_does_not_block_on_advisory_output`
- `exits_non_zero_only_when_it_is_misconfigured`
- `exits_zero_and_says_so_when_no_backend_was_available`
- `reports_that_it_cannot_run_on_a_fork_rather_than_appearing_to_pass`
- `records_an_external_backend_in_the_review_layers_line_without_executing_it`
- `resolves_the_api_key_from_the_variable_name_the_configuration_stores`
- `never_writes_the_key_into_the_comment_or_the_log`

## Reproducibility

- `go test ./...`
- `mf review --role r3 --pr <n> --dry-run`
- The posting path is exercised against a fake GitHub API, never the real one.

## Risks and Assumptions

- **A comment that replaces itself loses history.** The previous review's text is gone,
  so a reader cannot see that a finding was raised and then disappeared. The alternative
  is spam, and the pull request timeline still records that the comment was edited.
- **R3 sees the pull request body, which is user-controlled text going into a prompt.**
  A body crafted to instruct the reviewer is prompt injection with a straightforward
  delivery path, and this slice does not solve it; the mitigation is that R3 only reports
  and never acts.
- **Running a review on every push costs tokens proportional to pull request activity.**
  Nothing here rate-limits it, and a busy branch will pay repeatedly for reviews nobody
  reads.
- **Forks are the common case for outside contributions.** R3 will be unavailable exactly
  where an unknown contributor's change would benefit most.
- **`external` records a claim nothing verifies.** The framework will state that
  CodeRabbit reviewed because configuration says it is wired, not because it observed a
  review; that is weaker than every other backend's report and must read as such.
