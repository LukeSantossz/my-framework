#!/usr/bin/env bash
# Activation bootstrap: turns the framework's documented activation steps into
# one idempotent command. Sets core.hooksPath, reports toolchain state
# (advisory), and creates missing triage labels.
# See docs/standards/r2_gate.md and docs/agents/triage-labels.md.
#
# With --statusline it also applies the status line contract to this machine's
# agent configuration, which is the one thing here that writes outside the
# repository; see docs/standards/status_line.md.
set -u

script_dir="$(cd "$(dirname "$0")" && pwd)"

gh_bin="${GH_BIN:-gh}"
codex_bin="${CODEX_BIN:-codex}"
node_bin="${NODE_BIN:-node}"

# Where each agent keeps its configuration. Each tool's own override is
# honored, so a machine that already relocates them is followed rather than
# overruled.
claude_home="${CLAUDE_HOME:-${CLAUDE_CONFIG_DIR:-$HOME/.claude}}"
codex_home="${CODEX_HOME:-$HOME/.codex}"

log() { printf '[setup] %s\n' "$1"; }

# Optional modes; anything else is a usage error. Parsed in full before any
# work, so a run with a bad flag changes nothing.
interactive=0
statusline=0
reviewer=0
for arg in "$@"; do
  case "$arg" in
    --interactive) interactive=1 ;;
    --statusline) statusline=1 ;;
    --reviewer) reviewer=1 ;;
    *)
      log "unknown option: $arg (supported: --interactive, --statusline, --reviewer)"
      exit 1
      ;;
  esac
done

# Canonical triage labels (docs/agents/triage-labels.md): name|color|description.
LABEL_SPECS='needs-triage|ededed|Maintainer needs to evaluate this issue
needs-info|d876e3|Waiting on reporter for more information
ready-for-agent|0e8a16|Fully specified, ready for an AFK agent
ready-for-human|1d76db|Requires human implementation
wontfix|ffffff|Will not be actioned'

# The status line contract (docs/standards/status_line.md) in Codex's own
# vocabulary: the five contract facts, in contract order.
CODEX_STATUS_LINE='status_line = ["model-with-reasoning", "context-used", "context-window-size", "used-tokens", "five-hour-limit", "weekly-limit", "current-dir", "git-branch"]'
CODEX_STATUS_COLORS='status_line_use_colors = true'
RENDERER_NAME='my-framework-statusline.js'

# Windows-native form of a path, for values a Windows-native tool will read
# back. Claude Code resolves the statusLine command itself, so a POSIX path
# from this shell would not be one it can run.
native_path() {
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$1"
  else
    printf '%s' "$1"
  fi
}

