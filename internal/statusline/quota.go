package statusline

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// The cache is written in the shape the Node renderer already used, so a
// machine part-way through the migration reads one set of figures rather than
// two. It is deliberately not a new format.
const (
	CacheFileName       = ".usage-cache.json"
	CredentialsFileName = ".credentials.json"
	LockFileName        = ".usage-cache.json.lock"

	// RefreshInterval is how often the quota may be fetched; BackoffInterval is
	// how long a rate limit is respected for. The endpoint rate-limits per
	// token, so a refresh that retried on the normal interval would keep the
	// limit engaged.
	RefreshInterval = 5 * time.Minute
	BackoffInterval = 30 * time.Minute
	LockStale       = 30 * time.Second

	// DefaultEndpoint is the OAuth usage endpoint. It is overridable so a test
	// never reaches the network.
	DefaultEndpoint = "https://api.anthropic.com"

	requestBudget = 8 * time.Second
)

// Window is one usage window's utilization, and when it resets.
type Window struct {
	Util  float64 `json:"util"`
	Reset string  `json:"reset,omitempty"`
}

// Cache is the last reading taken. An absent cache is not zero utilization: it
// is no reading, and the two render differently on purpose.
type Cache struct {
	TS          int64   `json:"ts"`
	NextAllowed int64   `json:"nextAllowed"`
	FiveHour    *Window `json:"fiveHour"`
	SevenDay    *Window `json:"sevenDay"`
}

// Quota turns a cache into contract fact 4.
func (c Cache) Quota(now time.Time) Quota {
	if c.FiveHour == nil {
		return Quota{}
	}
	q := Quota{Known: true, FiveHour: c.FiveHour.Util}
	if c.SevenDay != nil {
		q.HasSevenDay, q.SevenDay = true, c.SevenDay.Util
	}
	if c.FiveHour.Reset != "" {
		if reset, err := time.Parse(time.RFC3339, c.FiveHour.Reset); err == nil {
			if remaining := reset.Sub(now); remaining > 0 {
				q.ResetIn = remaining
			}
		}
	}
	return q
}

// ReadCache loads the last reading. The second return value distinguishes "no
// cache" from "a cache full of zeros".
func ReadCache(home string) (Cache, bool) {
	if home == "" {
		return Cache{}, false
	}
	raw, err := os.ReadFile(filepath.Join(home, CacheFileName))
	if err != nil {
		return Cache{}, false
	}
	var c Cache
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cache{}, false
	}
	return c, true
}

// RefreshDue reports whether the cache is old enough to fetch again. Refreshing
// inline would block every render on the network, so this is the question the
// render pass asks before spawning one.
func RefreshDue(c Cache, now time.Time) bool {
	return now.UnixMilli() > c.NextAllowed
}

// RefreshOptions are the fetch's inputs. Endpoint and Now are injected so a
// test never reaches the real usage endpoint.
type RefreshOptions struct {
	Home     string
	Endpoint string
	Version  string
	Client   *http.Client
	Now      func() time.Time
}

