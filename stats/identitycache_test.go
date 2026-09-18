package stats

import (
	"sync"
	"testing"
	"time"
)

func TestIdentityCacheBuildsOnceThenServesFromMemory(t *testing.T) {
	cache := newIdentityCache(time.Hour)

	cards, updatedAt, stale, _ := cache.cards([]string{".."})
	if len(cards) != 1 || cards[0].Error != "" {
		t.Fatalf("got %d cards (%+v), want one readable card", len(cards), cards)
	}
	if updatedAt.IsZero() || stale {
		t.Errorf("a freshly built card should not be stale (updatedAt %s)", updatedAt)
	}

	again, secondUpdate, _, _ := cache.cards([]string{".."})
	if !secondUpdate.Equal(updatedAt) {
		t.Errorf("the second call should serve the cached card, rebuilt at %s", secondUpdate)
	}
	if again[0].History.Commits != cards[0].History.Commits {
		t.Error("the cached card should be the one that was built")
	}
}

func TestIdentityCacheRebuildsStaleCards(t *testing.T) {
	cache := newIdentityCache(time.Nanosecond)
	cache.cards([]string{".."})

	// The entry is older than the TTL: it is served, and rebuilt behind it.
	cards, _, stale, _ := cache.cards([]string{".."})
	if len(cards) != 1 || !stale {
		t.Errorf("an expired card should be served as stale, got %d cards (stale=%t)", len(cards), stale)
	}

	// A zero TTL disables the rebuild altogether.
	never := newIdentityCache(0)
	never.cards([]string{".."})
	if _, _, stale, _ := never.cards([]string{".."}); stale {
		t.Error("a zero TTL should keep cards fresh forever")
	}
}

func TestIdentityCacheBuildsAFolderOnlyOnceAtATime(t *testing.T) {
	cache := newIdentityCache(time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if cards, _, _, _ := cache.cards([]string{".."}); len(cards) != 1 {
				t.Errorf("concurrent call got %d cards, want 1", len(cards))
			}
		}()
	}
	wg.Wait()

	if cache.building([]string{".."}) {
		t.Error("no build should still be running once every caller returned")
	}
}

func TestIdentityCacheSkipsUnreadableFolders(t *testing.T) {
	cache := newIdentityCache(time.Hour)
	cards, _, _, _ := cache.cards([]string{t.TempDir()})
	if len(cards) != 1 || cards[0].Error == "" {
		t.Errorf("a folder that is not a repository should still yield a card carrying its error, got %+v", cards)
	}
}
