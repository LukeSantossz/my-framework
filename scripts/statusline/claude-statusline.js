#!/usr/bin/env node
// Claude Code renderer for the status line contract defined in
// docs/standards/status_line.md. It prints the same five facts, in the same
// order, that Codex renders natively from its `[tui] status_line` segments:
//
//   1. model with reasoning effort      (Codex: model-with-reasoning)
//   2. context used against the window  (Codex: context-used, context-window-size)
//   3. tokens spent this session        (Codex: used-tokens)
//   4. five-hour and weekly quota       (Codex: five-hour-limit, weekly-limit)
//   5. directory and git branch         (Codex: current-dir, git-branch)
//
// Installed into CLAUDE_HOME by `scripts/setup.sh --statusline`; Claude Code
// runs it once per render with the session JSON on stdin.
//
// A fact this cannot read degrades to a placeholder. The status line is not a
// place to fail: an exception here would replace the whole bar with an error,
// so every read is guarded and the process always exits 0.
//
// Environment:
//   CLAUDE_HOME / CLAUDE_CONFIG_DIR  where settings, credentials and the usage
//                                    cache live (default: ~/.claude)
//   NO_COLOR                         emit plain text, no escape sequences
//   MYFW_STATUSLINE_NO_REFRESH       never spawn the background usage refresh

const fs = require("fs");
const os = require("os");
const path = require("path");
const https = require("https");
const { spawn, execFileSync } = require("child_process");

const CLAUDE_HOME =
  process.env.CLAUDE_HOME ||
  process.env.CLAUDE_CONFIG_DIR ||
  path.join(os.homedir(), ".claude");

const CREDENTIALS = path.join(CLAUDE_HOME, ".credentials.json");
const CACHE = path.join(CLAUDE_HOME, ".usage-cache.json");
const LOCK = CACHE + ".lock";
const SETTINGS = path.join(CLAUDE_HOME, "settings.json");

const TTL_MS = 5 * 60 * 1000; // refresh the quota at most every 5 minutes
const BACKOFF_MS = 30 * 60 * 1000; // the usage endpoint rate-limits per token
const FALLBACK_VERSION = "2.1.161";
const LOCK_STALE_MS = 30 * 1000;

// ---------------------------------------------------------------- formatting

const plain = Boolean(process.env.NO_COLOR);
const esc = (n) => (plain ? "" : `\x1b[${n}m`);
const reset = esc(0);
const dim = esc(2);
const bold = esc(1);
const COLOR = {
  cyan: esc(36),
  green: esc(32),
  yellow: esc(33),
  red: esc(31),
  gray: esc(90),
  orange: esc("38;5;208"),
};

// Warmer as the resource runs out, so a glance at the bar is enough.
function heat(pct, strongAt) {
  if (pct >= strongAt) return bold + COLOR.red;
  if (pct >= 75) return COLOR.orange;
  if (pct >= 50) return COLOR.yellow;
  return COLOR.green;
}

function formatTokens(n) {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(2) + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(1) + "k";
  return String(n);
}

function bar(pct, cells = 10) {
  const filled = Math.max(0, Math.min(cells, Math.round((pct / 100) * cells)));
  return "■".repeat(filled) + "□".repeat(cells - filled);
}

function readJson(file) {
  try {
    return JSON.parse(fs.readFileSync(file, "utf8"));
  } catch (_) {
    return null;
  }
}

// ------------------------------------------------------------- refresh mode
// Fetches the real plan usage and writes the cache. Spawned detached by the
// render pass when the cache is stale, so rendering never waits on the network.

