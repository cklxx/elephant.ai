package state_store

import (
	"context"
	"errors"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

// ErrSnapshotNotFound is returned when a specific turn snapshot cannot be located.
var ErrSnapshotNotFound = errors.New("snapshot not found")

// Store defines the behaviour required by context/state consumers.
type Store interface {
	Init(ctx context.Context, sessionID string) error
	// ClearSession removes any persisted state for the provided session so callers can
	// safely rebuild snapshots from a known-good source (e.g. the journal).
	ClearSession(ctx context.Context, sessionID string) error
	SaveSnapshot(ctx context.Context, snapshot Snapshot) error
	LatestSnapshot(ctx context.Context, sessionID string) (Snapshot, error)
	GetSnapshot(ctx context.Context, sessionID string, turnID int) (Snapshot, error)
	ListSnapshots(ctx context.Context, sessionID string, cursor string, limit int) ([]SnapshotMetadata, string, error)
}

// Snapshot captures the structured view of a single turn.
type Snapshot struct {
	SessionID     string                     `json:"session_id"`
	TurnID        int                        `json:"turn_id"`
	LLMTurnSeq    int                        `json:"llm_turn_seq"`
	CreatedAt     time.Time                  `json:"created_at"`
	Summary       string                     `json:"summary"`
	Plans         []agent.PlanNode           `json:"plans"`
	Beliefs       []agent.Belief             `json:"beliefs"`
	World         map[string]any             `json:"world_state"`
	Diff          map[string]any             `json:"diff"`
	Messages      []ports.Message            `json:"messages"`
	Feedback      []agent.FeedbackSignal     `json:"feedback"`
	KnowledgeRefs []agent.KnowledgeReference `json:"knowledge_refs"`
}

// SnapshotMetadata provides lightweight info for pagination listings.
type SnapshotMetadata struct {
	SessionID  string    `json:"session_id"`
	TurnID     int       `json:"turn_id"`
	LLMTurnSeq int       `json:"llm_turn_seq"`
	Summary    string    `json:"summary"`
	CreatedAt  time.Time `json:"created_at"`
}
