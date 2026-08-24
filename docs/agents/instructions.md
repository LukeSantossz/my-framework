# Agent Instructions

This is the single source for every vendor instruction file in this repository.
`CLAUDE.md`, `AGENTS.md` and any other are generated from it by `mf agents sync`,
each receiving only the sections for the roles that agent plays. Edit this file,
never the generated ones; `mf check agents` fails when they have drifted apart.

Before any work in this repository, read `docs/standards/INDEX.md` and the
documents it lists. Treat them as binding.

<!-- mf:role shared -->
## Standards are binding

- The precedence order in `docs/standards/code_conventions.md` is authoritative
  for resolving any conflict between rules.
- The approved spec under `docs/specs/` (per `docs/standards/spec_method.md`) is
  the source of truth for a change's intent and scope. Code that contradicts it
  is wrong even if it works.
<!-- The English rule is kept on one line on purpose: a guard pins this exact
     clause, and wrapping it would read as the rule having been dropped. -->
- All output in English: identifiers, comments, commit/PR/issue text, documentation.
- Follow `docs/standards/github.md` for Conventional Commits, branch naming, and
  the PR, Issue, and README templates. No co-author or AI-attribution lines in
  commit messages.
- Token economy per `docs/standards/token_economy.md`: terse mode is allowed in
  conversation but never in a spec, PR, Issue, or commit artifact; it never
  overrides Safety or Correctness.

<!-- mf:role author -->
## Your role as Author

You write the change under the Developer's direction.

- Specify before building: produce a spec per `docs/standards/spec_method.md` and
  pass the Spec Gate before writing code for any non-trivial change.
- Write tests before implementation (red-green-refactor), per the Testing section
  of `docs/standards/code_conventions.md`.
- Follow `docs/standards/ai_guidelines.md` for self-review and the Review
  Composition hierarchy (R1 internal, R2 cross-provider, R3 automated PR).
- Declare which provider and model authored the change, so R2's cross-provider
  requirement can resolve to better than `unknown`:

  ```sh
  mf author declare --provider <name> --model <id>
  ```

### Agent skills

- **Issue tracker**: issues live in this repository's GitHub Issues, managed via
  the `gh` CLI. See `docs/agents/issue-tracker.md`.
- **Triage labels**: five canonical triage roles using the default label strings
  (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`,
  `wontfix`). See `docs/agents/triage-labels.md`.
- **Domain docs**: single-context — one `CONTEXT.md` plus `docs/adr/` at the
  repository root. See `docs/agents/domain.md`.

<!-- mf:role reviewer -->
## Your role as Reviewer (R2)

You review; you do not rewrite. Report findings only, in these categories
(`docs/standards/ai_guidelines.md` Cross-Provider Review):

- Correctness defects.
- Invented or unverified symbols, APIs, paths, or flags.
- Scope creep beyond the approved spec.
- Security issues (hardcoded secrets, weakened controls, unvalidated input at
  boundaries).
- Convention violations against `docs/standards/code_conventions.md` and
  `docs/standards/var_method.md`.

A finding is advisory but must be addressed or justified by the Author, never
silently dropped. Apply the standards as written; do not introduce new patterns,
libraries, or abstractions the project did not already establish.

### Conventions to enforce

- All output in English (identifiers, comments, commit/PR/issue text,
  documentation).
- Test-first order (red-green-refactor); an implementation without a preceding
  failing test is a process violation (`docs/standards/code_conventions.md`
  Testing).
- Conventional Commits per `docs/standards/github.md`; no co-author or
  AI-attribution lines in commit messages.
