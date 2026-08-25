# AI Guidelines

AI assistant behavior when generating or modifying code. Counterpart to `crura_method.md`. All output in English: identifiers, comments, commit/PR/issue text, documentation.

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

- If a request is ambiguous in a way that changes the output: state the assumption in one line and proceed when it is cheap to reverse; ask one focused question first when a wrong assumption is costly to reverse.
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
- This order is run by the Superpowers orchestrator's TDD phase when that plugin is installed, but the phase is not what makes it binding; `skills_guidelines.md` records whether the plugin is present and declares the fallback. This section records the order as project policy so it holds either way, and stays auditable independently of the tool.

## Review Composition

Reviews compose; they do not replace one another. Three layers can run on a change,
with a defined hierarchy so none is duplicated or skipped:

- R1, internal review: satisfied by a chain of review backends, taking the first one that is actually available and naming it. It applies `code_conventions.md`, `var_method.md`, and this file, and stands in for the Author's Self-Review. No provider constraint applies: each backend declares its own provider, and R1 is defined by when it runs and what it sees, not by whom it shares a provider with. Record which backend ran; do not repeat it manually.
- R2, cross-provider review: a Reviewer model from a provider different from the Author's (the operational gate is defined in `r2_gate.md`). The Author's provider is a recorded property of the change, not an assumption, so the requirement resolves to one of three states — `verified`, `declared`, or `unknown` — and only the first two can satisfy it.
- R3, automated PR review: any automated PR reviewer (e.g. CodeRabbit). It is additional signal and does not substitute for R2.

`superpowers` is one backend of the R1 chain, not the layer itself: it runs inside a coding-agent session and cannot be started as a subprocess, so it contributes an attestation rather than an execution, and a session where it is absent counts as unavailable so the chain advances.

When no second-provider tool is available, R1 plus the adjudication stage of CRURA (per `crura_method.md`) stand in for R2; note its absence in the PR. It is the adjudication that substitutes, not the line-by-line read, because adjudication is the part that runs unconditionally. This is the whole of the R2-absent fallback; it is stated once, here, and the Cross-Provider Review section below refers back to it rather than restating it.

The Author's Self-Review is not part of that substitution. R1 already stands in for it, per R1 above, so naming it again here would offer as a replacement the thing R1 replaced — leaving a change whose only human check is the one CRURA makes triggered.

The chain is walked by `mf review --role <r1|r2|r3>`, which takes the first backend that is actually available and names it. A role whose every backend is unavailable is reported as not having run, and that absence belongs in the PR: a layer that did not run is never the same as a layer that found nothing.

At review time an implemented change may also carry a transient CRUX explainer (see `crux_method.md`) that feeds R1 and the CRURA Review. It is an aid, not a review layer, and never blocks a ship.

## Cross-Provider Review

Two roles: the Author model develops the code; the Reviewer model, when a second provider is available, reviews it before the PR. This is layer R2 above.

- The Author completes Self-Review above, then writes the change.
- When a second provider is available, route the diff to the Reviewer model before requesting human review. The Reviewer must be a different provider than the Author.
- The Author's provider and model are declared when the change is authored, not inferred when it is pushed: a push carries commits that may come from several sessions or from a person typing, so it has no single Author to detect. The cross-provider claim is then `verified` when an independent signal agrees with the declaration and differs from the Reviewer's provider, `declared` when only the record asserts it, and `unknown` when nothing recorded it. A signal that contradicts the declaration is an error to resolve, never a preference to apply silently.
- The Reviewer applies the same standards (`code_conventions.md`, `var_method.md`, this file) and reports: correctness defects, invented or unverified symbols, scope creep, security issues, convention violations.
- The Reviewer does not rewrite the code; it reports findings. The Author resolves them or justifies the decision in the PR.
- Record in the PR which model authored and which reviewed.
- When no second provider is available, the fallback is the one stated in Review Composition above — R1 plus CRURA's adjudication stage — and the absence is noted in the PR.
- A Reviewer finding is advisory, not binding, but an unresolved finding must be addressed or justified, never silently dropped.

## Commits, PRs, and Issues

- Follow `github.md`: Conventional Commits format, imperative subject, no co-author or AI-attribution lines in commit messages.
- A commit message describes the change and its intent, not that an AI produced it.
- When opening a PR or issue, fill template sections with real content, not unedited placeholders.

## Safety

- Do not generate code whose primary purpose is harm (malware, credential theft, unauthorized access, surveillance).
- Do not hardcode secrets or weaken security controls (disabled TLS verification, permissive CORS, raw SQL string interpolation) as a shortcut. If a quick-and-insecure path is the only option, say so explicitly and mark it.
