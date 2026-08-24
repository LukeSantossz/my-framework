# Design Standard

What the surfaces this framework renders look like, in the `DESIGN.md` format —
a plain-text design system a person or an agent reads before producing UI.

This binds one page today: the CRUX explainer that `mf explain` generates. It is
the least load-bearing standard here, and it exists for one reason: the values
that page used were chosen while writing the renderer and recorded nowhere, so
nothing could tell a decision from a drift.

## Derivation

The format is the Stitch `DESIGN.md` layout collected in
`https://github.com/voltagent/awesome-design-md`. The design direction is derived
from that collection's Warp entry (`design-md/warp/`, read at `main` on 2026-08-24;
the entry carries no version, so a later read may differ).

Warp is a terminal product. Its visual language was formed under the constraints
this framework's surfaces already have — monospace, dark-first, a column of text
— which is why it was chosen over an entry whose page merely looks similar.

What is derived is direction. What is authored here is every value:

| Taken as direction | Why it applies |
|---|---|
| The neutral carries warmth; the identity is the temperature, not the darkness | Both of this framework's surfaces are long stretches of text, where a cold neutral reads as clinical |
| No chromatic brand accent — a near-white does the work a brand colour usually does | A tool whose premise is that the provider is configuration must not wear a vendor's accent |
| Very tight corner radii; never a pill for an action | Matches a surface made of rules and blocks rather than cards |
| Three faces laddered: prose, interface, code | The explainer is prose with code in it, which is exactly that ladder |
| No gradient, no shadow, no illustration system | Already true of the explainer; this makes it a rule instead of an accident |
| Semantic colour is in-product only, never part of the brand | Lets the quiz mark right and wrong without inventing a brand palette |

**Nothing is copied.** Every value below was authored for this project, and
`mf check design` refuses any token matching a recorded fingerprint of the
source's identity-carrying values. That check proves one thing: the literal
colours and typefaces of the source are not in this document. It cannot prove
independence of design — direction is not a value, and a restraint cannot be
fingerprinted. The fingerprints also conceal nothing; a six-digit hex is
trivially brute-forced. They exist so this repository can verify non-identity
without carrying a readable copy of another project's palette as content.

## 1. Visual Theme & Atmosphere

Document first. A page here is something a reader stays inside for twenty
minutes, not something that converts them. Warm neutral ground, ink-weight text,
rules instead of cards, and no ornament that is not carrying information.

The page must read the same in both polarities. Light is warm paper, dark is
warm charcoal; neither is the "real" one with the other as an afterthought.

## 2. Color Palette & Roles

No chromatic accent exists. Emphasis is carried by weight, rule and underline.

<!-- mf:design tokens -->
```toml
# Every colour the rendered surfaces may use. A value not in this table is a
# gate failure, because a colour nobody declared is a colour nobody decided.

[color.canvas]
role  = "the page ground"
light = "#faf8f4"
dark  = "#1a1815"

[color.canvas-soft]
role  = "inset surfaces: code blocks, callouts, quiz cards"
light = "#f0ece4"
dark  = "#262320"

[color.hairline]
role  = "1px rules and borders"
light = "#ddd6ca"
dark  = "#35302a"

[color.ink]
role  = "primary text, headings, links"
light = "#1c1a17"
dark  = "#f2eee6"

[color.body]
role  = "secondary text"
light = "#423d36"
dark  = "#cfc7b9"

[color.mute]
role  = "lowest-priority text: attribution, notes, scores"
light = "#6f675c"
dark  = "#948b7d"

# Semantic, in-product only. It is never brand colour and never appears outside
# a control that reports a result. It is also never the only signal: a glyph or
# a word carries the same meaning, so the state survives a reader who cannot
# separate the two hues.
[color.correct]
role  = "a quiz answer that was right"
semantic = true
light = "#3f6b4a"
dark  = "#7fae89"

[color.incorrect]
role  = "a quiz answer that was wrong"
semantic = true
light = "#9b3b32"
dark  = "#d98d84"

[typeface]
prose     = "ui-serif, Georgia, 'Times New Roman', serif"
interface = "ui-sans-serif, system-ui, 'Segoe UI', Arial, sans-serif"
code      = "ui-monospace, 'Cascadia Mono', Consolas, monospace"

[scale]
radius-sm = "3px"
radius-md = "4px"
radius-lg = "6px"
step-1    = "2px"
step-2    = "4px"
step-3    = "8px"
step-4    = "12px"
step-5    = "16px"
step-6    = "24px"
step-7    = "32px"
step-8    = "48px"

# One-way digests of the identity-carrying values in the source entry, so the
# gate can prove this document does not reuse them without reproducing them.
# Colours are normalised to lowercase hex; typefaces to a lowercase family name.
[source]
name = "voltagent/awesome-design-md design-md/warp"
read = "2026-08-24"
algorithm = "sha256, first 16 hex characters"
fingerprints = [
  "d474b848022c7e18", "7cd476046908e830", "9df2ef808c1070d2",
  "3ae517b2bc43e8d2", "dfd9c98a19a8a431", "423f581b4508b54e",
  "4bee74e299680c35",
  "c84c8016356014e0", "16fdb01dd000cfcd", "af213fd9ef9f40cf",
  "324bb56ef283edb7",
]
```

