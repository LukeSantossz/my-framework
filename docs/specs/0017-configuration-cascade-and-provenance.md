# SPEC: feat(config): resolve configuration across four layers with per-layer provenance

## Problem

Configuration lives only in `git config`, which is never committed, so a project cannot
state its own review policy, credentials and endpoints have no enforced home, and no
command can tell a reader which layer a surprising value came from.

## Design Decision

Implement `docs/adr/0006-configuration-split-policy-and-machine.md` as the first Go
package, because every later slice resolves settings before it does anything else.

Two files, split by the nature of the data. `.framework.toml` at the repository root
holds policy and is committed. The machine file — `os.UserConfigDir()/framework/config.toml`,
which is `%AppData%` on Windows and `~/.config` elsewhere, overridable with
`MF_CONFIG_HOME` — holds machine state and is not.

The rule that makes the split enforceable rather than advisory is one sentence: **a
project names providers; only the machine defines how to reach them.** A backend in
`.framework.toml` says `provider = "deepseek"`; what URL that is and which environment
variable carries its key exists only in the machine file. So a project file that declares
an endpoint, an API key, or an API key variable name is a load error, not a value to be
overridden. Policy is portable, reachability is local, and a secret has no syntactic
place in the committed file at all.

Policy resolves as environment, then project, then machine, then the deprecated `r2.*`
git-config keys, then the built-in default. Machine-only settings have no project layer,
so precedence never applies to them — a project that names one is refused.

Every resolved value carries the layer it came from, and that provenance is reachable
from the command line. This is not ergonomics: `docs/specs/0013-detach-r2-from-codex.md`
rejected a config file partly because it creates a second place to look, and that
objection survives the move to Go intact. Provenance reporting is the entire mitigation,
so `mf config get` names the layer and `mf config list --provenance` prints the resolved
table with it. Without both, this slice is a net regression against `0013`.

Nothing is migrated on upgrade. `mf config migrate` moves the legacy keys when asked, and
until it is asked they keep resolving where they are, so an existing clone — including
the `.standards` submodule consumer — sees no change it did not request.

This slice resolves and reports configuration. It does not run a review, walk a chain, or
contact a provider.

## Alternatives Considered

- **Let a project override an endpoint when it wants to.** Rejected. The moment the
  project file can carry reachability, it can carry a credential, and "no secrets in the
  repository" goes back to being a convention people remember rather than a rule the
  loader enforces. Refusing the key outright is what makes the guarantee structural.
- **Merge both files into one schema and distinguish layers only by path.** Rejected. It
  reads more simply, but nothing then prevents a machine-only key from appearing in the
  committed file, which is the failure above.
- **Migrate the legacy `git config` keys automatically on first run.** Rejected. It
  silently rewrites a machine's configuration as a side effect of an upgrade, and the one
  known adopter consumes this repository as a submodule where an unrequested change is
  hardest to notice.
- **YAML or JSON instead of TOML.** Rejected. JSON has no comments, which a policy file
  people edit by hand needs, and YAML's implicit typing is a known source of
  configuration defects. TOML is unambiguous and its Go support is mature.
- **Report provenance only in `doctor`.** Rejected. The question "where did this value
  come from" is asked while looking at one value, so it has to be answerable from the
  command that prints one value.

## Scope

- Includes:
  - `internal/config`: the schema for both files, the loader, the merge with precedence,
    validation, and the provenance record attached to each resolved value.
  - The machine-only key set (provider endpoints, API key variable names, local paths),
    refused with a named error when found in the project file.
  - The deprecated `r2.*` git-config read layer.
  - `mf config get <key>`, `mf config list [--provenance]`, `mf config set <key> <value>
    [--project|--machine]`, `mf config validate`, `mf config migrate`.
  - `go.mod` with the module path, the Go version, and the TOML dependency.
  - Table-driven tests over fixture trees, written before the implementation.

