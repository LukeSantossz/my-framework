#!/usr/bin/env bash
# Tests for the activation bootstrap (scripts/setup.sh).
# Each test maps to an Acceptance Criterion in SPEC.md.
set -u

# Isolate git config lookups from this machine's global/system scope so
# `git config codexreview.*` reads inside sandboxed repos never pick up
# an operator's real settings.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null

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
  "label list")
    [ -n "${STUB_GH_LIST_FAILS:-}" ] && exit 1
    [ -n "${STUB_GH_LABELS:-}" ] && printf '%s\n' $STUB_GH_LABELS; exit 0 ;;
  "label create") [ -n "${STUB_GH_CREATE_FAILS:-}" ] && exit 1; exit 0 ;;
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

# setup_is_idempotent (first run creates every label, second run — with the
# labels now existing — must add no new creates)
repo="$(new_repo idem)"
log="$SANDBOX/idem.log"; : > "$log"
set -- $ALL_LABELS; expected=$#
out1=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="" PATH="$STUB_DIR:$PATH" bash "$RUNNER" 2>&1); code1=$?
creates1="$(grep -c "label create" "$log")"
out2=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" 2>&1); code2=$?
creates2="$(grep -c "label create" "$log")"
hooks="$(git -C "$repo" config core.hooksPath || true)"
if [ "$code1" -eq 0 ] && [ "$code2" -eq 0 ] && [ "$hooks" = ".githooks" ] \
  && [ "$creates1" -eq "$expected" ] && [ "$creates2" -eq "$expected" ]; then
  ok "setup_is_idempotent"
else
  no "setup_is_idempotent" "code1=$code1 code2=$code2 hooksPath=$hooks creates1=$creates1 creates2=$creates2 out2=$out2"
fi

# setup_lists_labels_unpaginated
repo="$(new_repo unpaginated)"
log="$SANDBOX/unpaginated.log"; : > "$log"
out=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && grep -q -- "label list .*--limit" "$log"; then
  ok "setup_lists_labels_unpaginated"
else
  no "setup_lists_labels_unpaginated" "code=$code out=$out"
fi

# setup_fails_when_hookspath_cannot_be_set (a stale config lock makes
# `git config` fail; setup must not report the gate as active).
repo="$(new_repo lockedcfg)"
log="$SANDBOX/lockedcfg.log"; : > "$log"
touch "$repo/.git/config.lock"
out=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" 2>&1); code=$?
if [ "$code" -ne 0 ] && ! printf '%s' "$out" | grep -q "gate active"; then
  ok "setup_fails_when_hookspath_cannot_be_set"
else
  no "setup_fails_when_hookspath_cannot_be_set" "code=$code out=$out"
fi

# setup_skips_label_creation_when_list_fails (label discovery failing must not
# be treated as an empty label set; no creates, advisory message, exit 0).
repo="$(new_repo listfail)"
log="$SANDBOX/listfail.log"; : > "$log"
out=$(cd "$repo" && GH_LOG="$log" STUB_GH_LIST_FAILS=1 PATH="$STUB_DIR:$PATH" bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && ! grep -q "label create" "$log" \
  && printf '%s' "$out" | grep -q "could not list labels"; then
  ok "setup_skips_label_creation_when_list_fails"
else
  no "setup_skips_label_creation_when_list_fails" "code=$code out=$out"
fi

# setup_fails_when_label_create_fails (a failed create must not end in a
# success report; advisory covers only an absent/unauthenticated toolchain).
repo="$(new_repo createfail)"
log="$SANDBOX/createfail.log"; : > "$log"
out=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="" STUB_GH_CREATE_FAILS=1 PATH="$STUB_DIR:$PATH" bash "$RUNNER" 2>&1); code=$?
if [ "$code" -ne 0 ] && ! printf '%s' "$out" | grep -q "bootstrap complete"; then
  ok "setup_fails_when_label_create_fails"
else
  no "setup_fails_when_label_create_fails" "code=$code out=$out"
fi

