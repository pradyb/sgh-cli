// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withMockServer(t *testing.T, tagName string, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if status == http.StatusOK {
			_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": tagName})
		}
	}))
	t.Cleanup(srv.Close)
	original := releaseURL
	releaseURL = srv.URL
	t.Cleanup(func() { releaseURL = original })
	return srv.URL
}

func writeCacheFile(t *testing.T, dir string, data cacheData) {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, cacheFile), b, 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}
}

func readCacheFile(t *testing.T, dir string) cacheData {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, cacheFile))
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	var d cacheData
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("unmarshal cache: %v", err)
	}
	return d
}

func TestCheckDir_EmptyCurrentVersionSkips(t *testing.T) {
	dir := t.TempDir()
	withMockServer(t, "v9.9.9", http.StatusOK)

	latest, hasNewer := checkDir("", dir)

	if latest != "" || hasNewer {
		t.Errorf("checkDir(\"\") = (%q, %v), want (\"\", false)", latest, hasNewer)
	}
	if _, err := os.Stat(filepath.Join(dir, cacheFile)); err == nil {
		t.Error("expected no cache file written for an empty version")
	}
}

func TestCheckDir_NoPriorCacheFetchesAndReportsNewer(t *testing.T) {
	dir := t.TempDir()
	withMockServer(t, "v9.9.9", http.StatusOK)

	latest, hasNewer := checkDir("v1.0.0", dir)

	if latest != "v9.9.9" || !hasNewer {
		t.Errorf("checkDir() = (%q, %v), want (\"v9.9.9\", true)", latest, hasNewer)
	}
	cached := readCacheFile(t, dir)
	if cached.LatestVersion != "v9.9.9" || time.Since(cached.CheckedAt) > time.Minute {
		t.Errorf("cache not written correctly: %+v", cached)
	}
}

func TestCheckDir_CurrentIsLatest(t *testing.T) {
	dir := t.TempDir()
	withMockServer(t, "v1.2.0", http.StatusOK)

	latest, hasNewer := checkDir("v1.2.0", dir)

	if latest != "v1.2.0" || hasNewer {
		t.Errorf("checkDir() = (%q, %v), want (\"v1.2.0\", false)", latest, hasNewer)
	}
}

func TestCheckDir_CurrentIsNewerThanLatest(t *testing.T) {
	dir := t.TempDir()
	withMockServer(t, "v1.0.0", http.StatusOK)

	_, hasNewer := checkDir("v2.0.0", dir)

	if hasNewer {
		t.Error("current ahead of 'latest' must not report a newer version")
	}
}

func TestCheckDir_FreshCacheSkipsNetwork(t *testing.T) {
	dir := t.TempDir()
	writeCacheFile(t, dir, cacheData{CheckedAt: time.Now(), LatestVersion: "v5.0.0"})

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	releaseURL = srv.URL
	defer func() { releaseURL = "https://api.github.com/repos/pradyb/sgh-cli/releases/latest" }()

	latest, hasNewer := checkDir("v1.0.0", dir)

	if hits != 0 {
		t.Errorf("expected no network requests for a fresh cache, got %d", hits)
	}
	if latest != "v5.0.0" || !hasNewer {
		t.Errorf("checkDir() = (%q, %v), want (\"v5.0.0\", true) from cache", latest, hasNewer)
	}
}

func TestCheckDir_StaleCacheRefetches(t *testing.T) {
	dir := t.TempDir()
	writeCacheFile(t, dir, cacheData{CheckedAt: time.Now().Add(-25 * time.Hour), LatestVersion: "v1.0.0"})
	withMockServer(t, "v2.0.0", http.StatusOK)

	latest, hasNewer := checkDir("v1.0.0", dir)

	if latest != "v2.0.0" || !hasNewer {
		t.Errorf("checkDir() = (%q, %v), want (\"v2.0.0\", true) after refetch", latest, hasNewer)
	}
	cached := readCacheFile(t, dir)
	if cached.LatestVersion != "v2.0.0" {
		t.Errorf("cache not refreshed: %+v", cached)
	}
}

func TestCheckDir_NetworkErrorFallsBackToStaleCacheAndBacksOff(t *testing.T) {
	dir := t.TempDir()
	staleTime := time.Now().Add(-25 * time.Hour)
	writeCacheFile(t, dir, cacheData{CheckedAt: staleTime, LatestVersion: "v3.0.0"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	releaseURL = srv.URL
	srv.Close() // closed: connections now fail immediately, simulating being offline

	latest, hasNewer := checkDir("v1.0.0", dir)

	if latest != "v3.0.0" || !hasNewer {
		t.Errorf("checkDir() = (%q, %v), want the stale cached (\"v3.0.0\", true) on fetch failure", latest, hasNewer)
	}
	cached := readCacheFile(t, dir)
	if !cached.CheckedAt.After(staleTime) {
		t.Error("expected CheckedAt to be bumped (back off) even though the fetch failed")
	}
	if cached.LatestVersion != "v3.0.0" {
		t.Errorf("expected the prior LatestVersion to be preserved on fetch failure, got %q", cached.LatestVersion)
	}
}

func TestCheckDir_NetworkErrorNoPriorCache(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	releaseURL = srv.URL
	srv.Close()

	latest, hasNewer := checkDir("v1.0.0", dir)

	if latest != "" || hasNewer {
		t.Errorf("checkDir() = (%q, %v), want (\"\", false) with no cache and a failed fetch", latest, hasNewer)
	}
	// Still writes the cache (with an updated CheckedAt) so a second call within the TTL
	// does not hit the network again while genuinely offline.
	cached := readCacheFile(t, dir)
	if time.Since(cached.CheckedAt) > time.Minute {
		t.Errorf("expected CheckedAt to be refreshed even on failure: %+v", cached)
	}
}

func TestFetchLatestTag_NonOKStatus(t *testing.T) {
	withMockServer(t, "", http.StatusForbidden)

	if _, err := fetchLatestTag(); err == nil {
		t.Error("expected an error for a non-200 response")
	}
}

func TestFetchLatestTag_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	original := releaseURL
	releaseURL = srv.URL
	defer func() { releaseURL = original }()

	if _, err := fetchLatestTag(); err == nil {
		t.Error("expected an error for a malformed response body")
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"v1.0.0", "v1.1.0", true},
		{"v1.0.0", "v2.0.0", true},
		{"v1.2.0", "v1.2.0", false},
		{"v1.2.0", "v1.1.9", false},
		{"v2.0.0", "v1.9.9", false},
		{"1.0.0", "1.0.1", true}, // no leading 'v'
		{"garbage", "v1.0.0", false},
		{"v1.0.0", "garbage", false},
		{"v1.0", "v1.0.1", false}, // current has only 2 parts
		{"", "v1.0.0", false},
	}
	for _, tc := range cases {
		if got := isNewer(tc.current, tc.latest); got != tc.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

// Smoke test for the public Check() wrapper (checkDir has the real coverage): an empty
// version returns early before any file or network I/O, so this is safe to run against
// the real config directory.
func TestCheck_EmptyVersionIsSafeWithRealConfigDir(t *testing.T) {
	if latest, hasNewer := Check(""); latest != "" || hasNewer {
		t.Errorf("Check(\"\") = (%q, %v), want (\"\", false)", latest, hasNewer)
	}
}
