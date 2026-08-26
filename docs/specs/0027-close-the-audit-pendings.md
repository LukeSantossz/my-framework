# SPEC: fix(harness): close the pendings a full-codebase audit found

## Problem

An audit of the whole repository — five independent reviewers over the Go source, the
documentation corpus, the shell and CI layer, the adoption path, and the security and
enforceability of the harness — found roughly seventy distinct defects behind a fully
green build. `go build`, `go vet`, `go test ./...`, four shell suites, `mf check` and
`mf doctor` all pass, and none of them can see any of it, because every finding is a
silent divergence rather than a failure.

They share one cause. The rebuild in `0014` replaced a shell framework with a Go
harness, and the shell framework was never removed. Removing it retires the
compatibility both `0014` and `0018` specified for it — the shell shim that
execs the binary, and the `SKIP_CODEX_REVIEW` and `CODEX_REVIEW_BASE`
variables — so those acceptance criteria are superseded here and are not
carried forward. The repository now carries two
implementations of the same gate. The documentation describes the shell one, the
binary implements the other, and the two have drifted on the details that matter:
which chain runs, which environment variables are honoured, and whether a finding can
block a push. Around that seam sit three failures that make the harness unfit to apply
to another project:

- **The gates run nowhere.** `mf check` is invoked by no workflow and no hook. Every
  rule this repository calls binding — the Spec Gate, the commit and branch vocabularies,
  `docs/adr/0009`, the design gate `docs/adr/0011` calls binding — is enforced by nothing.
  The repository does not gate itself.
- **The documented adoption path does not work.** `mf init` scaffolds a project file
  whose empty role chain cannot take effect, so the next command an adopter runs fails
  naming a backend they never configured. The standards directory is hardcoded, so the
  one real downstream adopter, which consumes this repository as a `.standards`
  submodule, cannot run `mf check` at all.
- **The configuration split does not hold.** `docs/adr/0006` says a project names
  providers and only a machine defines how to reach them, but the machine layer cannot
  define a backend. The four-command recipe in the header of `.framework.toml` exits
  successfully four times and changes nothing.

## Design Decision

Delete the shell path. `scripts/r2-review.sh`, `scripts/reviewers/`, `scripts/setup.sh`
and the shell test suites go, and the `mf` binary becomes the only way any gate runs.
The dual implementation is what produced the drift, and a guard that compares the two is
a permanent tax on every future backend change; removing one side removes the class.

That set includes `scripts/test/docs-consistency.sh`, and this spec supersedes the
decision in `0019` that held it back. `0019` kept the script out of its own deletion
scope on one stated condition — that the submodule consumer running it directly had no
migration path yet — and this spec is what supplies that path: `mf check docs` already
ports the invariants the script enforced, and the configurable standards directory above
is what lets a `.standards` consumer run the checks at all. The condition is met, so the
exception ends with the reason for it rather than outliving it; keeping the script now
would restore, in miniature, the dual implementation this decision exists to remove.
`0019` is otherwise untouched: its five checks ship, its record stands as approved, and
only the retention it scoped out is overtaken here.

Give the machine layer backends. This is what `docs/adr/0006` already decided and the
loader never implemented, and it is what lets a role chain be completed by a machine or
a CI secret instead of by editing committed policy. It also makes R3 reachable in CI,
which today spends a runner on every pull request to report that it did not run.

Wire the gates to something. `mf check` runs in CI and from a hook, and the hook fails
closed: a gate that cannot find its runner must say so and stop the push, never exit
zero in silence.

Then make the documentation true. That pass comes last, because it records what the
earlier ones decide rather than predicting it.

## Alternatives Considered

- **Keep both paths and add a drift guard.** Rejected. The existing guard compares
  `setup.sh` to `r2-review.sh` and never reads `.framework.toml`, which is precisely how
  the shell and the binary came to disagree about the shipped chain without any test
  noticing. A guard covering the real surface would have to encode every backend field in
  a third place, and every future backend change would have to be made twice, forever.
- **Keep the shell path as a no-binary fallback.** Rejected. It is the option that
  preserves the failure it is meant to insure against: the fallback is the path the
  documentation already describes as the operational gate, so keeping it keeps the
  documentation pointing at the wrong implementation. An adopter without the binary is
  better served by the prebuilt release binaries, which already exist for five platforms
  and are simply undocumented.
