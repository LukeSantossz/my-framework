# Activation Gap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the framework's documented activation steps into mechanisms: an idempotent bootstrap script, GitHub PR/Issue templates, and a CI workflow that runs the shell tests and docs-consistency checks.

**Architecture:** Three independent mechanisms sharing one principle — CI and bootstrap only invoke scripts a developer can run by hand. `scripts/setup.sh` applies local activation state (hooks path, triage labels) and reports toolchain advisories. `scripts/test/docs-consistency.sh` holds the durable documentation checks, exercised by its own test file against temp-dir fixtures. `.github/` templates mirror `docs/standards/github.md` verbatim.

**Tech Stack:** Bash (POSIX-leaning, matching `scripts/codex-review.sh` style), `gh` CLI, GitHub Actions (`ubuntu-latest`).

## Global Constraints

- All output in English (identifiers, comments, commit text) — `docs/standards/INDEX.md`.
- Conventional Commits, imperative lowercase subject, no trailing period, NO co-author or AI-attribution lines — `docs/standards/github.md`.
- Test-first order: the failing-test commit precedes the implementation commit — `docs/standards/code_conventions.md` Testing.
- Scripts follow the house style of `scripts/codex-review.sh`: `set -u`, a `log()` helper, env-var overrides for testability, advisory behavior exits 0.
- No new CI dependencies beyond bash and grep (plus `actions/checkout`); no `shellcheck` — SPEC "Does NOT include".
- Shell files stay LF (`.gitattributes` already covers `*.sh`; new files under `.github/` are YAML/Markdown and unaffected).
- Always invoke scripts as `bash path/to/script.sh` (no reliance on the executable bit — repo convention, works on Windows Git Bash).
- The five triage labels and their meanings come verbatim from `docs/agents/triage-labels.md`.

---

### Task 1: Failing tests for the activation bootstrap

**Files:**
- Create: `scripts/test/setup.test.sh`

**Interfaces:**
- Consumes: nothing (first task).
- Produces: the test contract for `scripts/setup.sh` (Task 2): env overrides `GH_BIN`, `CODEX_BIN`; stub `gh` protocol (`GH_LOG` call log, `STUB_GH_LABELS` newline/space-separated existing labels); exit 0 on advisory failures; log phrase `not installed` for a missing codex.

- [ ] **Step 1: Write the failing test file**

