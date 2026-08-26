# R2 Gate

Operational definition of the R2 cross-provider review from `ai_guidelines.md`. It runs
a chain of reviewer backends as a `pre-push` gate, taking the first one that is actually
available, and reports which one reviewed. R1 is its own chain of backends and is
unchanged here;
this document makes R2 concrete and records the repository's R3 wiring. On this
repository, R3 is CodeRabbit (wired via its GitHub app);
its findings are adjudicated in the PR discussion like any reviewer finding.

Every operational statement below is about one program, `mf`. There is no second
implementation of this gate and no shell entry point: a repository that carried both
had two answers to every question here, and the drift between them is what this
document was rewritten to end.

## Roles

- Author: the provider and model that wrote the change. This is a property of the change
  and is declared while it is being authored, not inferred at push time — a push carries
  commits that may come from several sessions, several agents, or a person typing
  directly, so there is no single Author to detect at that moment. The declaration is
  recorded per branch and named per PR.
- Reviewer: whichever backend in the chain actually ran. The requirement is the role,
  not a name: the Reviewer must be a different provider than the Author. The built-in
  default chain is `codex` alone, but `mf init` scaffolds every chain empty, so an
  adopted repository names its reviewer or has none — a role with no chain reports that
  it did not run, which is the honest state for a reviewer nobody configured.

The Reviewer reads the repository's agent instruction file — `AGENTS.md` by default,
`paths.agents_file` where a repository keeps it elsewhere — for its role and the binding
standards. An agentic backend finds it on disk itself; a prompt-driven one has it sent in
the request.

### The cross-provider state

Whether the Reviewer's provider differs from the Author's resolves to one of three
values, because collapsing them into a yes/no would report "nobody recorded it" as
"satisfied":

| State | Meaning | Satisfies R2 |
|---|---|---|
| `verified` | An independent signal agrees with the declaration and differs from the Reviewer's provider | Yes |
| `declared` | Only the branch record asserts the Author's provider | Yes, as an assertion |
| `unknown` | Nothing recorded the Author's provider | No |

`unknown` **does not satisfy R2**. The run says so in the line meant for the PR, on the
same principle the chain already holds: falling back is allowed, falling back quietly is
not. A signal that contradicts the declaration is an error to resolve, never a preference
to apply silently — the run stops and names both claims rather than picking one.

A backend that runs inside a coding-agent session — `superpowers` is the one named today
— cannot be started as a subprocess. It therefore contributes
an attestation rather than an execution, and a session where it is absent counts as
unavailable so the chain advances.

`mf review --role r2` computes the state and refuses to report R2 satisfied when it is
`unknown`. Write the declaration while authoring:

```sh
mf author declare --provider <name> --model <id>
```

It records the provider and model in the repository-local git config, keyed by the
branch (`branch.<branch>.mfAuthorProvider` and `.mfAuthorModel`), which is why it is not
committed: it says what happened on this clone, and a value that travelled with the
branch would assert something about sessions it never saw.

Corroboration is configured per machine — an environment variable name mapped to the
provider whose agent sets it, under `[fingerprints]` in the machine file — and ships
empty, because guessing a vendor's variable names would be inventing environment
variables. The practical consequence is that `declared` is the realistic best case
today, and `verified` requires an adopter to fill that table in. `mf doctor` says which
of the two this machine can reach.

Only the Author's side of the claim has anything behind it. The Reviewer's provider is a
label in configuration, and nothing here checks it against the endpoint that backend
actually reached — a backend declared `provider = "openai"` whose machine layer points
that provider at another vendor still reports `openai`. The run says so in the same line,
because an unverified claim presented as an enforced one is worse than no claim.

## Why a chain

A single hard-wired reviewer means R2 silently stops existing whenever that one vendor's
account is unavailable. A quota that resets days out, an expired key, an offline laptop:
each turns the gate into a no-op that still reports success. The chain exists so an
unavailable reviewer is a fallback rather than a hole.

Falling back is allowed. Falling back quietly is not — a weaker reviewer is not the same
review, so the runner names the backend and model that ran, and that name belongs in the
PR's review-layers record.

## What counts as a review

Each backend is a `kind` declared in configuration, not a script, and the runner starts
it directly. Three outcomes, and only one of them stops the chain:

