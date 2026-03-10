// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package tui

import (
	"sort"
	"strings"
	"time"
)

type cacheEntry struct {
	data      any
	fetchedAt time.Time
	repoKey   string
}

type dataCache struct {
	entries map[string]cacheEntry
	ttl     time.Duration
}

func newDataCache(ttl time.Duration) *dataCache {
	return &dataCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
}

func makeRepoKey(repos []string) string {
	sorted := make([]string, len(repos))
	copy(sorted, repos)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// get looks up an entry by cacheKey (which embeds the filter state, e.g. "pr:open")
// and repos. Returns (data, hit, stale).
func (c *dataCache) get(cacheKey string, repos []string) (any, bool, bool) {
	entry, ok := c.entries[cacheKey]
	if !ok {
		return nil, false, false
	}
	rk := makeRepoKey(repos)
	if entry.repoKey != rk {
		return nil, false, false
	}
	stale := time.Since(entry.fetchedAt) > c.ttl
	return entry.data, true, stale
}

func (c *dataCache) set(cacheKey string, repos []string, data any) {
	c.entries[cacheKey] = cacheEntry{
		data:      data,
		fetchedAt: time.Now(),
		repoKey:   makeRepoKey(repos),
	}
}

// invalidate removes all cache entries whose key matches the command or starts with "command:".
func (c *dataCache) invalidate(command string) {
	for k := range c.entries {
		if k == command || strings.HasPrefix(k, command+":") {
			delete(c.entries, k)
		}
	}
}

func (c *dataCache) age(cacheKey string) time.Duration {
	entry, ok := c.entries[cacheKey]
	if !ok {
		return 0
	}
	return time.Since(entry.fetchedAt)
}
