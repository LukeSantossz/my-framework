package statusline

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestRefreshWithoutAnOauthSessionWritesNothing(t *testing.T) {
	// An API-key session has no plan windows to report. That is a declared
	// degradation, so it is not an error and it must not invent a cache.
	dir := t.TempDir()
	if err := Refresh(RefreshOptions{Home: dir, Endpoint: "http://127.0.0.1:1", Now: fixedNow}); err != nil {
		t.Fatalf("a missing OAuth session must not be an error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, CacheFileName)); !os.IsNotExist(err) {
		t.Error("a cache was written for a session with no quota to report")
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
