# SPEC: fix(agents): rewrite the agent-skill references a submodule consumer receives

## Problem

`mf agents sync` rewrites one prefix: `docs/standards/`. The source also names
its sibling agent-skill documents — `docs/agents/issue-tracker.md`,
`docs/agents/triage-labels.md`, `docs/agents/domain.md` — and those survive the
rewrite untouched. In a repository that vendors the corpus, they resolve to
nothing: the files are at `.standards/docs/agents/`, and the generated
`CLAUDE.md` sends every session to three paths that do not exist.

Both consumers adopted so far carry it. The failure is silent in the way this
framework exists to end: the file reads as though the skills are wired, `mf
check agents` passes because the output matches the source it was generated
from, and the agent discovers the truth by opening a path and finding nothing.

## Design Decision

Derive the agent-document prefix from the source path rather than adding
configuration. `paths.agents_source` already names where the instructions live,
and the sibling skill documents live beside them by construction — the framework
ships `docs/agents/instructions.md` next to `docs/agents/domain.md`, and a
submodule carries the whole directory. So the directory of the configured source
is the prefix those references need, and a repository that has already said
where its source is does not have to say it twice.

The rewrites move off the finished string and onto the body alone, with the
header prepended afterwards. The header names the configured source path, which
in a vendored layout already begins with `.standards/docs/agents/`; a rewrite
applied over it turns that into `.standards/.standards/docs/agents/`. Today the
`docs/standards/` rewrite misses the header by luck rather than by design,
because that string does not appear in it.

## Alternatives Considered

- **A second `path_prefix` key per agent.** Rejected: it is the same fact stated
  twice, and the two can disagree — a repository could point `agents_source`
  into the submodule and the skill prefix somewhere else, producing a file whose
  references are wrong in a new way nothing checks.
- **Rewrite `docs/` wholesale using the prefix's parent.** Rejected: `docs/specs`
  and `docs/adr` belong to the consumer, not to the submodule, and rewriting
  them would send the Spec Gate's reader into the framework's own archive.
- **Drop the skill references from the shared source.** Rejected: they are the
  Author role's skills, and a generated file that names no skills is not a fix,
  it is the same gap with less evidence.

## Scope

- Includes: `agents.Render` deriving the agent-document prefix from the source
  path and applying both rewrites to the body only; the header prepended after.
- Does NOT include: new configuration; the `docs/standards/` prefix, whose
  behaviour is unchanged; regenerating the files in the repositories already
  adopted, which is a change to those repositories.

## Acceptance Criteria

- `render_rewrites_a_skill_reference_to_where_the_vendored_source_keeps_it`
- `render_leaves_the_header_alone_when_the_source_is_vendored`
- `render_does_not_rewrite_the_consumer_owned_spec_and_adr_paths`
- The existing `path_prefix` behaviour is unchanged.

## Reproducibility

In a repository whose `paths.agents_source` is `.standards/docs/agents/instructions.md`:

```sh
mf agents sync
grep -n "docs/agents/" CLAUDE.md
```

Before this change: three references without the `.standards/` prefix. After:
three that resolve.

Versions: Go 1.26.7, `mf` at the commit under review.

## Risks and Assumptions

- Assumption: the skill documents sit beside the instruction source. That is how
  the framework ships them and how a submodule delivers them; a repository that
  separated them would need the second key this rejects, and none does.
- Risk: a repository owning its source in a non-default directory now has those
  references rewritten to that directory. That is the correct answer for the
  same reason — the skills would be there.