if (process.argv.includes("--refresh")) {
  const version = process.argv[3] || FALLBACK_VERSION;
  const previous = readJson(CACHE) || {};
  const credentials = readJson(CREDENTIALS);
  const token =
    credentials &&
    credentials.claudeAiOauth &&
    credentials.claudeAiOauth.accessToken;
  if (!token) process.exit(0);

  const keep = (nextAllowed) => {
    try {
      fs.writeFileSync(
        CACHE,
        JSON.stringify({ ...previous, ts: Date.now(), nextAllowed })
      );
    } catch (_) {}
  };

  const request = https.request(
    {
      hostname: "api.anthropic.com",
      path: "/api/oauth/usage",
      method: "GET",
      headers: {
        Authorization: "Bearer " + token,
        "User-Agent": "claude-code/" + version,
        Accept: "application/json",
      },
      timeout: 8000,
    },
    (res) => {
      let body = "";
      res.on("data", (chunk) => (body += chunk));
      res.on("end", () => {
        const now = Date.now();
        // A rate-limited or failed call keeps the previous figures and backs
        // off; showing stale quota beats showing none.
        if (res.statusCode === 429) return keep(now + BACKOFF_MS);
        if (res.statusCode !== 200) return keep(now + TTL_MS);
        let parsed;
        try {
          parsed = JSON.parse(body);
        } catch (_) {
          return keep(now + TTL_MS);
        }
        try {
          fs.writeFileSync(
            CACHE,
            JSON.stringify({
              ts: now,
              nextAllowed: now + TTL_MS,
              fiveHour: parsed.five_hour
                ? {
                    util: parsed.five_hour.utilization,
                    reset: parsed.five_hour.resets_at,
                  }
                : null,
              sevenDay: parsed.seven_day
                ? { util: parsed.seven_day.utilization }
                : null,
            })
          );
        } catch (_) {}
      });
    }
  );
  request.on("error", () => keep(Date.now() + TTL_MS));
  request.on("timeout", () => request.destroy());
  request.end();
  return;
}

// -------------------------------------------------------------- render mode

// Segment 1: model with reasoning effort. The session payload carries the
// model; the effort is settings state, not session state, so it is read from
// CLAUDE_HOME and simply omitted when unset.
function modelSegment(input) {
  const name = (input.model && input.model.display_name) || "model?";
  const settings = readJson(SETTINGS);
  const effort = settings && settings.effortLevel;
  const model = `${COLOR.cyan}${bold}${name}${reset}`;
  return effort ? `${model} ${dim}${effort}${reset}` : model;
}

// Reads the transcript once and returns both token facts:
//   context — the last main-chain turn's full input, which is what occupies
//             the window right now;
//   spent   — input + output + cache creation accumulated over the session,
//             excluding cache reads (a re-read of context already counted) so
//             the figure is comparable to Codex's used-tokens.
// Sidechain turns are subagent sessions and belong to neither.
function readTranscript(transcriptPath) {
  const result = { context: 0, spent: 0 };
  if (!transcriptPath) return result;
  let lines;
  try {
    lines = fs.readFileSync(transcriptPath, "utf8").split("\n");
  } catch (_) {
    return result;
  }
  let lastContext = 0;
  for (const line of lines) {
    if (!line) continue;
    let entry;
    try {
      entry = JSON.parse(line);
    } catch (_) {
      continue;
    }
    if (entry.type !== "assistant" || entry.isSidechain) continue;
    const usage = entry.message && entry.message.usage;
    if (!usage) continue;
    const input = usage.input_tokens || 0;
    const output = usage.output_tokens || 0;
    const created = usage.cache_creation_input_tokens || 0;
    const read = usage.cache_read_input_tokens || 0;
    result.spent += input + output + created;
    lastContext = input + created + read;
  }
  result.context = lastContext;
  return result;
}

// Segment 2: context used against the window. The 1M window is a model
// variant, marked in the id rather than reported as a number.
function contextSegment(input, contextTokens) {
  const id = (input.model && input.model.id) || "";
  const isMillion = /\[1m\]/i.test(id);
  const windowSize = isMillion ? 1_000_000 : 200_000;
  const windowLabel = isMillion ? "1M" : "200k";
  const pct = (contextTokens / windowSize) * 100;
  const color = heat(pct, 80);
  return (
    `${dim}ctx${reset} ${color}${bar(pct)}${reset} ${color}${pct.toFixed(0)}%${reset} ` +
    `${dim}${formatTokens(contextTokens)}/${windowLabel}${reset}`
  );
}

