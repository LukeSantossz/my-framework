# Token Economy

Controlled token consumption for this framework. Defines where output compression
is applied, where it is forbidden, and how it interacts with the rest of the standards.

The framework loads `CLAUDE.md` plus the standards documents as input on every
session, on top of the Superpowers orchestrator's own context. The dominant
recurring cost is this input, not conversational output. This norm targets that
first and treats conversational terseness as a secondary, bounded measure.

## Tool

Caveman (skill / rule set for coding agents) supplies the compression this norm
governs. Two capabilities are in scope, and only one of them exists today:

- Caveman terse mode: makes the agent's conversational replies drop filler. This
  is what the installed skill provides, and it is the whole of what it provides.
- `caveman-compress`: would rewrite `CLAUDE.md` into a compressed form, cutting
  input tokens every session while preserving code, URLs, and paths byte-for-byte.
  The installed Caveman does not have it. The skill is a conversational mode: it
  never mentions `CLAUDE.md`, context files, or byte-for-byte preservation, and
  nothing in it rewrites a file. `caveman-compress` is therefore a capability this
  policy scopes for the day it arrives, not one the framework can invoke now.

The distinction matters because the policy below is written against both: §1
describes what compressing the context file requires *if* an adopter opts into it
and a capability exists to do it, while §2 governs the terse mode that is real and
available. Naming a capability the framework does not have is only honest while the
document says plainly that it does not have it.

## Policy

### 1. Compress the context file (opt-in)

Compressing the context file is not a default this framework imposes. It is an
opt-in the adopter chooses when initializing the framework in a project, and a
repository that never opts in is fully conformant: it is not carrying a violation,
it simply declined a cost optimization it was free to decline. The rules below
govern the compression when it is chosen; they assert nothing about a repository
that has not chosen it.

- The capability the opt-in depends on is `caveman-compress`, and it does not exist
  in the installed Caveman (see Tool, above). The declared fallback, which is
  today's situation in every repository including this one, is that the context file
  is not compressed and stays in full prose. That is the conformant resting state,
  not a defect to be fixed by hand.
- When the opt-in is taken and a capability exists to honor it, `CLAUDE.md` is
  maintained in its compressed form, and that compressed file is the committed
  source of truth the agent loads.
- Compression must preserve, byte-for-byte: standards paths (`docs/standards/...`),
  the precedence reference to `code_conventions.md`, code blocks, and URLs.
- After compressing, verify the standards still resolve: the agent must still read
  `INDEX.md` and treat the precedence order as binding. A compression that breaks
  activation (the Gap this framework closes) is rejected.
- Keep a human-readable copy if the compressed form is hard to review (e.g.
  `CLAUDE.full.md`), but only the loaded file needs to be compressed.

The two preservation rules above are the reason this policy does not invite a
hand-rolled substitute. Compressing the context file by hand, with no skill to
repeat the transformation and no check to prove it preserved activation, trades a
cost saving for the exact failure the framework exists to prevent.

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
