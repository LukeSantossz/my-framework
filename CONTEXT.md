# my-framework

The domain language of a development-standards framework for AI-oriented work: the
roles that produce and review a change, the review layers they compose, the design
gate that precedes code, and the named methods and token rules that govern it all.
This glossary is the source of truth for what each term means; it holds no
implementation detail.

## Language

### Actors

**Developer**:
The human who owns the project — directs the Author, approves the Spec Gate, and
performs the CRURA Review. The Developer, not any model, is accountable for what ships.
_Avoid_: author (reserved for the model), maintainer, engineer, user (the end consumer
of the software being built).

**Author**:
The model that writes the change under the Developer's direction. One half of the
Cross-Provider Review pair. Which model and Provider it is belongs to the change, not to
the session or the push, and is fixed by the Author Declaration.
_Avoid_: coder, generator, assistant, the AI; naming a specific vendor as the definition.

**Author Declaration**:
The record of the Author's Provider and model, written per branch while the change is
being authored. It exists because a push carries commits that may come from several
sessions, several agents, or a person typing, so there is no single Author to detect at
push time. It is what the Cross-Provider State is evaluated against.
_Avoid_: session detection, fingerprint (the corroborating signal, not the record).

**Reviewer**:
The model that performs the R2 Cross-Provider Review, drawn from a different Provider
than the Author. Which one it is varies per run: the R2 Gate walks a chain of reviewer
backends and takes the first available, so the Reviewer is whichever backend actually ran
and is named as such in the PR. Reports findings; never rewrites the code.
_Avoid_: checker, validator, critic.

**Provider**:
The vendor behind a model (e.g. Anthropic, OpenAI). R2 is satisfied only when the
Reviewer's Provider differs from the Author's, as resolved by the Cross-Provider State.
_Avoid_: vendor, platform, model.

**Backend**:
A named way of performing a Role — an agentic CLI, an HTTP endpoint, an in-session
skill, or a deterministic in-process check. A Backend declares its own Provider and
classifies its own tool's failures, which is what lets a chain tell "unavailable" from
"reviewed with findings".
_Avoid_: adapter (the script implementing one), reviewer (the role it may fill), plugin.

**Role**:
A job the framework needs performed — Author, R1, R2, R3 — bound to an ordered chain of
Backends. The Role is the unit of configuration; the Backend is the unit of execution.
_Avoid_: layer (R1–R3 as review stages), phase, agent.

### Review

**Review Composition**:
The rule that the review layers compose rather than replace one another, so none is
duplicated or skipped. Spans the automated layers R1–R3 and the Developer's CRURA Review.
_Avoid_: review stack, review pipeline.

**R1 / Internal Review**:
The automated internal review, satisfied by a chain of Backends and named by whichever
one ran. No Provider constraint applies: what defines R1 is when it runs and how much of
the change it sees, not whom it shares a Provider with. Stands in for the Author
Self-Review. Superpowers is one Backend of this chain, never the layer itself.
_Avoid_: self-review, internal QA, same-provider review, the Superpowers pass.

**R2 / Cross-Provider Review**:
The automated review by the Reviewer, whose Provider differs from the Author's
(operationally the pre-push backend chain). Only valid across Providers, and only as
strongly as the Cross-Provider State reports.
_Avoid_: external review, second opinion.

**Cross-Provider State**:
How strongly R2's Provider requirement is established for a change: `verified` when an
independent signal agrees with the Author Declaration and differs from the Reviewer's
Provider, `declared` when only the Declaration asserts it, `unknown` when nothing
recorded it. `unknown` does not satisfy R2. The state is recorded in the PR beside the
Backend and model, because a reader adjudicating a PR needs to tell an enforced claim
from an asserted one.
_Avoid_: verified/unverified as a binary, cross-provider flag.

**R2 Gate**:
The automated push-time hook that runs the Reviewer against the base branch — the
operational form of R2; a.k.a. the pre-push gate. Advisory by default (findings are
surfaced, the push is not blocked unless blocking mode is on).
_Avoid_: gate (unqualified), the hook (alone), pre-commit gate.

**R3 / Automated PR Review**:
An automated reviewer that runs on the Pull Request (e.g. CodeRabbit). Additional
signal; never a substitute for R2.
_Avoid_: bot review, CI review.

