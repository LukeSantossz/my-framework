# SPEC: feat(cli): report the build version without running the doctor

## Problem

`mf --version`, `mf -v` and `mf version` all answer `mf: unknown command` and
print the usage banner, so the only place the build reports its identity is the
first line of `mf doctor` — a command that also resolves the configuration,
probes every backend and reads the standards.

## Design Decision

Add a `version` command that prints exactly the string `mf doctor` already
prints on its first line, with `--version` and `-v` accepted as aliases for it.
The string is composed in one place — a `versionLine` helper both `runVersion`
and `runDoctor` call — so the two can never drift into reporting different
identities for the same binary. The command's subject is the binary rather than
a repository, so it joins the short list in `requiresRepository` that runs
outside one: asking a binary what it is must not depend on where it is standing.

## Alternatives Considered

- **A `--version` flag parsed before dispatch, with no `version` command.** A
  flag is what a person types and a subcommand is what a script calls; the issue
  reports all three forms being tried, and shipping one of them leaves the other
  two answering `unknown command`, which is the failure being closed.
- **Have `version` print more than `doctor`'s line — commit, build date,
  platform.** That changes what the version string contains, which the issue
  puts out of scope, and it re-opens the question of which of the two lines is
  authoritative. One string, printed by both, is the property worth having.
- **Point the reader at `mf doctor` in the usage banner instead.** Documentation
  cannot fix a command that exits 2: a CI step that runs `mf --version` still
  fails, and it fails with a usage banner that never mentions the version at all.

## Scope

- Includes: a `version` command; `--version` and `-v` as aliases; the shared
  helper that composes the string; a usage-banner entry; tests over all three
  spellings, over the identity with `doctor`'s first line, and over running
  outside a repository.
- Does NOT include: any change to what the version string contains, to how the
  release workflow stamps it, or to `mf doctor`'s own output.

## Acceptance Criteria

- `version_command_prints_the_build_version` — `mf version` exits 0 and prints
  `mf <version>`.
- `version_flags_are_aliases_of_the_command` — `mf --version` and `mf -v`
  produce byte-identical output to `mf version` and exit 0.
- `version_matches_the_doctor_first_line` — the line `mf version` prints equals
  the first line `mf doctor` prints for the same build.
- `version_runs_outside_a_repository` — `mf version` exits 0 with no repository
  root, rather than being refused as a command that reads one.
- `unknown_command_still_exits_two` — `mf frobnicate` is unchanged.

## Reproducibility

```sh
go test ./internal/cli/...
go build -o mf ./cmd/mf && ./mf version && ./mf --version && ./mf -v
```

Go 1.26.7. The version an unstamped build reports is `0.0.0-dev`, per
`internal/version`; the equality with `doctor` is what is asserted, not a
literal.

## Risks and Assumptions

- Assumption: no existing command or flag spells itself `-v`, so the alias
  claims nothing already taken. `mf review` takes `--role`, not `-v`.
- Assumption: printing the version outside a repository is safe because the
  string is resolved from the build, never from the tree.
- What would invalidate this spec: a decision to make the version string carry
  repository-dependent facts, which would put it back under
  `requiresRepository`.
