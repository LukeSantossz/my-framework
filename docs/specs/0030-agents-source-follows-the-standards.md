# SPEC: fix(agents): let the instruction source move with the standards

## Problem

A repository that vendors these documents as a submodule can relocate its
standards, specs and records, but not the document its vendor instruction files
are generated from — `agents.SourcePath` is a constant — so `mf agents sync` and
`mf check agents` are the only commands such a repository cannot run.

## Scope

- Includes: a `paths.agents_source` key with the shipped layout as its default;
  `agents.Options` taking the resolved path; and the three call sites in
  `mf agents sync`, `mf check agents` and the generation step of `mf init`.
- Does NOT include: a command that copies standards into a submodule consumer —
  the submodule is the corpus, so there is nothing to copy; relocating the
  vendor output files, which `[agents.*].file` already names; any change to how
  `path_prefix` rewrites references inside the generated text.

## Acceptance Criteria

- `agents_sync_reads_the_source_where_the_repository_keeps_it`
- `the_source_path_falls_back_to_the_shipped_layout`
- `a_submodule_consumer_passes_every_gate_including_agents`
