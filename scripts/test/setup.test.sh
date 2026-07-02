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
