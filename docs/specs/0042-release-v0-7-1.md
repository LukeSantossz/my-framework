# SPEC: chore(release): publish v0.7.1 so the migration it unblocks has a tag to pin

## Problem

The repository being migrated has an open pull request holding spec `0037` and a
migration branch that must claim `0038`. Until the records gate could tell a
number another branch is holding from one a deleted record left behind, that
branch could not push: both hooks fail closed. The fix is on `main` and
unreleased, so the migration has nothing to pin but a moving branch.

## Scope

- Includes: the `v0.7.1` tag and the release it triggers; the README's Project
  Status and install commands brought in line with that tag; `.framework.lock`
  recording the version this repository then runs.
- Does NOT include: any change to what `v0.7.1` contains — the tag is cut from
  `main` as it stands; migrating any consumer, which is specified there.

## Acceptance Criteria

- `release_publishes_five_binaries_and_a_checksum_file_for_the_tag`
- `readme_names_no_release_older_than_the_newest_tag_as_current`
- `readme_install_commands_name_the_newest_tag`
- `lock_records_the_version_this_repository_runs`

## Reproducibility

```sh
gh release view v0.7.1 --json assets -q '.assets|length'   # 6
mf doctor                                                  # first line: mf v0.7.1
```

Versions: Go 1.26.7, `mf` at the tagged commit.

## Risks and Assumptions

- Assumption: patch rather than minor. No key, command or output shape changes;
  a gate stops refusing a case it should never have refused.
