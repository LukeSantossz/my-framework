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
	action := "render"
	if len(args) > 0 {
		action = args[0]
	}
	switch action {
	case "render":
		return statuslineRender(env, args[1:])
	case "apply":
		return statuslineApply(env, args[1:])
	case "refresh":
		return statuslineRefresh(env, args[1:])
	}
	fmt.Fprintf(env.Stderr, "mf statusline: unknown action %q (expected render, apply or refresh)\n", action)
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
	if !statusline.ClaimRefresh(home, now) {
		return
	}
	self, err := os.Executable()
	if err != nil {
		return
	}
	args := []string{"statusline", "refresh"}
	if facts.Version != "" {
		args = append(args, "--version", facts.Version)
	}
	cmd := exec.Command(self, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return
	}
	// Released rather than waited on: the render pass is finished, and reaping
	// this child is not worth holding the redraw open for.
	_ = cmd.Process.Release()
}

func statuslineRefresh(env Env, args []string) int {
	version := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version":
			if i+1 < len(args) {
				i++
				version = args[i]
			}
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
// This is the only command that writes outside the repository, which is why it
// is a command of its own rather than part of `mf init`: Codex's [tui] section
// has no per-project form, so applying the contract governs every project on
// the machine and nobody should get that by running an activation step.
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

// renderCommand is what Claude Code will run each redraw: this binary, by
// absolute path. A bare `mf` would depend on the PATH of whatever process the
// agent happens to spawn the status line from.
func renderCommand() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot resolve this binary's path: %w", err)
	}
	if strings.ContainsAny(self, " \t") {
		return fmt.Sprintf("%q statusline render", self), nil
	}
	return self + " statusline render", nil
}
