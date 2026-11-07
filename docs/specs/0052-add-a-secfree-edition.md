# SPEC: feat(edition): publish a sec-free edition that runs with no framework executable

## Problem

A Developer in a corporate environment cannot use this framework at all: its
gates run an unsigned `mf` binary that endpoint protection blocks, its hooks
fail closed when that binary is absent, its R2 and R3 send code to external
vendors, and its activation rewrites machine and git configuration — so the
standards never reach the environments where an AI-assisted change most needs
them.

## Design Decision

The framework gains a second **edition**, `secfree`, which is a documents-only
tree generated from `main` at each release and published on a dedicated orphan
branch `secfree`, tagged `secfree-vX.Y.Z`. There is one source: the standards
in `main` mark the regions that belong only to the full edition with
`<!-- mf:edition full -->` … `<!-- mf:edition end -->` — the same comment form
as the existing `<!-- mf:role … -->` markers — optionally naming a replacement
snippet kept under `docs/editions/secfree/`, and a new `mf edition build
secfree --out <dir>` assembles the tree on the maintainer's side, failing
closed on any marker it cannot resolve or any forbidden content left in the
output. In the edition, every rule the binary enforced becomes a procedure the
agent follows and the human checks in the PR: R1 stays the in-session review,
and R2 and R3 are performed by the **same model in isolated contexts** — R2
reviews the diff before push, R3 reviews the whole pull request, each in a
context that saw neither the authoring conversation nor the other review — and
the PR records them honestly as same-provider reviews, never as cross-provider.

## Alternatives Considered

- **A hand-maintained fork or branch.** Simplest to start. Rejected: the
  standards cite `mf` in more than sixty places and change every release, so a
  hand-kept copy drifts on the first release after it is cut, and nothing would
  detect the drift because the edition has no binary to run a gate.
- **A configuration profile of the full edition** (for example `mf init
  --profile corporate` with an empty R2 chain and no hooks). Rejected: it still
  requires running the `mf` binary on the Developer's machine, which is the
  control that blocks it.
- **A build mode of the binary** (a `secfree` build tag compiling out network
  and machine-configuration code). Rejected for the same reason: a smaller
  unsigned binary is still an unsigned binary, and the requirement is that no
  framework executable runs on the machine.
- **Script ports of the gates** (PowerShell or POSIX shell instead of `mf`).
  Rejected: script execution policy blocks them in the same environments, and
  `r2_gate.md` records why a second implementation of the gates was removed.
- **Publishing the edition as a release zip.** Rejected: the Developer asked
  for something reachable by `git clone`, and a downloaded archive is itself an
  artifact corporate controls inspect and often block.
- **Publishing on the same branch history as `main`.** Rejected: a plain clone
  of a branch that shares history with `main` brings Go sources, hooks and
  scripts with it; an orphan branch cloned with `--single-branch` brings only
  the edition's tree.

## Scope

- Includes:
  - The edition marker syntax (`<!-- mf:edition full -->`, optional
    `secfree=<snippet>`, `<!-- mf:edition end -->`) and its placement in every
    standard and in `docs/agents/instructions.md` wherever the text names the
    binary, a hook, a vendor backend, a forge app or a machine-configuration
    change.
  - `docs/editions/secfree/`: the edition's README, an adoption guide the agent
    follows to copy the files into a project without `mf init`, an
    `isolated_review.md` standard defining R2 and R3 by isolated context and
    how the PR records them, and the replacement snippets.
  - `mf edition build secfree --out <dir>`, which writes only these files:
    the edition README, `LICENSE`, `CLAUDE.md` and `AGENTS.md` pre-generated
    from the filtered instruction source with the author and reviewer sections
    both present, and under `docs/standards/` the filtered `INDEX.md`,
    `spec_method.md`, `code_conventions.md`, `var_method.md`, `github.md`,
    `crura_method.md`, `ai_guidelines.md`, `token_economy.md` and
    `isolated_review.md`.
  - A check in `mf check` that the edition builds from the current tree, so a
    broken or missing marker fails on `main` in CI rather than at release.
  - A release-workflow job that, on each `v*` tag, builds the edition from the
    tagged commit, commits it as the next commit on the orphan branch
    `secfree`, and pushes the tag `secfree-vX.Y.Z`.
  - Glossary entries in `CONTEXT.md` for Edition, Full Edition, Sec-free
    Edition and Isolated-Context Review.
