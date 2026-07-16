# SPEC: chore(standards): close the activation holes and correct the unenforced claims

## Problem

The framework's guards check its documents but not its activation surface, so the
two places where the Gap actually reopens — the Author's `CLAUDE.md` and commit
messages — are the only two with no guard at all, and the no-attribution rule
stated four times is the one with violations already merged to `main`; alongside
this, five claims across the standards describe an external reality that does not
hold.

## Design Decision

Guard the activation surface, and close every stale claim on the side that is
true rather than the side that is convenient.

The no-attribution rule gets the guard its four restatements never bought it: a
check over the commits a branch adds, wired into CI, so a `Co-Authored-By` or
AI-attribution line fails the build. The two existing violations (`859daf2`,
`757695d`) are reachable from `main` and already published; rewriting them means
force-pushing shared history, which costs more than it buys. They are recorded as
a known limitation instead, and the guard is scoped to what a branch adds so the
past does not permanently redden CI. `CLAUDE.md` gains the same content guard
`AGENTS.md` already has, asserting it still points at `INDEX.md` and the
precedence order — the invariant `token_economy.md` declares must survive
compression byte-for-byte and that nothing verified.

The token-economy claims are corrected against the installed skill rather than
aspirationally. Caveman is installed at `~/.claude/skills/caveman`, is 49 lines,
and provides terse conversational mode only: `caveman-compress` does not exist in
it. So `CLAUDE.md` is not in compressed form and cannot be — the sentence claiming
it is, is simply false, and `skills_guidelines.md` compounds it by reporting the
skill as not installed.

Policy §1 stops being always-on, on the Developer's decision of 2026-07-15: the
whole token economy is an opt-in the adopter chooses when initializing the
framework in a project, not a default the framework imposes. This resolves the
claim on the side that is both true and wanted — Caveman is optional, so a
repository that never opts in is fully conformant, and the policy describes what
happens when it is chosen rather than asserting a compression that never ran. The
capability's absence is recorded where the framework already records absent
capabilities, with a declared fallback. The reported benchmark percentages lose
the reproducibility they never had and are dropped rather than dressed up, per
`ai_guidelines.md` No Fabricated Evidence.

The remaining two are corrections of fact. The skills inventory names `grilling`
and `diagnosing-bugs`, which match nothing installed (the real names are
`grill-me` and `diagnose`), and `domain-modeling`, `codebase-design` and
`design-an-interface`, which match nothing at all. Superpowers is marked required
and is not installed, so "enforced by the Superpowers TDD phase" — asserted in
three documents — is the wrong verb for an honor-system rule; it becomes "run by
… when installed", with the existing fallback unchanged. And "All output in
English" is unscoped in `CLAUDE.md` and `ai_guidelines.md` while scoped in
`INDEX.md`, `code_conventions.md` and `AGENTS.md`; read literally the unscoped
form forbids answering the Developer in their own language. The scoped form is
the intended one, so the two unscoped statements adopt it — the conflict is
removed at the source rather than adjudicated, since `code_conventions.md`'s
precedence order ranks rule types and cannot arbitrate two repo documents stating
one rule at different scope.

## Alternatives Considered

- **Rewrite the two attributed commits out of `main`**: rejected — they are
  published; a force-push over shared history to fix a message costs more than
  the defect, and the honest record of a violation is itself useful. A known
  limitation plus a guard against recurrence is proportionate.
- **Guard attribution over all of history rather than the branch's commits**:
  rejected — it would fail CI forever on `main` for two commits nobody will
  rewrite, and a permanently red check is a disabled check.
- **Hand-compress `CLAUDE.md` so the claim becomes true**: rejected — the claim
  can be closed from either side, but this side has no tool, no verification, and
  a live risk: `token_economy.md` itself says a compression that breaks activation
  reopens the Gap and is rejected. Hand-rolling the one transformation the
  framework says must preserve activation byte-for-byte, with no skill to repeat
  it and no check to prove it, trades a false sentence for a real hazard.
- **Add a guard that greps `~/.claude/skills/` for the named skills**: rejected —
  it pins the suite to one machine's home directory and would fail in CI, where no
  skills are installed. Correcting the names is the proportionate fix; the
  inventory is documentation of an external environment, and no cheap guard can
  verify it from inside the repo.
- **Promote the token-economy correction to an ADR**: rejected — it reverses no
  hard-to-reverse decision and weighs no real trade-off; it records that a
  capability was never there. The spec archive carries it.

## Scope

