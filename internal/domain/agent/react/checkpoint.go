package react

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CheckpointVersion is the current schema version. Bump this when the
// Checkpoint layout changes in a backward-incompatible way so that loaders
// can detect stale data and apply migrations.
const CheckpointVersion = 1

// Checkpoint captures the minimal, JSON-serializable snapshot of a ReAct
// engine run at a given iteration boundary. It is designed to be written after
// each observe phase so that the engine can be restarted from the last
// successful iteration without replaying LLM calls or tool executions.
type Checkpoint struct {
	// ID is a unique identifier for this checkpoint instance (typically a UUID).
	ID string `json:"id"`
	// SessionID ties the checkpoint to the conversation session it belongs to.
	SessionID string `json:"session_id"`
	// Iteration is the zero-based iteration index at which the snapshot was
	// taken. On restore the engine should resume from Iteration+1.
	Iteration int `json:"iteration"`
	// MaxIterations records the configured iteration cap so the restored
	// engine can enforce the same limit.
	MaxIterations int `json:"max_iterations"`
	// Messages is the ordered conversation history (system, user, assistant,
	// tool) accumulated up to and including this iteration.
	Messages []MessageState `json:"messages"`
	// PendingTools captures tool calls that were dispatched but had not yet
	// produced a result when the checkpoint was created. On restore, the
	// engine may choose to re-execute or skip these calls.
	PendingTools []ToolCallState `json:"pending_tools"`
	// CreatedAt is the wall-clock time when the checkpoint was persisted.
	CreatedAt time.Time `json:"created_at"`
	// Version is the schema version used to encode this checkpoint. Loaders
	// must check this field before unmarshalling to apply any necessary
	// migrations.
	Version int `json:"version"`
}

// MessageState is a simplified, serialization-friendly representation of a
// conversation message. It intentionally omits provider-specific metadata,
// attachments, and thinking traces to keep checkpoints compact and portable.
type MessageState struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolCallState records the identity and progress of a single tool invocation
// at checkpoint time.
type ToolCallState struct {
	// ID is the unique call identifier assigned by the LLM response parser.
	ID string `json:"id"`
	// Name is the registered tool name (e.g. "web_search", "shell_exec").
	Name string `json:"name"`
	// Arguments is the decoded argument map passed to the tool.
	Arguments map[string]any `json:"arguments"`
	// Status describes the lifecycle phase of the call: "pending", "running",
	// "completed", or "failed".
	Status string `json:"status"`
	// Result holds the tool output when Status is "completed". It is nil for
	// calls that have not yet finished or that ended in failure.
	Result *string `json:"result,omitempty"`
}

// ---------------------------------------------------------------------------
// CheckpointStore defines the persistence contract for engine checkpoints.
// Implementations range from a simple filesystem store (below) to database or
// object-storage backends.
// ---------------------------------------------------------------------------

// CheckpointStore is the port through which the ReAct engine persists and
// retrieves execution checkpoints.
type CheckpointStore interface {
	// Save persists the checkpoint, overwriting any previous checkpoint for
	// the same session.
	Save(ctx context.Context, cp *Checkpoint) error
	// Load retrieves the most recent checkpoint for the given session. It
	// returns (nil, nil) when no checkpoint exists.
	Load(ctx context.Context, sessionID string) (*Checkpoint, error)
	// Delete removes the checkpoint for the given session. It is a no-op if
	// no checkpoint exists.
	Delete(ctx context.Context, sessionID string) error
	// SaveArchive persists pruned messages from a context checkpoint for
	// audit purposes. Archives are never loaded back into state.Messages.
	SaveArchive(ctx context.Context, archive *CheckpointArchive) error
}

// CheckpointArchive stores pruned messages from a context checkpoint.
// It is audit-only — archives are never reloaded into conversation state.
type CheckpointArchive struct {
	SessionID  string         `json:"session_id"`
	Seq        int            `json:"seq"`
	PhaseLabel string         `json:"phase_label"`
	Messages   []MessageState `json:"messages"`
	TokenCount int            `json:"token_count"`
	CreatedAt  time.Time      `json:"created_at"`
}

// ---------------------------------------------------------------------------
// FileCheckpointStore — a minimal, filesystem-backed implementation suitable
// for local development and single-node deployments.
// ---------------------------------------------------------------------------

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

// Save marshals cp to JSON and writes it to {Dir}/{cp.SessionID}.json.
func (s *FileCheckpointStore) Save(_ context.Context, cp *Checkpoint) error {
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("checkpoint: create dir failed: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("checkpoint: write failed: %w", err)
	}
	return nil
}

// Load reads the checkpoint file for sessionID and unmarshals it. If the file
// does not exist, it returns (nil, nil).
func (s *FileCheckpointStore) Load(_ context.Context, sessionID string) (*Checkpoint, error) {
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

	var cp Checkpoint
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
func (s *FileCheckpointStore) SaveArchive(_ context.Context, archive *CheckpointArchive) error {
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

	dir := filepath.Join(s.Dir, archive.SessionID, "archive")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("checkpoint: archive create dir failed: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("%d.json", archive.Seq))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("checkpoint: archive write failed: %w", err)
	}
	return nil
}

// path returns the filesystem path for the given session's checkpoint.
func (s *FileCheckpointStore) path(sessionID string) string {
	return filepath.Join(s.Dir, sessionID+".json")
}
