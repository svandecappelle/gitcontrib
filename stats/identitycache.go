package stats

import (
	"sync"
	"time"
)

// identityEntry is one cached identity card with the moment it was computed.
type identityEntry struct {
	card      RepositoryIdentity
	updatedAt time.Time
}

// identityCache keeps the identity card of each repository in memory. Cards
// describe the repository itself — its whole history and the current version
// of its code — so they do not depend on the analysis parameters and are
// cached per folder rather than per parameter set. They are rebuilt on demand
// once older than the TTL, serving the previous card meanwhile. Unlike the
// statistics cache they are not persisted: rebuilding them only costs a walk
// of the repository.
type identityCache struct {
	mu       sync.Mutex
	ttl      time.Duration
	entries  map[string]*identityEntry
	inflight map[string]chan struct{}
}

// setTTL changes how long a card stays fresh, following the value a config
// saved from the web UI carries.
func (c *identityCache) setTTL(ttl time.Duration) {
	c.mu.Lock()
	c.ttl = ttl
	c.mu.Unlock()
}

func newIdentityCache(ttl time.Duration) *identityCache {
	return &identityCache{
		ttl:      ttl,
		entries:  make(map[string]*identityEntry),
		inflight: make(map[string]chan struct{}),
	}
}

// cards returns the identity card of every folder, computing the ones missing
// from the cache (in parallel) and refreshing the stale ones in the
// background. It also reports the oldest card's age and whether any card is
// stale or currently being rebuilt.
func (c *identityCache) cards(folders []string) (cards []RepositoryIdentity, oldest time.Time, stale, refreshing bool) {
	var wg sync.WaitGroup
	for _, folder := range folders {
		entry, entryStale := c.state(folder)
		switch {
		case entry == nil:
			// Never computed: the client waits for it.
			wg.Add(1)
			go func(folder string) {
				defer wg.Done()
				c.build(folder)
			}(folder)
		case entryStale:
			// Stale-while-revalidate, like the statistics cache.
			stale = true
			c.refreshInBackground(folder)
		}
	}
	wg.Wait()

	cards = make([]RepositoryIdentity, 0, len(folders))
	for _, folder := range folders {
		entry, _ := c.state(folder)
		if entry == nil {
			continue
		}
		cards = append(cards, entry.card)
		if oldest.IsZero() || entry.updatedAt.Before(oldest) {
			oldest = entry.updatedAt
		}
	}
	return cards, oldest, stale, c.building(folders)
}

// state returns the cached card of a folder and whether it is stale.
func (c *identityCache) state(folder string) (*identityEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[folder]
	if entry == nil {
		return nil, true
	}
	return entry, c.ttl > 0 && time.Since(entry.updatedAt) > c.ttl
}

// building reports whether a card of one of the folders is being computed.
func (c *identityCache) building(folders []string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, folder := range folders {
		if _, ok := c.inflight[folder]; ok {
			return true
		}
	}
	return false
}

// build computes and stores the card of a folder. A build already running for
// that folder is joined rather than duplicated.
func (c *identityCache) build(folder string) {
	c.mu.Lock()
	if running, ok := c.inflight[folder]; ok {
		c.mu.Unlock()
		<-running
		return
	}
	done := make(chan struct{})
	c.inflight[folder] = done
	c.mu.Unlock()

	card := BuildIdentity(folder)

	c.mu.Lock()
	c.entries[folder] = &identityEntry{card: card, updatedAt: time.Now()}
	delete(c.inflight, folder)
	c.mu.Unlock()
	close(done)
}

// refreshInBackground rebuilds a card in a goroutine unless a build is already
// running for that folder.
func (c *identityCache) refreshInBackground(folder string) {
	c.mu.Lock()
	_, running := c.inflight[folder]
	c.mu.Unlock()
	if running {
		return
	}
	go c.build(folder)
}

// forceRefresh rebuilds every folder's card in the background, whatever their
// age — what the UI's refresh button asks for.
func (c *identityCache) forceRefresh(folders []string) {
	for _, folder := range folders {
		c.refreshInBackground(folder)
	}
}

// warm builds the cards one folder at a time in the background, so the first
// page load finds them ready without a burst of scans at startup. A folder a
// request already had built is left alone.
func (c *identityCache) warm(folders []string) {
	go func() {
		for _, folder := range folders {
			if entry, stale := c.state(folder); entry != nil && !stale {
				continue
			}
			c.build(folder)
		}
	}()
}
