# SPEC: chore(release): publish v0.5.0 and describe R3 as it actually runs

## Problem

`docs/specs/0027` scoped publishing a release out of itself, so the work it
carried sits unreleased on `main` while the newest tag is one whose `mf init`
cannot perform the adoption the README documents — and the README's Known Issues
separately claims R3 does not run here, when the forge app declared in
`.framework.toml` reviews every pull request and returned twenty-eight findings
on the one that closed `0027`.

## Scope

- Includes: the `v0.5.0` tag and the release it triggers; the README's Project
  Status, install recommendation and Known Issues brought in line with that tag;
  the Known Issues entry for R3 corrected to say that it runs as a forge app and
  that the workflow-hosted reviewer is an addition rather than a gap; the same
  correction to the heading of the workflow summary, whose body already said it;
  and `.framework.lock` recording the version this repository then runs.
- Does NOT include: any change to what `v0.5.0` contains — the tag is cut from
  `main` as it stands; configuring a reachable R2 or R3 backend, which needs a
  credential this repository does not hold; a fingerprint table for
  `verified` cross-provider state; or a second R1 backend.

## Acceptance Criteria

- `release_publishes_five_binaries_and_a_checksum_file_for_the_tag`
- `released_binary_reports_the_tag_rather_than_the_development_default`
- `readme_names_no_release_older_than_the_newest_tag_as_current`
- `readme_known_issues_state_r3_runs_as_a_forge_app`
- `workflow_summary_heading_names_which_reviewer_did_not_run`
- `lock_records_the_version_this_repository_runs`
