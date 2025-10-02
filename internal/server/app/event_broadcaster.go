package app

import (
	"context"
	"sync"

	"alex/internal/agent/domain"
	"alex/internal/utils"
)

// EventBroadcaster implements domain.EventListener and broadcasts events to SSE clients
type EventBroadcaster struct {
	// Map sessionID -> list of client channels
	clients map[string][]chan domain.AgentEvent
	mu      sync.RWMutex
	logger  *utils.Logger
}

// NewEventBroadcaster creates a new event broadcaster
func NewEventBroadcaster() *EventBroadcaster {
	return &EventBroadcaster{
		clients: make(map[string][]chan domain.AgentEvent),
		logger:  utils.NewComponentLogger("EventBroadcaster"),
	}
}

// OnEvent implements domain.EventListener - broadcasts event to all subscribed clients
func (b *EventBroadcaster) OnEvent(event domain.AgentEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Extract sessionID from event context
	// For now, we'll broadcast to all clients - in production, you'd extract sessionID
	// from the event's context or metadata
	sessionID := b.extractSessionID(event)

	if sessionID == "" {
		// Broadcast to all sessions if no session ID
		for sid, clients := range b.clients {
			b.broadcastToClients(sid, clients, event)
		}
		return
	}

	// Broadcast to specific session's clients
	if clients, ok := b.clients[sessionID]; ok {
		b.broadcastToClients(sessionID, clients, event)
	}
}

// broadcastToClients sends event to all clients in the list
func (b *EventBroadcaster) broadcastToClients(sessionID string, clients []chan domain.AgentEvent, event domain.AgentEvent) {
	for _, ch := range clients {
		select {
		case ch <- event:
			// Event sent successfully
		default:
			// Client buffer full, skip this event to avoid blocking
			b.logger.Warn("Client buffer full for session %s, dropping event", sessionID)
		}
	}
}

// extractSessionID extracts session ID from event context
// TODO: Implement proper context extraction when events carry context
func (b *EventBroadcaster) extractSessionID(event domain.AgentEvent) string {
	// For now, return empty string to broadcast to all
	// In production, you might have a SessionContext attached to events
	return ""
}

// RegisterClient registers a new client for a session
func (b *EventBroadcaster) RegisterClient(sessionID string, ch chan domain.AgentEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.clients[sessionID] = append(b.clients[sessionID], ch)
	b.logger.Info("Client registered for session %s (total: %d)", sessionID, len(b.clients[sessionID]))
}

// UnregisterClient removes a client from the session
func (b *EventBroadcaster) UnregisterClient(sessionID string, ch chan domain.AgentEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	clients := b.clients[sessionID]
	for i, client := range clients {
		if client == ch {
			// Remove client from list
			b.clients[sessionID] = append(clients[:i], clients[i+1:]...)
			close(ch)
			b.logger.Info("Client unregistered from session %s (remaining: %d)", sessionID, len(b.clients[sessionID]))

			// Clean up empty session entries
			if len(b.clients[sessionID]) == 0 {
				delete(b.clients, sessionID)
			}
			break
		}
	}
}

// GetClientCount returns the number of clients subscribed to a session
func (b *EventBroadcaster) GetClientCount(sessionID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.clients[sessionID])
}

// SetSessionContext sets the session context for event extraction
// This is called when a task is started to associate events with a session
func (b *EventBroadcaster) SetSessionContext(ctx context.Context, sessionID string) context.Context {
	// Store sessionID in context for later extraction
	return context.WithValue(ctx, sessionContextKey{}, sessionID)
}

type sessionContextKey struct{}
