#!/usr/bin/env bash
# Tests for the R2 pre-push review runner (scripts/r2-review.sh), its backend
# chain, and the adapters under scripts/reviewers/.
# Cases map to Acceptance Criteria in docs/specs/0001-add-codex-pre-push-gate.md
# and docs/specs/0013-detach-r2-from-codex.md.
set -u

# Isolate git config lookups from this machine's global/system scope so
# `git config codexreview.*` reads inside sandboxed repos never pick up
# an operator's real settings.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null

TEST_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$TEST_DIR/../.." && pwd)"
RUNNER="$REPO_ROOT/scripts/r2-review.sh"
AGENTS_FILE="$REPO_ROOT/AGENTS.md"
R2_DOC="$REPO_ROOT/docs/standards/r2_gate.md"

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
expected='codex review --base main -c model="gpt-5.6-terra" -c model_reasoning_effort="high"'
repo="$(new_repo default)"
out=$(cd "$repo" && PATH="$STUB_DIR:$PATH" CODEX_REVIEW_MODEL= CODEX_REVIEW_EFFORT= CODEX_REVIEW_BRANCH=feature/x CODEX_REVIEW_DRYRUN=1 bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] && printf '%s\n' "$out" | grep -qxF "$expected"; then
  ok "review_model_default_when_unset"
else
  no "review_model_default_when_unset" "code=$code out=$out"
fi

# honors_dryrun_even_when_codex_absent (R2 finding P2): dry-run prints the command
# regardless of Codex availability, per r2_gate.md.
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

# r2_gate_doc_qualifies_env_override (guard: the doc must state that the
# env override only applies when non-empty)
if grep -q 'CODEX_REVIEW_MODEL=<model>.*if non-empty' "$R2_DOC" \
  && grep -q 'CODEX_REVIEW_EFFORT=<effort>.*if non-empty' "$R2_DOC"; then
  ok "r2_gate_doc_qualifies_env_override"
else
  no "r2_gate_doc_qualifies_env_override" "r2_gate.md env-var bullets lack the 'if non-empty' qualifier"
fi

