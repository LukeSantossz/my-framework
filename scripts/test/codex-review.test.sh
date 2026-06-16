#!/usr/bin/env bash
# Tests for the Codex R2 pre-push review runner (scripts/codex-review.sh).
# Each test maps to an Acceptance Criterion in SPEC.md.
set -u

TEST_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$TEST_DIR/../.." && pwd)"
RUNNER="$REPO_ROOT/scripts/codex-review.sh"
AGENTS_FILE="$REPO_ROOT/AGENTS.md"

PASS=0
FAIL=0

ok() { PASS=$((PASS + 1)); printf 'ok   - %s\n' "$1"; }
no() { FAIL=$((FAIL + 1)); printf 'FAIL - %s\n' "$1"; printf '       %s\n' "$2"; }

# A stub `codex` so tests never call the real binary. STUB_EXIT controls its code.
STUB_DIR="$(mktemp -d)"
cat > "$STUB_DIR/codex" <<'STUB'
#!/bin/sh
echo "STUB_CODEX_CALLED $*"
exit "${STUB_EXIT:-0}"
STUB
chmod +x "$STUB_DIR/codex"
trap 'rm -rf "$STUB_DIR"' EXIT

# bypass_env_skips_gate
out=$(SKIP_CODEX_REVIEW=1 CODEX_REVIEW_BRANCH=feature/x bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && ! printf '%s' "$out" | grep -q "STUB_CODEX_CALLED"; then
  ok "bypass_env_skips_gate"
else
  no "bypass_env_skips_gate" "code=$code out=$out"
fi

# skips_review_when_pushing_base_branch
out=$(CODEX_REVIEW_BRANCH=main CODEX_REVIEW_BASE=main bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && ! printf '%s' "$out" | grep -q "STUB_CODEX_CALLED"; then
  ok "skips_review_when_pushing_base_branch"
else
  no "skips_review_when_pushing_base_branch" "code=$code out=$out"
fi

# skips_review_when_codex_binary_absent
out=$(CODEX_REVIEW_BRANCH=feature/x CODEX_BIN=__no_such_codex__ bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s' "$out" | grep -qi "not installed"; then
  ok "skips_review_when_codex_binary_absent"
else
  no "skips_review_when_codex_binary_absent" "code=$code out=$out"
fi

# builds_review_command_with_pinned_model (dry-run prints the exact command)
expected='codex review --base main -c model="gpt-5.5" -c model_reasoning_effort="high"'
out=$(PATH="$STUB_DIR:$PATH" CODEX_REVIEW_BRANCH=feature/x CODEX_REVIEW_DRYRUN=1 bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s\n' "$out" | grep -qxF "$expected"; then
  ok "builds_review_command_with_pinned_model"
else
  no "builds_review_command_with_pinned_model" "code=$code out=$out"
fi

# advisory_on_codex_failure_does_not_block (design: R2 is advisory by default)
out=$(PATH="$STUB_DIR:$PATH" STUB_EXIT=1 CODEX_REVIEW_BRANCH=feature/x bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s' "$out" | grep -q "STUB_CODEX_CALLED"; then
  ok "advisory_on_codex_failure_does_not_block"
else
  no "advisory_on_codex_failure_does_not_block" "code=$code out=$out"
fi

# agents_file_points_to_standards
if [ -f "$AGENTS_FILE" ] \
  && grep -q "docs/standards/INDEX.md" "$AGENTS_FILE" \
  && grep -q "code_conventions.md" "$AGENTS_FILE"; then
  ok "agents_file_points_to_standards"
else
  no "agents_file_points_to_standards" "AGENTS.md missing required references"
fi

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
