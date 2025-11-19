package app

import (
	"context"
	"sync"

	"alex/internal/agent/domain"
	agentports "alex/internal/agent/ports"
	serverports "alex/internal/server/ports"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

// EventBroadcaster implements ports.EventListener and broadcasts events to SSE clients
type EventBroadcaster struct {
	// Map sessionID -> list of client channels
	clients map[string][]chan agentports.AgentEvent
	mu      sync.RWMutex
	logger  *utils.Logger

	highVolumeMu       sync.Mutex
	highVolumeCounters map[string]int

	// Task progress tracking
	taskStore     serverports.TaskStore
	sessionToTask map[string]string // sessionID -> taskID mapping
	taskMu        sync.RWMutex      // separate mutex for task tracking

	// Event history for session replay
	eventHistory map[string][]agentports.AgentEvent // sessionID -> events
	historyMu    sync.RWMutex
	maxHistory   int // Maximum events to keep per session

	// Global events that apply to all sessions (e.g., diagnostics)
	globalHistory []agentports.AgentEvent
	globalMu      sync.RWMutex

	// Metrics tracking
	metrics broadcasterMetrics

	attachmentArchiver AttachmentArchiver
}

const (
	assistantMessageEventType = "assistant_message"
	assistantMessageLogBatch  = 10
)

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
		clients:            make(map[string][]chan agentports.AgentEvent),
		sessionToTask:      make(map[string]string),
		eventHistory:       make(map[string][]agentports.AgentEvent),
		highVolumeCounters: make(map[string]int),
		maxHistory:         1000, // Keep up to 1000 events per session
		logger:             utils.NewComponentLogger("EventBroadcaster"),
	}
}

// SetTaskStore sets the task store for progress tracking
func (b *EventBroadcaster) SetTaskStore(store serverports.TaskStore) {
	b.taskStore = store
}

// SetAttachmentArchiver configures optional sandbox persistence for generated assets.
func (b *EventBroadcaster) SetAttachmentArchiver(archiver AttachmentArchiver) {
	b.attachmentArchiver = archiver
}

