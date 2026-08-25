# my-framework — development standards that activate, not just document

![Language](https://img.shields.io/badge/language-Go-00ADD8)
[![CI](https://github.com/LukeSantossz/my-framework/actions/workflows/ci.yml/badge.svg)](https://github.com/LukeSantossz/my-framework/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

my-framework exists to close the Gap: development standards that are written but never activated (loaded and obeyed) by the coding agent.

## What It Does

Turns a set of written development standards into ones an AI coding agent actually reads, follows, and is checked against.

- Spec-gated design before code: a `SPEC.md` passes the Spec Gate before implementation starts.
- Three review roles (R1 internal, R2 cross-provider, R3 on the pull request), each a chain of backends chosen in configuration. No role is bound to a vendor, and a chain that falls back says which backend actually reviewed.
- Seven deterministic gates in one command. No model is called: judging an artifact and judging a process are different tasks, and only the first is reliable.
- Reviewer provenance recorded per branch, so R2's cross-provider claim resolves to `verified`, `declared` or `unknown` instead of being assumed.
- One status line across agents: the same five facts, in the same order, in Claude Code and Codex.
- A CRUX change explainer, a terse prompt style with an enforced boundary, and a visual identity with a gate. Every standard here has something that performs it.
- PR and Issue templates plus triage labels, ready to adopt as-is.

## What It Is

A single Go binary, `mf`, plus versioned Markdown standards under `docs/standards/`. The binary reads the standards, runs the review roles, and enforces deterministically everything that can be enforced without a model. It produces a repository where an AI agent's behavior — spec-first design, layered review, commit and PR conventions — is enforced rather than merely suggested.

The standards tree stays a runtime-free directory. An adopter consuming it as a submodule needs no binary at all; the two artifacts are deliberately separate.

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26, one dependency |
| Standards | Markdown, read by the binary and by the agent |
| Configuration | TOML, split into a committed project layer and a machine layer |
| Testing/CI | `go test` plus shell suites, on GitHub Actions |

## Engineering Decisions

| Decision | ADR |
|---|---|
| Decision records flow: SPEC → ADR → README | [`docs/adr/0001-decision-records-flow.md`](docs/adr/0001-decision-records-flow.md) |
| Specs become durable under `docs/specs/`; ADRs stay the curated rationale home | [`docs/adr/0002-durable-spec-archive.md`](docs/adr/0002-durable-spec-archive.md) |
| CRUX review explainers are transient; ADRs stay the durable record | [`docs/adr/0003-crux-explainers-are-transient.md`](docs/adr/0003-crux-explainers-are-transient.md) |
| The R2 reviewer default is `gpt-5.6-terra`, chosen on benchmark | [`docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md`](docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md) |
| Go is the substrate; the harness ships as a single binary | [`docs/adr/0005-go-substrate-single-binary.md`](docs/adr/0005-go-substrate-single-binary.md) |
| Configuration is TOML, split by policy and machine state | [`docs/adr/0006-configuration-split-policy-and-machine.md`](docs/adr/0006-configuration-split-policy-and-machine.md) |
| Author provenance is a property of the change, not of the push | [`docs/adr/0007-author-provenance-belongs-to-the-change.md`](docs/adr/0007-author-provenance-belongs-to-the-change.md) |
| CRURA becomes triggered review, shipped instrumented | [`docs/adr/0008-crura-becomes-triggered-review.md`](docs/adr/0008-crura-becomes-triggered-review.md) |
| Deterministic checks derive their vocabularies from the standards | [`docs/adr/0009-checks-derive-vocabularies-from-standards.md`](docs/adr/0009-checks-derive-vocabularies-from-standards.md) |
| R1's provider constraint is per-backend, not a layer requirement | [`docs/adr/0010-r1-provider-constraint-is-per-backend.md`](docs/adr/0010-r1-provider-constraint-is-per-backend.md) |
| A derived visual identity is verified by fingerprint, not asserted | [`docs/adr/0011-a-derived-visual-identity-is-verified-not-asserted.md`](docs/adr/0011-a-derived-visual-identity-is-verified-not-asserted.md) |

## Getting Started

### Prerequisites

- git >= 2.40
- Go >= 1.26, to build from source. A released binary needs nothing.
- gh CLI, for the triage labels and for R3 posting to a pull request
- Optional, for the R2 cross-provider gate: Codex CLI >= 0.144.1, Gemini CLI, or any OpenAI-compatible endpoint (Ollama, LM Studio, DeepSeek, Groq, ...)
- Optional, only to keep the retired Node status line renderer working for a submodule consumer that has not migrated: Node

### Installation

```sh
go install github.com/LukeSantossz/my-framework/cmd/mf@latest
```

Then, inside the repository you want to govern:

```sh
mf init
mf author declare --provider anthropic --model claude-opus-5
mf doctor
```

`mf init` writes the policy file if it is absent, points `core.hooksPath` at the versioned hook, and records which version of the framework this repository adopted. `mf author declare` is the one step that has to be repeated per branch: it records who wrote the change, which is what R2 checks the reviewer against. Without it the cross-provider state can be no better than `unknown`, and `unknown` does not satisfy R2. `mf doctor` reports what resolves, what is wired, and what is missing; it repairs nothing, because a diagnostic that fixes as it reads makes the second run disagree with the first.

Coming from an earlier version, `mf config migrate` takes over the deprecated `r2.*` git-config keys. It copies rather than moves and prints the commands to remove the originals, so the destructive half stays a human decision.

Adopting in another repository: copy the standards and everything they reference — `docs/standards/`, `docs/adr/`, `docs/agents/`, the root `CLAUDE.md`, `AGENTS.md` (the R2 reviewer's binding instructions), and `CONTEXT.md` (rewrite the glossary for your domain) — plus `.framework.toml`, `.githooks/`, and the `.github/` templates and workflows.

`bash scripts/setup.sh --interactive` persists the reviewer model and reasoning effort locally; the token-economy choice is informational. The shell bootstrap remains for consumers that have not migrated to the binary.

`mf statusline apply` applies the status line contract (`docs/standards/status_line.md`) to this machine's agent configuration: the same five facts, in the same order, in Claude Code and Codex. It is the only command that writes outside the repository — to the Claude Code settings file and the Codex config file — which is why it is a command of its own rather than part of `mf init`. A divergent configuration is backed up to a timestamped copy and then replaced; a matching one is left alone. The renderer is the `mf` binary itself, so nothing is installed and no runtime is required.

### Running

There is no long-running app: the framework runs as checks and as a pre-push gate.

```sh
mf check                      # the seven deterministic gates
mf review --role r2 --dry-run # the chain that would run, and what it would send
mf explain                    # a CRUX explainer for the current change
mf doctor                     # what resolves, what is wired, what is missing
```

Once `core.hooksPath` is wired, the R2 review runs automatically on `git push`. It is advisory: a reviewer that never ran is not a finding, so an expired quota or a missing tool never locks the repository. The run says so, and CRURA human review substitutes.

`mf eval` measures a backend against a corpus of diffs with planted defects. It reaches real providers, so it is deliberately not wired into CI.

### Tests

```sh
go test ./...
bash scripts/test/docs-consistency.test.sh
bash scripts/test/docs-consistency.sh
bash scripts/test/setup.test.sh
bash scripts/test/statusline.test.sh
bash scripts/test/r2-review.test.sh
```

`scripts/test/docs-consistency.test.sh` is this repository's self-test suite — it pins this repo's spec archive, git history, and README. Adopters validate with `bash scripts/test/docs-consistency.sh` and drop the self-test line from the copied CI workflow.

`statusline.test.sh` covers the retired Node renderer and the shell activation path that still installs it. It needs Node for its two renderer cases; without it they report as skipped and the rest of the suite still runs. The renderer this framework uses is `mf statusline render`, covered by `go test` and needing nothing but the binary.

## Project Structure

```
my-framework/
├── cmd/mf/               # the binary's entry point
├── internal/             # the harness: config, roles, backends, checks, report
├── docs/
│   ├── standards/        # binding development standards, read via INDEX.md
│   ├── adr/              # durable architecture decision records
│   ├── specs/            # durable archive of approved SPEC.md changes
│   ├── agents/           # the single source the vendor instruction files are generated from
│   └── eval/corpus/      # diffs with planted defects, for measuring a backend
├── .framework.toml       # committed policy: roles, backends, checks
├── scripts/              # the shell bootstrap and test suites, kept for unmigrated consumers
├── .githooks/            # versioned pre-push hook wiring the R2 gate
└── .github/              # PR/Issue templates, CI, release and review workflows
```

## Project Status

The harness is merged to `main` and unreleased. Tags `v0.1.0` through `v0.3.0` are the earlier standards-only releases, from before the binary existed; a build installed from source reports its version as `0.0.0-dev` on purpose, so a lock file written by one is never mistaken for a released adoption. Versioning policy is semver git tags, and adopters should record the tag they copied from.

## Known Issues & Limitations

- Executable bits are trusted from the git index, not the filesystem, because the Windows filesystem does not reliably report them; `git ls-files -s` is the source of truth instead.
- The R2 cross-provider gate needs at least one configured backend to be available. When none is, R2 does not run for that push, the runner says so, and CRURA human review substitutes per `docs/standards/crura_method.md`.
- The `codex` and `gemini` backends classify quota, authentication and network failures by matching their tool's error text, which will drift when a vendor rewords it. A drifted pattern reads an unavailable backend as one that reviewed, so the chain stops early and names it rather than falling through silently.
- The `gemini` backend is exercised against a stub, not the real CLI, because that CLI is not installed in this repository's development or CI environment. Its dispatch and contract are pinned; its real invocation is unverified, which is why the prompt argument is configurable.
- A reasoning model behind an `api` backend has volatile latency, so the wall-clock budget (`review.timeout_seconds`, default 240s) is reachable rather than guaranteed. Measured against `deepseek-v4-flash`: 8 KB of diff took 27s, 30 KB took 85s, 40 KB took 163s once and exceeded 240s the next time, and 112 KB never returned. Exceeding the budget counts as unavailability, so the chain advances and the push is never held hostage to a slow reviewer.
- A local model is a much weaker reviewer than a hosted frontier one, and on CPU-only hardware also a slow one. The reported backend line is what keeps a fallback from passing as an equivalent review.
- The deprecated `r2.openai.model` git-config key has no destination in the new key space and is not migrated. A per-backend model now lives on the backend it belongs to, and a machine layer cannot declare a backend, so the value is left where it is rather than given an invented home.
- The fingerprint table that would let R2 report `verified` rather than `declared` ships empty. Guessing which environment variables a vendor's agent sets would be inventing environment variables, which `docs/standards/ai_guidelines.md` forbids, so an adopter fills it in or accepts `declared` as the best case.
- The standards assume a single repository. A multi-repo setup with conflicting standards would need an authority hierarchy this framework does not yet define.
- Two commits reachable from `main` carry AI-attribution trailers, in violation of this repository's own rule against them. They are not rewritten, because both are already published and force-pushing over shared history to correct a commit message costs more than the defect it would fix; the honest record of the violation is itself useful. Recurrence is guarded instead: `scripts/test/docs-consistency.test.sh` fails a branch that adds a commit carrying such a line. The guard covers what a branch adds over `origin/main` or `main`, which is how work reaches this repository; a commit pushed straight to `main`, or a checkout where neither base resolves, is outside its range and stays uncovered.
- The status line contract is machine state, not repository state, because the Codex TUI section has no per-project scope. Applying it with `mf statusline apply` therefore governs every project on the machine, and a per-repository status line is not available for the Codex side at all.
- Two status line implementations coexist: `mf statusline render` and the Node renderer under `scripts/statusline/`. They are held to the same contract by their own test suites, not by a shared one, so they can drift apart. The Node renderer is retained only until the submodule consumer has migrated, and removing it is what closes this.
- The Codex segment names written into its config file were read out of the installed Codex build rather than a published schema. An upgrade that renames or drops one would leave the written configuration silently ignored — the line degrades, the tool does not break. The vocabulary as read is recorded in `docs/specs/0012-standardize-agent-status-line.md`.
- The quota fact on the Claude Code side reads an undocumented OAuth usage endpoint and needs an OAuth session; an API-key session shows `usage n/a`.
- `mf explain` sends a diff to a model and gets prose back, and nothing checks that prose against the code. An explainer that confidently describes behaviour the change does not have is worse than no explainer, which is why it stays advisory and is read against the diff rather than instead of it.
- The design gate reads colour literals with a regular expression, not a CSS parser. It finds hex and `rgb()` values, which are the forms this repository writes; a named CSS colour, an `oklch()` call or a computed value would pass unnoticed, so `docs/standards/design.md` forbids those forms rather than the gate pretending to catch them.
- The design standard's fingerprint check proves that the source entry's literal colours and typefaces are not reused. It cannot prove independence of design: direction is not a value, and a layout or a restraint cannot be fingerprinted. The source entry also carries no version, so a later read may differ from the one the fingerprints describe.
- The Token Economy's terse boundary is enforced only where the harness composes the prompt. A person writing a commit message or a PR body by hand is outside it, so that part of the rule remains a discipline rather than a check.
- `mf eval` grades by matching findings against planted defects, never by a model judging a model. The numbers are self-reported, measure this corpus and these prompts, and are not comparable to an independent evaluation.
- Small deferred follow-ups (documented gaps not yet closed) are tracked in the issue backlog rather than in this README.

## Contributing

Fork the repository, branch as `type/TASK-NNN-description`, write tests before implementation, and use Conventional Commits. Open a Pull Request following the PR Model in `docs/standards/github.md`.

## License

MIT, see [`LICENSE`](LICENSE).
