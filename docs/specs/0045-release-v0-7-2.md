# SPEC: chore(release): publish v0.7.2 so an overlay consumer pins the fixed header

## Problem

The first repository to use `paths.agents_overlay` received generated files
whose header names one of two sources — the one inside the submodule it does not
own — so the header tells a contributor to edit the single file they must not
touch. The fix is on `main` and unreleased, and that repository's adoption pull
request is open now, pinning a tag that still has the defect.

The release publishing fix is in the same state, and it is the one that keeps a
flaky upload from discarding a release: leaving it unreleased means the next tag
is cut by the code that already lost `v0.7.1` once.

## Design Decision

Cut the tag from `main` as it stands, as a patch. Both changes fix behaviour
that was wrong; neither adds a key, a command or an output shape. The one
visible effect is on repositories that declare an overlay, whose generated files
gain header lines — reported by the agents gate, fixed by the command it names,
and there is exactly one such repository, with its adoption still open.

## Alternatives Considered

- **Release as `v0.8.0`.** Rejected: a repository upgrading from `v0.7.1` gains
  no capability. The generated header changing is a correction, not a feature.
- **Wait for the adoption to merge and release after.** Rejected: it would merge
  a repository onto a tag whose defect is already fixed here, and then need a
  second pull request there to undo the pin.

## Scope

- Includes: the `v0.7.2` tag and the release it triggers; the README's Project
  Status and install commands; `.framework.lock`.
- Does NOT include: any change to what `v0.7.2` contains; bumping any consumer's
  pin, which is a change to that repository.

## Acceptance Criteria

- `release_publishes_five_binaries_and_a_checksum_file_for_the_tag`
- `readme_names_no_release_older_than_the_newest_tag_as_current`
- `readme_install_commands_name_the_newest_tag`
- `lock_records_the_version_this_repository_runs`

## Reproducibility

```sh
gh release view v0.7.2 --json assets -q '.assets|length'   # 6
mf doctor                                                  # first line: mf v0.7.2
```

Versions: Go 1.26.7, `mf` at the tagged commit.

## Risks and Assumptions

- Risk: this is the first tag cut by the rewritten publish step. If it fails, it
  now fails recoverably — that is what the step changed — and the run can be
  re-run rather than the tag deleted.
