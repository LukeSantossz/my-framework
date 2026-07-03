#!/usr/bin/env bash
# Activation bootstrap: turns the framework's documented activation steps into
# one idempotent command. Sets core.hooksPath, reports toolchain state
# (advisory), and creates missing triage labels.
# See docs/standards/codex_review.md and docs/agents/triage-labels.md.
set -u

gh_bin="${GH_BIN:-gh}"
codex_bin="${CODEX_BIN:-codex}"

log() { printf '[setup] %s\n' "$1"; }

# Optional interactive adoption mode; anything else is a usage error.
interactive=0
if [ "${1:-}" = "--interactive" ]; then
  interactive=1
elif [ -n "${1:-}" ]; then
  log "unknown option: $1 (supported: --interactive)"
  exit 1
fi

# Canonical triage labels (docs/agents/triage-labels.md): name|color|description.
LABEL_SPECS='needs-triage|ededed|Maintainer needs to evaluate this issue
needs-info|d876e3|Waiting on reporter for more information
ready-for-agent|0e8a16|Fully specified, ready for an AFK agent
ready-for-human|1d76db|Requires human implementation
wontfix|ffffff|Will not be actioned'

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
  log "codex: not installed; R2 reviews will be skipped until it is (see docs/standards/codex_review.md)."
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

# Interactive adoption questionnaire. Enter (or EOF) keeps the current
# default and writes nothing, so script defaults can evolve.
if [ "$interactive" -eq 1 ]; then
  current_model="$(git config codexreview.model 2>/dev/null || true)"
  current_effort="$(git config codexreview.effort 2>/dev/null || true)"
  printf '[setup] R2 reviewer model [%s]: ' "${current_model:-gpt-5.5}"
  IFS= read -r answer_model || answer_model=""
  [ -n "$answer_model" ] && git config codexreview.model "$answer_model"
  printf '[setup] R2 reasoning effort [%s]: ' "${current_effort:-high}"
  IFS= read -r answer_effort || answer_effort=""
  [ -n "$answer_effort" ] && git config codexreview.effort "$answer_effort"
  printf '[setup] token economy (caveman/terse/off) [terse]: '
  IFS= read -r answer_economy || answer_economy=""
  resolved_model="${answer_model:-${current_model:-gpt-5.5}}"
  resolved_effort="${answer_effort:-${current_effort:-high}}"
  log "choices: reviewer=$resolved_model effort=$resolved_effort token-economy=${answer_economy:-terse} (see docs/standards/skills_guidelines.md)."
fi

log "activation bootstrap complete."
exit 0
