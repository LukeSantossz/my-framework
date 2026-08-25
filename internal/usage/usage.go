// Package usage counts what a review consumed.
//
// The buckets are disjoint because a single total cannot distinguish a cheap
// cached prefix from an expensive fresh one, and the gate's whole prompt design
// — stable instructions first, volatile diff last — depends on that difference
// being visible.
//
// Vendors do not report usage in one shape. Layouts and terminology differ, and
// some paths return none at all, so parsing is per wire shape and a backend that
// returns nothing yields an estimate marked as an estimate everywhere it
// appears. Reporting zero as a measured value would be a fabricated number,
// which ai_guidelines.md forbids outright.
package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Usage is one review's consumption, in disjoint buckets.
type Usage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens"`
	ReasoningTokens  int `json:"reasoning_tokens"`

	// Estimated marks a figure this tool derived rather than read. It travels
	// with the value through every aggregate: an estimate that looks like a
	// measurement corrupts everything built on it later.
	Estimated bool `json:"estimated"`

	// Known is false when nothing could be determined at all. A review still
	// stands when its accounting fails — accounting is observation, and losing
	// a review because the observation failed inverts their importance.
	Known bool `json:"known"`
}

func (u Usage) Total() int {
	return u.InputTokens + u.OutputTokens + u.CacheReadTokens + u.CacheWriteTokens + u.ReasoningTokens
}

// Add combines two figures. The result is estimated if either side was, because
// a total containing an estimate is an estimate.
func (u Usage) Add(other Usage) Usage {
	if !other.Known {
		return u
	}
	return Usage{
		InputTokens:      u.InputTokens + other.InputTokens,
		OutputTokens:     u.OutputTokens + other.OutputTokens,
		CacheReadTokens:  u.CacheReadTokens + other.CacheReadTokens,
		CacheWriteTokens: u.CacheWriteTokens + other.CacheWriteTokens,
		ReasoningTokens:  u.ReasoningTokens + other.ReasoningTokens,
		Estimated:        u.Estimated || other.Estimated,
		Known:            true,
	}
}

func (u Usage) String() string {
	if !u.Known {
		return "usage unknown"
	}
	parts := []string{fmt.Sprintf("in %d", u.InputTokens), fmt.Sprintf("out %d", u.OutputTokens)}
	if u.CacheReadTokens > 0 {
		parts = append(parts, fmt.Sprintf("cache-read %d", u.CacheReadTokens))
	}
	if u.CacheWriteTokens > 0 {
		parts = append(parts, fmt.Sprintf("cache-write %d", u.CacheWriteTokens))
	}
	if u.ReasoningTokens > 0 {
		parts = append(parts, fmt.Sprintf("reasoning %d", u.ReasoningTokens))
	}
	suffix := " (measured)"
	if u.Estimated {
		suffix = " (ESTIMATED, not measured)"
	}
	return strings.Join(parts, ", ") + suffix
}

