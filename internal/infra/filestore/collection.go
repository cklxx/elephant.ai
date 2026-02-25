package filestore

import (
	"os"
	"sync"
	"time"

	jsonx "alex/internal/shared/json"
)

// CollectionConfig configures a Collection.
type CollectionConfig struct {
	FilePath string      // empty = in-memory only
	Perm     os.FileMode // file permissions; default 0o600
	Name     string      // for logging/debugging
}

// Collection is a generic in-memory map backed by a single JSON file.
// It handles atomic persistence, concurrent access, and custom envelope formats.
//
// Type parameters:
//   - K: map key (must be comparable)
//   - V: map value
type Collection[K comparable, V any] struct {
	mu       sync.RWMutex
	items    map[K]V
	filePath string
	perm     os.FileMode
	name     string

	// marshalDoc converts the in-memory map to its on-disk JSON representation.
	// If nil, the map is serialised directly.
	marshalDoc func(map[K]V) ([]byte, error)

	// unmarshalDoc populates the map from the on-disk JSON representation.
	// If nil, the data is unmarshalled directly into the map.
	unmarshalDoc func([]byte) (map[K]V, error)

	// Now returns the current time. Defaults to time.Now.
	// Exposed for test injection.
	Now func() time.Time
}

// NewCollection creates a new Collection. Call Load to populate from disk.
func NewCollection[K comparable, V any](cfg CollectionConfig) *Collection[K, V] {
	perm := cfg.Perm
	if perm == 0 {
		perm = 0o600
	}
	return &Collection[K, V]{
		items:    make(map[K]V),
		filePath: cfg.FilePath,
		perm:     perm,
		name:     cfg.Name,
		Now:      time.Now,
	}
}

// SetMarshalDoc sets a custom marshal function for the on-disk envelope format.
func (c *Collection[K, V]) SetMarshalDoc(fn func(map[K]V) ([]byte, error)) {
	c.marshalDoc = fn
}

// SetUnmarshalDoc sets a custom unmarshal function for the on-disk envelope format.
func (c *Collection[K, V]) SetUnmarshalDoc(fn func([]byte) (map[K]V, error)) {
	c.unmarshalDoc = fn
}

// Load reads the backing file into the in-memory map.
// No-op if filePath is empty or the file doesn't exist.
func (c *Collection[K, V]) Load() error {
	if c.filePath == "" {
		return nil
	}
	data, err := ReadFileOrEmpty(c.filePath)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.unmarshalDoc != nil {
		items, unmarshalErr := c.unmarshalDoc(data)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		c.items = items
		return nil
	}

	var m map[K]V
	if err := jsonx.Unmarshal(data, &m); err != nil {
		return err
	}
	c.items = m
	return nil
}

// EnsureDir creates the parent directory of the backing file.
// No-op if filePath is empty.
func (c *Collection[K, V]) EnsureDir() error {
	if c.filePath == "" {
		return nil
	}
	return EnsureParentDir(c.filePath)
}

// Get returns the value for key and whether it exists.
func (c *Collection[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.items[key]
	return v, ok
}

// Put sets a key-value pair and persists.
func (c *Collection[K, V]) Put(key K, value V) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = value
	return c.persistLocked()
}

// Delete removes a key and persists.
func (c *Collection[K, V]) Delete(key K) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return c.persistLocked()
}

// Len returns the number of items.
func (c *Collection[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Snapshot returns a shallow copy of the in-memory map.
func (c *Collection[K, V]) Snapshot() map[K]V {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap := make(map[K]V, len(c.items))
	for k, v := range c.items {
		snap[k] = v
	}
	return snap
}

// Mutate gives the caller exclusive access to the underlying map.
// The function fn receives the live map — mutations are visible immediately.
// After fn returns, the collection is persisted.
// If fn returns an error, no persistence occurs and the error is returned.
func (c *Collection[K, V]) Mutate(fn func(items map[K]V) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := fn(c.items); err != nil {
		return err
	}
	return c.persistLocked()
}

// MutateWithRollback is like Mutate but snapshots the map before fn.
// If fn returns an error, the map is restored to the pre-call state.
func (c *Collection[K, V]) MutateWithRollback(fn func(items map[K]V) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	snapshot := make(map[K]V, len(c.items))
	for k, v := range c.items {
		snapshot[k] = v
	}

	if err := fn(c.items); err != nil {
		c.items = snapshot
		return err
	}
	return c.persistLocked()
}

// ReadLocked calls fn with the map under a read lock. No persistence.
func (c *Collection[K, V]) ReadLocked(fn func(items map[K]V)) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	fn(c.items)
}

func (c *Collection[K, V]) persistLocked() error {
	if c.filePath == "" {
		return nil
	}

	var data []byte
	var err error
	if c.marshalDoc != nil {
		data, err = c.marshalDoc(c.items)
	} else {
		data, err = MarshalJSONIndent(c.items)
	}
	if err != nil {
		return err
	}
	return AtomicWrite(c.filePath, data, c.perm)
}
