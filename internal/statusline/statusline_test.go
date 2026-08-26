package statusline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

func fixedNow() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) }

// payload builds the session JSON Claude Code hands the renderer on stdin.
func payload(t *testing.T, transcript string) []byte {
	t.Helper()
	return []byte(fmt.Sprintf(`{
	  "model": {"id": "claude-opus-5[1m]", "display_name": "Opus 5 (1M context)"},
	  "transcript_path": %q,
	  "cwd": %q,
	  "workspace": {"current_dir": %q},
	  "version": "2.1.161"
	}`, filepath.ToSlash(transcript), "/work/repo", "/work/repo"))
}

func home(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// --- render -----------------------------------------------------------------

func TestRendersTheFiveContractFactsInContractOrder(t *testing.T) {
	// The order is normative in status_line.md: a Developer who reads the third
	// field as spend must not have to relearn it per tool. The golden pins the
	// whole line so a reordering fails here instead of agreeing with itself.
	claudeHome := home(t, map[string]string{"settings.json": `{"effortLevel": "high"}`})
	facts := Read(payload(t, "testdata/transcript.jsonl"), Options{
		Home:   claudeHome,
		Now:    fixedNow,
		Branch: func(string) string { return "feat/harness" },
	})
	got := Render(facts, false)

	golden := filepath.Join("testdata", "contract.golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading the golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("the rendered line drifted from the contract\n got: %s\nwant: %s", got, want)
	}

	// Order, asserted independently of the golden's exact spelling.
	positions := []int{
		strings.Index(got, "Opus 5"),
		strings.Index(got, "ctx"),
		strings.Index(got, "tok"),
		strings.Index(got, "usage"),
		strings.Index(got, "repo:"),
	}
	for i, p := range positions {
		if p < 0 {
			t.Fatalf("contract fact %d is missing from %q", i+1, got)
		}
		if i > 0 && p < positions[i-1] {
			t.Errorf("contract fact %d appears before fact %d: %q", i+1, i, got)
		}
	}
}

func TestCountsSpendAndContextTheWayTheContractDefinesThem(t *testing.T) {
	// Spend excludes cache reads (a re-read of context already counted) so the
	// figure stays comparable to what Codex reports under the same name;
	// context is the last main-chain turn's full input. A subagent's usage is
	// neither.
	facts := Read(payload(t, "testdata/transcript.jsonl"), Options{Now: fixedNow})
	if facts.SpentTokens != 1000+500+2000+1200+800+3000 {
		t.Errorf("spent = %d; a sidechain turn or a cache read leaked into it", facts.SpentTokens)
	}
	if facts.ContextTokens != 1200+3000+300000 {
		t.Errorf("context = %d; it must be the last main-chain turn's whole input", facts.ContextTokens)
	}
	if facts.WindowTokens != 1_000_000 || facts.WindowLabel != "1M" {
		t.Errorf("window = %d %q; the 1M variant is marked in the model id", facts.WindowTokens, facts.WindowLabel)
	}
}

func TestRendersWithoutNodeInstalled(t *testing.T) {
	// The whole point of the port: nothing here shells out to a runtime. The
	// branch lookup is the only external call and it is injected, so a render
	// on a machine with neither node nor git still produces the line.
	facts := Read(payload(t, "testdata/transcript.jsonl"), Options{
		Now:    fixedNow,
		Branch: func(string) string { return "" },
	})
	line := Render(facts, false)
	if line == "" {
		t.Fatal("the renderer produced nothing")
	}
	if strings.Contains(line, "repo:") {
		t.Errorf("a branch that could not be read must degrade to the directory alone: %q", line)
	}
	if !strings.Contains(line, "repo") {
		t.Errorf("the location fact lost the directory too: %q", line)
	}
}

func TestAnUnreadablePayloadStillProducesALine(t *testing.T) {
	// An exception where the status bar goes replaces every fact with an error
	// message, which is worse than losing one.
	line := Render(Read([]byte("not json at all"), Options{Now: fixedNow}), false)
	if line == "" {
		t.Fatal("a malformed payload emptied the status bar")
	}
	if !strings.Contains(line, "ctx") || !strings.Contains(line, "usage") {
		t.Errorf("the degraded line dropped contract facts: %q", line)
	}
}

func TestReportsUsageAsUnavailableRatherThanZeroWhenTheQuotaSourceCannotBeRead(t *testing.T) {
	// Zero utilization and no reading are opposite facts. Printing 0% for a
	// cache that does not exist reports a quota that was never observed.
	facts := Read(payload(t, "testdata/transcript.jsonl"), Options{Home: t.TempDir(), Now: fixedNow})
	if facts.Quota.Known {
		t.Fatal("a missing cache was read as a known quota")
	}
	line := Render(facts, false)
	if !strings.Contains(line, "usage n/a") {
		t.Errorf("the quota fact did not degrade to a placeholder: %q", line)
	}
	if strings.Contains(line, "0% 5h") {
		t.Errorf("an unread quota was rendered as zero utilization: %q", line)
	}
}

func TestAKnownQuotaRendersBothWindowsAndTheReset(t *testing.T) {
	cache := Cache{
		FiveHour: &Window{Util: 42, Reset: fixedNow().Add(90 * time.Minute).Format(time.RFC3339)},
		SevenDay: &Window{Util: 12},
	}
	body, _ := json.Marshal(cache)
	dir := home(t, map[string]string{CacheFileName: string(body)})
	facts := Read(payload(t, "testdata/transcript.jsonl"), Options{Home: dir, Now: fixedNow})
	line := Render(facts, false)
	for _, want := range []string{"42% 5h", "12% 7d", "reset 1h30"} {
		if !strings.Contains(line, want) {
			t.Errorf("the quota fact omits %q: %q", want, line)
		}
	}
}

func TestColorIsOptionalAndNeverChangesTheFacts(t *testing.T) {
	// status_line.md fixes the facts and their order; colours and glyphs are
	// explicitly the tool's business, so they must not carry meaning.
	facts := Read(payload(t, "testdata/transcript.jsonl"), Options{Now: fixedNow, Branch: func(string) string { return "main" }})
	plain := Render(facts, false)
	colored := Render(facts, true)
	if !strings.Contains(colored, "\x1b[") {
		t.Error("colour was requested and no escape sequence was emitted")
	}
	if stripANSI(colored) != plain {
		t.Errorf("colour changed the text:\n plain: %q\nstripped: %q", plain, stripANSI(colored))
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// --- applying the contract --------------------------------------------------

const divergentCodex = `model = "gpt-5.6-terra"

[features]
memories = true

[tui]
status_line = [
  "model",
  "current-dir",
]
theme = "monokai-extended-origin"
status_line_use_colors = false

[tui.model_availability_nux]
"gpt-5.5" = 4
`

func TestAppliesTheContractToBothAgentConfigurationsAndBacksUpADivergentOne(t *testing.T) {
	codexHome := home(t, map[string]string{"config.toml": divergentCodex})
	claudeHome := home(t, map[string]string{"settings.json": `{
  "model": "opus[1m]",
  "statusLine": {"type": "command", "command": "node /somewhere/personal.js"},
  "permissions": {"defaultMode": "auto"}
}`})

	codex, err := ApplyCodex(codexHome, fixedNow())
	if err != nil {
		t.Fatalf("ApplyCodex: %v", err)
	}
	if codex.Action != ActionWritten {
		t.Errorf("codex action = %q, want %q", codex.Action, ActionWritten)
	}
	if codex.Backup == "" {
		t.Error("a divergent codex config was replaced with no backup beside it")
	}
	if backup, readErr := os.ReadFile(codex.Backup); readErr != nil || string(backup) != divergentCodex {
		t.Errorf("the backup is not a faithful copy of the original: %v", readErr)
	}

	body, err := os.ReadFile(filepath.Join(codexHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, CodexStatusLine) {
		t.Errorf("the canonical segment list is missing:\n%s", got)
	}
	if !strings.Contains(got, "status_line_use_colors = true") {
		t.Errorf("colours were not enabled:\n%s", got)
	}
	if strings.Contains(got, `"model",`) || strings.Contains(got, "status_line_use_colors = false") {
		t.Errorf("the divergent values survived:\n%s", got)
	}
	// A line-oriented rewrite that does not consume a multi-line array in full
	// leaves a stray entry or a dangling bracket behind.
	for _, remnant := range []string{"\n]\n", `"current-dir",` + "\n"} {
		if strings.Contains(got, remnant) {
			t.Errorf("a remnant of the multi-line array survived (%q):\n%s", remnant, got)
		}
	}
	// Everything unrelated is left intact, including a subsection whose header
	// starts with the same word as the section being edited.
	for _, keep := range []string{`model = "gpt-5.6-terra"`, "[features]", "memories = true",
		`theme = "monokai-extended-origin"`, "[tui.model_availability_nux]", `"gpt-5.5" = 4`} {
		if !strings.Contains(got, keep) {
			t.Errorf("unrelated content was lost (%q):\n%s", keep, got)
		}
	}

	claude, err := ApplyClaude(claudeHome, "/usr/local/bin/mf statusline render", fixedNow())
	if err != nil {
		t.Fatalf("ApplyClaude: %v", err)
	}
	if claude.Action != ActionWritten || claude.Backup == "" {
		t.Errorf("claude result = %+v; a replaced settings file needs a backup", claude)
	}
	settings := map[string]any{}
	raw, _ := os.ReadFile(filepath.Join(claudeHome, "settings.json"))
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("the written settings are not JSON: %v", err)
	}
	line, _ := settings["statusLine"].(map[string]any)
	if line["type"] != "command" || line["command"] != "/usr/local/bin/mf statusline render" {
		t.Errorf("statusLine = %v", settings["statusLine"])
	}
	if settings["model"] != "opus[1m]" {
		t.Error("an unrelated top-level key was lost")
	}
	if perms, ok := settings["permissions"].(map[string]any); !ok || perms["defaultMode"] != "auto" {
		t.Error("a nested key was lost")
	}
}

func TestCreatesBothConfigurationsWhenNeitherExists(t *testing.T) {
	codexHome, claudeHome := t.TempDir(), t.TempDir()
	if _, err := ApplyCodex(filepath.Join(codexHome, "nested"), fixedNow()); err != nil {
		t.Fatalf("ApplyCodex: %v", err)
	}
	if _, err := ApplyClaude(filepath.Join(claudeHome, "nested"), "mf statusline render", fixedNow()); err != nil {
		t.Fatalf("ApplyClaude: %v", err)
	}
	toml, err := os.ReadFile(filepath.Join(codexHome, "nested", "config.toml"))
	if err != nil || !strings.Contains(string(toml), "[tui]") {
		t.Errorf("a fresh codex config was not created with a [tui] section: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(claudeHome, "nested", "settings.json")); err != nil {
		t.Errorf("a fresh settings.json was not created: %v", err)
	}
}

func TestLeavesAMatchingConfigurationUntouched(t *testing.T) {
	// Re-running the activation must not bury the original under generated
	// copies, so a conformant file is neither rewritten nor backed up.
	codexHome, claudeHome := t.TempDir(), t.TempDir()
	command := "mf statusline render"
	if _, err := ApplyCodex(codexHome, fixedNow()); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyClaude(claudeHome, command, fixedNow()); err != nil {
		t.Fatal(err)
	}
	firstToml, _ := os.ReadFile(filepath.Join(codexHome, "config.toml"))
	firstSettings, _ := os.ReadFile(filepath.Join(claudeHome, "settings.json"))

	codex, err := ApplyCodex(codexHome, fixedNow().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	claude, err := ApplyClaude(claudeHome, command, fixedNow().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if codex.Action != ActionUnchanged || claude.Action != ActionUnchanged {
		t.Errorf("a conformant configuration was rewritten: codex=%q claude=%q", codex.Action, claude.Action)
	}
	if codex.Backup != "" || claude.Backup != "" {
		t.Errorf("a second backup was written for an unchanged file: %q %q", codex.Backup, claude.Backup)
	}
	secondToml, _ := os.ReadFile(filepath.Join(codexHome, "config.toml"))
	secondSettings, _ := os.ReadFile(filepath.Join(claudeHome, "settings.json"))
	if string(firstToml) != string(secondToml) || string(firstSettings) != string(secondSettings) {
		t.Error("the second run changed a file it reported as unchanged")
	}
}

func TestRefusesToRewriteSettingsThatAreNotAJsonObject(t *testing.T) {
	// Rewriting a file that cannot be parsed discards every key in it, which is
	// worse than not applying the contract.
	broken := `{ "model": "opus", oops }`
	dir := home(t, map[string]string{"settings.json": broken})
	if _, err := ApplyClaude(dir, "mf statusline render", fixedNow()); err == nil {
		t.Fatal("an unparseable settings.json was rewritten")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	if string(after) != broken {
		t.Error("the unparseable file was modified")
	}
}

func TestRefusesASettingsFileThatIsValidJsonButNotAnObject(t *testing.T) {
	dir := home(t, map[string]string{"settings.json": `["a", "b"]`})
	if _, err := ApplyClaude(dir, "mf statusline render", fixedNow()); err == nil {
		t.Fatal("a JSON array was accepted as a settings object")
	}
}

// tuiTableOf parses a rewritten configuration the way Codex itself will. A
// rewrite that appends a second [tui] table produces a file TOML rejects
// outright for defining the same key twice, and nothing says so: the apply
// reported success and the status line simply stops.
func tuiTableOf(t *testing.T, config string) map[string]any {
	t.Helper()
	var parsed map[string]any
	if _, err := toml.Decode(config, &parsed); err != nil {
		t.Fatalf("the rewritten configuration is not parseable TOML: %v\n%s", err, config)
	}
	tui, ok := parsed["tui"].(map[string]any)
	if !ok {
		t.Fatalf("no [tui] table survived the rewrite:\n%s", config)
	}
	return tui
}

func TestRecognisesTheTuiHeaderInEveryFormTomlAllows(t *testing.T) {
	// A header is not simply a line wrapped in brackets: TOML allows padding
	// inside the brackets and a comment after them, and both name the same
	// table. Missing one leaves the section unseen and a second [tui] appended.
	cases := []struct {
		name     string
		existing string
		kept     string // the header, which is the user's line to keep verbatim
	}{
		{"a comment after the header", "[tui] # my terminal settings\ntheme = \"dark\"\n", "[tui] # my terminal settings"},
		{"padding inside the brackets", "[ tui ]\ntheme = \"dark\"\n", "[ tui ]"},
		{"a header after another table", "[features]\nmemories = true\n\n[tui] # mine\ntheme = \"dark\"\n", "[tui] # mine"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := codexConfigWithContract(c.existing)
			tui := tuiTableOf(t, got)
			if tui["status_line_use_colors"] != true {
				t.Errorf("colours were not enabled:\n%s", got)
			}
			if tui["theme"] != "dark" {
				t.Errorf("an unrelated key in the section was lost:\n%s", got)
			}
			if !strings.Contains(got, CodexStatusLine) {
				t.Errorf("the canonical segment list is missing:\n%s", got)
			}
			if !strings.Contains(got, c.kept) {
				t.Errorf("the header the user wrote (%q) was not kept as written:\n%s", c.kept, got)
			}
		})
	}
}

func TestStripsTheStatusLineKeysOnlyFromTheTuiSection(t *testing.T) {
	// A header the rewrite does not recognise reads as ordinary content, so it
	// stays in whichever table it thought it was in and strips the two keys out
	// of the next one — a section it was told to leave alone.
	existing := "[tui]\ntheme = \"dark\"\n\n[experimental] # not ours\nstatus_line = \"hand rolled\"\nstatus_line_use_colors = false\n"
	got := codexConfigWithContract(existing)

	var parsed map[string]any
	if _, err := toml.Decode(got, &parsed); err != nil {
		t.Fatalf("the rewritten configuration is not parseable TOML: %v\n%s", err, got)
	}
	other, ok := parsed["experimental"].(map[string]any)
	if !ok {
		t.Fatalf("an unrelated table was lost:\n%s", got)
	}
	if other["status_line"] != "hand rolled" || other["status_line_use_colors"] != false {
		t.Errorf("an unrelated table's own keys were rewritten: %+v\n%s", other, got)
	}
}

func TestASecondApplyInTheSameSecondKeepsTheFirstBackup(t *testing.T) {
	// The backup name carries a one-second timestamp, so an apply landing in the
	// same second as the last one would overwrite the copy of the configuration
	// the Developer actually hand-wrote — the one thing that makes replacing it
	// recoverable.
	dir := home(t, map[string]string{"config.toml": divergentCodex})
	first, err := ApplyCodex(dir, fixedNow())
	if err != nil {
		t.Fatalf("ApplyCodex: %v", err)
	}
	hand := "[tui]\nstatus_line = [\"model\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(hand), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := ApplyCodex(dir, fixedNow())
	if err != nil {
		t.Fatalf("ApplyCodex: %v", err)
	}
	if first.Backup == second.Backup {
		t.Fatalf("both applies claimed the same backup name %q", first.Backup)
	}
	body, err := os.ReadFile(first.Backup)
	if err != nil || string(body) != divergentCodex {
		t.Errorf("the first backup no longer holds the original: %v\n%s", err, body)
	}
}

func TestRevertRestoresWhatTheLastApplyReplaced(t *testing.T) {
	// Apply pushes a backup and revert pops it, so two applies can be walked
	// back one at a time; restoring without removing would make every revert
	// after the first a no-op that reports success.
	dir := home(t, map[string]string{"config.toml": divergentCodex})
	target := filepath.Join(dir, "config.toml")
	if _, err := ApplyCodex(dir, fixedNow()); err != nil {
		t.Fatalf("ApplyCodex: %v", err)
	}
	hand := "[tui]\nstatus_line = [\"model\"]\n"
	if err := os.WriteFile(target, []byte(hand), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyCodex(dir, fixedNow()); err != nil {
		t.Fatalf("ApplyCodex: %v", err)
	}

	for _, want := range []string{hand, divergentCodex} {
		res, err := Revert(target)
		if err != nil {
			t.Fatalf("Revert: %v", err)
		}
		if res.Action != ActionRestored {
			t.Fatalf("action = %q, want %q", res.Action, ActionRestored)
		}
		body, _ := os.ReadFile(target)
		if string(body) != want {
			t.Errorf("restored\n%s\nwant\n%s", body, want)
		}
	}

	spent, err := Revert(target)
	if err != nil {
		t.Fatalf("Revert with nothing left to restore: %v", err)
	}
	if spent.Action != ActionUnchanged {
		t.Errorf("action = %q, want %q when no backup is left", spent.Action, ActionUnchanged)
	}
}

func TestTheCodexSegmentListIsTheContractInOrder(t *testing.T) {
	// Written out here literally rather than compared against the constant's
	// own construction, so a silent reordering fails instead of agreeing with
	// itself.
	want := `status_line = ["model-with-reasoning", "context-used", "context-window-size", "used-tokens", "five-hour-limit", "weekly-limit", "current-dir", "git-branch"]`
	if CodexStatusLine != want {
		t.Errorf("CodexStatusLine = %s\nwant %s", CodexStatusLine, want)
	}
}

func TestReadTranscriptReportsNothingWhenItCouldNotFinish(t *testing.T) {
	// A single turn carrying a large tool result outgrows the buffer cap. The
	// loop then ended early and the accumulated figure was drawn as though the
	// whole transcript had been read — the one fact in this package that
	// degraded to a number instead of to the placeholder.
	path := filepath.Join(t.TempDir(), "session.jsonl")
	var b strings.Builder
	b.WriteString(`{"type":"assistant","message":{"usage":{"input_tokens":10,"output_tokens":5}}}` + "\n")
	b.WriteString(`{"type":"assistant","message":{"usage":{"input_tokens":1},"pad":"`)
	b.WriteString(strings.Repeat("x", 9*1024*1024))
	b.WriteString(`"}}` + "\n")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	context, spent := readTranscript(path)
	if context != 0 || spent != 0 {
		t.Errorf("a partly-read transcript reported ctx=%d spent=%d, want the placeholder", context, spent)
	}
}