- Does NOT include:
  - Running a review, walking a backend chain, or contacting any provider. The role
    runner is `0018`.
  - The deterministic checks, `doctor`, `init`, or hook wiring.
  - Reading or writing any credential. The configuration holds the *name* of an
    environment variable; resolving that name to a value belongs to whoever makes a
    request.
  - Migrating anything without being asked.
  - Removing `scripts/r2-review.sh` or any existing shell path.
  - A schema version migration mechanism. `version = 1` is recorded and an unknown
    version is refused; upgrading between versions has no second version to define yet.

## Acceptance Criteria

Resolution

- `resolves_a_policy_value_from_env_over_project_over_machine_over_legacy_over_default`
- `treats_an_empty_environment_override_as_unset_rather_than_as_an_empty_value`
- `reads_a_legacy_r2_git_config_key_when_no_project_or_machine_value_exists`
- `prefers_a_machine_value_over_a_legacy_git_config_key`
- `returns_the_built_in_default_when_no_layer_supplies_a_value`

The split

- `refuses_a_project_file_that_declares_a_provider_endpoint`
- `refuses_a_project_file_that_declares_an_api_key_variable_name`
- `refuses_a_project_file_that_declares_a_literal_api_key`
- `resolves_a_backend_provider_name_from_the_project_and_its_endpoint_from_the_machine`
- `refuses_a_backend_naming_a_provider_no_machine_file_defines`

Provenance

- `config_get_names_the_layer_the_value_resolved_from`
- `config_list_with_provenance_prints_every_resolved_value_with_its_layer`
- `names_the_legacy_layer_distinctly_from_the_machine_layer`

Writing and migration

- `config_set_writes_the_project_layer_by_default_and_the_machine_layer_on_request`
- `config_set_refuses_to_write_a_machine_only_key_into_the_project_file`
- `migrate_moves_legacy_git_config_keys_into_the_machine_file`
- `migrate_leaves_the_legacy_keys_resolvable_until_it_is_run`
- `migrate_is_idempotent`

Validation

- `config_validate_refuses_an_unknown_schema_version`
- `config_validate_refuses_a_backend_of_an_unknown_kind`
- `config_validate_reports_every_error_it_found_rather_than_only_the_first`
- `loads_a_repository_with_no_project_file_using_machine_and_default_layers_only`

## Reproducibility

- `go version` — the toolchain pinned in `go.mod`.
- `go test ./...`
- `go build ./cmd/mf`
- Existing shell suites must stay green, since this slice removes nothing:
  `bash scripts/test/docs-consistency.test.sh`, `bash scripts/test/r2-review.test.sh`,
  `bash scripts/test/setup.test.sh`, `bash scripts/test/statusline.test.sh`.

## Risks and Assumptions

- **Go is not installed in this environment at the time of writing.** The slice cannot be
  implemented test-first until it is, and writing Go that is never compiled would produce
  exactly the unverified code `ai_guidelines.md` forbids. The spec is therefore authored
  ahead of the toolchain, and implementation waits.
- **The machine file's location differs per operating system.** `os.UserConfigDir()`
  resolves to `%AppData%` on Windows and `~/.config` elsewhere, so documentation that
  names one path is wrong on the other platform. `MF_CONFIG_HOME` exists so a test and a
  confused user both have one answer.
- **The provenance record is only as honest as the loader.** If a value is transformed
  after resolution — defaulted, coerced, trimmed — and the record still names the layer
  it was read from, the report becomes subtly false. Provenance must therefore travel
  with the value through any transformation, not be attached at read time and forgotten.
- **Refusing machine-only keys in the project file will break someone's expectation.**
  A team that wants one shared endpoint has to put it on every machine or supply it by
  environment. That cost is accepted deliberately, because the alternative permits a
  credential-shaped value in a committed file.
- **Four layers is one more than anyone will hold in their head.** The provenance report
  is the mitigation, and it will be used only if it is cheap to reach; a report that
  needs a flag nobody remembers is a mitigation on paper.
- **The legacy layer has no end date.** Nothing in this slice removes it, so it will
  accumulate as permanent surface unless a later slice retires it deliberately, with the
  submodule consumer's migration confirmed first.
