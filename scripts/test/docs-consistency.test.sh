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
# Also includes one valid path-style reference (docs/adr/0001-real.md) to prove
# such references still pass once they resolve against the repo root.
make_fixture() {
  d="$SANDBOX/$1/docs/standards"
  adr="$SANDBOX/$1/docs/adr"
  mkdir -p "$d" "$adr"
  printf '# Index\n\n- `a.md`: sample standard.\n- See `docs/adr/0001-real.md` for the decision.\n' > "$d/INDEX.md"
  printf '# A\n\nClean content.\n' > "$d/a.md"
  printf '# Decision\n' > "$adr/0001-real.md"
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

# docs_consistency_detects_substring_orphan (hub.md unlisted must be flagged
# even though the listed github.md contains it as a substring)
root="$(make_fixture substr)"
printf -- '- `github.md`: git and GitHub standard.\n' >> "$root/docs/standards/INDEX.md"
printf '# GitHub\n' > "$root/docs/standards/github.md"
printf '# Hub\n' > "$root/docs/standards/hub.md"
out=$(ROOT_DIR="$root" bash "$CHECK" 2>&1); code=$?
if [ "$code" -ne 0 ] && printf '%s' "$out" | grep -qF "missing from INDEX.md: hub.md" \
  && ! printf '%s' "$out" | grep -qF "missing from INDEX.md: github.md"; then
  ok "docs_consistency_detects_substring_orphan"
else
  no "docs_consistency_detects_substring_orphan" "code=$code out=$out"
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

# docs_consistency_detects_missing_path_reference (path-style ref, not present)
root="$(make_fixture pathrev)"
printf -- '- See `docs/adr/0001-ghost.md` for the decision.\n' >> "$root/docs/standards/INDEX.md"
out=$(ROOT_DIR="$root" bash "$CHECK" 2>&1); code=$?
if [ "$code" -ne 0 ] && printf '%s' "$out" | grep -q "docs/adr/0001-ghost.md"; then
  ok "docs_consistency_detects_missing_path_reference"
else
  no "docs_consistency_detects_missing_path_reference" "code=$code out=$out"
fi

# skills_guidelines_covers_declared_capabilities (guard: the capability
# inventory keeps its five categories and the precedence rule)
SKILLS_DOC="$REPO_ROOT/docs/standards/skills_guidelines.md"
missing_sections=""
for h in "## Superpowers" "## Codex CLI" "## Caveman" "## Matt Pocock Engineering Skills" "## Project Agent Docs" "## Overlap Precedence"; do
  grep -qF "$h" "$SKILLS_DOC" 2>/dev/null || missing_sections="$missing_sections '$h'"
done
if [ -f "$SKILLS_DOC" ] && [ -z "$missing_sections" ]; then
  ok "skills_guidelines_covers_declared_capabilities"
else
  no "skills_guidelines_covers_declared_capabilities" "missing:$missing_sections"
fi

# codex_review_doc_depinned (guard: role-based doc — no stale Author pin,
# override variables documented)
CODEX_DOC="$REPO_ROOT/docs/standards/codex_review.md"
if ! grep -q "claude-opus-4-8" "$CODEX_DOC" \
  && grep -q "CODEX_REVIEW_MODEL" "$CODEX_DOC" \
  && grep -q "CODEX_REVIEW_EFFORT" "$CODEX_DOC"; then
  ok "codex_review_doc_depinned"
else
  no "codex_review_doc_depinned" "codex_review.md still pins models or lacks the override variables"
fi

# docs_consistency_detects_refs_in_standards_bodies (a dangling reference in
# any standard's body must fail, not only in INDEX.md)
root="$(make_fixture bodyrefs)"
printf -- 'See `phantom.md` for details.\n' >> "$root/docs/standards/a.md"
out=$(ROOT_DIR="$root" bash "$CHECK" 2>&1); code=$?
if [ "$code" -ne 0 ] && printf '%s' "$out" | grep -qF "a.md references missing file: phantom.md"; then
  ok "docs_consistency_detects_refs_in_standards_bodies"
else
  no "docs_consistency_detects_refs_in_standards_bodies" "code=$code out=$out"
fi

# docs_consistency_honors_docs_dir_override (invariant pin: DOCS_DIR points
# the check at an alternate tree)
root="$(make_fixture altdocs)"
alt="$SANDBOX/altdocs-alt/standards"
mkdir -p "$alt"
printf '# Index\n\n- `missing.md`: not there.\n' > "$alt/INDEX.md"
out=$(ROOT_DIR="$root" DOCS_DIR="$alt" bash "$CHECK" 2>&1); code=$?
if [ "$code" -ne 0 ] && printf '%s' "$out" | grep -q "missing.md"; then
  ok "docs_consistency_honors_docs_dir_override"
else
  no "docs_consistency_honors_docs_dir_override" "code=$code out=$out"
fi

# repo_scripts_are_executable (guard: every shell entry point carries the
# executable bit in the git index — the filesystem lies on Windows)
nonexec="$(cd "$REPO_ROOT" && git ls-files -s scripts .githooks | awk '$1 != "100755" {print $4}' | grep -E '\.sh$|pre-push$' || true)"
if [ -z "$nonexec" ]; then
  ok "repo_scripts_are_executable"
else
  no "repo_scripts_are_executable" "not 100755: $(printf '%s' "$nonexec" | tr '\n' ' ')"
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
