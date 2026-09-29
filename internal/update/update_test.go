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

func TestUpgradeCommandFor(t *testing.T) {
	const goInstall = "go install github.com/pradyb/sgh-cli/cmd/sgh@latest"
	const brew = "brew update && brew upgrade sgh"
	goBins := []string{"/home/u/go/bin", "/opt/gopath/bin"}
	cases := []struct {
		name string
		exe  string
		want string
	}{
		{"homebrew apple silicon", "/opt/homebrew/Cellar/sgh/1.3.1/bin/sgh", brew},
		{"homebrew intel", "/usr/local/Cellar/sgh/1.3.1/bin/sgh", brew},
		{"linuxbrew", "/home/linuxbrew/.linuxbrew/Cellar/sgh/1.3.1/bin/sgh", brew},
		{"go install default", "/home/u/go/bin/sgh", goInstall},
		{"go install second GOPATH entry", "/opt/gopath/bin/sgh", goInstall},
		{"downloaded release binary", "/usr/local/bin/sgh", ""},
		{"home local bin", "/home/u/.local/bin/sgh", ""},
		{"go bin subdirectory is not go install", "/home/u/go/bin/tools/sgh", ""},
		{"empty path", "", ""},
	}
	for _, tc := range cases {
		if got := upgradeCommandFor(tc.exe, goBins); got != tc.want {
			t.Errorf("%s: upgradeCommandFor(%q) = %q, want %q", tc.name, tc.exe, got, tc.want)
		}
	}
}

func TestUpgradeCommandFor_NoGoBins(t *testing.T) {
	if got := upgradeCommandFor("/home/u/go/bin/sgh", nil); got != "" {
		t.Errorf("with no known Go bin dirs got %q, want empty", got)
	}
}

func TestGoBinDirs(t *testing.T) {
	t.Setenv("GOENV", "off") // don't read the developer's real go env file
	sep := string(os.PathListSeparator)

	t.Setenv("GOBIN", "/custom/bin")
	t.Setenv("GOPATH", "/ignored")
	if got := goBinDirs(); len(got) != 1 || got[0] != "/custom/bin" {
		t.Errorf("GOBIN should win, got %v", got)
	}

	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "/a"+sep+"/b")
	got := goBinDirs()
	if len(got) != 2 || got[0] != filepath.Join("/a", "bin") || got[1] != filepath.Join("/b", "bin") {
		t.Errorf("GOPATH entries should each get /bin, got %v", got)
	}

	t.Setenv("GOPATH", "")
	t.Setenv("HOME", "/home/x")
	t.Setenv("USERPROFILE", "/home/x")
	got = goBinDirs()
	if len(got) != 1 || got[0] != filepath.Join("/home/x", "go", "bin") {
		t.Errorf("default should be ~/go/bin, got %v", got)
	}
}

// Smoke test: the real wrapper resolves the test binary, which is neither a Homebrew nor
// a go-install location, so it must not invent an upgrade command.
func TestUpgradeCommand_TestBinaryIsUnknown(t *testing.T) {
	t.Setenv("GOENV", "off")
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", t.TempDir())
	if got := UpgradeCommand(); got != "" {
		t.Errorf("UpgradeCommand() = %q for a go-test binary, want empty", got)
	}
}

// The #56 scenario: GOBIN itself is a symlink (e.g. macOS's /tmp -> /private/tmp), so the
// candidate must be resolved the same way the executable path already is, or it never matches.
func TestResolvedGoBinDirs_ResolvesSymlinkedGOBIN(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "gobin-link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot create symlinks on this system: %v", err)
	}
	t.Setenv("GOENV", "off")
	t.Setenv("GOBIN", link)

	got := resolvedGoBinDirs()

	realResolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", real, err)
	}
	if len(got) != 1 || got[0] != realResolved {
		t.Fatalf("resolvedGoBinDirs() = %v, want [%q]", got, realResolved)
	}

	// And the comparison that was silently failing before the fix now succeeds — mirroring
	// UpgradeCommand(), which resolves the executable path the same way before comparing.
	exe := filepath.Join(realResolved, "sgh")
	if upgradeCommandFor(exe, got) == "" {
		t.Error("a binary under a symlinked GOBIN should still be recognised as go-installed")
	}
}

