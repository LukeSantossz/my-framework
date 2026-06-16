# Token Economy

Controlled token consumption for this framework. Defines where output compression
is applied, where it is forbidden, and how it interacts with the rest of the standards.

The framework loads `CLAUDE.md` plus the standards documents as input on every
session, on top of the Superpowers orchestrator's own context. The dominant
recurring cost is this input, not conversational output. This norm targets that
first and treats conversational terseness as a secondary, bounded measure.

## Tool

Caveman (skill / rule set for coding agents): compresses agent output and can
rewrite context files into a terser form. Two capabilities are in scope here:

- `caveman-compress`: rewrites `CLAUDE.md` into a compressed form, cutting input
  tokens every session while preserving code, URLs, and paths byte-for-byte.
- Caveman terse mode: makes the agent's conversational replies drop filler.

Reported reductions (author benchmarks and third-party tests, treated as orders of
magnitude, not guarantees): compress ~46% of input tokens per session; terse mode
50–75% of conversational output. Terse mode does not affect thinking or generated
code, so per-session savings are smaller than the headline output figure.

## Policy

### 1. Compress the context file (default, always on)

- `CLAUDE.md` is maintained in its `caveman-compress` form. The compressed file is
  the committed source of truth the agent loads.
- Compression must preserve, byte-for-byte: standards paths (`docs/standards/...`),
  the precedence reference to `code_conventions.md`, code blocks, and URLs.
- After compressing, verify the standards still resolve: the agent must still read
  `INDEX.md` and treat the precedence order as binding. A compression that breaks
  activation (the Gap this framework closes) is rejected.
- Keep a human-readable copy if the compressed form is hard to review (e.g.
  `CLAUDE.full.md`), but only the loaded file needs to be compressed.

### 2. Terse conversational mode (permitted, bounded)

- Terse mode is allowed for conversational replies, status updates, and
  explanations during a session.
- It is the agent's communication style, never a reason to do less work or skip a
  required step.

### 3. Where terse mode is forbidden (hard boundary)

Terse mode never applies to versioned or review artifacts. These follow their own
templates in full prose, regardless of any active compression level:

- `SPEC.md` and the Spec Gate (`spec_method.md`): all sections filled with real content.
- Pull Request body and PR Review Checklist (`github.md`): templates filled, not abbreviated.
- Issue body (`github.md`): structured sections, not one-liners.
- Commit messages: Conventional Commits format and imperative subject as specified;
  Caveman's terse-commit helper is acceptable only if its output still satisfies
  `github.md` (type, scope, imperative subject) and adds no co-author or
  AI-attribution line.
- Code comments: the "why, not what" rule stands; terseness does not license dropping
  a comment that explains non-obvious intent.

If a compression or terse setting would empty or degrade any of these artifacts,
the artifact requirement wins. This is a Correctness-level concern in the
`code_conventions.md` precedence order, above the naming and style layers.

## Precedence

This norm sits below Safety and Correctness. Token economy never justifies:

- producing an incomplete or unverifiable spec, PR, or issue;
- skipping the test-first order or the review layers;
- omitting error handling, validation, or security controls to save tokens.

When token economy conflicts with any of the above, the other rule wins.

## Out of Scope (roadmap, not adopted)

The following are deliberately not adopted yet; adopt only if measured consumption
shows the corresponding bottleneck:

- Shell-output compression (e.g. RTK): adopt if sessions push large volumes of CLI
  output into context.
- Codebase knowledge graph (e.g. CodeGraph / Codebase Memory): adopt for large,
  mature repositories where file-reading for code discovery dominates cost.

Measure before adding either. Do not stack tools on headline percentages alone.
