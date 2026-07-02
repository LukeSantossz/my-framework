#!/usr/bin/env bash
# R2 cross-provider review runner: invokes Codex CLI as the Reviewer model
# (provider != Author) over the current branch, per docs/standards/ai_guidelines.md.
# Called by the pre-push hook; also runnable by hand. See docs/standards/codex_review.md.
set -u

# Reviewer identity: env override > git config (persisted by
# `setup.sh --interactive`) > default, per docs/standards/codex_review.md.
# An unset git config key is not an error; outside a repo the lookup is empty.
config_model="$(git config codexreview.model 2>/dev/null || true)"
config_effort="$(git config codexreview.effort 2>/dev/null || true)"
REVIEWER_MODEL="${CODEX_REVIEW_MODEL:-${config_model:-gpt-5.5}}"
REVIEWER_EFFORT="${CODEX_REVIEW_EFFORT:-${config_effort:-high}}"

# All overridable for testing and per-repo use.
base="${CODEX_REVIEW_BASE:-main}"
branch="${CODEX_REVIEW_BRANCH:-$(git rev-parse --abbrev-ref HEAD 2>/dev/null)}"
codex_bin="${CODEX_BIN:-codex}"

log() { printf '[codex-review] %s\n' "$1"; }

# 1. Explicit bypass.
if [ "${SKIP_CODEX_REVIEW:-}" = "1" ]; then
  log "SKIP_CODEX_REVIEW set; skipping R2 gate."
  exit 0
fi

# 2. Nothing to review when the branch is its own base.
if [ "$branch" = "$base" ]; then
  log "On base branch '$base'; nothing to review against itself. Skipping R2."
  exit 0
fi

# 3. Dry-run prints the command and exits, regardless of Codex availability,
#    so the documented CODEX_REVIEW_DRYRUN contract holds without Codex installed.
if [ "${CODEX_REVIEW_DRYRUN:-}" = "1" ]; then
  printf '%s\n' "codex review --base $base -c model=\"$REVIEWER_MODEL\" -c model_reasoning_effort=\"$REVIEWER_EFFORT\""
  exit 0
fi

# 4. Codex is optional tooling: absence must not block the push.
if ! command -v "$codex_bin" >/dev/null 2>&1; then
  log "Codex not installed; skipping R2 (cross-provider review did not run)."
  exit 0
fi

# 5. Run the review, with the reviewer model pinned for reproducibility.
log "Running R2 cross-provider review: $branch vs $base (reviewer: $REVIEWER_MODEL/$REVIEWER_EFFORT)."
# stdin is redirected from /dev/null so Codex never consumes the hook's ref list.
"$codex_bin" review --base "$base" \
  -c "model=$REVIEWER_MODEL" \
  -c "model_reasoning_effort=$REVIEWER_EFFORT" </dev/null
status=$?

# 6. Findings are advisory (ai_guidelines.md): surface them, do not block by default.
if [ "$status" -ne 0 ]; then
  if [ "${CODEX_REVIEW_BLOCKING:-}" = "1" ]; then
    log "Codex review exited $status and blocking mode is on; stopping push."
    exit "$status"
  fi
  log "Codex review exited $status; advisory only, push not blocked. Address or justify findings in the PR."
fi
exit 0
