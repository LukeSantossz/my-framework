# my-framework — sec-free edition

The development standards of my-framework, as plain documents, for environments
where no framework executable may run and no code may leave the organisation.

## What It Does

- Gives an AI coding agent and the Developer directing it one set of binding
  standards: a design spec approved before code, test-first order, Conventional
  Commits, and a pull request that records how the change was reviewed.
- Defines review without a second provider: the same model reviews the change
  in contexts isolated from the one that wrote it (`docs/standards/isolated_review.md`).
- Ships the agent instruction files ready to use, so an agent that starts in an
  adopting repository finds its obligations at the root.

## What It Is

A documents-only edition of my-framework: Markdown files and a licence, with no
executable, script, git hook, submodule or dependency of any kind. It is
generated from the full edition at each release and published read-only on the
`secfree` branch, tagged `secfree-vX.Y.Z` after the release it was cut from.

## Getting Started

### Prerequisites

Git, and an AI coding agent your organisation already allows.

### Installation

Clone only this branch, so nothing from the full edition comes with it:

```sh
git clone --branch secfree --single-branch --no-tags \
  https://github.com/LukeSantossz/my-framework my-framework-secfree
```

### Adopting it in a project

Ask your agent to do this, or do it by hand. Nothing here runs a program.

1. Copy `docs/standards/` from this edition into the project, at the same path.
2. Copy `CLAUDE.md` and `AGENTS.md` to the project root. If the project already
   has either file, keep it and add one line at its top telling the agent to
   read `docs/standards/INDEX.md` and treat it as binding.
3. Create `docs/specs/` and `docs/adr/` in the project. They start empty.
4. Commit the result as `docs: adopt the my-framework standards`.

To move to a newer edition, copy the same files again and review the diff
before committing it.

## Known Issues & Limitations

- **No gate runs.** The rules hold only as well as the agent follows them and
  the Developer checks them in the pull request. `docs/standards/isolated_review.md`
  lists what to check by hand.
- **Same-provider review.** R2 and R3 are performed by the same model as the
  Author. Separate contexts catch much of what a fresh reader would, but two
  contexts of one model share its blind spots, and every pull request says so.
- **Read-only.** A change to these documents belongs in the full edition's
  source; an edit made here is lost at the next release.

## License

MIT, as in `LICENSE`.
