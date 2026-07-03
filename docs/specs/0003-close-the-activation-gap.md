# SPEC: chore: close the activation gap with setup script, templates, and CI

## Problem
The framework's activation mechanisms exist only as written instructions — the R2
pre-push hook is not activated (`core.hooksPath` unset in the framework's own repo),
the triage labels do not exist in the tracker, the PR and Issue models live only as
prose in `github.md`, and nothing runs the shell tests or documentation-consistency
checks automatically — so the standards can silently fail to activate (the Gap).

## Design Decision
Turn instructions into mechanisms. Add `scripts/setup.sh`, an idempotent bootstrap
that activates the local mechanisms (sets `core.hooksPath`, creates missing triage
labels via `gh`) and reports toolchain state without installing anything. Mirror the
PR and Issue models as `.github/` templates so GitHub applies them mechanically. Add
a CI workflow that only calls locally runnable scripts: the existing shell tests plus
a new `scripts/test/docs-consistency.sh` holding the durable documentation checks.

## Alternatives Considered
- Keep activation as documented instructions only (status quo): rejected — the R2
  gate was fully documented and still sat inactive in the framework's own repository;
  instructions alone are exactly what reopens the Gap.
- Verify activation via a Claude Code SessionStart hook that checks the toolchain at
  the start of every session: rejected for now — it couples activation to a single
  harness and cannot cover the remote side (CI, templates); `setup.sh` plus CI covers
  local and remote with less coupling. May be revisited later as an addition.
- Put the consistency checks inline in the workflow YAML: rejected — logic embedded
  in YAML cannot be run locally, drifts from the scripts, and breaks the principle
  that CI only invokes scripts a developer can run by hand.

## Scope
- Includes:
  - `scripts/setup.sh`: sets `core.hooksPath .githooks`; checks `codex` and `gh`
    presence and `gh` authentication (advisory); creates the five triage labels from
    `docs/agents/triage-labels.md` when missing; prints an activation report.
  - `scripts/test/setup.test.sh`: tests for the setup runner with stubbed `git`/`gh`.
  - `scripts/test/docs-consistency.sh`: deprecated-wording check and
    `INDEX.md`-versus-`docs/standards/` listing drift check.
  - `.github/PULL_REQUEST_TEMPLATE.md` and `.github/ISSUE_TEMPLATE/issue.md`
    mirroring the current models in `github.md` verbatim.
  - `.github/workflows/ci.yml` running the shell tests and the consistency script on
    push and pull request to `main`.
  - Documentation touches: `codex_review.md` Activation section points at
    `setup.sh`; `INDEX.md` notes CI as the continuous verification of the standards.
- Does NOT include:
  - Generalizing the Issue Model (its migration bias is a separate finding; the
    template mirrors the model as it stands today).
  - `shellcheck` or any new CI dependency beyond bash and grep.
  - Any change to the review scripts' or pre-push hook's behavior.
  - The other review findings: global `CLAUDE.md` conflicts, model pins, token
    economy claims, durable specs, adoption story.
  - Installing or configuring Caveman, Superpowers, or Codex themselves.

## Acceptance Criteria
Phrased as verifiable test outcomes; each becomes a test (or a direct check where
noted) before its implementation.

- setup_configures_hookspath: after `setup.sh` runs, `git config core.hooksPath`
  returns `.githooks`.
- setup_creates_missing_labels: with a stubbed `gh` reporting no labels, `setup.sh`
  issues one `gh label create` per missing label, using the five names from
  `triage-labels.md`.
- setup_skips_existing_labels: with a stubbed `gh` reporting all five labels
  present, no `gh label create` call is issued.
- setup_reports_missing_toolchain: with `codex` absent from `PATH`, `setup.sh`
  reports the absence and exits 0 (advisory, consistent with the R2 gate).
- setup_is_idempotent: running `setup.sh` twice in a row exits 0 both times and
  produces the same end state.
- docs_consistency_detects_deprecated_wording: the check exits non-zero when a file
  under `docs/standards/` contains "Self-Review Checklist" or "author approves", and
  exits 0 on the current tree.
- docs_consistency_detects_index_drift: the check exits non-zero when a `.md` file
  other than `INDEX.md` exists under `docs/standards/` but is not referenced in
  `INDEX.md`, or when `INDEX.md` references a file that does not exist, and exits 0
  on the current tree.
- templates_mirror_standards: `PULL_REQUEST_TEMPLATE.md` contains the five PR Model
  section headings from `github.md`, and the issue template contains the Issue Model
  section headings (direct check).
- ci_calls_local_scripts_only: the workflow YAML contains no inline check logic;
  every step invokes a script under `scripts/` (direct check by reading the file).

## Reproducibility
- Verify on branch `chore/activation-gap` at the PR head.
- Commands: `bash scripts/test/setup.test.sh`,
  `bash scripts/test/docs-consistency.sh`, `bash scripts/test/codex-review.test.sh`;
  the CI run on the PR shows the same scripts passing on `ubuntu-latest`.
- Platform: Windows 11, Git Bash (POSIX sh) locally; `ubuntu-latest` in CI.

## Risks and Assumptions
- Assumption: the repository is hosted on GitHub, so `gh` and GitHub Actions are
  available; `setup.sh` treats an unauthenticated `gh` as a reported skip, not an
  error.
- Assumption: label descriptions come from the meanings in `triage-labels.md`;
  colors are GitHub defaults (approved at design review).
- Risk: the root `SPEC.md` is transient and overwrites the merged domain-glossary
  spec; that content remains in git history (PR #2). Moving to durable specs under
  `docs/specs/` is a separate finding and intentionally out of scope here.
- Assumption: CI failures on the consistency checks are a signal to fix the docs or
  the check, never to delete the check silently — consistent with `ai_guidelines.md`
  on never silencing errors.