// Segment 3: tokens spent this session.
function spentSegment(spentTokens) {
  return `${COLOR.green}${formatTokens(spentTokens)} tok${reset}`;
}

// Segment 4: the five-hour and weekly quota windows, from the cache the
// refresh pass maintains. Unavailable without an OAuth session, which is a
// declared degradation rather than an error.
function quotaSegment(cache) {
  if (!cache || !cache.fiveHour) return `${dim}usage n/a${reset}`;
  const fiveHour = cache.fiveHour.util || 0;
  let segment = `${dim}usage${reset} ${heat(fiveHour, 90)}${bar(fiveHour)} ${fiveHour.toFixed(0)}% 5h${reset}`;
  if (cache.sevenDay) {
    const weekly = cache.sevenDay.util || 0;
    segment += `  ${heat(weekly, 90)}${weekly.toFixed(0)}% 7d${reset}`;
  }
  try {
    const remaining = new Date(cache.fiveHour.reset).getTime() - Date.now();
    if (remaining > 0) {
      const hours = Math.floor(remaining / 3600000);
      const minutes = Math.floor((remaining % 3600000) / 60000);
      segment += ` ${dim}(reset ${hours}h${String(minutes).padStart(2, "0")})${reset}`;
    }
  } catch (_) {}
  return segment;
}

// Segment 5: where the session is. The branch is read from git rather than the
// payload, which does not carry it.
function locationSegment(input) {
  const cwd =
    input.cwd ||
    (input.workspace && input.workspace.current_dir) ||
    process.cwd();
  const dir = path.basename(cwd);
  let branch = "";
  for (const args of [
    ["symbolic-ref", "--short", "HEAD"],
    ["rev-parse", "--short", "HEAD"],
  ]) {
    try {
      branch = execFileSync("git", args, {
        cwd,
        encoding: "utf8",
        stdio: ["ignore", "pipe", "ignore"],
      }).trim();
      if (branch) break;
    } catch (_) {}
  }
  return branch
    ? `${COLOR.cyan}${dir}${reset}${dim}:${reset}${branch}`
    : `${COLOR.cyan}${dir}${reset}`;
}

// The quota is only as fresh as the last refresh, and refreshing inline would
// block every render on the network. The lock keeps concurrent renders from
// each spawning their own.
function scheduleRefresh(cache, version) {
  if (process.env.MYFW_STATUSLINE_NO_REFRESH) return;
  const now = Date.now();
  if (cache && now <= (cache.nextAllowed || 0)) return;
  let lockAge = Infinity;
  try {
    lockAge = now - fs.statSync(LOCK).mtimeMs;
  } catch (_) {}
  if (lockAge <= LOCK_STALE_MS) return;
  try {
    fs.writeFileSync(LOCK, "");
    const child = spawn(process.execPath, [__filename, "--refresh", version], {
      detached: true,
      stdio: "ignore",
      windowsHide: true,
    });
    child.unref();
  } catch (_) {}
}

let raw = "";
process.stdin.setEncoding("utf8");
process.stdin.on("data", (chunk) => (raw += chunk));
process.stdin.on("end", () => {
  let line;
  try {
    let input;
    try {
      input = JSON.parse(raw);
    } catch (_) {
      input = {};
    }

    const tokens = readTranscript(input.transcript_path);
    const cache = readJson(CACHE);
    scheduleRefresh(cache, input.version || FALLBACK_VERSION);

    const separator = `${COLOR.gray} | ${reset}`;
    line = [
      modelSegment(input),
      contextSegment(input, tokens.context),
      spentSegment(tokens.spent),
      quotaSegment(cache),
      locationSegment(input),
    ].join(separator);
  } catch (_) {
    // Last resort: a degraded line still beats an error where the bar goes.
    line = "status line unavailable";
  }
  process.stdout.write(line);
});
