#!/usr/bin/env bash
# R2 backend adapter: any endpoint speaking the OpenAI chat-completions shape.
# One adapter reaches Ollama, LM Studio, llama.cpp, vLLM, DeepSeek, Groq,
# OpenRouter and Together — local and hosted alike — which is what pays for the
# seam. Contract in docs/standards/r2_gate.md:
#   exit 0  reviewed, nothing blocking
#   exit 10 unavailable — the chain advances
#   other   reviewed, findings or a mid-review failure
#
# Unlike the agentic backends, this one sees only what it is sent: the branch
# diff, plus AGENTS.md for the Reviewer role and the binding standards. It
# cannot explore the repository, so its review is a different shape, not merely
# a weaker grade.
set -u

node_bin="${NODE_BIN:-node}"
base="${R2_BASE:-main}"
branch="${R2_BRANCH:-HEAD}"
model="${R2_RESOLVED_MODEL:-}"

endpoint="${R2_OPENAI_ENDPOINT:-$(git config --get r2.openai.endpoint 2>/dev/null || true)}"
key_env="${R2_OPENAI_API_KEY_ENV:-$(git config --get r2.openai.apiKeyEnv 2>/dev/null || true)}"
max_bytes="${R2_OPENAI_MAX_DIFF_BYTES:-$(git config --get r2.openai.maxDiffBytes 2>/dev/null || true)}"
# 30 KB, not the round 100 KB it started at. Measured against
# deepseek-v4-flash: 8 KB took 27s, 30 KB drew 9.2k reasoning tokens over 85s,
# 40 KB ran 163s once and blew a 240s budget the next time, and 112 KB never
# returned at all. A reasoning model's latency is volatile enough that the cap
# only makes the budget reachable, it does not make it certain — which is why
# exceeding the budget is unavailability rather than a failure.
max_bytes="${max_bytes:-30000}"
timeout_seconds="${R2_OPENAI_TIMEOUT_SECONDS:-$(git config --get r2.openai.timeoutSeconds 2>/dev/null || true)}"
timeout_seconds="${timeout_seconds:-240}"

log() { printf '[r2-review:openai] %s\n' "$1"; }

script_dir="$(cd "$(dirname "$0")" && pwd)"
agents_file="$script_dir/../../AGENTS.md"

if [ "${R2_DRYRUN:-}" = "1" ]; then
  printf '%s\n' "POST ${endpoint:-<unset r2.openai.endpoint>}/chat/completions model=\"${model:-<unset>}\" (diff of $branch vs $base, max ${max_bytes}B, budget ${timeout_seconds}s)"
  exit 0
fi

if [ -z "$endpoint" ]; then
  log "no endpoint configured (set r2.openai.endpoint)."
  exit 10
fi
if [ -z "$model" ]; then
  log "no model configured (set r2.openai.model)."
  exit 10
fi
# Node does the JSON and the HTTP request. It is already the framework's
# declared soft dependency; adding a second tool to do the same class of work
# would cost an adopter another install for nothing.
if ! command -v "$node_bin" >/dev/null 2>&1; then
  log "node is not installed; this backend needs it for the request and the JSON."
  exit 10
fi

diff_text="$(git diff "$base"..."$branch" 2>/dev/null || git diff "$base" 2>/dev/null || true)"
if [ -z "$diff_text" ]; then
  log "no diff of $branch against $base; nothing to review."
  exit 0
fi

api_key=""
if [ -n "$key_env" ]; then
  api_key="$(printenv "$key_env" 2>/dev/null || true)"
  if [ -z "$api_key" ]; then
    # A localhost endpoint (Ollama, LM Studio) needs no key; a remote one does,
    # and failing here rather than sending an unauthenticated request keeps the
    # reason legible.
    case "$endpoint" in
      *localhost*|*127.0.0.1*) : ;;
      *)
        log "\$$key_env is empty; this endpoint needs a key."
        exit 10
        ;;
    esac
  fi
fi

R2_OPENAI_ENDPOINT="$endpoint" \
R2_OPENAI_MODEL="$model" \
R2_OPENAI_KEY="$api_key" \
R2_OPENAI_MAX_BYTES="$max_bytes" \
R2_OPENAI_TIMEOUT_SECONDS="$timeout_seconds" \
R2_OPENAI_AGENTS="$agents_file" \
R2_OPENAI_BASE="$base" \
R2_OPENAI_BRANCH="$branch" \
R2_OPENAI_DIFF="$diff_text" \
"$node_bin" -e '
const fs = require("fs");
const http = require("http");
const https = require("https");
const { URL } = require("url");