Create `scripts/test/setup.test.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# Tests for the activation bootstrap (scripts/setup.sh).
# Each test maps to an Acceptance Criterion in SPEC.md.
set -u

TEST_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$TEST_DIR/../.." && pwd)"
RUNNER="$REPO_ROOT/scripts/setup.sh"

PASS=0
FAIL=0

ok() { PASS=$((PASS + 1)); printf 'ok   - %s\n' "$1"; }
no() { FAIL=$((FAIL + 1)); printf 'FAIL - %s\n' "$1"; printf '       %s\n' "$2"; }

ALL_LABELS='needs-triage needs-info ready-for-agent ready-for-human wontfix'

# Sandbox: throwaway git repos so hooksPath changes never touch this repo, plus
# a stub `gh` whose label list is driven by STUB_GH_LABELS and whose calls are
# appended to GH_LOG.
SANDBOX="$(mktemp -d)"
trap 'rm -rf "$SANDBOX"' EXIT
STUB_DIR="$SANDBOX/bin"
mkdir -p "$STUB_DIR"

cat > "$STUB_DIR/gh" <<'STUB'
#!/bin/sh
printf 'STUB_GH %s\n' "$*" >> "$GH_LOG"
case "$1 $2" in
  "auth status") exit 0 ;;
  "label list") [ -n "${STUB_GH_LABELS:-}" ] && printf '%s\n' $STUB_GH_LABELS; exit 0 ;;
  "label create") exit 0 ;;
esac
exit 0
STUB
chmod +x "$STUB_DIR/gh"

new_repo() {
  d="$SANDBOX/repo-$1"
  git init -q "$d"
  printf '%s\n' "$d"
}

# setup_configures_hookspath
repo="$(new_repo hooks)"
log="$SANDBOX/hooks.log"; : > "$log"
out=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" 2>&1); code=$?
hooks="$(git -C "$repo" config core.hooksPath || true)"
if [ "$code" -eq 0 ] && [ "$hooks" = ".githooks" ]; then
  ok "setup_configures_hookspath"
else
  no "setup_configures_hookspath" "code=$code hooksPath=$hooks out=$out"
fi

# setup_creates_missing_labels
repo="$(new_repo create)"
log="$SANDBOX/create.log"; : > "$log"
out=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="" PATH="$STUB_DIR:$PATH" bash "$RUNNER" 2>&1); code=$?
missing=""
for l in $ALL_LABELS; do
  grep -q -- "label create $l" "$log" || missing="$missing $l"
done
if [ "$code" -eq 0 ] && [ -z "$missing" ]; then
  ok "setup_creates_missing_labels"
else
  no "setup_creates_missing_labels" "code=$code missing_creates=[$missing] out=$out"
fi

# setup_skips_existing_labels
repo="$(new_repo skip)"
log="$SANDBOX/skip.log"; : > "$log"
out=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && ! grep -q "label create" "$log"; then
  ok "setup_skips_existing_labels"
else
  no "setup_skips_existing_labels" "code=$code out=$out"
fi

# setup_reports_missing_toolchain
repo="$(new_repo codex)"
log="$SANDBOX/codex.log"; : > "$log"
out=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" CODEX_BIN=__no_such_codex__ bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s' "$out" | grep -qi "not installed"; then
  ok "setup_reports_missing_toolchain"
else
  no "setup_reports_missing_toolchain" "code=$code out=$out"
fi

# setup_is_idempotent
repo="$(new_repo idem)"
log="$SANDBOX/idem.log"; : > "$log"
out1=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" 2>&1); code1=$?
out2=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" 2>&1); code2=$?
hooks="$(git -C "$repo" config core.hooksPath || true)"
if [ "$code1" -eq 0 ] && [ "$code2" -eq 0 ] && [ "$hooks" = ".githooks" ] && ! grep -q "label create" "$log"; then
  ok "setup_is_idempotent"
else
  no "setup_is_idempotent" "code1=$code1 code2=$code2 hooksPath=$hooks out2=$out2"
fi

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash scripts/test/setup.test.sh`
Expected: all 5 tests print `FAIL` (the runner `scripts/setup.sh` does not exist, so every invocation exits non-zero); final line `0 passed, 5 failed`; exit code 1.

- [ ] **Step 3: Commit the failing tests**

```bash
git add scripts/test/setup.test.sh
git commit -m "test(scripts): add failing tests for the activation bootstrap"
```

---

### Task 2: Implement `scripts/setup.sh`

**Files:**
- Create: `scripts/setup.sh`
- Test: `scripts/test/setup.test.sh` (from Task 1)

**Interfaces:**
- Consumes: the test contract from Task 1 (`GH_BIN`, `CODEX_BIN`, stub `gh` protocol, phrase `not installed`).
- Produces: `scripts/setup.sh`, invoked as `bash scripts/setup.sh`; referenced by Task 7 (`codex_review.md`, `INDEX.md`).

- [ ] **Step 1: Write the implementation**

