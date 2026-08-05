#!/usr/bin/env bash
# R2 backend adapter: Codex CLI (OpenAI), an agentic reviewer that explores the
# repository itself. Contract in docs/standards/r2_gate.md:
#   exit 0  reviewed, nothing blocking
#   exit 10 unavailable — the chain advances
#   other   reviewed, findings or a mid-review failure
set -u

codex_bin="${CODEX_BIN:-codex}"
base="${R2_BASE:-main}"
model="${R2_RESOLVED_MODEL:-gpt-5.6-terra}"
effort="${R2_RESOLVED_EFFORT:-high}"

log() { printf '[r2-review:codex] %s\n' "$1"; }

# Dry-run prints the command regardless of Codex availability, so the documented
# contract holds on a machine without it.
if [ "${R2_DRYRUN:-}" = "1" ]; then
  printf '%s\n' "codex review --base $base -c model=\"$model\" -c model_reasoning_effort=\"$effort\""
  exit 0
fi

if ! command -v "$codex_bin" >/dev/null 2>&1; then
  log "Codex not installed."
  exit 10
fi

output="$("$codex_bin" review --base "$base" \
  -c "model=$model" \
  -c "model_reasoning_effort=$effort" </dev/null 2>&1)"
status=$?
printf '%s\n' "$output"

[ "$status" -eq 0 ] && exit 0

# Classifying this vendor's failures is deliberately confined to this adapter:
# the runner must not carry a heuristic about a tool it does not own. These
# patterns are matched against Codex's own error text, which is why a drift in
# that wording degrades gracefully — a missed pattern reads as "reviewed", so
# the chain stops early and names this backend, rather than silently pretending
# a review happened somewhere else.
if printf '%s' "$output" | grep -qiE \
  "usage limit|rate limit|quota|not logged in|please (run )?codex login|unauthorized|401|403|network error|connection (refused|reset)"; then
  log "Codex unavailable (quota, authentication, or network); not a review."
  exit 10
fi

exit "$status"
