# Status Line Contract

What a coding agent's status line must show, and in what order, so that the same
Developer reading the same repository on two machines — or through two different
agents — is steering by the same facts.

## Why It Exists

The status line is the only part of a session that is on screen continuously. It
is where context exhaustion, quota exhaustion, and being on the wrong branch
become visible before they become expensive. Left to each machine's own setup it
is hand-rolled state: configured once, differently everywhere, and absent on a
machine that was set up in a hurry. That is the Gap in the one place the
Developer looks most often.

This standard fixes the facts. It does not fix the pixels.

## The Contract

Five facts, in this order:

| # | Fact | Definition |
|---|---|---|
| 1 | Model with reasoning effort | The model serving the session, and the reasoning effort it is running at. |
| 2 | Context used | Tokens currently occupying the context window, as a proportion of that window. |
| 3 | Tokens spent | Tokens the session has consumed so far. |
| 4 | Quota | Utilization of the five-hour and the weekly usage windows. |
| 5 | Location | The working directory and the checked-out git branch. |

Order is normative — a Developer who has learned to read the third field as
spend should not have to relearn it per tool. Colors, glyphs, separators, and
column widths are not: they are the tool's business, and Codex's segments are
not restyleable in any case.

Session cost in currency is deliberately absent. Claude Code reports it and
Codex has no equivalent segment, so putting it in the contract would make the
Codex line look defective when it is merely honest. Tokens spent is the nearest
fact both tools hold, and it is what the contract carries.

## How Each Tool Renders It

The two mechanisms are not interchangeable, so there is no shared renderer to
write. Codex reads a declarative list of built-in segment names; Claude Code
runs a command and prints its stdout. The contract is what they have in common.

| Fact | Codex segment(s) | Claude Code |
|---|---|---|
| Model with effort | `model-with-reasoning` | Model display name from the session payload, effort from `settings.json` |
| Context used | `context-used`, `context-window-size` | Last main-chain turn's input tokens, against the window implied by the model id |
| Tokens spent | `used-tokens` | Input + output + cache-creation tokens over the session |
| Quota | `five-hour-limit`, `weekly-limit` | The OAuth usage endpoint, cached and refreshed in the background |
| Location | `current-dir`, `git-branch` | Working directory from the payload, branch from `git` |

Tokens spent excludes cache reads on the Claude Code side. A cache read is a
re-read of context already counted; including it would inflate the figure by
the size of the conversation on every turn and make it incomparable to what
Codex reports under the same name.

Codex renders the contract as:

```toml
[tui]
status_line = ["model-with-reasoning", "context-used", "context-window-size", "used-tokens", "five-hour-limit", "weekly-limit", "current-dir", "git-branch"]
status_line_use_colors = true
```

Claude Code renders it by running `mf statusline render`, which reads the
session payload on standard input and prints the five facts. The renderer is the
same binary that carries every other part of the framework, so applying the
contract installs nothing and adds no runtime.

A Node renderer under `scripts/statusline/` preceded this one. It is still
versioned in the framework's own repository — not written into an adopting one —
because a repository that consumed the framework as a submodule may still be
pointing `settings.json` at that path, and deleting it would break the line
rather than migrate it. It is retained for that migration and is not the
renderer this standard describes; `mf statusline apply` repoints the setting.

## Applying It

```sh
mf statusline apply
mf statusline revert
```

Several commands write outside the repository — `mf config set --machine` and
`mf config migrate` write the per-user configuration file, `mf review` and
`mf usage reset` write the usage total beside it, `mf statusline refresh`
updates the cached quota — so what makes `apply` a command of its own is not
that it leaves the repository. It is **what** it rewrites and **whose** it is.
Every one of those others writes a file this framework owns, created for this
framework's purpose. `apply` edits the coding agent's own configuration: a file
the Developer set up, that governs every project on the machine, and that no
part of this framework created.

That is why it is not a step of `mf init`. `mf init` changes this repository's
local state and nothing else, and a Developer running the documented
one-command activation in a fresh clone has consented to adopting a framework in
one repository — not to having the settings that follow them into every other
one rewritten. Consent to the second has to be asked for separately, which is
what a separate command is.

`mf statusline revert` is the other half of that consent: it restores the newest
backup `apply` took, per configuration file, so a Developer who tries the
contract and does not want it is not left reassembling their own settings by
hand. Apply pushes a backup and revert pops it, so a machine can be walked back
through as many applies as it took. With no backup left it reports that and
changes nothing, which is the right answer for a file `apply` created rather
than replaced.

What `apply` does, in both configuration files:

- Writes the contract, creating the file and its directory when absent.
- Backs up first, to a timestamped copy beside the original, whenever it is
  replacing a value that differs from the contract. Convergence is the point —
  a machine already carrying a hand-rolled status line is the one that most
  needed standardizing — and the backup is what keeps replacement recoverable.
- Changes nothing when the configuration already matches, so re-running the
  activation does not bury the original under generated copies.
- Leaves every unrelated key, section, and subsection intact.

No renderer is copied into the agent's configuration directory. A hand-written
status line script already there is left on disk; only the setting that points
at one is changed, and it is pointed at this binary by absolute path so it does
not depend on the PATH of whatever process the agent spawns it from. The one
file this framework does put there is the quota cache `mf statusline refresh`
writes — its own file, under its own name, holding the last reading so a render
never waits on the network.

## Declared Degradation

A fact the renderer cannot read degrades to a placeholder. It never fails the
line: an exception where the status bar goes would replace every fact with an
error message, which is worse than losing one.

- **No OAuth session.** The quota fact reads `usage n/a`. An API-key session has
  no plan windows to report.
- **Usage endpoint rate-limited or unreachable.** The last known figures stand,
  and the refresh backs off. Stale quota beats no quota.
- **Not a git repository, or a detached HEAD.** The location fact falls back to
  the short commit, then to the directory alone.
- **`settings.json` present but not a JSON object.** The run fails and the file
  is left untouched. Rewriting a file that cannot be parsed would discard every
  key in it, which is a worse outcome than not applying the contract.
- **No configuration directory for an agent.** That side is skipped with a
  message and the other is still applied.

## Scope

This is machine state, not repository state. Codex's `[tui]` section has no
per-project form, so a repo-local status line would bind one tool and not the
other — two standards wearing one name. Applying the contract therefore governs
every project on the machine, and rewrites a file the Developer owns. That is
what a separate command, and a `revert` beside it, are for.

The framework does not install Codex or Claude Code. Absence is reported and the
activation continues, as it already does for a missing reviewer backend in
`r2_gate.md`. Nothing else is required: the renderer is the framework's own
binary, so a machine that can run `mf` can render the line.

The Codex segment names in the block above are read from the installed build
rather than from a published schema, because Codex publishes none. That is a
standing risk this standard carries rather than solves: an upgrade that renames
a segment leaves the written configuration silently ignored, with no error and
no visible difference except the missing fact. The mitigation is to read the
line after upgrading Codex, and to treat a fact that stopped appearing as a
renamed segment rather than as a broken tool. Claude Code has no equivalent
exposure — its side is a command this framework owns.