| Outcome | Chain |
|---|---|
| **Reviewed** — the backend answered | stops; the answer is reported, findings or none |
| **Unavailable** — everything else a running backend can do | advances to the next backend, naming the skip and its reason |
| **Misconfigured** — a chain naming a backend nothing defines | the run fails; nothing is reported as reviewed |

For a `cli` backend, "answered" means it exited zero **and** printed something. Every
other ending is unavailability: not on `PATH`, killed by the review budget, a non-zero
exit whether or not its text matched a vendor pattern, and — the case worth naming — a
clean exit that printed nothing at all. A killed or mis-argumented agentic reviewer looks
exactly like a reviewer that found nothing, and recording that as a review would publish
"Reviewed by" on a pull request for output no reviewer produced. That false negative is
the worst outcome this framework recognises, so silence is never read as approval.

For an `api` backend the same line falls between an HTTP 200 and everything else. No
endpoint, no model, an unreachable host, the budget elapsed, or any non-200 status is
unavailability — a retired model id and an expired key must advance the chain rather than
pass as a clean review. A 200 whose body does not parse into the findings schema is still
an answer: the prose is recorded verbatim as one unstructured finding, because a
malformed answer that reports nothing is indistinguishable from a clean one.

What `unavailable_patterns` buys is therefore not the chain decision — a failing CLI
advances the chain either way — but an honest reason. Matching a vendor's error text is
confined to the backend that owns that vendor, and a match turns "codex failed (exit 1):
…" into "quota, authentication, or network", which is the difference between a skip line
a reader can act on and one they cannot.

Misconfiguration is deliberately not unavailability. A chain that names a backend no
layer defines is a typo, and reporting a typo as a vendor outage lets a gate pass by
never running.

## Backend kinds

| Kind | How it runs | Notes |
|---|---|---|
| `cli` | A program on `PATH`, started with the declared `args` | A committed file may name only a bare program name here, never a path |
| `api` | An HTTP request to the endpoint its provider declares | The provider, its endpoint and its credential are machine state |
| `in-session` | Never started; satisfied by an attestation | Its session is already running, so there is nothing to spawn |
| `external` | Never started; runs as a forge app | Always reports unavailable here, so a declaration cannot read as an observed review |
| `inproc` | Deterministic checks inside the binary | Unavailable with none registered, rather than silently clean |

`{{.Base}}`, `{{.Head}}`, `{{.Model}}`, `{{.Effort}}` and `{{.Prompt}}` are the
substitutions a `cli` backend's `args` may carry. A backend that takes `{{.Prompt}}` is
prompt-driven: the system prompt for its role, the instruction file and the diff are
handed to it, and it sees only what it is sent. One that does not is agentic: it explores
the repository itself. That is a difference in the shape of the review, not merely in its
grade, and it is why one declarative form can serve both without a hand-written adapter
for either.

The system prompt is chosen by role, not by kind. The CRUX explainer's shipped chain is a
prompt-driven `cli` backend too, and handing it instructions to answer with findings
would produce an explainer shaped like a verdict.

`review.timeout_seconds` is one budget for every kind that can hang, `cli` and `api`
alike, because it is a property of the review rather than of the wire shape behind it.
Exceeding it is unavailability, so the chain advances rather than holding the push.

## Shipped backends

### `codex`

Codex CLI (OpenAI), agentic: it explores the repository itself. Runs

```sh
codex review --base <base> -c model="<model>" -c model_reasoning_effort="<effort>"
```

Default model `gpt-5.6-terra` with effort `high`, pinned per backend so a fallback never
inherits it. Requires `codex` on `PATH` and an authenticated session (`codex login`);
verify the toolchain with `codex doctor`, and what this framework resolves with
`mf doctor`.

### `agy`

Antigravity's CLI, prompt-driven like `gemini`: the Reviewer role, the instruction file
and the diff are handed to it through `{{.Prompt}}`. It declares `structured = true`, so
it is asked for the findings schema and its severities survive into the record rather
than being kept as prose.

It is a gateway rather than a vendor — the same command reaches several — so which
vendor actually reviews depends entirely on the model pinned on the backend. That is why
a chain that names it must pin a model from a provider the Author is not: the
cross-provider rule compares labels, and a same-vendor model behind a differently
labelled gateway satisfies it by name while defeating it in substance. Effort is baked
into the model id (`-high`, `-low`), so `--effort` is not passed: two settings for one
dial is a way to have them disagree.

