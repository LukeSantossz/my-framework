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

# numbering_violation <dir>: prints a violation token when the NNNN-<slug>.md
# numbering in <dir> is not a contiguous run 0001..N with no gap and no
# duplicate, and prints nothing when it is clean. This is the durability
# invariant in a form that does not decay: a frozen number list would need an
# edit per new record, while contiguity holds for every record ever added and
# still fails the moment one is deleted. Only for directories that use the
# NNNN-<slug>.md convention (docs/specs/, docs/adr/) — docs/standards/ does not
# and must never be checked this way.
numbering_violation() {
  nv_dir="$1"
  nv_nums=""
  nv_count=0
  for nv_match in "$nv_dir"/*.md; do
    [ -f "$nv_match" ] || continue
    nv_base="$(basename "$nv_match")"
    case "$nv_base" in
      [0-9][0-9][0-9][0-9]-*) : ;;
      *) continue ;;
    esac
    nv_nums="$nv_nums ${nv_base%%-*}"
    nv_count=$((nv_count + 1))
  done
  if [ "$nv_count" -eq 0 ]; then
    printf 'no_numbered_files'
    return 0
  fi
  # Compared as strings, never as integers: a leading-zero number like 0008 is
  # an invalid octal literal in shell arithmetic.
  nv_expected=1
  nv_prev=""
  for nv_num in $(printf '%s\n' $nv_nums | sort); do
    if [ "$nv_num" = "$nv_prev" ]; then
      printf 'duplicate_%s' "$nv_num"
      return 0
    fi
    nv_want="$(printf '%04d' "$nv_expected")"
    if [ "$nv_num" != "$nv_want" ]; then
      printf 'expected_%s_found_%s' "$nv_want" "$nv_num"
      return 0
    fi
    nv_prev="$nv_num"
    nv_expected=$((nv_expected + 1))
  done
}

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

# docs_consistency_detects_stale_r2_only_claim (a standard recording the R3
# wiring must not also claim the document only makes R2 concrete)
root="$(make_fixture staler2)"
printf 'this document only makes R2 concrete.\n' >> "$root/docs/standards/a.md"
out=$(ROOT_DIR="$root" bash "$CHECK" 2>&1); code=$?
if [ "$code" -ne 0 ] && printf '%s' "$out" | grep -qi "deprecated"; then
  ok "docs_consistency_detects_stale_r2_only_claim"
else
  no "docs_consistency_detects_stale_r2_only_claim" "code=$code out=$out"
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

# crura_composes_with_review_layers (guard: the human-review method names the
# three machine layers and the checklist it feeds, and honors the documented
# fallback path — a layer's recorded absence, not only its results)
CRURA_DOC="$REPO_ROOT/docs/standards/crura_method.md"
if grep -q "R1" "$CRURA_DOC" && grep -q "R2" "$CRURA_DOC" \
  && grep -q "R3" "$CRURA_DOC" && grep -q "PR Review Checklist" "$CRURA_DOC" \
  && grep -q "recorded absence" "$CRURA_DOC"; then
  ok "crura_composes_with_review_layers"
else
  no "crura_composes_with_review_layers" "crura_method.md does not reference the review layers or the checklist"
fi

# codex_review_doc_depinned (guard: role-based doc — no concrete Anthropic model
# id anywhere, so no Author pin can go stale; override variables documented)
# The id shape is anchored on the family names plus the legacy claude-<digit>
# ids. A bare claude-<word>-<digit> is wrong in both directions: it misses the
# whole claude-3 family (a digit follows claude-, not a word) and it rejects
# legitimate prose such as claude-code-2. Case-insensitive; `claude-` with the
# hyphen never matches the CLAUDE.md reference or the "Claude family" prose.
ANTHROPIC_MODEL_RE='claude-(opus|sonnet|haiku|fable|instant|[0-9])'
# Proven on fixtures first: grepping only the real doc (which pins no id) passes
# whether or not the pattern works, so a typo would sail through green.
depin_missing=""
for depin_pos in claude-opus-4-8 claude-fable-5 claude-sonnet-4-6 claude-haiku-4-5 \
  claude-3-opus-20240229 claude-3-5-sonnet-20241022 claude-2.1 Claude-Opus-4-8; do
  printf '%s\n' "$depin_pos" | grep -Eqi "$ANTHROPIC_MODEL_RE" \
    || depin_missing="$depin_missing missed:$depin_pos"
done
for depin_neg in claude-code-2 claude-agent-sdk-2 claude-plugins-official CLAUDE.md; do
  printf '%s\n' "$depin_neg" | grep -Eqi "$ANTHROPIC_MODEL_RE" \
    && depin_missing="$depin_missing false_positive:$depin_neg"
done
CODEX_DOC="$REPO_ROOT/docs/standards/codex_review.md"
grep -Eqi "$ANTHROPIC_MODEL_RE" "$CODEX_DOC" && depin_missing="$depin_missing doc_pins_a_model"
grep -q "CODEX_REVIEW_MODEL" "$CODEX_DOC" || depin_missing="$depin_missing no_model_override_documented"
grep -q "CODEX_REVIEW_EFFORT" "$CODEX_DOC" || depin_missing="$depin_missing no_effort_override_documented"
if [ -z "$depin_missing" ]; then
  ok "codex_review_doc_depinned"
else
  no "codex_review_doc_depinned" "missing:$depin_missing"
fi

# standards_authority_and_ambiguity_recorded (guard: repo-over-global rule in
# code_conventions.md and INDEX.md; hybrid ambiguity policy in ai_guidelines.md)
CONV_DOC="$REPO_ROOT/docs/standards/code_conventions.md"
AI_DOC="$REPO_ROOT/docs/standards/ai_guidelines.md"
INDEX_DOC="$REPO_ROOT/docs/standards/INDEX.md"
authority_missing=""
grep -q "override user-global defaults" "$CONV_DOC" || authority_missing="$authority_missing code_conventions"
grep -q "override user-global defaults" "$INDEX_DOC" || authority_missing="$authority_missing INDEX"
grep -q "Safety and Correctness are never overridden" "$CONV_DOC" || authority_missing="$authority_missing code_conventions"
grep -q "Safety and Correctness are never overridden" "$INDEX_DOC" || authority_missing="$authority_missing INDEX"
grep -q "one focused question" "$AI_DOC" || authority_missing="$authority_missing ai_guidelines"
if [ -z "$authority_missing" ]; then
  ok "standards_authority_and_ambiguity_recorded"
else
  no "standards_authority_and_ambiguity_recorded" "missing wording in:$authority_missing"
fi

# durable_spec_archive_recorded (guard: specs are authored durably under
# docs/specs/ with a documented spec-lite tier, the root SPEC.md is retired,
# ADR 0002 amends ADR 0001, and CONTEXT.md drops the overwritten-each-change
# wording)
SPEC_METHOD_DOC="$REPO_ROOT/docs/standards/spec_method.md"
ADR_0001_DOC="$REPO_ROOT/docs/adr/0001-decision-records-flow.md"
ADR_0002_DOC="$REPO_ROOT/docs/adr/0002-durable-spec-archive.md"
CONTEXT_DOC="$REPO_ROOT/CONTEXT.md"
durable_spec_missing=""
grep -q "docs/specs/" "$SPEC_METHOD_DOC" || durable_spec_missing="$durable_spec_missing spec_method_docs_specs"
grep -q "spec-lite" "$SPEC_METHOD_DOC" || durable_spec_missing="$durable_spec_missing spec_method_spec_lite"
# The rule the contiguity check below enforces must be stated in the standard, or
# the guard polices something no document requires. Pinned by exact clause so a
# reworded or dropped rule is caught rather than passing on a stray keyword.
grep -qF 'never reused' "$SPEC_METHOD_DOC" || durable_spec_missing="$durable_spec_missing spec_method_number_never_reused"
grep -qF 'marked Retired' "$SPEC_METHOD_DOC" || durable_spec_missing="$durable_spec_missing spec_method_retired_in_place"
# Every spec present under docs/specs/ must carry the "# SPEC:" header. Globbed
# (not a frozen number list) so specs added after this guard are covered too;
# the archive_pins block below stays the byte-exact integrity check for the
# backfilled 0001-0006.
spec_count=0
for match in "$REPO_ROOT"/docs/specs/*.md; do
  [ -f "$match" ] || continue
  spec_count=$((spec_count + 1))
  if ! head -1 "$match" | grep -q "^# SPEC:"; then
    durable_spec_missing="$durable_spec_missing specs_$(basename "$match")_header"
  fi
done
[ "$spec_count" -gt 0 ] || durable_spec_missing="$durable_spec_missing specs_none_found"
# The header check above only inspects whatever files happen to be present, so a
# deleted spec passes it silently. Contiguity is what makes a deletion fail:
# the numbers must run 0001..N with no gap and no duplicate.
durable_numbering_violation="$(numbering_violation "$REPO_ROOT/docs/specs")"
[ -z "$durable_numbering_violation" ] \
  || durable_spec_missing="$durable_spec_missing specs_numbering_$durable_numbering_violation"
# The backfilled archive is verbatim history: each committed blob must equal
# the blob at its pinned extraction commit (blob-to-blob, immune to eol
# conversion; needs full history — CI fetches with fetch-depth: 0).
archive_pins="0001-add-codex-pre-push-gate.md=1d1742a^
0002-add-domain-glossary.md=326bf49^
0003-close-the-activation-gap.md=4f03e9e^
0004-make-the-toolchain-portable.md=6a0680f^
0005-harden-scripts-and-checks.md=7e62d08^
0006-align-global-and-repo-standards.md=619ea7d"
for pin in $archive_pins; do
  pin_file="${pin%%=*}"
  pin_src="${pin##*=}"
  have_blob="$(cd "$REPO_ROOT" && git rev-parse "HEAD:docs/specs/$pin_file" 2>/dev/null)"
  want_blob="$(cd "$REPO_ROOT" && git rev-parse "$pin_src:SPEC.md" 2>/dev/null)"
  if [ -z "$have_blob" ] || [ -z "$want_blob" ] || [ "$have_blob" != "$want_blob" ]; then
    durable_spec_missing="$durable_spec_missing archive_blob_$pin_file"
  fi
done
[ ! -f "$REPO_ROOT/SPEC.md" ] || durable_spec_missing="$durable_spec_missing root_spec_present"
[ -f "$ADR_0002_DOC" ] || durable_spec_missing="$durable_spec_missing adr_0002_missing"
grep -q "0002" "$ADR_0001_DOC" || durable_spec_missing="$durable_spec_missing adr_0001_not_amended"
grep -q "overwritten by the next change" "$CONTEXT_DOC" && durable_spec_missing="$durable_spec_missing context_still_transient"
grep -q "stays transient" "$REPO_ROOT/docs/standards/INDEX.md" && durable_spec_missing="$durable_spec_missing index_still_transient"
grep -q "transient, before" "$CONTEXT_DOC" && durable_spec_missing="$durable_spec_missing context_intro_still_transient"
if [ -z "$durable_spec_missing" ]; then
  ok "durable_spec_archive_recorded"
else
  no "durable_spec_archive_recorded" "missing:$durable_spec_missing"
fi

# spec_numbering_is_contiguous (guard: the durable spec numbers run 0001..N with
# no gap and no duplicate, so deleting an archived spec cannot pass silently.
# Proven on throwaway fixtures first, so a toothless helper cannot pass this by
# reporting every tree clean.)
spec_numbering_missing=""
spec_gap_dir="$SANDBOX/numbering-specs-gap"
mkdir -p "$spec_gap_dir"
for n in 0001 0002 0004; do
  printf '# SPEC: sample\n' > "$spec_gap_dir/$n-sample.md"
done
[ -n "$(numbering_violation "$spec_gap_dir")" ] \
  || spec_numbering_missing="$spec_numbering_missing gap_fixture_not_flagged"
spec_dup_dir="$SANDBOX/numbering-specs-dup"
mkdir -p "$spec_dup_dir"
printf '# SPEC: sample\n' > "$spec_dup_dir/0001-sample.md"
printf '# SPEC: sample\n' > "$spec_dup_dir/0002-sample.md"
printf '# SPEC: sample\n' > "$spec_dup_dir/0002-other.md"
[ -n "$(numbering_violation "$spec_dup_dir")" ] \
  || spec_numbering_missing="$spec_numbering_missing duplicate_fixture_not_flagged"
spec_real_violation="$(numbering_violation "$REPO_ROOT/docs/specs")"
[ -z "$spec_real_violation" ] \
  || spec_numbering_missing="$spec_numbering_missing real_tree:$spec_real_violation"
if [ -z "$spec_numbering_missing" ]; then
  ok "spec_numbering_is_contiguous"
else
  no "spec_numbering_is_contiguous" "missing:$spec_numbering_missing"
fi

# adr_numbering_is_contiguous (guard: the same durability invariant over the ADR
# archive, which shares the NNNN-<slug>.md convention. docs/standards/ does not
# use that convention and is deliberately never checked this way.)
adr_numbering_missing=""
adr_gap_dir="$SANDBOX/numbering-adr-gap"
mkdir -p "$adr_gap_dir"
for n in 0001 0002 0004; do
  printf '# Decision\n' > "$adr_gap_dir/$n-sample.md"
done
[ -n "$(numbering_violation "$adr_gap_dir")" ] \
  || adr_numbering_missing="$adr_numbering_missing gap_fixture_not_flagged"
adr_dup_dir="$SANDBOX/numbering-adr-dup"
mkdir -p "$adr_dup_dir"
printf '# Decision\n' > "$adr_dup_dir/0001-sample.md"
printf '# Decision\n' > "$adr_dup_dir/0002-sample.md"
printf '# Decision\n' > "$adr_dup_dir/0002-other.md"
[ -n "$(numbering_violation "$adr_dup_dir")" ] \
  || adr_numbering_missing="$adr_numbering_missing duplicate_fixture_not_flagged"
adr_real_violation="$(numbering_violation "$REPO_ROOT/docs/adr")"
[ -z "$adr_real_violation" ] \
  || adr_numbering_missing="$adr_numbering_missing real_tree:$adr_real_violation"
if [ -z "$adr_numbering_missing" ]; then
  ok "adr_numbering_is_contiguous"
else
  no "adr_numbering_is_contiguous" "missing:$adr_numbering_missing"
fi

# durable_records_are_never_deleted (guard: a spec or ADR, once committed, is
# never removed. Contiguity cannot see this on its own: deleting the
# highest-numbered record leaves 0001..N-1 contiguous and clean, which is the
# exact shape of the PR #10 incident — spec 0009 and ADR 0003 were each the
# highest of their series. So the archive is checked against git history, not
# only against itself: every NNNN-<slug>.md ever added under docs/specs/ or
# docs/adr/ must still be present.
# The list below is CLOSED and records only the two deletions that happened
# before the rule existed (PR #10) and cannot be undone:
# - docs/specs/0009-switch-r2-reviewer-to-gpt-5-6-terra.md: the benchmark spec
#   deleted by PR #10 as disproportionate to the decision it carried; that
#   decision survives in docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md.
# - docs/adr/0003-r2-reviewer-model-gpt-5-6-terra.md: deleted by PR #10 and
#   restored at docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md.
# It is NOT the retirement mechanism. Under `spec_method.md` a retired record
# stays in place marked Retired, keeping its number and its file, so retiring a
# record never requires an entry here. Treating a missing path as a valid
# retirement would reopen the PR #10 failure mode (R2 finding, PR #12), so the
# closed_pre_rule_deletions assertion below pins this list to exactly those two
# paths: adding a third fails the suite rather than waving a deletion through.
# Needs full history; CI checks out with fetch-depth: 0.
PRE_RULE_DELETIONS='docs/specs/0009-switch-r2-reviewer-to-gpt-5-6-terra.md
docs/adr/0003-r2-reviewer-model-gpt-5-6-terra.md'
deleted_records=""
ever_added="$(cd "$REPO_ROOT" && git log --diff-filter=A --name-only --format= -- docs/specs docs/adr 2>/dev/null \
  | grep -E '^docs/(specs|adr)/[0-9]{4}-[^/]*\.md$' | sort -u)"
if [ -z "$ever_added" ]; then
  deleted_records=" history_unavailable"
else
  for rec in $ever_added; do
    [ -f "$REPO_ROOT/$rec" ] && continue
    case "
$PRE_RULE_DELETIONS
" in
      *"
$rec
"*) continue ;;
    esac
    deleted_records="$deleted_records $rec"
  done
fi
if [ -z "$deleted_records" ]; then
  ok "durable_records_are_never_deleted"
else
  no "durable_records_are_never_deleted" "deleted with no record left in place:$deleted_records"
fi

# closed_pre_rule_deletions (guard on the guard: the pre-rule deletion list must
# stay exactly the two PR #10 paths. Without this, retiring a record could be
# done by deleting the file and appending its path above — which both the
# history check and the contiguity check would then pass, recreating the very
# failure mode the rule forbids. R2 finding on PR #12, accepted.)
expected_pre_rule='docs/adr/0003-r2-reviewer-model-gpt-5-6-terra.md
docs/specs/0009-switch-r2-reviewer-to-gpt-5-6-terra.md'
actual_pre_rule="$(printf '%s\n' "$PRE_RULE_DELETIONS" | sed '/^$/d' | sort)"
if [ "$actual_pre_rule" = "$expected_pre_rule" ]; then
  ok "closed_pre_rule_deletions"
else
  no "closed_pre_rule_deletions" "the pre-rule deletion list changed; a retired record stays in place marked Retired, it is not appended here:
$actual_pre_rule"
fi

# no_attribution_in_branch_commits (guard: no commit a branch adds over main
# carries a co-author or AI-attribution line. The rule is stated four times —
# CLAUDE.md, docs/standards/github.md, docs/standards/ai_guidelines.md and
# AGENTS.md — and was the only rule in the repo with violations already merged:
# 859daf2 and 757695d both carry Co-Authored-By trailers because nothing checked
# them. Repetition is not activation; this is the check those four restatements
# never bought.
#
# Scoped to <base>..HEAD, which excludes those two BY CONSTRUCTION: both are
# ancestors of main, so they are never among the commits a branch adds and can
# never redden this guard. That is deliberate — they are published, a force-push
# over shared history costs more than the defect, and a permanently red check is
# a disabled check. They are recorded in README Known Issues & Limitations
# instead.
#
# The base resolves as main, then origin/main, because a pull_request checkout
# (actions/checkout@v4, fetch-depth: 0) fetches +refs/heads/*:refs/remotes/origin/*
# and lands on the PR merge ref DETACHED: refs/remotes/origin/main exists but no
# local main branch does, so a bare `main` does not resolve there. Without the
# fallback this guard would find no base and pass vacuously in CI — dead in the
# one place the rule must hold.
#
# Limitation: when neither base resolves, or the range is empty (running on main
# itself), the branch adds nothing and the guard passes rather than erroring. A
# direct push to main is therefore unguarded. Accepted: the repo works through
# PRs, and CI also runs on pull_request, where the range is non-empty.
# Anchored to the shapes attribution actually takes — a git trailer at the start
# of a line, or a tool's credit footer — rather than to loose substrings. An
# unanchored `generated with` was wrong in both directions (R2 finding, PR #13):
# it rejected ordinary prose such as "docs: explain reports generated with the
# CLI", and it missed "Generated by Codex", which is the same forbidden credit
# with a different preposition. The trailer half bans any co-author trailer
# whoever the co-author is; the credit half names the tools that inject one.
# Both halves are anchored to the start of their own line, because attribution is
# a standalone trailer or footer — never a clause inside a sentence. A second R2
# round caught that anchoring only the trailer half left the credit half loose,
# so "docs: explain output generated by Codex-compatible tooling" was rejected as
# attribution.
#
# The optional prefix is the literal robot emoji rather than a negated class: in
# a UTF-8 locale grep does not match a 4-byte emoji against [^[:alnum:]], so the
# obvious spelling silently failed to match the very footer it targets. A
# permissive `.{0,4}` would match instead, but would also reject prose opening
# with any short word. Naming the one prefix tools actually inject keeps the
# anchor exact.
#
# The credit allows up to two words before the tool name ("generated by GitHub
# Copilot"), bounded so it cannot reach across a sentence.
ATTRIBUTION_RE='^[[:space:]]*(co-authored-by|claude-session|x-generated-by):|^[[:space:]]*(🤖[[:space:]]*)?generated[[:space:]]+(with|by)[[:space:]]+(an[[:space:]]+agent|([^[:space:]]+[[:space:]]+){0,2}[^[:space:]]*(claude|codex|copilot|gpt|gemini|cursor))'
# Proven on fixtures first: on a clean branch this guard passes whether or not
# the pattern works, so a typo would sail through green forever — the same hazard
# codex_review_doc_depinned guards against above.
attribution_probe=""
for att_pos in "Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>" \
  "co-authored-by: someone <s@example.com>" \
  "🤖 Generated with [Claude Code](https://claude.com/claude-code)" \
  "Generated with an agent" \
  "Generated by Codex" \
  "generated by GitHub Copilot" \
  "Claude-Session: https://claude.ai/code/session_01AW1"; do
  printf '%s\n' "$att_pos" | grep -Eqi "$ATTRIBUTION_RE" \
    || attribution_probe="$attribution_probe missed:$att_pos"
done
for att_neg in "fix(scripts): read reviewer config from local scope only" \
  "docs(standards): record the number-reuse rule in the index" \
  "docs: explain reports generated with the CLI" \
  "docs: explain output generated by Codex-compatible tooling" \
  "chore: import the co-authored-by parser from upstream" \
  "test(scripts): pin the reviewer session default"; do
  printf '%s\n' "$att_neg" | grep -Eqi "$ATTRIBUTION_RE" \
    && attribution_probe="$attribution_probe false_positive:$att_neg"
done
# attributed_commits_in <repo>: prints the commits <repo>'s HEAD adds over its
# base that carry an attribution line, one per line, and nothing when clean.
# Extracted so the range traversal itself is testable against a fixture
# repository (R2 finding, PR #13): probing the pattern against isolated strings
# left base selection, range handling and per-commit lookup unexercised, so a
# regression in any of them would have passed green on a clean branch — the same
# vacuity this file guards against everywhere else.
# attribution_base_in <repo>: prints the base the range is taken against, or
# nothing when none resolves. origin/main is preferred over a local main: a stale
# local main makes main..HEAD reach back over commits already published, which
# would re-flag the very historical violations this guard exists to exclude
# (R3 finding, PR #13).
attribution_base_in() {
  ab_repo="$1"
  for ab_cand in origin/main main; do
    if (cd "$ab_repo" && git rev-parse --verify --quiet "$ab_cand^{commit}") >/dev/null 2>&1; then
      printf '%s' "$ab_cand"
      return 0
    fi
  done
}
attributed_commits_in() {
  ac_repo="$1"
  ac_base="$(attribution_base_in "$ac_repo")"
  [ -n "$ac_base" ] || return 0
  # Per commit, not one blob grep: the failure has to name the offending commit,
  # or it tells the Author that a rule broke without saying where.
  for ac_sha in $(cd "$ac_repo" && git log "$ac_base..HEAD" --format=%H 2>/dev/null); do
    ac_body="$(cd "$ac_repo" && git log -1 --format=%B "$ac_sha" 2>/dev/null)"
    if printf '%s' "$ac_body" | grep -Eqi "$ATTRIBUTION_RE"; then
      printf '       %s\n' "$(cd "$ac_repo" && git log -1 --format='%h %s' "$ac_sha" 2>/dev/null)"
    fi
  done
}
# Fixture repositories prove the traversal, not only the pattern: one whose
# feature branch adds an attributed commit must be flagged, one whose feature
# branch is clean must not, and an attributed commit already on main must be
# ignored — the exact exclusion that keeps 859daf2 and 757695d from reddening CI.
att_fixture() {
  af_dir="$SANDBOX/attr-$1"
  mkdir -p "$af_dir"
  (cd "$af_dir" && git init -q -b main . \
    && git -c user.email=t@example.com -c user.name=T commit -q --allow-empty -m "$2" -m "$3" \
    && git checkout -q -b feature \
    && git -c user.email=t@example.com -c user.name=T commit -q --allow-empty -m "$4" -m "$5")
  printf '%s\n' "$af_dir"
}
att_dirty="$(att_fixture dirty "chore: base" "" "feat: thing" "Co-Authored-By: X <x@example.com>")"
[ -n "$(attributed_commits_in "$att_dirty")" ] \
  || attribution_probe="$attribution_probe branch_violation_not_flagged"
att_clean="$(att_fixture clean "chore: base" "" "feat: thing" "a normal body")"
[ -z "$(attributed_commits_in "$att_clean")" ] \
  && : || attribution_probe="$attribution_probe clean_branch_flagged"
att_hist="$(att_fixture hist "chore: base" "Co-Authored-By: X <x@example.com>" "feat: thing" "a normal body")"
[ -z "$(attributed_commits_in "$att_hist")" ] \
  && : || attribution_probe="$attribution_probe merged_history_not_ignored"
# The failing branch below reports the base, so it must be resolved here too —
# reading a variable the extraction left behind inside the function crashed the
# suite under `set -u` at the exact moment it found a violation, and only the
# clean path had been re-tested after the refactor (R3 finding, PR #13).
attribution_base="$(attribution_base_in "$REPO_ROOT")"
attributed_commits="$(attributed_commits_in "$REPO_ROOT")"
if [ -n "$attribution_probe" ]; then
  no "no_attribution_in_branch_commits" "the attribution pattern is broken:$attribution_probe"
elif [ -n "$attributed_commits" ]; then
  no "no_attribution_in_branch_commits" "commits added over $attribution_base carry a co-author or AI-attribution line, forbidden by CLAUDE.md, github.md, ai_guidelines.md and AGENTS.md:$attributed_commits"
else
  ok "no_attribution_in_branch_commits"
fi

# token_economy_claims_match_reality (guard: the token-economy documents describe
# the capability that exists rather than the one the standard wished for. The
# installed Caveman skill is 49 lines of conversational terse mode: it contains no
# `caveman-compress`, never mentions `CLAUDE.md`, and cannot rewrite a context
# file. Three claims were false at once — `CLAUDE.md` said it was kept in its
# caveman-compress form while being full prose with articles and complete
# sentences, exactly what a compression would strip; Policy §1 called that
# compression a default that is always on; and `skills_guidelines.md` reported the
# skill as not installed at the very path where it is installed, stale in the
# inverted direction. The framework's only always-on policy described a capability
# that does not exist.
#
# The claims are pinned in both directions, because a guard that only asserted the
# false wording was gone would pass on a document that says nothing at all. The
# absence checks cover every file that carried the claim or could inherit it; the
# presence check pins the Developer's decision of 2026-07-15 — the token economy is
# an opt-in the adopter chooses at initialization, not a default the framework
# imposes — by exact clause, so a reworded always-on default cannot satisfy it on a
# stray keyword.
#
# The percentages are guarded as a bare `[0-9]+%` rather than by literal, so no new
# unreproducible number can be written where the old ones stood. `ai_guidelines.md`
# requires every reported number to carry the means to reproduce it; the dropped
# figures carried no command, no versions and no citation, which is No Fabricated
# Evidence, and the fix is to drop them rather than dress them up.
#
# What this guard deliberately does NOT do is grep `~/.claude/skills/` to prove the
# skill's contents: that pins the suite to one machine's home directory and fails in
# CI, where no skills are installed. The inventory documents an external
# environment, it does not manage it. What is checkable from inside the repo is that
# the documents agree with each other and assert nothing unreproducible.
TOKEN_DOC="$REPO_ROOT/docs/standards/token_economy.md"
TOKEN_SKILLS_DOC="$REPO_ROOT/docs/standards/skills_guidelines.md"
TOKEN_CLAUDE_DOC="$REPO_ROOT/CLAUDE.md"
TOKEN_CONTEXT_DOC="$REPO_ROOT/CONTEXT.md"
TOKEN_INDEX_DOC="$REPO_ROOT/docs/standards/INDEX.md"
token_economy_missing=""
# The claim is caught by meaning, not by one phrasing: it first survived here as
# INDEX.md's "CLAUDE.md is kept compressed", which said the same false thing in
# different words while a literal-scoped check passed green — the same failure
# codex_review_doc_depinned had.
for te_file in "$TOKEN_CLAUDE_DOC" "$TOKEN_DOC" "$TOKEN_CONTEXT_DOC" "$TOKEN_INDEX_DOC"; do
  grep -qEi 'caveman-compress form|is kept compressed|kept in (its )?compressed' "$te_file" 2>/dev/null \
    && token_economy_missing="$token_economy_missing compressed_claim_in:$(basename "$te_file")"
done
# No percentage at all, deliberately absolute (R2 finding, PR #13, adjudicated).
# A policy document is not a results document: a measured figure belongs in a
# spec's Reproducibility section or an ADR, which carry the command and versions
# ai_guidelines.md demands — docs/adr/0004 does exactly that for its benchmark.
# The alternative, inferring from nearby prose whether a number "has a
# reproduction path", is a heuristic that would pass the dressed-up figures this
# closes: the ones removed here already cited "author benchmarks and third-party
# tests" and would have satisfied any such check.
grep -qE '[0-9]+%' "$TOKEN_DOC" 2>/dev/null \
  && token_economy_missing="$token_economy_missing measurement_in_a_policy_doc"
grep -qF 'opt-in the adopter chooses when initializing the framework in a project' "$TOKEN_DOC" 2>/dev/null \
  || token_economy_missing="$token_economy_missing policy_not_stated_as_opt_in"
grep -qF 'Not currently installed' "$TOKEN_SKILLS_DOC" 2>/dev/null \
  && token_economy_missing="$token_economy_missing skills_claim_not_installed"
if [ -z "$token_economy_missing" ]; then
  ok "token_economy_claims_match_reality"
else
  no "token_economy_claims_match_reality" "documents contradict the installed Caveman skill:$token_economy_missing"
fi

# claude_md_points_to_standards (guard: the Author's entry point still activates
# the standards. AGENTS.md — the Reviewer's door — has had this guard since the
# R2 gate landed (agents_file_points_to_standards in codex-review.test.sh);
# CLAUDE.md, the door the Author reads on every session and the one
# token_economy.md permits rewriting, had none. That asymmetry is the Gap this
# framework exists to close, left open at its own front door: an edit that stops
# CLAUDE.md pointing at INDEX.md silently unloads every standard, and nothing
# said so. token_economy.md requires a compression to preserve the standards
# paths and the precedence reference byte-for-byte and declares that a
# compression breaking activation is rejected — this is what rejects it.)
CLAUDE_DOC="$REPO_ROOT/CLAUDE.md"
claude_md_missing=""
[ -f "$CLAUDE_DOC" ] || claude_md_missing="$claude_md_missing file_absent"
grep -qF "docs/standards/INDEX.md" "$CLAUDE_DOC" 2>/dev/null \
  || claude_md_missing="$claude_md_missing no_index_reference"
grep -qF "code_conventions.md" "$CLAUDE_DOC" 2>/dev/null \
  || claude_md_missing="$claude_md_missing no_precedence_reference"
if [ -z "$claude_md_missing" ]; then
  ok "claude_md_points_to_standards"
else
  no "claude_md_points_to_standards" "CLAUDE.md no longer activates the standards:$claude_md_missing"
fi

# skills_inventory_names_installed_skills (guard: the capability inventory names
# skills that exist. It named `grilling` and `diagnosing-bugs`, which match
# nothing installed — the real skills are `grill-me` and `diagnose` — and
# `domain-modeling`, `codebase-design` and `design-an-interface`, which match
# nothing anywhere. An inventory whose entries cannot be invoked is worse than no
# inventory: it sends the Author to a skill list that has no such skill, which
# reads as a broken toolchain rather than as the documentation defect it is, and
# the declared fallback never fires because nothing reports the skill as absent.
#
# Names are matched backtick-delimited, exactly as the document writes them,
# because the wrong names are not separable from the right ones by substring:
# `grilling` and `grill-me` share a prefix, and a bare `diagnose` would also
# match prose such as "diagnosed". The backticks are the word boundary the
# document already supplies.
#
# This guard deliberately does NOT grep `~/.claude/skills/` to prove the names
# resolve: that pins the suite to one machine's home directory and fails in CI,
# where nothing is installed — the same boundary token_economy_claims_match_reality
# holds. The inventory documents an external environment, it does not manage it,
# so what is checkable from inside the repo is that it names the set the record
# says exists. It will need updating when a skill is renamed upstream; a wrong
# name is worse than a name that may age.
SKILLS_INV_DOC="$REPO_ROOT/docs/standards/skills_guidelines.md"
skills_inventory_missing=""
for inv_present in '`grill-me`' '`diagnose`'; do
  grep -qF "$inv_present" "$SKILLS_INV_DOC" 2>/dev/null \
    || skills_inventory_missing="$skills_inventory_missing missing:$inv_present"
done
for inv_absent in '`grilling`' '`diagnosing-bugs`' '`domain-modeling`' '`codebase-design`' '`design-an-interface`'; do
  grep -qF "$inv_absent" "$SKILLS_INV_DOC" 2>/dev/null \
    && skills_inventory_missing="$skills_inventory_missing names_uninstalled:$inv_absent"
done
if [ -z "$skills_inventory_missing" ]; then
  ok "skills_inventory_names_installed_skills"
else
  no "skills_inventory_names_installed_skills" "skills_guidelines.md names skills that are not installed:$skills_inventory_missing"
fi

# superpowers_claim_is_not_enforcement (guard: "enforced by" is a claim about an
# external tool, and Superpowers is not installed — `~/.claude/plugins/` holds no
# such plugin — so nothing enforced the test-first order in the three documents
# that said it did. A rule described as enforced by a tool that is absent is the
# Gap in miniature: the Author reads that the phase has it covered and stops
# checking, and no phase runs. The true verb is that the TDD phase runs the order
# when it is installed, while the order itself is project policy and binds either
# way — which is why each document already records it independently of the tool.
#
# Pinned in both directions. An absence-only check passes on a document that
# dropped the test-first rule altogether, which would close the false claim by
# deleting the true one; the presence checks hold each document to still stating
# the rule, by exact clause so a reworded weakening cannot pass on a keyword.
SP_INDEX_DOC="$REPO_ROOT/docs/standards/INDEX.md"
SP_CONV_DOC="$REPO_ROOT/docs/standards/code_conventions.md"
SP_AI_DOC="$REPO_ROOT/docs/standards/ai_guidelines.md"
superpowers_claim_missing=""
for sp_file in "$SP_INDEX_DOC" "$SP_CONV_DOC" "$SP_AI_DOC"; do
  grep -qF 'enforced by the Superpowers' "$sp_file" 2>/dev/null \
    && superpowers_claim_missing="$superpowers_claim_missing claims_enforcement_in:$(basename "$sp_file")"
done
grep -qF 'Test-first order (red-green-refactor) is project policy' "$SP_INDEX_DOC" 2>/dev/null \
  || superpowers_claim_missing="$superpowers_claim_missing rule_dropped_from:INDEX.md"
grep -qF 'Write the test before the implementation' "$SP_CONV_DOC" 2>/dev/null \
  || superpowers_claim_missing="$superpowers_claim_missing rule_dropped_from:code_conventions.md"
grep -qF 'Write the test before the implementation' "$SP_AI_DOC" 2>/dev/null \
  || superpowers_claim_missing="$superpowers_claim_missing rule_dropped_from:ai_guidelines.md"
if [ -z "$superpowers_claim_missing" ]; then
  ok "superpowers_claim_is_not_enforcement"
else
  no "superpowers_claim_is_not_enforcement" "the test-first order is claimed enforced by a tool that is not installed, or the rule itself was dropped:$superpowers_claim_missing"
fi

# english_rule_scoped_consistently (guard: one rule, one scope. `INDEX.md`,
# `code_conventions.md` and `AGENTS.md` scope the English rule to the artifacts —
# identifiers, comments, commit/PR/issue text, documentation — while `CLAUDE.md`
# and `ai_guidelines.md` stated it unscoped, as a bare "All output in English."
# Read literally the unscoped form is the wider rule and forbids answering the
# Developer in Portuguese, which is neither intended nor practiced: the scope is
# what the framework versions, not the conversation about it.
#
# This could not be adjudicated at read time. The precedence order in
# `code_conventions.md` ranks rule TYPES (Safety, Correctness, idiom, existing
# pattern, suffixes) and repo-over-global sources; it has no rule for two repo
# documents stating one rule at two scopes, so it cannot arbitrate them and the
# conflict has to be removed at the source. The scoped form is the intended one.
#
# Pinned in both directions, and the discriminating pattern is proven on fixtures
# first: the absence check is a regex whose whole job is to separate the scoped
# form from the unscoped one, and a pattern that matched neither — or both —
# would pass green while enforcing nothing. The period is the discriminator: it
# is what makes the sentence end at "English" instead of continuing into its
# scope.
ENGLISH_UNSCOPED_RE='All output in English\.'
ENGLISH_SCOPED='All output in English: identifiers, comments, commit/PR/issue text, documentation.'
english_probe=""
for en_pos in "All output in English." "- All output in English." \
  "Counterpart to \`crura_method.md\`. All output in English."; do
  printf '%s\n' "$en_pos" | grep -Eq "$ENGLISH_UNSCOPED_RE" \
    || english_probe="$english_probe missed:$en_pos"
done
for en_neg in "$ENGLISH_SCOPED" \
  "- All output in English: identifiers, comments, commit/PR/issue text, documentation." \
  "All output in English (identifiers, comments, commit/PR/issue text, documentation)."; do
  printf '%s\n' "$en_neg" | grep -Eq "$ENGLISH_UNSCOPED_RE" \
    && english_probe="$english_probe false_positive:$en_neg"
done
english_rule_missing=""
for en_file in "$REPO_ROOT/CLAUDE.md" "$REPO_ROOT/docs/standards/ai_guidelines.md"; do
  grep -qE "$ENGLISH_UNSCOPED_RE" "$en_file" 2>/dev/null \
    && english_rule_missing="$english_rule_missing unscoped_in:$(basename "$en_file")"
  grep -qF "$ENGLISH_SCOPED" "$en_file" 2>/dev/null \
    || english_rule_missing="$english_rule_missing scope_missing_in:$(basename "$en_file")"
done
if [ -n "$english_probe" ]; then
  no "english_rule_scoped_consistently" "the unscoped-English pattern is broken:$english_probe"
elif [ -z "$english_rule_missing" ]; then
  ok "english_rule_scoped_consistently"
else
  no "english_rule_scoped_consistently" "the English rule is stated at a scope INDEX.md does not state it at:$english_rule_missing"
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

# issue_model_generalized_and_template_aligned (guard: the Issue Model is
# type-agnostic — conditional Current State, Proposed Solution — mirrored in
# the issue template with no literal title pre-fill, and the What It Is
# classification reads as illustrative, not exhaustive)
GITHUB_DOC_ISSUE="$REPO_ROOT/docs/standards/github.md"
ISSUE_TEMPLATE="$REPO_ROOT/.github/ISSUE_TEMPLATE/issue.md"
issue_model_missing=""
grep -qF "### Current State" "$GITHUB_DOC_ISSUE" || issue_model_missing="$issue_model_missing github:Current-State-header"
grep -qF "### Proposed Solution" "$GITHUB_DOC_ISSUE" || issue_model_missing="$issue_model_missing github:Proposed-Solution-header"
grep -qF "Current Usage" "$GITHUB_DOC_ISSUE" && issue_model_missing="$issue_model_missing github:stale-Current-Usage"
grep -qF "Recommended Alternative" "$GITHUB_DOC_ISSUE" && issue_model_missing="$issue_model_missing github:stale-Recommended-Alternative"
grep -qF "## Current State" "$ISSUE_TEMPLATE" || issue_model_missing="$issue_model_missing template:Current-State-header"
grep -qF "## Proposed Solution" "$ISSUE_TEMPLATE" || issue_model_missing="$issue_model_missing template:Proposed-Solution-header"
grep -qF "Current Usage" "$ISSUE_TEMPLATE" && issue_model_missing="$issue_model_missing template:stale-Current-Usage"
grep -qF "Recommended Alternative" "$ISSUE_TEMPLATE" && issue_model_missing="$issue_model_missing template:stale-Recommended-Alternative"
grep -q '^title:' "$ISSUE_TEMPLATE" && issue_model_missing="$issue_model_missing template:title-prefill-present"
what_it_is_section="$(awk '/^### What It Is$/{flag=1; next} /^### /{flag=0} flag' "$GITHUB_DOC_ISSUE")"
printf '%s' "$what_it_is_section" | grep -qF "e.g." || issue_model_missing="$issue_model_missing github:what-it-is-not-illustrative"
if [ -z "$issue_model_missing" ]; then
  ok "issue_model_generalized_and_template_aligned"
else
  no "issue_model_generalized_and_template_aligned" "issues:$issue_model_missing"
fi

# badges_rationale_and_wired_r3_recorded (guard: the shop-window/honesty
# separation in github.md with one canonical Known Issues & Limitations label;
# CodeRabbit named as this repo's wired R3, with the wiring claim intact)
GITHUB_DOC="$REPO_ROOT/docs/standards/github.md"
CODEX_DOC_D="$REPO_ROOT/docs/standards/codex_review.md"
if grep -q "honesty duty is discharged" "$GITHUB_DOC" \
  && grep -q "Known Issues & Limitations, Contributing" "$GITHUB_DOC" \
  && grep -q "R3 is CodeRabbit" "$CODEX_DOC_D" \
  && grep -q "wired via its GitHub app" "$CODEX_DOC_D" \
  && grep -q "adjudicated in the PR discussion" "$CODEX_DOC_D"; then
  ok "badges_rationale_and_wired_r3_recorded"
else
  no "badges_rationale_and_wired_r3_recorded" "missing badges rationale or canonical Known Issues label in github.md, or the wired-R3 claim in codex_review.md"
fi

# framework_readme_and_license_recorded (guard: root README.md exists in
# canonical section order, links both decision-flow ADRs, Known Issues &
# Limitations is present, the MIT LICENSE exists and is named in the README
# License section, the Codex CLI prerequisite reads as a minimum rather than an
# exact pin, and no HTML comments or {...} placeholders remain)
README_DOC="$REPO_ROOT/README.md"
LICENSE_FILE="$REPO_ROOT/LICENSE"
readme_order_ok=0
if [ -f "$README_DOC" ]; then
  l_what_does="$(grep -n '^## What It Does$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_what_is="$(grep -n '^## What It Is$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_tech="$(grep -n '^## Tech Stack$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_engdec="$(grep -n '^## Engineering Decisions$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_getting="$(grep -n '^## Getting Started$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_structure="$(grep -n '^## Project Structure$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_status="$(grep -n '^## Project Status$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_known="$(grep -n '^## Known Issues & Limitations$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_contrib="$(grep -n '^## Contributing$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_license="$(grep -n '^## License$' "$README_DOC" | head -1 | cut -d: -f1)"
  # Getting Started must carry all four model-mandated subsections, in order.
  l_gs_prereq="$(grep -n '^### Prerequisites$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_gs_install="$(grep -n '^### Installation$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_gs_running="$(grep -n '^### Running$' "$README_DOC" | head -1 | cut -d: -f1)"
  l_gs_tests="$(grep -n '^### Tests$' "$README_DOC" | head -1 | cut -d: -f1)"
  if [ -n "$l_what_does" ] && [ -n "$l_what_is" ] && [ -n "$l_tech" ] && [ -n "$l_engdec" ] \
    && [ -n "$l_getting" ] && [ -n "$l_structure" ] && [ -n "$l_status" ] && [ -n "$l_known" ] \
    && [ -n "$l_contrib" ] && [ -n "$l_license" ] \
    && [ "$l_what_does" -lt "$l_what_is" ] && [ "$l_what_is" -lt "$l_tech" ] \
    && [ "$l_tech" -lt "$l_engdec" ] && [ "$l_engdec" -lt "$l_getting" ] \
    && [ "$l_getting" -lt "$l_structure" ] && [ "$l_structure" -lt "$l_status" ] \
    && [ "$l_status" -lt "$l_known" ] && [ "$l_known" -lt "$l_contrib" ] \
    && [ "$l_contrib" -lt "$l_license" ] \
    && [ -n "$l_gs_prereq" ] && [ -n "$l_gs_install" ] && [ -n "$l_gs_running" ] \
    && [ -n "$l_gs_tests" ] \
    && [ "$l_gs_prereq" -lt "$l_gs_install" ] && [ "$l_gs_install" -lt "$l_gs_running" ] \
    && [ "$l_gs_running" -lt "$l_gs_tests" ]; then
    readme_order_ok=1
  fi
fi
if [ "$readme_order_ok" -eq 1 ] \
  && grep -q "docs/adr/0001-decision-records-flow.md" "$README_DOC" \
  && grep -q "docs/adr/0002-durable-spec-archive.md" "$README_DOC" \
  && grep -q "AGENTS.md" "$README_DOC" \
  && grep -q "docs/agents/" "$README_DOC" \
  && grep -q "CLAUDE.md" "$README_DOC" \
  && grep -q "CONTEXT.md" "$README_DOC" \
  && grep -q "self-test" "$README_DOC" \
  && grep -q "token-economy choice is informational" "$README_DOC" \
  && grep -qE 'Codex CLI >= [0-9]' "$README_DOC" \
  && grep -q "MIT" "$README_DOC" \
  && [ -f "$LICENSE_FILE" ] \
  && ! grep -q "<!--" "$README_DOC" \
  && ! grep -qE '\{[^}]*\}' "$README_DOC"; then
  ok "framework_readme_and_license_recorded"
else
  no "framework_readme_and_license_recorded" "README missing/out of canonical order, missing ADR links, MIT naming, LICENSE file, a minimum-version (>=) Codex CLI prerequisite, or contains comments/placeholders"
fi

# repo_scripts_are_executable (guard: every shell entry point carries the
# executable bit in the git index — the filesystem lies on Windows)
if ! listing="$(cd "$REPO_ROOT" && git ls-files -s scripts .githooks)"; then
  no "repo_scripts_are_executable" "git ls-files listing failed"
else
  nonexec="$(printf '%s\n' "$listing" | awk '$1 != "100755" {print $4}' | grep -E '\.sh$|pre-push$' || true)"
  if [ -z "$nonexec" ]; then
    ok "repo_scripts_are_executable"
  else
    no "repo_scripts_are_executable" "not 100755: $(printf '%s' "$nonexec" | tr '\n' ' ')"
  fi
fi

# crux_method_and_skill_recorded (guard: the CRUX review-explanation method is
# recorded as an advisory review aid, indexed, with its skill entry, glossary
# term, and its three behavior requirements — humanizer pass, medium-default
# quiz difficulty, and skippable wrong-answer remediation)
CRUX_DOC="$REPO_ROOT/docs/standards/crux_method.md"
CRUX_INDEX="$REPO_ROOT/docs/standards/INDEX.md"
CRUX_SKILLS="$REPO_ROOT/docs/standards/skills_guidelines.md"
CRUX_CONTEXT="$REPO_ROOT/CONTEXT.md"
crux_missing=""
[ -f "$CRUX_DOC" ] || crux_missing="$crux_missing crux_method_file"
# INDEX must list the standard in both the Documents list and the Reading Order.
grep -qF '`crux_method.md`:' "$CRUX_INDEX" 2>/dev/null || crux_missing="$crux_missing index_documents"
grep -qF '10. `crux_method.md`' "$CRUX_INDEX" 2>/dev/null || crux_missing="$crux_missing index_reading_order"
# Advisory placement, both fallbacks (generator-absent vs humanizer-only-absent),
# and the three behavior requirements, asserted by exact clause so a reworded
# default or a dropped fallback is caught rather than passing on a stray keyword.
grep -q "advisory" "$CRUX_DOC" 2>/dev/null || crux_missing="$crux_missing advisory"
grep -qi "never blocks" "$CRUX_DOC" 2>/dev/null || crux_missing="$crux_missing not_blocking"
grep -qF '`humanizer` skill' "$CRUX_DOC" 2>/dev/null || crux_missing="$crux_missing humanizer_in_method"
grep -qF 'only `humanizer` is absent' "$CRUX_DOC" 2>/dev/null || crux_missing="$crux_missing humanizer_only_fallback"
grep -qF 'defaulting to `medium`' "$CRUX_DOC" 2>/dev/null || crux_missing="$crux_missing difficulty_medium"
grep -qF 'skip it and proceed' "$CRUX_DOC" 2>/dev/null || crux_missing="$crux_missing remediation_skip"
# The skill entry, scoped to its own section so a generic label from a different
# skill's section cannot satisfy the check.
grep -qF '## Explain-Change (CRUX)' "$CRUX_SKILLS" 2>/dev/null || crux_missing="$crux_missing skills_header"
crux_skill_section="$(awk '/^## Explain-Change \(CRUX\)$/{flag=1; next} /^## /{flag=0} flag' "$CRUX_SKILLS" 2>/dev/null)"
printf '%s' "$crux_skill_section" | grep -qF 'Pipeline stage:' || crux_missing="$crux_missing skills_pipeline"
printf '%s' "$crux_skill_section" | grep -qF 'Install/verify:' || crux_missing="$crux_missing skills_installverify"
printf '%s' "$crux_skill_section" | grep -qF 'Fallback:' || crux_missing="$crux_missing skills_fallback"
printf '%s' "$crux_skill_section" | grep -qF 'humanizer' || crux_missing="$crux_missing skills_humanizer"
# The glossary defines the term and lists CRUX among the framework's Methods.
grep -qF 'CRUX Method' "$CRUX_CONTEXT" 2>/dev/null || crux_missing="$crux_missing glossary_term"
grep -qF 'CRUX (review explanation)' "$CRUX_CONTEXT" 2>/dev/null || crux_missing="$crux_missing context_method_lists_crux"
# Security: the artifact contract must require escaping change-derived text and
# sanitizing generated URLs, so a reviewed diff cannot inject into the explainer.
grep -qi 'escap' "$CRUX_DOC" 2>/dev/null || crux_missing="$crux_missing artifact_escaping"
grep -qiE 'sanitiz|allowlist' "$CRUX_DOC" 2>/dev/null || crux_missing="$crux_missing artifact_url_sanitize"
if [ -z "$crux_missing" ]; then
  ok "crux_method_and_skill_recorded"
else
  no "crux_method_and_skill_recorded" "missing:$crux_missing"
fi

# crux_decision_promoted_to_adr (guard: the transient-explainer decision is a
# durable ADR, indexed in the README Engineering Decisions table)
CRUX_ADR="$REPO_ROOT/docs/adr/0003-crux-explainers-are-transient.md"
CRUX_README="$REPO_ROOT/README.md"
crux_adr_missing=""
[ -f "$CRUX_ADR" ] || crux_adr_missing="$crux_adr_missing adr_file"
grep -q "docs/adr/0003-crux-explainers-are-transient.md" "$CRUX_README" 2>/dev/null \
  || crux_adr_missing="$crux_adr_missing readme_row"
if [ -z "$crux_adr_missing" ]; then
  ok "crux_decision_promoted_to_adr"
else
  no "crux_decision_promoted_to_adr" "missing:$crux_adr_missing"
fi

# reviewer_switch_adr_restored (guard: the gpt-5.5 → gpt-5.6-terra reviewer
# decision deleted by PR #10 is restored as a durable ADR at 0004, carries its
# numbering history, and is indexed in the README Engineering Decisions table)
REVIEWER_ADR="$REPO_ROOT/docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md"
REVIEWER_README="$REPO_ROOT/README.md"
reviewer_adr_missing=""
[ -f "$REVIEWER_ADR" ] || reviewer_adr_missing="$reviewer_adr_missing adr_file"
grep -qF "gpt-5.6-terra" "$REVIEWER_ADR" 2>/dev/null \
  || reviewer_adr_missing="$reviewer_adr_missing chosen_default"
grep -qF "PR #10" "$REVIEWER_ADR" 2>/dev/null \
  || reviewer_adr_missing="$reviewer_adr_missing deletion_recorded"
grep -qF "0009" "$REVIEWER_ADR" 2>/dev/null \
  || reviewer_adr_missing="$reviewer_adr_missing reused_numbers_recorded"
grep -qF "docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md" "$REVIEWER_README" 2>/dev/null \
  || reviewer_adr_missing="$reviewer_adr_missing readme_row"
if [ -z "$reviewer_adr_missing" ]; then
  ok "reviewer_switch_adr_restored"
else
  no "reviewer_switch_adr_restored" "missing:$reviewer_adr_missing"
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