- Includes:
  - `scripts/test/docs-consistency.test.sh`: a guard failing when any commit the
    branch adds over `main` carries a `Co-Authored-By` or AI-attribution line; a
    `claude_md_points_to_standards` guard mirroring the existing
    `agents_file_points_to_standards`, asserting `CLAUDE.md` still references
    `docs/standards/INDEX.md` and `code_conventions.md`.
  - `README.md`: one Known Issues & Limitations entry recording the two attributed
    commits in `main`'s history, why they are not rewritten, and that recurrence is
    now guarded.
  - `CLAUDE.md`: the false "kept in its caveman-compress form" sentence removed;
    the English rule scoped as `INDEX.md` scopes it.
  - `docs/standards/token_economy.md`: Policy §1 restated as an opt-in chosen at
    initialization rather than an always-on default, naming `caveman-compress` as
    absent and declaring the fallback; the unreproducible percentages dropped.
  - `docs/standards/skills_guidelines.md`: Caveman recorded as installed and
    providing terse mode only, with `caveman-compress` absent and its fallback
    declared; `grilling` → `grill-me`, `diagnosing-bugs` → `diagnose`; the three
    skills that match nothing removed from the stage table.
  - `docs/standards/ai_guidelines.md`: the English rule scoped; "enforced by the
    Superpowers TDD phase" reworded.
  - `docs/standards/code_conventions.md` and `docs/standards/INDEX.md`: the same
    rewording of the Superpowers enforcement claim.
  - `CONTEXT.md`: the caveman-compress and Token Economy terms corrected to match.
  - Every change lands test-first where a guard is involved.
- Does NOT include:
  - Rewriting, reverting or amending any commit already on `main`.
  - Compressing `CLAUDE.md`, or authoring a `caveman-compress` capability.
  - Installing, vendoring or verifying any external skill; the inventory records
    the environment, it does not manage it.
  - A `commit-msg` hook; CI is where the rule must hold, and a local hook is
    opt-in state a clone can silently lack — the same failure `core.hooksPath`
    already demonstrates.
  - Setting `core.hooksPath` as a versioned change; it is local state and
    `scripts/setup.sh` already applies it. That the gate was never activated in
    this clone is an operational finding, recorded here, not a code defect.
  - Any change to the R2/R3 layers, the Type Table, or the spec/ADR numbering
    rules landed by spec `0010`.
  - Building the interactive initialization the Developer proposed on 2026-07-15
    — a CLI that asks which capabilities to install and wires the skills, hooks
    and configs the framework needs. This spec only makes the token economy
    opt-in in the standards; making the choice real, and extending
    `setup.sh --interactive` beyond the informational prompt it has today, is a
    feature with its own design decisions (what may be installed, what a
    declined capability leaves behind, how a choice is persisted and re-run) and
    earns its own spec and Spec Gate.

## Acceptance Criteria

- attribution_in_branch_commits_fails: with a commit carrying a
  `Co-Authored-By:` line on the branch, the guard fails naming the commit; on the
  branch as it stands, it passes.
- attribution_guard_ignores_merged_history: the guard does not fail on the two
  pre-existing attributed commits reachable from `main`.
- claude_md_points_to_standards: the guard fails when `CLAUDE.md` stops
  referencing `docs/standards/INDEX.md` or `code_conventions.md`, and passes on
  the current file.
- readme_records_attributed_history: Known Issues & Limitations names the
  attributed commits and why they stand.
- caveman_compress_claim_removed: no document states that `CLAUDE.md` is kept in
  compressed form; `token_economy.md` names `caveman-compress` as absent, states
  that the token economy is opt-in at initialization rather than always on, and
  declares the fallback; `skills_guidelines.md` records Caveman as installed with
  terse mode only.
- token_economy_drops_unreproducible_numbers: `token_economy.md` contains no
  percentage claim lacking a reproduction path.
- skills_inventory_names_installed_skills: `skills_guidelines.md` names
  `grill-me` and `diagnose`, and no longer names `grilling`, `diagnosing-bugs`,
  `domain-modeling`, `codebase-design` or `design-an-interface`.
- superpowers_claim_is_not_enforcement: no document claims the test-first order is
  "enforced by" Superpowers.
- english_rule_scoped_consistently: `CLAUDE.md` and `ai_guidelines.md` scope the
  English rule as `INDEX.md` does.
- all_suites_green: all four suites pass on the final tree.

## Reproducibility

Run, from the repository root, with git >= 2.40 and bash (Git for Windows):

```sh
bash scripts/test/docs-consistency.test.sh
bash scripts/test/docs-consistency.sh
bash scripts/test/setup.test.sh
bash scripts/test/codex-review.test.sh
```

All four pass, 0 failed. The attribution guard reads `main..HEAD`, so it needs the
base branch present; CI already checks out with `fetch-depth: 0`. The installed
Caveman skill was inspected at `~/.claude/skills/caveman/SKILL.md` (49 lines, no
`caveman-compress`). No randomness is involved.

## Risks and Assumptions

- Assumption: the Spec Gate approval for this scope is the Developer's approval of
  the seven-item finding list on 2026-07-15, with the instruction to fix them one
  at a time.
- Assumption: this branch stacks on `chore/restore-reviewer-record-and-harden-guards`
  (PR #12), because spec `0011` must follow `0010` for the contiguity guard landed
  by that PR. If #12 is not merged first, this PR's base is wrong.
- Risk: the attribution guard reads `main..HEAD`, which is empty on `main` itself,
  so a direct push to `main` is unguarded. Accepted: the repo works through PRs,
  and CI also runs on `pull_request`, where the range is non-empty.
- Risk: correcting the skills inventory pins it to today's environment, and it will
  drift again as skills are renamed upstream. Accepted deliberately — no guard can
  verify an external home directory from CI, and a wrong name is worse than a name
  that may age.
- What would invalidate this spec: a real `caveman-compress` capability arriving,
  which reopens Policy §1 as an always-on rule rather than a declared target.
