package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LukeSantossz/my-framework/internal/statusline"
)

// runStatusline renders and applies the status line contract.
//
// `render` is what an agent runs once per redraw, so it never fails and never
// waits on the network: a fact it cannot read degrades to a placeholder, and
// the quota refresh happens in a detached process the render pass only
// schedules.
func runStatusline(env Env, args []string) int {
	// The action defaults to render, and so must the arguments: reaching for
	// args[1:] after an action nobody typed panicked, which for the caller is a
	// crash in the one command whose whole contract is to degrade rather than
	// fail. The status line runs this on every redraw of the prompt.
	action, rest := "render", args
	if len(args) > 0 {
		action, rest = args[0], args[1:]
	}
	switch action {
	case "render":
		return statuslineRender(env, rest)
	case "apply":
		return statuslineApply(env, rest)
	case "revert":
		return statuslineRevert(env, rest)
	case "refresh":
		return statuslineRefresh(env, rest)
	}
	fmt.Fprintf(env.Stderr, "mf statusline: unknown action %q (expected render, apply, revert or refresh)\n", action)
	return 2
}

// claudeHome resolves where Claude Code keeps its settings, credentials and
// usage cache.
func claudeHome(env Env) string {
	for _, name := range []string{"CLAUDE_HOME", "CLAUDE_CONFIG_DIR"} {
		if dir := env.Getenv(name); dir != "" {
			return dir
		}
	}
	if home := userHome(env); home != "" {
		return filepath.Join(home, ".claude")
	}
	return ""
}

func codexHome(env Env) string {
	if dir := env.Getenv("CODEX_HOME"); dir != "" {
		return dir
	}
	if home := userHome(env); home != "" {
		return filepath.Join(home, ".codex")
	}
	return ""
}

func userHome(env Env) string {
	for _, name := range []string{"HOME", "USERPROFILE"} {
		if dir := env.Getenv(name); dir != "" {
			return dir
		}
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return dir
}

func statuslineRender(env Env, args []string) int {
	noRefresh := env.Getenv("MYFW_STATUSLINE_NO_REFRESH") != ""
	for _, a := range args {
		switch a {
		case "--no-refresh":
			noRefresh = true
		default:
			fmt.Fprintf(env.Stderr, "mf statusline render: unknown option %q\n", a)
			return 2
		}
	}

	var raw []byte
	if env.Stdin != nil {
		raw, _ = io.ReadAll(env.Stdin)
	}
	home := claudeHome(env)
	facts := statusline.Read(raw, statusline.Options{Home: home, Now: env.Now})

	if !noRefresh {
		scheduleRefresh(env, home, facts)
	}

	// NO_COLOR is the one convention every terminal tool already honours, and
	// the contract binds the facts rather than the colours, so obeying it costs
	// nothing the standard cares about.
	fmt.Fprint(env.Stdout, statusline.Render(facts, env.Getenv("NO_COLOR") == ""))

	// Always zero. An exit code where the status bar goes replaces every fact
	// with an error message, which is worse than losing one.
	return 0
}

// scheduleRefresh spawns the quota fetch in a detached process. Fetching inline
// would block every redraw on the network, and a status line that waits is a
// status line the Developer turns off.
func scheduleRefresh(env Env, home string, facts statusline.Facts) {
	if home == "" {
		return
	}
	now := time.Now()
	if env.Now != nil {
		now = env.Now()
	}
	if !statusline.RefreshDue(facts.Cache, now) {
		return
	}
	token, claimed := statusline.ClaimRefresh(home, now)
	if !claimed {
		return
	}
	self, err := os.Executable()
	if err != nil {
		// Claimed and not handed to anyone: the claim has to go back, or the
		// lock outlives a refresh that never started.
		statusline.ReleaseRefresh(home, token)
		return
	}
	// The child is told it holds a claim, because it is the only process that
	// may end one. A refresh a person starts by hand holds none, and releasing
	// what it never took would drop a scheduled refresh's claim mid-fetch —
	// letting the next render start a second request against an endpoint that
	// rate-limits per token.
	args := []string{"statusline", "refresh", claimedFlag, token}
	if facts.Version != "" {
		args = append(args, "--version", facts.Version)
	}
	cmd := exec.Command(self, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		statusline.ReleaseRefresh(home, token)
		return
	}
	// Released rather than waited on: the render pass is finished, and reaping
	// this child is not worth holding the redraw open for.
	_ = cmd.Process.Release()
}

// claimedFlag carries the token the refresh was spawned under. It is not a
// user-facing option, and it is not a secret either: the token identifies which
// claim this process may end, so a refresh started by hand — or one whose claim
// was taken over as stale while it ran — releases nothing.
const claimedFlag = "--claimed"

func statuslineRefresh(env Env, args []string) int {
	version := ""
	claim := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case claimedFlag:
			// The token the claim was taken under. A refresh started by hand
			// carries none, so it releases nothing.
			value, ok := optionValue(env, "mf statusline refresh", args, &i)
			if !ok {
				return 2
			}
			claim = value
			continue
		case "--version":
			// Ignoring the missing value would refresh under a version nobody
			// asked for and report success for it.
			value, ok := optionValue(env, "mf statusline refresh", args, &i)
			if !ok {
				return 2
			}
			version = value
		default:
			fmt.Fprintf(env.Stderr, "mf statusline refresh: unknown option %q\n", args[i])
			return 2
		}
	}
	home := claudeHome(env)
	if home == "" {
		fmt.Fprintln(env.Stderr, "mf statusline refresh: no configuration directory to cache the quota in")
		return 1
	}
	// This process ends the claim only if it is the one the claim was taken
	// for — whichever way it ends.
	if claim != "" {
		defer statusline.ReleaseRefresh(home, claim)
	}
	if err := statusline.Refresh(statusline.RefreshOptions{
		Home:     home,
		Endpoint: env.Getenv("MF_USAGE_ENDPOINT"),
		Version:  version,
		Now:      env.Now,
	}); err != nil {
		fmt.Fprintf(env.Stderr, "mf statusline refresh: %v\n", err)
		return 1
	}
	return 0
}

