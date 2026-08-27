# SPEC: fix(check): close the gates that pass without checking

## Problem

A full audit of the tree found six places where a gate reports `ok` having
verified nothing — an exempt-path pattern that switches the Spec Gate off for
every change, a per-run path override that points a gate outside the repository,
an R1 attestation satisfied by a machine-wide git setting, an agent target that
makes `mf check` fail forever with a message naming the wrong cause, a Scope
section whose mandatory "Does NOT include" list may be empty, and — found while
closing the first of those — an exempt-path glob that means different things on
Windows and on Linux.

## Design Decision

Each hole is closed where its rule already lives, rather than by adding a
seventh place that knows about all six. The containment rule for configured
paths moves from the project decoder to the finished cascade, so it holds for
every layer that can write one instead of the single layer that happened to be
checked. The exempt-path prefix match stops accepting an empty prefix, which is
what made a one-character pattern mean "everything", and the glob beside it
moves from `filepath.Match` to `path.Match`: git names paths with slashes, and
only `path.Match` reads them that way on every platform — under `filepath.Match`
on Windows a `/` is an ordinary character, so `*` matched a path at any depth
and one committed list exempted different files there than in CI. The
attestation is read with the reader
whose meaning matches what its own comment claims. The agent-file check stops
skipping the empty value that `pathProblem` already has the right message for.
And "Does NOT include" is satisfied only by content that belongs to it, not by
whatever line follows it in the section.

## Alternatives Considered

- **Refuse `exempt_paths = ["*"]` by name in validation, leaving the matcher
  alone.** It names one spelling of the hole. `["**"]`, `["*x"]` trimmed against
  a path that begins with anything, and any future pattern trimming to empty all
  reach the same branch; the branch is what is wrong, so the branch is what is
  fixed. Validation of an empty pattern is added as well, not instead.
- **Let a per-run `MF_PATHS_*` escape the root, and document it as an escape
  hatch.** `r2_gate.md` states the opposite in as many words: a layer able to
  redirect `paths.standards` could make the same commit pass here and fail in
  CI. The override itself stays — it is deliberate, and `paths.agents_overlay`
  is declared specifically so one can land — only the escape is refused.
- **Keep reading the attestation from any scope and document that a global
  value counts.** R1's whole shipped chain is one in-session backend, so a
  global `mf.attestation.r1` would satisfy the internal review layer in every
  repository on the machine sitting at that commit. The attestation names a
  commit precisely so it cannot cover work it never saw; scope is the other half
  of that promise.
- **Have `mf check agents` report the empty `file` itself.** It is a
  configuration error, and the configuration layer already holds the rule and
  the wording for it; reporting it from the gate means every other consumer of
  that value meets it unreported.

## Scope

- Includes: `allExempt`'s prefix branch, its glob matcher, and validation of an
  empty exempt pattern; a containment check over the resolved `paths.*` values,
  applied after the whole cascade; `hasAttestation` reading only this
  repository's configuration; the empty `agents.<name>.file`;
  `hasDoesNotInclude`; tests for each.
- Does NOT include: the vendor-specific attribution pattern, the branch gate's
  unchecked description shape, the design gate's scale and typeface rules, the
  `inproc` backend kind, the Anthropic wire shape's missing temperature, or any
  of the documentation drift the same audit found — each is a separate change
  with its own decision to make, and this one is confined to gates that pass
  while checking nothing.

## Acceptance Criteria

- `a_pattern_that_trims_to_nothing_exempts_nothing` — `exempt_paths = ["*"]`
  leaves a change under `internal/` non-exempt, so the Spec Gate still applies.
- `an_empty_exempt_pattern_is_a_configuration_problem` — loading a project file
  with `exempt_paths = [""]` fails with a message naming the key.
- `a_directory_prefix_pattern_still_matches` — `docs/specs/*` still exempts
  `docs/specs/0049-x.md`, which this repository's own policy depends on.
- `a_glob_does_not_cross_a_separator_on_any_platform` — the same exempt list
  exempts the same files on Windows as on Linux; `*` reaches a root-level file
  and no deeper on both.
- `an_environment_path_that_leaves_the_root_is_refused` —
  `MF_PATHS_SPECS=../elsewhere` makes `config.Load` fail rather than resolve.
- `an_environment_path_inside_the_root_still_resolves` — `MF_PATHS_SPECS=spec`
  loads and is reported with the environment layer as its provenance.
- `an_attestation_set_globally_does_not_satisfy_r1` — with the key set only in
  the global scope, the in-session backend still reports no attestation.
- `an_attestation_set_locally_still_satisfies_r1` — unchanged behaviour for the
  documented command.
- `an_empty_agent_file_is_a_configuration_problem` — `agents.claude.file = ""`
  fails to load, naming `agents.claude.file`.
- `an_empty_does_not_include_fails_the_spec_gate` — a Scope whose "Does NOT
  include" line carries nothing, and is followed only by the "Includes" line,
  does not pass.
- `a_does_not_include_with_content_beneath_it_passes` — the multi-line form
  every spec in this archive uses is unaffected.

## Reproducibility

```sh
go test ./internal/check/ ./internal/config/ ./internal/cli/
go build -o mf ./cmd/mf && ./mf check
```

Go 1.26.7. The five holes were found by an audit of the tree at `f48cecd`
(v0.7.2) and each was confirmed by reading the code path end to end before
being written down here.

## Risks and Assumptions

- `exempt_paths = ["*"]` is a behaviour change for any repository that used it
  to switch the Spec Gate off. Nothing documents it as a way to do that, and a
  gate a one-character value silently disables is the defect; a repository that
  wants no Spec Gate says so by exempting the paths it means.
- Assumption: no repository sets `MF_PATHS_*` to a location outside its own
  tree, because a gate reading one has been reporting `ok` over an empty
  directory rather than doing anything useful.
- What would invalidate this spec: a decision that a per-run override may point
  a gate at another tree, which would make containment a project-layer rule
  rather than a rule about paths.
