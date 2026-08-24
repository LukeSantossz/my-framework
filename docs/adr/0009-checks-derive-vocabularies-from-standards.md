# Deterministic checks derive their vocabularies from the standards

The framework already forbids parallel lists: "The canonical type vocabulary lives only
in the Type Table in `github.md`. No parallel list exists." A checker that validates
commit types by carrying its own copy of that table would violate the rule it enforces,
and would drift the first time the table changed — silently, because a stale list still
passes. So the checks read the standards instead. `mf check commit` parses the Type Table
out of `docs/standards/github.md`; `mf check spec` parses the required sections out of
`docs/standards/spec_method.md`. The documents stop being documentation *about* the
behaviour and become the data the binary executes.

The same reasoning excludes a class of check rather than merely relocating one. Judging an
artifact and judging a process are different tasks, and the second is not reliable from a
model today. Every process rule this framework states — test-first order, spec
conformance, scope discipline — is therefore checked deterministically or not at all, and
none of them is delegated to a reviewer model.

## Status

Accepted.

## Considered Options

- **Parse the standards at check time (chosen)**: one representation, and adding a commit
  type becomes a documentation edit that the checker follows with no release.
- **Duplicate the vocabularies in code**: rejected — it is precisely the parallel list the
  standards forbid, and it drifts silently the first time a document changes.
- **Generate code from the standards at build time**: rejected — it removes runtime
  parsing, but it reintroduces two representations, and a generated file checked into the
  tree drifts exactly like a hand-written one when someone edits the document without
  regenerating.
- **Ask a reviewer model whether a rule was followed**: rejected — process judgment is not
  reliable today, and a check that is sometimes right is worse than no check at all,
  because it is trusted.

## Consequences

- The parsed regions of the standards become a machine-readable surface and gain tests of
  their own. Editing them freely is no longer safe, and that constraint is accepted
  deliberately rather than discovered later.
- A formatting change that breaks a parse fails loudly. Falling back to a compiled-in list
  would silently reinstate the forbidden parallel vocabulary, so the failure mode is a
  hard error and never a default.
- Adding or renaming a commit type is a single edit to `github.md`; nothing else changes
  and no release is required.
- No process rule gains an LLM-based check. A rule that cannot be checked
  deterministically stays enforced by discipline, and the framework says so rather than
  implying coverage it does not have.
