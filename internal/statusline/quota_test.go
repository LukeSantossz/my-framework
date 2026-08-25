package statusline

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func credentials(t *testing.T, dir, token string) {
	t.Helper()
	body := `{"claudeAiOauth":{"accessToken":` + jsonString(token) + `}}`
	if err := os.WriteFile(filepath.Join(dir, CredentialsFileName), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func readCache(t *testing.T, dir string) Cache {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, CacheFileName))
	if err != nil {
		t.Fatalf("no cache written: %v", err)
	}
	var c Cache
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("the cache is not readable: %v", err)
	}
	return c
}

func TestRefreshRecordsBothQuotaWindows(t *testing.T) {
	var seen *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		w.Write([]byte(`{"five_hour":{"utilization":42,"resets_at":"2026-08-24T13:30:00Z"},"seven_day":{"utilization":12}}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	credentials(t, dir, "sk-oauth-token")
	if err := Refresh(RefreshOptions{Home: dir, Endpoint: server.URL, Version: "2.1.161", Now: fixedNow}); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := seen.Header.Get("Authorization"); got != "Bearer sk-oauth-token" {
		t.Errorf("Authorization = %q", got)
	}
	cache := readCache(t, dir)
	if cache.FiveHour == nil || cache.FiveHour.Util != 42 {
		t.Errorf("five hour window = %+v", cache.FiveHour)
	}
	if cache.SevenDay == nil || cache.SevenDay.Util != 12 {
		t.Errorf("seven day window = %+v", cache.SevenDay)
	}
}

func TestRefreshWithoutAnOauthSessionRecordsWhenToLookAgain(t *testing.T) {
	// An API-key session has no plan windows to report. That is a declared
	// degradation, so it is not an error and it must invent no figures — but it
	// must still record when to look again: a refresh that writes nothing leaves
	// NextAllowed at zero, every render finds the refresh due, and the render
	// pass spawns a detached fetch every redraw for a session that can never
	// have a quota to fetch.
	dir := t.TempDir()
	if err := Refresh(RefreshOptions{Home: dir, Endpoint: "http://127.0.0.1:1", Now: fixedNow}); err != nil {
		t.Fatalf("a missing OAuth session must not be an error: %v", err)
	}
	cache := readCache(t, dir)
	if cache.FiveHour != nil || cache.SevenDay != nil {
		t.Errorf("figures were invented for a session with no quota to report: %+v", cache)
	}
	if want := fixedNow().Add(RefreshInterval).UnixMilli(); cache.NextAllowed != want {
		t.Errorf("NextAllowed = %d, want %d", cache.NextAllowed, want)
	}
	if RefreshDue(cache, fixedNow()) {
		t.Error("the next render would spawn another fetch for a session that can never have one to make")
	}
	if cache.Quota(fixedNow()).Known {
		t.Error("a recorded absence was rendered as a reading")
	}
}

func TestARateLimitedRefreshKeepsTheLastFiguresAndBacksOff(t *testing.T) {
	// Stale quota beats no quota: dropping the figures on a 429 would make the
	// fact disappear exactly when the Developer most needs to see it.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	dir := t.TempDir()
	credentials(t, dir, "token")
	previous := Cache{FiveHour: &Window{Util: 42}, SevenDay: &Window{Util: 12}}
	body, _ := json.Marshal(previous)
	os.WriteFile(filepath.Join(dir, CacheFileName), body, 0o644)

	if err := Refresh(RefreshOptions{Home: dir, Endpoint: server.URL, Now: fixedNow}); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	cache := readCache(t, dir)
	if cache.FiveHour == nil || cache.FiveHour.Util != 42 {
		t.Errorf("the previous figures were dropped: %+v", cache.FiveHour)
	}
	if want := fixedNow().Add(BackoffInterval).UnixMilli(); cache.NextAllowed != want {
		t.Errorf("NextAllowed = %d, want %d (a rate limit must back off further than a normal refresh)", cache.NextAllowed, want)
	}
}

func TestAFailedRefreshKeepsTheLastFiguresWithTheNormalInterval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dir := t.TempDir()
	credentials(t, dir, "token")
	body, _ := json.Marshal(Cache{FiveHour: &Window{Util: 7}})
	os.WriteFile(filepath.Join(dir, CacheFileName), body, 0o644)

	if err := Refresh(RefreshOptions{Home: dir, Endpoint: server.URL, Now: fixedNow}); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	cache := readCache(t, dir)
	if cache.FiveHour == nil || cache.FiveHour.Util != 7 {
		t.Errorf("the previous figures were dropped: %+v", cache.FiveHour)
	}
	if want := fixedNow().Add(RefreshInterval).UnixMilli(); cache.NextAllowed != want {
		t.Errorf("NextAllowed = %d, want %d", cache.NextAllowed, want)
	}
}

func TestExactlyOneOfManyConcurrentRendersClaimsTheRefresh(t *testing.T) {
	// A claim that stats before it writes lets every render redrawing on the
	// same tick pass the check, and the endpoint rate-limits per token: the
	// second fetch costs the figures a 30-minute backoff to recover from.
	dir := t.TempDir()
	now := time.Now()
	const renders = 16

	var start, done sync.WaitGroup
	var claims atomic.Int64
	start.Add(1)
	for range renders {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			if ClaimRefresh(dir, now) {
				claims.Add(1)
			}
		}()
	}
	start.Done()
	done.Wait()

	if got := claims.Load(); got != 1 {
		t.Errorf("%d of %d concurrent renders claimed the refresh, want exactly 1", got, renders)
	}
}

func TestAStaleClaimIsTakenOver(t *testing.T) {
	// A process killed mid-refresh would otherwise freeze the quota fact
	// forever, so a lock older than the refresh it was taken for is contested
	// rather than respected.
	dir := t.TempDir()
	now := time.Now()
	if !ClaimRefresh(dir, now) {
		t.Fatal("the first claim on an unlocked directory was refused")
	}
	if ClaimRefresh(dir, now) {
		t.Fatal("a fresh claim was taken twice")
	}
	abandoned := now.Add(-2 * LockStale)
	if err := os.Chtimes(filepath.Join(dir, LockFileName), abandoned, abandoned); err != nil {
		t.Fatal(err)
	}
	if !ClaimRefresh(dir, now) {
		t.Error("a lock left behind by a killed process was respected forever")
	}
}

func TestRefreshIsDueOnlyWhenTheCacheAllowsIt(t *testing.T) {
	now := fixedNow()
	if !RefreshDue(Cache{}, now) {
		t.Error("an absent cache must be refreshable")
	}
	if RefreshDue(Cache{NextAllowed: now.Add(time.Minute).UnixMilli()}, now) {
		t.Error("a cache that asked to be left alone was refreshed anyway")
	}
	if !RefreshDue(Cache{NextAllowed: now.Add(-time.Minute).UnixMilli()}, now) {
		t.Error("an expired cache was not refreshed")
	}
}