Create `scripts/setup.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# Activation bootstrap: turns the framework's documented activation steps into
# one idempotent command. Sets core.hooksPath, reports toolchain state
# (advisory), and creates missing triage labels.
# See docs/standards/codex_review.md and docs/agents/triage-labels.md.
set -u

gh_bin="${GH_BIN:-gh}"
codex_bin="${CODEX_BIN:-codex}"

log() { printf '[setup] %s\n' "$1"; }

# Canonical triage labels (docs/agents/triage-labels.md): name|color|description.
LABEL_SPECS='needs-triage|ededed|Maintainer needs to evaluate this issue
needs-info|d876e3|Waiting on reporter for more information
ready-for-agent|0e8a16|Fully specified, ready for an AFK agent
ready-for-human|1d76db|Requires human implementation
wontfix|ffffff|Will not be actioned'

# 1. Must run inside a git repository; the only hard requirement.
repo_root="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  log "not inside a git repository; nothing to activate."
  exit 1
}
log "repo: $repo_root"

# 2. Activate the R2 pre-push gate (idempotent local setting).
git config core.hooksPath .githooks
log "core.hooksPath -> .githooks (R2 pre-push gate active)."

# 3. Toolchain report. Advisory: absence never fails the bootstrap, matching
#    the R2 gate's own skip-with-message behavior.
if command -v "$codex_bin" >/dev/null 2>&1; then
  log "codex: found."
else
  log "codex: not installed; R2 reviews will be skipped until it is (see docs/standards/codex_review.md)."
fi

labels_ok=1
if ! command -v "$gh_bin" >/dev/null 2>&1; then
  log "gh: not installed; skipping triage-label creation."
  labels_ok=0
elif ! "$gh_bin" auth status >/dev/null 2>&1; then
  log "gh: not authenticated (run 'gh auth login'); skipping triage-label creation."
  labels_ok=0
fi

# 4. Create the triage labels that are missing from the tracker.
if [ "$labels_ok" -eq 1 ]; then
  existing="$("$gh_bin" label list --json name --jq '.[].name' 2>/dev/null)"
  printf '%s\n' "$LABEL_SPECS" | while IFS='|' read -r name color desc; do
    if printf '%s\n' "$existing" | grep -qx "$name"; then
      log "label '$name': present."
    elif "$gh_bin" label create "$name" --color "$color" --description "$desc" >/dev/null 2>&1; then
      log "label '$name': created."
    else
      log "label '$name': create failed (check gh permissions for this repo)."
    fi
  done
fi

log "activation bootstrap complete."
exit 0
```

- [ ] **Step 2: Run the tests to verify they pass**

Run: `bash scripts/test/setup.test.sh`
Expected: `5 passed, 0 failed`; exit code 0.

- [ ] **Step 3: Verify the existing suite still passes**

Run: `bash scripts/test/codex-review.test.sh`
Expected: `7 passed, 0 failed` (unchanged).

- [ ] **Step 4: Commit**

```bash
git add scripts/setup.sh
git commit -m "chore(scripts): add idempotent activation bootstrap"
```

---

### Task 3: Failing tests for the docs-consistency check

**Files:**
- Create: `scripts/test/docs-consistency.test.sh`

**Interfaces:**
- Consumes: nothing new.
- Produces: the test contract for `scripts/test/docs-consistency.sh` (Task 4): env overrides `ROOT_DIR` (repo root; docs dir defaults to `$ROOT_DIR/docs/standards`) and `DOCS_DIR`; exit 0 when consistent, non-zero when not; failure output names the offending file or wording.

- [ ] **Step 1: Write the failing test file**

