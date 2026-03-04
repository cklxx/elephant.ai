package app

import (
	"context"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	sessionstate "alex/internal/infra/session/state_store"
	"alex/internal/shared/logging"
)

// SnapshotService handles snapshot listing, retrieval, and context
// window preview. Extracted from ServerCoordinator.
type SnapshotService struct {
	agentCoordinator AgentExecutor
	stateStore       sessionstate.Store
	broadcaster      *EventBroadcaster
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
			SessionID:       sessionID,
			RunID:           e.GetRunID(),
			ParentRunID:     e.GetParentRunID(),
			RequestID:       e.Data.RequestID,
			Iteration:       e.Data.Iteration,
			Timestamp:       e.Timestamp(),
			ContextMsgCount: e.Data.ContextMsgCount,
			ExcludedCount:   e.Data.ExcludedCount,
			ContextPreview:  e.Data.ContextPreview,
		}
		snapshots = append(snapshots, record)
		return nil
	})
	if len(snapshots) == 0 {
		return nil
	}
	return snapshots
}
