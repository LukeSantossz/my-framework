#!/usr/bin/env bash
# Tests for the status line contract: the `scripts/setup.sh --statusline` path
# and the Claude Code renderer it installs.
# Each test maps to an Acceptance Criterion in
# docs/specs/0012-standardize-agent-status-line.md.
set -u

# Isolate git config lookups from this machine's global/system scope, and pin
# CLAUDE_HOME/CODEX_HOME per test so nothing here reads or writes the
# operator's real agent configuration.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
# No network calls from the renderer under test.
export MYFW_STATUSLINE_NO_REFRESH=1
# Assertions match plain text, not escape sequences.
export NO_COLOR=1

TEST_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$TEST_DIR/../.." && pwd)"
RUNNER="$REPO_ROOT/scripts/setup.sh"
RENDERER="$REPO_ROOT/scripts/statusline/claude-statusline.js"

PASS=0
FAIL=0

ok() { PASS=$((PASS + 1)); printf 'ok   - %s\n' "$1"; }
no() { FAIL=$((FAIL + 1)); printf 'FAIL - %s\n' "$1"; printf '       %s\n' "$2"; }

SANDBOX="$(mktemp -d)"
trap 'rm -rf "$SANDBOX"' EXIT

# The bootstrap's own dependencies are pointed at absent binaries so these
# tests exercise the status line path and nothing else: the label step skips
# itself when `gh` is missing, which is already covered by setup.test.sh.
export GH_BIN="$SANDBOX/absent-gh" CODEX_BIN="$SANDBOX/absent-codex"

# The canonical Codex segment list, in contract order. Written out literally
# rather than sourced from the script, so a silent reordering in the
# implementation fails here instead of agreeing with itself.
CODEX_SEGMENTS='status_line = ["model-with-reasoning", "context-used", "context-window-size", "used-tokens", "five-hour-limit", "weekly-limit", "current-dir", "git-branch"]'

n=0
new_case() {
  n=$((n + 1))
  CASE_DIR="$SANDBOX/case-$n"
  REPO="$CASE_DIR/repo"
  CLAUDE_DIR="$CASE_DIR/claude"
  CODEX_DIR="$CASE_DIR/codex"
  mkdir -p "$CASE_DIR"
  git init -q "$REPO"
}

# Runs the bootstrap inside this case's repo with this case's sandboxed homes.
run_setup() {
  (cd "$REPO" && CLAUDE_HOME="$CLAUDE_DIR" CODEX_HOME="$CODEX_DIR" bash "$RUNNER" "$@" 2>&1)
}

count_files() { ls -1 "$1" 2>/dev/null | wc -l | tr -d ' '; }

# Paths handed to Node have to be ones a Windows-native Node can resolve, the
# same way a real session's payload carries native paths. Mixed form (forward
# slashes) is both a valid Windows path and a valid JSON string body.
native() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s' "$1"; fi
}

# --- setup_without_flag_leaves_global_state_untouched ------------------------
new_case
out=$(run_setup); code=$?
if [ "$code" -eq 0 ] && [ ! -e "$CLAUDE_DIR" ] && [ ! -e "$CODEX_DIR" ]; then
  ok "setup_without_flag_leaves_global_state_untouched"
else
  no "setup_without_flag_leaves_global_state_untouched" \
    "code=$code claude_exists=$([ -e "$CLAUDE_DIR" ] && echo yes || echo no) codex_exists=$([ -e "$CODEX_DIR" ] && echo yes || echo no) out=$out"
fi

# --- statusline_writes_codex_contract ---------------------------------------
new_case
out=$(run_setup --statusline); code=$?
toml="$CODEX_DIR/config.toml"
if [ "$code" -eq 0 ] && [ -f "$toml" ] \
  && grep -qF "$CODEX_SEGMENTS" "$toml" \
  && grep -qE '^status_line_use_colors = true$' "$toml" \
  && grep -qE '^\[tui\]$' "$toml"; then
  ok "statusline_writes_codex_contract"
