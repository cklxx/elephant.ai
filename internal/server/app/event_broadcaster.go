package app

import (
	"context"
	"sync"

	"alex/internal/agent/domain"
	agentports "alex/internal/agent/ports"
	serverports "alex/internal/server/ports"
	"alex/internal/utils"
)

// EventBroadcaster implements ports.EventListener and broadcasts events to SSE clients
type EventBroadcaster struct {
	// Map sessionID -> list of client channels
	clients map[string][]chan agentports.AgentEvent
	mu      sync.RWMutex
	logger  *utils.Logger

	// Task progress tracking
	taskStore     serverports.TaskStore
	sessionToTask map[string]string // sessionID -> taskID mapping
	taskMu        sync.RWMutex      // separate mutex for task tracking

	// Metrics tracking
	metrics broadcasterMetrics
}

// broadcasterMetrics tracks broadcaster performance metrics
type broadcasterMetrics struct {
	mu sync.RWMutex

	totalEventsSent   int64
	droppedEvents     int64 // Events dropped due to full buffers
	totalConnections  int64 // Total connections ever made
	activeConnections int64 // Currently active connections
}

// NewEventBroadcaster creates a new event broadcaster
func NewEventBroadcaster() *EventBroadcaster {
	return &EventBroadcaster{
		clients:       make(map[string][]chan agentports.AgentEvent),
		sessionToTask: make(map[string]string),
		logger:        utils.NewComponentLogger("EventBroadcaster"),
	}
}

// SetTaskStore sets the task store for progress tracking
func (b *EventBroadcaster) SetTaskStore(store serverports.TaskStore) {
	b.taskStore = store
}

// OnEvent implements ports.EventListener - broadcasts event to all subscribed clients
func (b *EventBroadcaster) OnEvent(event agentports.AgentEvent) {
	b.logger.Debug("[OnEvent] Received event: type=%s, sessionID=%s", event.EventType(), event.GetSessionID())

	// Update task progress before broadcasting
	b.updateTaskProgress(event)

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Extract sessionID from event context
	// For now, we'll broadcast to all clients - in production, you'd extract sessionID
	// from the event's context or metadata
	sessionID := b.extractSessionID(event)

	b.logger.Debug("[OnEvent] SessionID extracted: '%s', total clients map size: %d", sessionID, len(b.clients))

	if sessionID == "" {
		// Broadcast to all sessions if no session ID
		b.logger.Warn("[OnEvent] No sessionID in event, broadcasting to all %d sessions", len(b.clients))
		for sid, clients := range b.clients {
			b.logger.Debug("[OnEvent] Broadcasting to session '%s' with %d clients", sid, len(clients))
			b.broadcastToClients(sid, clients, event)
		}
		return
	}

	// Broadcast to specific session's clients
	if clients, ok := b.clients[sessionID]; ok {
		b.logger.Debug("[OnEvent] Found %d clients for session '%s', broadcasting event type: %s", len(clients), sessionID, event.EventType())
		b.broadcastToClients(sessionID, clients, event)
	} else {
		b.logger.Warn("[OnEvent] No clients found for sessionID='%s' (event: %s). Available sessions: %v", sessionID, event.EventType(), b.getSessionIDs())
	}
}

// getSessionIDs returns list of session IDs for debugging
func (b *EventBroadcaster) getSessionIDs() []string {
	ids := make([]string, 0, len(b.clients))
	for id := range b.clients {
		ids = append(ids, id)
	}
	return ids
}

// updateTaskProgress updates task progress based on event type
func (b *EventBroadcaster) updateTaskProgress(event agentports.AgentEvent) {
	if b.taskStore == nil {
		return
	}

	sessionID := event.GetSessionID()
	if sessionID == "" {
		return
	}

	// Get taskID for this session
	b.taskMu.RLock()
	taskID, ok := b.sessionToTask[sessionID]
	b.taskMu.RUnlock()

	if !ok {
		return
	}

	ctx := context.Background()

	// Update progress based on event type
	switch e := event.(type) {
	case *domain.IterationStartEvent:
		// Update current iteration only, preserve tokens
		task, err := b.taskStore.Get(ctx, taskID)
		if err == nil {
			_ = b.taskStore.UpdateProgress(ctx, taskID, e.Iteration, task.TokensUsed)
		}

	case *domain.IterationCompleteEvent:
		// Update current iteration and tokens
		_ = b.taskStore.UpdateProgress(ctx, taskID, e.Iteration, e.TokensUsed)

	case *domain.TaskCompleteEvent:
		// Final update is handled by SetResult, but we can update one more time
		_ = b.taskStore.UpdateProgress(ctx, taskID, e.TotalIterations, e.TotalTokens)
	}
}

