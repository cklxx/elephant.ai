package react

import (
	"context"
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
// Implementations live in infrastructure packages (e.g. infra/checkpoint).
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

// CheckpointDirProvider is an optional interface that CheckpointStore
// implementations may satisfy to expose their base storage directory.
// Domain code uses this to co-locate artifacts (e.g. compaction files)
// alongside checkpoints without importing infra packages.
type CheckpointDirProvider interface {
	BaseDir() string
}