else
  no "statusline_writes_codex_contract" "code=$code toml=$(cat "$toml" 2>&1)"
fi

# --- statusline_replaces_divergent_codex_segments ---------------------------
# A realistic pre-existing config: a divergent status_line, an unrelated key in
# [tui], a [tui.*] subsection that must not be mistaken for [tui], and
# unrelated sections on both sides.
new_case
mkdir -p "$CODEX_DIR"
cat > "$CODEX_DIR/config.toml" <<'TOML'
model = "gpt-5.6-terra"

[features]
memories = true

[tui]
status_line = ["model", "current-dir"]
theme = "monokai-extended-origin"
status_line_use_colors = false

[tui.model_availability_nux]
"gpt-5.5" = 4

[projects.'c:\users\lucas']
trust_level = "trusted"
TOML
out=$(run_setup --statusline); code=$?
toml="$CODEX_DIR/config.toml"
missing=""
grep -qF "$CODEX_SEGMENTS" "$toml" || missing="$missing canonical_segments"
grep -qF '"model", "current-dir"' "$toml" && missing="$missing old_segments_survived"
grep -qE '^status_line_use_colors = true$' "$toml" || missing="$missing colors_not_enabled"
grep -qE '^status_line_use_colors = false$' "$toml" && missing="$missing old_colors_survived"
grep -qF 'theme = "monokai-extended-origin"' "$toml" || missing="$missing tui_theme_lost"
grep -qF '[tui.model_availability_nux]' "$toml" || missing="$missing subsection_lost"
grep -qF '"gpt-5.5" = 4' "$toml" || missing="$missing subsection_body_lost"
grep -qF '[features]' "$toml" || missing="$missing features_lost"
grep -qF 'trust_level = "trusted"' "$toml" || missing="$missing projects_lost"
grep -qF 'model = "gpt-5.6-terra"' "$toml" || missing="$missing preamble_lost"
if [ "$code" -eq 0 ] && [ -z "$missing" ]; then
  ok "statusline_replaces_divergent_codex_segments"
else
  no "statusline_replaces_divergent_codex_segments" "code=$code missing:$missing toml=$(cat "$toml" 2>&1)"
fi

# --- statusline_backs_up_replaced_codex_config ------------------------------
# Also covers the multi-line array shape, which a line-oriented rewrite has to
# consume in full or leave a stray "]" behind.
new_case
mkdir -p "$CODEX_DIR"
cat > "$CODEX_DIR/config.toml" <<'TOML'
[tui]
status_line = [
  "model",
  "current-dir",
]
theme = "dark"
TOML
original="$(cat "$CODEX_DIR/config.toml")"
out=$(run_setup --statusline); code=$?
toml="$CODEX_DIR/config.toml"
backup="$(ls -1 "$CODEX_DIR"/config.toml.bak.* 2>/dev/null | head -1)"
backup_missing=""
[ -n "$backup" ] || backup_missing="no_backup_written"
[ -n "$backup" ] && [ "$(cat "$backup")" = "$original" ] || backup_missing="$backup_missing backup_not_identical"
# A remnant is an entry left on its own line; the canonical one-line array
# legitimately contains "current-dir", so the shape is what distinguishes them.
grep -qE '^[[:space:]]+"current-dir",[[:space:]]*$' "$toml" && backup_missing="$backup_missing multiline_remnant"
grep -qE '^\]$' "$toml" && backup_missing="$backup_missing dangling_bracket"
grep -qF 'theme = "dark"' "$toml" || backup_missing="$backup_missing theme_lost"
grep -qF "$CODEX_SEGMENTS" "$toml" || backup_missing="$backup_missing canonical_segments"
if [ "$code" -eq 0 ] && [ -z "$backup_missing" ]; then
  ok "statusline_backs_up_replaced_codex_config"
else
  no "statusline_backs_up_replaced_codex_config" "code=$code missing:$backup_missing toml=$(cat "$toml" 2>&1)"
