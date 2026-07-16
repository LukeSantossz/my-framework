# Code Conventions

Rules for the code itself. Complement `var_method.md` (naming) and `github.md` (workflow).

## Precedence

When rules conflict, lower number wins:

1. Safety: never produce harmful or insecure code.
2. Correctness: code must work as written.
3. Language idiomatic syntax (e.g. snake_case in Python) over a generic rule here; the rule's intent still applies.
4. The project's existing established pattern over any default here.
5. VAR Method naming suffixes.

Authority between rule sources: a repository's standards (its `CLAUDE.md` and
`docs/standards/`) override user-global defaults (for example a global
`CLAUDE.md`) wherever both speak. Safety and Correctness are never overridden
by either side.

When still ambiguous, follow `ai_guidelines.md` and state the assumption. The
approved `SPEC.md` (per `spec_method.md`) is the source of truth for what the
change should do and what is out of scope; code that contradicts the spec is wrong
even if it works.

## Language

- English for all identifiers, comments, commit messages, PR/issue text, documentation.
- No mixed-language identifiers.
- Comments explain why, not what. (Exception: short structural labels in scaffolding such as a README project tree.)
- No commented-out code in committed files.

## Naming

- Names reveal responsibility; a reader infers what a symbol does without reading its body.
- Casing follows the language idiom consistently: camelCase/PascalCase for JS/TS/Dart, snake_case/PascalCase for Python. Never mix within a file.
- Booleans read as predicates (isActive, hasPermission, canRetry).
- Avoid abbreviations except widely understood ones (id, url, http). No single-letter names outside short-lived loop indices.
- Apply VAR Method suffixes only when they describe the real responsibility.

## Size and Structure

- A function does one thing. If describing it needs "and", split it.
- Prefer early returns over deep nesting; guard clauses at the top.
- Keep functions short enough to read without scrolling.
- One public responsibility per file/module. Group by feature, not by technical layer, unless the project established otherwise.
- No magic numbers or strings; name them as constants at the appropriate scope.

## Error Handling

- Never silently swallow errors. Handle them meaningfully or let them propagate with context.
- Catch the narrowest error type the language allows; avoid catch-all handlers except at boundaries (entrypoints, request handlers).
- Error messages state what failed and, where possible, what to do about it. No generic "error".
- Validate inputs at boundaries (user input, network, file I/O). Trust internal calls.
- Fail fast on programming errors; recover gracefully from expected runtime errors.

## Consistency

- Match the surrounding code's existing style before introducing a new one.
- Use the project's formatter and linter; do not hand-format against the tool.
- Follow existing patterns for logging, config, and dependency injection rather than inventing a parallel approach.

## What Never Enters a Commit

- Secrets, API keys, tokens, credentials, .env contents.
- Debug output: console.log, print, dump, debugger statements.
- Generated artifacts, build output, dependency directories that belong in .gitignore.
- Large binaries or data files that bloat history without need.

## Testing

- Write the test before the implementation: red (watch it fail), green (minimal implementation passes), refactor. An implementation commit without a preceding failing-test commit is a process violation. This order is run by the Superpowers TDD phase when that plugin is installed, and recorded here as project policy so it binds with or without the tool.
- A test asserts behavior, not implementation detail.
- Test names describe scenario and expected outcome (returns_empty_list_when_no_matches).
- Cover meaningful branches and edge cases (empty, boundary, error path), not just the happy path.
- Each Acceptance Criterion in the `SPEC.md` has a corresponding test.
- Do not write a test that only re-states the implementation.

## Dependencies

- Prefer the standard library and existing project dependencies before adding a new one.
- A new dependency must be actively maintained and justified; note the reason in the PR.
- Do not add a dependency to replace a few lines of trivial code.
