package app

import (
	"context"
	"sort"

	"alex/internal/domain/agent"
	ports "alex/internal/domain/agent/ports"
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
	historyStore     sessionstate.Store
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

// WithSnapshotHistoryStore wires the history (turns) store for on-demand
// message retrieval without duplicating messages in the state store.
func WithSnapshotHistoryStore(store sessionstate.Store) SnapshotServiceOption {
	return func(svc *SnapshotService) {
		svc.historyStore = store
	}
}

// ListSnapshots returns paginated session snapshots.
func (svc *SnapshotService) ListSnapshots(ctx context.Context, sessionID string, cursor string, limit int) ([]sessionstate.SnapshotMetadata, string, error) {
	if svc.stateStore == nil {
		return nil, "", UnavailableError("state store not configured")
	}
	return svc.stateStore.ListSnapshots(ctx, sessionID, cursor, limit)
}

// GetSnapshot fetches a specific turn snapshot. If the snapshot has no
// messages and a history store is configured, messages are loaded on-demand
// from the history store to keep the write path memory-friendly.
func (svc *SnapshotService) GetSnapshot(ctx context.Context, sessionID string, turnID int) (sessionstate.Snapshot, error) {
	if svc.stateStore == nil {
		return sessionstate.Snapshot{}, UnavailableError("state store not configured")
	}
	snapshot, err := svc.stateStore.GetSnapshot(ctx, sessionID, turnID)
	if err != nil {
		return snapshot, err
	}
	if len(snapshot.Messages) == 0 && svc.historyStore != nil {
		snapshot.Messages = svc.loadHistoryMessages(ctx, sessionID)
	}
	return snapshot, nil
}

// loadHistoryMessages replays all turns from the history store into a flat
// message slice. Kept as a best-effort helper — errors are swallowed so that
// structural snapshot data is still returned.
func (svc *SnapshotService) loadHistoryMessages(ctx context.Context, sessionID string) []ports.Message {
	lister, ok := svc.historyStore.(sessionstate.SnapshotPayloadLister)
	if ok {
		return svc.loadHistoryViaPayloads(ctx, sessionID, lister)
	}
	return svc.loadHistoryViaList(ctx, sessionID)
}

func (svc *SnapshotService) loadHistoryViaPayloads(ctx context.Context, sessionID string, lister sessionstate.SnapshotPayloadLister) []ports.Message {
	cursor := ""
	var messages []ports.Message
	for {
		page, next, err := lister.ListSnapshotPayloads(ctx, sessionID, cursor, 200)
		if err != nil || len(page) == 0 {
			break
		}
		sort.Slice(page, func(i, j int) bool { return page[i].TurnID < page[j].TurnID })
		for _, snap := range page {
			messages = append(messages, snap.Messages...)
		}
		if next == "" {
			break
		}
		cursor = next
	}
	return messages
}

func (svc *SnapshotService) loadHistoryViaList(ctx context.Context, sessionID string) []ports.Message {
	metas, _, err := svc.historyStore.ListSnapshots(ctx, sessionID, "", 200)
	if err != nil || len(metas) == 0 {
		return nil
	}
	var messages []ports.Message
	for _, meta := range metas {
		snap, err := svc.historyStore.GetSnapshot(ctx, sessionID, meta.TurnID)
		if err != nil {
			continue
		}
		messages = append(messages, snap.Messages...)
	}
	return messages
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
