# my-framework — development standards that activate, not just document

![Language](https://img.shields.io/badge/language-Bash%2FShell-4EAA25)
[![CI](https://github.com/LukeSantossz/my-framework/actions/workflows/ci.yml/badge.svg)](https://github.com/LukeSantossz/my-framework/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

my-framework exists to close the Gap: development standards that are written but never activated (loaded and obeyed) by the coding agent.

## What It Does

Turns a set of written development standards into ones an AI coding agent actually reads, follows, and is checked against.

- Spec-gated design before code: a `SPEC.md` passes the Spec Gate before implementation starts.
- Layered review: R1 internal (Superpowers), R2 cross-provider (a pre-push chain of reviewer backends), R3 automated PR review.
- One-command activation: `bash scripts/setup.sh` wires the R2 gate and the triage labels.
- One status line across agents: the same five facts, in the same order, in Claude Code and Codex.
- Docs-consistency invariants enforced in CI, catching orphaned or dangling standards on every push.
- PR and Issue templates plus triage labels, ready to adopt as-is.

## What It Is

A development-standards framework: versioned Markdown standards under `docs/standards/` plus the shell scripts that activate and guard them. It produces a repository where an AI agent's behavior — spec-first design, layered review, commit and PR conventions — is enforced rather than merely suggested, closing the Gap between documented intent and what the agent actually does.

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Bash + Markdown |
| Testing/CI | Shell test suites + GitHub Actions |

## Engineering Decisions

| Decision | ADR |
|---|---|
| Decision records flow: SPEC → ADR → README | [`docs/adr/0001-decision-records-flow.md`](docs/adr/0001-decision-records-flow.md) |
| Specs become durable under `docs/specs/`; ADRs stay the curated rationale home | [`docs/adr/0002-durable-spec-archive.md`](docs/adr/0002-durable-spec-archive.md) |
| CRUX review explainers are transient; ADRs stay the durable record | [`docs/adr/0003-crux-explainers-are-transient.md`](docs/adr/0003-crux-explainers-are-transient.md) |
| The R2 reviewer default is `gpt-5.6-terra`, chosen on benchmark | [`docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md`](docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md) |

## Getting Started

### Prerequisites

- git >= 2.40
- bash (Git for Windows works)
- gh CLI
- Optional, for the R2 cross-provider gate: Codex CLI >= 0.144.1, Gemini CLI, or any OpenAI-compatible endpoint (Ollama, LM Studio, DeepSeek, Groq, ...) plus Node

### Installation

Adopting in another repository: copy the standards and everything they reference — `docs/standards/`, `docs/adr/`, `docs/agents/`, the root `CLAUDE.md`, `AGENTS.md` (the R2 reviewer's binding instructions), and `CONTEXT.md` (rewrite the glossary for your domain) — plus `scripts/`, `.githooks/`, and the `.github/` templates and workflow, then run:

```sh
bash scripts/setup.sh
```

Use `bash scripts/setup.sh --interactive` to persist the reviewer model and reasoning effort locally; the token-economy choice is informational.

Use `bash scripts/setup.sh --reviewer` to configure the R2 reviewer chain for the whole
machine (`git config --global`): which backends to try and in what order, plus the
endpoint, model, and the *name* of the environment variable holding the API key for the
`openai` backend. A repository can still override any of it with
`git config --local`. The key itself is never stored — see `docs/standards/r2_gate.md`.

Use `bash scripts/setup.sh --statusline` to apply the status line contract
(`docs/standards/status_line.md`) to this machine's agent configuration: the same five
facts, in the same order, in Claude Code and Codex. It is the only part of the bootstrap
that writes outside the repository — to `$CLAUDE_HOME/settings.json` and
`$CODEX_HOME/config.toml` — which is why it is opt-in. A divergent configuration is
backed up to a timestamped copy and then replaced; a matching one is left alone.

`scripts/test/docs-consistency.test.sh` is this repository's self-test suite — it pins this repo's spec archive, git history, and README. Adopters validate with `bash scripts/test/docs-consistency.sh` and drop the self-test line from the copied CI workflow.

### Running

There is no long-running app: the framework runs as checks. Validate the standards tree at any time with:

```sh
bash scripts/test/docs-consistency.sh
```

Once wired by `scripts/setup.sh`, the R2 review gate runs automatically on `git push`. Preview the resolved chain without running any reviewer with
`R2_DRYRUN=1 bash scripts/r2-review.sh`.

### Tests

```sh
bash scripts/test/docs-consistency.test.sh
bash scripts/test/docs-consistency.sh
bash scripts/test/setup.test.sh
bash scripts/test/statusline.test.sh
bash scripts/test/r2-review.test.sh
```

`statusline.test.sh` needs Node for its two renderer cases; without it they report as
skipped and the rest of the suite still runs.

## Project Structure

```
my-framework/
├── docs/
│   ├── standards/     # binding development standards, read via INDEX.md
│   ├── adr/            # durable architecture decision records
│   └── specs/           # durable archive of approved SPEC.md changes
├── scripts/             # activation bootstrap, docs-consistency checks, test suites
│   ├── statusline/       # the Claude Code renderer of the status line contract
│   └── reviewers/        # R2 backend adapters (codex, gemini, openai)
├── .githooks/            # versioned pre-push hook wiring the R2 gate
└── .github/              # PR/Issue templates and the CI workflow
```

## Project Status

In development. Versioning policy: semver git tags, with `v0.1.0` tagged when the durable-specs batch merges. Adopters should record the tag they copied from.

## Known Issues & Limitations

- Executable bits are trusted from the git index, not the filesystem, because the Windows filesystem does not reliably report them; `git ls-files -s` is the source of truth instead.
- The R2 cross-provider gate needs at least one configured backend to be available. When none is, R2 does not run for that push, the runner says so, and CRURA human review substitutes per `docs/standards/crura_method.md`.
- The `codex` and `gemini` adapters classify quota, authentication and network failures by matching their tool's error text, which will drift when a vendor rewords it. A drifted pattern reads an unavailable backend as one that reviewed, so the chain stops early and names it rather than falling through silently.
- The `gemini` adapter is exercised against a stub, not the real CLI, because that CLI is not installed in this repository's development or CI environment. Its dispatch and contract are pinned; its real invocation is unverified, which is why the prompt flag is configurable.
- A local model behind the `openai` backend is a much weaker reviewer than a hosted frontier one, and on CPU-only hardware also a slow one. The default chain stays `codex` alone; the reported backend line is what keeps a fallback from passing as an equivalent review.
- The standards assume a single repository. A multi-repo setup with conflicting standards would need an authority hierarchy this framework does not yet define.
- Two commits reachable from `main` — `859daf2` and `757695d` — carry AI-attribution trailers, in violation of this repository's own rule against them. They are not rewritten, because both are already published and force-pushing over shared history to correct a commit message costs more than the defect it would fix; the honest record of the violation is itself useful. Recurrence is guarded instead: `scripts/test/docs-consistency.test.sh` fails a branch that adds a commit carrying such a line, so the rule holds going forward without reddening CI over two commits nobody will rewrite. The guard covers what a branch adds over `origin/main` or `main`, which is how work reaches this repository; a commit pushed straight to `main`, or a checkout where neither base resolves, is outside its range and stays uncovered.
- The status line contract is machine state, not repository state, because Codex's `[tui]` section has no per-project scope. Applying it with `--statusline` therefore governs every project on the machine, and a per-repository status line is not available for the Codex side at all.
- The Claude Code side of the status line needs Node: it runs the renderer and merges `settings.json`. Without Node that side is skipped with a message and only the Codex side is applied, so the two agents diverge on that machine until Node is installed.
- The Codex segment names written into `config.toml` were read out of the installed Codex build rather than a published schema. An upgrade that renames or drops one would leave the written configuration silently ignored — the line degrades, the tool does not break. The vocabulary as read is recorded in `docs/specs/0012-standardize-agent-status-line.md`.
- The quota fact on the Claude Code side reads an undocumented OAuth usage endpoint and needs an OAuth session; an API-key session shows `usage n/a`.
- Small deferred follow-ups (documented gaps not yet closed) are tracked in the issue backlog rather than in this README.

## Contributing

Fork the repository, branch as `type/TASK-NNN-description`, write tests before implementation, and use Conventional Commits. Open a Pull Request following the PR Model in `docs/standards/github.md`.

## License

MIT, see [`LICENSE`](LICENSE).