# Rewrites the `[tui]` status line keys in place, preserving everything else in
# the file: other keys in the section, the `[tui.*]` subsections, and every
# unrelated section. A `[tui]` section that does not exist is appended.
codex_config_with_contract() {
  awk -v line="$CODEX_STATUS_LINE" -v colors="$CODEX_STATUS_COLORS" '
    BEGIN { in_tui = 0; skipping = 0; placed = 0 }
    # Consume the tail of a multi-line status_line array, which would
    # otherwise leave its entries and its closing bracket behind.
    { if (skipping) { if (index($0, "]") > 0) skipping = 0; next } }
    /^[ \t]*\[/ {
      header = $0
      sub(/^[ \t]+/, "", header); sub(/[ \t]+$/, "", header)
      print
      if (header == "[tui]") { print line; print colors; in_tui = 1; placed = 1 }
      else { in_tui = 0 }
      next
    }
    {
      if (in_tui && $0 ~ /^[ \t]*status_line[ \t]*=/) {
        if (index($0, "[") > 0 && index($0, "]") == 0) skipping = 1
        next
      }
      if (in_tui && $0 ~ /^[ \t]*status_line_use_colors[ \t]*=/) next
      print
    }
    END { if (!placed) { print ""; print "[tui]"; print line; print colors } }
  ' "$1"
}

apply_codex_status_line() {
  target="$codex_home/config.toml"
  if ! mkdir -p "$codex_home"; then
    log "codex: cannot create $codex_home; status line not applied."
    return 1
  fi
  tmp="$target.tmp.$$"
  if [ -f "$target" ]; then
    codex_config_with_contract "$target" > "$tmp" || { rm -f "$tmp"; return 1; }
  else
    printf '[tui]\n%s\n%s\n' "$CODEX_STATUS_LINE" "$CODEX_STATUS_COLORS" > "$tmp" || {
      rm -f "$tmp"; return 1
    }
  fi

  if [ -f "$target" ] && cmp -s "$target" "$tmp"; then
    rm -f "$tmp"
    log "codex status line: already canonical."
    return 0
  fi

  # Replacing a divergent config is the point (a machine already configured is
  # the one that needed standardizing), so the original is kept beside it.
  if [ -f "$target" ]; then
    backup="$target.bak.$(date +%Y%m%d%H%M%S)"
    if ! cp "$target" "$backup"; then
      rm -f "$tmp"
      log "codex: cannot back up $target; status line not applied."
      return 1
    fi
    log "codex config backed up -> $(basename "$backup")"
  fi
  if ! mv "$tmp" "$target"; then
    rm -f "$tmp"
    log "codex: cannot write $target; status line not applied."
    return 1
  fi
  log "codex status line: applied to $target."
  return 0
}

# Merges the statusLine key into settings.json, leaving every other key alone.
# Node does the merge because the file is JSON the user owns: a shell rewrite
# would have to reproduce it, and reproducing it wrongly loses settings.
apply_claude_status_line() {
  if ! command -v "$node_bin" >/dev/null 2>&1; then
    log "node: not installed; Claude Code status line skipped (the renderer and the settings merge both need it)."
    return 0
  fi
  if ! mkdir -p "$claude_home"; then
    log "claude: cannot create $claude_home; status line not applied."
    return 1
  fi

  # Installed under a framework-scoped name so it never collides with a
  # hand-written statusline script already in the user's CLAUDE_HOME.
  source_renderer="$script_dir/statusline/claude-statusline.js"
  if [ ! -f "$source_renderer" ]; then
    log "claude: renderer missing at $source_renderer; status line not applied."
    return 1
  fi

  installed="$claude_home/$RENDERER_NAME"
  if ! cmp -s "$source_renderer" "$installed" 2>/dev/null; then
    if ! cp "$source_renderer" "$installed"; then
      log "claude: cannot install the renderer into $claude_home; status line not applied."
      return 1
    fi
    log "claude renderer: installed -> $installed"
  fi

  settings="$claude_home/settings.json"
  command_value="node \"$(native_path "$installed")\""
  "$node_bin" -e '
    const fs = require("fs");
    const [file, backup, command] = process.argv.slice(1);
    let settings = {};
    let original = null;
    if (fs.existsSync(file)) {
      original = fs.readFileSync(file, "utf8");
      try { settings = JSON.parse(original); } catch (e) { process.exit(2); }
      if (settings === null || typeof settings !== "object" || Array.isArray(settings)) process.exit(2);
    }
    const current = settings.statusLine;
    if (current && current.type === "command" && current.command === command) process.exit(3);
    if (original !== null) fs.writeFileSync(backup, original);
    settings.statusLine = { type: "command", command };
    fs.writeFileSync(file, JSON.stringify(settings, null, 2) + "\n");
  ' "$settings" "$settings.bak.$(date +%Y%m%d%H%M%S)" "$command_value"
  case $? in
    0) log "claude status line: applied to $settings." ;;
    3) log "claude status line: already canonical." ;;
    2)
      log "claude: $settings is not a JSON object; refusing to rewrite it (fix or move it, then re-run). Status line not applied."
      return 1
      ;;
    *)
      log "claude: failed to write $settings; status line not applied."
      return 1
      ;;
  esac
  return 0
}

# 1. Must run inside a git repository; the only hard requirement.
repo_root="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  log "not inside a git repository; nothing to activate."
  exit 1
}
log "repo: $repo_root"

# 2. Activate the R2 pre-push gate (idempotent local setting). This is the
#    activation itself: if it cannot be set, the bootstrap failed.
if ! git config core.hooksPath .githooks; then
  log "failed to set core.hooksPath (is .git/config writable?); activation incomplete."
  exit 1
fi
log "core.hooksPath -> .githooks (R2 pre-push gate active)."

# 3. Toolchain report. Advisory: absence never fails the bootstrap, matching
#    the R2 gate's own skip-with-message behavior.
if command -v "$codex_bin" >/dev/null 2>&1; then
  log "codex: found."
else
  log "codex: not installed; R2 reviews will be skipped until it is (see docs/standards/r2_gate.md)."
fi

labels_ok=1
if ! command -v "$gh_bin" >/dev/null 2>&1; then
  log "gh: not installed; skipping triage-label creation."
  labels_ok=0
elif ! "$gh_bin" auth status >/dev/null 2>&1; then
  log "gh: not authenticated (run 'gh auth login'); skipping triage-label creation."
  labels_ok=0
fi

# 4. Create the triage labels that are missing from the tracker.
# A failed listing must not read as an empty label set: creating against an
# unknown set misreports existing labels as creation failures.
if [ "$labels_ok" -eq 1 ] && ! existing="$("$gh_bin" label list --limit 500 --json name --jq '.[].name')"; then
  log "gh: could not list labels (older gh, missing repo permission, or network error); skipping triage-label creation."
  labels_ok=0
fi

# The loop reads from a heredoc (not a pipe) so label_failures survives it:
# a failed create must fail the bootstrap, not end in a success report.
if [ "$labels_ok" -eq 1 ]; then
  label_failures=0
  while IFS='|' read -r name color desc; do
    if printf '%s\n' "$existing" | grep -qx "$name"; then
      log "label '$name': present."
    elif "$gh_bin" label create "$name" --color "$color" --description "$desc" >/dev/null 2>&1; then
      log "label '$name': created."
    else
      log "label '$name': create failed (check gh permissions for this repo)."
      label_failures=$((label_failures + 1))
    fi
  done <<EOF
