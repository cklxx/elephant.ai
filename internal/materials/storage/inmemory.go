package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// InMemoryMapper is a testing helper that mimics uploads by storing bytes in
// process memory.
type InMemoryMapper struct {
	mu      sync.Mutex
	blobs   map[string][]byte
	baseCDN string
}

// NewInMemoryMapper constructs a mapper with the provided CDN base URL.
func NewInMemoryMapper(baseCDN string) *InMemoryMapper {
	return &InMemoryMapper{blobs: make(map[string][]byte), baseCDN: baseCDN}
}

// Upload persists the payload and returns deterministic metadata.
func (m *InMemoryMapper) Upload(ctx context.Context, req UploadRequest) (UploadResult, error) {
	if len(req.Data) == 0 {
		return UploadResult{}, fmt.Errorf("storage: empty payload for %s", req.Name)
	}
	sum := sha256.Sum256(req.Data)
	key := fmt.Sprintf("materials/%s", hex.EncodeToString(sum[:]))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blobs[key] = append([]byte(nil), req.Data...)
	return UploadResult{
		StorageKey:  key,
		CDNURL:      fmt.Sprintf("%s/%s", m.baseCDN, key),
		ContentHash: hex.EncodeToString(sum[:]),
		SizeBytes:   uint64(len(req.Data)),
	}, nil
}

// Delete removes a stored payload for cleanup tests.
func (m *InMemoryMapper) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.blobs, key)
	return nil
}

// Prewarm is a no-op in memory but satisfies the Mapper contract.
func (m *InMemoryMapper) Prewarm(ctx context.Context, key string) error {
	return nil
}

// Refresh is a no-op in memory but satisfies the Mapper contract.
func (m *InMemoryMapper) Refresh(ctx context.Context, key string) error {
	return nil
}

// Bytes returns the stored payload for assertions.
func (m *InMemoryMapper) Bytes(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.blobs[key]
	return append([]byte(nil), data...), ok
}