// OnEvent implements ports.EventListener - broadcasts event to all subscribed clients
func (b *EventBroadcaster) OnEvent(event agentports.AgentEvent) {
	suppressLogs := b.shouldSuppressHighVolumeLogs(event)
	if suppressLogs {
		b.trackHighVolumeEvent(event)
	} else {
		b.logger.Debug("[OnEvent] Received event: type=%s, sessionID=%s", event.EventType(), event.GetSessionID())
	}

	// Store event in history for session replay
	sessionID := event.GetSessionID()
	if sessionID != "" {
		b.storeEventHistory(sessionID, event)
	} else {
		b.storeGlobalEvent(event)
	}

	b.archiveAttachments(event)

	// Update task progress before broadcasting
	b.updateTaskProgress(event)

	b.mu.RLock()
	defer b.mu.RUnlock()

	if !suppressLogs {
		b.logger.Debug("[OnEvent] SessionID extracted: '%s', total clients map size: %d", sessionID, len(b.clients))
	}

	if sessionID == "" {
		// Broadcast to all sessions if no session ID
		b.logger.Warn("[OnEvent] No sessionID in event, broadcasting to all %d sessions", len(b.clients))
		for sid, clients := range b.clients {
			if !suppressLogs {
				b.logger.Debug("[OnEvent] Broadcasting to session '%s' with %d clients", sid, len(clients))
			}
			b.broadcastToClients(sid, clients, event)
		}
		return
	}

	// Broadcast to specific session's clients
	if clients, ok := b.clients[sessionID]; ok {
		if !suppressLogs {
			b.logger.Debug("[OnEvent] Found %d clients for session '%s', broadcasting event type: %s", len(clients), sessionID, event.EventType())
		}
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

	ctx := id.WithSessionID(context.Background(), sessionID)
	ctx = id.WithTaskID(ctx, taskID)
	if !b.shouldSuppressHighVolumeLogs(event) {
		b.logger.Debug("[updateTaskProgress] Tracking event type=%s for session=%s task=%s", event.EventType(), sessionID, taskID)
	}

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

func (b *EventBroadcaster) archiveAttachments(event agentports.AgentEvent) {
	if b.attachmentArchiver == nil {
		return
	}
	sessionID := event.GetSessionID()
	if sessionID == "" {
		return
	}
	attachments := collectEventAttachments(event)
	if len(attachments) == 0 {
		return
	}
	b.attachmentArchiver.Persist(context.Background(), sessionID, attachments)
}

// broadcastToClients sends event to all clients in the list
func (b *EventBroadcaster) broadcastToClients(sessionID string, clients []chan agentports.AgentEvent, event agentports.AgentEvent) {
	suppressLogs := b.shouldSuppressHighVolumeLogs(event)
	if !suppressLogs {
		b.logger.Debug("[broadcastToClients] Sending event type=%s to %d clients for session=%s", event.EventType(), len(clients), sessionID)
	}

	for i, ch := range clients {
		select {
		case ch <- event:
			// Event sent successfully
			if !suppressLogs {
				b.logger.Debug("[broadcastToClients] Event sent successfully to client %d/%d for session=%s", i+1, len(clients), sessionID)
			}
			b.metrics.incrementEventsSent()
		default:
			if b.ensureCriticalEventDelivery(sessionID, i, len(clients), ch, event) {
				continue
			}
			// Client buffer full, skip this event to avoid blocking
			b.logger.Warn("Client buffer full for session %s, dropping event (client %d/%d)", sessionID, i+1, len(clients))
			b.metrics.incrementDroppedEvents()
		}
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
	case "task_complete", "task_cancelled":
		return true
	default:
		return false
	}
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
				b.clearHighVolumeCounter(sessionID)
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
	return id.WithSessionID(ctx, sessionID)
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

// storeEventHistory stores an event in the session's history
func (b *EventBroadcaster) storeEventHistory(sessionID string, event agentports.AgentEvent) {
	b.historyMu.Lock()
	defer b.historyMu.Unlock()

	history := b.eventHistory[sessionID]
	history = append(history, event)

	// Trim history if it exceeds max size
	if len(history) > b.maxHistory {
		// Keep only the most recent maxHistory events
		history = history[len(history)-b.maxHistory:]
	}

	b.eventHistory[sessionID] = history
	if !b.shouldSuppressHighVolumeLogs(event) {
		b.logger.Debug("Stored event in history: sessionID=%s, type=%s, total=%d", sessionID, event.EventType(), len(history))
	}
}

func (b *EventBroadcaster) storeGlobalEvent(event agentports.AgentEvent) {
	b.globalMu.Lock()
	defer b.globalMu.Unlock()

	b.globalHistory = append(b.globalHistory, event)
	if len(b.globalHistory) > b.maxHistory {
		b.globalHistory = b.globalHistory[len(b.globalHistory)-b.maxHistory:]
	}
}

// GetEventHistory returns all stored events for a session
func (b *EventBroadcaster) GetEventHistory(sessionID string) []agentports.AgentEvent {
	b.historyMu.RLock()
	defer b.historyMu.RUnlock()

	history := b.eventHistory[sessionID]
	if len(history) == 0 {
		return nil
	}

	// Return a copy to prevent concurrent modification
	historyCopy := make([]agentports.AgentEvent, len(history))
	copy(historyCopy, history)
	return historyCopy
}

// GetGlobalHistory returns global events for diagnostics replay.
func (b *EventBroadcaster) GetGlobalHistory() []agentports.AgentEvent {
	b.globalMu.RLock()
	defer b.globalMu.RUnlock()

	if len(b.globalHistory) == 0 {
		return nil
	}
	historyCopy := make([]agentports.AgentEvent, len(b.globalHistory))
	copy(historyCopy, b.globalHistory)
	return historyCopy
}

// ClearEventHistory clears the event history for a session
func (b *EventBroadcaster) ClearEventHistory(sessionID string) {
	b.historyMu.Lock()
	defer b.historyMu.Unlock()

	delete(b.eventHistory, sessionID)
	b.clearHighVolumeCounter(sessionID)
	b.logger.Info("Cleared event history for session: %s", sessionID)
}

func collectEventAttachments(event agentports.AgentEvent) map[string]agentports.Attachment {
	switch e := event.(type) {
	case *domain.ToolCallCompleteEvent:
		return agentports.CloneAttachmentMap(e.Attachments)
	case *domain.TaskCompleteEvent:
		return agentports.CloneAttachmentMap(e.Attachments)
	default:
		return nil
	}
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
	TotalEventsSent   int64          `json:"total_events_sent"`
	DroppedEvents     int64          `json:"dropped_events"`
	TotalConnections  int64          `json:"total_connections"`
	ActiveConnections int64          `json:"active_connections"`
	BufferDepth       map[string]int `json:"buffer_depth"` // Per-session buffer depth
	SessionCount      int            `json:"session_count"`
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