### `gemini`

Gemini CLI (Google), also agentic, but prompt-driven rather than repository-reading: the
Reviewer role, the instruction file and the diff are handed to it through `{{.Prompt}}`.
Its argument list is ordinary configuration, so an upstream flag change is fixable by
editing `args` rather than by a framework release.

### An OpenAI-compatible endpoint

Any endpoint speaking the OpenAI chat-completions shape is reachable as an `api` backend
whose provider declares `kind = "openai-compatible"`. One shape therefore reaches Ollama,
LM Studio, llama.cpp, vLLM, DeepSeek, Groq, OpenRouter and Together — local and hosted
alike — and it is the only path to a fully local reviewer. The `anthropic` and `google`
shapes are the other two this build speaks.

No such backend ships in this repository's chain, because the endpoint and the credential
are machine state and a committed file may hold neither. Define one where it belongs:

```sh
mf config set providers.local.kind openai-compatible --machine
mf config set providers.local.endpoint http://localhost:11434/v1 --machine
mf config set backends.local.kind api --machine
mf config set backends.local.provider local --machine
mf config set backends.local.model qwen3-coder --machine
```

Unlike an agentic backend it sees only what it is sent: the branch diff, plus the
instruction file for the role and the standards. The request puts the stable prefix first
and the volatile diff last, because providers on this shape bill cached prompt tokens at
a fraction of fresh ones and a pre-push gate re-sends that prefix on every push.

The budget is total elapsed time, not socket inactivity: a reasoning model sends nothing
while it thinks, so an inactivity timeout never fires and the request runs until something
else drops it. A diff larger than `review.max_diff_bytes` is truncated and the truncation
is reported; a response cut off by the output limit is reported as incomplete. Both make
the review partial, and a partial review that reads as a whole one is what those two
notes exist to prevent. The model's `reasoning_content`, when present, is never reported
as findings.

### `superpowers`

The `in-session` backend of the R1 chain. It cannot be started as a subprocess, so it is
satisfied by an attestation the session records for the exact commit under review:

```sh
git config --local mf.attestation.r1 $(git rev-parse HEAD)
```

The commit is compared rather than the branch. An attestation for an earlier tip has not
seen what is being pushed now, and a per-branch record would quietly cover every commit
added after it.

### `coderabbit`

Declared `external`: it runs as a GitHub app rather than as anything this tool starts.
The chain treats it as unavailable so that a configured reviewer never reads as an
observed review — which is a weaker claim than any other backend makes, and saying so is
the point.

## Activation (one-time, local)

```sh
mf init
```

That scaffolds the project policy file, writes the standards and the agent-instruction
source, puts the hook files in the versioned `.githooks/` directory, points git at it,
records the adopted framework version, and generates the vendor instruction files. It
changes this repository's local state and nothing outside it.

To wire only the hooks in a repository that already has the rest:

```sh
mf hooks install          # sets core.hooksPath to .githooks
mf hooks status           # what git currently says
mf hooks uninstall        # removes only the wiring mf installed
```

Install and uninstall are both idempotent, and uninstall removes only what `mf` wired,
never the versioned directory. A `core.hooksPath` this repository set and `mf` did not is
left exactly as it is: git honours one hooks path, so replacing another tool's does not
add this gate beside theirs, it switches theirs off.

Two hooks live in `.githooks/`, and git discovers both by name once the path is set:

- `pre-push` runs `mf check` and then `mf review --role r2`.
- `commit-msg` runs `mf check commit --message "$1"`, so the commit vocabulary of
  `github.md` is checked against the subject still under the author's cursor rather than
  against a commit already written.

**Both fail closed.** A hook that cannot find the `mf` binary, or cannot identify a
repository, stops the push or the commit and says what to do about it. A gate that cannot
find its runner has not passed; it has not run, and those are only the same thing to a
hook that lies. Set `MF_BIN` to an absolute path when the binary is neither on `PATH` nor
built at the repository root.

The same gates run in CI, in the `gates` job of `.github/workflows/gate.yml`, from a
binary built out of the tree under test — so a change that breaks a gate can fail its own
pull request.

