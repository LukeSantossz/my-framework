#!/usr/bin/env bash
# R2 cross-provider review runner: walks a chain of reviewer backends until one
# actually reviews the branch, then reports which one did. The Reviewer must be
# a provider different from the Author, per docs/standards/ai_guidelines.md.
# Called by the pre-push hook; also runnable by hand.
# See docs/standards/r2_gate.md for the adapter contract and configuration.
set -u

script_dir="$(cd "$(dirname "$0")" && pwd)"
reviewers_dir="${R2_REVIEWERS_DIR:-$script_dir/reviewers}"

log() { printf '[r2-review] %s\n' "$1"; }

# An adapter reports one of three outcomes, because the distinction the chain
# turns on — did this reviewer run at all — cannot be recovered from outside
# the adapter that owns the tool.
ADAPTER_UNAVAILABLE=10

# Reads a setting through git's own scope cascade (local, then global). This is
# what makes a machine-global default possible while a repository can still
# override it, matching the authority rule in code_conventions.md.
cfg() { git config --get "$1" 2>/dev/null || true; }

# The legacy codexreview.* keys are deliberately repo-local: they were persisted
# per repository before the cascade existed, and promoting them to global scope
# now would make a machine-wide value appear in repositories that never asked
# for it.
cfg_local() { git config --local --get "$1" 2>/dev/null || true; }

# First non-empty wins. An empty environment override is treated as unset, so
# `FOO= command` falls through to the persisted value rather than resolving to
# the empty string.
first_set() {
  for candidate in "$@"; do
    if [ -n "$candidate" ]; then
      printf '%s' "$candidate"
      return 0
    fi
  done
  printf ''
}

backends="$(first_set "${R2_BACKENDS:-}" "$(cfg r2.backends)" "codex")"
base="$(first_set "${R2_BASE:-}" "${CODEX_REVIEW_BASE:-}" "$(cfg r2.base)" "main")"
branch="$(first_set "${R2_BRANCH:-}" "${CODEX_REVIEW_BRANCH:-}" "$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)")"
blocking="$(first_set "${R2_BLOCKING:-}" "${CODEX_REVIEW_BLOCKING:-}")"
dryrun="$(first_set "${R2_DRYRUN:-}" "${CODEX_REVIEW_DRYRUN:-}")"

# 1. Explicit bypass.
if [ "${SKIP_R2_REVIEW:-}" = "1" ] || [ "${SKIP_CODEX_REVIEW:-}" = "1" ]; then
  log "bypass set; skipping R2 gate."
  exit 0
fi

# 2. Nothing to review when the branch is its own base.
if [ "$branch" = "$base" ]; then
  log "On base branch '$base'; nothing to review against itself. Skipping R2."
  exit 0
fi

# 2b. Nothing to review when the branch adds nothing over its base. Announcing
#     a review of an empty diff would be a false entry in the PR's review-layers
#     record, so this is answered here rather than left to a backend: it is a
#     property of the branch, not of any reviewer. A base that does not resolve
#     is not this check's business — the adapters report that in their own terms.
if git rev-parse --verify --quiet "$base" >/dev/null 2>&1 \
  && git rev-parse --verify --quiet "$branch" >/dev/null 2>&1; then
  if [ -z "$(git diff --name-only "$base...$branch" 2>/dev/null)" ]; then
    log "'$branch' adds nothing over '$base'; nothing to review. Skipping R2."
    exit 0
  fi
fi

# Resolves the model and effort for one backend. The per-backend key beats the
# chain-wide one, so a chain can mix a hosted reviewer with a local fallback
# without either inheriting the other's model name. The codex backend
# additionally honors the legacy keys, so a clone configured before the seam
# existed keeps resolving exactly as it did.
resolve_model() {
  resolve_backend="$1"
  config_model="$(first_set "$(cfg "r2.$resolve_backend.model")" "$(cfg r2.model)")"
  config_effort="$(first_set "$(cfg "r2.$resolve_backend.effort")" "$(cfg r2.effort)")"
  legacy_model=""
  legacy_effort=""
  legacy_env_model=""
  legacy_env_effort=""
  default_model=""
  default_effort="high"
  if [ "$resolve_backend" = "codex" ]; then
    legacy_model="$(cfg_local codexreview.model)"
    legacy_effort="$(cfg_local codexreview.effort)"
    legacy_env_model="${CODEX_REVIEW_MODEL:-}"
    legacy_env_effort="${CODEX_REVIEW_EFFORT:-}"
    default_model="gpt-5.6-terra"
  fi
  R2_RESOLVED_MODEL="$(first_set "${R2_MODEL:-}" "$legacy_env_model" "$config_model" "${legacy_model:-$default_model}")"
  R2_RESOLVED_EFFORT="$(first_set "${R2_EFFORT:-}" "$legacy_env_effort" "$config_effort" "${legacy_effort:-$default_effort}")"
  export R2_RESOLVED_MODEL R2_RESOLVED_EFFORT
}

# 3. Walk the chain. Order is quality order: the strongest reviewer first, the
#    rest there for when it is unavailable.
export R2_BASE="$base" R2_BRANCH="$branch"
reviewed_by=""
skipped=""
last_status=0

old_ifs="$IFS"
IFS=','
set -f
# shellcheck disable=SC2086
set -- $backends
set +f
IFS="$old_ifs"

for backend in "$@"; do
  [ -n "$backend" ] || continue
  adapter="$reviewers_dir/$backend.sh"

  if [ ! -f "$adapter" ]; then
    log "backend '$backend': no adapter at $adapter; skipping."
    skipped="$skipped $backend(no adapter)"
    continue
  fi

  resolve_model "$backend"

  if [ "$dryrun" = "1" ]; then
    # Dry-run describes the whole chain rather than stopping at the first
    # backend: the point is to show what would happen, fallbacks included.
    R2_DRYRUN=1 bash "$adapter"
    continue
  fi

  log "trying backend '$backend' (model: ${R2_RESOLVED_MODEL:-<tool default>}/${R2_RESOLVED_EFFORT})."
  # stdin is redirected from /dev/null so an agentic reviewer never consumes
  # the hook's ref list.
  bash "$adapter" </dev/null
  status=$?

  if [ "$status" -eq "$ADAPTER_UNAVAILABLE" ]; then
    log "backend '$backend': unavailable; advancing to the next backend."
    skipped="$skipped $backend(unavailable)"
    continue
  fi

  reviewed_by="$backend"
  last_status="$status"
  break
done

if [ "$dryrun" = "1" ]; then
  exit 0
fi

# 4. Report which reviewer actually ran. This line is meant to be copied into
#    the PR's review-layers record: falling back is allowed, falling back
#    quietly is not.
if [ -z "$reviewed_by" ]; then
  log "R2 did not run: no configured backend was available (tried:$skipped). Record the absence in the PR."
  # A reviewer that never ran is not a finding, so blocking mode must not turn
  # an expired quota into a locked repository.
  exit 0
fi

log "Reviewed by: $reviewed_by / ${R2_RESOLVED_MODEL:-<tool default>} (skipped:${skipped:- none})"

# 5. Findings are advisory (ai_guidelines.md): surface them, do not block by
#    default.
if [ "$last_status" -ne 0 ]; then
  if [ "$blocking" = "1" ]; then
    log "backend '$reviewed_by' exited $last_status and blocking mode is on; stopping push."
    exit "$last_status"
  fi
  log "backend '$reviewed_by' exited $last_status; advisory only, push not blocked. Address or justify findings in the PR."
fi
exit 0
