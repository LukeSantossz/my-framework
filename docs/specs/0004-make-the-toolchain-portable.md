# SPEC: chore: make the toolchain portable with a skills standard, de-pinned models, and interactive setup

## Problem
The framework hard-pins concrete model names in its standards and scripts and never
says which external skills it consumes, when they enter the pipeline, or what to do
when one is absent — so any project (or session) whose toolchain differs from the
original machine silently loses parts of the framework instead of falling back
deliberately.

## Design Decision
Make the toolchain explicit and configurable in three connected pieces. A new binding
standard, `docs/standards/skills_guidelines.md`, inventories every external capability
the framework consumes (Superpowers, Codex CLI, Caveman, the Matt Pocock engineering
skills, and the project agent docs) with pipeline stage, usage, required-vs-optional,
install/verify command, and declared fallback. The standards and the R2 runner drop
model pins in favor of roles: `scripts/codex-review.sh` resolves the reviewer as
`CODEX_REVIEW_MODEL`/`CODEX_REVIEW_EFFORT` (env) → `git config codexreview.model` /
`codexreview.effort` (persisted, local) → current defaults (`gpt-5.5`/`high`), and
`codex_review.md` states the role requirement (Reviewer provider ≠ Author) with the
default named as a default, recording concrete models per PR. `scripts/setup.sh`
gains an `--interactive` mode that, after the normal bootstrap, asks for the R2
reviewer model, the reasoning effort, and the token-economy choice, persisting the
first two via `git config` and echoing a summary; the default non-interactive mode
is unchanged.

## Alternatives Considered
- Committed config file (`.framework.conf`) for model choices: rejected — invents a
  file format plus bash parsing, and per-machine differences would still need env
  overrides, duplicating the mechanism. `git config` is local, uncommitted, and
  matches the existing `core.hooksPath` activation pattern.
- Docs-only de-pinning (role language plus env vars, no persistence and no
  interactive mode): rejected — drops the approved adoption-questionnaire item and
  leaves every new clone re-deriving its model choices by hand.
- Making Caveman a required skill verified by setup: rejected — a hard dependency
  for a stylistic gain; it stays optional with a declared fallback (plain terse
  mode in conversation).
- Inventorying all locally installed skills, including frontend/styling packs
  (gsap-*, ui-ux-pro-max, etc.): rejected — those are project-type tools, not
  development-process capabilities; the standard records this exclusion in one line.

## Scope
- Includes:
  - `docs/standards/skills_guidelines.md`, new binding standard, listed in
    `docs/standards/INDEX.md`, covering five capability categories: Superpowers
    (process orchestration, R1), Codex CLI (R2), Caveman (token economy, optional),
    Matt Pocock engineering skills (per-stage tools: `to-prd`/`to-issues` at intake,
    `triage` on the label state machine, `grilling`/`grill-with-docs` before the
    Spec Gate, `domain-modeling`/`codebase-design`/`design-an-interface`/
    `improve-codebase-architecture` at design, `tdd`/`diagnosing-bugs` at
    implementation, `find-skills`/`handoff`/`setup-matt-pocock-skills` as support),
    and project agent docs (`docs/agents/`, which are the persisted configuration of
    `setup-matt-pocock-skills`). Each entry answers: pipeline stage, how to use,
    required vs optional, install/verify command, declared fallback when absent.
  - Overlap precedence rule in the same standard: Superpowers process skills govern
    the pipeline when the agentic flow is orchestrating; the Matt Pocock equivalents
    (`tdd`, `diagnosing-bugs`) apply in standalone/manual use. One skill per concern
    per session, never both.
  - `scripts/codex-review.sh`: reviewer model and effort resolved with precedence
    env (`CODEX_REVIEW_MODEL`, `CODEX_REVIEW_EFFORT`) → `git config codexreview.model`
    / `codexreview.effort` → defaults `gpt-5.5`/`high`. Dry-run prints the resolved
    command.
  - `docs/standards/codex_review.md`: Roles section becomes role-based (provider
    requirement, defaults named as defaults), the stale `claude-opus-4-8` Author pin
    is removed from Recording in the PR (record the concrete session models instead),
    and the two new variables join the Environment variables section with their
    precedence.
  - `scripts/setup.sh --interactive`: after the normal bootstrap, three stdin
    prompts — R2 reviewer model (Enter = keep default), reasoning effort (Enter =
    keep default), token economy (Caveman / plain terse / off, informational) — the
    first two persisted via `git config codexreview.*` (Enter writes nothing, so
    defaults can evolve; EOF on stdin counts as Enter), then a summary of the
    choices. Without the flag, behavior is byte-for-byte the current one.
  - Tests first for every criterion below, in the existing suites.
- Does NOT include:
  - Issue Model generalization or issue-template changes.
  - Framework README, adoption story, or versioning.
  - Global-vs-repo `CLAUDE.md` conflict resolution.
  - Any change to the advisory behavior of the R2 gate or the hook.
  - Installing skills automatically (setup only verifies and reports).
  - Changes to `token_economy.md` (Caveman reference stays as is).
  - Frontend/styling skills in the inventory.

## Acceptance Criteria
- review_model_env_override: with `CODEX_REVIEW_MODEL=m CODEX_REVIEW_EFFORT=e`,
  dry-run prints `model="m"` and `model_reasoning_effort="e"`.
- review_model_git_config_fallback: with no env vars and `git config
  codexreview.model m2` / `codexreview.effort e2` set, dry-run prints `m2`/`e2`.
- review_model_env_beats_git_config: with both set, the env values win.
- review_model_default_when_unset: with neither, dry-run prints `gpt-5.5`/`high`.
- setup_interactive_persists_choices: `setup.sh --interactive` fed
  `m3\nxhigh\nterse\n` on stdin leaves `git config codexreview.model` = `m3` and
  `codexreview.effort` = `xhigh` in the repo, and the output summary names them.
- setup_interactive_enter_keeps_defaults: fed empty answers, no `codexreview.*`
  keys are written.
- setup_noninteractive_never_prompts: without the flag and with stdin closed, no
  prompt is emitted, no `codexreview.*` key is written, and exit code is 0.
- skills_guidelines_covers_declared_capabilities: a guard test asserts
  `skills_guidelines.md` contains a section for each of the five categories and the
  precedence rule.
- docs_consistency_passes_with_new_standard: the docs-consistency check passes on
  the tree with `skills_guidelines.md` listed in `INDEX.md`, and
  `codex_review.md` no longer contains `claude-opus-4-8`.

## Reproducibility
- `bash scripts/test/codex-review.test.sh` — expected: all pass, 0 failed.
- `bash scripts/test/setup.test.sh` — expected: all pass, 0 failed.
- `bash scripts/test/docs-consistency.test.sh && bash scripts/test/docs-consistency.sh`
  — expected: all pass; `all checks passed.`
- Versions: bash (Git for Windows), git ≥ 2.40, gh ≥ 2.40, Codex CLI 0.132.0
  (downgraded from 0.135.0 for the Windows sandbox fix). No randomness involved.

## Risks and Assumptions
- Assumes `git config codexreview.*` keys are acceptable local state, like
  `core.hooksPath`; cloning fresh loses them by design (re-run setup).
- Assumes the Matt Pocock skills remain installed at user scope
  (`~/.claude/skills/`); the standard documents `setup-matt-pocock-skills` and
  `find-skills` as the recovery path, not this repo.
- Assumes Codex CLI accepts any model string passed via `-c model=`; an invalid
  model fails inside the advisory gate without blocking pushes.
- Invalidated if the framework later adopts a committed per-project config file;
  the precedence chain would need a fourth level.