Create `scripts/test/docs-consistency.test.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# Tests for the docs-consistency check (scripts/test/docs-consistency.sh).
# Each test maps to an Acceptance Criterion in SPEC.md.
set -u

TEST_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$TEST_DIR/../.." && pwd)"
CHECK="$REPO_ROOT/scripts/test/docs-consistency.sh"

PASS=0
FAIL=0

ok() { PASS=$((PASS + 1)); printf 'ok   - %s\n' "$1"; }
no() { FAIL=$((FAIL + 1)); printf 'FAIL - %s\n' "$1"; printf '       %s\n' "$2"; }

SANDBOX="$(mktemp -d)"
trap 'rm -rf "$SANDBOX"' EXIT

# A minimal consistent docs tree: INDEX.md referencing a.md, and a.md present.
make_fixture() {
  d="$SANDBOX/$1/docs/standards"
  mkdir -p "$d"
  printf '# Index\n\n- `a.md`: sample standard.\n' > "$d/INDEX.md"
  printf '# A\n\nClean content.\n' > "$d/a.md"
  printf '%s\n' "$SANDBOX/$1"
}

# passes_on_clean_fixture
root="$(make_fixture clean)"
out=$(ROOT_DIR="$root" bash "$CHECK" 2>&1); code=$?
if [ "$code" -eq 0 ]; then
  ok "passes_on_clean_fixture"
else
  no "passes_on_clean_fixture" "code=$code out=$out"
fi

# docs_consistency_detects_deprecated_wording
root="$(make_fixture depr)"
printf 'Fill the Self-Review Checklist before pushing.\n' >> "$root/docs/standards/a.md"
out=$(ROOT_DIR="$root" bash "$CHECK" 2>&1); code=$?
if [ "$code" -ne 0 ] && printf '%s' "$out" | grep -qi "deprecated"; then
  ok "docs_consistency_detects_deprecated_wording"
else
  no "docs_consistency_detects_deprecated_wording" "code=$code out=$out"
fi

# docs_consistency_detects_index_drift (file present, not referenced)
root="$(make_fixture fwd)"
printf '# B\n' > "$root/docs/standards/b.md"
out=$(ROOT_DIR="$root" bash "$CHECK" 2>&1); code=$?
if [ "$code" -ne 0 ] && printf '%s' "$out" | grep -q "b.md"; then
  ok "docs_consistency_detects_index_drift"
else
  no "docs_consistency_detects_index_drift" "code=$code out=$out"
fi

# docs_consistency_detects_missing_reference (referenced, not present)
root="$(make_fixture rev)"
printf -- '- `ghost.md`: does not exist.\n' >> "$root/docs/standards/INDEX.md"
out=$(ROOT_DIR="$root" bash "$CHECK" 2>&1); code=$?
if [ "$code" -ne 0 ] && printf '%s' "$out" | grep -q "ghost.md"; then
  ok "docs_consistency_detects_missing_reference"
else
  no "docs_consistency_detects_missing_reference" "code=$code out=$out"
fi

# passes_on_current_tree (the real docs/standards must be consistent)
out=$(bash "$CHECK" 2>&1); code=$?
if [ "$code" -eq 0 ]; then
  ok "passes_on_current_tree"
else
  no "passes_on_current_tree" "code=$code out=$out"
fi

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash scripts/test/docs-consistency.test.sh`
Expected: all 5 tests print `FAIL` (the check script does not exist yet); final line `0 passed, 5 failed`; exit code 1.

- [ ] **Step 3: Commit the failing tests**

```bash
git add scripts/test/docs-consistency.test.sh
git commit -m "test(scripts): add failing tests for the docs-consistency check"
```

---

### Task 4: Implement `scripts/test/docs-consistency.sh`

**Files:**
- Create: `scripts/test/docs-consistency.sh`
- Test: `scripts/test/docs-consistency.test.sh` (from Task 3)

**Interfaces:**
- Consumes: the test contract from Task 3 (`ROOT_DIR`, `DOCS_DIR`, exit codes, output naming the offender).
- Produces: `scripts/test/docs-consistency.sh`, invoked as `bash scripts/test/docs-consistency.sh`; called by CI (Task 6).

- [ ] **Step 1: Write the implementation**

Create `scripts/test/docs-consistency.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# Docs-consistency check: the durable documentation invariants of the
# standards, runnable locally and in CI. Exits non-zero on any violation.
# Overridable for testing: ROOT_DIR (repo root), DOCS_DIR (standards dir).
set -u

root="${ROOT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null)}"
docs_dir="${DOCS_DIR:-$root/docs/standards}"
index="$docs_dir/INDEX.md"
fail=0

log() { printf '[docs-consistency] %s\n' "$1"; }

if [ ! -f "$index" ]; then
  log "missing $index; nothing to check against."
  exit 1
fi

# Check 1: deprecated wording must not reappear in the standards
# (glossary alignment, see CONTEXT.md and the domain-glossary SPEC).
if matches=$(grep -rEn "Self-Review Checklist|author approves" "$docs_dir"); then
  log "deprecated wording found:"
  printf '%s\n' "$matches"
  fail=1
fi

# Check 2: every standard is listed in INDEX.md (no orphan documents).
for f in "$docs_dir"/*.md; do
  base="$(basename "$f")"
  [ "$base" = "INDEX.md" ] && continue
  if ! grep -qF "$base" "$index"; then
    log "missing from INDEX.md: $base"
    fail=1
  fi
done

# Check 3: every plain-file .md reference in INDEX.md resolves, either in the
# standards dir or at the repo root (e.g. CLAUDE.md, SPEC.md). References
# containing a path separator (e.g. docs/adr/...) are out of scope here.
refs="$(grep -oE '[A-Za-z0-9_./-]+\.md' "$index" | sort -u)"
for ref in $refs; do
  case "$ref" in */*) continue ;; esac
  if [ ! -f "$docs_dir/$ref" ] && [ ! -f "$root/$ref" ]; then
    log "INDEX.md references missing file: $ref"
    fail=1
  fi
done

[ "$fail" -eq 0 ] && log "all checks passed."
exit "$fail"
```

