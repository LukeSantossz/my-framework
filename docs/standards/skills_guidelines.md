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
  requesting/receiving-code-review, finishing-a-development-branch. Its review
  pass is one backend of the R1 chain (`ai_guidelines.md` Review Composition),
  not the R1 layer itself: R1 is satisfied by whichever backend in its chain is
  available, and this one is in-session, so it contributes an attestation rather
  than an execution and an absent session counts as unavailable.
- How to use: invoke the phase's skill before acting in that phase; the
  orchestrator's phases map one-to-one onto `spec_method.md` (Brainstorm feeds
  the SPEC, the Plan turns Acceptance Criteria into failing tests).
- Required: by the agentic flow, when that flow is the one orchestrating — its
  phases are the pipeline, so a session that runs them cannot skip it. The
  requirement is on the flow, not on the framework: the phases can be executed by
  hand, and a session that does so is fully conformant. Whether it is installed
  on a given machine is a fact about that machine, so this standard does not
  assert one; `mf doctor` reports what the R1 chain can actually reach here.
- Install/verify: Claude Code plugin `superpowers` (claude-plugins-official
  marketplace); verify that `superpowers:*` skills appear in the session's
  skill list.
- Record the review: because it is `in-session`, nothing can start it and
  nothing can observe that it ran. The session that reviewed records an
  attestation naming the exact commit:

  ```sh
  git config --local mf.attestation.r1 $(git rev-parse HEAD)
  ```

  Without one, `mf review --role r1` reports the backend as unavailable and the
  chain advances. The commit is compared rather than the branch, so an
  attestation does not silently cover the commits pushed after it.
- Fallback: execute the phases manually per `spec_method.md` and
  `ai_guidelines.md` — SPEC, Spec Gate, failing test, implementation,
  self-review. With no backend in the R1 chain available, R1 is reported as not
  having run; record that in the PR review-layers section rather than letting an
  absent layer read as a clean one.

## Codex CLI

- Pipeline stage: R2 cross-provider review, as one backend of the pre-push
  chain defined in `r2_gate.md`. It is the shipped default; `gemini` and any
  OpenAI-compatible endpoint are the alternatives, and the chain advances past
  whichever is unavailable.
- How to use: `mf init` (or `mf hooks install`) points `core.hooksPath` at the
  versioned hooks; `.githooks/pre-push` then runs `mf review --role r2` on every
  push. Run it by hand with `mf review --role r2`, or `--dry-run` to see the
  chain without calling anything.
- Required: optional, strongly recommended (it is the first second-provider
  reviewer in the shipped chain).
- Install/verify: `npm install -g @openai/codex`, then `codex login`; verify
  with `mf doctor`, which reports each backend in each role's chain and whether
  it is reachable from here.
- Fallback: per `ai_guidelines.md` Review Composition — R1 plus CRURA's
  adjudication stage stand in for R2, and the PR notes the absence. The chain
  advances past an unavailable backend first: `gemini` is next in the shipped
  order, and a machine may add an `api` backend of its own.

## The CRUX Explainer

- Pipeline stage: the CRUX Method review aid (`crux_method.md`) — at review time,
  generates a transient, interactive HTML explainer of an implemented change.
- How to use: `mf explain [--base <ref>] [--difficulty easy|medium|hard]
  [--dir <path>] [--dry-run]`. It writes a self-contained, date-prefixed HTML
  file outside version control. Quiz difficulty is `easy`, `medium`, or `hard`,
  defaulting to `medium`; a wrong quiz answer reveals a deeper, skippable
  explanation.
- Required: optional. It is a review aid, never a blocking gate, and the command
  exits zero on every path — including every path where no explainer was
  produced.
- Install/verify: nothing to install. It is not a skill; it is a role in the
  same configuration as R1, R2 and R3 (`roles.explain.backends`), so which model
  explains a change is chosen the same way as which model reviews one. Only a
  prompt-driven backend can serve it: an agentic reviewer answers with a review,
  and this asks for an explainer. Verify with `mf doctor`, which lists the
  explain chain and whether any of it is reachable, or `mf explain --dry-run`.
