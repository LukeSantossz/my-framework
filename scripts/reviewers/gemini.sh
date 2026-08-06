#!/usr/bin/env bash
# R2 backend adapter: Gemini CLI (Google), a second agentic reviewer that
# explores the repository itself. Contract in docs/standards/r2_gate.md:
#   exit 0  reviewed, nothing blocking
#   exit 10 unavailable — the chain advances
#   other   reviewed, findings or a mid-review failure
#
# The invocation is configurable (r2.gemini.promptFlag) rather than hard-coded,
# because this adapter cannot be verified against the real CLI in this
# repository's test environment: a flag change upstream must be fixable by
# configuration, not by a framework release. See the risk recorded in
# docs/specs/0013-detach-r2-from-codex.md.
set -u

gemini_bin="${GEMINI_BIN:-gemini}"
base="${R2_BASE:-main}"
branch="${R2_BRANCH:-HEAD}"
model="${R2_RESOLVED_MODEL:-}"
prompt_flag="$(git config --get r2.gemini.promptFlag 2>/dev/null || true)"
prompt_flag="${prompt_flag:---prompt}"

log() { printf '[r2-review:gemini] %s\n' "$1"; }

script_dir="$(cd "$(dirname "$0")" && pwd)"
agents_file="$script_dir/../../AGENTS.md"

prompt="You are the R2 cross-provider reviewer for this repository.
Review the changes on branch '$branch' against '$base'. Report findings only;
do not rewrite code. Follow the role and the binding standards below.

$( [ -f "$agents_file" ] && cat "$agents_file" )"

if [ "${R2_DRYRUN:-}" = "1" ]; then
  printf '%s\n' "gemini ${model:+-m \"$model\" }$prompt_flag \"<R2 review prompt for $branch vs $base>\""
  exit 0
fi

if ! command -v "$gemini_bin" >/dev/null 2>&1; then
  log "Gemini CLI not installed."
  exit 10
fi

set -- "$prompt_flag" "$prompt"
[ -n "$model" ] && set -- -m "$model" "$@"

output="$("$gemini_bin" "$@" </dev/null 2>&1)"
status=$?
printf '%s\n' "$output"

[ "$status" -eq 0 ] && exit 0

if printf '%s' "$output" | grep -qiE \
  "quota|rate limit|resource[_ ]exhausted|not authenticated|unauthorized|401|403|network error|connection (refused|reset)"; then
  log "Gemini unavailable (quota, authentication, or network); not a review."
  exit 10
fi

exit "$status"