- [ ] **Step 2: Run the tests to verify they pass**

Run: `bash scripts/test/docs-consistency.test.sh`
Expected: `5 passed, 0 failed`; exit code 0. The `passes_on_current_tree` test doubles as the acceptance check that the real `docs/standards/` tree is consistent today.

- [ ] **Step 3: Commit**

```bash
git add scripts/test/docs-consistency.sh
git commit -m "test(scripts): add docs-consistency check for the standards"
```

---

### Task 5: GitHub PR and Issue templates

**Files:**
- Create: `.github/PULL_REQUEST_TEMPLATE.md`
- Create: `.github/ISSUE_TEMPLATE/issue.md`

**Interfaces:**
- Consumes: the PR Model and Issue Model section headings in `docs/standards/github.md` (mirrored verbatim).
- Produces: templates GitHub auto-applies to new PRs and issues; the issue template auto-applies the `needs-triage` label created by Task 2's script.

- [ ] **Step 1: Write the PR template**

Create `.github/PULL_REQUEST_TEMPLATE.md` with exactly this content:

```markdown
<!-- Title: Conventional Commits format, type from the canonical Type Table in
     docs/standards/github.md. Example: feat(auth): implement password recovery -->

## 1. Context

- Motivation:
- Task Link:
- Spec Link:

## 2. What Was Done

<!-- List the technical changes in summary form. -->

## 3. How to Test

<!-- Step-by-step for the reviewer, with concrete commands. -->

1.

## 4. Evidence

<!-- Screenshots or videos if the change is visual; omit if backend or configuration only. -->

## 5. PR Review Checklist

- [ ] Reviewed in the Files Changed tab.
- [ ] Spec approved at the Gate before implementation, and the change matches its Scope (per `docs/standards/spec_method.md`).
- [ ] Each Acceptance Criterion has a passing test; tests were written before their implementation.
- [ ] Commented-out code and unnecessary debug statements removed.
- [ ] Code follows the project style guide.
- [ ] New dependencies work without breaking the build.
- [ ] Review layers recorded: internal Superpowers review (R1), cross-provider review (R2), automated PR review (R3) where applicable, with Author and Reviewer models named (per `docs/standards/ai_guidelines.md` Review Composition). Note any layer that did not run and why.
```

- [ ] **Step 2: Write the Issue template**

Create `.github/ISSUE_TEMPLATE/issue.md` with exactly this content:

```markdown
---
name: Issue
about: Standard issue per docs/standards/github.md Issue Model
title: "type(scope): subject"
labels: needs-triage
---

<!-- Title: Conventional Commits format, type from the canonical Type Table.
     Use fix for defects, not bug. -->

## Description

<!-- What needs to be done and why. -->

## Context

<!-- History or technical motivation justifying the issue. -->

## Current Usage

<!-- Current state of what will change: entities, operations, affected dependencies. -->

## Recommended Alternative

<!-- Proposed solution and the criteria justifying it (name plus reasons). -->

## Acceptance Criteria

<!-- Concrete deliverables that define the issue as complete. -->
```

- [ ] **Step 3: Verify the templates mirror the standards (direct check)**

Run:

```bash
for h in "1. Context" "2. What Was Done" "3. How to Test" "4. Evidence" "5. PR Review Checklist"; do
  grep -q "$h" .github/PULL_REQUEST_TEMPLATE.md || echo "PR template missing: $h"
done
for h in "Description" "Context" "Current Usage" "Recommended Alternative" "Acceptance Criteria"; do
  grep -q "## $h" .github/ISSUE_TEMPLATE/issue.md || echo "Issue template missing: $h"
done
```

Expected: no output (all headings present). This is the `templates_mirror_standards` acceptance criterion.

- [ ] **Step 4: Commit**

```bash
git add .github/PULL_REQUEST_TEMPLATE.md .github/ISSUE_TEMPLATE/issue.md
git commit -m "chore(github): add PR and issue templates mirroring the standards"
```

