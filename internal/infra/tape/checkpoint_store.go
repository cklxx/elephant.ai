package tape

import (
	"context"
	"encoding/json"
	"fmt"

	coretape "alex/internal/core/tape"
	"alex/internal/domain/agent/react"
)

// CheckpointStore implements react.CheckpointStore by persisting checkpoints
// as tape entries with kind=checkpoint. Each session's checkpoint is stored
// on a tape named "cp_{sessionID}".
type CheckpointStore struct {
	store   coretape.TapeStore
	baseDir string
}

// NewCheckpointStore returns a CheckpointStore backed by the given TapeStore.
// baseDir is exposed via BaseDir() for CheckpointDirProvider compatibility.
func NewCheckpointStore(store coretape.TapeStore, baseDir string) *CheckpointStore {
	return &CheckpointStore{store: store, baseDir: baseDir}
}

// BaseDir implements react.CheckpointDirProvider.
func (s *CheckpointStore) BaseDir() string { return s.baseDir }

// Save persists a checkpoint as a tape entry, overwriting any previous
// checkpoint for the same session by deleting the tape first.
func (s *CheckpointStore) Save(ctx context.Context, cp *react.Checkpoint) error {
	if cp == nil {
		return fmt.Errorf("checkpoint: cannot save nil checkpoint")
	}
	if cp.SessionID == "" {
		return fmt.Errorf("checkpoint: session_id is required")
	}

	data, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("checkpoint: marshal failed: %w", err)
	}

	tapeName := checkpointTapeName(cp.SessionID)

	// Overwrite semantics: delete the old tape, then append.
	if err := s.store.Delete(ctx, tapeName); err != nil {
		return fmt.Errorf("checkpoint: delete old tape: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("checkpoint: unmarshal payload: %w", err)
	}

	entry := coretape.NewCheckpoint("checkpoint", payload, coretape.EntryMeta{
		SessionID: cp.SessionID,
	})
	if err := s.store.Append(ctx, tapeName, entry); err != nil {
		return fmt.Errorf("checkpoint: append failed: %w", err)
	}
	return nil
}

// Load retrieves the most recent checkpoint for sessionID.
// Returns (nil, nil) when no checkpoint exists.
func (s *CheckpointStore) Load(ctx context.Context, sessionID string) (*react.Checkpoint, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("checkpoint: session_id is required")
	}

	entries, err := s.store.Query(ctx, checkpointTapeName(sessionID), coretape.Query().Kinds(coretape.KindCheckpoint))
	if err != nil {
		return nil, fmt.Errorf("checkpoint: query failed: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}

	// Take the last (most recent) entry.
	last := entries[len(entries)-1]
	state, ok := last.Payload["state"]
	if !ok {
		state = last.Payload
	}

	raw, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("checkpoint: marshal state: %w", err)
	}

	var cp react.Checkpoint
	if err := json.Unmarshal(raw, &cp); err != nil {
		return nil, fmt.Errorf("checkpoint: unmarshal checkpoint: %w", err)
	}
	return &cp, nil
}

// Delete removes the checkpoint tape for sessionID. No-op if absent.
func (s *CheckpointStore) Delete(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("checkpoint: session_id is required")
	}
	return s.store.Delete(ctx, checkpointTapeName(sessionID))
}

// SaveArchive persists an archive entry on a separate archive tape.
func (s *CheckpointStore) SaveArchive(ctx context.Context, archive *react.CheckpointArchive) error {
	if archive == nil {
		return fmt.Errorf("checkpoint: cannot save nil archive")
	}
	if archive.SessionID == "" {
		return fmt.Errorf("checkpoint: archive session_id is required")
	}

	data, err := json.Marshal(archive)
	if err != nil {
		return fmt.Errorf("checkpoint: archive marshal failed: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("checkpoint: archive unmarshal payload: %w", err)
	}

	tapeName := archiveTapeName(archive.SessionID)
	entry := coretape.NewCheckpoint(fmt.Sprintf("archive_%d", archive.Seq), payload, coretape.EntryMeta{
		SessionID: archive.SessionID,
		Seq:       int64(archive.Seq),
	})
	if err := s.store.Append(ctx, tapeName, entry); err != nil {
		return fmt.Errorf("checkpoint: archive append failed: %w", err)
	}
	return nil
}

func checkpointTapeName(sessionID string) string {
	return "cp_" + sessionID
}

func archiveTapeName(sessionID string) string {
	return "cp_archive_" + sessionID
}
