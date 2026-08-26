# SPEC: chore(release): publish v0.7.0 so a consumer can carry its own instructions

## Problem

The repository next in line to adopt the harness carries 130 lines of
project-specific instructions in `CLAUDE.md`. Until `paths.agents_overlay`
landed there was nowhere to put them: a repository vendoring the corpus
generated the framework's text and nothing else, and editing the output is what
`mf check agents` reports as drift. That feature, and the fix for the three
skill references such a repository received unrewritten, sit unreleased on
`main` — so the migration they exist to unblock has nothing to pin but a moving
branch.

## Scope

- Includes: the `v0.7.0` tag and the release it triggers; the README's Project
  Status and install commands brought in line with that tag; `.framework.lock`
  recording the version this repository then runs.
- Does NOT include: any change to what `v0.7.0` contains — the tag is cut from
  `main` as it stands; migrating any consumer, which is a change to that
  repository and is specified there; regenerating the instruction files in the
  two repositories already adopted, which is the same.

## Acceptance Criteria

- `release_publishes_five_binaries_and_a_checksum_file_for_the_tag`
- `released_binary_reports_the_tag_rather_than_the_development_default`
- `readme_names_no_release_older_than_the_newest_tag_as_current`
- `readme_install_commands_name_the_newest_tag`
- `lock_records_the_version_this_repository_runs`

## Reproducibility

```sh
gh release view v0.7.0 --json assets -q '.assets|length'   # 6
mf doctor | head -1                                        # mf v0.7.0
```

`mf doctor` is where the build reports itself; there is no `mf --version`.

Versions: Go 1.26.7, `mf` at the tagged commit.

## Risks and Assumptions

- Assumption: minor rather than patch. `paths.agents_overlay` is a new key and a
  new capability, and nothing that existed before behaves differently — a
  repository declaring no overlay generates byte-identical files.
- Risk: the two repositories already adopted keep the three unrewritten skill
  references until they update the pin. They are wrong references in a generated
  file, not a gate that fails, and each update is specified in that repository.