fi

# --- statusline_writes_claude_contract --------------------------------------
new_case
out=$(run_setup --statusline); code=$?
settings="$CLAUDE_DIR/settings.json"
installed="$CLAUDE_DIR/my-framework-statusline.js"
if [ "$code" -eq 0 ] && [ -f "$settings" ] \
  && grep -q '"statusLine"' "$settings" \
  && grep -q '"type": "command"' "$settings" \
  && grep -qF "my-framework-statusline.js" "$settings" \
  && [ -f "$installed" ]; then
  ok "statusline_writes_claude_contract"
else
  no "statusline_writes_claude_contract" "code=$code settings=$(cat "$settings" 2>&1)"
fi

# --- statusline_installs_renderer_into_claude_home --------------------------
if cmp -s "$RENDERER" "$CLAUDE_DIR/my-framework-statusline.js"; then
  ok "statusline_installs_renderer_into_claude_home"
else
  no "statusline_installs_renderer_into_claude_home" "installed copy differs from $RENDERER"
fi

# --- statusline_merges_into_existing_claude_settings ------------------------
new_case
mkdir -p "$CLAUDE_DIR"
cat > "$CLAUDE_DIR/settings.json" <<'JSON'
{
  "model": "opus[1m]",
  "theme": "dark",
  "statusLine": {
    "type": "command",
    "command": "node /somewhere/else/personal.js"
  },
  "permissions": {
    "defaultMode": "auto"
  }
}
JSON
original="$(cat "$CLAUDE_DIR/settings.json")"
out=$(run_setup --statusline); code=$?
settings="$CLAUDE_DIR/settings.json"
backup="$(ls -1 "$CLAUDE_DIR"/settings.json.bak.* 2>/dev/null | head -1)"
merge_missing=""
grep -qF 'my-framework-statusline.js' "$settings" || merge_missing="$merge_missing statusline_not_applied"
grep -qF 'personal.js' "$settings" && merge_missing="$merge_missing old_statusline_survived"
grep -qF '"opus[1m]"' "$settings" || merge_missing="$merge_missing model_key_lost"
grep -qF '"theme"' "$settings" || merge_missing="$merge_missing theme_key_lost"
grep -qF '"defaultMode"' "$settings" || merge_missing="$merge_missing nested_key_lost"
[ -n "$backup" ] || merge_missing="$merge_missing no_backup_written"
[ -n "$backup" ] && [ "$(cat "$backup")" = "$original" ] || merge_missing="$merge_missing backup_not_identical"
if [ "$code" -eq 0 ] && [ -z "$merge_missing" ]; then
  ok "statusline_merges_into_existing_claude_settings"
else
  no "statusline_merges_into_existing_claude_settings" "code=$code missing:$merge_missing settings=$(cat "$settings" 2>&1)"
fi

# --- statusline_is_idempotent -----------------------------------------------
# The second run must find both files already conformant and write nothing —
# no rewrite, and above all no second backup, which is how a re-run of the
# bootstrap would otherwise bury the original config under generated copies.
new_case
out=$(run_setup --statusline); code=$?
toml_first="$(cat "$CODEX_DIR/config.toml")"
settings_first="$(cat "$CLAUDE_DIR/settings.json")"
backups_first=$(( $(count_files "$CODEX_DIR"/config.toml.bak.*) + $(count_files "$CLAUDE_DIR"/settings.json.bak.*) ))
out2=$(run_setup --statusline); code2=$?
toml_second="$(cat "$CODEX_DIR/config.toml")"
settings_second="$(cat "$CLAUDE_DIR/settings.json")"
backups_second=$(( $(count_files "$CODEX_DIR"/config.toml.bak.*) + $(count_files "$CLAUDE_DIR"/settings.json.bak.*) ))
if [ "$code" -eq 0 ] && [ "$code2" -eq 0 ] \
  && [ "$toml_first" = "$toml_second" ] && [ "$settings_first" = "$settings_second" ] \
  && [ "$backups_first" -eq 0 ] && [ "$backups_second" -eq 0 ]; then
  ok "statusline_is_idempotent"