// ParseOpenAI reads the openai-compatible usage object. Cached input is
// reported inside prompt_tokens by these providers, so it is subtracted out to
// keep the buckets disjoint rather than counted twice.
func ParseOpenAI(raw []byte) Usage {
	var parsed struct {
		Usage struct {
			PromptTokens        int `json:"prompt_tokens"`
			CompletionTokens    int `json:"completion_tokens"`
			PromptTokensDetails struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CompletionTokensDetails struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Usage{}
	}
	u := parsed.Usage
	if u.PromptTokens == 0 && u.CompletionTokens == 0 {
		return Usage{}
	}
	cached := u.PromptTokensDetails.CachedTokens
	input := u.PromptTokens - cached
	if input < 0 {
		input = 0
	}
	reasoning := u.CompletionTokensDetails.ReasoningTokens
	output := u.CompletionTokens - reasoning
	if output < 0 {
		output = 0
	}
	return Usage{
		InputTokens: input, OutputTokens: output,
		CacheReadTokens: cached, ReasoningTokens: reasoning,
		Known: true,
	}
}

// ParseAnthropic reads the anthropic usage object, which names its buckets
// differently and reports cache creation and cache reads separately.
func ParseAnthropic(raw []byte) Usage {
	var parsed struct {
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Usage{}
	}
	u := parsed.Usage
	if u.InputTokens == 0 && u.OutputTokens == 0 && u.CacheReadInputTokens == 0 {
		return Usage{}
	}
	return Usage{
		InputTokens: u.InputTokens, OutputTokens: u.OutputTokens,
		CacheWriteTokens: u.CacheCreationInputTokens,
		CacheReadTokens:  u.CacheReadInputTokens,
		Known:            true,
	}
}

// ParseGoogle reads the google usageMetadata object, whose field names differ
// again — the reason this is per-shape rather than one parser.
func ParseGoogle(raw []byte) Usage {
	var parsed struct {
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			CachedContentTokens  int `json:"cachedContentTokenCount"`
			ThoughtsTokenCount   int `json:"thoughtsTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Usage{}
	}
	u := parsed.UsageMetadata
	if u.PromptTokenCount == 0 && u.CandidatesTokenCount == 0 {
		return Usage{}
	}
	input := u.PromptTokenCount - u.CachedContentTokens
	if input < 0 {
		input = 0
	}
	return Usage{
		InputTokens: input, OutputTokens: u.CandidatesTokenCount,
		CacheReadTokens: u.CachedContentTokens, ReasoningTokens: u.ThoughtsTokenCount,
		Known: true,
	}
}

// charsPerToken is a rough English-text ratio. It is not a tokenizer and its
// accuracy is unknown and unclaimed, which is why every figure it produces is
// marked estimated and must never be compared against a measured one as though
// the two were the same unit.
const charsPerToken = 4

// Estimate derives a figure when a backend reported none.
func Estimate(promptChars, answerChars int) Usage {
	if promptChars <= 0 && answerChars <= 0 {
		return Usage{}
	}
	return Usage{
		InputTokens:  promptChars / charsPerToken,
		OutputTokens: answerChars / charsPerToken,
		Estimated:    true,
		Known:        true,
	}
}

// --- money ------------------------------------------------------------------

// Price is what one million tokens costs. Prices change constantly, so no table
// ships with this tool: a stale price presented as cost is worse than no cost at
// all, and a figure the framework cannot defend is not printed.
type Price struct {
	InputPerMillion  float64 `toml:"input_per_million" json:"input_per_million"`
	OutputPerMillion float64 `toml:"output_per_million" json:"output_per_million"`
	CachedPerMillion float64 `toml:"cached_per_million" json:"cached_per_million"`
}

type Table map[string]Price

// Cost returns money only when the user supplied a price for this model.
func (t Table) Cost(model string, u Usage) (float64, bool) {
	price, ok := t[model]
	if !ok || !u.Known {
		return 0, false
	}
	per := func(tokens int, rate float64) float64 {
		return float64(tokens) / 1_000_000 * rate
	}
	cached := price.CachedPerMillion
	if cached == 0 {
		cached = price.InputPerMillion
	}
	total := per(u.InputTokens, price.InputPerMillion) +
		per(u.CacheWriteTokens, price.InputPerMillion) +
		per(u.CacheReadTokens, cached) +
		per(u.OutputTokens+u.ReasoningTokens, price.OutputPerMillion)
	return total, true
}

// --- the running total ------------------------------------------------------

// Store is the cumulative counter. It lives beside the machine configuration
// rather than in the repository, so it answers "what did I spend" and never
// "what did this project cost".
type Store struct {
	Path string
}

type Totals struct {
	Runs  int   `json:"runs"`
	Usage Usage `json:"usage"`
}

func (s Store) Read() Totals {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return Totals{}
	}
	var t Totals
	if err := json.Unmarshal(data, &t); err != nil {
		return Totals{}
	}
	return t
}

// Add folds one run into the cumulative total.
//
// The update is a read-modify-write of a file that is not private to this
// process: r2 and r3 run as parallel CI jobs against the same store, and two
// runs finishing together would otherwise leave only one of them recorded. That
// loss is silent — the survivor's file still parses and still reads as a
// plausible total — so the exclusion is taken rather than hoped for.
func (s Store) Add(u Usage) error {
	if !u.Known {
		return nil
	}
	if s.Path == "" {
		return fmt.Errorf("no usage store path configured; nothing was recorded")
	}
	release, err := s.lock()
	if err != nil {
		return err
	}
	defer release()

	t := s.Read()
	t.Runs++
	t.Usage = t.Usage.Add(u)
	encoded, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return s.writeAtomically(append(encoded, '\n'))
}

func (s Store) Reset() error {
	if err := os.Remove(s.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// The lock is a directory, created and removed by name, rather than an advisory
// lock from the operating system: the writers are separate `mf` processes on
// whatever platform the user runs, and no portable advisory lock exists in the
// standard library. A directory rather than a sentinel file because creating one
// is atomic on every platform and nothing keeps a handle to it — a lock file is
// the same idea, but on Windows an unlinked one lingers in a delete-pending
// state that makes the next creation fail as a permission error instead of as
// contention, which is exactly the distinction this loop must be able to draw.
const (
	// lockPoll is how often a waiting writer retries. Short, because the
	// critical section is one small read and one small write.
	lockPoll = 5 * time.Millisecond

	// lockWait bounds the wait so a jammed store degrades to a reported
	// accounting failure rather than a hung review. Accounting is observation:
	// the review it belongs to has already happened and must not be held up.
	lockWait = 10 * time.Second

	// lockStale is when a held lock is treated as abandoned. A process killed
	// mid-update leaves the directory behind, and a counter that stops counting
	// until someone deletes it by hand is a worse failure than the race it was
	// guarding against.
	lockStale = 2 * time.Minute

	// lockDenyGrace is how long a refusal to create the lock is read as
	// contention rather than as a real refusal. Windows leaves a just-removed
	// name in a delete-pending state where re-creating it is denied for
	// permission instead of reported as existing, and under several writers
	// that window is hit constantly. It clears in milliseconds, so a refusal
	// that outlasts this is a directory this process genuinely cannot write,
	// and it is returned rather than waited out for the full budget.
	lockDenyGrace = 2 * time.Second
)

func (s Store) lockPath() string { return s.Path + ".lock" }

// lock takes the store for this writer and returns the function that gives it
// back. The release only removes what this call created, so deferring it is
// safe on every path that follows.
func (s Store) lock() (func(), error) {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return nil, err
	}
	path := s.lockPath()
	deadline := time.Now().Add(lockWait)
	var deniedSince time.Time
	for {
		err := os.Mkdir(path, 0o755)
		switch {
		case err == nil:
			return func() { os.Remove(path) }, nil
		case os.IsExist(err):
			deniedSince = time.Time{}
			if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > lockStale {
				// Taking someone else's lock is only defensible because it is
				// this old: a writer still inside the critical section cannot
				// have held it for minutes, since the section is two file
				// operations.
				os.Remove(path)
			}
		case os.IsPermission(err):
			if deniedSince.IsZero() {
				deniedSince = time.Now()
			} else if time.Since(deniedSince) > lockDenyGrace {
				return nil, err
			}
		default:
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("another writer still holds %s after %s; this run's usage was not recorded",
				path, lockWait)
		}
		time.Sleep(lockPoll)
	}
}

// writeAtomically replaces the store in one step. `mf usage` and `mf doctor`
// read it without taking the lock, and Read answers a parse failure with an
// empty total, so a truncate-then-write would let a reader see the running
// total as zero and be given no reason to doubt it.
func (s Store) writeAtomically(data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), filepath.Base(s.Path)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, s.Path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