// statuslineApply writes the contract into both agents' configurations.
//
// It is a command of its own rather than part of `mf init` because of what it
// writes, not because it is alone in writing outside the repository — several
// commands do that. The others write files this framework created; this one
// rewrites a file the Developer set up, and Codex's [tui] section has no
// per-project form, so applying the contract governs every project on the
// machine. Nobody should get that by running an activation step.
func statuslineApply(env Env, args []string) int {
	for _, a := range args {
		fmt.Fprintf(env.Stderr, "mf statusline apply: unknown option %q\n", a)
		return 2
	}
	now := time.Now()
	if env.Now != nil {
		now = env.Now()
	}

	failures := 0

	codexDir := codexHome(env)
	if codexDir == "" {
		fmt.Fprintln(env.Stdout, "codex   no home directory could be resolved; skipped")
		failures++
	} else if res, err := statusline.ApplyCodex(codexDir, now); err != nil {
		fmt.Fprintf(env.Stderr, "codex   %v\n", err)
		failures++
	} else {
		printApplied(env, "codex", res)
	}

	claudeDir := claudeHome(env)
	switch {
	case claudeDir == "":
		fmt.Fprintln(env.Stdout, "claude  no home directory could be resolved; skipped")
		failures++
	default:
		command, err := renderCommand()
		if err != nil {
			fmt.Fprintf(env.Stderr, "claude  %v\n", err)
			failures++
			break
		}
		res, err := statusline.ApplyClaude(claudeDir, command, now)
		if err != nil {
			fmt.Fprintf(env.Stderr, "claude  %v\n", err)
			failures++
			break
		}
		printApplied(env, "claude", res)
	}

	fmt.Fprintln(env.Stdout, "\nThis is machine state, not repository state: Codex's [tui] section has no")
	fmt.Fprintln(env.Stdout, "per-project form, so the contract now governs every project on this machine.")
	if failures > 0 {
		return 1
	}
	return 0
}

// statuslineRevert puts back what the last apply replaced.
//
// Apply is the one command that rewrites a file the Developer owns, which
// makes it the one that owes the machine a way back: a Developer who applied the
// contract to see what it does should not have to work out which backup name
// belongs to which run, in a directory the framework does not own.
func statuslineRevert(env Env, args []string) int {
	for _, a := range args {
		fmt.Fprintf(env.Stderr, "mf statusline revert: unknown option %q\n", a)
		return 2
	}

	failures := 0
	for _, target := range []struct{ agent, dir, file string }{
		{"codex", codexHome(env), "config.toml"},
		{"claude", claudeHome(env), "settings.json"},
	} {
		if target.dir == "" {
			fmt.Fprintf(env.Stdout, "%-7s no home directory could be resolved; skipped\n", target.agent)
			failures++
			continue
		}
		res, err := statusline.Revert(filepath.Join(target.dir, target.file))
		if err != nil {
			fmt.Fprintf(env.Stderr, "%-7s %v\n", target.agent, err)
			failures++
			continue
		}
		printReverted(env, target.agent, res)
	}
	if failures > 0 {
		return 1
	}
	return 0
}

func printApplied(env Env, agent string, res statusline.Result) {
	switch res.Action {
	case statusline.ActionUnchanged:
		fmt.Fprintf(env.Stdout, "%-7s already canonical: %s\n", agent, res.Target)
	default:
		fmt.Fprintf(env.Stdout, "%-7s applied to %s\n", agent, res.Target)
		if res.Backup != "" {
			fmt.Fprintf(env.Stdout, "        backed up -> %s\n", filepath.Base(res.Backup))
		}
	}
}

func printReverted(env Env, agent string, res statusline.Result) {
	if res.Action != statusline.ActionRestored {
		fmt.Fprintf(env.Stdout, "%-7s no backup left to restore: %s\n", agent, res.Target)
		return
	}
	fmt.Fprintf(env.Stdout, "%-7s restored %s\n", agent, res.Target)
	fmt.Fprintf(env.Stdout, "        from <- %s\n", filepath.Base(res.Backup))
}

// renderCommand is what Claude Code will run each redraw: this binary, by
// absolute path. A bare `mf` would depend on the PATH of whatever process the
// agent happens to spawn the status line from.
func renderCommand() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot resolve this binary's path: %w", err)
	}
	return shellQuote(self) + " statusline render", nil
}

// shellQuote wraps a path so the shell that runs the command receives it
// unchanged.
//
// It is always quoted, and quoted for a shell rather than for Go. Go's %q is
// the wrong syntax twice over: on Windows it doubles every backslash, so
// `C:\Users\x\mf.exe` reaches the agent as a path that does not exist; and
// quoting only when a space is present leaves the same path bare, where a POSIX
// shell eats the backslashes instead. Single quotes are literal in every shell
// this command is handed to, so both the space and the backslash survive.
func shellQuote(path string) string {
	// A single quote cannot appear inside single quotes: the run has to be
	// closed, the quote escaped outside it, and the run reopened.
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}