- **Leave backends in the project layer and drop the recipe from `.framework.toml`.**
  Rejected. It resolves the contradiction by retreating from `docs/adr/0006` rather than
  implementing it, and it leaves a committed file as the only place a reviewer command
  can be named — which is also the trust boundary that makes an arbitrary `command` in a
  cloned repository executable on a contributor's machine.
- **Fix only what blocks adoption and defer the rest.** Rejected at the Developer's
  decision. The stated goal of the work is a repository that is clean to apply, and a
  documentation corpus that contradicts the binary is not clean — it is the same defect
  class that produced the audit.

## Scope

- Includes: removal of the shell review path and its suites; backends in the machine
  configuration layer; an empty project list that can override a lower layer; a
  configurable standards directory for every gate that reads one; `mf check` wired into
  CI and into a hook that fails closed; the activation defects around `core.hooksPath`,
  repository discovery and the `mf init` scaffold; the backend-runtime defects in error
  classification, prompt selection, timeouts and the `in-session` kind; the statusline
  writes that can corrupt a user's Codex configuration; CI matrix, release gating and
  version-stamp verification; and a documentation pass reconciling the corpus with the
  binary.
- Does NOT include: new review roles or backends beyond those already declared; a
  redesign of the CRUX explainer or the design gate; changes to the standards' content
  where the standard and the code already agree; any change to the eval corpus's cases;
  publishing a release.

## Acceptance Criteria

- `mf_check_runs_in_ci_and_fails_the_job_on_a_violation`
- `pre_push_hook_fails_closed_when_its_runner_is_absent`
- `project_role_chain_declared_empty_overrides_the_built_in_default`
- `machine_layer_backend_completes_a_role_chain_the_project_names`
- `check_reads_standards_from_a_configured_directory_outside_docs_standards`
- `upgrade_reports_missing_standards_as_missing_rather_than_as_differing`
- `cli_backend_exiting_non_zero_without_a_pattern_match_is_not_recorded_as_a_review`
- `explain_role_reaching_a_cli_backend_receives_the_explainer_prompt`
- `cli_backend_is_bound_by_the_configured_review_timeout`
- `project_backend_declaring_a_command_is_refused_by_the_loader`
- `statusline_apply_leaves_a_codex_config_with_a_commented_tui_header_parseable`
- `init_outside_a_git_repository_refuses_rather_than_scaffolding_into_the_working_directory`
- `hooks_install_refuses_to_overwrite_a_hooks_path_it_does_not_own`
- `hooks_uninstall_is_idempotent`
- `doctor_reports_hooks_as_unwired_when_only_a_global_hooks_path_is_set`
- `release_fails_when_the_built_binary_does_not_report_the_tag`
- `docs_corpus_names_no_command_or_configuration_key_the_binary_does_not_have`

## Reproducibility

```sh
go build ./... && go vet ./... && go test ./...
mf check
mf doctor
```

The adoption path is verified against a scratch repository rather than this one, since
this repository already carries every file an adopter is missing:

```sh
git init /tmp/adopt && cd /tmp/adopt && mf init && mf check && mf review --role r2 --dry-run
```

Versions: Go per `go.mod`; `gh` for the forge paths; the audit was performed against
`465cb7e` on Windows 11 with `core.autocrlf=true`, which is the configuration under which
the CRLF findings reproduce.

## Risks and Assumptions

- Assumption: no consumer depends on the shell entry points. The one known adopter takes
  this repository as a `.standards` submodule for its documents; the deletion is
  breaking for anyone invoking `scripts/r2-review.sh` directly, and the release that
  carries it must say so.
- Assumption: the prebuilt release binaries are an acceptable substitute for the shell
  path for adopters without a Go toolchain.
- Assumption: making the hook fail closed is wanted. It converts a silent no-op into a
  blocked push, which is the point, but it will stop pushes that previously passed.
- Risk: wiring `mf check` into CI will fail this repository's own history until every
  gate it enforces is satisfied. If a gate proves to encode a rule the project does not
  actually want, the standard is what changes, not the gate.
- Risk: `* text=auto eol=lf` requires a one-time renormalization that touches every file
  and will conflict with any work in flight. It is applied once, after the code settles.
- What would invalidate this spec: a decision to keep the shell path, or a decision that
  backends must remain committed policy. Either one returns the configuration split to
  the state `docs/adr/0006` describes and this spec implements.
