# SPEC: feat(standards): add the CRUX review-explanation method

## Problem

A change that ships is reviewed against its diff, but the framework has no
discipline that guarantees the reviewer (and the Developer) actually understands
what the change does before approving it, so "we shipped it knowing what we did"
is an aspiration rather than a produced, checkable outcome.

## Design Decision

Add CRUX (Change Review Understanding eXplanation), a new Method under
`docs/standards/crux_method.md`. At review time, an implemented change is
explained by a transient, self-contained, interactive HTML artifact generated
outside version control. CRUX is an aid that feeds the existing R1 and CRURA
review layers; it is never a new review layer and never a blocking gate, and it
adds no versioned per-change record — durable rationale continues to live in
ADRs. When the generating skill is absent the Method degrades deliberately: the
reviewer reads the diff directly and the Pull Request notes the aid was absent,
mirroring the R2 Codex-absent fallback.

The artifact contract is embedded here so the reference folder `ex/` can be
deleted before implementation without losing the format:

- Output: a single self-contained HTML file with inline CSS and JavaScript,
  written outside the code repository, its filename prefixed with the current
  date in `YYYY-MM-DD-` format (for example `2026-07-13-explanation-<slug>.html`)
  so the files stay time-sorted and out of version control.
- Structure: one long page with section headers and a table of contents, no
  tabs for the top-level structure, with basic responsive styling.
- Sections, in order:
  - Background: the existing system relevant to the change, with a deep version
    for beginners (skippable if the reader is already familiar) followed by a
    narrow background directly relevant to the change.
  - Intuition: the core intuition, focused on the essence rather than full
    detail, using concrete toy-data examples and figures.
  - Code: a high-level walkthrough of the changes, grouped and ordered so they
    are understandable.
  - Quiz: five interactive multiple-choice questions that test real
    understanding of the change (not gotchas); clicking an option tells the
    reader whether they were correct and gives feedback.
- Diagrams: reuse a small number of diagram families across the explanation;
  useful kinds are a simplified rendering of the app UI for UI changes and a
  system diagram showing data flow between components with example data
  included. Never use ASCII diagrams — use simple HTML designs for diagrams and
  HTML lists for lists.
- Code blocks: always rendered with `<pre>` tags, or a styled `div` that sets
  `white-space: pre` or `pre-wrap`; each code block's CSS is confirmed before
  the file is saved so newlines are not collapsed.
- Callouts: used for key concepts, definitions, and important edge cases.
- Security: all change-derived text (diff content, code, identifiers) is
  context-appropriately escaped for its HTML, CSS, or JavaScript insertion
  point, and any generated URL is sanitized or allowlisted, so reviewed content
  cannot inject markup or scripts into the explainer.
- Prose: written in a clear, flowing classic style with smooth transitions
  between sections.

Three behaviors extend the reference format:

- Anti-slop: the explainer's prose is passed through the `humanizer` skill
  before the final render, to remove signs of AI-generated writing. This is a
  declared composition; if `humanizer` is absent the Method flags the AI-slop
  risk and requires a manual pass — never a silent skip.
- Quiz difficulty: the invocation accepts a `difficulty` of `easy`, `medium`,
  or `hard`, defaulting to `medium`.
- Wrong-answer remediation: when the Developer answers a quiz question
  incorrectly, the artifact reveals a deeper, better explanation of that concept
  before advancing, with a control that lets the Developer skip it and proceed
  if they already understand.

## Alternatives Considered

- Lightweight extension with no new Method (a paragraph in `ai_guidelines.md`
  plus a `skills_guidelines.md` entry). Rejected: it contradicts the Developer's
  explicit request for a named Method, and an unnamed review aid has no glossary
  identity or reading-order slot, weakening activation — the exact failure the
  framework exists to prevent.
- Vendoring the generating skill into the repository. Rejected: it breaks the
  framework's standing convention of documenting external skills while
  versioning only its own Bash and Markdown, and it adds in-repo skill
  maintenance and test surface for a generator whose HTML output is already kept
  out of version control, so vendoring buys little.
- Persisting a durable per-change explanation record in the repository.
  Rejected: the Developer ruled that explainers are reviews of already
  implemented code and need no persistence, ADRs already hold durable rationale,
  and a second durable record would duplicate the ADR and bloat the archive —
  the same disproportion the PR #10 curation removed.

## Scope

