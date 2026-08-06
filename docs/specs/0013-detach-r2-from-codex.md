# SPEC: feat(scripts): detach the R2 gate from Codex behind a reviewer backend seam

## Problem

The standards say the R2 reviewer is a role — "the requirement is the role, not a
name" — while the implementation invokes `codex review` and nothing else, so the
gate silently stops existing whenever that one vendor's account is unavailable.

## Design Decision

Put a seam where the standards already claim one is. The gate becomes a chain of
named backends, each a small adapter script under `scripts/reviewers/` that knows
how to invoke one reviewer and, crucially, how to classify its own tool's
failures. The runner walks the chain in order until a backend actually reviews,
and then says which one did.

The adapter contract is three exit codes, because the distinction the chain turns
on cannot be recovered from the outside. `0` means the backend reviewed and had
nothing blocking to say. `10` means the backend was unavailable — not installed,
not authenticated, out of quota, endpoint unreachable — and the chain advances.
Any other code means the backend reviewed and the run has findings or failed
mid-review, and the chain stops. Putting the classification inside each adapter
keeps the string-matching against a vendor's error text local to the adapter that
owns that vendor, instead of smearing a fragile heuristic across the runner.

Three backends ship. `codex` preserves today's behavior exactly and stays the
sole default, so an existing clone that upgrades sees no change. `gemini` is a
second agentic CLI that explores the repository the way Codex does, from a
different provider, on a free tier. `openai` is the one that pays for
the seam: a single adapter that speaks the OpenAI chat-completions shape and
therefore reaches Ollama, LM Studio, llama.cpp, vLLM, DeepSeek, Groq, OpenRouter
and Together without another line of framework code. It is also the only path to
a fully local reviewer.

Configuration reuses git's own scope cascade rather than inventing a file format:
environment, then `git config --local`, then `git config --global`, then the
built-in default. That gives the machine-global configuration the Developer asked
for at the `--global` layer, keeps a repository able to override it at `--local`,
and matches the authority rule `code_conventions.md` already states. A new
`setup.sh --reviewer` writes the global layer; `--interactive` keeps its current
per-repository behavior untouched, so each flag owns one scope.

A weaker reviewer is not the same review. The chain therefore reports the backend
and resolved model that actually ran, in a line meant to be copied into the PR's
review-layers record. Falling back is allowed; falling back quietly is not, and a
run where every backend was unavailable says so and still does not block the
push, because a missing reviewer is not a finding.

Secrets stay out of git config. The `openai` backend is configured
with the *name* of the environment variable holding its key, never the key, since
`git config --list` output is pasted into bug reports and screenshots.

The request is assembled with a stable prefix — the role instructions and
`AGENTS.md` in the system message, the volatile diff last — because providers on
this shape bill cached prompt tokens at a fraction of fresh ones, and a pre-push
gate re-sends that prefix on every push. Ordering the message for the cache is
free, and skipping it would quietly triple the cost of the cheapest backend.

## Alternatives Considered

- **A free-form `command` backend taking the diff on stdin**: rejected at the
  Developer's decision. It is the cheapest thing to build and the most flexible,
  but the framework then cannot classify unavailable-versus-reviewed, which is
  exactly the distinction the chain runs on, so every such backend would be a
  chain terminator that reports nothing useful.
- **Keep one backend and switch it by hand**: rejected — it is today's failure
  with an extra config key. The push that motivated this spec lost R2 entirely to
  a quota reset three days out, and a manual switch only helps a Developer who
  notices in the moment.
- **Gate the fallback on reviewer tier, never degrading to a weak model**:
  rejected — it requires the framework to publish a capability ranking of third
  party models, which is judgment that ages badly and would need editing every
  time a vendor ships. Reporting which backend ran puts the same judgment where
  it belongs, with the human adjudicating the PR.
- **A dedicated config file under `~/.my-framework/`**: rejected — it duplicates
  a scope cascade git already implements correctly, adds a format to parse in
  shell, and creates a second place to look when a value resolves surprisingly.
- **Keep the `codex_review.md` and `codex-review.sh` names**: rejected — a
  document called Codex Review that specifies a Gemini backend and a local
  llama.cpp endpoint is actively misleading, and the misleading name is load
  bearing here because `AGENTS.md` and the PR checklist point readers at it. The
  rename costs edits to `INDEX.md`, the docs-consistency guard that pins the
  path, `crura_method.md`, `skills_guidelines.md`, the pre-push hook, the CI
  workflow and the README; all mechanical, and the guards fail on a missed one.
- **Require `jq` for the `openai` adapter's JSON**: rejected — Node is
  already the framework's declared soft dependency for the status line, and
  adding a second one to do the same class of work costs an adopter another
  install for no benefit. Node also removes the need for `curl`, since it can
  make the request itself.
- **Run the backends in parallel and merge findings**: rejected — it multiplies
  cost and quota burn on every push to produce a merged report nobody asked for,
  when the chain's purpose is to get one review from whichever reviewer is
  actually available.

