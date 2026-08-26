# SPEC: chore(release): publish v0.6.0 so a submodule consumer has a tag to pin

## Problem

Four repositories vendor these standards as a `.standards` submodule, and every
one of them is pinned to a commit whose `mf init` writes a second corpus beside
the one the submodule already supplies. The tag that fixes that — along with the
fourteen defects a full audit found — sits unreleased on `main`, so the
migration those four need has nothing to point at but a moving branch.

## Scope

- Includes: the `v0.6.0` tag and the release it triggers; the README's Project
  Status, install commands and Pending list brought in line with that tag; and
  `.framework.lock` recording the version this repository then runs.
- Does NOT include: any change to what `v0.6.0` contains — the tag is cut from
  `main` as it stands; migrating any consumer, which is a change to that
  repository and is specified there; a fingerprint table for `verified`
  cross-provider state; removal of the Node status line renderer, which waits on
  the consumers this release exists to unblock.

## Acceptance Criteria

- `release_publishes_five_binaries_and_a_checksum_file_for_the_tag`
- `released_binary_reports_the_tag_rather_than_the_development_default`
- `readme_names_no_release_older_than_the_newest_tag_as_current`
- `readme_install_commands_name_the_newest_tag`
- `lock_records_the_version_this_repository_runs`
