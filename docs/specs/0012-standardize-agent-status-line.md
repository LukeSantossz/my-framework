# SPEC: feat(scripts): standardize the agent status line across Claude Code and Codex

## Problem

The status line each coding agent shows is machine-local, hand-configured state, so
the same Developer reading the same repository on two machines sees two different
sets of facts about the session, and nothing in the framework says which facts a
session must display.

## Design Decision

Standardize the *segment contract* — which facts appear, in which order — and let
each tool render it with its own mechanism, because the two mechanisms are not
interchangeable. Codex reads a declarative list of built-in segment names from
`[tui] status_line` in `$CODEX_HOME/config.toml`; Claude Code runs an arbitrary
command whose stdout is the line. There is no shared renderer to write, so a
standard that promised byte-identical output would be promising something neither
tool can deliver. What both can deliver is the same facts in the same order, and
that is what the standard fixes.

The contract is five facts, in this order: model with reasoning effort, context
consumed against the window, tokens spent this session, quota remaining in the
five-hour and weekly windows, and where the session is (directory and git branch).
Codex renders it as `["model-with-reasoning", "context-used",
"context-window-size", "used-tokens", "five-hour-limit", "weekly-limit",
"current-dir", "git-branch"]`. Claude Code renders it through
`scripts/statusline/claude-statusline.js`, versioned in this repository and
installed to `$CLAUDE_HOME`, which reads the session JSON on stdin, the transcript
for token counts, and the OAuth usage endpoint for the quota windows.

Session cost in dollars is deliberately not in the contract even though Claude
Code supplies it, because Codex has no cost segment; `used-tokens` is the nearest
fact both tools hold, so the contract carries tokens and both sides show the same
thing. This is the one place where the richer tool is held to the poorer one, and
it is the price of the contract meaning anything.

Applying the contract writes outside the repository, to two files this framework
has never touched: `$CLAUDE_HOME/settings.json` and `$CODEX_HOME/config.toml`.
That boundary is crossed only on request. `bash scripts/setup.sh` keeps its
current repo-local behavior exactly; `bash scripts/setup.sh --statusline` is what
reaches into the machine. When it does reach, it converges: an existing divergent
configuration is backed up with a timestamped copy and then replaced, because a
standard that skips every machine already configured would standardize only fresh
machines, which are the ones that needed it least.

## Alternatives Considered

- **Apply the status line on every `setup.sh` run**: rejected — the bootstrap's
  contract today is that it changes this repository's local state and nothing
  else. Silently rewriting a global config that governs every other project on
  the machine is a different kind of act, and a Developer running the documented
  one-command activation in a cloned repository has not consented to it. The flag
  is what makes the consent explicit.
- **Document the canonical configuration and let the Developer copy it**:
  rejected — this is exactly the Gap the framework exists to close. A standard
  that depends on someone hand-editing TOML and JSON on each new machine is a
  written standard that never activates.
- **Preserve an existing status line and only warn**: rejected at the Developer's
  decision — it converges nothing. The machines that most need standardizing are
  the ones already carrying a hand-rolled line, and a warning leaves them
  divergent indefinitely. The timestamped backup is what makes replacement
  recoverable, so the destructive edge is blunted without giving up convergence.
- **Ship a repo-local status line instead of a global one**: rejected — Claude
  Code would accept it via `.claude/settings.json`, but Codex's `[tui]` section
  has no per-project form, so this halves the standard. A contract that binds one
  tool globally and the other per-project is two standards wearing one name.
- **Keep session cost in the contract and accept the asymmetry**: rejected at the
  Developer's decision in favor of `used-tokens`. An asymmetric segment makes the
  Codex line look defective when it is merely honest, and every reader has to
  learn the exception.
- **Write a POSIX-shell renderer with no network calls, dropping the quota
  segment**: rejected — the quota windows are the segments a Developer actually
  steers by, and Codex renders them natively. Dropping them to avoid a Node
  dependency would leave the Claude line strictly poorer than the Codex line on
  the same machine.

## Scope

- Includes:
  - `scripts/statusline/claude-statusline.js`: the versioned Claude Code renderer
    of the contract, in English, degrading to a placeholder for any fact it cannot
    read rather than failing the line.
  - `scripts/setup.sh`: multi-flag argument parsing, and a `--statusline` path
    that installs the renderer into `$CLAUDE_HOME`, merges `statusLine` into
    `$CLAUDE_HOME/settings.json`, and writes `[tui] status_line` plus
    `status_line_use_colors` into `$CODEX_HOME/config.toml`, backing up either
    file before replacing a divergent value.
  - `scripts/test/statusline.test.sh`: one test per Acceptance Criterion, run
    against sandboxed `CLAUDE_HOME` and `CODEX_HOME` directories.
  - `docs/standards/status_line.md`: the contract, its per-tool rendering, the
    definition of each fact, and the declared degradation when a fact is
    unavailable.
  - `docs/standards/INDEX.md`: the new document in Documents and Reading Order,
    plus the System Rule that names the contract as opt-in machine state.
  - `CONTEXT.md`: the Status Line Contract term in the glossary.
  - `README.md`: the flag in Installation, the suite in Tests, the directory in
    Project Structure, and the Node and global-config limitations in Known
    Issues.
  - `.github/workflows/ci.yml`: the new suite in the Shell tests step.