## Scope

- Includes:
  - `scripts/reviewers/codex.sh`, `scripts/reviewers/gemini.sh`,
    `scripts/reviewers/openai.sh`: the three adapters, each honoring
    the three-code contract and each classifying its own tool's unavailability.
  - `scripts/r2-review.sh` (renamed from `scripts/codex-review.sh`): resolves the
    chain and per-backend settings through the cascade, walks the chain, and
    reports the backend that reviewed.
  - `.githooks/pre-push`: calls the renamed runner.
  - `scripts/setup.sh`: a `--reviewer` flag writing the machine-global reviewer
    configuration into `git config --global`.
  - `docs/standards/r2_gate.md` (renamed from `codex_review.md`): the
    provider-agnostic gate, the adapter contract, one subsection per shipped
    backend, the configuration cascade and key reference, and the PR recording
    rule naming the backend that ran.
  - `scripts/test/r2-review.test.sh` (renamed from `codex-review.test.sh`): the
    existing guard logic plus one case per new Acceptance Criterion, with stub
    adapters and a stub HTTP endpoint.
  - `docs/standards/INDEX.md`, `docs/standards/ai_guidelines.md`,
    `docs/standards/crura_method.md`, `docs/standards/skills_guidelines.md`,
    `AGENTS.md`, `CONTEXT.md`, `README.md`, `.github/workflows/ci.yml`,
    `.github/PULL_REQUEST_TEMPLATE.md`, `docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md`,
    and the docs-consistency guard that pins the old path: updated for the
    rename and the seam. The ADR's decision is untouched; only the paths it
    points at are corrected, so a reader following them still lands on files
    that exist.
- Does NOT include:
  - Changing the default. `r2.backends` defaults to `codex` alone, so a clone
    that upgrades and configures nothing behaves exactly as it does today.
  - The free-form `command` backend, and any backend beyond the three named.
  - Any tier or capability ranking of reviewer models.
  - Running backends in parallel, or merging findings from more than one.
  - Storing an API key, token, or any secret in git config or any versioned file.
  - Installing, authenticating, or pulling models for any backend. Absence is
    reported and the chain advances, as `codex_review.md` already does for Codex.
  - Making the gate blocking by default; `R2_BLOCKING` keeps its opt-in meaning.
  - Any change to R1, R3, the Spec Gate, the Type Table, or the spec and ADR
    numbering rules.

## Acceptance Criteria

- chain_advances_past_unavailable_backend: with a first stub adapter exiting 10
  and a second exiting 0, the runner invokes both and exits 0.
- chain_stops_at_first_backend_that_reviews: with a first stub adapter exiting 0,
  the second is never invoked.
- chain_reports_which_backend_reviewed: the runner's output names the backend and
  the resolved model that actually reviewed, and names each backend it skipped
  with the reason.
- chain_reports_when_no_backend_ran: with every adapter exiting 10, the output
  states that R2 did not run, and the runner exits 0 so the push proceeds.
- unknown_backend_is_reported_not_ignored: a name in the chain with no adapter is
  reported and skipped, and the chain continues to the next name.
- blocking_mode_blocks_on_findings: with `R2_BLOCKING=1` and an adapter exiting a
  code that is neither 0 nor 10, the runner exits non-zero.
- blocking_mode_does_not_block_on_unavailable: with `R2_BLOCKING=1` and every
  adapter exiting 10, the runner still exits 0 — a missing reviewer is not a
  finding.
- settings_resolve_by_scope_cascade: for each of backends, model and effort, an
  environment value beats `--local`, which beats `--global`, which beats the
  built-in default.
- legacy_codex_settings_still_resolve: `CODEX_REVIEW_MODEL`,
  `CODEX_REVIEW_EFFORT` and `git config codexreview.model` still drive the codex
  backend, and `SKIP_CODEX_REVIEW=1` still skips the gate.
- default_chain_is_codex_only: with no configuration at any scope, the resolved
  chain is exactly `codex`.
- reviewer_flag_writes_global_scope: `setup.sh --reviewer` persists the answers
  into `git config --global`, leaving `--local` untouched.
- reviewer_flag_refuses_a_secret_value: an answer for the API key setting that
  looks like a key rather than an environment variable name is refused, and
  nothing is persisted for that key.
- openai_adapter_sends_the_contract_payload: against a stub endpoint, the request
  carries the resolved model, a system message containing `AGENTS.md`, and a user
  message containing the branch diff.
- openai_adapter_reports_truncation: a diff larger than the configured limit is
  truncated and the adapter's output says that it was.
- openai_adapter_reads_content_not_reasoning: a response carrying both
  `content` and `reasoning_content` yields the `content` as the review, and the
  reasoning field is not reported as findings.
- openai_adapter_reports_a_cut_off_review: a response with
  `finish_reason: "length"` is reported as a review that was cut off, distinctly
  from a review that completed with no findings.
- openai_adapter_is_unavailable_on_unreachable_endpoint: an endpoint refusing the
  connection makes the adapter exit 10 rather than fail the run.
