# SPEC: feat(harness): rebuild the framework as a provider-agnostic binary with a role runner

## Problem

The framework's standards describe a review pipeline whose layers are bound to named
vendors — R1 to a Claude Code plugin that is not installed, R3 to a GitHub app — and
nine of its eleven standards have no executor at all, so the documents that claim to
close the Gap are themselves on the wrong side of it.

## Design Decision

Promote the R2 backend chain into a general **role runner** and ship it as a single Go
binary named `mf`, keeping the standards as Markdown that the binary reads.

R1, R2 and R3 stop being three technologies and become three configurations of one
operation: review a change under a role prompt, with a chosen backend, and report which
backend actually ran. What separates them is when they run, how much of the change they
see, and what constraint applies to the provider — not what code executes them. The
three-outcome adapter contract that `docs/standards/r2_gate.md` already defines (`0`
reviewed, `10` unavailable, other reviewed-with-findings) generalizes unchanged to every
role, because the distinction it encodes — availability versus verdict — is what keeps a
chain from reporting a review that never happened.

Configuration moves to TOML in both layers, and splits by the nature of the data rather
than by scope. Policy — which roles exist, their backend chains, their constraints,
their thresholds — lives in a versioned `.framework.toml` so it travels with the
repository. Machine state — endpoints, the *name* of the environment variable holding a
key, local preferences — lives in `~/.config/framework/config.toml` and may not be
declared in the project file at all, which makes "secrets never enter the repository" a
validation rule instead of a recommendation. Environment variables override both, and
the existing `r2.*` git-config keys are read as a deprecated fourth layer until
`mf config migrate` moves them. Because this puts a value in more than one possible
place, provenance reporting is part of the design rather than a convenience:
`mf config get` names the layer a value resolved from, and `mf doctor` prints the whole
resolved table with its provenance.

The cross-provider constraint is answered as a property of the change rather than of the
push. A push carries commits that may come from several sessions, several agents, or a
human typing, so asking "which provider authored this" at push time is structurally too
late. `mf author declare` records the Author's provider and model per branch at the
moment the change is made, and the R2 gate reads that record. Environment fingerprints
are used only to cross-check it: a detected provider that contradicts the declaration is
a loud error, never a silent preference. That yields three states — `verified` when a
fingerprint agrees and differs from the Reviewer's provider, `declared` when only the
record asserts it, and `unknown` when there is no signal — and `unknown` is not
satisfied. It reports the requirement as unverified in the line meant for the Pull
Request, on the same principle the framework already holds: falling back is allowed,
falling back quietly is not.

Two standards move before any of this can be built. R1 becomes a backend chain with
`superpowers` retained as one backend rather than the layer's only executor, because a
layer with exactly one possible executor is the failure `0013` already fixed for R2. And
the CRURA Method is re-scoped: its always-on obligation becomes the Spec Gate, where the
human decides before the machine acts, while post-change human review becomes triggered
— by a blocking or security finding, by a failing check, or by a declared high-risk path
— and instrumented, recording whether it found anything the automated layers did not.

Backends are declarative. An agentic CLI backend is a config block naming the command,
an argument template, its provider identity, and the patterns that mean "unavailable",
so adding a reviewer CLI is a configuration change rather than a release of the binary.
Only the HTTP wire protocols (OpenAI-compatible, Anthropic, Google) and the
deterministic in-process checks are compiled in.

The deterministic checks derive their data from the standards documents rather than
restating it. `mf check commit` parses the Type Table out of `docs/standards/github.md`;
`mf check spec` parses the required sections out of `docs/standards/spec_method.md`.
This honors the framework's own rule that no parallel list of commit types exists
anywhere, and it makes drift between a document and the check that enforces it
structurally impossible: the documents become the data the binary executes.

## Alternatives Considered

- **Keep Bash and grow the CLI there.** Rejected. The `openai` adapter already had to
  shell out to Node for JSON and HTTP, which is the shape of every future provider, and
  subcommand parsing, config validation, structured findings and usage accounting are
  all the same class of work. Git Bash on Windows, the primary development environment,
  amplifies each of these rather than easing them.
- **Node/TypeScript distributed on npm.** Rejected at the Developer's decision. It is
  the smallest step from today's soft dependency on Node and it has the best library
  situation, but it makes a runtime a hard prerequisite of a development-process tool
  that must run in repositories of any language. A single binary installs with no
  runtime at all, which is the property that turns "copy these directories" into an
  installation.
