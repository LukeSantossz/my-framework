# SPEC: fix(init): name the configured source in the scaffold's agents comment

## Problem

The `[agents]` section of the policy file `mf init` scaffolds opens by naming
`docs/agents/instructions.md` as the source the instruction files are generated
from, which is the built-in default and not what the same file says twenty lines
above in a repository that vendors these standards as a submodule.

## Design Decision

Reword the shipped comment to name the key rather than a path: the source is
"the file `paths.agents_source` names". A comment that names a key is true in
every layout, so `vendoredScaffold` gains nothing to rewrite and there is no
second copy of the path to keep in step. This is the same move
`docs/specs/0036` made for the instruction preamble, applied to the one place in
the scaffold that still writes from this repository's own point of view.

## Alternatives Considered

- **Have `vendoredScaffold` rewrite the comment with the submodule's real
  path.** More informative for the adopter who reads it, and one more derived
  string to keep in step: the rewrite matches literal text, so any later edit to
  the comment silently stops matching and the vendored scaffold quietly reverts
  to the wrong path. A comment that cannot go stale beats one that is more
  specific and can.
- **Delete the comment.** It carries two facts worth keeping — that the file is
  generated and that `mf check agents` compares against the source — and the
  section it heads is precisely the one an adopter is most likely to edit by
  hand, which is the edit the comment exists to prevent.
- **Leave it and document the discrepancy elsewhere.** The failure is silent in
  both directions: an adopter edits the default path, `mf agents sync`
  regenerates from the configured one, and `mf check agents` keeps passing. A
  note in another document is not read by the person already reading this one.

## Scope

- Includes: the `[agents]` comment in the `scaffold` constant of
  `internal/activate`; a test asserting the scaffold names no agents-source path
  that the file's own `paths` block contradicts, in both the shipped and the
  vendored layouts.
- Does NOT include: the `[paths]` block's own commented sample, which names the
  submodule case deliberately; `vendoredScaffold`'s existing rewrites; any
  change to `mf agents sync` or `mf check agents`.

## Acceptance Criteria

- `scaffold_agents_comment_names_the_key_not_a_path` — the scaffold's `[agents]`
  comment contains `paths.agents_source` and does not contain the literal
  `docs/agents/instructions.md`.
- `vendored_scaffold_states_no_source_path_it_does_not_configure` — for a
  scaffold rewritten for a submodule at `.standards`, every occurrence of
  `agents/instructions.md` in the file sits on a line that also names the
  submodule, so no line asserts the default path.
- `vendored_scaffold_is_still_valid_configuration` — the existing round-trip
  test over the rewritten scaffold still passes.

## Reproducibility

```sh
go test ./internal/activate/...
```

Go 1.26.7. The scaffold is a constant in `internal/activate/activate.go`; no
repository state is involved.

## Risks and Assumptions

- Assumption: an adopter reading `paths.agents_source` in a comment can find the
  key in the same file, because the `[paths]` block is twenty lines above it and
  is where the answer is.
- What would invalidate this spec: making the agents source derivable from
  something other than `paths.agents_source`, which would make the key named
  here the wrong answer.