- openai_adapter_is_unavailable_without_node: with `node` absent, the adapter
  exits 10 with a message naming Node.
- dryrun_prints_the_resolved_chain: `R2_DRYRUN=1` prints the chain and each
  backend's resolved command without invoking any of them.
- all_suites_green: all five suites and the docs-consistency check pass on the
  final tree.

## Reproducibility

Run, from the repository root, with git >= 2.40, bash (Git for Windows), and
Node >= 18:

```sh
bash scripts/test/r2-review.test.sh
bash scripts/test/setup.test.sh
bash scripts/test/statusline.test.sh
bash scripts/test/docs-consistency.test.sh
bash scripts/test/docs-consistency.sh
```

All pass, 0 failed. The chain cases use stub adapters on a sandboxed
`R2_REVIEWERS_DIR`, and the `openai` cases use a stub HTTP server
started by the suite, so no test reaches a real provider, spends quota, or needs
a key. `GIT_CONFIG_GLOBAL` and `GIT_CONFIG_SYSTEM` are pinned per test so the
scope-cascade cases cannot read the operator's real configuration. No randomness
is involved.

Environment surveyed on the development machine, 2026-08-05: Codex CLI v0.146.1
present but out of account quota until 2026-08-08; Gemini CLI not installed;
Ollama 0.17.7 serving on `http://localhost:11434` with `llama3.1:8b` and
`llama3.2:3b` pulled; hardware is an i5-1135G7 with 19.8 GB RAM and Intel Iris Xe
integrated graphics, so local inference is CPU-only.

The first configured `openai` target is DeepSeek, chosen by the
Developer. Its catalogue was read from the provider rather than assumed —
`GET https://api.deepseek.com/models` returns exactly `deepseek-v4-flash` and
`deepseek-v4-pro` — and `deepseek-v4-flash` was exercised against
`POST https://api.deepseek.com/chat/completions` to confirm the shape this
adapter depends on: OpenAI-compatible request and response, a `content` field
carrying the review alongside a `reasoning_content` field that must be ignored,
`finish_reason` distinguishing a completed review from a cut-off one, and
`prompt_cache_hit_tokens` confirming the cached-prefix billing the message
ordering is built for. The key is read from `DEEPSEEK_API_KEY`, whose name — not
value — is what `r2.openai.apiKeyEnv` stores.

## Risks and Assumptions

- Assumption: the Spec Gate approval for this scope is the Developer's decision
  of 2026-08-05 selecting the three backends, the ordered chain with loud
  recording, and rejecting the free-form `command` backend.
- Assumption: `AGENTS.md` is the instruction file injected into non-agentic
  backends, since it already carries the Reviewer role and the binding standards
  for exactly this purpose.
- Risk: the `codex` adapter classifies quota and auth failures by matching that
  tool's error text, which will drift when the vendor rewords it. A drifted
  pattern degrades gracefully — an unavailable backend is misread as a backend
  that reviewed, so the chain stops early and the output names it — but the
  matching is the fragile part of this design and is deliberately confined to
  the one adapter that owns that vendor.
- Risk: the `gemini` adapter cannot be verified against the real CLI on this
  machine, because it is not installed. It is exercised through a stub, the way
  `gh` already is in `setup.test.sh`, so its dispatch and contract are pinned but
  its real invocation is unverified until someone installs it. The invocation is
  therefore configurable rather than hard-coded, so a flag change does not
  require a framework release.
- Risk: an `openai` backend pointed at a small local model is a much
  weaker reviewer than the current default — more false positives, more missed
  defects — and on this hardware also a slow one, which invites `--no-verify` and
  would end with R2 not running at all. Mitigated by the default chain staying
  `codex` alone and by the reported backend line, not by preventing the choice.
- Risk: the diff sent to a non-agentic backend is truncated at a byte limit, so a
  large change is reviewed in part. The adapter says so; a silent partial review
  would be the worse failure.
- Risk: the shipped default chain stays `codex` alone, but this machine is
  configured to `codex,openai` with the `openai` backend on `deepseek-v4-flash`.
  While Codex is out of quota every push will reach DeepSeek, which is billed
  usage rather than a free tier. It is cheap per review, not free, and a push-heavy
  day is the case where that distinction shows up.
- Risk: a provider's model catalogue is a moving target — the id configured today
  is one the provider currently serves, and a retired id fails the request rather
  than degrading. The adapter treats an HTTP error as unavailable, so the chain
  advances and the push is not blocked, but the configured backend then silently
  stops contributing until someone reads the reported reason.
- Risk: naming an environment variable keeps the key out of git config but not
  out of the process environment, which every child process of the push inherits.
  This is the same exposure any CLI credential has and is not solved here.
- What would invalidate this spec: Codex gaining a stable machine-readable exit
  code for quota and auth failures, which would make the adapter-local text
  matching unnecessary; or the R2 requirement changing from "different provider"
  to something a single vendor can satisfy, which would remove the reason for a
  chain.