const endpoint = process.env.R2_OPENAI_ENDPOINT.replace(/\/+$/, "");
const maxBytes = parseInt(process.env.R2_OPENAI_MAX_BYTES, 10) || 30000;
const budgetMs = (parseInt(process.env.R2_OPENAI_TIMEOUT_SECONDS, 10) || 240) * 1000;
const UNAVAILABLE = 10;

let diff = process.env.R2_OPENAI_DIFF || "";
let truncated = false;
if (Buffer.byteLength(diff, "utf8") > maxBytes) {
  diff = Buffer.from(diff, "utf8").slice(0, maxBytes).toString("utf8");
  truncated = true;
}

let agents = "";
try { agents = fs.readFileSync(process.env.R2_OPENAI_AGENTS, "utf8"); } catch (_) {}

// Stable prefix first, volatile diff last: providers on this shape bill cached
// prompt tokens at a fraction of fresh ones, and a pre-push gate re-sends this
// prefix on every push.
const messages = [
  {
    role: "system",
    content:
      "You are the R2 cross-provider reviewer for this repository. Report findings only; " +
      "do not rewrite code. Report correctness defects, invented or unverified symbols, " +
      "scope creep, security issues, and convention violations. If you find nothing, say so " +
      "in one line.\n\n" + agents,
  },
  {
    role: "user",
    content:
      "Review the diff of branch " + process.env.R2_OPENAI_BRANCH +
      " against " + process.env.R2_OPENAI_BASE +
      (truncated ? " (TRUNCATED: only the first " + maxBytes + " bytes are shown)" : "") +
      ".\n\n" + diff,
  },
];

const body = JSON.stringify({
  model: process.env.R2_OPENAI_MODEL,
  messages,
  temperature: 0,
});

const url = new URL(endpoint + "/chat/completions");
const client = url.protocol === "http:" ? http : https;
const headers = { "Content-Type": "application/json", "Content-Length": Buffer.byteLength(body) };
if (process.env.R2_OPENAI_KEY) headers.Authorization = "Bearer " + process.env.R2_OPENAI_KEY;

const req = client.request(
  { hostname: url.hostname, port: url.port, path: url.pathname + url.search, method: "POST", headers },
  (res) => {
    let raw = "";
    res.on("data", (c) => (raw += c));
    res.on("end", () => {
      // An HTTP error is this backend being unavailable, not a review with
      // findings: a retired model id or an expired key must advance the chain.
      if (res.statusCode !== 200) {
        console.error("[r2-review:openai] endpoint returned HTTP " + res.statusCode + ": " + raw.slice(0, 300));
        process.exit(UNAVAILABLE);
      }
      let parsed;
      try { parsed = JSON.parse(raw); } catch (_) {
        console.error("[r2-review:openai] response was not JSON.");
        process.exit(UNAVAILABLE);
      }
      const choice = (parsed.choices || [])[0] || {};
      // reasoning_content carries the model private chain of thought on
      // reasoning models. It is not the review and must never be reported as
      // findings.
      const content = (choice.message || {}).content || "";
      clearTimeout(budget);
      if (truncated) {
        console.log("[r2-review:openai] the diff was truncated at " + maxBytes + " bytes; this review is partial.");
      }
      console.log(content.trim() || "(no findings reported)");
      if (choice.finish_reason === "length") {
        console.log("[r2-review:openai] the review was cut off by the output limit; it is incomplete.");
      }
      process.exit(0);
    });
  }
);
// The budget is total elapsed time, not socket inactivity. Node own request
// timeout measures the latter, which never fires while a reasoning model is
// thinking and sending nothing — so it protected nothing, and a real run sat
// for fifteen minutes before the connection dropped. A pre-push gate that can
// do that is a gate that gets bypassed.
let expired = false;
const budget = setTimeout(() => {
  expired = true;
  req.destroy();
  console.error("[r2-review:openai] no answer within the " + budgetMs / 1000 + "s budget; treating this backend as unavailable.");
  process.exit(UNAVAILABLE);
}, budgetMs);

req.on("error", (e) => {
  if (expired) return;
  console.error("[r2-review:openai] endpoint unreachable: " + e.message);
  process.exit(UNAVAILABLE);
});
req.write(body);
req.end();
'
exit $?
