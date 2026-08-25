# my-framework — development standards that activate, not just document

![Language](https://img.shields.io/badge/language-Go-00ADD8)
[![CI](https://github.com/LukeSantossz/my-framework/actions/workflows/ci.yml/badge.svg)](https://github.com/LukeSantossz/my-framework/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

my-framework exists to close the Gap: development standards that are written but never activated (loaded and obeyed) by the coding agent.

## What It Does

Turns a set of written development standards into ones an AI coding agent actually reads, follows, and is checked against.

- Spec-gated design before code: a spec under `docs/specs/` passes the Spec Gate before implementation starts.
- Three review roles (R1 internal, R2 cross-provider, R3 on the pull request), each a chain of backends chosen in configuration. No role is bound to a vendor, and a chain that falls back says which backend actually reviewed.
- Seven deterministic gates in one command. No model is called: judging an artifact and judging a process are different tasks, and only the first is reliable.
- Those gates run where they can stop something — in CI, in a pre-push hook, and in a commit-msg hook that reads the message still under the author's cursor. All three fail closed: a gate that cannot find its runner says so and stops, rather than exiting zero in silence.
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
| Testing/CI | `go test` and `mf check`, on GitHub Actions across Linux, macOS and Windows |

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
| The deterministic gates are enforced by CI and by fail-closed hooks | [`docs/adr/0012-gates-are-enforced-by-ci-and-fail-closed-hooks.md`](docs/adr/0012-gates-are-enforced-by-ci-and-fail-closed-hooks.md) |

## Getting Started

### Prerequisites

- git >= 2.40
- gh CLI, for the triage labels and for R3 posting to a pull request
- Go >= 1.26. Required today, because the adoption below needs a build from a checkout; a prebuilt binary from the next tag onward will need nothing.
- Optional, for the R2 cross-provider gate: Codex CLI >= 0.144.1, Gemini CLI, or any OpenAI-compatible endpoint (Ollama, LM Studio, DeepSeek, Groq, ...)
- Optional, only to keep the retired Node status line renderer working for a submodule consumer that has not migrated: Node

### Installation

Three paths. **Once the next tag is published, prefer the prebuilt binary**: it is stamped with the tag it was released under, the release workflow refuses to publish a binary that does not report its own tag, and it needs no Go toolchain on the machine that runs the gates — which matters because a hook that cannot find `mf` now stops the commit.

```sh
# Prebuilt, with the checksum verified. Pick your platform from
# mf_<tag>_{linux,darwin}_{amd64,arm64} and mf_<tag>_windows_amd64.exe.
# --repo is required: you are not standing in a clone of this repository.
gh release download v0.4.0 --repo LukeSantossz/my-framework \
  --pattern 'mf_v0.4.0_linux_amd64' --pattern 'SHA256SUMS'
sha256sum --ignore-missing -c SHA256SUMS
# macOS ships no sha256sum; there the same check is:
#   shasum -a 256 --ignore-missing -c SHA256SUMS
install -m 0755 mf_v0.4.0_linux_amd64 ~/.local/bin/mf
```

Without `gh`, the same assets are at `https://github.com/LukeSantossz/my-framework/releases/tag/v0.4.0`, and `SHA256SUMS` covers every one of them.

```sh
# From the module proxy. Needs a Go toolchain; from the next tag onward the
# resulting binary reports the module version the toolchain recorded, so a lock
# file it writes names a real tag.
go install github.com/LukeSantossz/my-framework/cmd/mf@latest
```

```sh
# From a checkout. This is the only path that can perform the adoption below
# until the next tag is published.
git clone https://github.com/LukeSantossz/my-framework && cd my-framework
go build -o mf ./cmd/mf && install -m 0755 mf ~/.local/bin/mf
```

**Read this before choosing.** `v0.4.0` is the newest tag, and its `mf init` predates the adoption described in the next section: run it and you get `.framework.toml` and `.framework.lock` and nothing else — no standards, no hooks, no instruction files, and a `mf check` that stops on the standards it cannot find. `go install …@latest` resolves to that same tag, and a `v0.4.0` binary installed that way additionally reports its version as `0.0.0-dev`, because the fallback that reads the module version also landed after the tag. Both are fixed on `main` and land in the next release. Until then, build from a checkout.

### Adopting a repository

`mf init` performs the adoption. There is no list of paths to copy by hand.

```sh
cd /path/to/the/repository/you/want/to/govern
mf init
```

Nothing here picks a reviewer for you. If you already know which provider should
review your code, name it and `init` records the route on this machine and the
chain in committed policy in one step:

```sh
mf init --provider <name> --endpoint <url> --api-key-env <VAR> --model <id>
```

`<VAR>` is the *name* of the environment variable holding the key, never the key
itself — the loader refuses a credential in either file. `--kind` selects the wire
shape (`openai-compatible`, the default, or `anthropic` or `google`). Without
these flags `init` writes no machine state at all, and a provider named without
`--endpoint` and `--api-key-env` is refused rather than recorded: a backend with
half a route resolves, gets named in a chain, and reports itself unavailable on
every run for a reason nothing states.

It writes, and never overwrites anything already there:

| What | Where |
|---|---|
| The committed policy file | `.framework.toml` |
| 13 standards | `docs/standards/` (or wherever `paths.standards` points) |
| 4 agent source documents | `docs/agents/` |
| 2 versioned git hooks | `.githooks/` |
| The hooks wiring | `core.hooksPath = .githooks` |
| The adopted framework version | `.framework.lock` |
| The vendor instruction files | `CLAUDE.md`, `AGENTS.md`, generated from `docs/agents/instructions.md` |

It refuses rather than guesses in three cases: outside a git repository it stops instead of scaffolding into whatever directory you are standing in; it writes no standards into a path that lies inside a submodule, because the submodule already supplies them; and it leaves a `core.hooksPath` that another tool set exactly as it is, because git honours one hooks path and replacing that one would switch their gate off rather than add this one beside it.

Three things `mf init` does not do, the first two of which will otherwise stop your next command:

1. **Put `mf` where the hooks can find it.** Both hooks fail closed, so the very next `git commit` is refused until `mf` is on `PATH`, or a binary named `mf`/`mf.exe` sits at the repository root, or `MF_BIN` points at one. This is deliberate: the previous hook exited zero when it could not find its runner, which is how a repository could report an activated gate and have none.
2. **Set the base branch, if yours is not `main`.** `review.base` defaults to `main`, and three gates read the range between a branch and its base. On a `master` repository, `mf check` stops with `ref "main" does not resolve`. Fix it once: `mf config set review.base master --project`.
3. **Write `CONTEXT.md`.** The domain glossary at your repository root is what `docs/agents/domain.md` tells every agent to read before exploring your code, and no shipped file can guess your domain. Nothing gates it, which is exactly why it gets skipped. Copy this repository's as a shape and replace every term.

Then declare the Author and see where you stand:

```sh
mf author declare --provider anthropic --model claude-opus-5
mf doctor
mf check
```

`mf author declare` is the one step that has to be repeated per branch: it records who wrote the change, which is what R2 checks the reviewer against. Without it the cross-provider state can be no better than `unknown`, and `unknown` does not satisfy R2. `mf doctor` reports what resolves, what is wired, and what is missing; it repairs nothing, because a diagnostic that fixes as it reads makes the second run disagree with the first.

Every role chain ships empty. This framework will not name a reviewer you have not configured, so `mf doctor` will report four roles with no chain until you give them one — `.framework.toml` carries the recipe in its own header, and `docs/adr/0006` explains why naming a reviewer takes a step in the project file and a step on the machine.

Coming from an earlier version, `mf config migrate` takes over the deprecated `r2.*` git-config keys. It copies rather than moves and prints the commands to remove the originals, so the destructive half stays a human decision.

### Machine state and consent

`mf statusline apply` writes the status line contract (`docs/standards/status_line.md`) into this machine's agent configuration: the same five facts, in the same order, in Claude Code and Codex. Several commands write outside the repository — `mf config set --machine` and `mf config migrate` write the machine configuration file, `mf review` and `mf usage reset` write the usage store beside it, `mf statusline refresh` writes the quota cache — but `apply` is the only one that edits a file another tool owns and that a person configured for themselves. That is why it is a command of its own rather than a step of `mf init`: Codex's `[tui]` section has no per-project form, so applying the contract governs every project on the machine, and nobody should get that as a side effect of activating a repository. A divergent configuration is backed up to a timestamped copy and then replaced; a matching one is left alone; `mf statusline revert` puts back what the last apply replaced. The renderer is the `mf` binary itself, so nothing is installed and no runtime is required.

### Running

There is no long-running app: the framework runs as checks, as hooks, and in CI.

```sh
mf check                      # the seven deterministic gates
mf review --role r2 --dry-run # the chain that would run, and what it would send
mf explain                    # a CRUX explainer for the current change
mf doctor                     # what resolves, what is wired, what is missing
```

Once `core.hooksPath` is wired, `mf check commit` gates each commit message as it is written, and `git push` runs the full `mf check` followed by the R2 review. The checks are not advisory and stop the push. The review is: a reviewer that never ran is not a finding, so an expired quota or a missing tool never locks the repository — the run says so, and CRURA human review substitutes. `git push --no-verify` bypasses both, and `docs/standards/github.md`'s checklist requires a bypass to be recorded in the pull request.

`mf eval` measures a backend against a corpus of diffs with planted defects. It reaches real providers, so it is deliberately not wired into CI.

### Tests

```sh
go build ./... && go vet ./... && go test ./...
go run ./cmd/mf check
```

That is the whole suite. The shell suites this repository used to carry were deleted with the shell review path under `docs/specs/0027`: `mf check docs` is a strict superset of what `docs-consistency.sh` checked, and the rest tested an implementation that no longer exists. `.github/workflows/gate.yml` is the single definition of "does this tree pass" — CI and the release workflow both call it and nothing else, so a tag cannot publish what a commit would have been rejected for.

## Command Reference

`mf` is fourteen commands over one configuration. Every one of them exits `0` on success, `1` on a failure it is reporting, and `2` on a usage error. All of them need a git repository and resolve paths against its root, except the ones whose whole subject is this machine: `mf statusline`, `mf usage`, `mf config migrate`, and `mf config set --machine`. A command with no root to resolve against would act on whatever directory you happen to be standing in, so it refuses instead.

### Activation and diagnosis

| Command | What it does |
|---|---|
| `mf init [--provider <name> --endpoint <url> --api-key-env <VAR> [--model <id>] [--kind <shape>]]` | Adopt this repository: materialise the standards, the agent source and the hooks, wire `core.hooksPath`, record the version, generate the instruction files. Overwrites nothing. The provider flags additionally record the reviewer you choose in the machine layer and name it in the R2 chain; without them no machine state is written. |
| `mf doctor` | Report the build, activation state, every role's chain and each backend's reachability, the cross-provider state, credentials, usage, and the token-economy claims. Changes nothing. |
| `mf hooks install` | Point `core.hooksPath` at `.githooks`. Refuses a hooks path it does not own. |
| `mf hooks uninstall` | Remove only the wiring `mf` installed. Idempotent; leaves the directory. |
| `mf hooks status` | The hooks path in effect, whether this repository set it, and whether the directory is there. |
| `mf upgrade` | Compare this repository's standards against the ones this build carries. Applies nothing — your standards are your content. |

### Gates

| Command | What it does |
|---|---|
| `mf check` | All seven gates: `spec`, `commit`, `branch`, `docs`, `records`, `agents`, `design`. |
| `mf check <gate>...` | Only the named gates. |
| `mf check commit --message <file>` | Check one message file rather than the branch's commits. What the `commit-msg` hook runs, with git's `$1`. |

The gates, in order: `spec` requires a spec under `docs/specs/` for a branch touching non-exempt paths; `commit` checks every subject on the branch against the Type Table read out of `github.md`, and rejects co-author and AI-attribution lines; `branch` checks the branch name against the same table; `docs` checks that no standard is an orphan, carries retired wording, or names a file that is not there; `records` checks the numbering and deletion archive of `docs/specs/` and `docs/adr/`; `agents` checks the generated instruction files against their single source; `design` checks the declared surfaces against `design.md`, including the fingerprints that separate a derived identity from a copied one.

### Review

| Command | What it does |
|---|---|
| `mf review --role <r1\|r2\|r3>` | Walk the role's backend chain, take the first available backend, report which one reviewed. |
| `mf review ... --base <ref>` | Review against a base other than `review.base`. |
| `mf review ... --dry-run` | Print the chain and what would be sent. Calls nothing. |
| `mf review --role r3 --pr <n> --post` | Post the R3 result as a pull request comment. |
| `mf author declare --provider <name> [--model <id>]` | Record the Author for this branch. `--provider` is required: it is the claim R2 is checked against. |
| `mf eval [--role <role>] [--backend <name>]` | Measure a backend against `docs/eval/corpus/`. Reaches real providers. |
| `mf explain [--base <ref>] [--difficulty easy\|medium\|hard] [--dir <path>] [--dry-run]` | Generate the CRUX explainer outside version control. Advisory; every path exits `0` except a mistyped flag. |

An R1 backend of kind `in-session` runs inside a coding-agent session and cannot be started as a subprocess, so it contributes an attestation rather than an execution. The session that reviewed records one:

```sh
git config --local mf.attestation.r1 "$(git rev-parse HEAD)"
```

The commit is compared, not the branch: an attestation for an earlier tip has not seen what is being pushed now.

### Configuration

| Command | What it does |
|---|---|
| `mf config get <key>` | One resolved value and the layer it came from. |
| `mf config list [--provenance]` | Every resolved value, optionally with its layer and source. |
| `mf config set <key> <value> [--machine\|--project]` | Write one value. `--project` edits `.framework.toml`; `--machine` edits the per-user file. |
| `mf config validate` | Load the configuration and report every problem, including a route it names and cannot reach. |
| `mf config migrate` | Copy the deprecated `r2.*` git-config keys into the machine layer and print the commands to remove the originals. |
| `mf models pin` | Record the model id each backend currently resolves to, in `.framework.lock`. |
| `mf models list` | Compare the pins against the configuration and report drift. |
| `mf usage [show]` | Runs and tokens spent, in disjoint buckets, plus cost when a price is configured. |
| `mf usage reset` | Clear the usage total. |
| `mf agents sync` | Regenerate every `[agents.*]` file from `docs/agents/instructions.md`. |
| `mf agents check` | Report which generated file has drifted from that source. Also runs as `mf check agents`. |

Five layers resolve, lowest precedence first: built-in default, deprecated git config, machine file, project file, environment. Any key is overridable for one run by uppercasing it and replacing `.` with `_` behind an `MF_` prefix — `MF_REVIEW_BASE`, `MF_ROLES_R2_BACKENDS`, `MF_ROLES_R2_BLOCKING`.

| Key | Default | Meaning |
|---|---|---|
| `paths.standards` | `docs/standards` | Where every gate reads the standards. |
| `paths.specs` | `docs/specs` | The durable spec archive the Spec Gate and the records gate read. |
| `paths.adr` | `docs/adr` | The durable decision archive the records gate reads. |
| `paths.agents_file` | `AGENTS.md` | The instruction file a repository-reading agentic backend finds on disk. |
| `review.base` | `main` | The ref a change is compared against. |
| `review.effort` | `high` | Reasoning effort passed to a backend that takes one. |
| `review.max_diff_bytes` | `30000` | The diff budget a backend is handed. |
| `review.timeout_seconds` | `240` | The wall-clock budget; exceeding it counts as unavailability. |
| `roles.<role>.backends` | `codex` for `r2`, empty otherwise | The ordered chain. An empty list declared in the project file overrides a lower layer. |
| `roles.<role>.blocking` | `false` | Whether a finding classed as blocking stops the pre-push hook. Replaces the dead `R2_BLOCKING`. |
| `roles.<role>.require_cross_provider` | `true` for `r2`, `false` otherwise | Whether the Reviewer's provider must differ from the Author's. |
| `backends.<name>.*` | — | `kind`, `provider`, `command`, `args`, `model`, `effort`, `unavailable_patterns`. |
| `providers.<name>.*` | — | `kind`, `endpoint`, `api_key_env`. Machine layer only. |

The project layer may not carry `endpoint`, `api_key_env` or `api_key`: those are machine state, and the loader refuses them by name rather than reporting a typo. A backend's `command` it does accept, but only as a bare program name — no path, no shell metacharacters — because a committed command runs verbatim on every contributor who clones and pushes. That narrows the trust boundary rather than closing it: the honest claim for a committed backend is reviewable policy, not a sandbox. The machine layer may define backends, which is how a role chain a project names is completed by a machine or a CI secret; a name the project file defines wins whole, never field by field, so a machine adds reviewers and never substitutes one the repository already chose.

### Status line

| Command | What it does |
|---|---|
| `mf statusline render [--no-refresh]` | Render the five facts. Never fails, never blocks on the network; a fact it cannot read degrades to a placeholder. |
| `mf statusline apply` | Write the contract into Claude Code's and Codex's own configuration, backing up what it replaces. Machine state, not repository state. |
| `mf statusline revert` | Restore what the last `apply` replaced. |
| `mf statusline refresh [--version <v>]` | Refresh the quota cache. Normally spawned by `render`, not typed. |

### Environment

| Variable | Read by | Meaning |
|---|---|---|
| `MF_BIN` | both hooks | Path to the `mf` binary, for a checkout that is neither on `PATH` nor built in place. Pointing it at a non-executable is an error, not a fallback. |
| `MF_CONFIG_HOME` | the whole binary | Where the machine layer and the usage store live. CI points it at the runner's temp directory. |
| `MF_<KEY>` | the config loader | One-run override of any resolved key. |
| `SKIP_R2_REVIEW=1` | `pre-push` | Skip the review, never the checks. |
| `NO_COLOR` | `statusline render` | Render without colour. |

## Project Structure

```
my-framework/
├── cmd/mf/               # the binary's entry point
├── internal/             # the harness: config, roles, backends, checks, report
├── docs/
│   ├── standards/        # binding development standards, read via INDEX.md
│   ├── adr/              # durable architecture decision records
│   ├── specs/            # durable archive of approved specs
│   ├── agents/           # the single source the vendor instruction files are generated from
│   └── eval/corpus/      # diffs with planted defects, for measuring a backend
├── .framework.toml       # committed policy: paths, roles, backends, checks
├── .githooks/            # versioned commit-msg and pre-push hooks, both failing closed
├── scripts/statusline/   # the retired Node renderer, kept only for an unmigrated consumer
└── .github/              # PR/Issue templates, and the CI, gate, release and review workflows
```

## Project Status

In development. `v0.4.0` is the current release and the first tag to contain `cmd/mf/`: it publishes stamped binaries for `linux/{amd64,arm64}`, `darwin/{amd64,arm64}` and `windows/amd64` with a `SHA256SUMS` covering all five. Tags `v0.1.0` through `v0.3.0` are the earlier standards-only releases, from before the binary existed; an adopter recording the tag they adopted from wants `v0.4.0` or later, and `mf init` records it in `.framework.lock` automatically.

Unreleased since `v0.4.0`, and landing in the next tag: the deletion of the shell review path, backends in the machine layer, `[paths]`, `roles.<role>.blocking`, the fail-closed hooks, `mf check commit --message`, `mf statusline revert`, and the version fallback that makes a `go install` build report a real version. Versioning policy is semver git tags.

Done, in the tree: the seven gates and the three places they run, the four role chains, the configuration cascade, the status line contract, the CRUX explainer, the eval corpus, the design gate, and an `mf init` that genuinely adopts a repository.

Pending: the fingerprint table that would let R2 report `verified`; an R3 reviewer configured on this repository; a first R1 backend that is not `in-session`; the removal of the Node status line renderer once the submodule consumer has migrated.

## Known Issues & Limitations

- **R1 runs only from inside a session.** Its whole shipped chain is `superpowers`, an `in-session` backend that cannot be started as a subprocess. It is satisfiable — the session that reviewed records `git config --local mf.attestation.r1 $(git rev-parse HEAD)`, and the runner then reports R1 as reviewed — but nothing outside a session can produce that record, so a CI run or a push from a plain terminal reports R1 as not run. A second, subprocess-startable R1 backend is what would close this; none is configured here.
- **R3 does not run on this repository.** The mechanism exists: `.github/workflows/review.yml` builds a machine-layer `api` backend out of `MF_R3_ENDPOINT`, `MF_R3_MODEL`, `MF_R3_PROVIDER_KIND` and the `MF_R3_API_KEY` secret, and prepends it to the chain for that run. None of those is set here, so every pull request gets a job that says R3 did not run and what to set. A fork's pull request cannot run it at all, by GitHub's design: forks get no secrets and a read-only token.
- **The newest tag cannot perform the adoption this README describes.** `v0.4.0`'s `mf init` writes the policy file and the lock and stops; the materialisation of the standards, the agent source and the hooks is on `main` and unreleased. Adopting today means building from a checkout. This closes when the next tag ships.
- **`mf init` adopts the harness, not the forge.** It writes the standards, the agent source, the hooks, the policy file and the instruction files. It does not write `.github/` templates or workflows, `CONTEXT.md`, or the `.gitattributes` that keeps a Windows checkout from failing the formatting gate. Those are still copied by hand from this repository.
- **The R2 cross-provider gate needs at least one configured backend to be available.** When none is, R2 does not run for that push, the runner says so, and CRURA human review substitutes per `docs/standards/crura_method.md`.
- **The cross-provider requirement compares two labels.** Both provider names are configuration strings nothing verifies against the endpoint actually reached, so a chain that selects a local OpenAI-compatible endpoint satisfies the rule by naming a provider. This is the standards question issue #16 raised, confirmed still live by the audit and unresolved: the claim is exactly as strong as the labels, which is why the Cross-Provider State is recorded beside the review rather than reduced to a pass.
- **The `codex` and `gemini` backends classify quota, authentication and network failures by matching their tool's error text**, which will drift when a vendor rewords it. A drifted pattern reads an unavailable backend as one that reviewed, so the chain stops early and names it rather than falling through silently.
- **The `gemini` backend is exercised against a stub, not the real CLI**, because that CLI is not installed in this repository's development or CI environment. Its dispatch and contract are pinned; its real invocation is unverified, which is why the prompt argument is configurable.
- **A reasoning model behind an `api` backend has volatile latency**, so the wall-clock budget (`review.timeout_seconds`, default 240s) is reachable rather than guaranteed. Measured against `deepseek-v4-flash`: 8 KB of diff took 27s, 30 KB took 85s, 40 KB took 163s once and exceeded 240s the next time, and 112 KB never returned. Exceeding the budget counts as unavailability, so the chain advances and the push is never held hostage to a slow reviewer.
- **A local model is a much weaker reviewer than a hosted frontier one**, and on CPU-only hardware also a slow one. The reported backend line is what keeps a fallback from passing as an equivalent review.
- **The deprecated `r2.openai.model` git-config key has no destination in the new key space and is not migrated.** A per-backend model now lives on the backend it belongs to, so the value is left where it is rather than given an invented home.
- **The fingerprint table that would let R2 report `verified` rather than `declared` ships empty.** Guessing which environment variables a vendor's agent sets would be inventing environment variables, which `docs/standards/ai_guidelines.md` forbids, so an adopter fills it in or accepts `declared` as the best case.
- **The standards assume a single repository.** A multi-repo setup with conflicting standards would need an authority hierarchy this framework does not yet define.
- **Two commits reachable from `main` carry AI-attribution trailers**, in violation of this repository's own rule against them. They are not rewritten, because both are already published and force-pushing over shared history to correct a commit message costs more than the defect it would fix. Recurrence is guarded twice now: the `commit-msg` hook rejects such a line as it is typed, and `mf check commit` rejects one anywhere in the range a branch adds over its base — which is how work reaches this repository. A commit pushed straight to `main` is still outside both.
- **The status line contract is machine state, not repository state**, because the Codex TUI section has no per-project scope. Applying it with `mf statusline apply` therefore governs every project on the machine, and a per-repository status line is not available for the Codex side at all.
- **Two status line implementations coexist**: `mf statusline render` and the Node renderer under `scripts/statusline/`. The shell suite that held the Node one to the contract went with the rest of the shell path, so it is now untested and can drift from the Go renderer with nothing noticing. It is retained only until the submodule consumer has migrated, and deleting it is what closes this.
- **The Codex segment names written into its config file were read out of the installed Codex build rather than a published schema.** An upgrade that renames or drops one would leave the written configuration silently ignored — the line degrades, the tool does not break. The vocabulary as read is recorded in `docs/specs/0012-standardize-agent-status-line.md`.
- **The quota fact on the Claude Code side reads an undocumented OAuth usage endpoint** and needs an OAuth session; an API-key session shows `usage n/a`.
- **`mf explain` sends a diff to a model and gets prose back, and nothing checks that prose against the code.** An explainer that confidently describes behaviour the change does not have is worse than no explainer, which is why it stays advisory and is read against the diff rather than instead of it.
- **The design gate reads colour literals with a regular expression, not a CSS parser.** It finds hex and `rgb()` values, which are the forms this repository writes; a named CSS colour, an `oklch()` call or a computed value would pass unnoticed, so `docs/standards/design.md` forbids those forms rather than the gate pretending to catch them.
- **The design standard's fingerprint check proves that the source entry's literal colours and typefaces are not reused.** It cannot prove independence of design: direction is not a value, and a layout or a restraint cannot be fingerprinted. The source entry also carries no version, so a later read may differ from the one the fingerprints describe.
- **The Token Economy's terse boundary is enforced only where the harness composes the prompt.** A person writing a commit message or a PR body by hand is outside it, so that part of the rule remains a discipline rather than a check.
- **`mf eval` grades by matching findings against planted defects**, never by a model judging a model. The numbers are self-reported, measure this corpus and these prompts, and are not comparable to an independent evaluation.
- **Executable bits are trusted from the git index, not the filesystem**, because the Windows filesystem does not reliably report them; `git ls-files -s` is the source of truth instead.
- Small deferred follow-ups are tracked in the issue backlog rather than in this README.

## Contributing

Fork the repository, branch as `type/TASK-NNN-description`, write tests before implementation, and use Conventional Commits. Open a Pull Request following the PR Model in `docs/standards/github.md`.

## License

MIT, see [`LICENSE`](LICENSE).