# reviewer_defaults_match_across_scripts (guard: the default literals shown by
# setup's prompts must match the runner's resolution fallbacks)
SETUP_SH="$REPO_ROOT/scripts/setup.sh"
runner_model_default="$(sed -n 's/^ *default_model="\([^"]\+\)".*/\1/p' "$RUNNER" | sort -u)"
runner_effort_default="$(sed -n 's/^ *default_effort="\([^"]\+\)".*/\1/p' "$RUNNER" | sort -u)"
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

# =====================================================================
# The backend chain (spec 0013). Stub adapters stand in for the real
# reviewers, so the chain's control flow is exercised without a provider,
# a key, or a quota.
# =====================================================================

ADAPTERS="$REPO_SANDBOX/reviewers"
mkdir -p "$ADAPTERS"

# A stub adapter honors both halves of the contract: under R2_DRYRUN it
# describes what it would do and runs nothing, and otherwise it announces
# itself and exits whatever STUB_<NAME>_EXIT says (0 reviewed, 10 unavailable,
# other reviewed-with-findings).
make_adapter() {
  cat > "$ADAPTERS/$1.sh" <<STUB
#!/bin/sh
if [ "\${R2_DRYRUN:-}" = "1" ]; then
  echo "DRYRUN_$1 model=\${R2_RESOLVED_MODEL:-}"
  exit 0
fi
echo "STUB_ADAPTER_$1 model=\${R2_RESOLVED_MODEL:-} effort=\${R2_RESOLVED_EFFORT:-} base=\${R2_BASE:-}"
exit \${STUB_$(printf '%s' "$1" | tr '[:lower:]-' '[:upper:]_')_EXIT:-0}
STUB
  chmod +x "$ADAPTERS/$1.sh"
}
make_adapter alpha
make_adapter beta

# Runs the chain with the stub adapter directory and a branch that is not the
# base. `env` is what makes the caller's assignments in "$@" take effect: bash
# only recognizes assignments written literally before the command, so an
# expanded one would be run as a command name instead. CODEX_BIN is pointed at
# an absent path so a regression in chain resolution can never reach the real
# reviewer and spend the operator's quota from a test run.
run_chain() {
  env R2_REVIEWERS_DIR="$ADAPTERS" R2_BRANCH=feature/x \
    CODEX_BIN="$REPO_SANDBOX/absent-codex" "$@" bash "$RUNNER" 2>&1
}

# --- chain_advances_past_unavailable_backend --------------------------------
out=$(run_chain R2_BACKENDS=alpha,beta STUB_ALPHA_EXIT=10 STUB_BETA_EXIT=0); code=$?
if [ "$code" -eq 0 ] \
  && printf '%s' "$out" | grep -q "STUB_ADAPTER_alpha" \
  && printf '%s' "$out" | grep -q "STUB_ADAPTER_beta"; then
  ok "chain_advances_past_unavailable_backend"
else
  no "chain_advances_past_unavailable_backend" "code=$code out=$out"
fi

# --- chain_stops_at_first_backend_that_reviews ------------------------------
out=$(run_chain R2_BACKENDS=alpha,beta STUB_ALPHA_EXIT=0 STUB_BETA_EXIT=0); code=$?
if [ "$code" -eq 0 ] \
  && printf '%s' "$out" | grep -q "STUB_ADAPTER_alpha" \
  && ! printf '%s' "$out" | grep -q "STUB_ADAPTER_beta"; then
  ok "chain_stops_at_first_backend_that_reviews"
else
  no "chain_stops_at_first_backend_that_reviews" "code=$code out=$out"
fi

# --- chain_reports_which_backend_reviewed -----------------------------------
# The line exists to be copied into the PR's review-layers record, so it must
# name both the backend that reviewed and the one that was skipped, with why.
out=$(run_chain R2_BACKENDS=alpha,beta R2_MODEL=modelZ STUB_ALPHA_EXIT=10 STUB_BETA_EXIT=0); code=$?
report_missing=""
printf '%s' "$out" | grep -qi "reviewed by" || report_missing="$report_missing no_reviewed_by_line"
printf '%s' "$out" | grep -q "beta" || report_missing="$report_missing reviewer_not_named"
printf '%s' "$out" | grep -q "modelZ" || report_missing="$report_missing model_not_named"
printf '%s' "$out" | grep -qi "alpha.*unavailable\|unavailable.*alpha" || report_missing="$report_missing skip_not_reported"
if [ "$code" -eq 0 ] && [ -z "$report_missing" ]; then
  ok "chain_reports_which_backend_reviewed"
else
  no "chain_reports_which_backend_reviewed" "code=$code missing:$report_missing out=$out"
fi

# --- chain_reports_when_no_backend_ran --------------------------------------
out=$(run_chain R2_BACKENDS=alpha,beta STUB_ALPHA_EXIT=10 STUB_BETA_EXIT=10); code=$?
if [ "$code" -eq 0 ] && printf '%s' "$out" | grep -qi "did not run"; then
  ok "chain_reports_when_no_backend_ran"
else
  no "chain_reports_when_no_backend_ran" "code=$code out=$out"
fi

# --- empty_diff_is_not_reported_as_a_review ---------------------------------
# A branch level with its base has nothing to review. Letting a backend answer
# "no diff" with exit 0 would make the chain announce a review that never
# happened, which is the one thing the reporting line exists to prevent.
empty_repo="$REPO_SANDBOX/emptydiff"
git init -q "$empty_repo"
git -C "$empty_repo" config user.email t@example.com
git -C "$empty_repo" config user.name Test
printf 'x\n' > "$empty_repo/f.txt"
git -C "$empty_repo" add -A && git -C "$empty_repo" commit -qm base
git -C "$empty_repo" branch -M main
git -C "$empty_repo" checkout -qb feature/x
out=$(cd "$empty_repo" && env R2_REVIEWERS_DIR="$ADAPTERS" R2_BACKENDS=alpha \
  R2_BASE=main R2_BRANCH=feature/x STUB_ALPHA_EXIT=0 bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] \
  && ! printf '%s' "$out" | grep -qi "reviewed by" \
  && ! printf '%s' "$out" | grep -q "STUB_ADAPTER_alpha" \
  && printf '%s' "$out" | grep -qi "nothing to review"; then
  ok "empty_diff_is_not_reported_as_a_review"
else
  no "empty_diff_is_not_reported_as_a_review" "code=$code out=$out"
fi

# --- unknown_backend_is_reported_not_ignored --------------------------------
out=$(run_chain R2_BACKENDS=nosuch,beta STUB_BETA_EXIT=0); code=$?
if [ "$code" -eq 0 ] \
  && printf '%s' "$out" | grep -q "nosuch" \
  && printf '%s' "$out" | grep -q "STUB_ADAPTER_beta"; then
  ok "unknown_backend_is_reported_not_ignored"
else
  no "unknown_backend_is_reported_not_ignored" "code=$code out=$out"
fi

# --- blocking_mode_blocks_on_findings ---------------------------------------
out=$(run_chain R2_BACKENDS=alpha R2_BLOCKING=1 STUB_ALPHA_EXIT=3); code=$?
if [ "$code" -ne 0 ]; then
  ok "blocking_mode_blocks_on_findings"
else
  no "blocking_mode_blocks_on_findings" "code=$code out=$out"
fi

# --- blocking_mode_does_not_block_on_unavailable ----------------------------
# A reviewer that never ran is not a finding; blocking on it would turn an
# expired quota into a locked repository.
out=$(run_chain R2_BACKENDS=alpha,beta R2_BLOCKING=1 STUB_ALPHA_EXIT=10 STUB_BETA_EXIT=10); code=$?
if [ "$code" -eq 0 ]; then
  ok "blocking_mode_does_not_block_on_unavailable"
else
  no "blocking_mode_does_not_block_on_unavailable" "code=$code out=$out"
fi

# --- default_chain_is_codex_only --------------------------------------------
repo="$(new_repo defaultchain)"
out=$(cd "$repo" && R2_BRANCH=feature/x R2_DRYRUN=1 CODEX_REVIEW_MODEL= CODEX_REVIEW_EFFORT= bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] \
  && printf '%s' "$out" | grep -q "codex" \
  && ! printf '%s' "$out" | grep -qi "gemini\|openai"; then
  ok "default_chain_is_codex_only"
else
  no "default_chain_is_codex_only" "code=$code out=$out"
fi

# --- settings_resolve_by_scope_cascade --------------------------------------
# The new r2.* keys read the full cascade; this is what makes a machine-global
# default possible, and is the one behavior the legacy keys deliberately lack.
repo="$(new_repo cascade)"
globalcfg2="$REPO_SANDBOX/globalconfig2"
git config --file "$globalcfg2" r2.backends alpha
git config --file "$globalcfg2" r2.model global-model
cascade_missing=""
out=$(cd "$repo" && GIT_CONFIG_GLOBAL="$globalcfg2" R2_REVIEWERS_DIR="$ADAPTERS" \
  R2_BRANCH=feature/x bash "$RUNNER" 2>&1)
printf '%s' "$out" | grep -q "model=global-model" || cascade_missing="$cascade_missing global_not_read"
git -C "$repo" config r2.model local-model
out=$(cd "$repo" && GIT_CONFIG_GLOBAL="$globalcfg2" R2_REVIEWERS_DIR="$ADAPTERS" \
  R2_BRANCH=feature/x bash "$RUNNER" 2>&1)
printf '%s' "$out" | grep -q "model=local-model" || cascade_missing="$cascade_missing local_does_not_beat_global"
out=$(cd "$repo" && GIT_CONFIG_GLOBAL="$globalcfg2" R2_REVIEWERS_DIR="$ADAPTERS" \
  R2_BRANCH=feature/x R2_MODEL=env-model bash "$RUNNER" 2>&1)
printf '%s' "$out" | grep -q "model=env-model" || cascade_missing="$cascade_missing env_does_not_beat_local"
if [ -z "$cascade_missing" ]; then
  ok "settings_resolve_by_scope_cascade"
else
  no "settings_resolve_by_scope_cascade" "missing:$cascade_missing"
fi

# --- per_backend_model_beats_shared_model -----------------------------------
repo="$(new_repo perbackend)"
git -C "$repo" config r2.backends alpha
git -C "$repo" config r2.model shared-model
git -C "$repo" config r2.alpha.model alpha-model
out=$(cd "$repo" && R2_REVIEWERS_DIR="$ADAPTERS" R2_BRANCH=feature/x bash "$RUNNER" 2>&1)
if printf '%s' "$out" | grep -q "model=alpha-model"; then
  ok "per_backend_model_beats_shared_model"
else
  no "per_backend_model_beats_shared_model" "out=$out"
fi

# --- legacy_bypass_still_skips_the_gate -------------------------------------
out=$(SKIP_CODEX_REVIEW=1 R2_REVIEWERS_DIR="$ADAPTERS" R2_BACKENDS=alpha R2_BRANCH=feature/x bash "$RUNNER" 2>&1); code=$?
out2=$(SKIP_R2_REVIEW=1 R2_REVIEWERS_DIR="$ADAPTERS" R2_BACKENDS=alpha R2_BRANCH=feature/x bash "$RUNNER" 2>&1); code2=$?
if [ "$code" -eq 0 ] && [ "$code2" -eq 0 ] \
  && ! printf '%s' "$out" | grep -q "STUB_ADAPTER_alpha" \
  && ! printf '%s' "$out2" | grep -q "STUB_ADAPTER_alpha"; then
  ok "legacy_bypass_still_skips_the_gate"
else
  no "legacy_bypass_still_skips_the_gate" "code=$code code2=$code2"
fi

# --- legacy_control_variables_still_resolve ---------------------------------
# Raised by the R2 review of this change: the spec claimed the codex backend
# preserves today's behavior exactly, but only the model and effort variables
# had a criterion. These three were implemented and unpinned, which is the
# shape a later refactor quietly breaks.
legacy_missing=""
out=$(run_chain R2_BACKENDS=alpha CODEX_REVIEW_BLOCKING=1 STUB_ALPHA_EXIT=3); code=$?
[ "$code" -ne 0 ] || legacy_missing="$legacy_missing CODEX_REVIEW_BLOCKING"
out=$(env R2_REVIEWERS_DIR="$ADAPTERS" R2_BACKENDS=alpha R2_BRANCH=release \
  CODEX_REVIEW_BASE=release bash "$RUNNER" 2>&1)
printf '%s' "$out" | grep -qi "nothing to review against itself" \
  || legacy_missing="$legacy_missing CODEX_REVIEW_BASE"
repo="$(new_repo legacydry)"
out=$(cd "$repo" && env R2_REVIEWERS_DIR="$ADAPTERS" R2_BACKENDS=alpha \
  R2_BRANCH=feature/x CODEX_REVIEW_DRYRUN=1 bash "$RUNNER" 2>&1)
printf '%s' "$out" | grep -q "DRYRUN_alpha" || legacy_missing="$legacy_missing CODEX_REVIEW_DRYRUN"
printf '%s' "$out" | grep -q "STUB_ADAPTER_alpha" && legacy_missing="$legacy_missing dryrun_ran_the_backend"
if [ -z "$legacy_missing" ]; then
  ok "legacy_control_variables_still_resolve"
else
  no "legacy_control_variables_still_resolve" "not honored:$legacy_missing"
fi

# --- dryrun_prints_the_resolved_chain ---------------------------------------
# Dry-run must describe the whole chain, not stop at the first backend: the
# point is to show what would happen, including the fallbacks.
repo="$(new_repo dryrunchain)"
out=$(cd "$repo" && R2_REVIEWERS_DIR="$ADAPTERS" R2_BACKENDS=alpha,beta \
  R2_BRANCH=feature/x R2_DRYRUN=1 bash "$RUNNER" 2>&1); code=$?
if [ "$code" -eq 0 ] \
  && printf '%s' "$out" | grep -q "alpha" && printf '%s' "$out" | grep -q "beta" \
  && ! printf '%s' "$out" | grep -q "STUB_ADAPTER_alpha"; then
  ok "dryrun_prints_the_resolved_chain"
else
  no "dryrun_prints_the_resolved_chain" "code=$code out=$out"
fi

# =====================================================================
# The openai-compatible adapter, against a stub endpoint. No test reaches
# a real provider, spends quota, or needs a key.
# =====================================================================

OPENAI_ADAPTER="$REPO_ROOT/scripts/reviewers/openai.sh"
node_bin="$(command -v node || true)"

if [ -z "$node_bin" ]; then
  for skipped in openai_adapter_sends_the_contract_payload openai_adapter_reads_content_not_reasoning \
    openai_adapter_reports_a_cut_off_review openai_adapter_reports_truncation \
    openai_adapter_is_unavailable_on_unreachable_endpoint openai_adapter_is_unavailable_without_node \n    openai_adapter_is_unavailable_past_its_time_budget; do
    printf 'skip - %s (node not installed)\n' "$skipped"
  done
else
  SERVER_DIR="$REPO_SANDBOX/stub-endpoint"
  mkdir -p "$SERVER_DIR"
  cat > "$SERVER_DIR/server.js" <<'SRV'
// Minimal OpenAI-compatible stub: records the request body, replies per MODE.
const fs = require("fs");
const http = require("http");
const capture = process.env.CAPTURE;
const portFile = process.env.PORT_FILE;
const mode = process.env.MODE || "normal";
const bodies = {
  normal: { content: "Finding: unquoted variable on line 3.", finish: "stop" },
  reasoning: { content: "Finding: real defect.", reasoning: "internal chain of thought", finish: "stop" },
  cutoff: { content: "Finding: partial", finish: "length" },
  slow: { content: "Finding: eventually.", finish: "stop", delayMs: 5000 },
};
http
  .createServer((req, res) => {
    let body = "";
    req.on("data", (c) => (body += c));
    req.on("end", () => {
      fs.writeFileSync(capture, body);
      const b = bodies[mode];
      const message = { role: "assistant", content: b.content };
      if (b.reasoning) message.reasoning_content = b.reasoning;
      const reply = () => {
        res.writeHead(200, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ choices: [{ index: 0, message, finish_reason: b.finish }] }));
      };
      if (b.delayMs) setTimeout(reply, b.delayMs); else reply();
    });
  })
  .listen(0, "127.0.0.1", function () {
    fs.writeFileSync(portFile, String(this.address().port));
  });
SRV

  start_stub_endpoint() {
    STUB_CAPTURE="$SERVER_DIR/body-$1.json"
    STUB_PORTFILE="$SERVER_DIR/port-$1"
    rm -f "$STUB_CAPTURE" "$STUB_PORTFILE"
    CAPTURE="$STUB_CAPTURE" PORT_FILE="$STUB_PORTFILE" MODE="$2" \
      node "$SERVER_DIR/server.js" &
    STUB_PID=$!
    tries=0
    while [ ! -s "$STUB_PORTFILE" ] && [ "$tries" -lt 100 ]; do
      tries=$((tries + 1)); sleep 0.1
    done
    STUB_PORT="$(cat "$STUB_PORTFILE" 2>/dev/null || true)"
  }

  # A repo with a real diff for the adapter to send.
  diff_repo="$REPO_SANDBOX/diffrepo"
  git init -q "$diff_repo"
  git -C "$diff_repo" config user.email t@example.com
  git -C "$diff_repo" config user.name Test
  printf 'base\n' > "$diff_repo/file.txt"
  git -C "$diff_repo" add -A && git -C "$diff_repo" commit -qm base
  git -C "$diff_repo" branch -M main
  git -C "$diff_repo" checkout -qb feature/x
  printf 'base\nDISTINCTIVE_ADDED_LINE\n' > "$diff_repo/file.txt"
  git -C "$diff_repo" add -A && git -C "$diff_repo" commit -qm change

  run_openai() {
    (cd "$diff_repo" && env R2_BASE=main R2_BRANCH=feature/x \
      R2_RESOLVED_MODEL=stub-model R2_OPENAI_ENDPOINT="http://127.0.0.1:$STUB_PORT" \
      "$@" bash "$OPENAI_ADAPTER" 2>&1)
  }

  # --- openai_adapter_sends_the_contract_payload ----------------------------
  start_stub_endpoint payload normal
  out=$(run_openai); code=$?
  kill "$STUB_PID" 2>/dev/null
  payload_missing=""
  [ -s "$STUB_CAPTURE" ] || payload_missing="$payload_missing no_request_captured"
  if [ -s "$STUB_CAPTURE" ]; then
    node -e '
      const fs = require("fs");
      const b = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
      const sys = b.messages.filter(m => m.role === "system").map(m => m.content).join("\n");
      const usr = b.messages.filter(m => m.role === "user").map(m => m.content).join("\n");
      const agents = fs.readFileSync(process.argv[2], "utf8");
      const probe = agents.split("\n").find(l => l.trim().length > 40) || "";
      if (b.model !== "stub-model") { console.log("model_not_sent"); }
      if (!sys.includes(probe.trim())) { console.log("agents_md_not_in_system"); }
      if (!usr.includes("DISTINCTIVE_ADDED_LINE")) { console.log("diff_not_in_user"); }
      const sysIdx = b.messages.findIndex(m => m.role === "system");
      const usrIdx = b.messages.findIndex(m => m.role === "user");
      if (!(sysIdx < usrIdx)) { console.log("volatile_diff_not_last"); }
    ' "$STUB_CAPTURE" "$AGENTS_FILE" > "$SERVER_DIR/payload-check" 2>&1
    payload_missing="$payload_missing $(tr '\n' ' ' < "$SERVER_DIR/payload-check")"
  fi
  if [ "$code" -eq 0 ] && [ -z "$(printf '%s' "$payload_missing" | tr -d ' ')" ]; then
    ok "openai_adapter_sends_the_contract_payload"
  else
    no "openai_adapter_sends_the_contract_payload" "code=$code missing:$payload_missing out=$out"
  fi

  # --- openai_adapter_reads_content_not_reasoning ---------------------------
  start_stub_endpoint reasoning reasoning
  out=$(run_openai); code=$?
  kill "$STUB_PID" 2>/dev/null
  if [ "$code" -eq 0 ] \
    && printf '%s' "$out" | grep -qF "Finding: real defect." \
    && ! printf '%s' "$out" | grep -qF "internal chain of thought"; then
    ok "openai_adapter_reads_content_not_reasoning"
  else
    no "openai_adapter_reads_content_not_reasoning" "code=$code out=$out"
  fi

  # --- openai_adapter_reports_a_cut_off_review ------------------------------
  start_stub_endpoint cutoff cutoff
  out=$(run_openai); code=$?
  kill "$STUB_PID" 2>/dev/null
  if printf '%s' "$out" | grep -qi "cut off\|truncated"; then
    ok "openai_adapter_reports_a_cut_off_review"
  else
    no "openai_adapter_reports_a_cut_off_review" "code=$code out=$out"
  fi

  # --- openai_adapter_reports_truncation ------------------------------------
  # A silently partial review is the worse failure; the limit must be audible.
  start_stub_endpoint trunc normal
  out=$(run_openai R2_OPENAI_MAX_DIFF_BYTES=40); code=$?
  kill "$STUB_PID" 2>/dev/null
  if printf '%s' "$out" | grep -qi "truncat"; then
    ok "openai_adapter_reports_truncation"
  else
    no "openai_adapter_reports_truncation" "code=$code out=$out"
  fi

  # --- openai_adapter_is_unavailable_past_its_time_budget -------------------
  # A pre-push gate that can run for a quarter of an hour is a gate that gets
  # bypassed. The budget is total elapsed time, not socket inactivity: a
  # reasoning model streams nothing while it thinks, so an inactivity timeout
  # never fires and the request runs until something else drops it.
  start_stub_endpoint slow slow
  out=$(run_openai R2_OPENAI_TIMEOUT_SECONDS=1); code=$?
  kill "$STUB_PID" 2>/dev/null
  if [ "$code" -eq 10 ] && printf '%s' "$out" | grep -qi "budget\|timed out"; then
    ok "openai_adapter_is_unavailable_past_its_time_budget"
  else
    no "openai_adapter_is_unavailable_past_its_time_budget" "code=$code out=$out"
  fi

  # --- openai_adapter_is_unavailable_on_unreachable_endpoint ----------------
  out=$(cd "$diff_repo" && R2_BASE=main R2_BRANCH=feature/x R2_RESOLVED_MODEL=stub-model \
    R2_OPENAI_ENDPOINT="http://127.0.0.1:1" bash "$OPENAI_ADAPTER" 2>&1); code=$?
  if [ "$code" -eq 10 ]; then
    ok "openai_adapter_is_unavailable_on_unreachable_endpoint"
  else
    no "openai_adapter_is_unavailable_on_unreachable_endpoint" "code=$code out=$out"
  fi

  # --- openai_adapter_is_unavailable_without_node ---------------------------
  out=$(cd "$diff_repo" && R2_BASE=main R2_BRANCH=feature/x R2_RESOLVED_MODEL=stub-model \
    R2_OPENAI_ENDPOINT="http://127.0.0.1:$STUB_PORT" NODE_BIN="$REPO_SANDBOX/absent-node" \
    bash "$OPENAI_ADAPTER" 2>&1); code=$?
  if [ "$code" -eq 10 ] && printf '%s' "$out" | grep -qi "node"; then
    ok "openai_adapter_is_unavailable_without_node"
  else
    no "openai_adapter_is_unavailable_without_node" "code=$code out=$out"
  fi
fi

# =====================================================================
# setup.sh --reviewer: the machine-global reviewer configuration.
# =====================================================================

SETUP_RUNNER="$REPO_ROOT/scripts/setup.sh"

# --- reviewer_flag_writes_global_scope --------------------------------------
repo="$(new_repo reviewerflag)"
globalcfg3="$REPO_SANDBOX/globalconfig3"
: > "$globalcfg3"
out=$(printf 'codex,openai\nhttps://api.deepseek.com\ndeepseek-v4-flash\nDEEPSEEK_API_KEY\n' \
  | (cd "$repo" && GIT_CONFIG_GLOBAL="$globalcfg3" GH_BIN="$REPO_SANDBOX/absent-gh" \
    CODEX_BIN="$REPO_SANDBOX/absent-codex" bash "$SETUP_RUNNER" --reviewer) 2>&1); code=$?
global_backends="$(git config --file "$globalcfg3" --get r2.backends 2>/dev/null || true)"
local_backends="$(git -C "$repo" config --local --get r2.backends 2>/dev/null || true)"
if [ "$code" -eq 0 ] && [ "$global_backends" = "codex,openai" ] && [ -z "$local_backends" ]; then
  ok "reviewer_flag_writes_global_scope"
else
  no "reviewer_flag_writes_global_scope" "code=$code global=[$global_backends] local=[$local_backends] out=$out"
fi

# --- reviewer_flag_refuses_a_secret_value -----------------------------------
# The setting holds the NAME of the variable carrying the key. Accepting the key
# itself would put a live credential into a file that gets pasted into bug
# reports, so a value that looks like one is refused rather than stored.
repo="$(new_repo reviewersecret)"
globalcfg4="$REPO_SANDBOX/globalconfig4"
: > "$globalcfg4"
out=$(printf 'codex,openai\nhttps://api.deepseek.com\ndeepseek-v4-flash\nsk-abcdef0123456789abcdef0123456789\n' \
  | (cd "$repo" && GIT_CONFIG_GLOBAL="$globalcfg4" GH_BIN="$REPO_SANDBOX/absent-gh" \
    CODEX_BIN="$REPO_SANDBOX/absent-codex" bash "$SETUP_RUNNER" --reviewer) 2>&1); code=$?
stored_key="$(git config --file "$globalcfg4" --get r2.openai.apiKeyEnv 2>/dev/null || true)"
if [ "$code" -ne 0 ] && [ -z "$stored_key" ] && printf '%s' "$out" | grep -qi "name"; then
  ok "reviewer_flag_refuses_a_secret_value"
else
  no "reviewer_flag_refuses_a_secret_value" "code=$code stored=[$stored_key] out=$out"
fi

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
