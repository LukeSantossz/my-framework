# SPEC: chore(release): publish v0.8.0 so the closed gates reach the repositories they govern

## Problem

Six gates that reported `ok` while verifying nothing are fixed on `main` and
unreleased, so every repository pinning `v0.7.2` still has them — including one
whose exempt-path glob means different things on Windows than in CI, and an R1
attestation any machine-wide git setting satisfies.

## Design Decision

Cut `v0.8.0` from `main` as it stands, as a **minor** rather than a patch. The
release adds a command — `mf version` — and a repository upgrading to it gains a
capability, which is what separates a minor from the patch that `docs/specs/0045`
argued for. Five of the gate fixes also change what an existing configuration
does, and a release that only said "fixes" would not prepare an adopter for
that; the Scope below names each so the release notes can carry it.

## Alternatives Considered

- **Release as `v0.7.3`.** Rejected: `mf version` is new behaviour an adopter
  can call, and five configurations that loaded and passed under `v0.7.2` now
  fail. Numbering that as a patch tells a reader upgrading it that nothing they
  have can break.
- **Split into two tags — the fixes now, the command later.** Rejected: the
  fixes are what the consumers are waiting for, and holding a merged, tested
  command back to make a version number simpler puts the number ahead of the
  users.
- **Wait until the remaining audit findings are closed.** Rejected: those are
  nine separate changes with their own decisions to make, and the gate fixes are
  the ones whose absence is actively silent in three repositories.

## Scope

- Includes: the `v0.8.0` tag and the release it triggers; the README's Project
  Status and install commands; `.framework.lock`.
- Does NOT include: any change to what `v0.8.0` contains; bumping any consumer's
  pin, which is a change to that repository; the nine audit findings recorded in
  `docs/specs/0049`'s "Does NOT include" list.

### What an adopter upgrading from v0.7.2 has to know

Five fixes reject a configuration that previously loaded or passed. None is a
change of intent — each closes a gate that was reporting `ok` without checking —
but each is visible on upgrade:

- `checks.exempt_paths = ["*"]` no longer exempts everything. It now means what
  the glob says: files at the repository root, and no deeper.
- An `exempt_paths` glob no longer crosses a `/` on Windows, so a list that
  exempted more there than in CI now exempts the same set on both.
- A `paths.*` value from any layer — including `MF_PATHS_*` — that leaves the
  repository root is refused at load rather than resolved.
- `agents.<name>.file = ""` is refused at load, naming the key, instead of
  failing `mf check agents` forever with a message about drift.
- `mf.attestation.r1` is read from this repository's own git configuration only.
  A value recorded with `--global` no longer satisfies R1; the documented
  command has always been `git config --local`.

## Acceptance Criteria

- `release_publishes_five_binaries_and_a_checksum_file_for_the_tag`
- `readme_names_no_release_older_than_the_newest_tag_as_current`
- `readme_install_commands_name_the_newest_tag`
- `lock_records_the_version_this_repository_runs`
- `the_tagged_binary_reports_the_tag` — the release workflow's own smoke test,
  which reads `mf doctor`'s first line; `mf version` prints the same string.

## Reproducibility

```sh
gh release view v0.8.0 --json assets -q '.assets|length'   # 6
mf version                                                 # mf v0.8.0
mf doctor                                                  # first line: mf v0.8.0
```

Versions: Go 1.26.7, `mf` at the tagged commit.

## Risks and Assumptions

- Risk: a consumer pinned to `v0.7.2` whose `.framework.toml` relies on one of
  the five rejected configurations fails on upgrade rather than on adoption. The
  failure is at load, names the key, and is the point — a gate that was passing
  without checking had to start failing somewhere.
- Assumption: none of the three initialised consumers uses `exempt_paths = ["*"]`
  or an empty `agents.<name>.file`; verified before their pins are bumped, which
  is a separate change in each of those repositories.
- Assumption: the R2 chain's state does not gate the release. `codex` is out of
  quota until 2026-09-26 and reports itself unavailable, which the chain is
  built to survive.