## 3. Typography Rules

Three faces, three jobs. All of them are system stacks: the explainer is a
single self-contained file that may not reach a font host, so a webfont is not
available to it and a face that must be downloaded is not a face this framework
has.

- **Prose** — the serif stack, for everything a reader reads in sentences. Body
  is 16px at a 1.65 line height; long-form reading is what the page is for.
- **Interface** — the sans stack, for headings, navigation, metadata and
  controls. It separates the page's own voice from the change being explained.
- **Code** — the mono stack, for `<pre>`, inline code and identifiers, at 0.85em
  so a code block does not tower over the prose around it.

Headings are set at weight and size only. No letterspacing tricks, no small
caps, no uppercase headings.

## 4. Component Stylings

- **Section heading** — interface face, a hairline rule above, generous space
  before and little after, so a heading belongs to what follows it.
- **Code block** — `canvas-soft` fill, hairline border, `radius-lg`, its own
  horizontal scroll. It never makes the page scroll sideways.
- **Callout** — `canvas-soft` fill with a hairline rule on the leading edge.
  Square on that edge, `radius-lg` on the other.
- **Details / summary** — the interface face, an ink-coloured summary. Used for
  anything skippable, which is how the deep background is offered.
- **Quiz card** — hairline border, `radius-lg`. Options are full-width buttons
  with a hairline border that changes to the semantic colour once answered,
  alongside a mark that says the same thing without colour.

## 5. Layout Principles

One column, `46rem` at most, centred. A reading measure is the whole layout;
there is no grid, no sidebar and no second column.

Vertical rhythm comes from the spacing scale. Space above a heading is larger
than the space below it.

## 6. Depth & Elevation

There is none. No shadow, no blur, no gradient, no overlay, no z-index. Surfaces
are separated by fill and hairline. A page that never lifts anything never has
to decide what floats above what.

## 7. Do's and Don'ts

- **Do** use a declared token for every colour.
- **Do** carry meaning in weight, rule, spacing and wording before reaching for
  a hue.
- **Do** give both polarities a real value, defined together.
- **Don't** introduce a chromatic accent. There is no brand colour here, and a
  page that borrowed one from whichever vendor produced its content would be
  advertising that vendor.
- **Don't** use a semantic colour as the only signal for a state.
- **Don't** write a colour as a named CSS colour, an `oklch()` call, or anything
  the gate cannot read. The gate finds hex and `rgb()` literals; a colour hidden
  from it is a colour that escaped the standard.
- **Don't** load a font, a stylesheet, an image or a script from a URL.

## 8. Responsive Behavior

The column is capped, not fixed, so a narrow screen simply gets a narrower
measure. Padding shrinks; type does not. Wide content — code blocks, tables —
scrolls inside its own box, and the page body never scrolls horizontally.

## 9. Agent Prompt Guide

When generating or changing a surface this standard governs:

> Use only the colours declared in `docs/standards/design.md`, by role, and give
> both polarities a value. There is no accent colour: carry emphasis with weight,
> rule and underline. Use the three declared typeface stacks for prose, interface
> and code respectively; load nothing from a URL. Radii are 3, 4 or 6 pixels.
> There is no shadow, gradient or overlay. One column, capped at a reading
> measure. Semantic colour appears only on a control reporting a result, and
> never as that result's only signal.

## What This Does Not Reach

**The status line.** `status_line.md` binds the five facts and their order and
deliberately binds neither colours nor glyphs, because Codex's segments are not
restyleable and the reader's theme owns the terminal. A design standard that
reached it would be overruling a standard rather than decorating a surface.

**Terminal output generally.** A terminal has no fonts, no radii, no elevation
and no surfaces, and its palette belongs to the reader's theme. Of the rules
above, exactly two reach it: emphasis is carried by structure and wording rather
than hue, and colour is never the only signal for a state. `NO_COLOR` is honoured
wherever this framework writes colour at all. Everything else in this document
is about a page.

Recording that boundary is the point. A design standard that implied it governed
the terminal would be describing behaviour nothing performs, which is the
failure this framework exists to fix.