// A candidate that doesn't exist (EvalSymlinks fails) must be kept as-is rather than
// dropped or causing a panic.
func TestResolvedGoBinDirs_NonexistentCandidateKeptAsIs(t *testing.T) {
	t.Setenv("GOENV", "off")
	t.Setenv("GOBIN", "/does/not/exist/anywhere")

	got := resolvedGoBinDirs()

	if len(got) != 1 || got[0] != "/does/not/exist/anywhere" {
		t.Errorf("resolvedGoBinDirs() = %v, want the original candidate unchanged", got)
	}
}

func writeGoEnv(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", p)
	return p
}

func TestGoEnvSetting(t *testing.T) {
	writeGoEnv(t, "# comment\nGOBIN=/from/file\r\nGOPATH=/first\nGOPATH=/last\nMALFORMED\nGOFLAGS=-mod=mod\n")
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")

	if got := goEnvSetting("GOBIN"); got != "/from/file" {
		t.Errorf("GOBIN from file (CRLF line) = %q, want /from/file", got)
	}
	if got := goEnvSetting("GOPATH"); got != "/last" {
		t.Errorf("last assignment should win, got %q", got)
	}
	if got := goEnvSetting("GOFLAGS"); got != "-mod=mod" {
		t.Errorf("value containing '=' = %q, want -mod=mod", got)
	}
	if got := goEnvSetting("GOMISSING"); got != "" {
		t.Errorf("missing key = %q, want empty", got)
	}

	t.Setenv("GOBIN", "/from/process")
	if got := goEnvSetting("GOBIN"); got != "/from/process" {
		t.Errorf("process environment must beat the file, got %q", got)
	}
}

func TestGoEnvSetting_OffAndMissingFile(t *testing.T) {
	t.Setenv("GOBIN", "")
	t.Setenv("GOENV", "off")
	if got := goEnvSetting("GOBIN"); got != "" {
		t.Errorf("GOENV=off should ignore files, got %q", got)
	}
	t.Setenv("GOENV", filepath.Join(t.TempDir(), "does-not-exist"))
	if got := goEnvSetting("GOBIN"); got != "" {
		t.Errorf("missing file should yield empty, got %q", got)
	}
}

func TestGoEnvSetting_DefaultLocation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Setenv("GOENV", "")
	t.Setenv("GOBIN", "")
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Skip("no user config dir on this platform")
	}
	if err := os.MkdirAll(filepath.Join(dir, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go", "env"), []byte("GOBIN=/default/loc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := goEnvSetting("GOBIN"); got != "/default/loc" {
		t.Errorf("default env file location: got %q, want /default/loc", got)
	}
}

// The #53 scenario end to end: GOBIN only in the go env file, and the binary lives there.
func TestGoBinDirs_HonoursGoEnvW(t *testing.T) {
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	writeGoEnv(t, "GOBIN=/tmp/x\n")
	if got := upgradeCommandFor("/tmp/x/sgh", goBinDirs()); got == "" {
		t.Error("a binary in a GOBIN set via `go env -w` should be recognised as go-installed")
	}
}

func TestUpgradeCommandFor_CaseSensitivity(t *testing.T) {
	orig := caseInsensitiveFS
	t.Cleanup(func() { caseInsensitiveFS = orig })
	const goInstall = "go install github.com/pradyb/sgh-cli/cmd/sgh@latest"
	bins := []string{"C:/Users/U/go/bin"}

	caseInsensitiveFS = true
	if got := upgradeCommandFor("c:/users/u/GO/bin/sgh.exe", bins); got != goInstall {
		t.Errorf("case-insensitive FS: got %q, want the go install command", got)
	}
	caseInsensitiveFS = false
	if got := upgradeCommandFor("c:/users/u/GO/bin/sgh.exe", bins); got != "" {
		t.Errorf("case-sensitive FS must not match different case, got %q", got)
	}
}
