# AI Guidelines

AI assistant behavior when generating or modifying code. Counterpart to `crura_method.md`. All output in English.

## Correctness Before Volume

- Do not invent APIs, functions, library methods, configuration keys, or CLI flags. If unsure a symbol exists, verify it or state it needs verification.
- Do not invent file paths, environment variables, or project structure. Inspect the actual project first.
- Treat memory of library or framework behavior as potentially stale; flag it rather than asserting confidently.
- Prefer a smaller correct answer over a larger speculative one.

## Specify Before Building

- For any non-trivial change, produce a `SPEC.md` per `spec_method.md` and pass the Spec Gate before writing code. Skip the spec only for changes too small to have a design.
- Do not enter the implementation Plan until the spec passes the Gate: Problem stated, Scope filled (including a non-empty "Does NOT include" list), and at least one verifiable Acceptance Criterion.
- Each Acceptance Criterion becomes a failing test in the Plan before its implementation.

## Declare Assumptions

- If a request is ambiguous in a way that changes the output, state the assumption in one line and proceed. Ask only when a wrong assumption is costly to reverse.
- If the task needs context you lack, request it rather than guessing.
- Surface non-obvious trade-offs and discarded alternatives. Record them in the spec's Alternatives Considered section.

## Match the Existing Codebase

- Read the surrounding code before adding to it. Mirror its patterns for naming, error handling, logging, config, structure.
- Do not introduce a new library, pattern, or abstraction when an established one exists.
- Follow `code_conventions.md` and `var_method.md`. Existing patterns outrank any default preference.

## Scope Discipline

- Change only what the task requires. Do not refactor, reformat, or improve unrelated code.
- Do not add features, options, or configuration the user did not ask for.
- Do not leave dead code, commented-out blocks, debug prints, or TODO placeholders in delivered code.
- When editing, return the minimal changed region with enough context to locate it, not the entire file unless asked.

## No Fabricated Evidence

- Never invent benchmark numbers, test results, metrics, or citations. Populate a Results section only with real, defensible data.
- Do not claim code was tested if it was not. Distinguish "should work" from "verified".
- Every reported number must carry the means to reproduce it: the exact command, the seed if randomness is involved, and the relevant versions. Record these in the spec's Reproducibility section.

## Self-Review Before Delivering

- Does it run, compile, or type-check as written, given the stated context?
- Are all referenced symbols, imports, and paths real?
- Are inputs validated at boundaries and errors handled, not swallowed?
- Are there leftover debug statements, secrets, or commented-out code?
- Does it follow `code_conventions.md` and `var_method.md`?
- Is the change scoped to the request and to the spec, with no unrequested edits?
- Was each Acceptance Criterion met, and does the test that covers it pass?

## Test-First Order

- Write the test before the implementation: red (test fails), green (minimal implementation passes), refactor.
- An implementation commit without a preceding failing-test commit is a process violation.
- This order is enforced by the Superpowers orchestrator's TDD phase; this section records it as project policy so it is auditable independently of the tool.

## Review Composition

Reviews compose; they do not replace one another. Three layers can run on a change,
with a defined hierarchy so none is duplicated or skipped:

- R1, internal review: the Superpowers two-stage subagent review (same provider, Claude). It applies `code_conventions.md`, `var_method.md`, and this file, and stands in for the Author's Self-Review. Record that it ran; do not repeat it manually.
- R2, cross-provider review: a Reviewer model from a provider different from the Author (e.g. the Codex pre-commit gate; the operational gate is defined in `codex_review.md`). If the pre-commit reviewer is a different provider than the Author, the cross-provider requirement is satisfied and no third reviewer is required for that purpose.
- R3, automated PR review: any automated PR reviewer (e.g. CodeRabbit). It is additional signal and does not substitute for R2.

When no second-provider tool is available, R1 plus the human PR review (per `crura_method.md`) stand in for R2; note its absence in the PR.

## Cross-Provider Review

Two roles: the Author model develops the code; the Reviewer model, when a second provider is available, reviews it before the PR. This is layer R2 above.

- The Author completes Self-Review above, then writes the change.
- When a second provider is available, route the diff to the Reviewer model before requesting human review. The Reviewer must be a different provider than the Author.
- The Reviewer applies the same standards (`code_conventions.md`, `var_method.md`, this file) and reports: correctness defects, invented or unverified symbols, scope creep, security issues, convention violations.
- The Reviewer does not rewrite the code; it reports findings. The Author resolves them or justifies the decision in the PR.
- Record in the PR which model authored and which reviewed.
- When no second provider is available, the Author's Self-Review and the human PR review (per `crura_method.md`) stand in for this step; note its absence in the PR.
- A Reviewer finding is advisory, not binding, but an unresolved finding must be addressed or justified, never silently dropped.

## Commits, PRs, and Issues

- Follow `github.md`: Conventional Commits format, imperative subject, no co-author or AI-attribution lines in commit messages.
- A commit message describes the change and its intent, not that an AI produced it.
- When opening a PR or issue, fill template sections with real content, not unedited placeholders.

## Safety

- Do not generate code whose primary purpose is harm (malware, credential theft, unauthorized access, surveillance).
- Do not hardcode secrets or weaken security controls (disabled TLS verification, permissive CORS, raw SQL string interpolation) as a shortcut. If a quick-and-insecure path is the only option, say so explicitly and mark it.
