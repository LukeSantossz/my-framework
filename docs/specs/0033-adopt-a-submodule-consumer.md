# SPEC: feat(init): adopt a repository that carries the standards as a submodule

## Problem

`mf init` in a repository that vendors these standards as a `.standards`
submodule writes a second corpus under `docs/standards/`, scaffolds a policy
file whose `[paths]` block is commented out, and generates vendor instruction
files that point at the copy rather than at the submodule — the exact drift
`writeStandards` was written to prevent, which it cannot see because the guard
reads the *configured* standards directory and a repository being adopted has no
configuration yet.

## Design Decision

`mf init` decides where the standards are before it materialises anything, from
evidence rather than from the default. A declared submodule whose checkout
carries `docs/standards/INDEX.md` is proof that this repository already has a
corpus, so init adopts that layout: it writes an uncommented `[paths]` block
naming `<sub>/docs/standards` and `<sub>/docs/agents/instructions.md`, gives
every `[agents.*]` target the matching `path_prefix`, and materialises neither
corpus. A declared submodule that is not checked out is not evidence of
anything, so init refuses rather than guessing, and names the two ways forward.
A new `--standards <dir>` flag settles the question outright and skips detection,
for the adopter whose layout no rule can infer.

## Alternatives Considered

- **Detect from the submodule's URL.** `.gitmodules` records the remote, and
  matching it against this framework's module path would classify an
  uninitialised submodule too. Rejected: a fork, a mirror, an SSH remote or a
  vendored rename all read as unrelated, so the rule would refuse to fire
  exactly where a private adopter needs it, and it makes the framework's own
  repository URL a load-bearing constant in an adopter's activation path.
- **Materialise the corpus anyway and warn.** Keeps init non-blocking. Rejected:
  the warning arrives after the files are on disk, and `mf init` never
  overwrites, so a re-run after `git submodule update --init` leaves both
  corpora in place. The harm is done by the write, not by the silence.
- **Leave detection out and document the hand-edit.** This is today's behaviour
  plus README prose. Rejected: the README already documents the submodule
  `[paths]` block, and all four known consumers still have no `.framework.toml`
  at all — the documentation is not what is missing.
- **Refuse whenever `.gitmodules` exists.** Simplest rule. Rejected: it refuses
  a repository whose only submodule is an unrelated dependency, which is the
  common case for everyone who is not adopting these standards.

## Scope

- Includes: submodule-aware standards resolution in `mf init`; an uncommented
  `[paths]` block in the scaffold for that case; `path_prefix` on each generated
  `[agents.*]` target; a `--standards <dir>` flag on `mf init`; refusal on an
  uninitialised declared submodule; `paths.agents_source` added to the three
  adopter-facing `[paths]` enumerations that omit it.
- Does NOT include: initialising the submodule on the adopter's behalf; bumping
  any consumer's pin; `mf upgrade` changes; writing `CONTEXT.md`, `.github/`
  templates or `.gitattributes`, which `mf init` still does not do; any change
  to how the gates resolve paths once configured.

## Acceptance Criteria

- `adopts_the_submodule_layout_when_a_declared_submodule_carries_the_corpus`
- `writes_no_second_corpus_when_the_submodule_supplies_one`
- `scaffolds_an_uncommented_paths_block_naming_the_submodule`
- `gives_every_generated_agent_target_a_matching_path_prefix`
- `refuses_when_a_declared_submodule_is_not_checked_out`
- `refuses_a_submodule_that_carries_no_instruction_source`
- `writes_nothing_when_it_refuses`
- `honours_an_explicit_standards_flag_over_detection`
- `leaves_a_repository_with_an_unrelated_checked_out_submodule_unchanged`
- `behaves_exactly_as_before_in_a_repository_with_no_gitmodules`
- `the_scaffolded_paths_enumeration_names_agents_source`

## Reproducibility

```sh
git init -b main probe && cd probe
printf '[submodule ".standards"]\n\tpath = .standards\n\turl = https://example.invalid/mf.git\n' > .gitmodules
mkdir -p .standards && git add -A && git commit -m "chore: seed"
mf init
```

Before this change: `+ standards wrote 13 document(s) to docs/standards`.
After: init refuses, because `.standards` is not checked out. With the submodule
populated, init reports the submodule supplies the standards and writes a
`[paths]` block naming it.

Versions: Go 1.26.7, `mf` at the commit under review.

## Risks and Assumptions

- Assumption: a checkout containing `docs/standards/INDEX.md` is a standards
  corpus. Invalidated if the corpus layout inside the submodule ever moves,
  which would also break every path the README documents.
- Assumption: a corpus and the instruction source travel together. They do in
  every release that has the source at all, and a pin older than it — which is
  what all four known consumers carry — is refused with the command that moves
  it forward rather than adopted into a state nothing can generate from.
- Assumption: refusing on an uninitialised submodule costs less than a duplicate
  corpus. Invalidated if adopters commonly carry unrelated uninitialised
  submodules; `--standards` is the escape hatch and the refusal names it.
- Risk: an adopter who ran `mf init` before this change already has both a copy
  and a submodule. This change does not clean that up, and cannot: which corpus
  they edited since is not knowable from the files.
