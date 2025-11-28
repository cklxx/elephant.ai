package http

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type cachedPayload struct {
	contentType string
	data        []byte
	storedAt    time.Time
}

// DataCache stores small blobs decoded from data URIs and serves them via a URL.
type DataCache struct {
	mu         sync.Mutex
	maxEntries int
	ttl        time.Duration
	items      map[string]cachedPayload
}

func NewDataCache(maxEntries int, ttl time.Duration) *DataCache {
	if maxEntries <= 0 {
		maxEntries = 64
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &DataCache{
		maxEntries: maxEntries,
		ttl:        ttl,
		items:      make(map[string]cachedPayload),
	}
}

// MaybeStoreDataURI returns a lightweight descriptor when given a data URI; non-data URIs are returned as nil.
func (c *DataCache) MaybeStoreDataURI(value string) map[string]any {
	if !strings.HasPrefix(value, "data:") || !strings.Contains(value, ";base64,") {
		return nil
	}

	mediaType, payload, ok := decodeDataURI(value)
	if !ok || len(payload) == 0 {
		return nil
	}

	hash := sha256.Sum256(payload)
	id := fmt.Sprintf("%x", hash[:])
	c.store(id, mediaType, payload)

	return map[string]any{
		"url":          "/api/data/" + id,
		"content_type": mediaType,
		"size_bytes":   len(payload),
	}
}

func (c *DataCache) store(id string, mediaType string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict stale entries when exceeding capacity
	if len(c.items) >= c.maxEntries {
		c.evictLocked()
	}

	c.items[id] = cachedPayload{
		contentType: mediaType,
		data:        data,
		storedAt:    time.Now(),
	}
}

func (c *DataCache) evictLocked() {
	if len(c.items) == 0 {
		return
	}
	// Remove oldest entry
	var oldestKey string
	var oldestTime time.Time
	for key, entry := range c.items {
		if oldestKey == "" || entry.storedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.storedAt
		}
	}
	delete(c.items, oldestKey)
}

// Handler returns an http.Handler that serves cached payloads by id.
func (c *DataCache) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/data/")
		if id == "" {
			http.NotFound(w, r)
			return
		}

		entry, ok := c.lookup(id)
		if !ok {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", entry.contentType)
		_, _ = w.Write(entry.data)
	})
}

func (c *DataCache) lookup(id string) (cachedPayload, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[id]
	if !ok {
		return cachedPayload{}, false
	}
	if c.ttl > 0 && time.Since(entry.storedAt) > c.ttl {
		delete(c.items, id)
		return cachedPayload{}, false
	}
	return entry, true
}

func decodeDataURI(value string) (string, []byte, bool) {
	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 {
		return "", nil, false
	}
	header := parts[0]
	payload := parts[1]

	mediaType := strings.TrimPrefix(header, "data:")
	mediaType = strings.TrimSuffix(mediaType, ";base64")
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", nil, false
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	return mediaType, decoded, true
}
