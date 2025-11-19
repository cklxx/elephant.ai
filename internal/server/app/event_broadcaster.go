package app

import (
	"context"
	"errors"
	"sync"
	"time"

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
	attachmentExporter AttachmentExporter

	sessionAttachments  map[string]map[string]agentports.Attachment
	sessionAttachmentMu sync.RWMutex
}

const maxExportDuration = 30 * time.Second

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
		maxHistory:         1000, // Keep up to 1000 events per session
		logger:             utils.NewComponentLogger("EventBroadcaster"),
		sessionAttachments: make(map[string]map[string]agentports.Attachment),
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

// SetAttachmentExporter configures the optional CDN/export hook invoked when the
// last client for a session disconnects.
func (b *EventBroadcaster) SetAttachmentExporter(exporter AttachmentExporter) {
	b.attachmentExporter = exporter
}

// OnEvent implements ports.EventListener - broadcasts event to all subscribed clients
func (b *EventBroadcaster) OnEvent(event agentports.AgentEvent) {
	b.logger.Debug("[OnEvent] Received event: type=%s, sessionID=%s", event.EventType(), event.GetSessionID())

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

func (b *EventBroadcaster) archiveAttachments(event agentports.AgentEvent) {
sessionID := event.GetSessionID()
if sessionID == "" {
return
}
attachments := collectEventAttachments(event)
if len(attachments) == 0 {
return
}
b.mergeSessionAttachments(sessionID, attachments)

if b.attachmentArchiver == nil {
return
}
b.attachmentArchiver.Persist(context.Background(), sessionID, event.GetTaskID(), attachments)
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
				go b.handleSessionClosed(sessionID)
			}
			break
		}
	}
}

func (b *EventBroadcaster) handleSessionClosed(sessionID string) {
	if sessionID == "" {
		return
	}
	attachments := b.drainSessionAttachments(sessionID)
	if len(attachments) == 0 {
		return
	}
	result := AttachmentExportResult{
		AttachmentCount: len(attachments),
		Skipped:         true,
		ExporterKind:    "none",
		Error:           errors.New("attachment exporter not configured"),
	}
	if b.attachmentExporter == nil {
		b.logger.Debug("No attachment exporter configured; skipping CDN export for session %s", sessionID)
	} else {
		b.logger.Info("Exporting %d attachments for session %s after last client disconnected", len(attachments), sessionID)
		ctx, cancel := context.WithTimeout(context.Background(), maxExportDuration)
		defer cancel()
		result = b.attachmentExporter.ExportSession(ctx, sessionID, attachments)
		if result.AttachmentCount == 0 {
			result.AttachmentCount = len(attachments)
		}
	}
	if len(result.AttachmentUpdates) > 0 {
		b.applyAttachmentUpdates(sessionID, result.AttachmentUpdates)
	}
	b.emitAttachmentExportEvent(sessionID, b.lookupTaskID(sessionID), result)
}

func (b *EventBroadcaster) applyAttachmentUpdates(sessionID string, updates map[string]agentports.Attachment) {
	if sessionID == "" || len(updates) == 0 {
		return
	}
	clonedUpdates := cloneAttachmentPayload(updates)
	b.historyMu.Lock()
	history := b.eventHistory[sessionID]
	for _, evt := range history {
		switch e := evt.(type) {
		case *domain.UserTaskEvent:
			e.Attachments = mergeAttachmentMaps(e.Attachments, clonedUpdates)
		case *domain.ToolCallCompleteEvent:
			e.Attachments = mergeAttachmentMaps(e.Attachments, clonedUpdates)
		case *domain.TaskCompleteEvent:
			e.Attachments = mergeAttachmentMaps(e.Attachments, clonedUpdates)
		}
	}
	b.historyMu.Unlock()
}

func (b *EventBroadcaster) mergeSessionAttachments(sessionID string, attachments map[string]agentports.Attachment) {
	if sessionID == "" || len(attachments) == 0 {
		return
	}
	b.sessionAttachmentMu.Lock()
	defer b.sessionAttachmentMu.Unlock()
	existing := b.sessionAttachments[sessionID]
	if existing == nil {
		existing = make(map[string]agentports.Attachment, len(attachments))
	}
	for key, att := range attachments {
		existing[key] = att
	}
	b.sessionAttachments[sessionID] = existing
}

