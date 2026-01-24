package http

import (
	"container/list"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
)

type cachedPayload struct {
	contentType string
	data        []byte
	storedAt    time.Time
}

type cacheEntry struct {
	payload cachedPayload
	element *list.Element
}

type cachedDataURI struct {
	descriptor map[string]any
	storedAt   time.Time
	element    *list.Element
}

// DataCache stores small blobs decoded from data URIs and serves them via a URL.
type DataCache struct {
	mu                sync.Mutex
	maxEntries        int
	ttl               time.Duration
	items             map[string]*cacheEntry
	order             *list.List
	dataURICache      map[string]*cachedDataURI
	dataURIOrder      *list.List
	dataURIMaxEntries int
}

func NewDataCache(maxEntries int, ttl time.Duration) *DataCache {
	if maxEntries <= 0 {
		maxEntries = 64
	}
	dataURIMax := maxEntries * 2
	if dataURIMax < 64 {
		dataURIMax = 64
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &DataCache{
		maxEntries:        maxEntries,
		ttl:               ttl,
		items:             make(map[string]*cacheEntry),
		order:             list.New(),
		dataURICache:      make(map[string]*cachedDataURI),
		dataURIOrder:      list.New(),
		dataURIMaxEntries: dataURIMax,
	}
}

// StoreBytes caches an arbitrary payload and returns a stable URL for retrieval.
func (c *DataCache) StoreBytes(mediaType string, data []byte) string {
	if len(data) == 0 || c == nil {
		return ""
	}
	if strings.TrimSpace(mediaType) == "" {
		mediaType = "application/octet-stream"
	}

	hash := sha256.Sum256(data)
	id := fmt.Sprintf("%x", hash[:])
	c.storeWithID(id, mediaType, data)
	return "/api/data/" + id
}

// MaybeStoreDataURI returns a lightweight descriptor when given a data URI; non-data URIs are returned as nil.
func (c *DataCache) MaybeStoreDataURI(value string) map[string]any {
	if c == nil {
		return nil
	}
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "data:") || !strings.Contains(value, ";base64,") {
		return nil
	}

	cacheKey := dataURIKey(value)
	if cached := c.getDataURI(cacheKey); cached != nil {
		return cached
	}

	mediaType, payload, ok := decodeDataURI(value)
	if !ok || len(payload) == 0 {
		return nil
	}

	hash := sha256.Sum256(payload)
	id := fmt.Sprintf("%x", hash[:])
	descriptor := map[string]any{
		"url":          "/api/data/" + id,
		"content_type": mediaType,
		"size_bytes":   len(payload),
	}
	c.storeWithID(id, mediaType, payload)
	c.storeDataURI(cacheKey, descriptor)
	return descriptor
}

func (c *DataCache) storeWithID(id string, mediaType string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.items[id]; ok {
		entry.payload = cachedPayload{
			contentType: mediaType,
			data:        data,
			storedAt:    time.Now(),
		}
		c.order.MoveToFront(entry.element)
		return
	}

	element := c.order.PushFront(id)
	c.items[id] = &cacheEntry{
		payload: cachedPayload{
			contentType: mediaType,
			data:        data,
			storedAt:    time.Now(),
		},
		element: element,
	}
	for len(c.items) > c.maxEntries {
		c.evictLocked()
	}
}

func (c *DataCache) evictLocked() {
	if len(c.items) == 0 || c.order == nil {
		return
	}
	oldest := c.order.Back()
	if oldest == nil {
		return
	}
	key, ok := oldest.Value.(string)
	if !ok {
		c.order.Remove(oldest)
		return
	}
	delete(c.items, key)
	c.order.Remove(oldest)
}

// Handler returns an http.Handler that serves cached payloads by id.
func (c *DataCache) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/data/"))
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
	if c.ttl > 0 && time.Since(entry.payload.storedAt) > c.ttl {
		delete(c.items, id)
		if entry.element != nil {
			c.order.Remove(entry.element)
		}
		return cachedPayload{}, false
	}
	if entry.element != nil {
		c.order.MoveToFront(entry.element)
	}
	return entry.payload, true
}

func (c *DataCache) getDataURI(key string) map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.dataURICache[key]
	if !ok {
		return nil
	}
	if c.ttl > 0 && time.Since(entry.storedAt) > c.ttl {
		delete(c.dataURICache, key)
		if entry.element != nil {
			c.dataURIOrder.Remove(entry.element)
		}
		return nil
	}
	if entry.element != nil {
		c.dataURIOrder.MoveToFront(entry.element)
	}
	return entry.descriptor
}

func (c *DataCache) storeDataURI(key string, descriptor map[string]any) {
	if c.dataURIMaxEntries <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.dataURICache[key]; ok {
		entry.descriptor = descriptor
		entry.storedAt = time.Now()
		if entry.element != nil {
			c.dataURIOrder.MoveToFront(entry.element)
		}
		return
	}
	element := c.dataURIOrder.PushFront(key)
	c.dataURICache[key] = &cachedDataURI{
		descriptor: descriptor,
		storedAt:   time.Now(),
		element:    element,
	}
	for len(c.dataURICache) > c.dataURIMaxEntries {
		c.evictDataURILocked()
	}
}

func (c *DataCache) evictDataURILocked() {
	if len(c.dataURICache) == 0 || c.dataURIOrder == nil {
		return
	}
	oldest := c.dataURIOrder.Back()
	if oldest == nil {
		return
	}
	key, ok := oldest.Value.(string)
	if !ok {
		c.dataURIOrder.Remove(oldest)
		return
	}
	delete(c.dataURICache, key)
	c.dataURIOrder.Remove(oldest)
}

func dataURIKey(value string) string {
	hash := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", hash[:])
}

func decodeDataURI(value string) (string, []byte, bool) {
	trimmed := strings.TrimSpace(value)
	parts := strings.SplitN(trimmed, ",", 2)
	if len(parts) != 2 {
		return "", nil, false
	}
	header := parts[0]
	payload := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, parts[1])

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