## Configuration

Policy lives in the committed `.framework.toml`; how to reach a vendor lives in the
per-user machine file. Read and write both with `mf config`:

```sh
mf config list --provenance    # every resolved value and the layer it came from
mf config get roles.r2.backends
mf config set roles.r2.blocking true --machine
mf config validate             # load and report every problem, including missing routes
```

Values resolve through a cascade, highest precedence first:

```text
environment  >  .framework.toml  >  machine file  >  deprecated r2.* git config  >  built-in default
```

The project layer outranks the machine layer: a machine may complete a chain a repository
left undeclared, never substitute one the repository chose. The environment outranks both,
for one run and only for the person running it.

The environment form of a key is mechanical: uppercase the key and replace each dot with
an underscore, prefixed `MF_`. `roles.r2.backends` is `MF_ROLES_R2_BACKENDS`;
`review.base` is `MF_REVIEW_BASE`.

It reaches a key the cascade already resolved. Every key in the table below is one of
those, so each has a working environment form. A key under an entity no file defines —
`backends.<name>.model` for a backend nothing declares — has nothing to override, and the
variable is ignored rather than creating one: a backend defined by an environment
variable would be a reviewer no repository chose and no clone can reproduce.

| Key | Meaning | Default |
|---|---|---|
| `roles.<role>.backends` | Ordered chain for `r1`, `r2`, `r3` or `explain` | `codex` for `r2`; empty for the rest |
| `roles.<role>.blocking` | A blocking finding from this role stops its caller | `false` |
| `roles.<role>.require_cross_provider` | This role carries the cross-provider rule | `true` for `r2`, `false` otherwise |
| `review.base` | Branch to review against | `main` |
| `review.model`, `review.effort` | Chain-wide, for a backend that pins neither of its own | `effort` is `high` |
| `review.max_diff_bytes` | Diff size limit before truncation | `30000` |
| `review.timeout_seconds` | Total wall-clock budget for one review | `240` |
| `backends.<name>.kind` | `cli`, `api`, `in-session`, `external` or `inproc` | — |
| `backends.<name>.provider` | The provider label this backend claims | — |
| `backends.<name>.command`, `.args` | For a `cli` backend | — |
| `backends.<name>.unavailable_patterns` | What this tool's failures look like when it is unavailable | — |
| `backends.<name>.model`, `.effort` | Pins one backend, outranking the chain-wide `review.*` | — |
| `backends.<name>.structured` | This `cli` answers with the findings schema the role prompt asks for, so its severities survive and can block | `false` |
| `providers.<name>.kind` | `openai-compatible`, `anthropic` or `google` | `openai-compatible` |
| `providers.<name>.endpoint` | Base URL, without `/chat/completions` | — |
| `providers.<name>.api_key_env` | **Name** of the environment variable holding the key | — |
| `paths.standards`, `paths.specs`, `paths.adr`, `paths.agents_source`, `paths.agents_file` | Where the gates look | `docs/standards`, `docs/specs`, `docs/adr`, `docs/agents/instructions.md`, `AGENTS.md` |

A backend a machine defines is shadowed by a project backend of the same name **whole**,
never field by field, so a machine adds backends and never quietly substitutes one the
repository already chose.

**Secrets are never stored here.** `providers.<name>.api_key_env` holds the *name* of the
variable carrying the key. A committed file may not name an endpoint, an `api_key_env` or
an `api_key` at all, and the loader refuses one that does, by key, rather than overriding
it: a file that can carry reachability can carry a credential. `git config --list` and a
committed TOML both end up in bug reports and screenshots; a key does not belong in
either. A `localhost` endpoint needs no key.

A committed `command` may name only a bare program to resolve on `PATH`. Not a path, not
a string with shell metacharacters, and not an interpreter — the loader refuses each by
name. The rule exists because a committed backend runs on whoever clones the repository:
it may select a tool the contributor already installed, never supply the code it
executes. A machine file is its owner's own, so the rule does not apply there.

Where the documents live has no machine layer at all. A machine able to redirect
`paths.standards` could make the same commit pass a gate here and fail it there, which is
the drift these gates exist to catch.

## Environment variables

