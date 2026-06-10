# CLAUDE.md

## Development Standards

Before any development work in this repository, read `docs/standards/INDEX.md` and
the documents it lists. Treat them as binding:

- Specify before building: produce a `SPEC.md` per `docs/standards/spec_method.md`
  and pass the Spec Gate before writing code for any non-trivial change.
- Follow `docs/standards/code_conventions.md`, including its precedence order, which
  is authoritative for resolving any conflict between rules.
- Write tests before implementation (red-green-refactor), per the Testing section
  of `code_conventions.md`.
- Follow `docs/standards/ai_guidelines.md` for self-review and the Review Composition
  hierarchy (R1 internal, R2 cross-provider, R3 automated PR).
- Follow `docs/standards/github.md` for Conventional Commits, branch naming, and the
  PR, Issue, and README templates. No co-author or AI-attribution lines in commits.
- Token economy per `docs/standards/token_economy.md`: terse mode is allowed in
  conversation but never in `SPEC.md`, PR, Issue, or commit artifacts; it never
  overrides Safety or Correctness. This file is kept in its caveman-compress form.
- All output in English.

Adjust the `docs/standards/` path above if you place the standards elsewhere.
