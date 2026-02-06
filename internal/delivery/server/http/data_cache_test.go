package http

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestDataCacheLRUEviction(t *testing.T) {
	cache := NewDataCache(2, time.Hour)

	first := cache.StoreBytes("text/plain", []byte("first"))
	second := cache.StoreBytes("text/plain", []byte("second"))

	firstID := strings.TrimPrefix(first, "/api/data/")
	secondID := strings.TrimPrefix(second, "/api/data/")
	if firstID == "" || secondID == "" {
		t.Fatalf("expected cache urls, got %q and %q", first, second)
	}

	if _, ok := cache.lookup(firstID); !ok {
		t.Fatalf("expected first entry to exist")
	}

	_ = cache.StoreBytes("text/plain", []byte("third"))

	if _, ok := cache.lookup(secondID); ok {
		t.Fatalf("expected second entry to be evicted")
	}
	if _, ok := cache.lookup(firstID); !ok {
		t.Fatalf("expected first entry to remain")
	}
}

func TestDataCacheDataURICache(t *testing.T) {
	cache := NewDataCache(2, time.Hour)

	payload := base64.StdEncoding.EncodeToString([]byte("hello"))
	dataURI := "data:text/plain;base64," + payload

	first := cache.MaybeStoreDataURI(dataURI)
	if first == nil {
		t.Fatalf("expected descriptor from data uri")
	}
	if len(cache.items) != 1 {
		t.Fatalf("expected 1 cached item, got %d", len(cache.items))
	}
	if len(cache.dataURICache) != 1 {
		t.Fatalf("expected 1 data uri cache entry, got %d", len(cache.dataURICache))
	}

	second := cache.MaybeStoreDataURI(dataURI)
	if second == nil {
		t.Fatalf("expected cached descriptor from data uri")
	}
	if len(cache.items) != 1 {
		t.Fatalf("expected 1 cached item after reuse, got %d", len(cache.items))
	}
	if len(cache.dataURICache) != 1 {
		t.Fatalf("expected 1 data uri cache entry after reuse, got %d", len(cache.dataURICache))
	}
}
