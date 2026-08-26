# SPEC: fix(agents): say where the instruction source can be edited

## Problem

The instruction preamble tells every reader to edit the source and never the
generated file, which in a repository that vendors these standards as a
submodule points inside somebody else's checkout — the source is the framework's,
a local edit there is untracked, and the next submodule update discards it.

## Scope

- Includes: the preamble in `docs/agents/instructions.md`, reworded to be true
  in both layouts and to stop calling the generated file its own source; the
  regenerated `CLAUDE.md` and `AGENTS.md`; the `v0.6.1` tag and the release it
  triggers, so the consumers about to pin have one whose instructions are true
  where they read them; the README and `.framework.lock` naming that tag.
- Does NOT include: any change to which sections a role receives, to
  `path_prefix` rewriting, or to any gate; migrating a consumer, which is a
  change to that repository and is specified there.

## Acceptance Criteria

- `the_preamble_names_both_layouts_and_what_each_may_edit`
- `mf_check_agents_passes_after_the_regeneration`
- `release_publishes_five_binaries_and_a_checksum_file_for_the_tag`
- `readme_names_no_release_older_than_the_newest_tag_as_current`
- `lock_records_the_version_this_repository_runs`
