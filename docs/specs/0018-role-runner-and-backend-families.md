# SPEC: feat(review): run any review role through a chain of backends

## Problem

The chain that walks reviewer backends exists only inside `scripts/r2-review.sh` and only
for R2, so R1 has no executor, R3 has none either, and the one working piece of the
pipeline cannot be reused by the layers that need it.

## Design Decision

Promote the chain into a role runner in Go, per
`docs/specs/0014-rebuild-the-framework-as-a-harness.md`, and express R1, R2 and R3 as
configurations of it rather than as separate code paths.

The runner takes a role, resolves its ordered backend chain through `internal/config`,
and walks it until a backend actually reviews. It reports which backend ran, under which
provider and model, and what it found. The three-outcome contract from
`docs/standards/r2_gate.md` is preserved exactly — `0` reviewed, `10` unavailable, any
other code reviewed with findings or failed mid-review — because the distinction it
encodes is availability versus verdict, and losing it would let a chain report a review
that never happened.

Four backend kinds. `cli` is declarative: a command, an argument template, a provider
identity, and the patterns that mean unavailable, all from configuration, so a new
agentic reviewer is a config block rather than a release. `api` is compiled and speaks
one of three wire shapes — OpenAI-compatible, Anthropic, Google. `inproc` runs
deterministic checks with no model. `in-session` cannot be started as a subprocess at
all: it reports unavailable unless an attestation for this change exists, which is the
honest shape for `superpowers` and the one `0015` already wrote into the standards.

R2 alone consults the Author Declaration and computes the cross-provider state. The
runner refuses to report R2 satisfied when the state is `unknown`, and errors loudly when
a detected fingerprint contradicts the declaration.

Findings are structured — the five categories `AGENTS.md` enumerates, each with a
severity and an optional file and line. An `api` backend is asked for that shape
directly. A `cli` backend cannot produce it, so its output is captured as one textual
finding and reported as such, never as an empty result, because "no findings" and "a
finding this backend cannot express" must not look alike.

`scripts/r2-review.sh` becomes a shim: it execs the binary when present, and runs today's
shell path when not. Its environment variables and its output line survive unchanged, so
the submodule consumer notices nothing.

## Alternatives Considered

- **Keep R2 in shell and add R1 and R3 beside it.** Rejected. It triples the surface that
  already needed Node for JSON and HTTP, and each new provider repeats the cost — the
  reason `docs/adr/0005-go-substrate-single-binary.md` exists.
- **Compile every backend, including the agentic CLIs.** Rejected. It puts vendor error
  strings in the binary, so a reworded quota message needs a release to fix, and it is
  what makes an unknown backend name a defect rather than a configuration choice.
- **Make `in-session` backends run by shelling into the agent's CLI.** Rejected. It
  cannot work: the session is already running and the skill executes inside it, so
  spawning the CLI would start an unrelated session and attest to nothing.
- **Let a `cli` backend return no findings when its output cannot be parsed.** Rejected.
  Silence would be read as a clean review, which is a false negative reported as a pass —
  the failure mode this framework treats as worst.
- **Compute the cross-provider state for every role.** Rejected. Only R2 carries the
  requirement, and computing it elsewhere would invite a later reader to think R1 or R3
  enforces something it does not.

## Scope

- Includes:
  - `internal/role`: chain resolution, the walk, availability classification, and the
    report naming the backend, provider, model and cross-provider state.
  - `internal/backend`: the `cli`, `api`, `inproc` and `in-session` kinds behind one
    interface.
  - `internal/report`: the structured finding type, its five categories, and terminal
    rendering.
  - `internal/vcs`: the git plumbing the runner needs — resolving refs, producing a
    bounded diff, and refusing a ref that does not resolve rather than treating an empty
    diff as nothing to review.
  - `mf review --role <r1|r2|r3> [--base <ref>] [--dry-run]`.
  - The Author Declaration read path and the three-state cross-provider computation.
  - The wall-clock budget and diff cap already measured for the `openai` path, carried
    over with their current defaults.
  - The `scripts/r2-review.sh` shim and its compatibility tests.

