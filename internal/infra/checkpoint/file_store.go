package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"alex/internal/domain/agent/react"
	"alex/internal/infra/filestore"
)

// FileCheckpointStore persists checkpoints as individual JSON files inside a
// directory. Each session maps to {Dir}/{sessionID}.json.
type FileCheckpointStore struct {
	// Dir is the directory where checkpoint files are stored. It must exist
	// before Save is called.
	Dir string
}

// NewFileCheckpointStore returns a store that writes checkpoints under dir.
func NewFileCheckpointStore(dir string) *FileCheckpointStore {
	return &FileCheckpointStore{Dir: dir}
}

// BaseDir returns the checkpoint storage directory. This implements the
// react.CheckpointDirProvider interface so domain code can discover the
// storage root without importing the infra package.
func (s *FileCheckpointStore) BaseDir() string {
	return s.Dir
}

// Save marshals cp to JSON and writes it to {Dir}/{cp.SessionID}.json.
func (s *FileCheckpointStore) Save(_ context.Context, cp *react.Checkpoint) error {
	if cp == nil {
		return fmt.Errorf("checkpoint: cannot save nil checkpoint")
	}
	if cp.SessionID == "" {
		return fmt.Errorf("checkpoint: session_id is required")
	}

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("checkpoint: marshal failed: %w", err)
	}

	path := s.path(cp.SessionID)
	if err := filestore.AtomicWrite(path, data, 0o644); err != nil {
		return fmt.Errorf("checkpoint: write failed: %w", err)
	}
	return nil
}

// Load reads the checkpoint file for sessionID and unmarshals it. If the file
// does not exist, it returns (nil, nil).
func (s *FileCheckpointStore) Load(_ context.Context, sessionID string) (*react.Checkpoint, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("checkpoint: session_id is required")
	}

	data, err := os.ReadFile(s.path(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("checkpoint: read failed: %w", err)
	}

	var cp react.Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("checkpoint: unmarshal failed: %w", err)
	}
	return &cp, nil
}

// Delete removes the checkpoint file for sessionID. If the file does not
// exist, Delete is a silent no-op.
func (s *FileCheckpointStore) Delete(_ context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("checkpoint: session_id is required")
	}

	err := os.Remove(s.path(sessionID))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("checkpoint: delete failed: %w", err)
	}
	return nil
}

// SaveArchive writes pruned messages to {Dir}/{sessionID}/archive/{seq}.json.
func (s *FileCheckpointStore) SaveArchive(_ context.Context, archive *react.CheckpointArchive) error {
	if archive == nil {
		return fmt.Errorf("checkpoint: cannot save nil archive")
	}
	if archive.SessionID == "" {
		return fmt.Errorf("checkpoint: archive session_id is required")
	}

	data, err := json.MarshalIndent(archive, "", "  ")
	if err != nil {
		return fmt.Errorf("checkpoint: archive marshal failed: %w", err)
	}

	path := filepath.Join(s.Dir, archive.SessionID, "archive", fmt.Sprintf("%d.json", archive.Seq))
	if err := filestore.AtomicWrite(path, data, 0o644); err != nil {
		return fmt.Errorf("checkpoint: archive write failed: %w", err)
	}
	return nil
}

// path returns the filesystem path for the given session's checkpoint.
func (s *FileCheckpointStore) path(sessionID string) string {
	return filepath.Join(s.Dir, sessionID+".json")
}