**CRURA Review**:
The Developer's human review track — Change, Review, Upload, Review Again — feeding the
PR Review Checklist. Its adjudication of the recorded layers and its merge decision run
on every change; its line-by-line reads run when a Review Trigger fires or the change is
drawn as the Untriggered Sample. Distinct from the automated R-layers; its adjudication
stage also substitutes for R2 when no second Provider is available.
_Avoid_: manual review, human QA, R4, always-on track.

**Review Trigger**:
A named condition that makes a CRURA line-by-line read owed for a change — a security or
blocking finding, a failing deterministic check, a high-risk path, a `unknown`
Cross-Provider State, or no layer having run. Enumerated in `crura_method.md`, because a
trigger set left to judgment degrades into never.
_Avoid_: heuristic, threshold, rule (unqualified).

**Untriggered Sample**:
The periodic draw of changes that no Review Trigger selected, read and recorded anyway.
It exists because instrumenting only triggered reads observes the population already
suspected, which can confirm the trigger set but never correct it. The sample is what
makes the evidence able to falsify the triggered regime.
_Avoid_: spot check, audit, random review.

**Self-Review**:
The Author's pre-delivery checklist over its own change (does it run, are all symbols
real, are inputs validated, is scope honored). R1 stands in for it.
_Avoid_: self-check; not the PR Review Checklist (a different artifact, different actor).

**PR Review Checklist**:
The checklist the Developer completes in the Pull Request — confirming the change
matches its approved spec and recording which review layers ran, with the Author and
Reviewer models named. Fed by CRURA Review.
_Avoid_: Self-Review Checklist, PR self-review.

### Specification

**Brainstorm**:
The phase before the SPEC.md that refines requirements into a draft spec, resolving
architectural decisions while they are still cheap to change. A Superpowers phase the
SPEC Method builds on.
_Avoid_: discovery, ideation, scoping.

**SPEC.md**:
The one-per-change design artifact (Problem, Design Decision, Alternatives Considered,
Scope with a mandatory "Does NOT include", Acceptance Criteria, Reproducibility, Risks
and Assumptions), authored directly under `docs/specs/NNNN-<slug>.md` and archived
there once approved — its durable home; later changes get their own numbered file. The
source of truth for a change's intent and scope; code that contradicts it is wrong
even if it works.
_Avoid_: design doc, RFC, ticket, PRD.

**Spec-lite**:
The lighter SPEC.md tier for a change with no Design Decision worth recording — it
keeps exactly the three Gate-checked sections (Problem, Scope, Acceptance Criteria). A
spec that turns out to need Alternatives Considered after all is full-tier instead.
_Avoid_: mini-spec, spec stub.

**Spec Gate**:
The human checkpoint between design and implementation. The Developer approves the
SPEC.md here — Problem stated, Scope filled with a non-empty "Does NOT include", at
least one verifiable Acceptance Criterion — before any code is written.
_Avoid_: gate (unqualified), design review, sign-off.

**Acceptance Criterion**:
A verifiable outcome stated in the SPEC.md, phrased as a test result
(returns_empty_list_when_no_matches). Each becomes a failing test before its
implementation.
_Avoid_: requirement, success metric, definition of done.

**Plan**:
The gated build phase after the Spec Gate: it turns each Acceptance Criterion into a
failing test, then the minimal implementation that passes it (red-green-refactor).
Never entered until the spec passes the Spec Gate. A Superpowers phase.
_Avoid_: the plan (generic), roadmap, implementation plan.

### Decision records

Three artifacts record a decision's rationale, in one flow: SPEC (gate-approved
intent, archived under `docs/specs/`) → ADR (curated rationale) → README (curated
index). Each has a distinct audience and lifetime; rationale is authored once, in
the ADR, and referenced — never restated.

**Alternatives Considered**:
The SPEC.md section listing at least two rejected approaches for the current change,
each with its rejection reason, recorded before code. Preserved in the durable spec
archive under `docs/specs/`, but not the curated decision record — a qualifying
decision is still promoted to an ADR at the Spec Gate, which stays the curated home
for an outside reader.
_Avoid_: Engineering Decisions (the durable after-code counterpart), options, trade-offs.

**ADR** (Architecture Decision Record):
A numbered, durable record under `docs/adr/` of a single decision that is hard to
reverse, surprising without context, and the result of a real trade-off. The permanent
home for a decision's "why"; promoted from a SPEC's Design Decision at the Spec Gate, or
later if its significance only emerges during implementation.
_Avoid_: design doc, decision log, RFC.