- Does NOT include:
  - Writing the Author Declaration. Reading it is here; `mf author declare` is its own
    work in a later slice, and until it exists the state is `declared` at best.
  - The deterministic check implementations. The `inproc` kind is wired; what it runs is
    `0019`.
  - `doctor`, `init`, `hooks`, or `upgrade`.
  - Posting anything to a Pull Request.
  - Token usage accounting beyond capturing what a backend returns.
  - Removing the shell path or its test suite.
  - Retrying a failed request, or any streaming. A review is one bounded request.

## Acceptance Criteria

The walk

- `advances_the_chain_when_a_backend_reports_unavailable`
- `stops_the_chain_at_the_first_backend_that_reviews`
- `reports_the_backend_provider_and_model_that_actually_reviewed`
- `exits_zero_and_names_every_backend_tried_when_none_was_available`
- `skips_the_role_when_the_branch_adds_nothing_over_its_base`
- `refuses_to_build_a_diff_from_a_ref_that_does_not_resolve`
- `dry_run_describes_every_backend_in_the_chain_and_runs_none`

Backend kinds

- `runs_a_cli_backend_declared_only_in_configuration`
- `classifies_a_cli_backend_unavailable_using_its_configured_patterns`
- `reports_an_in_session_backend_unavailable_when_no_attestation_exists`
- `treats_an_http_error_from_an_api_backend_as_unavailable_rather_than_as_findings`
- `treats_exceeding_the_wall_clock_budget_as_unavailability`
- `never_reports_reasoning_content_as_findings`

Findings

- `parses_structured_findings_from_an_api_backend_into_the_five_categories`
- `records_unparseable_cli_output_as_one_textual_finding_rather_than_as_none`
- `reports_a_truncated_diff_and_an_output_cut_off_by_the_limit_as_partial`

Cross-provider

- `computes_the_state_only_for_r2`
- `refuses_to_report_r2_satisfied_when_the_state_is_unknown`
- `fails_loudly_when_a_detected_provider_contradicts_the_declaration`

Compatibility

- `the_shim_execs_the_binary_when_present_and_runs_the_shell_path_when_absent`
- `honors_the_legacy_skip_and_base_environment_variables`
- `keeps_the_reviewed_by_output_line_the_pull_request_record_quotes`

## Reproducibility

- `go test ./...`
- `go build ./cmd/mf`
- `mf review --role r2 --dry-run`
- `bash scripts/test/r2-review.test.sh` — the shell suite must stay green, because the
  shim is what protects the submodule consumer.
- API backends are exercised against a local scriptable HTTP server, never a vendor, so
  the suite is deterministic and needs no key.

## Risks and Assumptions

- **Go is still not installed at the time of writing.** Implementation waits; the spec is
  authored ahead so that the toolchain is the only thing on the critical path.
- **The `cli` argument template is a small language, and small languages grow.** It
  starts as substitution of named fields and nothing else. The moment it gains
  conditionals it has become a program in a config file, which is harder to debug than
  the adapter script it replaced.
- **Availability patterns are still vendor error text.** Moving them to configuration
  makes drift fixable without a release, but it does not make drift detectable: a pattern
  that stops matching reads an unavailable backend as one that reviewed, and the chain
  stops early rather than falling through. The report naming the backend is what gives a
  human the chance to notice.
- **The `in-session` kind will almost always report unavailable.** Superpowers is not
  installed and nothing writes attestations yet, so R1's chain will fall through to
  whatever follows it. That is correct behaviour and will look like a broken feature.
- **Structured findings depend on a backend honouring a schema.** Models return
  malformed JSON, and the fallback of recording prose must not quietly become the normal
  path; a parse failure rate worth knowing is a reason for the eval slice to measure it.
- **The shim doubles the number of paths a change can break.** Every behavioural change
  to the runner must be checked against the shell suite too, until the slice that retires
  it — which cannot happen before the submodule consumer has migrated.