$LABEL_SPECS
EOF
  if [ "$label_failures" -gt 0 ]; then
    log "activation bootstrap incomplete: $label_failures label(s) could not be created."
    exit 1
  fi
fi

# 5. Apply the status line contract to this machine. Opt-in, because this is
#    the only step that writes outside the repository and so governs every
#    other project on the machine (docs/standards/status_line.md).
if [ "$statusline" -eq 1 ]; then
  statusline_failures=0
  apply_codex_status_line || statusline_failures=$((statusline_failures + 1))
  apply_claude_status_line || statusline_failures=$((statusline_failures + 1))
  if [ "$statusline_failures" -gt 0 ]; then
    log "activation bootstrap incomplete: the status line could not be applied."
    exit 1
  fi
fi

# 6. Machine-global reviewer configuration for the R2 gate. This writes the
#    --global scope on purpose: it is the layer a repository can still override
#    with --local, which is the authority order code_conventions.md states.
#    --interactive keeps its own repo-local scope, so each flag owns one.
if [ "$reviewer" -eq 1 ]; then
  current_backends="$(git config --global --get r2.backends 2>/dev/null || true)"
  current_endpoint="$(git config --global --get r2.openai.endpoint 2>/dev/null || true)"
  current_openai_model="$(git config --global --get r2.openai.model 2>/dev/null || true)"
  current_key_env="$(git config --global --get r2.openai.apiKeyEnv 2>/dev/null || true)"

  printf '[setup] R2 backend chain, in order [%s]: ' "${current_backends:-codex}"
  IFS= read -r answer_backends || answer_backends=""
  printf '[setup] openai-compatible endpoint [%s]: ' "${current_endpoint:-<none>}"
  IFS= read -r answer_endpoint || answer_endpoint=""
  printf '[setup] openai-compatible model [%s]: ' "${current_openai_model:-<none>}"
  IFS= read -r answer_openai_model || answer_openai_model=""
  printf '[setup] name of the env var holding the API key [%s]: ' "${current_key_env:-<none>}"
  IFS= read -r answer_key_env || answer_key_env=""

  # The setting holds the NAME of the variable, never the key. `git config
  # --list` output ends up in bug reports and screenshots, so a value that is
  # not a valid environment variable name is refused rather than stored: the
  # shapes that fail here are exactly the shapes a real credential has.
  if [ -n "$answer_key_env" ] && ! printf '%s' "$answer_key_env" | grep -qE '^[A-Za-z_][A-Za-z0-9_]*$'; then
    log "that is not an environment variable name. Give the NAME of the variable holding the key (for example DEEPSEEK_API_KEY), never the key itself. Nothing was persisted."
    exit 1
  fi

  persist_global() {
    [ -n "$2" ] || return 0
    if ! git config --global "$1" "$2"; then
      log "failed to persist $1 (is the global git config writable?); activation incomplete."
      return 1
    fi
    log "reviewer config: $1 = $2"
    return 0
  }

  reviewer_failures=0
  persist_global r2.backends "$answer_backends" || reviewer_failures=1
  persist_global r2.openai.endpoint "$answer_endpoint" || reviewer_failures=1
  persist_global r2.openai.model "$answer_openai_model" || reviewer_failures=1
  persist_global r2.openai.apiKeyEnv "$answer_key_env" || reviewer_failures=1
  if [ "$reviewer_failures" -ne 0 ]; then
    exit 1
  fi
  log "reviewer chain: $(git config --global --get r2.backends 2>/dev/null || echo codex) (see docs/standards/r2_gate.md)."
fi

# Interactive adoption questionnaire. Enter (or EOF) keeps the current
# default and writes nothing, so script defaults can evolve.
if [ "$interactive" -eq 1 ]; then
  current_model="$(git config --local codexreview.model 2>/dev/null || true)"
  current_effort="$(git config --local codexreview.effort 2>/dev/null || true)"
  printf '[setup] R2 reviewer model [%s]: ' "${current_model:-gpt-5.6-terra}"
  IFS= read -r answer_model || answer_model=""
  if [ -n "$answer_model" ] && ! git config --local codexreview.model "$answer_model"; then
    log "failed to persist codexreview.model (is .git/config writable?); activation incomplete."
    exit 1
  fi
  printf '[setup] R2 reasoning effort [%s]: ' "${current_effort:-high}"
  IFS= read -r answer_effort || answer_effort=""
  if [ -n "$answer_effort" ] && ! git config --local codexreview.effort "$answer_effort"; then
    log "failed to persist codexreview.effort (is .git/config writable?); activation incomplete."
    exit 1
  fi
  printf '[setup] token economy (caveman/terse/off) [terse]: '
  IFS= read -r answer_economy || answer_economy=""
  resolved_model="${answer_model:-${current_model:-gpt-5.6-terra}}"
  resolved_effort="${answer_effort:-${current_effort:-high}}"
  log "choices: reviewer=$resolved_model effort=$resolved_effort token-economy=${answer_economy:-terse} (see docs/standards/skills_guidelines.md)."
fi

log "activation bootstrap complete."
exit 0
