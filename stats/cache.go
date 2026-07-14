package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// cacheEntry is the persisted cache payload: the aggregated statistics plus the
// moment they were computed.
type cacheEntry struct {
	Stats     AggregatedStats `json:"stats"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// statsCache keeps the last computed statistics in memory, persists them to a
// JSON file, and refreshes them: synchronously on demand at startup, and in the
// background afterwards. A single refresh runs at a time.
type statsCache struct {
	opts LaunchOptions
	ttl  time.Duration
	file string

	mu         sync.RWMutex
	entry      *cacheEntry
	refreshing bool
}

func newStatsCache(opts LaunchOptions, ttl time.Duration, file string) *statsCache {
	return &statsCache{opts: opts, ttl: ttl, file: file}
}

// load restores a previously persisted cache file, if any. A missing or
// unreadable file is not an error: the cache simply starts empty.
func (c *statsCache) load() {
	if c.file == "" {
		return
	}
	raw, err := os.ReadFile(c.file)
	if err != nil {
		return
	}
	var entry cacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		fmt.Printf("Ignoring invalid cache file %s: %s\n", c.file, err)
		return
	}
	c.mu.Lock()
	c.entry = &entry
	c.mu.Unlock()
}

// persist writes the current cache entry to the JSON file (best effort).
func (c *statsCache) persist() {
	if c.file == "" {
		return
	}
	c.mu.RLock()
	entry := c.entry
	c.mu.RUnlock()
	if entry == nil {
		return
	}
	raw, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(c.file, raw, 0644); err != nil {
		fmt.Printf("Cannot write cache file %s: %s\n", c.file, err)
	}
}

// scan runs a full analysis, stores the result in memory and persists it.
func (c *statsCache) scan() {
	entry := &cacheEntry{
		Stats:     Aggregate(Launch(c.opts)),
		UpdatedAt: time.Now(),
	}
	c.mu.Lock()
	c.entry = entry
	c.mu.Unlock()
	c.persist()
}

// refreshInBackground starts a scan in a goroutine unless one is already
// running. It returns true when it actually starts a new refresh.
func (c *statsCache) refreshInBackground() bool {
	c.mu.Lock()
	if c.refreshing {
		c.mu.Unlock()
		return false
	}
	c.refreshing = true
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			c.refreshing = false
			c.mu.Unlock()
		}()
		c.scan()
	}()
	return true
}

// state returns the current entry along with whether it is stale (older than
// the TTL) and whether a refresh is currently running.
func (c *statsCache) state() (entry *cacheEntry, stale, refreshing bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.entry == nil {
		return nil, true, c.refreshing
	}
	stale = c.ttl > 0 && time.Since(c.entry.UpdatedAt) > c.ttl
	return c.entry, stale, c.refreshing
}
