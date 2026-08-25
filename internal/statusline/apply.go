package statusline

import (
	"bytes"
	"encoding/json"
	"errors"
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

	tuiTable  = "tui"
	tuiHeader = "[" + tuiTable + "]"
)

// Action says what applying the contract did. "unchanged" is a distinct outcome
// rather than a quiet success: re-running the activation must not bury the
// original under generated copies, and the only way to see that it did not is
// for the command to say so.
type Action string

const (
	ActionWritten   Action = "written"
	ActionUnchanged Action = "unchanged"
	ActionRestored  Action = "restored"
)

// Result is one configuration file's outcome.
type Result struct {
	Target string
	Action Action
	Backup string
}

// backupAttempts caps the search for a free backup name. The timestamp only
// resolves to the second, so the sequence exists for applies that land inside
// one; a hundred of them is already past what any sequence of applies produces.
const backupAttempts = 100

// writeBackup copies what is about to be replaced to a name nothing else holds.
//
// The name is claimed by creating it exclusively rather than by checking first:
// two applies in the same second share a timestamp, and the loser of a plain
// write would silently overwrite the copy of the configuration the Developer
// actually hand-wrote — the one thing that makes replacing it recoverable.
//
// The sequence is fixed-width so the names sort in the order they were taken,
// which is what lets Revert find the newest.
func writeBackup(target string, now time.Time, body []byte) (string, error) {
	base := target + ".bak." + now.Format("20060102150405")
	for n := 0; n < backupAttempts; n++ {
		name := base
		if n > 0 {
			name = fmt.Sprintf("%s.%02d", base, n)
		}
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := f.Write(body); err != nil {
			f.Close()
			return "", err
		}
		if err := f.Close(); err != nil {
			return "", err
		}
		return name, nil
	}
	return "", fmt.Errorf("no free backup name beside %s", target)
}

// Revert restores the newest backup over the configuration it was taken from.
//
// Apply pushes a backup and revert pops it, so a machine can be walked back
// through however many applies it took: restoring without removing would make
// every revert after the first a no-op reporting success. Nothing is lost by
// removing it, because what it held is now the file itself.
func Revert(target string) (Result, error) {
	res := Result{Target: target}
	newest, err := newestBackup(target)
	if err != nil {
		return res, err
	}
	if newest == "" {
		res.Action = ActionUnchanged
		return res, nil
	}
	body, err := os.ReadFile(newest)
	if err != nil {
		return res, fmt.Errorf("cannot read %s: %w", newest, err)
	}
	if err := os.WriteFile(target, body, 0o644); err != nil {
		return res, fmt.Errorf("cannot restore %s: %w", target, err)
	}
	if err := os.Remove(newest); err != nil {
		return res, fmt.Errorf("cannot remove %s once restored: %w", newest, err)
	}
	res.Action, res.Backup = ActionRestored, newest
	return res, nil
}

// newestBackup finds the last backup taken of target, or "" when there is none.
// The directory is read rather than globbed because a `[` anywhere in the
// operator's home directory is a glob metacharacter and would match nothing.
func newestBackup(target string) (string, error) {
	dir := filepath.Dir(target)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("cannot read %s: %w", dir, err)
	}
	prefix := filepath.Base(target) + ".bak."
	newest := ""
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && e.Name() > newest {
			newest = e.Name()
		}
	}
	if newest == "" {
		return "", nil
	}
	return filepath.Join(dir, newest), nil
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
		backup, err := writeBackup(target, now, existing)
		if err != nil {
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
		return tuiHeader + "\n" + CodexStatusLine + "\n" + CodexStatusColors + "\n"
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

		if table, isHeader := tomlTable(trimmed); isHeader {
			// `[tui.model_availability_nux]` is a different table and must not
			// be mistaken for the section being edited.
			inTui = table == tuiTable
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
		out = append(out, "", tuiHeader, CodexStatusLine, CodexStatusColors, "")
	}
	return strings.Join(out, "\n")
}

// tomlTable reports which table a line opens, and whether it opens one at all.
//
// A header is not simply a line wrapped in brackets: TOML allows padding inside
// them and a comment after them, and `[tui] # my terminal settings` names the
// same table as `[tui]`. Reading either as ordinary content costs more than the
// section it missed — the rewrite stays in whichever table it thought it was in
// and strips the two keys from the next one, and the [tui] it never saw gets a
// second one appended, which leaves Codex a file it refuses to parse while the
// command reports success.
func tomlTable(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, "[") {
		return "", false
	}
	header := strings.TrimSpace(stripComment(trimmed))
	if len(header) < 2 || !strings.HasSuffix(header, "]") {
		return "", false
	}
	name := strings.TrimSpace(header[1 : len(header)-1])
	return strings.Trim(name, `"'`), true
}

// stripComment cuts a line at the `#` that starts a comment, leaving one inside
// a quoted key alone: `["a#b"]` names a table rather than commenting one out.
func stripComment(line string) string {
	quote := rune(0)
	for i, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '#':
			return line[:i]
		}
	}
	return line
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
		backup, err := writeBackup(target, now, existing)
		if err != nil {
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