- Does NOT include:
  - Changing what `bash scripts/setup.sh` does without the flag. Its output and
    its effects stay byte-for-byte what they are today.
  - Any per-project status line. The contract is machine state, because Codex
    offers no other scope.
  - Colors, glyphs, or column widths as normative. The contract fixes which facts
    appear and their order; how a tool draws them is the tool's business, and
    Codex's segments are not restyleable anyway.
  - Session cost in dollars, in either tool.
  - Installing Node, Codex, or Claude Code. The bootstrap reports a missing
    toolchain and continues, as it already does for `codex` and `gh`.
  - Reconciling the renderer with the Developer's pre-existing
    `~/.claude/statusline.js`. That file is personal machine state; the framework
    installs its own copy under its own name and points `settings.json` at it,
    leaving the original on disk.
  - Any change to the R1/R2/R3 review layers, the Type Table, or the spec and ADR
    numbering rules.

## Acceptance Criteria

- setup_without_flag_leaves_global_state_untouched: a default `setup.sh` run
  against sandboxed `CLAUDE_HOME` and `CODEX_HOME` creates and modifies nothing in
  either.
- statusline_writes_codex_contract: with no `config.toml` present, the run creates
  one whose `[tui] status_line` is the canonical eight-segment array and whose
  `status_line_use_colors` is `true`.
- statusline_replaces_divergent_codex_segments: an existing `[tui]` section with a
  different `status_line` ends with the canonical array, and its other keys, its
  `[tui.model_availability_nux]` subsection, and every unrelated section survive
  unchanged.
- statusline_backs_up_replaced_codex_config: replacing a divergent `config.toml`
  leaves a backup file beside it whose content is byte-identical to the original.
- statusline_writes_claude_contract: with no `settings.json` present, the run
  creates one whose `statusLine.command` invokes the renderer installed under
  `CLAUDE_HOME`.
- statusline_merges_into_existing_claude_settings: an existing `settings.json`
  keeps every unrelated key, has its `statusLine` replaced, and is backed up
  first.
- statusline_installs_renderer_into_claude_home: the installed renderer is
  byte-identical to `scripts/statusline/claude-statusline.js`.
- statusline_is_idempotent: a second consecutive run changes neither file and
  writes no second backup.
- statusline_skips_claude_side_without_node: with `node` absent from `PATH`, the
  run reports the Claude side as skipped, still applies the Codex side, and exits
  0.
- statusline_fails_on_unreadable_claude_settings: a `settings.json` that is not
  valid JSON fails the run with a non-zero exit and is left on disk unmodified.
- statusline_composes_with_interactive: `--statusline --interactive` in either
  order is accepted, and an unknown flag is still rejected with a non-zero exit.
- renderer_emits_every_contract_segment: fed a representative session payload on
  stdin, the renderer's output contains the model, the context percentage, the
  spent-token count, the quota windows, and the directory with the git branch, in
  contract order.
- renderer_degrades_without_usage_cache: with no usage cache and no credentials,
  the renderer still emits every other segment and marks the quota segment as
  unavailable rather than failing.
- all_suites_green: all five suites and the docs-consistency check pass on the
  final tree.

## Reproducibility

Run, from the repository root, with git >= 2.40, bash (Git for Windows), and
Node >= 18 for the renderer tests:

```sh
bash scripts/test/codex-review.test.sh
bash scripts/test/setup.test.sh
bash scripts/test/statusline.test.sh
bash scripts/test/docs-consistency.test.sh
bash scripts/test/docs-consistency.sh
```

All pass, 0 failed. The suite exports `CLAUDE_HOME` and `CODEX_HOME` into
throwaway directories, so no test reads or writes the operator's real
configuration. The renderer tests set `MYFW_STATUSLINE_NO_REFRESH=1`, which
suppresses the background usage refresh, so no test performs a network call. No
randomness is involved.

The Codex segment vocabulary was read out of the installed
`codex.exe` (build `3135b80b111fd431`, 2026-07-14) rather than from
documentation: `app-name`, `project-name`, `current-dir`, `activity`, `status`,
`run-state`, `thread-title`, `git-branch`, `context-remaining`, `context-used`,
`context-window-size`, `five-hour-limit`, `weekly-limit`, `codex-version`,
`used-tokens`, `total-input-tokens`, `total-output-tokens`, `thread-id`,
`fast-mode`, `model`, `model-with-reasoning`, `reasoning`, `task-progress`.

## Risks and Assumptions

- Assumption: `CLAUDE_HOME` and `CODEX_HOME` default to `$HOME/.claude` and
  `$HOME/.codex`, and each tool honors its own environment override, so the
  sandboxed tests exercise the same code path a real run takes.
- Assumption: Codex's `used-tokens` counts the tokens the session has spent. The
  renderer computes the Claude-side equivalent as input plus output plus
  cache-creation tokens, excluding cache reads, and `status_line.md` states that
  definition so the two sides are comparable rather than merely similarly named.
- Risk: the Codex segment names are read from a binary, not a published schema,
  so an upgrade could rename or drop one and the written config would then be
  silently ignored by Codex. Accepted: the names are what the running build
  accepts, the alternative is guessing, and a wrong segment degrades the line
  rather than breaking the tool.
- Risk: the quota segment depends on an undocumented OAuth usage endpoint and on
  `.credentials.json` being present, so an API-key session shows it as
  unavailable. Accepted and declared: the renderer degrades that segment alone.
- Risk: rewriting `config.toml` by line editing rather than by a TOML parser
  could mangle an exotic file — a multi-line `status_line` array is handled, but
  an inline `[tui]` table written as `tui = { ... }` is not. Accepted: the backup
  makes it recoverable, and the tests pin the shapes the tools themselves write.
- What would invalidate this spec: Codex gaining a per-project `[tui]` scope, or
  a custom-command status line, either of which would reopen whether the contract
  should be repo-local and whether a single shared renderer is now possible.
