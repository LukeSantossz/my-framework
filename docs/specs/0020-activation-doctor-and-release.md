# SPEC: feat(cli): activate a repository, report what resolved, and ship the binary

## Problem

Activation is a shell script that reports the toolchain but cannot say which backend a
role resolves to or whether a configured key exists, so the one real adopter wrote its
own test to check the gate was wired at all.

## Design Decision

Give the binary the four commands that make it installable and inspectable: `init`,
`doctor`, `hooks` and `upgrade`, plus the release pipeline that puts it on a machine.

`doctor` is the centrepiece and answers questions nothing answers today: which backend
and model each role resolves to, from which configuration layer each value came, whether
the cross-provider constraint can hold, whether the hooks path points at the versioned
directory, and whether every configured API key variable is actually set. It reports; it
never repairs, because a diagnostic that silently fixes things stops being a diagnostic.

`init` writes `.framework.toml`, wires the hooks path, records the framework version in
`.framework.lock`, and prompts for the Author Declaration so the cross-provider state has
a chance of being better than `unknown`. It is idempotent and it never overwrites a value
a human set.

The release pipeline builds for windows, linux and darwin on tag, publishes checksums,
and the standards corpus stays a runtime-free directory so the submodule consumer keeps
working with no binary at all. Two artifacts from one repository, as
`docs/adr/0005-go-substrate-single-binary.md` decided.

`upgrade` compares `.framework.lock` against the running binary's version and reports
what changed in the standards tree. It does not apply anything: an adopter's standards
are their content, and rewriting them from a release is how a framework loses a user's
edits.

## Alternatives Considered

- **Let `doctor` fix what it finds.** Rejected. The value of a diagnostic is that its
  output describes reality; one that repairs as it reads makes the second run disagree
  with the first, and hides the drift that mattered.
- **Have `upgrade` rewrite the standards from the release.** Rejected. Adopters edit
  their standards — that is the point of copying them — so applying a release over them
  destroys local intent. Reporting the difference leaves the merge with the person who
  knows.
- **Keep `scripts/setup.sh` as the activation entry point and shell out to it.** Rejected.
  It cannot resolve the configuration cascade, which is most of what activation now means,
  and two entry points would disagree about what "activated" is.
- **Publish only via `go install`.** Rejected. It requires the toolchain on every adopter
  machine, which is the prerequisite the single binary exists to remove; `go install`
  stays available for people who have Go.
- **Prompt for the API key during `init`.** Rejected. The configuration holds the name of
  a variable and never a key, and prompting for one invites pasting it into a file that
  is committed.

## Scope

- Includes:
  - `mf doctor`: role resolution with provenance, hook state, key-variable presence,
    toolchain report, and the cross-provider state it can compute.
  - `mf init [--interactive]`: project file, hooks path, `.framework.lock`, Author
    Declaration prompt.
  - `mf hooks install|uninstall|status`, wiring `pre-push` and `commit-msg`.
  - `mf upgrade`: version comparison and a report of standards-tree differences.
  - `mf author declare`, deferred from `0018`, writing the per-branch record the runner
    already reads.
  - `.github/workflows/release.yml`: tagged cross-platform build with checksums.
  - Tests over fixture repositories, written first.

- Does NOT include:
  - Repairing anything from `doctor`.
  - Applying standards changes from `upgrade`.
  - Removing `scripts/setup.sh`, which stays until the submodule consumer migrates.
  - Publishing to any package manager beyond GitHub Releases.
  - The status line, which is `0025`.
  - Signing or notarizing binaries.

## Acceptance Criteria

Doctor

- `names_the_backend_and_model_each_role_resolves_to`
- `names_the_configuration_layer_each_resolved_value_came_from`
- `reports_a_configured_api_key_variable_that_is_unset`
- `reports_whether_the_hooks_path_points_at_the_versioned_directory`
- `reports_the_cross_provider_state_and_why_it_is_not_verified`
- `exits_non_zero_only_on_a_condition_that_prevents_work_not_on_a_warning`
- `changes_nothing_it_reports_on`

Init and hooks

- `init_is_idempotent`
- `init_never_overwrites_a_value_a_human_set`
- `init_records_the_framework_version_in_the_lock_file`
- `hooks_install_points_the_hooks_path_at_the_versioned_directory`
- `hooks_status_reports_a_hooks_path_pointing_somewhere_else`
- `author_declare_writes_the_record_the_runner_reads`

Upgrade and release

- `upgrade_reports_standards_differences_and_applies_none`
- `upgrade_reports_a_lock_file_newer_than_the_running_binary`
- `the_release_workflow_builds_windows_linux_and_darwin_and_publishes_checksums`

## Reproducibility

- `go test ./...`
- `mf doctor` against this repository.
- `bash scripts/test/setup.test.sh` — must stay green, since `setup.sh` is not removed.

## Risks and Assumptions

- **`doctor` reports what configuration claims, not what a provider will do.** A key
  variable that is set may still be expired, and a model id that resolves may still be
  retired. The report can only ever say the configuration is coherent, and phrasing that
  does not oversell it is part of the work.
- **`init` prompting for the Author Declaration puts a question in front of someone who
  does not yet know why.** If it is answered carelessly the cross-provider state reads
  `declared` on a false basis, which is worse than `unknown`; the prompt has to make
  skipping it the easy and honest choice.
- **Two activation paths coexist.** `setup.sh` and `mf init` can disagree about what a
  repository needs, and a user who runs the old one will believe they are activated.
- **A release pipeline is new surface with no prior art here.** Cross-compilation is the
  easy part; tag hygiene, checksum publication and a broken release that people have
  already downloaded are the parts that need a rollback story this spec does not define.
- **`.framework.lock` records a version nobody is forced to update.** A stale lock makes
  `upgrade` report a difference that does not exist, and nothing detects a lock a human
  edited.
