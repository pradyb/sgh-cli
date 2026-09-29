// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

// Package update checks GitHub for a newer sgh-cli release without slowing commands
// down: the result is cached with a TTL, so the network is only contacted at most
// once per cacheTTL, and that one check is bounded by a short timeout.
package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pradyb/sgh-cli/utils"
)

// releaseURL is a var so tests can point it at a mock server.
var releaseURL = "https://api.github.com/repos/pradyb/sgh-cli/releases/latest"

const (
	cacheTTL    = 24 * time.Hour
	httpTimeout = 2 * time.Second
	cacheFile   = "update-check.json"
)

type cacheData struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

// Check reports the latest known release tag and whether it is newer than currentVersion.
// It never returns an error: any failure (network, parsing, cache I/O) just means no
// newer version is reported for this run. The network is contacted only when the cached
// result is missing or older than cacheTTL; a failed fetch still refreshes the cache
// timestamp, so an offline machine backs off for a day rather than retrying every command.
func Check(currentVersion string) (latest string, hasNewer bool) {
	dir, err := utils.ConfigDir()
	if err != nil {
		return "", false
	}
	return checkDir(currentVersion, dir)
}

func checkDir(currentVersion, dir string) (latest string, hasNewer bool) {
	if currentVersion == "" {
		return "", false
	}
	path := filepath.Join(dir, cacheFile)
	data := readCache(path)
	if time.Since(data.CheckedAt) > cacheTTL {
		if fetched, ferr := fetchLatestTag(); ferr == nil && fetched != "" {
			data = cacheData{CheckedAt: time.Now(), LatestVersion: fetched}
		} else {
			data.CheckedAt = time.Now() // back off 24h even on failure
		}
		_ = writeCache(path, data) // best-effort; a write failure just means we retry next time
	}
	if data.LatestVersion == "" {
		return "", false
	}
	return data.LatestVersion, isNewer(currentVersion, data.LatestVersion)
}

func readCache(path string) cacheData {
	b, err := os.ReadFile(path)
	if err != nil {
		return cacheData{}
	}
	var d cacheData
	if json.Unmarshal(b, &d) != nil {
		return cacheData{}
	}
	return d
}

func writeCache(path string, data cacheData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func fetchLatestTag() (string, error) {
	client := &http.Client{Timeout: httpTimeout}
	req, err := http.NewRequest(http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update check: unexpected status %d", resp.StatusCode)
	}
	var r struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&r); err != nil {
		return "", err
	}
	return r.TagName, nil
}

// UpgradeCommand returns the command that upgrades the running binary, chosen by where
// it was installed from, or "" when the install method is unknown (e.g. a downloaded
// release binary), in which case callers should point at the releases page instead.
func UpgradeCommand() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return upgradeCommandFor(exe, goBinDirs())
}

// upgradeCommandFor maps an already-resolved executable path to an upgrade command.
func upgradeCommandFor(exe string, goBins []string) string {
	p := filepath.ToSlash(exe)
	if strings.Contains(p, "/Cellar/") { // Homebrew and Linuxbrew both install under a Cellar
		return "brew update && brew upgrade sgh"
	}
	dir := filepath.ToSlash(filepath.Dir(exe))
	for _, b := range goBins {
		if b != "" && samePath(dir, filepath.ToSlash(filepath.Clean(b))) {
			return "go install github.com/pradyb/sgh-cli/cmd/sgh@latest"
		}
	}
	return ""
}

// caseInsensitiveFS is a var so tests can exercise both behaviours on any OS. Windows and
// macOS (default APFS) compare paths case-insensitively, so a differing case is still the
// same directory there.
var caseInsensitiveFS = runtime.GOOS == "windows" || runtime.GOOS == "darwin"

func samePath(a, b string) bool {
	if caseInsensitiveFS {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// goEnvSetting resolves a Go setting the way `go env` does, minus the subprocess: the
// process environment wins, then the file `go env -w` writes ($GOENV, else
// <UserConfigDir>/go/env). Reading a small file keeps this cheap and works on machines
// with no Go toolchain installed.
func goEnvSetting(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	path := os.Getenv("GOENV")
	if path == "off" {
		return ""
	}
	if path == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return ""
		}
		path = filepath.Join(dir, "go", "env")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	val := ""
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && k == key {
			val = v // last assignment wins
		}
	}
	return val
}

// goBinDirs lists where `go install` puts binaries: $GOBIN, else <each GOPATH entry>/bin.
func goBinDirs() []string {
	if b := goEnvSetting("GOBIN"); b != "" {
		return []string{b}
	}
	gopath := goEnvSetting("GOPATH")
	if gopath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		gopath = filepath.Join(home, "go")
	}
	var out []string
	for _, p := range filepath.SplitList(gopath) {
		out = append(out, filepath.Join(p, "bin"))
	}
	return out
}

// isNewer reports whether latest is a strictly greater SemVer than current. Either
// string failing to parse as vMAJOR.MINOR.PATCH is "not newer" — fail safe, never
// notify on a version format sgh doesn't recognise.
func isNewer(current, latest string) bool {
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	l, ok := parseVersion(latest)
	if !ok {
		return false
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parseVersion(v string) ([3]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	var out [3]int
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
