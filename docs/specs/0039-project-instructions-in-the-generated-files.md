# SPEC: feat(agents): let a repository add its own sections to the generated instructions

## Problem

`mf agents sync` renders each vendor file entirely from one source. That is
right for a repository that owns the source, and wrong for one that vendors it:
the file it generates is the framework's text and nothing else, so the only way
to state a project-specific obligation in `CLAUDE.md` is to edit a generated
file, which `mf check agents` then reports as drift.

The consequence is not hypothetical. The repository this was found in carries
130 lines of instructions in `CLAUDE.md` — the toolchain pin and why another
3.x SDK rewrites the lockfile, the isolate the inference runs in, the six causes
a classification collapses into and the retry the accepted decision forbids
until the spec that separates them lands — none of which any shipped document
can guess. Adopting the harness as it stands means deleting them or failing the
gate. Both answers are wrong, so the harness has been telling adopters to keep
their instructions out of the file the agent actually reads.

## Design Decision

A second, repository-owned source: `paths.agents_overlay`, marked up with the
same role markers. Each vendor file receives the source's sections for the roles
it plays, then the overlay's sections for those same roles. One file rather than
one per agent, because the role markers already say who gets what, and a second
mechanism for the same question is a way to have the two disagree.

The overlay is appended after the framework's sections and never rewritten.
Both follow from what it is. It is the repository's own text about its own
layout, so its paths are already correct and a prefix rewrite could only damage
them. And `code_conventions.md` puts an established project pattern above a
framework default, so the project's obligations read last, as the refinement
they are.

Configured-but-unreadable is an error rather than a skip. A silently dropped
overlay is a `CLAUDE.md` that looks complete and has lost exactly the
obligations no other document carries.

## Alternatives Considered

- **Per-agent `overlay` key.** Rejected: the roles already answer who receives
  what, and two answers to one question drift apart.
- **Let the consumer own the whole source.** Available today, and rejected: it
  means copying the framework's instructions into a file nothing keeps in step,
  so a repository would silently keep the standards of whatever day it adopted.
- **Put the project text in `CONTEXT.md`.** Rejected as the answer for all of
  it: `CONTEXT.md` is the domain glossary, its own skill says to proceed
  silently when it is absent, and a toolchain pin is not a domain term. Domain
  language still belongs there; obligations belong where the agent reads them.
- **Leave `CLAUDE.md` hand-written and generate `AGENTS.md` only.** Rejected: it
  gives up the drift check on the file that matters most, and the framework
  standards in it then rot exactly as they did before `mf agents sync` existed.

## Scope

- Includes: `paths.agents_overlay`; `Source.Overlay`; overlay sections appended
  per role, unrewritten; roles the overlay declares counting as declared;
  `agents sync` and `agents check` reading it; a configured overlay that cannot
  be read failing both.
- Does NOT include: a per-agent overlay key; overlay support in `mf init`, which
  writes no overlay and names none; rewriting overlay paths; any change to how
  the primary source is parsed or rendered.

## Acceptance Criteria

- `an_overlay_section_reaches_the_vendor_file_for_a_role_it_plays`
- `an_overlay_section_for_a_role_the_vendor_does_not_play_is_left_out`
- `overlay_text_is_not_path_rewritten`
- `the_overlay_may_declare_a_role_the_source_does_not`
- `a_configured_overlay_that_cannot_be_read_fails_sync_and_check`
- `check_reports_drift_when_the_overlay_changed_and_sync_was_not_run`
- A repository that configures no overlay generates byte-identical files.

## Reproducibility

```toml
[paths]
agents_overlay = "docs/agents/project.md"
```

```sh
mf agents sync && mf check agents
```

Before this change: `paths.agents_overlay` is not a key, and the project text
can only live in the generated file, where `mf check agents` reports it as
drift.

Versions: Go 1.26.7, `mf` at the commit under review.

## Risks and Assumptions

- Risk: an overlay is a place to restate what the framework already says, and a
  contradiction between the two would read as policy. Nothing detects it. The
  precedence order in `code_conventions.md` is what resolves it, and the overlay
  reading last is what makes the resolution visible.
- Assumption: one overlay is enough for every vendor a repository declares. If a
  project ever needs to tell two agents different project-specific things, it
  can declare a role for each; that is what roles are.