- **Python via uv or pipx.** Rejected. It aligns with `sb100_agents`, the one real
  adopter, and `pydantic-settings` would supply config schema and validation for free,
  but it imposes a Python environment on repositories that are not Python for the same
  reason Node was rejected.
- **Keep `git config` as the only configuration store,** as
  `docs/specs/0013-detach-r2-from-codex.md` decided. Rejected. This supersedes that
  decision, so its three stated reasons are answered rather than waved past. *"Adds a
  format to parse in shell"* was substrate-dependent and does not survive the move to
  Go. *"Duplicates a scope cascade git already implements correctly"* is true but cheap:
  the cascade is a few lines of resolution, not a mechanism of weight, and git's cascade
  cannot express the layer actually required, because `.git/config` is never committed
  and project policy must travel with the repository. *"Creates a second place to look
  when a value resolves surprisingly"* survives intact and is the real cost; it is
  accepted only against the mandatory mitigation of per-layer provenance reporting in
  `config get` and `doctor`, which is why that reporting is an acceptance criterion and
  not a nicety. Note also that what `0013` rejected was a machine-level file under
  `~/.my-framework/`; it never ruled on a versioned project file, because the
  requirement had not yet appeared.
- **Add only the versioned project layer and keep `git config` for machine state.**
  Rejected at the Developer's decision. It would supersede nothing and would require no
  reconfiguration from existing adopters, but it leaves two configuration technologies in
  permanent coexistence and keeps a tool's machine state hanging off git's configuration
  — a place it landed only because shell could not parse a format.
- **Materialize a committed file into `git config` at `mf init`.** Rejected. It keeps a
  single runtime store, but it creates two representations of the same data with nothing
  keeping them in sync, which is the drift failure this framework designs against
  everywhere else.
- **Keep R1 defined as the same-provider Superpowers pass.** Rejected at the Developer's
  decision to keep Superpowers configured as one backend rather than as the sole
  executor. A layer with exactly one possible executor is precisely the failure
  `docs/specs/0013-detach-r2-from-codex.md` fixed for R2: when that one executor is
  absent, the layer silently stops existing while still reporting success. R1 is absent
  today for exactly that reason.
- **Detect the Author's provider at push time instead of declaring it at authoring
  time.** Rejected. A push has no single Author: it carries commits that may come from
  several sessions, several agents, or a human typing directly, so provider identity is a
  property of the change and inferring it from the pushing process is too late by
  construction. Detection is retained, but only as a cross-check against the declaration.
- **Leave the cross-provider rule as a report with no gate.** Rejected at the Developer's
  decision. It is honest about what a machine can guarantee, but it reduces the one rule
  that distinguishes R2 from every other review layer to a convention nothing enforces.
  The three-state report keeps the rule enforceable where there is signal and visibly
  unverified where there is not.
- **Leave the CRURA Method unchanged and only record the tension.** Rejected at the
  Developer's decision. Recording a known defect in a standard while continuing to
  prescribe it is the documentation-without-activation failure the framework exists to
  remove.
- **Invert CRURA completely** — the human states the expected outcome up front and the
  harness interrupts only on disagreement, with no scheduled post-change review.
  Rejected. It is the cleanest form of the argument, but it bets everything on acceptance
  criteria being complete, and they rarely are; the triggered form keeps a path for what
  the criteria did not anticipate.
- **Have the harness run the Author as well** — its own agent loop, tools and sandbox.
  Rejected. It is an order of magnitude more work and it competes with the agent CLIs
  the framework exists to govern, rather than governing them. The harness stays the
  surface of the team, not a replacement for the vendor's agent.
- **Judge process with a model** — ask a reviewer whether the change followed test-first
  order, or whether the spec was honored. Rejected on recorded evidence: judging an
  artifact and judging a process are different tasks, and the second is not reliable
  today. Every process rule the framework states is therefore checked deterministically
  or not at all.

## Scope

