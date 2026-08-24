# SPEC: feat(agents): generate the vendor instruction files from one source

## Problem

`CLAUDE.md` and `AGENTS.md` state overlapping instructions in two hand-maintained files,
so a rule can be updated in one and not the other, and adding a third agent means a third
copy nobody will keep in step.

## Design Decision

Make `AGENTS.md` the single source and generate every vendor file from it with
`mf agents sync`, then guard the result with a drift check.

`AGENTS.md` is the right source because it is the vendor-neutral one: agentic reviewers
already find it at the repository root, and the framework already sends it to non-agentic
backends. The vendor files become derived artifacts carrying a generated header that says
so and names the command that regenerates them.

Generation is not copying. Each vendor file is composed from the shared body plus a
vendor block for what only that tool needs — a path prefix, a tool-specific directive,
the frontmatter one format requires — declared in configuration rather than compiled in,
so a new agent is a config entry.

The drift check is what makes this hold. `mf check agents` regenerates in memory and
compares; a divergence fails, naming the file and the line. Without it the generated
files are a convention people bypass by editing the output, which is the current failure
with extra steps.

Derived files stay committed. A cloned repository must present `CLAUDE.md` to a session
that starts before anyone runs a command, so generating at read time would break
activation — the exact Gap this framework exists to close.

The submodule consumer complicates the shared body: its `CLAUDE.md` points into
`.standards/docs/standards/`, while this repository's points at `docs/standards/`. The
prefix is therefore a generation parameter, not a constant.

## Alternatives Considered

- **Keep `CLAUDE.md` as the source and generate the rest.** Rejected. It makes one
  vendor's format the canonical one, which is the coupling the whole architecture is
  removing, and it reads badly to every other tool.
- **Do not generate; keep a shared file each vendor file includes by reference.**
  Rejected. No agent tool resolves includes in its instruction file, so the content would
  simply not be loaded.
- **Generate at read time instead of committing the output.** Rejected. A session starts
  before any command runs; a `CLAUDE.md` that does not exist on clone is a standard that
  never activates.
- **Skip the drift check and trust the command.** Rejected. An edited output file is
  indistinguishable from a regenerated one without the check, and the drift would be
  discovered by an agent behaving oddly rather than by CI.

## Scope

- Includes:
  - `internal/agents`: the composition of a vendor file from the shared body and its
    vendor block.
  - The vendor set, declared in configuration: Claude Code, Gemini CLI, GitHub Copilot,
    Cursor.
  - The path-prefix parameter, so a submodule consumer generates files pointing at
    `.standards/`.
  - The generated header naming the source and the regenerating command.
  - `mf agents sync` and `mf agents check`, the latter wired into `mf check`.
  - Regenerating this repository's own `CLAUDE.md` from `AGENTS.md`.
  - Tests over fixture trees, written first.

- Does NOT include:
  - Changing what the instructions say. This slice changes where they live.
  - Generating skill or command files for any agent.
  - Reconciling a vendor file a human edited. The check reports drift; a human decides
    whether the source or the output was right.
  - Supporting a vendor whose instruction file needs content the shared body cannot
    express.

## Acceptance Criteria

- `generates_every_configured_vendor_file_from_agents_md`
- `applies_the_vendor_block_only_to_the_file_it_belongs_to`
- `applies_the_configured_path_prefix_to_every_standards_reference`
- `writes_a_generated_header_naming_the_source_and_the_command`
- `fails_the_drift_check_when_a_generated_file_was_edited_by_hand`
- `fails_the_drift_check_when_agents_md_changed_and_sync_was_not_run`
- `passes_the_drift_check_immediately_after_sync`
- `sync_is_idempotent`
- `adds_a_new_vendor_from_configuration_with_no_code_change`
- `this_repository_claude_md_is_generated_and_passes_the_drift_check`
- `claude_md_still_activates_the_standards_after_generation`

## Reproducibility

- `go test ./...`
- `mf agents sync && mf agents check`
- `bash scripts/test/docs-consistency.test.sh` — the existing guard that `CLAUDE.md`
  still activates the standards must stay green, because generation must not weaken it.

## Risks and Assumptions

- **Generated files are committed, so a reviewer sees noise in every diff that touches
  the source.** That is the price of activation-on-clone, and it makes the drift check
  the only thing standing between "regenerated" and "edited in place".
- **The shared body must be true for every vendor at once.** The first instruction that
  is right for one tool and wrong for another will be resolved by putting it in a vendor
  block, and vendor blocks are where a single source quietly becomes several.
- **A vendor changing its file format breaks generation silently.** The output is still
  written; the tool just stops reading it, and nothing here can detect that.
- **The path prefix assumes every standards reference is a path this slice recognises.**
  A reference written in prose rather than as a path will not be rewritten, and a
  submodule consumer will get a file pointing at a location that does not exist there.
- **Regenerating this repository's `CLAUDE.md` risks the activation guard.** That guard
  is load-bearing, and this is the first change that rewrites the file it protects.