**Engineering Decisions**:
The curated README section that indexes the project's most significant decisions for an
outside reader — each row links the ADR holding the full rationale, rather than
restating it. The after-code, product-facing face of decisions the ADRs already record.
_Avoid_: Alternatives Considered (the before-code SPEC counterpart), changelog.

### Standards & Methods

**Standard**:
A binding document under `docs/standards/` that the Author and Developer must follow;
the whole set is the framework's "Development Standards." Conflicts between Standards
resolve by the precedence order in `code_conventions.md`.
_Avoid_: norm, guideline, rule, policy (as the umbrella term).

**Method**:
A named discipline original to this framework, identified by an acronym — SPEC (design
before code), VAR (naming suffixes), CRURA (human review), CRUX (review explanation). A subtype of Standard; the
other Standards (conventions, AI guidelines, GitHub, token economy) are not Methods.
_Avoid_: process, methodology, framework.

**VAR Method**:
The framework's naming-suffix guide and the lowest layer of naming precedence: Data
(raw payloads/attributes), Info (processed/descriptive/config), Manager (orchestrating
classes — use sparingly), Handler (event-reacting functions — use sparingly). Apply a
suffix only when it names the real responsibility; drop it when a specific name is clearer.
_Avoid_: Hungarian notation, type prefixes.

**CRUX Method**:
The review-time discipline that explains an implemented change with a transient,
interactive HTML explainer (Background, Intuition, Code, Quiz) generated outside
version control, to make comprehension a produced, checkable outcome before
review. An aid feeding R1 and the CRURA Review — never a review layer and never
a blocking gate; durable rationale stays in ADRs. Acronym: Change Review
Understanding eXplanation.
_Avoid_: explainer (the artifact, not the method), code walkthrough, tutorial.

**Status Line Contract**:
The Standard fixing which five facts a coding agent's status line shows and in what
order — model with reasoning effort, context used, tokens spent, quota, location —
across Claude Code and Codex. It binds the facts and their order, never the colors,
glyphs, or widths, because the two tools render through mechanisms that are not
interchangeable: Codex reads a declarative segment list, Claude Code runs a command.
Applying it is machine state and therefore opt-in.
_Avoid_: status bar, statusline (as the Standard rather than the rendered line), theme.

### Token Economy

**Token Economy**:
The Standard governing controlled token consumption — compress the loaded context file
(opt-in, chosen at initialization), allow bounded Terse mode in conversation, forbid
terseness in versioned artifacts. Sits below Safety and Correctness and never justifies
skipping a required step. A repository that never opts in is conformant.
_Avoid_: token budget, cost control, always on.

**caveman-compress**:
The not-yet-available capability the Token Economy's compression opt-in would use: it
would rewrite `CLAUDE.md` into a compressed, loaded form, preserving standards paths,
code blocks, and URLs byte-for-byte, targeting input cost — the dominant recurring cost.
The installed Caveman does not provide it, so the context file stays uncompressed.
_Avoid_: minify, summarize; naming it as a capability the framework has today.

**Terse mode**:
Caveman's conversational compression — drops filler from the agent's replies. A
communication style only; never a reason to do less work, and forbidden in SPEC.md, PR,
Issue, and commit artifacts.
_Avoid_: brief mode, caveman mode (ambiguous with the tool).

### External dependencies

**Caveman**:
External skill / rule set for coding agents that compresses the agent's output.
Supplies Terse mode to the Token Economy; it does not supply caveman-compress, so the
Token Economy's context-file compression has no capability behind it today.
_Avoid_: compressor, minifier.

**Superpowers**:
External orchestrator that runs the Brainstorm, Plan, and TDD phases and the two-stage
subagent review that is R1.
_Avoid_: the orchestrator (alone).

### Cross-cutting

**the Gap**:
The problem this framework exists to close — Standards that are written but never
actually activated (loaded and obeyed) by the agent. A change that breaks activation
(e.g. a compression that stops the agent reading `INDEX.md`) reopens the Gap and is rejected.
_Avoid_: the problem, the bug.

**Type Table**:
The single canonical Conventional Commits type vocabulary in `github.md` (feat, fix,
docs, style, refactor, perf, test, chore, build, ci, revert). Commits, PR titles, issue
titles, and branch names all draw from it; no parallel list exists.
_Avoid_: commit types list, type enum.