- Fallback: if no backend in the explain chain is available — or the one that
  answers does not answer with an explainer — none is produced: the reviewer
  reads the diff directly and the Pull Request notes the CRUX aid was absent,
  per `crux_method.md`.
- Composition: the explainer's prose is meant to pass through the `humanizer`
  skill before the final render. `mf explain` does not perform that step —
  nothing in the binary calls a skill — so every explainer it writes says on its
  own face that the pass did not run. Do the pass by hand, or accept the flagged
  degradation; never a silent skip.

## Caveman

- Pipeline stage: token economy, conversation style only (`token_economy.md`).
- How to use: compressed terse replies in conversation; never in `SPEC.md`,
  PR, Issue, or commit artifacts.
- Required: optional, and the whole token economy is optional with it — opting
  in is a choice the adopter makes when initializing the framework in a project,
  not a default the framework imposes. A repository that never opts in is fully
  conformant.
- Install/verify: installed at user scope (`~/.claude/skills/caveman`); verify
  that `caveman` appears in the session's skill list. What is installed provides
  conversational terse mode only: the `caveman-compress` capability that
  `token_economy.md` §1 scopes — the one that would rewrite `CLAUDE.md` into a
  compressed form — is absent from it, so §1's compression cannot be performed
  today by this skill.
- Fallback: plain terse mode in conversation, which the installed skill supplies;
  the context file stays uncompressed, per the fallback `token_economy.md` §1
  declares. `token_economy.md` is unchanged either way.

## Matt Pocock Engineering Skills

Installed at user scope (`~/.claude/skills/`). Their per-repo configuration is
`docs/agents/` (issue tracker, triage labels, domain docs), which `mf init`
writes and `mf agents sync` reads to generate the vendor instruction files.
When installed, these skills must be used at their stages:

| Stage | Skills |
| --- | --- |
| Intake | `to-prd`, `to-issues` |
| Triage | `triage` (five canonical labels state machine) |
| Before the Spec Gate | `grill-me`, `grill-with-docs` |
| Design | `improve-codebase-architecture` |
| Implementation | `tdd`, `diagnose` |
| Support | `find-skills`, `handoff`, `setup-matt-pocock-skills` |

- Install/verify: `ls ~/.claude/skills/` shows the set; recover the skills with
  `find-skills` (discovery), and the per-repo configuration they read with
  `mf init`.
- Fallback: the framework's own standards cover each stage manually — the
  issue tracker and labels per `docs/agents/issue-tracker.md` and
  `docs/agents/triage-labels.md` via `gh`, design per `spec_method.md`,
  implementation per `code_conventions.md`.

## Project Agent Docs

- Pipeline stage: cross-cutting configuration read by agents and by the skills
  above: `docs/agents/issue-tracker.md`, `docs/agents/triage-labels.md`, and
  `docs/agents/domain.md`, which names the repository's own domain-context file
  and its decision-record directory.
- How to use: read before acting on issues, labels, or domain decisions; they
  are the authoritative per-repo answers to "where do issues live", "which
  label strings", and "where domain knowledge sits".
- Required: yes (they are versioned repo content, not an external install).
- Install/verify: `mf init` writes them, along with the instruction source the
  vendor files are generated from. `mf check docs` verifies the standards tree
  they link into; `mf check agents` verifies the generated instruction files
  still match that source.
- Fallback: not applicable (versioned repo content has no degraded mode); if
  missing, `mf init` writes back what is absent and leaves what an adopter has
  edited alone.

## Overlap Precedence

- Superpowers process skills govern the pipeline whenever the agentic flow is
  orchestrating; the Matt Pocock equivalents (`tdd`, `diagnose`) apply
  in standalone or manual use. One skill per concern per session, never both.
- User instructions and the repo `CLAUDE.md` outrank skills; skills outrank
  default behavior. Conflicts between standards resolve per the precedence
  order in `code_conventions.md`.
