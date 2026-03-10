package app

import (
	"context"
	"strings"
	"time"

	domain "alex/internal/domain/agent"
	agentports "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

// storeEventHistory stores an event in the session's history
func (b *EventBroadcaster) storeEventHistory(sessionID string, event agentports.AgentEvent) {
	event = sanitizeEventForHistory(event)
	if event == nil {
		return
	}
	if b.historyStore != nil {
		if err := b.historyStore.Append(context.Background(), event); err != nil {
			b.logger.Warn("Failed to persist event history for session %s: %v", sessionID, err)
		}
		return
	}

	now := time.Now()

	b.historyMu.Lock()
	defer b.historyMu.Unlock()

	if b.pruneCounter.Add(1)%pruneEveryN == 0 {
		b.pruneExpiredSessionsLocked(now)
	}

	history := b.eventHistory[sessionID]
	if history == nil {
		history = &sessionHistory{}
	}
	history.events = append(history.events, event)
	history.lastSeen = now

	// Trim history if it exceeds max size
	if b.maxHistory > 0 && len(history.events) > b.maxHistory {
		history.events = history.events[len(history.events)-b.maxHistory:]
	}

	b.eventHistory[sessionID] = history
	b.enforceMaxSessionsLocked()
}

func (b *EventBroadcaster) storeGlobalEvent(event agentports.AgentEvent) {
	event = sanitizeEventForHistory(event)
	if event == nil {
		return
	}
	if b.historyStore != nil {
		if err := b.historyStore.Append(context.Background(), event); err != nil {
			b.logger.Warn("Failed to persist global event history: %v", err)
		}
		return
	}

	b.globalMu.Lock()
	defer b.globalMu.Unlock()

	b.globalHistory = append(b.globalHistory, event)
	if len(b.globalHistory) > b.maxHistory {
		b.globalHistory = b.globalHistory[len(b.globalHistory)-b.maxHistory:]
	}
}

func sanitizeEventForHistory(event agentports.AgentEvent) agentports.AgentEvent {
	if event == nil {
		return nil
	}
	if wrapper, ok := event.(agentports.SubtaskWrapper); ok && wrapper != nil {
		if setter, ok := event.(interface{ SetWrappedEvent(agentports.AgentEvent) }); ok {
			if sanitized := sanitizeBaseEventForHistory(wrapper.WrappedEvent()); sanitized != nil {
				setter.SetWrappedEvent(sanitized)
			}
		}
		return event
	}

	return sanitizeBaseEventForHistory(event)
}

func sanitizeBaseEventForHistory(event agentports.AgentEvent) agentports.AgentEvent {
	if event == nil {
		return nil
	}
	base := BaseAgentEvent(event)
	if base == nil {
		return nil
	}

	switch e := base.(type) {
	case *domain.WorkflowEventEnvelope:
		cloned := *e
		if e.Payload != nil {
			if cleaned, ok := stripBinaryPayloadsWithStore(e.Payload, nil).(map[string]any); ok {
				cloned.Payload = cleaned
			}
		}
		return &cloned
	case *domain.Event:
		cloned := *e
		cloned.Data = sanitizeDomainEventDataForHistory(e.Kind, e.Data)
		return &cloned
	default:
		return base
	}
}

// StreamHistory streams stored events for a session or global scope.
func (b *EventBroadcaster) StreamHistory(ctx context.Context, filter EventHistoryFilter, fn func(agentports.AgentEvent) error) error {
	if b == nil || fn == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if b.historyStore != nil {
		return b.historyStore.Stream(ctx, filter, fn)
	}

	var history []agentports.AgentEvent
	if filter.SessionID == "" {
		b.globalMu.RLock()
		history = append(history, b.globalHistory...)
		b.globalMu.RUnlock()
	} else {
		now := time.Now()
		b.historyMu.RLock()
		if entry := b.eventHistory[filter.SessionID]; entry != nil {
			if b.sessionTTL <= 0 || now.Sub(entry.lastSeen) <= b.sessionTTL {
				history = append(history, entry.events...)
			}
		}
		b.historyMu.RUnlock()
	}
	if len(history) == 0 {
		return nil
	}

	var filterSet map[string]struct{}
	if len(filter.EventTypes) > 0 {
		filterSet = make(map[string]struct{}, len(filter.EventTypes))
		for _, eventType := range filter.EventTypes {
			filterSet[eventType] = struct{}{}
		}
	}

	for _, event := range history {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		base := BaseAgentEvent(event)
		if base == nil {
			continue
		}
		if filterSet != nil {
			if _, ok := filterSet[base.EventType()]; !ok {
				continue
			}
		}
		if err := fn(event); err != nil {
			return err
		}
	}
	return nil
}

// GetEventHistory returns all stored events for a session
func (b *EventBroadcaster) GetEventHistory(sessionID string) []agentports.AgentEvent {
	var history []agentports.AgentEvent
	_ = b.StreamHistory(context.Background(), EventHistoryFilter{SessionID: sessionID}, func(event agentports.AgentEvent) error {
		history = append(history, event)
		return nil
	})
	if len(history) == 0 {
		return nil
	}
	return history
}

// GetGlobalHistory returns global events for diagnostics replay.
func (b *EventBroadcaster) GetGlobalHistory() []agentports.AgentEvent {
	var history []agentports.AgentEvent
	_ = b.StreamHistory(context.Background(), EventHistoryFilter{SessionID: ""}, func(event agentports.AgentEvent) error {
		history = append(history, event)
		return nil
	})
	if len(history) == 0 {
		return nil
	}
	return history
}

// ClearEventHistory clears the event history for a session
func (b *EventBroadcaster) ClearEventHistory(sessionID string) {
	if b.historyStore != nil {
		if err := b.historyStore.DeleteSession(context.Background(), sessionID); err != nil {
			b.logger.Warn("Failed to clear persisted event history for session %s: %v", sessionID, err)
		}
		return
	}

	b.historyMu.Lock()
	defer b.historyMu.Unlock()

	delete(b.eventHistory, sessionID)
	b.clearHighVolumeCounter(sessionID)
	b.metrics.clearSessionDrops(sessionID)
	b.metrics.clearSessionNoClient(sessionID)
	b.logger.Debug("Cleared event history for session: %s", sessionID)
}

func shouldPersistToHistory(event agentports.AgentEvent) bool {
	event = BaseAgentEvent(event)
	if event == nil {
		return false
	}
	switch evt := event.(type) {
	case *domain.WorkflowEventEnvelope:
		if strings.HasPrefix(evt.Event, "workflow.executor.") {
			return false
		}
		if shouldDropHistoryEnvelope(evt) {
			return false
		}
		return true
	case *domain.Event:
		switch evt.Kind {
		case types.EventInputReceived, types.EventDiagnosticContextSnapshot:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func shouldDropHistoryEnvelope(evt *domain.WorkflowEventEnvelope) bool {
	if evt == nil {
		return false
	}
	switch evt.Event {
	case types.EventNodeOutputDelta, types.EventToolProgress:
		return true
	case types.EventResultFinal:
		return isStreamingHistoryEnvelope(evt.Payload)
	default:
		return false
	}
}

func isStreamingHistoryEnvelope(payload map[string]any) bool {
	if len(payload) == 0 {
		return false
	}
	if isStreaming, ok := payload["is_streaming"].(bool); ok && isStreaming {
		return true
	}
	if finished, ok := payload["stream_finished"].(bool); ok && !finished {
		return true
	}
	return false
}
