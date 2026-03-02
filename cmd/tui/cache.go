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

func (c *dataCache) get(command string, repos []string) (any, bool, bool) {
	entry, ok := c.entries[command]
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

func (c *dataCache) set(command string, repos []string, data any) {
	c.entries[command] = cacheEntry{
		data:      data,
		fetchedAt: time.Now(),
		repoKey:   makeRepoKey(repos),
	}
}

func (c *dataCache) invalidate(command string) {
	delete(c.entries, command)
}

func (c *dataCache) age(command string) time.Duration {
	entry, ok := c.entries[command]
	if !ok {
		return 0
	}
	return time.Since(entry.fetchedAt)
}
