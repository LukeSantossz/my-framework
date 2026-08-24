package statusline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The Codex rendering of the contract. The segment names are read from an
// installed build rather than a published schema: an upgrade that renames one
// leaves this silently ignored — the line degrades, nothing breaks — and
// porting to Go does not improve that.
const (
	CodexStatusLine   = `status_line = ["model-with-reasoning", "context-used", "context-window-size", "used-tokens", "five-hour-limit", "weekly-limit", "current-dir", "git-branch"]`
	CodexStatusColors = `status_line_use_colors = true`

	tuiSection = "[tui]"
)

// Action says what applying the contract did. "unchanged" is a distinct outcome
// rather than a quiet success: re-running the activation must not bury the
// original under generated copies, and the only way to see that it did not is
// for the command to say so.
type Action string

const (
	ActionWritten   Action = "written"
	ActionUnchanged Action = "unchanged"
)

// Result is one configuration file's outcome.
type Result struct {
	Target string
	Action Action
	Backup string
}

func backupPath(target string, now time.Time) string {
	return target + ".bak." + now.Format("20060102150405")
}

// ApplyCodex writes the contract into Codex's TOML, leaving every unrelated
// key, section and subsection intact.
func ApplyCodex(codexHome string, now time.Time) (Result, error) {
	target := filepath.Join(codexHome, "config.toml")
	res := Result{Target: target}

	existing, err := os.ReadFile(target)
	if err != nil && !os.IsNotExist(err) {
		return res, fmt.Errorf("cannot read %s: %w", target, err)
	}
	next := codexConfigWithContract(string(existing))
	if string(existing) == next {
		res.Action = ActionUnchanged
		return res, nil
	}

	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		return res, fmt.Errorf("cannot create %s: %w", codexHome, err)
	}
	// Replacing a divergent configuration is the point — a machine already
	// carrying a hand-rolled status line is the one that most needed
	// standardizing — and the backup is what keeps the replacement recoverable.
	if len(existing) > 0 {
		backup := backupPath(target, now)
		if err := os.WriteFile(backup, existing, 0o644); err != nil {
			return res, fmt.Errorf("cannot back up %s: %w", target, err)
		}
		res.Backup = backup
	}
	if err := os.WriteFile(target, []byte(next), 0o644); err != nil {
		return res, fmt.Errorf("cannot write %s: %w", target, err)
	}
	res.Action = ActionWritten
	return res, nil
}

// codexConfigWithContract rewrites the [tui] section's two status line keys and
// nothing else.
//
// It is line-oriented rather than a TOML round-trip because a round-trip
// rewrites the whole file: comments, ordering and formatting the user chose are
// not ours to normalise in a file we do not own.
func codexConfigWithContract(existing string) string {
	if strings.TrimSpace(existing) == "" {
		return tuiSection + "\n" + CodexStatusLine + "\n" + CodexStatusColors + "\n"
	}

	lines := strings.Split(existing, "\n")
	out := make([]string, 0, len(lines)+2)
	inTui, sawTui, inserted, skippingArray := false, false, false, false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// A multi-line array has to be consumed in full, or a line-oriented
		// rewrite leaves a stray entry or a dangling bracket behind.
		if skippingArray {
			if strings.Contains(trimmed, "]") {
				skippingArray = false
			}
			continue
		}

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			// `[tui.model_availability_nux]` is a different table and must not
			// be mistaken for the section being edited.
			inTui = trimmed == tuiSection
			if inTui {
				sawTui = true
			}
			out = append(out, line)
			if inTui && !inserted {
				out = append(out, CodexStatusLine, CodexStatusColors)
				inserted = true
			}
			continue
		}

		if inTui {
			switch tomlKey(trimmed) {
			case "status_line":
				if strings.Contains(trimmed, "[") && !strings.Contains(trimmed, "]") {
					skippingArray = true
				}
				continue
			case "status_line_use_colors":
				continue
			}
		}
		out = append(out, line)
	}

	if !sawTui {
		for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			out = out[:len(out)-1]
		}
		out = append(out, "", tuiSection, CodexStatusLine, CodexStatusColors, "")
	}
	return strings.Join(out, "\n")
}

func tomlKey(line string) string {
	name, _, found := strings.Cut(line, "=")
	if !found {
		return ""
	}
	return strings.Trim(strings.TrimSpace(name), `"'`)
}

// ApplyClaude points Claude Code's status line at a command, leaving every
// other setting alone.
func ApplyClaude(claudeHome, command string, now time.Time) (Result, error) {
	target := filepath.Join(claudeHome, "settings.json")
	res := Result{Target: target}

	existing, err := os.ReadFile(target)
	if err != nil && !os.IsNotExist(err) {
		return res, fmt.Errorf("cannot read %s: %w", target, err)
	}

	settings := map[string]any{}
	if len(bytes.TrimSpace(existing)) > 0 {
		// Rewriting a settings.json that cannot be parsed would discard every
		// key in it, which is a worse outcome than not applying the contract.
		decoder := json.NewDecoder(bytes.NewReader(existing))
		decoder.UseNumber() // so a number the user wrote is not re-spelled
		if err := decoder.Decode(&settings); err != nil {
			return res, fmt.Errorf("%s is not a JSON object; fix or move it, then re-run (%w)", target, err)
		}
	}

	if current, ok := settings["statusLine"].(map[string]any); ok {
		if current["type"] == "command" && current["command"] == command {
			res.Action = ActionUnchanged
			return res, nil
		}
	}
	settings["statusLine"] = map[string]any{"type": "command", "command": command}

	// Key order is normalised by the encoder. The content is preserved and the
	// backup holds the file exactly as it was, so what is lost is formatting,
	// not settings.
	body, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return res, fmt.Errorf("cannot render %s: %w", target, err)
	}
	body = append(body, '\n')

	if err := os.MkdirAll(claudeHome, 0o755); err != nil {
		return res, fmt.Errorf("cannot create %s: %w", claudeHome, err)
	}
	if len(existing) > 0 {
		backup := backupPath(target, now)
		if err := os.WriteFile(backup, existing, 0o644); err != nil {
			return res, fmt.Errorf("cannot back up %s: %w", target, err)
		}
		res.Backup = backup
	}
	if err := os.WriteFile(target, body, 0o644); err != nil {
		return res, fmt.Errorf("cannot write %s: %w", target, err)
	}
	res.Action = ActionWritten
	return res, nil
}