- Includes:
  - A Go binary `mf` with subcommands `init`, `doctor`, `config`, `review`, `check`,
    `author`, `agents`, `hooks`, `models`, `eval`, `explain`, `statusline`, `upgrade`.
  - A role runner generalizing the R2 chain to the roles `author`, `r1`, `r2`, `r3`.
  - A TOML configuration system with the four-layer cascade, the policy/machine split,
    per-layer provenance reporting, and `mf config migrate` for the legacy `r2.*`
    git-config keys.
  - `mf author declare` recording the Author's provider and model per branch, plus the
    three-state cross-provider report (`verified`, `declared`, `unknown`) and the
    fingerprint cross-check.
  - Backend families: declarative `cli`, compiled `api` (OpenAI-compatible, Anthropic,
    Google), `inproc` deterministic checks, `external` (declared, runs elsewhere,
    recorded), and `in-session` for Superpowers.
  - Structured findings in the five categories `AGENTS.md` already enumerates.
  - Deterministic checks for spec, commit, branch, docs and ADR invariants, deriving
    their vocabularies from the standards documents.
  - The standards realignment this architecture requires: Review Composition (R1 as a
    chain, Author provenance) and the CRURA re-scope with its instrumentation.
  - `mf agents sync`: `AGENTS.md` as the single source generating the vendor instruction
    files, with a drift check.
  - R3 as a CI workflow posting findings to the Pull Request.
  - Token usage accounting in disjoint buckets, per backend.
  - `mf eval`: a corpus of diffs with planted defects, reporting hit rate per backend.
  - Ports of the three standards that have code or are being given code:
    `status_line.md` (removing the Node dependency), `crux_method.md` as `mf explain`,
    `token_economy.md` as a prompt style applied by the harness.
  - A shim keeping `scripts/r2-review.sh` and the existing environment variables working
    for current adopters.
  - Prebuilt release binaries for windows, linux and darwin, alongside the standards
    corpus that stays consumable as a git submodule.

- Does NOT include:
  - Running the Author. The binary never drives a coding agent's loop, holds tools, or
    executes a sandbox.
  - Deciding CRURA's final shape from evidence. The re-scope ships with instrumentation;
    the next cut is made when there is data, not in this spec.
  - A maintained price table per vendor. Usage is counted in tokens; a monetary figure
    is produced only from a price table the user supplies.
  - A capability ranking of third-party models. Which backend is stronger stays the
    human's judgment, as `docs/specs/0013-detach-r2-from-codex.md` decided.
  - Automatic application of findings. `mf review` reports; it never rewrites code.
  - Any LLM-judged check of process rather than artifact.
  - An authority hierarchy for multi-repository setups, still out of scope.
  - Rewriting published git history to correct the two commits carrying AI-attribution
    trailers.
  - A GUI or TUI. The binary is non-interactive except where `init`, `config` and
    `author declare` prompt.
  - Renaming the repository. The binary is `mf`; the repository stays `my-framework`.

## Acceptance Criteria

Configuration

- `resolves_a_role_binding_from_env_over_project_over_machine_over_default`
- `rejects_a_project_config_that_declares_an_endpoint_or_a_credential_reference`
- `reads_legacy_git_config_r2_keys_when_no_project_or_machine_value_is_set`
- `migrate_moves_legacy_git_config_keys_into_the_machine_file_and_leaves_the_key_value_untouched`
- `stores_only_the_name_of_the_api_key_variable_and_refuses_a_value_that_is_not_a_variable_name`
- `names_the_layer_each_resolved_value_came_from`
- `doctor_prints_the_full_resolved_configuration_with_its_provenance`

Role runner

- `advances_the_chain_when_a_backend_reports_unavailable`
- `stops_the_chain_at_the_first_backend_that_reviews`
- `reports_the_backend_provider_and_model_that_actually_reviewed`
- `exits_zero_and_says_so_when_no_backend_in_the_chain_was_available`
- `skips_the_role_when_the_branch_adds_nothing_over_its_base`
- `runs_a_cli_backend_declared_only_in_configuration_with_no_compiled_adapter`
- `classifies_a_backend_as_unavailable_using_its_configured_patterns`
- `reports_a_truncated_diff_and_a_response_cut_off_by_the_output_limit_as_partial`
- `treats_exceeding_the_wall_clock_budget_as_unavailability_rather_than_a_finding`

Author provenance and the cross-provider rule

- `records_the_author_provider_and_model_per_branch_at_declaration_time`
- `reports_verified_when_a_detected_fingerprint_agrees_and_differs_from_the_reviewer_provider`
- `reports_declared_when_only_the_branch_record_asserts_the_author_provider`
- `reports_unknown_and_does_not_treat_r2_as_satisfied_when_there_is_no_signal`
- `fails_loudly_when_a_detected_provider_contradicts_the_declared_one`
- `refuses_an_r2_run_whose_resolved_backend_provider_equals_a_verified_author_provider`