func (o RefreshOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// Refresh fetches the plan usage and writes the cache.
//
// Every failure keeps the previous figures and backs off: stale quota beats no
// quota, because dropping the fact on a rate limit removes it exactly when the
// Developer most needs to see it. A session with no OAuth token has no plan
// windows to report at all — that is a declared degradation, not an error, and
// it invents no figures.
func Refresh(opts RefreshOptions) error {
	if opts.Home == "" {
		return fmt.Errorf("no configuration directory to cache the quota in")
	}
	previous, _ := ReadCache(opts.Home)
	now := opts.now()

	token, ok := oauthToken(opts.Home)
	if !ok {
		// Nothing to report, but the absence still has to be recorded. Writing
		// no cache leaves NextAllowed at zero, so every render finds a refresh
		// due and spawns a detached fetch — every 30 seconds, forever, for a
		// session that can never have a quota to fetch.
		return writeCache(opts.Home, keep(previous, now, RefreshInterval))
	}

	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	req, err := http.NewRequest(http.MethodGet, endpoint+"/api/oauth/usage", nil)
	if err != nil {
		return writeCache(opts.Home, keep(previous, now, RefreshInterval))
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if opts.Version != "" {
		req.Header.Set("User-Agent", "claude-code/"+opts.Version)
	}

	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: requestBudget}
	}
	resp, err := client.Do(req)
	if err != nil {
		return writeCache(opts.Home, keep(previous, now, RefreshInterval))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return writeCache(opts.Home, keep(previous, now, BackoffInterval))
	}
	if resp.StatusCode != http.StatusOK {
		return writeCache(opts.Home, keep(previous, now, RefreshInterval))
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return writeCache(opts.Home, keep(previous, now, RefreshInterval))
	}
	var parsed struct {
		FiveHour *struct {
			Utilization float64 `json:"utilization"`
			ResetsAt    string  `json:"resets_at"`
		} `json:"five_hour"`
		SevenDay *struct {
			Utilization float64 `json:"utilization"`
		} `json:"seven_day"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return writeCache(opts.Home, keep(previous, now, RefreshInterval))
	}

	fresh := Cache{TS: now.UnixMilli(), NextAllowed: now.Add(RefreshInterval).UnixMilli()}
	if parsed.FiveHour != nil {
		fresh.FiveHour = &Window{Util: parsed.FiveHour.Utilization, Reset: parsed.FiveHour.ResetsAt}
	}
	if parsed.SevenDay != nil {
		fresh.SevenDay = &Window{Util: parsed.SevenDay.Utilization}
	}
	return writeCache(opts.Home, fresh)
}

func keep(previous Cache, now time.Time, interval time.Duration) Cache {
	previous.TS = now.UnixMilli()
	previous.NextAllowed = now.Add(interval).UnixMilli()
	return previous
}

func writeCache(home string, c Cache) error {
	body, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, CacheFileName), body, 0o644)
}

func oauthToken(home string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(home, CredentialsFileName))
	if err != nil {
		return "", false
	}
	var creds struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(raw, &creds); err != nil {
		return "", false
	}
	return creds.ClaudeAiOauth.AccessToken, creds.ClaudeAiOauth.AccessToken != ""
}

// ClaimRefresh takes the lock that keeps concurrent renders from each spawning
// their own fetch.
//
// The claim is the exclusive creation of the lock file, not a check followed by
// a write: two renders redrawing on the same tick both pass a check, and the
// endpoint rate-limits per token, so the second fetch costs the figures a
// 30-minute backoff to recover from — the opposite of what the lock is for.
//
// A stale lock is taken over rather than respected forever: a process killed
// mid-refresh would otherwise freeze the quota fact. Only the render whose
// removal of the abandoned file succeeds goes on to contest the fresh claim.
func ClaimRefresh(home string, now time.Time) bool {
	if home == "" {
		return false
	}
	lock := filepath.Join(home, LockFileName)
	if createdExclusively(lock) {
		return true
	}
	info, err := os.Stat(lock)
	if err != nil || now.Sub(info.ModTime()) <= LockStale {
		return false
	}
	if err := os.Remove(lock); err != nil {
		return false
	}
	return createdExclusively(lock)
}

// ReleaseRefresh drops the claim, so the next render can take one.
//
// Nothing released it, so the file outlived every refresh and the documented
// "a stale lock is taken over rather than respected" path became the only path
// a claim could ever succeed through: 30 seconds of hard serialisation after
// each render, then an mtime bump forever.
//
// A lock that is not there is not an error: the refresh may have been started
// by hand rather than claimed.
func ReleaseRefresh(home string) {
	if home == "" {
		return
	}
	_ = os.Remove(filepath.Join(home, LockFileName))
}

func createdExclusively(lock string) bool {
	f, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return false
	}
	return f.Close() == nil
}
