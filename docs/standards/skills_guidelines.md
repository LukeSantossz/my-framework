# Skills Guidelines

External capabilities the framework consumes: what each is for, when it enters
the pipeline, whether it is required, how to install and verify it, and the
declared fallback when it is absent. An absent capability degrades the pipeline
deliberately — never silently.

Scope: development-process capabilities only. Project-type skill packs
(frontend/styling such as gsap-*, ui-ux-pro-max, industrial-brutalist-ui) are
deliberately outside this standard; they belong to individual projects, not to
the process.

## Superpowers

- Pipeline stage: process orchestration end to end — brainstorming (design),
  writing-plans (plan), subagent-driven-development (TDD implementation),
  requesting/receiving-code-review (R1), finishing-a-development-branch.
- How to use: invoke the phase's skill before acting in that phase; the
  orchestrator's phases map one-to-one onto `spec_method.md` (Brainstorm feeds
  the SPEC, the Plan turns Acceptance Criteria into failing tests).
- Required: yes for the agentic flow.
- Install/verify: Claude Code plugin `superpowers` (claude-plugins-official
  marketplace); verify that `superpowers:*` skills appear in the session's
  skill list.
- Fallback: execute the phases manually per `spec_method.md` and
  `ai_guidelines.md` — SPEC, Spec Gate, failing test, implementation,
  self-review. R1 then degrades to the Author Self-Review; record that in the
  PR review-layers section.

## Codex CLI

- Pipeline stage: R2 cross-provider review, as the pre-push gate defined in
  `codex_review.md`.
- How to use: activated by `bash scripts/setup.sh` (hooks path); runs on every
  push; also runnable by hand via `bash scripts/codex-review.sh`.
- Required: optional, strongly recommended (it is the only second-provider
  reviewer wired today).
- Install/verify: `npm install -g @openai/codex`, then `codex login`; verify
  with the toolchain report of `bash scripts/setup.sh`.
- Fallback: per `ai_guidelines.md` Review Composition — R1 plus the human PR
  review stand in for R2, and the PR notes the absence.

## Caveman

- Pipeline stage: token economy, conversation style only (`token_economy.md`).
- How to use: compressed terse replies in conversation; never in `SPEC.md`,
  PR, Issue, or commit artifacts.
- Required: optional. Not currently installed; if adopted, install at user
  scope (`~/.claude/skills/`) and record the source in this section.
- Fallback: plain terse mode in conversation. `token_economy.md` is unchanged
  either way.

## Matt Pocock Engineering Skills

Installed at user scope (`~/.claude/skills/`). Their per-repo configuration is
`docs/agents/` (issue tracker, triage labels, domain docs), produced by the
`setup-matt-pocock-skills` skill. When installed, these skills must be used at
their stages:

| Stage | Skills |
| --- | --- |
| Intake | `to-prd`, `to-issues` |
| Triage | `triage` (five canonical labels state machine) |
| Before the Spec Gate | `grilling`, `grill-with-docs` |
| Design | `domain-modeling`, `codebase-design`, `design-an-interface`, `improve-codebase-architecture` |
| Implementation | `tdd`, `diagnosing-bugs` |
| Support | `find-skills`, `handoff`, `setup-matt-pocock-skills` |

- Install/verify: `ls ~/.claude/skills/` shows the set; recover with
  `setup-matt-pocock-skills` (per-repo config) and `find-skills` (discovery).
- Fallback: the framework's own standards cover each stage manually — the
  issue tracker and labels per `docs/agents/issue-tracker.md` and
  `docs/agents/triage-labels.md` via `gh`, design per `spec_method.md`,
  implementation per `code_conventions.md`.

## Project Agent Docs

- Pipeline stage: cross-cutting configuration read by agents and by the skills
  above: `docs/agents/issue-tracker.md`, `docs/agents/triage-labels.md`,
  `docs/agents/domain.md`, plus `CONTEXT.md` and `docs/adr/` at the repo root.
- How to use: read before acting on issues, labels, or domain decisions; they
  are the authoritative per-repo answers to "where do issues live", "which
  label strings", and "where domain knowledge sits".
- Required: yes (they are versioned repo content, not an external install).
- Install/verify: present in the repo; `bash scripts/test/docs-consistency.sh`
  verifies the standards tree they link into.
- Fallback: none needed — if missing, run `setup-matt-pocock-skills` to
  regenerate them.

## Overlap Precedence

- Superpowers process skills govern the pipeline whenever the agentic flow is
  orchestrating; the Matt Pocock equivalents (`tdd`, `diagnosing-bugs`) apply
  in standalone or manual use. One skill per concern per session, never both.
- User instructions and the repo `CLAUDE.md` outrank skills; skills outrank
  default behavior. Conflicts between standards resolve per the precedence
  order in `code_conventions.md`.
