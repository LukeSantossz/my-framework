// Package statusline renders and applies the status line contract defined in
// docs/standards/status_line.md.
//
// Five facts, in one order, across both agents. The contract binds the facts
// and their order; colours, glyphs and separators are explicitly the tool's
// business and carry no meaning.
//
// This is the Go port of scripts/statusline/claude-statusline.js. Porting it
// removes the last hard Node dependency, which is what made a machine without
// Node have its two agents diverge — a defect the README recorded rather than
// fixed.
//
// Nothing here fails. The status line is not a place to fail: an exception
// where the bar goes replaces every fact with an error message, which is worse
// than losing one. Every read degrades to a placeholder instead.
package statusline

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Payload is the session JSON Claude Code writes to the renderer's stdin.
type Payload struct {
	Model struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	Workspace      struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
	Version string `json:"version"`
}

// Quota is contract fact 4. Known separates "utilization is zero" from "no
// reading was taken": rendering an unread quota as 0% reports a figure nobody
// observed.
type Quota struct {
	Known       bool
	FiveHour    float64
	HasSevenDay bool
	SevenDay    float64
	ResetIn     time.Duration
}

// Facts is the contract, resolved. One struct so the order lives in exactly one
// place — Render walks it top to bottom.
type Facts struct {
	Model  string // 1. model...
	Effort string //    ...with reasoning effort

	ContextTokens int // 2. context used...
	WindowTokens  int //    ...against the window
	WindowLabel   string

	SpentTokens int // 3. tokens spent

	Quota Quota // 4. quota

	Dir    string // 5. location...
	Branch string //    ...and branch

	// Cache and Version are carried so the caller can decide whether to
	// schedule a refresh. Rendering never waits on the network.
	Cache   Cache
	Version string
	Cwd     string
}

// Options are the renderer's inputs. Home, Now and Branch are injected so a
// test never reads the operator's real agent configuration or shells out.
type Options struct {
	Home   string
	Now    func() time.Time
	Branch func(dir string) string
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// Read resolves the five facts from a session payload. A payload it cannot
// parse yields the degraded facts rather than an error: there is no caller that
// could do anything better with one.
func Read(raw []byte, opts Options) Facts {
	var p Payload
	_ = json.Unmarshal(raw, &p)

	facts := Facts{
		Model:   firstNonEmpty(p.Model.DisplayName, "model?"),
		Effort:  effortFrom(opts.Home),
		Version: p.Version,
	}

	context, spent := readTranscript(p.TranscriptPath)
	facts.ContextTokens, facts.SpentTokens = context, spent
	facts.WindowTokens, facts.WindowLabel = windowFor(p.Model.ID)

	cache, ok := ReadCache(opts.Home)
	facts.Cache = cache
	if ok {
		facts.Quota = cache.Quota(opts.now())
	}

	cwd := firstNonEmpty(p.Cwd, p.Workspace.CurrentDir)
	facts.Cwd = cwd
	if cwd != "" {
		facts.Dir = filepath.Base(filepath.Clean(cwd))
	}
	branch := opts.Branch
	if branch == nil {
		branch = GitBranch
	}
	facts.Branch = branch(cwd)
	return facts
}

// Render writes the facts in contract order. The separator, the bar glyphs and
// the colours are this tool's choice; the order is not.
func Render(f Facts, color bool) string {
	p := palette{on: color}
	return strings.Join([]string{
		p.modelSegment(f),
		p.contextSegment(f),
		p.spentSegment(f),
		p.quotaSegment(f),
		p.locationSegment(f),
	}, p.gray(" | "))
}

// --- facts ------------------------------------------------------------------

// effortFrom reads the reasoning effort. It is settings state rather than
// session state, so the payload does not carry it and an unset one is simply
// omitted.
func effortFrom(home string) string {
	if home == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(home, "settings.json"))
	if err != nil {
		return ""
	}
	var settings struct {
		EffortLevel string `json:"effortLevel"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return ""
	}
	return settings.EffortLevel
}

// windowFor reads the context window from the model id. The 1M window is a
// model variant marked in the id rather than a number the payload reports.
func windowFor(id string) (int, string) {
	if strings.Contains(strings.ToLower(id), "[1m]") {
		return 1_000_000, "1M"
	}
	return 200_000, "200k"
}

// readTranscript returns both token facts in one pass:
//
//	context — the last main-chain turn's whole input, which is what occupies
//	          the window right now;
//	spent   — input + output + cache creation over the session, excluding cache
//	          reads. A cache read is a re-read of context already counted;
//	          including it would inflate the figure by the size of the
//	          conversation every turn and make it incomparable to what Codex
//	          reports under the same name.
//
// Sidechain turns are subagent sessions and belong to neither.
func readTranscript(path string) (context, spent int) {
	if path == "" {
		return 0, 0
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// A single transcript line carries a whole turn and outgrows the default
	// 64KB buffer long before the session does.
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry struct {
			Type        string `json:"type"`
			IsSidechain bool   `json:"isSidechain"`
			Message     struct {
				Usage struct {
					Input         int `json:"input_tokens"`
					Output        int `json:"output_tokens"`
					CacheCreation int `json:"cache_creation_input_tokens"`
					CacheRead     int `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.Type != "assistant" || entry.IsSidechain {
			continue
		}
		u := entry.Message.Usage
		spent += u.Input + u.Output + u.CacheCreation
		context = u.Input + u.CacheCreation + u.CacheRead
	}
	return context, spent
}

