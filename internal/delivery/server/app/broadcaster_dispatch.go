package app

import (
	"time"

	domain "alex/internal/domain/agent"
	agentports "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

// OnEvent implements agent.EventListener - broadcasts event to all subscribed clients
func (b *EventBroadcaster) OnEvent(event agentports.AgentEvent) {
	if event == nil {
		return
	}

	baseEvent := BaseAgentEvent(event)
	if baseEvent == nil {
		return
	}

	if b.isHighVolumeEvent(baseEvent) {
		b.trackHighVolumeEvent(baseEvent)
	}

	// Store event in history for session replay
	sessionID := baseEvent.GetSessionID()
	isGlobal := isGlobalSessionID(sessionID)
	if shouldPersistToHistory(baseEvent) {
		if sessionID != "" && !isGlobal {
			b.storeEventHistory(sessionID, event)
		} else {
			b.storeGlobalEvent(event)
		}
	}

	clientsBySession := b.loadClients()

	if sessionID == "" {
		count := b.metrics.incrementNoClientEvents(missingSessionIDKey)
		if shouldSampleCounter(count) {
			b.logger.Warn("No sessionID in event, dropping event %s (count=%d)", event.EventType(), count)
		}
		return
	}

	if isGlobal {
		for sid, clients := range clientsBySession {
			b.broadcastToClients(sid, clients, event)
		}
		return
	}

	clients := clientsBySession[sessionID]
	if len(clients) == 0 {
		count := b.metrics.incrementNoClientEvents(sessionID)
		if shouldSampleCounter(count) {
			b.logger.Warn("No clients found for session %s (event=%s count=%d)", sessionID, event.EventType(), count)
		}
		return
	}

	b.broadcastToClients(sessionID, clients, event)
}

func isGlobalSessionID(sessionID string) bool {
	return sessionID == globalHighVolumeSessionID
}

// broadcastToClients sends event to all clients in the list
func (b *EventBroadcaster) broadcastToClients(sessionID string, clients []chan agentports.AgentEvent, event agentports.AgentEvent) {
	for i, ch := range clients {
		select {
		case ch <- event:
			b.metrics.incrementEventsSent()
		default:
			if b.ensureCriticalEventDelivery(sessionID, i, len(clients), ch, event) {
				continue
			}
			// Client buffer full, skip this event to avoid blocking
			b.logger.Warn("Client buffer full for session %s, dropping event %s (client %d/%d)", sessionID, event.EventType(), i+1, len(clients))
			dropCount := b.metrics.incrementDroppedEvents(sessionID)

			// Notify the client about the drop so the frontend can display a gap indicator.
			// We only send this notification periodically (powers of 2) to avoid flooding
			// an already-saturated channel.
			if dropCount&(dropCount-1) == 0 { // power of 2: 1, 2, 4, 8, ...
				droppedNotice := newStreamDroppedEnvelope(sessionID, event.EventType(), dropCount)
				select {
				case ch <- droppedNotice:
					b.metrics.incrementEventsSent()
				default:
					// Channel still full — don't block on the notification itself.
				}
			}
		}
	}
}

// newStreamDroppedEnvelope creates a synthetic event notifying the client that
// events were dropped due to buffer saturation.
func newStreamDroppedEnvelope(sessionID, droppedEventType string, totalDrops int64) *domain.WorkflowEventEnvelope {
	return &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agentports.LevelCore, sessionID, "", "", time.Now()),
		Version:   1,
		Event:     types.EventStreamDropped,
		NodeKind:  "system",
		Payload: map[string]any{
			"dropped_event_type": droppedEventType,
			"total_drops":        totalDrops,
		},
	}
}

func (b *EventBroadcaster) ensureCriticalEventDelivery(sessionID string, clientIndex, totalClients int, ch chan agentports.AgentEvent, event agentports.AgentEvent) bool {
	if !isCriticalEvent(event) {
		return false
	}

	// First, retry in case the consumer drained the buffer after the initial attempt.
	select {
	case ch <- event:
		b.logger.Warn("Client buffer previously full for session %s, but critical event %s was delivered on retry (client %d/%d)", sessionID, event.EventType(), clientIndex+1, totalClients)
		b.metrics.incrementEventsSent()
		return true
	default:
	}

	// Drop the oldest event to make room for the critical one.
	select {
	case <-ch:
	default:
		// Buffer no longer full but send still failed; treat as delivered failure.
		b.logger.Warn("Failed to free space for critical event %s for session %s (client %d/%d)", event.EventType(), sessionID, clientIndex+1, totalClients)
		return false
	}

	select {
	case ch <- event:
		b.logger.Warn("Client buffer saturated for session %s; dropped oldest event to deliver critical %s (client %d/%d)", sessionID, event.EventType(), clientIndex+1, totalClients)
		b.metrics.incrementEventsSent()
		return true
	default:
		// Should be rare – buffer filled again before we could send.
		b.logger.Warn("Client buffer refilled before delivering critical %s for session %s (client %d/%d)", event.EventType(), sessionID, clientIndex+1, totalClients)
		return false
	}
}

func isCriticalEvent(event agentports.AgentEvent) bool {
	if event == nil {
		return false
	}
	switch event.EventType() {
	case types.EventResultFinal, types.EventResultCancelled:
		return true
	default:
		return false
	}
}

// isHighVolumeEvent reports whether the event type is high-volume (e.g.
// assistant streaming deltas) and should be tracked in batches to avoid
// flooding the logs.
func (b *EventBroadcaster) isHighVolumeEvent(base agentports.AgentEvent) bool {
	return base != nil && base.EventType() == assistantMessageEventType
}

func (b *EventBroadcaster) trackHighVolumeEvent(base agentports.AgentEvent) {
	if base == nil {
		return
	}
	sessionID := base.GetSessionID()
	if sessionID == "" {
		sessionID = globalHighVolumeSessionID
	}

	b.highVolumeMu.Lock()
	b.highVolumeCounters[sessionID]++
	count := b.highVolumeCounters[sessionID]
	b.highVolumeMu.Unlock()

	if count%assistantMessageLogBatch == 0 {
		b.logger.Debug("Processed %d high-volume events of type %s for session=%s", count, base.EventType(), sessionID)
	}
}

func (b *EventBroadcaster) clearHighVolumeCounter(sessionID string) {
	if sessionID == "" {
		sessionID = globalHighVolumeSessionID
	}

	b.highVolumeMu.Lock()
	delete(b.highVolumeCounters, sessionID)
	b.highVolumeMu.Unlock()
}

func intFromPayload(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	switch val := payload[key].(type) {
	case int:
		return val
	case int32:
		return int(val)
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}