---

### Task 6: CI workflow

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `scripts/test/codex-review.test.sh` (existing), `scripts/test/setup.test.sh` (Task 1), `scripts/test/docs-consistency.test.sh` (Task 3), `scripts/test/docs-consistency.sh` (Task 4) — all invoked as `bash <path>`.
- Produces: a `ci` workflow on push and PR to `main`.

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/ci.yml` with exactly this content. Every `run` step invokes a script under `scripts/` — no check logic lives in the YAML (the `ci_calls_local_scripts_only` acceptance criterion):

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  checks:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Shell tests
        run: |
          bash scripts/test/codex-review.test.sh
          bash scripts/test/setup.test.sh
          bash scripts/test/docs-consistency.test.sh
      - name: Docs consistency
        run: bash scripts/test/docs-consistency.sh
```

- [ ] **Step 2: Verify locally that every step the workflow runs passes**

Run:

```bash
bash scripts/test/codex-review.test.sh && \
bash scripts/test/setup.test.sh && \
bash scripts/test/docs-consistency.test.sh && \
bash scripts/test/docs-consistency.sh && echo ALL_GREEN
```

Expected: final line `ALL_GREEN`. (The workflow itself is verified end-to-end by the CI run on the PR — record it as Evidence there.)

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: run shell tests and docs-consistency checks on push and pr"
```

---

### Task 7: Documentation touches

**Files:**
- Modify: `docs/standards/codex_review.md` (Activation section, after the `git config core.hooksPath .githooks` code block and its following paragraph)
- Modify: `docs/standards/INDEX.md` (System Rules list, append one bullet)

**Interfaces:**
- Consumes: `scripts/setup.sh` (Task 2), `.github/workflows/ci.yml` (Task 6), `scripts/test/docs-consistency.sh` (Task 4).
- Produces: standards that point at the new mechanisms.

- [ ] **Step 1: Point the Activation section at the bootstrap**

In `docs/standards/codex_review.md`, the Activation section currently ends with:

```markdown
This is a local setting and is not committed. Requires `codex` on `PATH` and an
authenticated session (`codex login`). Verify the toolchain with `codex doctor`.
```

Append this paragraph directly after it:

```markdown
Alternatively, run `bash scripts/setup.sh`: it applies this setting, reports the
toolchain state, and creates any missing triage labels. It is idempotent and safe
to re-run.
```

- [ ] **Step 2: Record the CI verification in the index**

In `docs/standards/INDEX.md`, append this bullet at the end of the `## System Rules` list (after the "Conflict resolution..." line):

```markdown
- Activation is bootstrapped, not assumed: `bash scripts/setup.sh` applies the local
  activation state (hooks path, triage labels) and reports the toolchain; CI
  (`.github/workflows/ci.yml`) runs the shell tests and the docs-consistency checks
  (`scripts/test/docs-consistency.sh`) on every push and pull request to `main`.
```

- [ ] **Step 3: Verify docs consistency still passes**

Run: `bash scripts/test/docs-consistency.sh`
Expected: `[docs-consistency] all checks passed.`; exit code 0.

- [ ] **Step 4: Commit**

```bash
git add docs/standards/codex_review.md docs/standards/INDEX.md
git commit -m "docs(standards): point activation at setup.sh and record ci verification"
```

---

### Task 8: Full verification and activation

**Files:**
- No new files; runs the complete suite and activates this repo.

- [ ] **Step 1: Run the whole test suite**

Run:

```bash
bash scripts/test/codex-review.test.sh && \
bash scripts/test/setup.test.sh && \
bash scripts/test/docs-consistency.test.sh && \
bash scripts/test/docs-consistency.sh && echo ALL_GREEN
```

Expected: `ALL_GREEN`.

- [ ] **Step 2: Run the bootstrap for real on this repo**

Run: `bash scripts/setup.sh`
Expected: hooksPath set (`git config core.hooksPath` prints `.githooks`), codex reported as found, the five triage labels reported as created (first run) — this is the live proof the Gap is closed, and the Evidence for the PR.

- [ ] **Step 3: Re-run to confirm idempotency on the real repo**

Run: `bash scripts/setup.sh`
Expected: exit 0; all five labels reported as `present.`
