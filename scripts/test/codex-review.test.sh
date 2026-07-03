#!/usr/bin/env bash
# Tests for the Codex R2 pre-push review runner (scripts/codex-review.sh).
# Each test maps to an Acceptance Criterion in SPEC.md.
set -u

# Isolate git config lookups from this machine's global/system scope so
# `git config codexreview.*` reads inside sandboxed repos never pick up
# an operator's real settings.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null

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

# Throwaway git repos so codexreview.* config never leaks from this repo.
REPO_SANDBOX="$(mktemp -d)"
trap 'rm -rf "$STUB_DIR" "$REPO_SANDBOX"' EXIT
new_repo() {
  d="$REPO_SANDBOX/repo-$1"
  git init -q "$d"
  printf '%s\n' "$d"
}

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

# review_model_default_when_unset (dry-run prints the default command)
expected='codex review --base main -c model="gpt-5.5" -c model_reasoning_effort="high"'
repo="$(new_repo default)"
out=$(cd "$repo" && PATH="$STUB_DIR:$PATH" CODEX_REVIEW_MODEL= CODEX_REVIEW_EFFORT= CODEX_REVIEW_BRANCH=feature/x CODEX_REVIEW_DRYRUN=1 bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s\n' "$out" | grep -qxF "$expected"; then
  ok "review_model_default_when_unset"
else
  no "review_model_default_when_unset" "code=$code out=$out"
fi

# honors_dryrun_even_when_codex_absent (R2 finding P2): dry-run prints the command
# regardless of Codex availability, per codex_review.md.
repo="$(new_repo dryrun)"
out=$(cd "$repo" && CODEX_REVIEW_MODEL= CODEX_REVIEW_EFFORT= CODEX_REVIEW_BRANCH=feature/x CODEX_BIN=__no_such_codex__ CODEX_REVIEW_DRYRUN=1 bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s\n' "$out" | grep -qxF "$expected"; then
  ok "honors_dryrun_even_when_codex_absent"
else
  no "honors_dryrun_even_when_codex_absent" "code=$code out=$out"
fi

# review_model_env_override (env vars replace model and effort)
expected_env='codex review --base main -c model="modelX" -c model_reasoning_effort="xhigh"'
repo="$(new_repo envover)"
out=$(cd "$repo" && CODEX_REVIEW_MODEL=modelX CODEX_REVIEW_EFFORT=xhigh CODEX_REVIEW_BRANCH=feature/x CODEX_REVIEW_DRYRUN=1 bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s\n' "$out" | grep -qxF "$expected_env"; then
  ok "review_model_env_override"
else
  no "review_model_env_override" "code=$code out=$out"
fi

# review_model_git_config_fallback (persisted keys used when env is absent)
expected_cfg='codex review --base main -c model="modelY" -c model_reasoning_effort="low"'
repo="$(new_repo cfg)"
git -C "$repo" config codexreview.model modelY
git -C "$repo" config codexreview.effort low
out=$(cd "$repo" && CODEX_REVIEW_MODEL= CODEX_REVIEW_EFFORT= CODEX_REVIEW_BRANCH=feature/x CODEX_REVIEW_DRYRUN=1 bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s\n' "$out" | grep -qxF "$expected_cfg"; then
  ok "review_model_git_config_fallback"
else
  no "review_model_git_config_fallback" "code=$code out=$out"
fi

# review_model_env_beats_git_config (precedence: env wins over persisted keys)
repo="$(new_repo prec)"
git -C "$repo" config codexreview.model modelY
git -C "$repo" config codexreview.effort low
out=$(cd "$repo" && CODEX_REVIEW_MODEL=modelX CODEX_REVIEW_EFFORT=xhigh CODEX_REVIEW_BRANCH=feature/x CODEX_REVIEW_DRYRUN=1 bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s\n' "$out" | grep -qxF "$expected_env"; then
  ok "review_model_env_beats_git_config"
else
  no "review_model_env_beats_git_config" "code=$code out=$out"
fi

# review_model_ignores_global_scope (persisted choices are repo-local state;
# a machine-wide codexreview.* must not leak into other repos)
repo="$(new_repo globalscope)"
globalcfg="$REPO_SANDBOX/globalconfig"
git config --file "$globalcfg" codexreview.model global-model
git config --file "$globalcfg" codexreview.effort global-effort
out=$(cd "$repo" && GIT_CONFIG_GLOBAL="$globalcfg" CODEX_REVIEW_MODEL= CODEX_REVIEW_EFFORT= CODEX_REVIEW_BRANCH=feature/x CODEX_REVIEW_DRYRUN=1 bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s\n' "$out" | grep -qxF "$expected"; then
  ok "review_model_ignores_global_scope"
else
  no "review_model_ignores_global_scope" "code=$code out=$out"
fi

# review_model_empty_env_treated_as_unset (invariant pin: an empty override
# must fall through to the persisted config, never produce -c model="")
repo="$(new_repo emptyenv)"
git -C "$repo" config codexreview.model modelY
git -C "$repo" config codexreview.effort low
out=$(cd "$repo" && CODEX_REVIEW_MODEL="" CODEX_REVIEW_EFFORT="" CODEX_REVIEW_BRANCH=feature/x CODEX_REVIEW_DRYRUN=1 bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s\n' "$out" | grep -qxF "$expected_cfg"; then
  ok "review_model_empty_env_treated_as_unset"
else
  no "review_model_empty_env_treated_as_unset" "code=$code out=$out"
fi

# codex_review_doc_qualifies_env_override (guard: the doc must state that the
# env override only applies when non-empty)
CODEX_DOC="$REPO_ROOT/docs/standards/codex_review.md"
if grep -q 'CODEX_REVIEW_MODEL=<model>.*if non-empty' "$CODEX_DOC" \
  && grep -q 'CODEX_REVIEW_EFFORT=<effort>.*if non-empty' "$CODEX_DOC"; then
  ok "codex_review_doc_qualifies_env_override"
else
  no "codex_review_doc_qualifies_env_override" "codex_review.md env-var bullets lack the 'if non-empty' qualifier"
fi

# reviewer_defaults_match_across_scripts (guard: the default literals shown by
# setup's prompts must match the runner's resolution fallbacks)
SETUP_SH="$REPO_ROOT/scripts/setup.sh"
runner_model_default="$(sed -n 's/.*{config_model:-\([^}]*\)}.*/\1/p' "$RUNNER")"
runner_effort_default="$(sed -n 's/.*{config_effort:-\([^}]*\)}.*/\1/p' "$RUNNER")"
setup_model_defaults="$(grep -o '{current_model:-[^}]*}' "$SETUP_SH" | sed 's/{current_model:-//; s/}$//' | sort -u)"
setup_effort_defaults="$(grep -o '{current_effort:-[^}]*}' "$SETUP_SH" | sed 's/{current_effort:-//; s/}$//' | sort -u)"
if [ -n "$runner_model_default" ] && [ "$setup_model_defaults" = "$runner_model_default" ] \
  && [ -n "$runner_effort_default" ] && [ "$setup_effort_defaults" = "$runner_effort_default" ]; then
  ok "reviewer_defaults_match_across_scripts"
else
  no "reviewer_defaults_match_across_scripts" "runner=[$runner_model_default/$runner_effort_default] setup_model=[$setup_model_defaults] setup_effort=[$setup_effort_defaults]"
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