func (b *EventBroadcaster) drainSessionAttachments(sessionID string) map[string]agentports.Attachment {
	b.sessionAttachmentMu.Lock()
	defer b.sessionAttachmentMu.Unlock()
	attachments := b.sessionAttachments[sessionID]
	if len(attachments) == 0 {
		delete(b.sessionAttachments, sessionID)
		return nil
	}
	cloned := make(map[string]agentports.Attachment, len(attachments))
	for key, att := range attachments {
		cloned[key] = att
	}
	delete(b.sessionAttachments, sessionID)
	return cloned
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

func (b *EventBroadcaster) lookupTaskID(sessionID string) string {
	b.taskMu.RLock()
	defer b.taskMu.RUnlock()
	return b.sessionToTask[sessionID]
}

func (b *EventBroadcaster) emitAttachmentExportEvent(sessionID, taskID string, result AttachmentExportResult) {
	if sessionID == "" {
		return
	}
	status := AttachmentExportStatusFailed
	switch {
	case result.Skipped:
		status = AttachmentExportStatusSkipped
	case result.Exported:
		status = AttachmentExportStatusSucceeded
	}
	exporterKind := result.ExporterKind
	if exporterKind == "" {
		exporterKind = "custom"
	}
	var errMsg string
	if result.Error != nil {
		errMsg = result.Error.Error()
	}
	event := NewAttachmentExportEvent(
		agentports.LevelCore,
		sessionID,
		taskID,
		status,
		result.AttachmentCount,
		result.Attempts,
		result.Duration,
		exporterKind,
		result.Endpoint,
		errMsg,
		result.AttachmentUpdates,
		time.Now(),
	)
	b.OnEvent(event)
}

// ReportAttachmentScan implements AttachmentScanReporter so malware verdicts get
// surfaced to SSE clients.
func (b *EventBroadcaster) ReportAttachmentScan(sessionID, taskID, placeholder string, attachment agentports.Attachment, result AttachmentScanResult) {
if sessionID == "" {
return
}
event := NewAttachmentScanEvent(
agentports.LevelCore,
sessionID,
taskID,
placeholder,
result.Verdict,
result.Details,
attachment,
time.Now(),
)
b.OnEvent(event)
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
	b.logger.Debug("Stored event in history: sessionID=%s, type=%s, total=%d", sessionID, event.EventType(), len(history))
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
	b.logger.Info("Cleared event history for session: %s", sessionID)
}

func collectEventAttachments(event agentports.AgentEvent) map[string]agentports.Attachment {
	switch e := event.(type) {
	case *domain.UserTaskEvent:
		return cloneAttachmentPayload(e.Attachments)
	case *domain.ToolCallCompleteEvent:
		return cloneAttachmentPayload(e.Attachments)
	case *domain.TaskCompleteEvent:
		return cloneAttachmentPayload(e.Attachments)
	default:
		return nil
	}
}

func cloneAttachmentPayload(values map[string]agentports.Attachment) map[string]agentports.Attachment {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]agentports.Attachment, len(values))
	for key, att := range values {
		cloned[key] = att
	}
	return cloned
}

func mergeAttachmentMaps(base map[string]agentports.Attachment, updates map[string]agentports.Attachment) map[string]agentports.Attachment {
	if len(updates) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]agentports.Attachment, len(updates))
	}
	for key, upd := range updates {
		existing, ok := base[key]
		if !ok {
			base[key] = upd
			continue
		}
		if upd.Name != "" {
			existing.Name = upd.Name
		}
		if upd.MediaType != "" {
			existing.MediaType = upd.MediaType
		}
		if upd.Description != "" {
			existing.Description = upd.Description
		}
		if upd.URI != "" {
			existing.URI = upd.URI
		}
		if upd.Data != "" {
			existing.Data = upd.Data
		}
		if upd.Source != "" {
			existing.Source = upd.Source
		}
		if upd.SizeBytes > 0 {
			existing.SizeBytes = upd.SizeBytes
		}
		if upd.ParentTaskID != "" {
			existing.ParentTaskID = upd.ParentTaskID
		}
		base[key] = existing
	}
	return base
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