else
  no "statusline_is_idempotent" \
    "code=$code code2=$code2 toml_changed=$([ "$toml_first" = "$toml_second" ] && echo no || echo yes) settings_changed=$([ "$settings_first" = "$settings_second" ] && echo no || echo yes) backups=$backups_first/$backups_second"
fi

# --- statusline_skips_claude_side_without_node ------------------------------
# Node is what merges settings.json and what runs the renderer; without it the
# Claude side cannot be applied. That is advisory, like the absent Codex CLI:
# the Codex side still lands and the bootstrap still succeeds.
new_case
out=$(cd "$REPO" && CLAUDE_HOME="$CLAUDE_DIR" CODEX_HOME="$CODEX_DIR" \
  NODE_BIN="$SANDBOX/absent-node" bash "$RUNNER" --statusline 2>&1); code=$?
if [ "$code" -eq 0 ] \
  && printf '%s' "$out" | grep -qi "node" \
  && [ ! -f "$CLAUDE_DIR/settings.json" ] \
  && grep -qF "$CODEX_SEGMENTS" "$CODEX_DIR/config.toml"; then
  ok "statusline_skips_claude_side_without_node"
else
  no "statusline_skips_claude_side_without_node" "code=$code out=$out"
fi

# --- statusline_fails_on_unreadable_claude_settings -------------------------
# Rewriting a settings.json that cannot be parsed would silently discard every
# key in it, so the run stops instead and leaves the file alone.
new_case
mkdir -p "$CLAUDE_DIR"
printf '{ "model": "opus", oops }\n' > "$CLAUDE_DIR/settings.json"
original="$(cat "$CLAUDE_DIR/settings.json")"
out=$(run_setup --statusline); code=$?
if [ "$code" -ne 0 ] && [ "$(cat "$CLAUDE_DIR/settings.json")" = "$original" ]; then
  ok "statusline_fails_on_unreadable_claude_settings"
else
  no "statusline_fails_on_unreadable_claude_settings" "code=$code settings=$(cat "$CLAUDE_DIR/settings.json")"
fi

# --- statusline_composes_with_interactive -----------------------------------
new_case
out=$(printf '\n\n\n' | (cd "$REPO" && CLAUDE_HOME="$CLAUDE_DIR" CODEX_HOME="$CODEX_DIR" \
  bash "$RUNNER" --statusline --interactive) 2>&1); code=$?
new_case
out2=$(printf '\n\n\n' | (cd "$REPO" && CLAUDE_HOME="$CLAUDE_DIR" CODEX_HOME="$CODEX_DIR" \
  bash "$RUNNER" --interactive --statusline) 2>&1); code2=$?
new_case
out3=$(run_setup --statusline --bogus); code3=$?
if [ "$code" -eq 0 ] && [ "$code2" -eq 0 ] && [ "$code3" -ne 0 ] \
  && grep -qF "$CODEX_SEGMENTS" "$CODEX_DIR/config.toml" 2>/dev/null; then
  no "statusline_composes_with_interactive" "the rejected run still wrote a config"
elif [ "$code" -eq 0 ] && [ "$code2" -eq 0 ] && [ "$code3" -ne 0 ]; then
  ok "statusline_composes_with_interactive"
else
  no "statusline_composes_with_interactive" "code=$code code2=$code2 code3=$code3 out3=$out3"
fi

# --- renderer_emits_every_contract_segment ----------------------------------
# Node is required for the two renderer criteria; without it they are reported
# as skipped rather than silently passing.
node_bin="$(command -v node || true)"
if [ -z "$node_bin" ]; then
  printf 'skip - renderer_emits_every_contract_segment (node not installed)\n'
  printf 'skip - renderer_degrades_without_usage_cache (node not installed)\n'
