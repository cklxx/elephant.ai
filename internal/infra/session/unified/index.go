package unified

import (
	"fmt"
	"os"
	"sync"
	"time"

	fstore "alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
)

// SurfaceIndex maps (surface, surfaceID) to sessionID. File-backed JSON.
type SurfaceIndex struct {
	mu       sync.RWMutex
	bindings map[string]SurfaceBinding
	filePath string
	nowFn    func() time.Time
}

// NewSurfaceIndex loads or creates a surface index at filePath.
func NewSurfaceIndex(filePath string, nowFn func() time.Time) (*SurfaceIndex, error) {
	idx := &SurfaceIndex{
		bindings: make(map[string]SurfaceBinding),
		filePath: filePath,
		nowFn:    nowFn,
	}
	if err := idx.load(); err != nil {
		return nil, err
	}
	return idx, nil
}

// Bind associates a surface-specific ID with a session.
func (idx *SurfaceIndex) Bind(surface Surface, surfaceID, sessionID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	key := bindingKey(surface, surfaceID)
	idx.bindings[key] = SurfaceBinding{
		Surface:   surface,
		SurfaceID: surfaceID,
		SessionID: sessionID,
		BoundAt:   idx.nowFn(),
	}
	return idx.persist()
}

// Lookup returns the sessionID for a surface binding, if it exists.
func (idx *SurfaceIndex) Lookup(surface Surface, surfaceID string) (string, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	b, ok := idx.bindings[bindingKey(surface, surfaceID)]
	return b.SessionID, ok
}

// Remove deletes a single surface binding.
func (idx *SurfaceIndex) Remove(surface Surface, surfaceID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	delete(idx.bindings, bindingKey(surface, surfaceID))
	return idx.persist()
}

// RemoveBySession deletes all bindings for a session.
func (idx *SurfaceIndex) RemoveBySession(sessionID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for key, b := range idx.bindings {
		if b.SessionID == sessionID {
			delete(idx.bindings, key)
		}
	}
	return idx.persist()
}

// ListForSession returns all bindings for a given session.
func (idx *SurfaceIndex) ListForSession(sessionID string) []SurfaceBinding {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var result []SurfaceBinding
	for _, b := range idx.bindings {
		if b.SessionID == sessionID {
			result = append(result, b)
		}
	}
	return result
}

func (idx *SurfaceIndex) load() error {
	data, err := fstore.ReadFileOrEmpty(idx.filePath)
	if err != nil {
		return fmt.Errorf("load surface index: %w", err)
	}
	if data == nil {
		return nil
	}
	return jsonx.Unmarshal(data, &idx.bindings)
}

func (idx *SurfaceIndex) persist() error {
	data, err := jsonx.MarshalIndent(idx.bindings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal surface index: %w", err)
	}
	return fstore.AtomicWrite(idx.filePath, data, 0o644)
}

func bindingKey(surface Surface, surfaceID string) string {
	return string(surface) + ":" + surfaceID
}

// snapshot returns a copy of all bindings for testing.
func (idx *SurfaceIndex) snapshot() map[string]SurfaceBinding {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	out := make(map[string]SurfaceBinding, len(idx.bindings))
	for k, v := range idx.bindings {
		out[k] = v
	}
	return out
}

// fileExists checks whether the backing file currently exists on disk.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