# setup_interactive_persists_choices (answers persisted via git config and
# echoed in the summary)
repo="$(new_repo interactive)"
log="$SANDBOX/interactive.log"; : > "$log"
out=$(cd "$repo" && printf 'modelZ\nxhigh\nterse\n' | GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" --interactive 2>&1); code=$?
model="$(git -C "$repo" config codexreview.model || true)"
effort="$(git -C "$repo" config codexreview.effort || true)"
if [ "$code" -eq 0 ] && [ "$model" = "modelZ" ] && [ "$effort" = "xhigh" ] \
  && printf '%s' "$out" | grep -q "reviewer=modelZ" \
  && printf '%s' "$out" | grep -q "effort=xhigh"; then
  ok "setup_interactive_persists_choices"
else
  no "setup_interactive_persists_choices" "code=$code model=$model effort=$effort out=$out"
fi

# setup_interactive_enter_keeps_defaults (empty answers write no keys, so the
# script defaults can evolve without stale persisted copies)
repo="$(new_repo enterdefault)"
log="$SANDBOX/enterdefault.log"; : > "$log"
out=$(cd "$repo" && printf '\n\n\n' | GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" --interactive 2>&1); code=$?
if [ "$code" -eq 0 ] && ! git -C "$repo" config codexreview.model >/dev/null 2>&1 \
  && ! git -C "$repo" config codexreview.effort >/dev/null 2>&1; then
  ok "setup_interactive_enter_keeps_defaults"
else
  no "setup_interactive_enter_keeps_defaults" "code=$code out=$out"
fi

# setup_noninteractive_never_prompts (no flag: no prompt text, no keys, exit 0
# even with stdin closed)
repo="$(new_repo noninteractive)"
log="$SANDBOX/noninteractive.log"; : > "$log"
out=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" </dev/null 2>&1); code=$?
if [ "$code" -eq 0 ] && ! printf '%s' "$out" | grep -q "reviewer model" \
  && ! git -C "$repo" config codexreview.model >/dev/null 2>&1; then
  ok "setup_noninteractive_never_prompts"
else
  no "setup_noninteractive_never_prompts" "code=$code out=$out"
fi

# setup_rejects_unknown_option (usage error: fail fast instead of silently
# running a bootstrap the caller did not ask for)
repo="$(new_repo unknownopt)"
log="$SANDBOX/unknownopt.log"; : > "$log"
out=$(cd "$repo" && GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" --interactve 2>&1); code=$?
if [ "$code" -ne 0 ] && printf '%s' "$out" | grep -q "unknown option"; then
  ok "setup_rejects_unknown_option"
else
  no "setup_rejects_unknown_option" "code=$code out=$out"
fi

# setup_interactive_ignores_global_scope (prompts and summary must reflect the
# local scope the runner reads, not machine-wide git config)
repo="$(new_repo globalscope)"
log="$SANDBOX/globalscope.log"; : > "$log"
globalcfg="$SANDBOX/globalconfig"
git config --file "$globalcfg" codexreview.model global-model
out=$(cd "$repo" && printf '\n\n\n' | GIT_CONFIG_GLOBAL="$globalcfg" GH_LOG="$log" STUB_GH_LABELS="$ALL_LABELS" PATH="$STUB_DIR:$PATH" bash "$RUNNER" --interactive 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s' "$out" | grep -q "reviewer=gpt-5.5" \
  && ! printf '%s' "$out" | grep -q "global-model"; then
  ok "setup_interactive_ignores_global_scope"
else
  no "setup_interactive_ignores_global_scope" "code=$code out=$out"
fi

# setup_label_specs_match_triage_labels_doc (guard: fails if setup.sh's
# LABEL_SPECS drifts from docs/agents/triage-labels.md's mapping table).
TRIAGE_DOC="$REPO_ROOT/docs/agents/triage-labels.md"
spec_labels="$(sed -n "/^LABEL_SPECS='/,/'\$/p" "$RUNNER" \
  | sed "1s/^LABEL_SPECS='//; \$s/'\$//" \
  | cut -d'|' -f1 | sort)"
doc_labels="$(grep -E '^\| `' "$TRIAGE_DOC" \
  | awk -F'|' '{print $3}' \
  | tr -d '` ' | sort)"
if [ "$spec_labels" = "$doc_labels" ]; then
  ok "setup_label_specs_match_triage_labels_doc"
else
  no "setup_label_specs_match_triage_labels_doc" "setup.sh=[$(printf '%s' "$spec_labels" | tr '\n' ' ')] triage-labels.md=[$(printf '%s' "$doc_labels" | tr '\n' ' ')]"
fi

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