Deterministic checks

- `fails_a_branch_whose_spec_has_an_empty_does_not_include_list`
- `fails_a_non_trivial_branch_that_carries_no_spec`
- `derives_the_commit_type_vocabulary_from_github_md_rather_than_a_literal_list`
- `fails_when_the_type_table_cannot_be_parsed_rather_than_falling_back_to_a_stale_list`
- `fails_a_commit_message_carrying_an_ai_attribution_trailer`
- `fails_when_a_spec_number_is_reused_or_a_previously_committed_spec_is_absent`
- `fails_when_a_standards_document_is_missing_from_index_md`

Human review

- `triggers_a_required_human_review_on_a_blocking_or_security_finding`
- `records_whether_a_human_review_found_a_defect_the_automated_layers_missed`

Author neutrality

- `generates_the_vendor_instruction_files_from_agents_md`
- `fails_the_drift_check_when_a_generated_instruction_file_diverged_from_agents_md`

R3

- `posts_findings_as_one_pull_request_comment_naming_the_backend_and_model`
- `exits_zero_in_ci_when_no_backend_was_available`

Usage and evaluation

- `reports_token_usage_in_disjoint_buckets_per_review`
- `marks_usage_as_estimated_when_the_backend_returned_none`
- `reports_hit_rate_per_backend_over_the_planted_defect_corpus`
- `sends_temperature_zero_on_every_evaluation_request`

Doctor and activation

- `names_the_backend_and_model_that_each_role_resolves_to`
- `reports_a_configured_api_key_variable_that_is_unset`
- `reports_whether_the_hooks_path_points_at_the_versioned_hooks_directory`

Ported standards

- `renders_the_five_status_line_facts_in_contract_order_without_node`
- `writes_a_transient_crux_explainer_outside_version_control`
- `applies_terse_style_only_to_conversation_prompts_and_never_to_spec_pr_or_commit_text`

Migration

- `r2_review_sh_delegates_to_the_binary_when_present_and_runs_the_previous_path_when_absent`
- `honors_the_legacy_skip_codex_review_and_codex_review_base_variables`

## Implementation Slices

Each slice gets its own numbered spec and its own Spec Gate. The order is a dependency
order, not a priority order. The two standards slices come first because they define what
the code is required to do.

1. `0015` — realign Review Composition: R1 as a backend chain with `superpowers` retained
   as one backend, and Author provenance as a three-state property of the change.
   Touches `ai_guidelines.md`, `r2_gate.md`, `CONTEXT.md`.
2. `0016` — re-scope the CRURA Method from always-on to triggered, with instrumentation.
   Touches `crura_method.md`, `ai_guidelines.md`, `CONTEXT.md`.
3. `0017` — configuration cascade, policy/machine split, `config`, `migrate`, and
   per-layer provenance reporting.
4. `0018` — the role runner and the backend families, with the R2 chain ported onto it
   behind the compatibility shim.
5. `0019` — deterministic checks deriving their vocabularies from the standards.
6. `0020` — `doctor`, `init`, `hooks`, `upgrade`, and the release pipeline.
7. `0021` — `agents sync` and the removal of the `CLAUDE.md` / `AGENTS.md` duplication.
8. `0022` — R3 in CI with Pull Request posting.
9. `0023` — usage accounting and `models pin`.
10. `0024` — `eval` and the planted-defect corpus.
11. `0025` — the ported `statusline`, `explain`, and token-economy style.
12. `0026` — the tool's visual identity, decided once every other slice is
    implemented and validated. Added to this list after the architecture was
    approved, at the Developer's direction. The reference is
    `https://github.com/voltagent/awesome-design-md`, a collection of `DESIGN.md`
    files in Google Stitch format — palette, typography, components, spacing,
    depth — written to be read by an agent that then generates matching UI.

    It is placed last because it depends on the surfaces existing. Two of them
    are genuinely visual: the CRUX explainer that `0025` generates, which is an
    HTML page and where a `DESIGN.md` applies directly, and the terminal output,
    where it barely applies at all — a terminal has no fonts, no shadows and no
    cards, and its palette belongs to the reader's theme rather than to the
    tool. The status line is explicitly out of reach: `status_line.md` binds the
    five facts and their order and deliberately binds neither colours nor glyphs.

    The decision that slice has to make is which of two things is being adopted:
    the `DESIGN.md` *format*, with values authored for this project, or a
    specific brand's file taken as-is. The second is what the reference
    repository is built for, and it means shipping a published tool dressed in
    another company's identity — a question of trademark and honest attribution
    rather than of taste, and one the Developer decides rather than the spec.