// broadcastToClients sends event to all clients in the list
func (b *EventBroadcaster) broadcastToClients(sessionID string, clients []chan agentports.AgentEvent, event agentports.AgentEvent) {
	b.logger.Debug("[broadcastToClients] Sending event type=%s to %d clients for session=%s", event.EventType(), len(clients), sessionID)

	for i, ch := range clients {
		select {
		case ch <- event:
			// Event sent successfully
			b.logger.Debug("[broadcastToClients] Event sent successfully to client %d/%d for session=%s", i+1, len(clients), sessionID)
			b.metrics.incrementEventsSent()
		default:
			// Client buffer full, skip this event to avoid blocking
			b.logger.Warn("Client buffer full for session %s, dropping event (client %d/%d)", sessionID, i+1, len(clients))
			b.metrics.incrementDroppedEvents()
		}
	}
}

// extractSessionID extracts session ID from event
func (b *EventBroadcaster) extractSessionID(event agentports.AgentEvent) string {
	// Events now carry session ID directly
	return event.GetSessionID()
}

// RegisterClient registers a new client for a session
func (b *EventBroadcaster) RegisterClient(sessionID string, ch chan agentports.AgentEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.clients[sessionID] = append(b.clients[sessionID], ch)
	b.metrics.incrementConnections()
	b.logger.Info("Client registered for session %s (total: %d)", sessionID, len(b.clients[sessionID]))
}

// UnregisterClient removes a client from the session
func (b *EventBroadcaster) UnregisterClient(sessionID string, ch chan agentports.AgentEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	clients := b.clients[sessionID]
	for i, client := range clients {
		if client == ch {
			// Remove client from list
			b.clients[sessionID] = append(clients[:i], clients[i+1:]...)
			close(ch)
			b.metrics.decrementConnections()
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
	// Store sessionID in context using shared key from ports package
	return context.WithValue(ctx, agentports.SessionContextKey{}, sessionID)
}

// RegisterTaskSession associates a taskID with a sessionID for progress tracking
func (b *EventBroadcaster) RegisterTaskSession(sessionID, taskID string) {
	b.taskMu.Lock()
	defer b.taskMu.Unlock()

	b.sessionToTask[sessionID] = taskID
	b.logger.Info("Registered task-session mapping: sessionID=%s, taskID=%s", sessionID, taskID)
}

// UnregisterTaskSession removes the taskID-sessionID mapping
func (b *EventBroadcaster) UnregisterTaskSession(sessionID string) {
	b.taskMu.Lock()
	defer b.taskMu.Unlock()

	delete(b.sessionToTask, sessionID)
	b.logger.Info("Unregistered task-session mapping: sessionID=%s", sessionID)
}

// Metrics helper methods
func (m *broadcasterMetrics) incrementEventsSent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalEventsSent++
}

func (m *broadcasterMetrics) incrementDroppedEvents() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.droppedEvents++
}

func (m *broadcasterMetrics) incrementConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalConnections++
	m.activeConnections++
}

func (m *broadcasterMetrics) decrementConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeConnections--
}

// BroadcasterMetrics represents broadcaster metrics for export
type BroadcasterMetrics struct {
	TotalEventsSent   int64            `json:"total_events_sent"`
	DroppedEvents     int64            `json:"dropped_events"`
	TotalConnections  int64            `json:"total_connections"`
	ActiveConnections int64            `json:"active_connections"`
	BufferDepth       map[string]int   `json:"buffer_depth"` // Per-session buffer depth
	SessionCount      int              `json:"session_count"`
}

// GetMetrics returns current broadcaster metrics
func (b *EventBroadcaster) GetMetrics() BroadcasterMetrics {
	b.metrics.mu.RLock()
	totalEvents := b.metrics.totalEventsSent
	droppedEvents := b.metrics.droppedEvents
	totalConns := b.metrics.totalConnections
	activeConns := b.metrics.activeConnections
	b.metrics.mu.RUnlock()

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Calculate buffer depth per session
	bufferDepth := make(map[string]int)
	for sessionID, clients := range b.clients {
		totalDepth := 0
		for _, ch := range clients {
			totalDepth += len(ch)
		}
		if totalDepth > 0 {
			bufferDepth[sessionID] = totalDepth
		}
	}

	return BroadcasterMetrics{
		TotalEventsSent:   totalEvents,
		DroppedEvents:     droppedEvents,
		TotalConnections:  totalConns,
		ActiveConnections: activeConns,
		BufferDepth:       bufferDepth,
		SessionCount:      len(b.clients),
	}
}
