# SPEC: feat(design): derive a binding visual identity for the surfaces the framework renders

## Problem

The one page this framework generates — the CRUX explainer from `0025` — carries a
palette, a type scale and an accent colour that were chosen while writing the renderer
and are recorded nowhere, so nothing can say whether a future change to them is a
decision or a drift.

## Design Decision

Adopt the `DESIGN.md` format from `https://github.com/voltagent/awesome-design-md` and
author the values for this project, taking the design direction from that collection's
Warp entry. Warp is a terminal product, so its visual language was formed under the same
constraints this framework's surfaces have — monospace, dark-first, a column of text —
and its strongest principle is directly useful here: it carries no chromatic brand
accent, letting an off-white do the work a brand colour usually does. A tool whose
premise is that the provider is configuration should not wear any vendor's accent
colour, and this is the design decision that makes that visible rather than merely
stated.

The result is binding. `docs/standards/design.md` becomes a standard with a gate,
`mf check design`, which reads the palette out of the standard the way every other
deterministic check reads its vocabulary out of the document that owns it
(`docs/adr/0009-checks-derive-vocabularies-from-standards.md`). The gate fails a
rendered surface that uses a colour the standard does not declare.

Derivation is checked, not asserted. The standard records a one-way fingerprint of each
identity-carrying value in the source entry — its colours and its typefaces — and the
gate refuses any declared value that matches one. That converts "inspired, not copied"
from a claim about intent into a property a machine verifies, which is what the
Developer's choice of a derived identity needs in order to be defensible.

## Alternatives Considered

- **Copy the Warp entry as-is.** Rejected at the Developer's decision. It is what the
  reference repository is built for, but it means publishing a tool dressed in another
  company's identity. The repository's MIT licence covers the documents; it explicitly
  disclaims ownership of any site's visual identity, so the licence is not the thing
  that would make it acceptable.
- **Author values with no source at all.** Rejected at the Developer's decision. It has
  no trademark question to answer, but it also has no design direction: the values would
  have been chosen the same way the current ones were, which is the problem this spec
  exists to fix.
- **Keep the standard advisory, with no gate.** Rejected at the Developer's decision.
  It has a real precedent — `status_line.md` fixes the five facts and their order and
  deliberately binds neither colours nor glyphs — but this framework's own premise is
  that a standard describing behaviour nothing performs is the failure it exists to fix.
- **Derive from a documentation-site entry (Mintlify) instead.** Rejected at the
  Developer's decision. The explainer is shaped like a documentation page, so the
  analogy was closer in form; the terminal-formed direction was preferred because it
  covers both surfaces this framework renders rather than only the page.
- **Derive from the Claude entry.** Rejected. The Author of this change runs on that
  provider, and dressing a provider-agnostic tool in one provider's identity contradicts
  the goal that motivated the rebuild, on the most visible surface the tool has.
- **Fingerprint every value in the source, including spacing and radii.** Rejected. A
  4px radius and an 8px step are nobody's identity, and a gate that flagged them would
  fail on arithmetic rather than on appropriation, which teaches people to route around
  it.

## Scope

- Includes:
  - `docs/standards/design.md`, in the nine-section `DESIGN.md` format, with values
    authored for this project and a Derivation section recording the source, the date it
    was read, and what the fingerprint check does and does not prove.
  - `internal/design`, which parses the palette and the fingerprints out of that
    standard; a parse failure is a hard error, never a silently empty vocabulary.
  - `mf check design`, wired into `mf check` as a seventh gate.
  - The CRUX explainer restyled onto the declared tokens, including removing its
    chromatic accent.
  - What the standard says about the terminal: which of its rules reach terminal output
    and which cannot, stated rather than left to be inferred.
  - Tests, written first, including one that fails a surface using an undeclared colour
    and one that fails a token colliding with a source fingerprint.

- Does NOT include:
  - Any change to `status_line.md` or to what the status line renders. It binds the five
    facts and their order and deliberately binds neither colours nor glyphs; a design
    standard that reached it would be overruling a standard, not decorating a surface.
  - Colour in terminal output beyond what already exists. The reader's theme owns the
    terminal palette.
  - A logo, a wordmark, or any brand asset.
  - Copying any value from the source entry into this repository in readable form.
  - Applying the standard to surfaces that do not exist: there is no web UI, no
    dashboard and no marketing page to style.

## Acceptance Criteria

The standard and its vocabulary

- `parses_every_declared_token_out_of_the_standard`
- `fails_hard_when_the_standard_cannot_be_parsed_rather_than_reporting_an_empty_palette`
- `declares_no_chromatic_accent`
- `carries_a_value_for_both_polarities_of_every_colour_role`

The gate

- `passes_on_the_explainer_as_shipped`
- `fails_a_surface_using_a_colour_the_standard_does_not_declare`
- `fails_a_declared_token_matching_a_recorded_source_fingerprint`
- `names_the_offending_value_and_the_file_it_is_in`
- `runs_with_no_model_and_no_network`

The rendered surface

- `renders_the_explainer_using_only_declared_tokens`
- `keeps_the_four_sections_and_their_order_unchanged`
- `states_which_rules_do_not_reach_terminal_output`

## Reproducibility

- `go test ./...`
- `mf check design` against this repository, which must pass.
- `mf check`, which must report seven gates.
- The source entry is `design-md/warp/DESIGN.md` in `voltagent/awesome-design-md`, read
  on 2026-08-24 at the repository's `main`. It carries no version tag, so a later read
  may differ; the fingerprints in `design.md` are of what was read on that date.

## Risks and Assumptions

- **The fingerprint check proves non-identity of values, not independence of design.**
  Direction is not a value: a layout, a restraint, a decision to carry no accent cannot
  be fingerprinted, and this gate would pass a page that reproduced all of them. It
  makes one specific failure — shipping the source's literal colours or typefaces —
  impossible, and claims nothing beyond that.
- **A short hex string is trivially brute-forced, so the fingerprints conceal nothing.**
  They are not a privacy measure. They exist so this repository can verify non-identity
  without carrying a readable copy of another project's palette as content.
- **The source has no version.** It is a `main`-branch file that may be rewritten or
  removed. The fingerprints then describe a document that no longer exists, and the
  recorded read date is the only thing that makes that legible.
- **The gate reads CSS with a regular expression, not a parser.** It finds colour
  literals in the shapes this repository writes them in. A colour expressed some other
  way — a named colour, an `oklch()` call, a computed value — would pass unnoticed, so
  the standard forbids those forms rather than the gate pretending to catch them.
- **This is the least load-bearing standard in the repository.** It governs one
  generated page. It is defensible only because the alternative was leaving the values
  that page already uses recorded nowhere, which is the drift the framework exists to
  prevent.
