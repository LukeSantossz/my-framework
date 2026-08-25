package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func statuslineEnv(t *testing.T, stdin string, vars map[string]string, args ...string) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	return Env{
		Args:        args,
		Stdin:       strings.NewReader(stdin),
		Stdout:      &out,
		Stderr:      &errOut,
		RepoRoot:    t.TempDir(),
		MachinePath: filepath.Join(t.TempDir(), "config.toml"),
		Now:         func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) },
		Getenv:      func(name string) string { return vars[name] },
		GitConfig:   func(string) (string, bool) { return "", false },
	}, &out, &errOut
}

func sessionPayload(t *testing.T, dir string) string {
	t.Helper()
	transcript := filepath.Join(dir, "transcript.jsonl")
	body := `{"type":"assistant","message":{"usage":{"input_tokens":1200,"output_tokens":800,` +
		`"cache_creation_input_tokens":3000,"cache_read_input_tokens":300000}}}` + "\n"
	if err := os.WriteFile(transcript, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(filepath.ToSlash(transcript))
	return fmt.Sprintf(`{"model":{"id":"claude-opus-5[1m]","display_name":"Opus 5 (1M context)"},`+
		`"transcript_path":%s,"cwd":%q,"version":"2.1.161"}`, encoded, "/nowhere/repo")
}

func TestStatuslineRenderEmitsTheContractWithoutANodeRuntime(t *testing.T) {
	// The point of the port: the renderer is this binary. Nothing is spawned to
	// produce the line, and an empty PATH — no node, and no git either — still
	// yields every fact the contract names.
	t.Setenv("PATH", "")
	dir := t.TempDir()
	claudeHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(claudeHome, "settings.json"), []byte(`{"effortLevel":"high"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	e, out, errOut := statuslineEnv(t, sessionPayload(t, dir), map[string]string{
		"CLAUDE_HOME":                claudeHome,
		"MYFW_STATUSLINE_NO_REFRESH": "1",
		"NO_COLOR":                   "1",
	}, "statusline", "render")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	line := out.String()
	for _, want := range []string{"Opus 5", "high", "ctx", "304.2k/1M", "5.0k tok", "usage n/a", "repo"} {
		if !strings.Contains(line, want) {
			t.Errorf("the line lacks %q: %q", want, line)
		}
	}
}

func TestStatuslineRenderAlwaysExitsZero(t *testing.T) {
	// An exit code where the status bar goes replaces every fact with an error
	// message, which is worse than losing one.
	e, out, _ := statuslineEnv(t, "not json", map[string]string{"MYFW_STATUSLINE_NO_REFRESH": "1", "NO_COLOR": "1"},
		"statusline", "render")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if out.String() == "" {
		t.Error("a malformed payload emptied the status bar")
	}
}

func TestStatuslineApplyWritesBothConfigurationsAndIsIdempotent(t *testing.T) {
	codexHome, claudeHome := t.TempDir(), t.TempDir()
	vars := map[string]string{"CODEX_HOME": codexHome, "CLAUDE_HOME": claudeHome}

	e, out, errOut := statuslineEnv(t, "", vars, "statusline", "apply")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "machine state, not repository state") {
		t.Errorf("the command did not say what it just changed: %q", out.String())
	}

	toml, err := os.ReadFile(filepath.Join(codexHome, "config.toml"))
	if err != nil {
		t.Fatalf("no codex config written: %v", err)
	}
	if !strings.Contains(string(toml), "model-with-reasoning") {
		t.Errorf("the codex contract is missing:\n%s", toml)
	}
	settings, err := os.ReadFile(filepath.Join(claudeHome, "settings.json"))
	if err != nil {
		t.Fatalf("no settings written: %v", err)
	}
	if !strings.Contains(string(settings), "statusline render") {
		t.Errorf("the settings do not point at the binary:\n%s", settings)
	}
	// The Node renderer is not what gets wired any more.
	if strings.Contains(string(settings), ".js") {
		t.Errorf("the settings still point at a script:\n%s", settings)
	}

	second, out2, _ := statuslineEnv(t, "", vars, "statusline", "apply")
	if code := Run(second); code != 0 {
		t.Fatalf("second run exit %d", code)
	}
	if !strings.Contains(out2.String(), "already canonical") {
		t.Errorf("a conformant configuration was rewritten: %q", out2.String())
	}
	backups, _ := filepath.Glob(filepath.Join(claudeHome, "settings.json.bak.*"))
	if len(backups) != 0 {
		t.Errorf("re-running buried the original under %d generated copy/copies", len(backups))
	}
}

func TestTheClaudeCommandSurvivesTheShellThatWillRunIt(t *testing.T) {
	// Claude Code hands this string to a shell, so Go quoting is the wrong
	// syntax twice over: it doubles every backslash in a Windows path, and
	// leaving the path bare lets a POSIX shell eat them instead.
	command, err := renderCommand()
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if want := "'" + self + "' statusline render"; command != want {
		t.Errorf("renderCommand() = %s\nwant %s", command, want)
	}
}

func TestShellQuotingSurvivesSpacesBackslashesAndQuotes(t *testing.T) {
	for _, c := range []struct{ path, want string }{
		{`C:\Users\dev\mf.exe`, `'C:\Users\dev\mf.exe'`},
		{`/opt/my tools/mf`, `'/opt/my tools/mf'`},
		{`/home/o'brien/mf`, `'/home/o'\''brien/mf'`},
	} {
		if got := shellQuote(c.path); got != c.want {
			t.Errorf("shellQuote(%q) = %s, want %s", c.path, got, c.want)
		}
	}
}

func TestStatuslineRevertRestoresWhatApplyReplaced(t *testing.T) {
	codexHome, claudeHome := t.TempDir(), t.TempDir()
	vars := map[string]string{"CODEX_HOME": codexHome, "CLAUDE_HOME": claudeHome}
	toml := "[tui]\nstatus_line = [\"model\"]\n"
	settings := `{"model":"opus[1m]","statusLine":{"type":"command","command":"node /somewhere/personal.js"}}`
	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeHome, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	e, _, errOut := statuslineEnv(t, "", vars, "statusline", "apply")
	if code := Run(e); code != 0 {
		t.Fatalf("apply exit %d: %s", code, errOut.String())
	}

	back, out, errOut := statuslineEnv(t, "", vars, "statusline", "revert")
	if code := Run(back); code != 0 {
		t.Fatalf("revert exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "restored") {
		t.Errorf("the command did not say what it put back: %q", out.String())
	}
	for path, want := range map[string]string{
		filepath.Join(codexHome, "config.toml"):    toml,
		filepath.Join(claudeHome, "settings.json"): settings,
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != want {
			t.Errorf("%s was not restored:\n%s\nwant\n%s", path, body, want)
		}
	}
}

func TestStatuslineRejectsAFlagWithNoValue(t *testing.T) {
	// Ignoring the missing value refreshes under a version nobody asked for and
	// reports success for it. The home is a temporary one so a rejection that
	// stops working reaches an empty directory rather than the operator's own
	// credentials.
	e, _, errOut := statuslineEnv(t, "", map[string]string{"CLAUDE_HOME": t.TempDir()},
		"statusline", "refresh", "--version")
	if code := Run(e); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "--version") {
		t.Errorf("stderr %q does not name the option that lacks a value", errOut.String())
	}
}

func TestStatuslineRejectsAnUnknownAction(t *testing.T) {
	e, _, errOut := statuslineEnv(t, "", nil, "statusline", "wat")
	if code := Run(e); code == 0 {
		t.Fatal("an unknown action was accepted")
	}
	if !strings.Contains(errOut.String(), "wat") {
		t.Errorf("stderr %q does not name the action", errOut.String())
	}
}

func TestStatuslineWithNoActionRenders(t *testing.T) {
	// `mf statusline` defaults to render, and the status line runs it on every
	// redraw of the agent's prompt. Reaching for the arguments after an action
	// that was never typed panicked, which for the caller is a crash in the one
	// command that is supposed to degrade rather than fail.
	// The refresh is suppressed for the reason every other render test
	// suppresses it: render spawns the quota fetch as a detached copy of
	// os.Executable(), which under `go test` is the test binary itself. The
	// orphan outlives the run and Windows then refuses to unlink the binary,
	// failing the whole package after every test in it has passed.
	e, out, errOut := statuslineEnv(t, "", map[string]string{
		"CLAUDE_HOME":                t.TempDir(),
		"MYFW_STATUSLINE_NO_REFRESH": "1",
	}, "statusline")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d: %s%s", code, out.String(), errOut.String())
	}
	if out.String() == "" {
		t.Error("the default action produced no line")
	}
}