- Does NOT include:
  - Any change to the full edition's behaviour, gates or generated files.
  - Code signing of the `mf` binary.
  - Excluded documents: `r2_gate.md`, `status_line.md`, `crux_method.md`,
    `design.md`, `skills_guidelines.md`, `CONTEXT.md`, `docs/specs/`,
    `docs/adr/`, `docs/eval/`, and the `gh`-dependent `docs/agents/`
    references.
  - Any executable, script, hook, submodule, LFS pointer or filter attribute in
    the edition tree.
  - Mirroring the branch to a corporate forge, or any process the edition
    assumes for issues.
  - Mechanical enforcement inside the edition: by design, nothing in it runs.

## Acceptance Criteria

- `build_writes_exactly_the_declared_file_set`
- `build_output_contains_no_executable_bit_script_extension_hook_or_gitmodules`
- `build_strips_every_full_only_region`
- `build_substitutes_the_named_snippet_for_a_replaced_region`
- `build_fails_on_an_unclosed_or_nested_edition_marker`
- `build_fails_on_a_snippet_name_that_does_not_exist`
- `build_fails_when_output_still_invokes_the_mf_binary_or_names_a_hook`
- `build_fails_when_output_names_a_vendor_backend_command_or_forge_app`
- `full_edition_generated_files_are_byte_identical_before_and_after_markers`
- `mf_check_fails_on_main_when_the_edition_would_not_build`
- `edition_instructions_carry_author_and_reviewer_sections`
- `isolated_review_requires_a_context_that_saw_neither_authoring_nor_the_other_review`
- `isolated_review_pr_record_names_same_provider_and_never_a_cross_provider_state`
- `release_job_commits_the_tagged_build_onto_the_orphan_secfree_branch`
- `secfree_tag_name_does_not_match_the_v_star_release_trigger`
- `single_branch_clone_of_secfree_checks_out_only_documents` (verified once by
  hand at the first release, with the command under Reproducibility)

## Reproducibility

```sh
mf edition build secfree --out /tmp/secfree
find /tmp/secfree -type f | sort
git clone --branch secfree --single-branch --no-tags \
  https://github.com/LukeSantossz/my-framework mf-secfree
git -C mf-secfree ls-files -s | awk '$1 != "100644"'   # expect no output
```

Versions: Go 1.26 per `go.mod`, `mf` at the commit under review, first
published edition `secfree-v0.9.0` or the next release after this change.

## Risks and Assumptions

- Assumption: the Developer's approved AI agent can open a fresh context (a
  subagent or a new session) that does not inherit the authoring conversation.
  Invalidated if the agent can only review inside the session that wrote the
  change; the edition would then have only R1.
- Assumption: plain Markdown and `LICENSE` files cloned from GitHub over HTTPS
  pass corporate controls. Invalidated if GitHub itself is blocked, which
  needs a mirror and is out of scope.
- Assumption: the `gh` CLI is not available in the target environments, so
  the issue-tracker and triage references are left out.
- Risk: same-provider review is weaker than cross-provider review; two contexts
  of one model share its blind spots. The edition accepts this and records it
  in every PR rather than hiding it.
- Risk: with no gate, the edition's rules hold only as well as the agent
  follows them and the human checks them. This is the requirement, not a
  defect, and the README of the edition says so.
- Risk: a `secfree-v*` tag must never match the release trigger `v*`, or the
  release would rebuild binaries from the edition branch; the tag name is
  chosen for that and covered by a criterion.
- Candidate ADR at the Gate: replacing cross-provider R2 with same-provider
  isolated-context review in this edition is hard to reverse for adopters,
  surprising without context, and the result of a real trade-off.
