# SPEC: fix(agents): name the overlay in the header of the files it went into

## Problem

A generated file's header says it was generated from one source. With
`paths.agents_overlay` configured there are two, and the header names only the
framework's — which, in the repository this was found in, is inside a submodule
that repository does not own. So the header tells a contributor to edit the one
file they must not touch, and says nothing about the one that holds every
project-specific obligation in the same document.

The instruction below it makes it worse: "edit the source and re-run" is
unambiguous and wrong for anything the overlay contributed.

## Design Decision

The header names both files when both were used, and points the reader at the
one their section came from rather than at "the source". Overlay sections are
filtered by role like every other section, so "used" means this file received
one: a target playing no role the overlay covers is not sent to a file that
contributed nothing to it. A file generated
without an overlay keeps exactly the header it has now, so nothing churns in a
repository that declares none.

The overlay path travels on `Source`, beside the sections it produced, rather
than as another parameter on `Render`. It is a property of what was loaded, and
a second positional string next to `sourcePath` is a pair of arguments waiting
to be passed in the wrong order.

## Alternatives Considered

- **Name only the overlay when one exists.** Rejected: the framework's source is
  still where most of the file comes from, and a header naming one of two is the
  defect being fixed, whichever one it picks.
- **Mark each section with its origin.** Rejected: it is more machinery than the
  problem needs, it doubles the diff of every generated file, and the two
  sources are already visually separated — the framework's sections come first.
- **Leave it and say so in the adopter's config comment.** Rejected: that is
  where this was found, in a comment that had to be written by hand and was
  wrong. The generated file is what a contributor reads.

## Scope

- Includes: `Source.OverlayPath`; `header` taking it; `load` recording it;
  refusing either path when it carries `-->`, which would close the comment the
  header is written in.
- Does NOT include: what the sections are or the order they are written in; any
  change when no overlay is configured; marking individual sections.

## Acceptance Criteria

- `the_header_names_both_sources_when_an_overlay_was_used`
- `the_header_is_byte_identical_when_no_overlay_is_configured`
- `the_header_tells_the_reader_to_edit_the_source_their_section_came_from`
- `render_refuses_a_path_that_would_close_the_header_comment`
- `the_header_does_not_name_an_overlay_that_contributed_nothing`

## Reproducibility

In a repository with `paths.agents_overlay` configured:

```sh
mf agents sync && head -4 CLAUDE.md
```

Before this change: the header names only `paths.agents_source`.

Versions: Go 1.26.7, `mf` at the commit under review.

## Risks and Assumptions

- Risk: every generated file in a repository that declares an overlay changes by
  the header lines, so the first `mf agents sync` after upgrading reports drift.
  That is the gate doing its job, and the fix is the command it names.
