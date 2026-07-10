# my-framework — development standards that activate, not just document

![Language](https://img.shields.io/badge/language-Bash%2FShell-4EAA25)
[![CI](https://github.com/LukeSantossz/my-framework/actions/workflows/ci.yml/badge.svg)](https://github.com/LukeSantossz/my-framework/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

my-framework exists to close the Gap: development standards that are written but never activated (loaded and obeyed) by the coding agent.

## What It Does

Turns a set of written development standards into ones an AI coding agent actually reads, follows, and is checked against.

- Spec-gated design before code: a `SPEC.md` passes the Spec Gate before implementation starts.
- Layered review: R1 internal (Superpowers), R2 cross-provider (Codex pre-push gate), R3 automated PR review.
- One-command activation: `bash scripts/setup.sh` wires the R2 gate and the triage labels.
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
| R2 reviewer model: switch from gpt-5.5 to gpt-5.6-terra | [`docs/adr/0003-r2-reviewer-model-gpt-5-6-terra.md`](docs/adr/0003-r2-reviewer-model-gpt-5-6-terra.md) |

## Getting Started

### Prerequisites

- git >= 2.40
- bash (Git for Windows works)
- gh CLI
- Optional: Codex CLI 0.144.1, for the R2 cross-provider gate

### Installation

Adopting in another repository: copy the standards and everything they reference — `docs/standards/`, `docs/adr/`, `docs/agents/`, the root `CLAUDE.md`, `AGENTS.md` (the R2 reviewer's binding instructions), and `CONTEXT.md` (rewrite the glossary for your domain) — plus `scripts/`, `.githooks/`, and the `.github/` templates and workflow, then run:

```sh
bash scripts/setup.sh
```

Use `bash scripts/setup.sh --interactive` to persist the reviewer model and reasoning effort locally; the token-economy choice is informational.

`scripts/test/docs-consistency.test.sh` is this repository's self-test suite — it pins this repo's spec archive, git history, and README. Adopters validate with `bash scripts/test/docs-consistency.sh` and drop the self-test line from the copied CI workflow.

### Running

There is no long-running app: the framework runs as checks. Validate the standards tree at any time with:

```sh
bash scripts/test/docs-consistency.sh
```

Once wired by `scripts/setup.sh`, the R2 review gate runs automatically on `git push`.

### Tests

```sh
bash scripts/test/docs-consistency.test.sh
bash scripts/test/docs-consistency.sh
bash scripts/test/setup.test.sh
bash scripts/test/codex-review.test.sh
```

## Project Structure

```
my-framework/
├── docs/
│   ├── standards/     # binding development standards, read via INDEX.md
│   ├── adr/            # durable architecture decision records
│   └── specs/           # durable archive of approved SPEC.md changes
├── scripts/             # activation bootstrap, docs-consistency checks, test suites
├── .githooks/            # versioned pre-push hook wiring the R2 gate
└── .github/              # PR/Issue templates and the CI workflow
```

## Project Status

In development. Versioning policy: semver git tags, with `v0.1.0` tagged when the durable-specs batch merges. Adopters should record the tag they copied from.

## Known Issues & Limitations

- Executable bits are trusted from the git index, not the filesystem, because the Windows filesystem does not reliably report them; `git ls-files -s` is the source of truth instead.
- The R2 cross-provider gate requires a locally installed Codex CLI. Without it, R2 does not run for that push, and CRURA human review substitutes per `docs/standards/crura_method.md`.
- The standards assume a single repository. A multi-repo setup with conflicting standards would need an authority hierarchy this framework does not yet define.
- Small deferred follow-ups (documented gaps not yet closed) are tracked in the issue backlog rather than in this README.

## Contributing

Fork the repository, branch as `type/TASK-NNN-description`, write tests before implementation, and use Conventional Commits. Open a Pull Request following the PR Model in `docs/standards/github.md`.

## License

MIT, see [`LICENSE`](LICENSE).