## Reproducibility

- Go toolchain: the version pinned in `go.mod`, reported by `go version`.
- Unit and integration tests: `go test ./...`.
- Deterministic checks against this repository: `mf check`.
- Chain resolution without running any reviewer: `mf review --role r2 --dry-run`.
- Resolved configuration with provenance: `mf doctor`.
- Legacy path, until its slice lands: `bash scripts/test/r2-review.test.sh`,
  `bash scripts/test/setup.test.sh`, `bash scripts/test/statusline.test.sh`,
  `bash scripts/test/docs-consistency.test.sh`, `bash scripts/test/docs-consistency.sh`.
- Evaluation runs are reproducible only under `temperature = 0` and a pinned model id; a
  run that cannot state both is not a result.

## Risks and Assumptions

- **No Go exists in this repository's history.** The toolchain, the idioms and the
  release pipeline are all new at once. What limits the damage is that the slices are
  independently gate-able and the shim keeps the current behavior available throughout.
- **Roughly 1,900 lines of tested shell and Node are being replaced.** The assumption is
  that their test suites stay green until each subject is ported, and that the shim makes
  the cutover invisible to `sb100_agents`, the one known adopter, which consumes this
  repository as a `.standards` submodule.
- **Moving the machine layer off `git config` is a supersession, not an addition.**
  Existing adopters have values in the old store; the deprecated read layer and
  `config migrate` are what keep that from being a breaking change, and they are
  load-bearing rather than courtesies.
- **A value can now resolve from four places.** Provenance reporting is the whole
  mitigation. If `config get` and `doctor` do not name the layer, this change is a net
  regression against `0013`, which is why both are acceptance criteria.
- **Parsing the standards couples a check to a document's formatting.** A formatting
  change that breaks the parse must fail loudly; falling back to a compiled-in list would
  silently reinstate the parallel vocabulary the framework forbids. This is why the
  failure mode is a hard error rather than a default.
- **Structured findings assume the backend can produce them.** API backends can be asked
  for a schema; agentic CLI backends cannot, so their output stays prose and is recorded
  as a single textual finding. Any feature that needs per-line findings is therefore
  unavailable behind a CLI backend, and must say so rather than appearing empty.
- **Token usage is not reported uniformly across vendors.** Layouts and terminology
  differ, and some frameworks drop usage on some execution paths. Accounting fails open
  to an estimate and marks it as such; reporting zero as a measured value would be a
  fabricated number.
- **The Author declaration remains the load-bearing input.** Fingerprint detection
  narrows the window but ages with every vendor release, so most branches will report
  `declared` rather than `verified`. A branch whose declaration was never written reports
  `unknown`, which is correct but will be common until `init` wires the declaration into
  the authoring flow — and a framework that reports `unknown` most of the time has an
  adoption problem even though it has no correctness problem.
- **The `superpowers` backend cannot be invoked as a subprocess.** It runs inside a
  Claude Code session, so its participation is an instruction plus an attestation rather
  than an execution. That asymmetry is designed and reported, not hidden: a run satisfied
  by attestation says so, and the chain treats an absent session as unavailability.
- **The CRURA re-scope is a bet, and the instrumentation is what makes it falsifiable.**
  The bet is that triggered attention beats fixed-rate attention. The instrumentation
  depends on the Developer honestly recording whether a human review found something the
  automated layers missed — the same honor-system problem the framework exists to remove,
  accepted here because there is no mechanical way to know what a person noticed.
- **The v1 scope is large.** Slicing is what keeps it tractable; a slice that cannot pass
  its own Spec Gate is evidence the decomposition was wrong, not a reason to skip the
  gate.

## ADR Candidates

To be promoted or declined by the Developer at the Spec Gate:

- Go as the substrate and a single binary as the distribution.
- TOML in both layers with the policy/machine split, superseding the
  `docs/specs/0013-detach-r2-from-codex.md` decision to use `git config` alone.
- Author provenance as a property of the change, with the three-state cross-provider
  report.
- Re-scoping the CRURA Method from always-on to triggered, with instrumentation.
- Deterministic checks deriving their vocabularies from the standards documents.
- R1 as a backend chain rather than a same-provider layer, with `superpowers` retained as
  one backend.
