package app

import (
	"context"
	"fmt"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/analytics/journal"
	"alex/internal/logging"
	sessionstate "alex/internal/session/state_store"
)

// ListSnapshots returns paginated session snapshots for API consumers.
func (s *ServerCoordinator) ListSnapshots(ctx context.Context, sessionID string, cursor string, limit int) ([]sessionstate.SnapshotMetadata, string, error) {
	if s.stateStore == nil {
		return nil, "", fmt.Errorf("state store not configured")
	}
	return s.stateStore.ListSnapshots(ctx, sessionID, cursor, limit)
}

// GetSnapshot fetches a specific turn snapshot.
func (s *ServerCoordinator) GetSnapshot(ctx context.Context, sessionID string, turnID int) (sessionstate.Snapshot, error) {
	if s.stateStore == nil {
		return sessionstate.Snapshot{}, fmt.Errorf("state store not configured")
	}
	return s.stateStore.GetSnapshot(ctx, sessionID, turnID)
}

// ReplaySession rehydrates the snapshot store from persisted turn journal entries.
func (s *ServerCoordinator) ReplaySession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}
	if s.journalReader == nil {
		return fmt.Errorf("journal reader not configured")
	}
	if s.stateStore == nil {
		return fmt.Errorf("state store not configured")
	}
	var snapshots []sessionstate.Snapshot
	streamErr := s.journalReader.Stream(ctx, sessionID, func(entry journal.TurnJournalEntry) error {
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
		return fmt.Errorf("no journal entries for session %s", sessionID)
	}
	if err := s.stateStore.ClearSession(ctx, sessionID); err != nil {
		return fmt.Errorf("clear state store: %w", err)
	}
	if err := s.stateStore.Init(ctx, sessionID); err != nil {
		return fmt.Errorf("init state store: %w", err)
	}
	for _, snapshot := range snapshots {
		if err := s.stateStore.SaveSnapshot(ctx, snapshot); err != nil {
			return fmt.Errorf("save snapshot: %w", err)
		}
	}
	logger := logging.FromContext(ctx, s.logger)
	logger.Info("[Replay] Rehydrated %d turn(s) for session %s", len(snapshots), sessionID)
	return nil
}

// PreviewContextWindow returns the constructed context window for a session.
func (s *ServerCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	if s.agentCoordinator == nil {
		return agent.ContextWindowPreview{}, fmt.Errorf("agent coordinator not configured")
	}
	return s.agentCoordinator.PreviewContextWindow(ctx, sessionID)
}

// GetContextSnapshots retrieves context snapshots captured during LLM calls for a session.
func (s *ServerCoordinator) GetContextSnapshots(sessionID string) []ContextSnapshotRecord {
	if s.broadcaster == nil || sessionID == "" {
		return nil
	}

	snapshots := make([]ContextSnapshotRecord, 0)
	filter := EventHistoryFilter{
		SessionID:  sessionID,
		EventTypes: []string{(&domain.WorkflowDiagnosticContextSnapshotEvent{}).EventType()},
	}
	_ = s.broadcaster.StreamHistory(context.Background(), filter, func(event agent.AgentEvent) error {
		snapshot, ok := event.(*domain.WorkflowDiagnosticContextSnapshotEvent)
		if !ok {
			return nil
		}
		record := ContextSnapshotRecord{
			SessionID:   sessionID,
			RunID:       snapshot.GetRunID(),
			ParentRunID: snapshot.GetParentRunID(),
			RequestID:    snapshot.RequestID,
			Iteration:    snapshot.Iteration,
			Timestamp:    snapshot.Timestamp(),
			Messages:     cloneMessages(snapshot.Messages),
			Excluded:     cloneMessages(snapshot.Excluded),
		}
		snapshots = append(snapshots, record)
		return nil
	})
	if len(snapshots) == 0 {
		return nil
	}
	return snapshots
}

func cloneMessages(messages []ports.Message) []ports.Message {
	if len(messages) == 0 {
		return nil
	}
	cloned := make([]ports.Message, len(messages))
	for i, msg := range messages {
		cloned[i] = cloneMessage(msg)
	}
	return cloned
}

func cloneMessage(msg ports.Message) ports.Message {
	cloned := msg
	if len(msg.ToolCalls) > 0 {
		cloned.ToolCalls = append([]ports.ToolCall(nil), msg.ToolCalls...)
	}
	if len(msg.ToolResults) > 0 {
		cloned.ToolResults = make([]ports.ToolResult, len(msg.ToolResults))
		for i, result := range msg.ToolResults {
			cloned.ToolResults[i] = cloneToolResult(result)
		}
	}
	if len(msg.Metadata) > 0 {
		metadata := make(map[string]any, len(msg.Metadata))
		for key, value := range msg.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(msg.Attachments) > 0 {
		cloned.Attachments = ports.CloneAttachmentMap(msg.Attachments)
	}
	return cloned
}

func cloneToolResult(result ports.ToolResult) ports.ToolResult {
	cloned := result
	if len(result.Metadata) > 0 {
		metadata := make(map[string]any, len(result.Metadata))
		for key, value := range result.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(result.Attachments) > 0 {
		cloned.Attachments = ports.CloneAttachmentMap(result.Attachments)
	}
	return cloned
}
