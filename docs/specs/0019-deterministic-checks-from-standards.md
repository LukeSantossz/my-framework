# SPEC: feat(check): enforce the process rules by parsing the standards that declare them

## Problem

Nothing verifies that a non-trivial change carries a complete spec or that a commit uses
a type the Type Table declares, and any check that grew to do so would have to restate
vocabularies the standards call canonical, which is the parallel list those standards
forbid.

## Design Decision

Implement `docs/adr/0009-checks-derive-vocabularies-from-standards.md` as `mf check`.

The vocabularies are read, not restated. `mf check commit` parses the Type Table out of
`docs/standards/github.md`; `mf check spec` parses the required section headings out of
the template block in `docs/standards/spec_method.md`. Adding a commit type becomes a
documentation edit that the checker follows with no release, and a document and its
enforcement cannot drift apart because there is only one of them.

A parse that fails is a hard error. Falling back to a compiled-in list would silently
reinstate the forbidden parallel vocabulary and would turn a broken document into a
passing build, so the check refuses to run rather than run on stale data.

Five checks ship. `spec` verifies a non-trivial branch carries `docs/specs/NNNN-*.md`
with the Gate-checked sections present, a non-empty "Does NOT include", and at least one
Acceptance Criterion. `commit` validates Conventional Commits against the parsed table
and refuses AI-attribution trailers. `branch` validates the naming pattern with a type
drawn from the same table. `docs` ports the invariants `scripts/test/docs-consistency.sh`
already enforces. `records` verifies numbering is contiguous and that no previously committed
record disappeared.

What is not checked here is not checked anywhere: no rule about process is delegated to a
model. Test-first order, spec conformance and scope discipline stay enforced by
discipline, and the standards say so rather than implying coverage that does not exist.

Triviality is decided by an explicit rule, not by a model or a line count alone: a branch
is non-trivial unless every changed path matches a configured exempt set. The rule is
crude on purpose, because a check nobody can predict is a check people route around.

## Alternatives Considered

- **Carry the vocabularies in Go.** Rejected. It is the parallel list the standards
  forbid, and it drifts silently — a stale list still passes.
- **Generate Go from the standards at build time.** Rejected. It removes runtime parsing
  but reintroduces two representations, and a generated file in the tree drifts exactly
  like a hand-written one when someone edits the document and does not regenerate.
- **Ask a reviewer model whether the spec was honoured.** Rejected. Process judgment is
  not reliable, and a check that is sometimes right is worse than none because it is
  trusted.
- **Keep the checks in shell and only add the spec check.** Rejected. Parsing a markdown
  table and a template block in shell is the same class of work that already forced the
  `openai` adapter into Node.
- **Decide triviality from the diff with a heuristic.** Rejected. An unpredictable gate
  gets bypassed, and a bypassed gate is worse than an explicit exemption list someone can
  read and argue with.

## Scope

- Includes:
  - `internal/check`: the five checks, each independently runnable and independently
    reporting.
  - Parsers for the Type Table and the spec template block, with named errors when a
    document's shape no longer matches.
  - `mf check [spec|commit|branch|docs|records]`, defaulting to all.
  - The exempt-path set for triviality, read from the project configuration.
  - Wiring the checks as the `inproc` backend kind the runner already carries.
  - Table-driven tests over fixture repositories, written first.

- Does NOT include:
  - Deleting `scripts/test/docs-consistency.sh`. It stays until the submodule consumer,
    which runs it directly, has a migration path.
  - Any check that calls a model.
  - Enforcing test-first order. It cannot be checked from a diff without reading intent,
    so it stays project policy and is stated as unenforced.
  - A commit-msg or pre-commit hook. Wiring is `0020`.
  - Auto-fixing anything.

## Acceptance Criteria

Derivation

- `derives_the_commit_type_vocabulary_from_github_md_rather_than_a_literal_list`
- `derives_the_required_spec_sections_from_the_spec_method_template_block`
- `fails_when_the_type_table_cannot_be_parsed_rather_than_using_a_compiled_in_list`
- `accepts_a_commit_type_added_to_github_md_with_no_code_change`

Spec check

- `fails_a_non_trivial_branch_that_carries_no_spec`
- `fails_a_spec_whose_does_not_include_list_is_empty`
- `fails_a_spec_with_no_acceptance_criterion`
- `passes_a_spec_lite_that_keeps_the_three_gate_checked_sections`
- `passes_a_branch_whose_every_changed_path_is_in_the_exempt_set`

Commit and branch

- `fails_a_commit_message_whose_type_is_absent_from_the_table`
- `fails_a_commit_message_carrying_an_ai_attribution_trailer`
- `fails_a_branch_name_whose_type_is_absent_from_the_table`

Durability

- `fails_when_a_spec_or_adr_number_is_reused`
- `fails_when_a_previously_committed_record_is_absent_from_the_tree`
- `reports_every_violation_it_found_rather_than_only_the_first`

## Reproducibility

- `go test ./...`
- `mf check`
- `bash scripts/test/docs-consistency.sh` — must still pass, since it is not removed.

## Risks and Assumptions

- **The parsed regions of the standards become a machine-readable surface.** Editing them
  freely is no longer safe. That constraint is accepted deliberately, but it will surprise
  someone reformatting a table for readability, and the error message is the only thing
  that will explain why.
- **A hard parse error blocks work when a document is mid-edit.** That is the intended
  trade against silently passing on stale data, but it means a malformed standard stops
  the build, and the fix must always be to repair the document rather than to loosen the
  parser.
- **The exempt-path set is a hole with a name.** Anyone can widen it to make the spec
  check stop firing. It is visible in a committed file and shows up in review, which is
  the only defence a configurable exemption can have.
- **Two implementations of the docs invariants coexist.** The Go check and the shell
  script must agree, and nothing forces them to; a divergence would let a repository pass
  one and fail the other depending on which it runs.
- **"Non-trivial" remains a judgment encoded as a path list.** A one-line change to an
  authentication path is non-trivial and a large mechanical rename is not, and a path set
  cannot express that. The rule will be wrong in both directions and is chosen for
  predictability, not accuracy.
