package stats

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Params holds the analysis parameters a client can tweak. The set of scanned
// folders is intentionally not exposed: it is fixed when the server starts.
type Params struct {
	Weeks    int      // 0 keeps the server default
	Delta    string   // "" means no offset
	User     string   // "" means no user filter
	CountAll bool     // analyze every user (ignores User)
	Merge    bool     // merge all folders into a single result
	Include  []string // file include patterns
	Exclude  []string // file exclude patterns
}

// cacheEntry is the persisted cache payload for a single parameter set: the
// aggregated statistics plus the moment they were computed.
type cacheEntry struct {
	Stats     AggregatedStats `json:"stats"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// statsCache keeps the last computed statistics per parameter set in memory,
// persists them to a JSON file, and refreshes them: synchronously the first
// time a parameter set is requested, in the background afterwards. At most one
// refresh runs at a time per parameter set.
type statsCache struct {
	baseOpts LaunchOptions // folders and startup defaults
	ttl      time.Duration
	file     string

	mu         sync.RWMutex
	entries    map[string]*cacheEntry
	refreshing map[string]bool
}

func newStatsCache(baseOpts LaunchOptions, ttl time.Duration, file string) *statsCache {
	return &statsCache{
		baseOpts:   baseOpts,
		ttl:        ttl,
		file:       file,
		entries:    make(map[string]*cacheEntry),
		refreshing: make(map[string]bool),
	}
}

// optsFor turns a Params into the LaunchOptions to scan with, keeping the
// server's fixed folders and applying the client overrides on top.
func (c *statsCache) optsFor(p Params) LaunchOptions {
	opts := c.baseOpts
	if p.Weeks > 0 {
		opts.DurationInWeeks = p.Weeks
	}
	opts.Delta = p.Delta
	opts.Merge = p.Merge
	opts.PatternToInclude = p.Include
	opts.PatternToExclude = p.Exclude

	switch {
	case p.CountAll || p.User == "":
		opts.User = nil
	default:
		user := p.User
		opts.User = &user
	}
	return opts
}

// cacheKey is a canonical, stable identifier for a set of launch options.
func cacheKey(opts LaunchOptions) string {
	user := "all"
	if opts.User != nil {
		user = *opts.User
	}
	return fmt.Sprintf(
		"w=%d|d=%s|u=%s|m=%t|inc=%s|exc=%s",
		opts.DurationInWeeks,
		opts.Delta,
		user,
		opts.Merge,
		strings.Join(opts.PatternToInclude, ","),
		strings.Join(opts.PatternToExclude, ","),
	)
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
	var entries map[string]*cacheEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		fmt.Printf("Ignoring invalid cache file %s: %s\n", c.file, err)
		return
	}
	c.mu.Lock()
	c.entries = entries
	c.mu.Unlock()
}

// persist writes the whole cache to the JSON file (best effort).
func (c *statsCache) persist() {
	if c.file == "" {
		return
	}
	c.mu.RLock()
	raw, err := json.MarshalIndent(c.entries, "", "  ")
	c.mu.RUnlock()
	if err != nil {
		return
	}
	if err := os.WriteFile(c.file, raw, 0644); err != nil {
		fmt.Printf("Cannot write cache file %s: %s\n", c.file, err)
	}
}

// scan runs a full analysis for the given options and stores the result under
// its key.
func (c *statsCache) scan(key string, opts LaunchOptions) {
	start := time.Now()
	log.Printf("Analyzing commits (%s)", describeOpts(opts))

	stats := Aggregate(Launch(opts))
	entry := &cacheEntry{Stats: stats, UpdatedAt: time.Now()}
	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()
	c.persist()

	log.Printf("Analysis done (%s): %d commits, %d contributors in %s",
		describeOpts(opts), stats.TotalCommits, len(stats.Contributors),
		time.Since(start).Round(time.Millisecond))
}

// describeOpts summarizes a set of launch options for logging.
func describeOpts(o LaunchOptions) string {
	user := "all users"
	if o.User != nil {
		user = *o.User
	}
	desc := fmt.Sprintf("user=%s, weeks=%d, folders=%d", user, o.DurationInWeeks, len(o.Folders))
	if o.Delta != "" {
		desc += ", delta=" + o.Delta
	}
	if o.Merge {
		desc += ", merged"
	}
	return desc
}

// refreshInBackground starts a scan in a goroutine unless one is already
// running for that key. It returns true when it actually starts a refresh.
func (c *statsCache) refreshInBackground(key string, opts LaunchOptions) bool {
	c.mu.Lock()
	if c.refreshing[key] {
		c.mu.Unlock()
		return false
	}
	c.refreshing[key] = true
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			delete(c.refreshing, key)
			c.mu.Unlock()
		}()
		c.scan(key, opts)
	}()
	return true
}

// state returns the cached entry for a key along with whether it is stale
// (older than the TTL) and whether a refresh is currently running.
func (c *statsCache) state(key string) (entry *cacheEntry, stale, refreshing bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	refreshing = c.refreshing[key]
	entry = c.entries[key]
	if entry == nil {
		return nil, true, refreshing
	}
	stale = c.ttl > 0 && time.Since(entry.UpdatedAt) > c.ttl
	return entry, stale, refreshing
}