else
  new_case
  transcript="$CASE_DIR/transcript.jsonl"
  # Two main-chain assistant turns plus one sidechain turn that must not be
  # counted, since a subagent's usage is not this session's context.
  cat > "$transcript" <<'JSONL'
{"type":"assistant","message":{"usage":{"input_tokens":1000,"output_tokens":500,"cache_creation_input_tokens":2000,"cache_read_input_tokens":100000}}}
{"type":"assistant","isSidechain":true,"message":{"usage":{"input_tokens":9000,"output_tokens":9000,"cache_creation_input_tokens":9000,"cache_read_input_tokens":9000}}}
{"type":"assistant","message":{"usage":{"input_tokens":1200,"output_tokens":800,"cache_creation_input_tokens":3000,"cache_read_input_tokens":300000}}}
JSONL
  branch="$(git -C "$REPO" symbolic-ref --short HEAD 2>/dev/null || echo HEAD)"
  payload="$(printf '{"model":{"id":"claude-opus-5[1m]","display_name":"Opus 5 (1M context)"},"transcript_path":"%s","cwd":"%s","workspace":{"current_dir":"%s"},"version":"2.1.161"}' \
    "$(native "$transcript")" "$(native "$REPO")" "$(native "$REPO")")"
  mkdir -p "$CLAUDE_DIR"
  printf '{"effortLevel": "high"}\n' > "$CLAUDE_DIR/settings.json"
  line=$(printf '%s' "$payload" | CLAUDE_HOME="$CLAUDE_DIR" node "$RENDERER" 2>&1)

  # Contract facts: model+effort, context (304.2k of the 1M window = 30%),
  # tokens spent (1000+500+2000 + 1200+800+3000 = 8.5k, sidechain excluded),
  # quota, and location.
  seg_missing=""
  printf '%s' "$line" | grep -qF "Opus 5" || seg_missing="$seg_missing model"
  printf '%s' "$line" | grep -qF "high" || seg_missing="$seg_missing effort"
  printf '%s' "$line" | grep -qF "30%" || seg_missing="$seg_missing context_pct"
  printf '%s' "$line" | grep -qF "304.2k/1M" || seg_missing="$seg_missing context_window"
  printf '%s' "$line" | grep -qF "8.5k tok" || seg_missing="$seg_missing spent_tokens"
  printf '%s' "$line" | grep -qF "usage" || seg_missing="$seg_missing quota"
  printf '%s' "$line" | grep -qF "repo:$branch" || seg_missing="$seg_missing location"
  # Contract order: model, context, tokens, quota, location.
  order_ok=$(printf '%s' "$line" | grep -cE 'Opus 5.*ctx.*tok.*usage.*repo:')
  [ "$order_ok" = "1" ] || seg_missing="$seg_missing order"
  if [ -z "$seg_missing" ]; then
    ok "renderer_emits_every_contract_segment"
  else
    no "renderer_emits_every_contract_segment" "missing:$seg_missing line=$line"
  fi

  # --- renderer_degrades_without_usage_cache --------------------------------
  # No cache file and no credentials: the quota segment alone degrades, and the
  # renderer still emits a line rather than failing the status bar.
  new_case
  mkdir -p "$CLAUDE_DIR"
  payload='{"model":{"id":"claude-opus-5","display_name":"Opus 5"},"version":"2.1.161"}'
  line=$(printf '%s' "$payload" | CLAUDE_HOME="$CLAUDE_DIR" node "$RENDERER" 2>&1); rcode=$?
  if [ "$rcode" -eq 0 ] \
    && printf '%s' "$line" | grep -qF "Opus 5" \
    && printf '%s' "$line" | grep -qF "ctx" \
    && printf '%s' "$line" | grep -qF "tok" \
    && printf '%s' "$line" | grep -qF "usage n/a" \
    && [ ! -f "$CLAUDE_DIR/.usage-cache.json" ]; then
    ok "renderer_degrades_without_usage_cache"
  else
    no "renderer_degrades_without_usage_cache" "code=$rcode line=$line"
  fi
fi

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
