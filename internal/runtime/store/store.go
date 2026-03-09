// Package store persists runtime session metadata to the local filesystem.
// Each session is stored as a JSON file under a configurable directory.
// The store is the single source of truth for session recovery after restart.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"alex/internal/runtime/session"
)

// Store persists session metadata as JSON files on disk.
// File layout: <dir>/<session-id>.json
type Store struct {
	mu  sync.RWMutex
	dir string
}

// New creates a Store rooted at dir, creating the directory if needed.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: create dir %s: %w", dir, err)
	}
	return &Store{dir: dir}, nil
}

// Save writes (or overwrites) a session's metadata to disk.
func (s *Store) Save(sess *session.Session) error {
	snap := sess.Snapshot()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal session %s: %w", snap.ID, err)
	}

	path := s.path(snap.ID)
	tmp := path + ".tmp"

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("store: write session %s: %w", snap.ID, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("store: rename session %s: %w", snap.ID, err)
	}
	return nil
}

// Load reads a single session from disk by ID.
func (s *Store) Load(id string) (*session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("store: session %s not found", id)
		}
		return nil, fmt.Errorf("store: read session %s: %w", id, err)
	}

	var snap session.Session
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("store: decode session %s: %w", id, err)
	}
	return &snap, nil
}

// LoadAll reads every session file from the store directory.
func (s *Store) LoadAll() ([]*session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("store: read dir: %w", err)
	}

	var sessions []*session.Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue // skip unreadable files
		}
		var snap session.Session
		if err := json.Unmarshal(data, &snap); err != nil {
			continue // skip corrupt files
		}
		sessions = append(sessions, &snap)
	}
	return sessions, nil
}

// Delete removes a session file from disk.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// AppendEvent appends a serialised event line to <dir>/<session-id>.events.jsonl.
// The file is created on first write. Errors are silently ignored to keep
// event persistence from blocking the hot path.
func (s *Store) AppendEvent(sessionID string, eventType string, payload map[string]any) {
	if sessionID == "" {
		return
	}
	entry := make(map[string]any, len(payload)+2)
	for k, v := range payload {
		entry[k] = v
	}
	entry["session_id"] = sessionID
	entry["type"] = eventType
	entry["at"] = time.Now().UTC().Format(time.RFC3339Nano)

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	path := s.eventsPath(sessionID)
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *Store) eventsPath(id string) string {
	return filepath.Join(s.dir, id+".events.jsonl")
}
