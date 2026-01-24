package builtin

import (
	"container/list"
	"testing"
	"time"
)

func TestFetchCacheEvictsLeastRecentlyUsed(t *testing.T) {
	cache := &fetchCache{
		entries:    make(map[string]*cacheEntry),
		order:      list.New(),
		ttl:        time.Hour,
		maxEntries: 2,
	}

	cache.put("a", &cacheEntry{content: "a", timestamp: time.Now(), url: "a"})
	cache.put("b", &cacheEntry{content: "b", timestamp: time.Now(), url: "b"})

	if cache.get("a") == nil {
		t.Fatalf("expected cache hit for a")
	}

	cache.put("c", &cacheEntry{content: "c", timestamp: time.Now(), url: "c"})

	if cache.get("b") != nil {
		t.Fatalf("expected b to be evicted")
	}
	if cache.get("a") == nil {
		t.Fatalf("expected a to remain")
	}
}
