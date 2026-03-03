package react

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// testFileCheckpointStore is a minimal filesystem-backed CheckpointStore used
// exclusively by tests within this package. It avoids importing infra/checkpoint
// which would create an import cycle (react ↔ infra/checkpoint).
type testFileCheckpointStore struct {
	Dir string
}

func newTestFileCheckpointStore(dir string) *testFileCheckpointStore {
	return &testFileCheckpointStore{Dir: dir}
}

func (s *testFileCheckpointStore) BaseDir() string { return s.Dir }

func (s *testFileCheckpointStore) Save(_ context.Context, cp *Checkpoint) error {
	if cp == nil {
		return fmt.Errorf("checkpoint: cannot save nil checkpoint")
	}
	if cp.SessionID == "" {
		return fmt.Errorf("checkpoint: session_id is required")
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.Dir, cp.SessionID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *testFileCheckpointStore) Load(_ context.Context, sessionID string) (*Checkpoint, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("checkpoint: session_id is required")
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, sessionID+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

func (s *testFileCheckpointStore) Delete(_ context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("checkpoint: session_id is required")
	}
	err := os.Remove(filepath.Join(s.Dir, sessionID+".json"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *testFileCheckpointStore) SaveArchive(_ context.Context, archive *CheckpointArchive) error {
	if archive == nil {
		return fmt.Errorf("checkpoint: cannot save nil archive")
	}
	if archive.SessionID == "" {
		return fmt.Errorf("checkpoint: archive session_id is required")
	}
	data, err := json.MarshalIndent(archive, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.Dir, archive.SessionID, "archive", fmt.Sprintf("%d.json", archive.Seq))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
