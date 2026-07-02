#!/usr/bin/env bash
# Activation bootstrap: turns the framework's documented activation steps into
# one idempotent command. Sets core.hooksPath, reports toolchain state
# (advisory), and creates missing triage labels.
# See docs/standards/codex_review.md and docs/agents/triage-labels.md.
set -u

gh_bin="${GH_BIN:-gh}"
codex_bin="${CODEX_BIN:-codex}"

log() { printf '[setup] %s\n' "$1"; }

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

# 2. Activate the R2 pre-push gate (idempotent local setting).
git config core.hooksPath .githooks
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
if [ "$labels_ok" -eq 1 ]; then
  existing="$("$gh_bin" label list --limit 500 --json name --jq '.[].name' 2>/dev/null)"
  printf '%s\n' "$LABEL_SPECS" | while IFS='|' read -r name color desc; do
    if printf '%s\n' "$existing" | grep -qx "$name"; then
      log "label '$name': present."
    elif "$gh_bin" label create "$name" --color "$color" --description "$desc" >/dev/null 2>&1; then
      log "label '$name': created."
    else
      log "label '$name': create failed (check gh permissions for this repo)."
    fi
  done
fi

log "activation bootstrap complete."
exit 0
