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

# The .md reference tokens in INDEX.md, shared by Checks 2 and 3.
refs="$(grep -oE '[A-Za-z0-9_./-]+\.md' "$index" | sort -u)"

# Check 2: every standard is listed in INDEX.md (no orphan documents).
# Whole-token match: a name that is a substring of a listed reference
# (hub.md inside github.md) must still be flagged as missing.
for f in "$docs_dir"/*.md; do
  base="$(basename "$f")"
  [ "$base" = "INDEX.md" ] && continue
  base_re="$(printf '%s' "$base" | sed 's/\./\\./g')"
  if ! printf '%s\n' "$refs" | grep -qE "(^|/)${base_re}\$"; then
    log "missing from INDEX.md: $base"
    fail=1
  fi
done

# Check 3: every .md reference in INDEX.md resolves. Plain filenames (no path
# separator) are checked against the standards dir or the repo root (e.g.
# CLAUDE.md, SPEC.md). References containing a path separator (e.g.
# docs/adr/...) are checked relative to the repo root.
for ref in $refs; do
  case "$ref" in
    */*)
      if [ ! -f "$root/$ref" ]; then
        log "INDEX.md references missing file: $ref"
        fail=1
      fi
      ;;
    *)
      if [ ! -f "$docs_dir/$ref" ] && [ ! -f "$root/$ref" ]; then
        log "INDEX.md references missing file: $ref"
        fail=1
      fi
      ;;
  esac
done

[ "$fail" -eq 0 ] && log "all checks passed."
exit "$fail"
