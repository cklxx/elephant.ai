package app

import (
	"context"
	"fmt"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/infra/analytics/journal"
	sessionstate "alex/internal/infra/session/state_store"
	"alex/internal/shared/logging"
)

// SnapshotService handles snapshot listing, retrieval, replay, and context
// window preview. Extracted from ServerCoordinator.
type SnapshotService struct {
	agentCoordinator AgentExecutor
	stateStore       sessionstate.Store
	broadcaster      *EventBroadcaster
	journalReader    journal.Reader
	logger           logging.Logger
}

// NewSnapshotService creates a new snapshot service.
func NewSnapshotService(
	agentCoordinator AgentExecutor,
	broadcaster *EventBroadcaster,
	opts ...SnapshotServiceOption,
) *SnapshotService {
	svc := &SnapshotService{
		agentCoordinator: agentCoordinator,
		broadcaster:      broadcaster,
		logger:           logging.NewComponentLogger("SnapshotService"),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

// SnapshotServiceOption configures optional behavior.
type SnapshotServiceOption func(*SnapshotService)

// WithSnapshotStateStore wires the state store.
func WithSnapshotStateStore(store sessionstate.Store) SnapshotServiceOption {
	return func(svc *SnapshotService) {
		svc.stateStore = store
	}
}

// WithSnapshotJournalReader wires the journal reader for replay.
func WithSnapshotJournalReader(reader journal.Reader) SnapshotServiceOption {
	return func(svc *SnapshotService) {
		svc.journalReader = reader
	}
}

// ListSnapshots returns paginated session snapshots.
func (svc *SnapshotService) ListSnapshots(ctx context.Context, sessionID string, cursor string, limit int) ([]sessionstate.SnapshotMetadata, string, error) {
	if svc.stateStore == nil {
		return nil, "", UnavailableError("state store not configured")
	}
	return svc.stateStore.ListSnapshots(ctx, sessionID, cursor, limit)
}

// GetSnapshot fetches a specific turn snapshot.
func (svc *SnapshotService) GetSnapshot(ctx context.Context, sessionID string, turnID int) (sessionstate.Snapshot, error) {
	if svc.stateStore == nil {
		return sessionstate.Snapshot{}, UnavailableError("state store not configured")
	}
	return svc.stateStore.GetSnapshot(ctx, sessionID, turnID)
}

// ReplaySession rehydrates the snapshot store from persisted turn journal entries.
func (svc *SnapshotService) ReplaySession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return ValidationError("session id required")
	}
	if svc.journalReader == nil {
		return UnavailableError("journal reader not configured")
	}
	if svc.stateStore == nil {
		return UnavailableError("state store not configured")
	}
	var snapshots []sessionstate.Snapshot
	streamErr := svc.journalReader.Stream(ctx, sessionID, func(entry journal.TurnJournalEntry) error {
		snapshot := sessionstate.Snapshot{
			SessionID:     entry.SessionID,
			TurnID:        entry.TurnID,
			LLMTurnSeq:    entry.LLMTurnSeq,
			Summary:       entry.Summary,
			Plans:         entry.Plans,
			Beliefs:       entry.Beliefs,
			World:         entry.World,
			Diff:          entry.Diff,
			Messages:      entry.Messages,
			Feedback:      entry.Feedback,
			KnowledgeRefs: entry.KnowledgeRefs,
		}
		if snapshot.SessionID == "" {
			snapshot.SessionID = sessionID
		}
		if entry.Timestamp.IsZero() {
			snapshot.CreatedAt = time.Now().UTC()
		} else {
			snapshot.CreatedAt = entry.Timestamp
		}
		snapshots = append(snapshots, snapshot)
		return nil
	})
	if streamErr != nil {
		return fmt.Errorf("replay journal: %w", streamErr)
	}
	if len(snapshots) == 0 {
		return NotFoundError(fmt.Sprintf("no journal entries for session %s", sessionID))
	}
	if err := svc.stateStore.ClearSession(ctx, sessionID); err != nil {
		return fmt.Errorf("clear state store: %w", err)
	}
	if err := svc.stateStore.Init(ctx, sessionID); err != nil {
		return fmt.Errorf("init state store: %w", err)
	}
	for _, snapshot := range snapshots {
		if err := svc.stateStore.SaveSnapshot(ctx, snapshot); err != nil {
			return fmt.Errorf("save snapshot: %w", err)
		}
	}
	logger := logging.FromContext(ctx, svc.logger)
	logger.Info("[Replay] Rehydrated %d turn(s) for session %s", len(snapshots), sessionID)
	return nil
}

// PreviewContextWindow returns the constructed context window for a session.
func (svc *SnapshotService) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	if svc.agentCoordinator == nil {
		return agent.ContextWindowPreview{}, UnavailableError("agent coordinator not configured")
	}
	return svc.agentCoordinator.PreviewContextWindow(ctx, sessionID)
}

// GetContextSnapshots retrieves context snapshots captured during LLM calls for a session.
func (svc *SnapshotService) GetContextSnapshots(sessionID string) []ContextSnapshotRecord {
	if svc.broadcaster == nil || sessionID == "" {
		return nil
	}

	snapshots := make([]ContextSnapshotRecord, 0)
	filter := EventHistoryFilter{
		SessionID:  sessionID,
		EventTypes: []string{types.EventDiagnosticContextSnapshot},
	}
	_ = svc.broadcaster.StreamHistory(context.Background(), filter, func(event agent.AgentEvent) error {
		e, ok := event.(*domain.Event)
		if !ok || e.Kind != types.EventDiagnosticContextSnapshot {
			return nil
		}
		record := ContextSnapshotRecord{
			SessionID:   sessionID,
			RunID:       e.GetRunID(),
			ParentRunID: e.GetParentRunID(),
			RequestID:   e.Data.RequestID,
			Iteration:   e.Data.Iteration,
			Timestamp:   e.Timestamp(),
			Messages:    cloneMessages(e.Data.Messages),
			Excluded:    cloneMessages(e.Data.Excluded),
		}
		snapshots = append(snapshots, record)
		return nil
	})
	if len(snapshots) == 0 {
		return nil
	}
	return snapshots
}
