# SPEC: feat(cli): port the status line, the CRUX explainer, and the terse style into the binary

## Problem

Three standards have no working executor in the binary: the status line renderer is Node
that this rebuild set out to remove, the CRUX Method names a skill that was never
created, and the Token Economy's terse mode is a convention the framework applies to
nothing it controls.

## Design Decision

Close the last three gaps between what the standards say and what the framework does, so
no standard remains that describes behaviour nothing performs.

The status line moves from the Node renderer into the binary. It is the same contract —
five facts, in the same order, across Claude Code and Codex — with the same
machine-scoped, opt-in application, and it drops the last hard Node dependency, closing
the recorded limitation that a machine without Node has the two agents diverge. What the
contract binds is unchanged: the facts and their order, never the colours or glyphs.

CRUX becomes `mf explain`. It generates the transient, interactive HTML explainer the
method describes — Background, Intuition, Code, Quiz — writes it outside version control,
and stays advisory: never a review layer, never a gate. It uses the same role runner, so
which model explains is configuration like every other role.

The Token Economy becomes a style the harness applies to the prompts it sends, which is
the only place the framework can apply anything. Its scope boundary is enforced rather
than trusted: terse style reaches conversation-shaped prompts and is refused for any text
that becomes a spec, pull request, issue or commit artifact. That boundary was previously
a rule a person had to remember.

Where a capability still does not exist, the standard says so instead of implying it.
`caveman-compress` — the context-file compression the Token Economy describes — has no
implementation here and is recorded as absent, not quietly claimed.

## Alternatives Considered

- **Keep the Node renderer and shell out to it.** Rejected. It preserves the dependency
  the rebuild exists to remove, and the limitation it causes is already recorded in the
  README as a defect.
- **Drop the status line entirely as cosmetic.** Rejected. It is a written standard with
  a working implementation; deleting it to reduce surface would remove the one part of
  the framework that already worked.
- **Retire `crux_method.md` and `token_economy.md` instead of implementing them.**
  Rejected at the Developer's decision to port all three. It was the honest alternative
  to leaving them unexecuted, and it stops being necessary once they have executors.
- **Implement `caveman-compress`.** Rejected. Rewriting the loaded context file risks the
  activation the whole framework depends on, and a compression that stops an agent
  reading `INDEX.md` reopens the Gap by definition.
- **Make CRUX a review layer once it has an implementation.** Rejected.
  `docs/adr/0003-crux-explainers-are-transient.md` settled that it is an aid; having code
  does not change what it is.

## Scope

- Includes:
  - `mf statusline render` and `mf statusline apply`, replacing the Node renderer and the
    `setup.sh --statusline` path, keeping the contract and the opt-in machine scope.
  - `mf explain`, generating the transient HTML explainer outside version control, with
    selectable quiz difficulty.
  - The terse prompt style, with an enforced boundary refusing it for versioned artifacts.
  - Recording `caveman-compress` as having no implementation.
  - Removing the Node dependency and the README limitation it caused.
  - Tests, written first, including a golden render of the status line and a check that
    the explainer is written outside the repository.

- Does NOT include:
  - `caveman-compress`, or any rewriting of `CLAUDE.md`.
  - Making CRUX a review layer or a gate.
  - Changing which five facts the status line shows, or their order.
  - A per-repository status line for Codex, which its configuration cannot express.
  - Removing `scripts/statusline/claude-statusline.js` before the submodule consumer has
    migrated.

## Acceptance Criteria

Status line

- `renders_the_five_contract_facts_in_contract_order`
- `renders_without_node_installed`
- `applies_the_contract_to_both_agent_configurations_and_backs_up_a_divergent_one`
- `leaves_a_matching_configuration_untouched`
- `reports_usage_as_unavailable_rather_than_zero_when_the_quota_source_cannot_be_read`

Explain

- `writes_the_explainer_outside_version_control`
- `refuses_to_write_the_explainer_into_the_repository`
- `produces_the_four_sections_the_method_names`
- `never_blocks_or_reports_a_verdict`
- `resolves_its_model_through_the_role_configuration`

Terse style

- `applies_terse_style_to_a_conversation_prompt`
- `refuses_terse_style_for_spec_pull_request_issue_and_commit_text`
- `never_lets_terse_style_shorten_a_safety_or_correctness_instruction`
- `records_caveman_compress_as_having_no_implementation`

## Reproducibility

- `go test ./...`
- `mf statusline render` against a fixture session payload, compared to a golden file.
- `bash scripts/test/statusline.test.sh` — must stay green until the Node renderer is
  removed, since it is not removed in this slice.

## Risks and Assumptions

- **The Codex segment vocabulary was read from an installed build, not a published
  schema.** An upgrade that renames a segment leaves the written configuration silently
  ignored — the line degrades, nothing breaks — and porting to Go does not improve that.
- **`mf explain` sends a diff to a model to produce prose, and prose is where fabrication
  hides.** An explainer that confidently explains something the code does not do is worse
  than no explainer, and nothing here verifies its claims.
- **The terse boundary is enforced only where the harness composes the prompt.** A person
  writing a commit message by hand is outside it, so the rule remains partly a discipline.
- **Two status line implementations coexist** until the Node renderer is removed, and they
  can disagree about what the contract renders.
- **Porting all three adds surface for standards that are advisory.** The status line is
  cosmetic and CRUX never blocks; this slice spends real implementation effort on the
  least load-bearing parts of the framework, which is defensible only because leaving a
  standard unexecuted is the failure the project exists to fix.
