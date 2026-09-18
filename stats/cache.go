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

// Params holds the analysis parameters a client can tweak. New folders cannot
// be introduced: Repos may only select among the folders fixed at startup, and
// Group names a saved selection of those folders.
type Params struct {
	Weeks    int      // 0 keeps the server default
	Delta    string   // "" means no offset
	User     string   // "" means no user filter
	CountAll bool     // analyze every user (ignores User)
	Merge    bool     // merge all folders into a single result
	Repos    []string // empty means every repository
	Group    string   // a saved group of repositories, used when Repos is empty
	Include  []string // file include patterns
	Exclude  []string // file exclude patterns
}

// cacheEntry is the persisted cache payload for a single parameter set: the
// aggregated statistics plus the moment they were computed.
type cacheEntry struct {
	Stats     AggregatedStats `json:"stats"`
	UpdatedAt time.Time       `json:"updatedAt"`
	// Folders is what the entry was computed over, so an entry can be dropped
	// when one of its repositories changes under it (a pull, for instance).
	Folders []string `json:"folders,omitempty"`
}

// statsCache keeps the last computed statistics per parameter set in memory,
// persists them to a JSON file, and refreshes them: synchronously the first
// time a parameter set is requested, in the background afterwards. At most one
// refresh runs at a time per parameter set.
type statsCache struct {
	file   string
	groups *groupStore // saved repository selections, for the "group" parameter

	// startupOpts and startupTTL are how the server was started — command line
	// and config file. A config saved from the web UI is applied on top of
	// them, so clearing a value there brings the startup value back rather than
	// keeping whatever was running.
	startupOpts LaunchOptions
	startupTTL  time.Duration

	// optsMu guards the options the server currently analyzes with: they are
	// fixed at startup, but a config saved from the web UI replaces them.
	optsMu   sync.RWMutex
	baseOpts LaunchOptions
	ttl      time.Duration

	mu         sync.RWMutex
	entries    map[string]*cacheEntry
	refreshing map[string]bool
}

// options returns the options the server currently analyzes with.
func (c *statsCache) options() LaunchOptions {
	c.optsMu.RLock()
	defer c.optsMu.RUnlock()
	return c.baseOpts
}

// folders returns the repositories the server currently scans.
func (c *statsCache) folders() []string {
	return c.options().Folders
}

// setOptions replaces the options every later request starts from. Cached
// results are keyed by their own options, so they stay valid: the entries that
// no longer match any request simply stop being served.
func (c *statsCache) setOptions(opts LaunchOptions, ttl time.Duration) {
	c.optsMu.Lock()
	c.baseOpts = opts
	c.ttl = ttl
	c.optsMu.Unlock()
}

// lifetime is how long an entry stays fresh.
func (c *statsCache) lifetime() time.Duration {
	c.optsMu.RLock()
	defer c.optsMu.RUnlock()
	return c.ttl
}

func newStatsCache(baseOpts LaunchOptions, ttl time.Duration, file string, groups *groupStore) *statsCache {
	if groups == nil {
		groups = newGroupStore("")
	}
	return &statsCache{
		baseOpts:    baseOpts,
		ttl:         ttl,
		startupOpts: baseOpts,
		startupTTL:  ttl,
		file:        file,
		groups:      groups,
		entries:     make(map[string]*cacheEntry),
		refreshing:  make(map[string]bool),
	}
}

// folderSelection resolves the repositories a request applies to: the ones it
// names, the ones of the group it names, or — when it names neither — every
// configured folder. Selecting an unknown folder is an error; a group that has
// outlived some of its repositories keeps the ones still scanned.
func (c *statsCache) folderSelection(p Params) ([]string, error) {
	scanned := c.folders()
	if len(p.Repos) > 0 {
		for _, repo := range p.Repos {
			if !containsFolder(scanned, repo) {
				return nil, fmt.Errorf("unknown repository: %s", repo)
			}
		}
		return intersectFolders(p.Repos, scanned), nil
	}

	if p.Group != "" {
		group, ok := c.groups.find(p.Group)
		if !ok {
			return nil, fmt.Errorf("unknown group: %s", p.Group)
		}
		folders := intersectFolders(group.Repositories, scanned)
		if len(folders) == 0 {
			return nil, fmt.Errorf("group %s holds no scanned repository", group.Name)
		}
		return folders, nil
	}

	return scanned, nil
}

// optsFor turns a Params into the LaunchOptions to scan with, restricted to
// the folders the request selected and with the client overrides applied on
// top of the server's defaults.
func (c *statsCache) optsFor(p Params, folders []string) LaunchOptions {
	opts := c.options()
	if p.Weeks > 0 {
		opts.DurationInWeeks = p.Weeks
	}
	opts.Delta = p.Delta
	opts.Merge = p.Merge
	opts.PatternToInclude = p.Include
	opts.PatternToExclude = p.Exclude
	opts.Folders = folders

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
		"f=%s|w=%d|d=%s|u=%s|m=%t|inc=%s|exc=%s",
		strings.Join(opts.Folders, ","),
		opts.DurationInWeeks,
		opts.Delta,
		user,
		opts.Merge,
		strings.Join(opts.PatternToInclude, ","),
		strings.Join(opts.PatternToExclude, ","),
	)
}

// containsFolder reports whether folder is one of the folders.
func containsFolder(folders []string, folder string) bool {
	for _, f := range folders {
		if f == folder {
			return true
		}
	}
	return false
}

// sameFolders reports whether two folder lists are identical (order included).
func sameFolders(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
	entry := &cacheEntry{Stats: stats, UpdatedAt: time.Now(), Folders: opts.Folders}
	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()
	c.persist()

	log.Printf("Analysis done (%s): %d commits, %d contributors in %s",
		describeOpts(opts), stats.TotalCommits, len(stats.Contributors),
		time.Since(start).Round(time.Millisecond))
}

// invalidateFolders drops every cached result covering one of the folders,
// because their history just changed. An entry from an older cache file, which
// does not say what it covers, is dropped too rather than trusted.
func (c *statsCache) invalidateFolders(folders []string) {
	if len(folders) == 0 {
		return
	}
	changed := map[string]bool{}
	for _, folder := range folders {
		changed[folder] = true
	}

	c.mu.Lock()
	for key, entry := range c.entries {
		drop := len(entry.Folders) == 0
		for _, folder := range entry.Folders {
			if changed[folder] {
				drop = true
				break
			}
		}
		if drop {
			delete(c.entries, key)
		}
	}
	c.mu.Unlock()
	c.persist()
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
	ttl := c.lifetime()
	stale = ttl > 0 && time.Since(entry.UpdatedAt) > ttl
	return entry, stale, refreshing
}
