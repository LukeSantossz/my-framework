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