- Includes:
  - New standard `docs/standards/crux_method.md` defining the trigger, the
    embedded artifact contract, the placement in Review Composition, and the
    declared fallback.
  - A new section in `docs/standards/skills_guidelines.md` for the
    `explain-change` skill (pipeline stage, install/verify, fallback) recording
    the `humanizer` composition and its fallback.
  - Edits to `docs/standards/INDEX.md`: add `crux_method.md` to the Documents
    list and the Reading Order, and add one System Rule line.
  - A cross-reference in `docs/standards/ai_guidelines.md` Review Composition
    stating the explainer is an aid feeding R1 and CRURA, not a new R-layer and
    not a blocking gate.
  - A glossary term "CRUX Method" in `CONTEXT.md`, with its acronym expansion
    and an `_Avoid_` list.
  - Docs-consistency checks for the Acceptance Criteria below, added test-first.
  - Companion deliverable, out of the versioned scope: author and install the
    external `explain-change` skill at `~/.claude/skills/`, based on the
    reference `explain-diff-html` skill plus the three behaviors above; delete
    the untracked `ex/` folder before implementation.
- Does NOT include:
  - Persisting the explainer artifact in the repository; it stays transient and
    out of version control.
  - Any new durable per-change record; the ADR remains the durable rationale
    home.
  - Any build-time discipline; no `teach-as-you-build` loop and no overlap with
    the existing TDD stage.
  - Making the explainer a blocking review gate; it is advisory only.
  - Vendoring the skill file into the repository.
  - Any Redis, MiniRedis, or portfolio-class material from `AULA.md`.
  - Changing the definitions of R1, R2, R3, or CRURA; the Method only
    cross-references them.
  - Any README change beyond an optional Engineering Decisions row, added only
    if a decision is promoted to an ADR at the Spec Gate.

## Acceptance Criteria

- crux_method_standard_exists_and_indexed: `docs/standards/crux_method.md`
  exists and is listed in both the INDEX Documents section and the Reading
  Order, and the docs-consistency suite reports no orphaned or dangling
  standard.
- crux_skill_recorded_in_skills_guidelines: `skills_guidelines.md` contains an
  `explain-change` section with a pipeline stage, an install/verify entry, and a
  declared fallback.
- crux_glossary_term_present: `CONTEXT.md` defines the "CRUX Method" term.
- crux_is_advisory_not_blocking: the standard states in a grep-detectable form
  that the explainer is advisory and never blocks a ship.
- crux_declares_humanizer_pass: the standard mandates the `humanizer` pass on
  the prose, and `skills_guidelines.md` records the `humanizer` composition and
  its fallback.
- crux_quiz_difficulty_default_medium: the standard documents the `difficulty`
  parameter and its `medium` default.
- crux_wrong_answer_offers_skippable_remediation: the standard documents the
  wrong-answer deeper-explanation behavior and that it is skippable.
- docs_consistency_suite_green: `docs-consistency.test.sh`,
  `docs-consistency.sh`, `setup.test.sh`, and `codex-review.test.sh` all pass on
  the resulting tree.

## Reproducibility

Run, from the repository root, with git >= 2.40 and bash:

```sh
bash scripts/test/docs-consistency.test.sh
bash scripts/test/docs-consistency.sh
bash scripts/test/setup.test.sh
bash scripts/test/codex-review.test.sh
```

All four suites pass. The repository tests are deterministic and use no seed;
the quiz difficulty default is a fixed constant (`medium`). The external skill's
HTML generation is non-deterministic (model-driven) and is out of scope for
reproducibility beyond the documented contract, which the skill must honor.

## Risks and Assumptions

- Assumption: the `humanizer` skill is available where the explainer is
  generated; if it is absent, the Method's fallback (flag the AI-slop risk and
  require a manual pass) applies and is declared, not silent.
- Assumption: wrong-answer remediation triggers per incorrectly answered
  question, not on an aggregate score; this is cheap to reverse if the Developer
  meant the aggregate.
- Assumption: `0009` is the free spec number, since `0009` was retired by
  PR #10.
- Risk: the external skill is not covered by the repository's shell tests, so
  the three behavior requirements are guaranteed in-repo only as documentation
  invariants; actual behavior depends on the external skill honoring the
  contract. Mitigation: the standard states the contract explicitly and the
  skill is verified manually in the same session.
- Risk: adding a Method touches INDEX, `ai_guidelines.md`, and `CONTEXT.md`, and
  the docs-consistency invariants must move in lockstep or CI breaks.
  Mitigation: write the failing checks first (red-green).
- What would invalidate this spec: a later decision that explainers must persist
  as a durable record, which reopens the persistence decision and changes Scope.