- `SKIP_R2_REVIEW=1`: skip the review for this push. The deterministic checks still run —
  they call no model and cost nothing, while the review can take minutes.
- `MF_ROLES_R2_BLOCKING=1`: block this push when the reviewer classes a finding as
  blocking. Same key, run scope; `mf config set roles.r2.blocking true --machine` is the
  persistent form.
- `MF_REVIEW_BASE=<branch>`, `MF_ROLES_R2_BACKENDS=<list>`, `MF_REVIEW_MODEL=<model>`,
  `MF_REVIEW_EFFORT=<effort>`: the same keys, for one run. `MF_REVIEW_MODEL` and
  `MF_REVIEW_EFFORT` are chain-wide, so they reach only a backend that pins neither of
  its own — a backend carrying `backends.<name>.model` keeps it. That is the point of a
  per-backend pin: a chain mixes models deliberately, and a value meant to fill the gaps
  must not silently retune the reviewer a repository benchmarked and chose.
- `MF_BIN=<path>`: where this machine keeps the binary, for a checkout that is neither on
  `PATH` nor built in place. The hooks refuse a value that is not executable rather than
  reading a broken override as no override.
- `NO_COLOR`: honoured wherever this framework writes colour.

The deprecated `r2.*` git-config keys are still read, at the lowest layer above the
built-in defaults, so a clone configured before the seam still resolves. They are read
and never written; move them up with

```sh
mf config migrate
```

which takes them over into the machine file and leaves the originals in place.

## Behavior

- On the base branch (nothing to review against itself): skipped, push proceeds.
- A branch that adds nothing over its base: skipped, and the run says so.
- A backend that is unavailable: the chain advances, and the skip and its reason are
  printed.
- No backend available: R2 did not run, the push proceeds, and the run says so — even
  under `roles.r2.blocking`, because a reviewer that never ran is not a finding and an
  expired quota must not lock the repository.
- A backend that reviewed and reported findings: printed, push proceeds — unless blocking
  is on **and** the reviewer itself classed a finding as blocking. Severity is the
  reviewer's judgement; treating every advisory note as a stop sign would make the flag
  unusable and push people to `--no-verify`.
- Prose from a backend that cannot express per-finding structure is advisory by
  construction, so a chain of those backends can raise findings and never block. Nothing
  in that answer claimed a severity, and inventing one would be filing a finding under a
  heading its author did not choose.
- A chain naming a backend nothing defines: the run fails and names it. That is a
  misconfiguration, not an outage.

The review is **advisory** by default, matching `ai_guidelines.md` ("A Reviewer finding
is advisory, not binding, but an unresolved finding must be addressed or justified,
never silently dropped").

## Bypass

Use `SKIP_R2_REVIEW=1 git push`, or Git's own `git push --no-verify`. The second skips
the deterministic checks too. A bypass means the layer did not run; record that in the PR
per the next section.

## Recording in the PR

In the PR Review Checklist (`github.md`), name the backend that reviewed and the
concrete models on both sides (for example: Author `<author-model>`, Reviewer
`codex / <reviewer-model>`), including any override in effect. The runner prints

```text
Reviewed by: <backend> / <provider> / <model>
Cross-provider: <state> (<what was corroborated and what was not>)
```

for exactly that purpose — the provider is in the line because it is the fact R2 is about,
and a report naming only the backend leaves the reader to look it up. Copy the
cross-provider note whole: half of it copied is the half that overclaims. If R2 did not
run — no backend available, skipped, or bypassed — note it and why, as the checklist
requires.

Naming the backend is not bookkeeping: a chain that fell through to a weaker reviewer
produced a weaker review, and the human adjudicating the PR is the one who decides what
that is worth.

## Manual run

```sh
mf review --role r2                 # run the gate now
mf review --role r2 --dry-run       # print the whole chain and its fallbacks, run nothing
mf review --role r2 --base <ref>    # against a base other than the configured one
```

`--dry-run` describes every backend rather than stopping at the first, because the point
is to show what would happen, fallbacks included. R3 additionally takes `--pr <number>`
and `--post`, which read the pull request for its intent and upsert the review as a
comment.

`mf eval` measures a backend against diffs carrying deliberately planted defects and
reports the hit rate and the false-positive count separately. It says what a reviewer
catches, which is a different question from whether one is configured.