// GitBranch resolves the checked-out branch, falling back to the short commit
// on a detached HEAD and to nothing outside a repository. It is the one
// external call the renderer makes, and its absence is a declared degradation.
func GitBranch(dir string) string {
	if dir == "" {
		return ""
	}
	for _, args := range [][]string{
		{"symbolic-ref", "--short", "HEAD"},
		{"rev-parse", "--short", "HEAD"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		if branch := strings.TrimSpace(string(out)); branch != "" {
			return branch
		}
	}
	return ""
}

// --- rendering --------------------------------------------------------------

type palette struct{ on bool }

func (p palette) esc(code string) string {
	if !p.on {
		return ""
	}
	return "\x1b[" + code + "m"
}

func (p palette) wrap(code, s string) string {
	if !p.on {
		return s
	}
	return p.esc(code) + s + p.esc("0")
}

func (p palette) dim(s string) string  { return p.wrap("2", s) }
func (p palette) gray(s string) string { return p.wrap("90", s) }
func (p palette) cyan(s string) string { return p.wrap("1;36", s) }

// heat warms as the resource runs out, so a glance at the bar is enough.
func (p palette) heat(pct, strongAt float64, s string) string {
	switch {
	case pct >= strongAt:
		return p.wrap("1;31", s)
	case pct >= 75:
		return p.wrap("38;5;208", s)
	case pct >= 50:
		return p.wrap("33", s)
	default:
		return p.wrap("32", s)
	}
}

func (p palette) modelSegment(f Facts) string {
	model := p.cyan(f.Model)
	if f.Effort == "" {
		return model
	}
	return model + " " + p.dim(f.Effort)
}

func (p palette) contextSegment(f Facts) string {
	window := f.WindowTokens
	if window <= 0 {
		window = 200_000
	}
	pct := float64(f.ContextTokens) / float64(window) * 100
	return p.dim("ctx") + " " +
		p.heat(pct, 80, bar(pct)) + " " +
		p.heat(pct, 80, fmt.Sprintf("%.0f%%", pct)) + " " +
		p.dim(fmt.Sprintf("%s/%s", formatTokens(f.ContextTokens), orLabel(f.WindowLabel)))
}

func (p palette) spentSegment(f Facts) string {
	return p.wrap("32", formatTokens(f.SpentTokens)+" tok")
}

func (p palette) quotaSegment(f Facts) string {
	if !f.Quota.Known {
		return p.dim("usage n/a")
	}
	five := f.Quota.FiveHour
	segment := p.dim("usage") + " " +
		p.heat(five, 90, fmt.Sprintf("%s %.0f%% 5h", bar(five), five))
	if f.Quota.HasSevenDay {
		segment += "  " + p.heat(f.Quota.SevenDay, 90, fmt.Sprintf("%.0f%% 7d", f.Quota.SevenDay))
	}
	if f.Quota.ResetIn > 0 {
		hours := int(f.Quota.ResetIn / time.Hour)
		minutes := int(f.Quota.ResetIn/time.Minute) % 60
		segment += " " + p.dim(fmt.Sprintf("(reset %dh%02d)", hours, minutes))
	}
	return segment
}

func (p palette) locationSegment(f Facts) string {
	dir := f.Dir
	if dir == "" {
		dir = "?"
	}
	if f.Branch == "" {
		return p.cyan(dir)
	}
	return p.cyan(dir) + p.dim(":") + f.Branch
}

func bar(pct float64) string {
	const cells = 10
	filled := int(pct/100*cells + 0.5)
	if filled < 0 {
		filled = 0
	}
	if filled > cells {
		filled = cells
	}
	return strings.Repeat("■", filled) + strings.Repeat("□", cells-filled)
}

func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func orLabel(s string) string {
	if s == "" {
		return "200k"
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
