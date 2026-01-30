package app

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"alex/internal/agent/domain"
	core "alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
	"alex/internal/logging"
	id "alex/internal/utils/id"
)

type clientMap map[string][]chan agent.AgentEvent

// EventBroadcaster implements agent.EventListener and broadcasts events to SSE clients
type EventBroadcaster struct {
	// Map sessionID -> list of client channels
	clients   atomic.Value // clientMap
	clientsMu sync.Mutex
	logger    logging.Logger

	highVolumeMu       sync.Mutex
	highVolumeCounters map[string]int

	// Event history for session replay
	eventHistory map[string][]agent.AgentEvent // sessionID -> events
	historyMu    sync.RWMutex
	maxHistory   int // Maximum events to keep per session
	historyStore EventHistoryStore

	// Global events that apply to all sessions (e.g., diagnostics)
	globalHistory []agent.AgentEvent
	globalMu      sync.RWMutex

	// Metrics tracking
	metrics broadcasterMetrics
}

const (
	assistantMessageEventType = types.EventNodeOutputDelta
	assistantMessageLogBatch  = 10
	globalHighVolumeSessionID = "__global__"
)

// broadcasterMetrics tracks broadcaster performance metrics using lock-free
// atomic counters. All fields are updated via Add/Load — no mutex required.
type broadcasterMetrics struct {
	totalEventsSent   atomic.Int64
	droppedEvents     atomic.Int64 // Events dropped due to full buffers
	totalConnections  atomic.Int64 // Total connections ever made
	activeConnections atomic.Int64 // Currently active connections
}

// EventBroadcasterOption configures a broadcaster instance.
type EventBroadcasterOption func(*EventBroadcaster)

// WithEventHistoryStore wires a persistent history store into the broadcaster.
func WithEventHistoryStore(store EventHistoryStore) EventBroadcasterOption {
	return func(b *EventBroadcaster) {
		b.historyStore = store
	}
}

// WithMaxHistory overrides the in-memory history cap.
func WithMaxHistory(max int) EventBroadcasterOption {
	return func(b *EventBroadcaster) {
		if max >= 0 {
			b.maxHistory = max
		}
	}
}

// NewEventBroadcaster creates a new event broadcaster
func NewEventBroadcaster(opts ...EventBroadcasterOption) *EventBroadcaster {
	b := &EventBroadcaster{
		eventHistory:       make(map[string][]agent.AgentEvent),
		highVolumeCounters: make(map[string]int),
		maxHistory:         1000, // Keep up to 1000 events per session
		logger:             logging.NewComponentLogger("EventBroadcaster"),
	}
	b.clients.Store(clientMap{})
	for _, opt := range opts {
		if opt != nil {
			opt(b)
		}
	}
	return b
}

// OnEvent implements agent.EventListener - broadcasts event to all subscribed clients
func (b *EventBroadcaster) OnEvent(event agent.AgentEvent) {
	if event == nil {
		return
	}

	baseEvent := BaseAgentEvent(event)
	if baseEvent == nil {
		return
	}

	if b.shouldSuppressHighVolumeLogs(baseEvent) {
		b.trackHighVolumeEvent(baseEvent)
	}

	// Store event in history for session replay
	sessionID := baseEvent.GetSessionID()
	if shouldPersistToHistory(baseEvent) {
		if sessionID != "" {
			b.storeEventHistory(sessionID, event)
		} else {
			b.storeGlobalEvent(event)
		}
	}

	clientsBySession := b.loadClients()

	if sessionID == "" {
		// Broadcast to all sessions if no session ID
		b.logger.Warn("[OnEvent] No sessionID in event, broadcasting to all %d sessions", len(clientsBySession))
		for sid, clients := range clientsBySession {
			b.broadcastToClients(sid, clients, event)
		}
		return
	}

	clients := clientsBySession[sessionID]
	if len(clients) == 0 {
		b.logger.Warn("[OnEvent] No clients found for sessionID='%s' (event: %s). Available sessions: %v", sessionID, event.EventType(), b.getSessionIDs())
		return
	}

	b.broadcastToClients(sessionID, clients, event)
}

// getSessionIDs returns list of session IDs for debugging
func (b *EventBroadcaster) getSessionIDs() []string {
	clients := b.loadClients()
	ids := make([]string, 0, len(clients))
	for id := range clients {
		ids = append(ids, id)
	}
	return ids
}

// broadcastToClients sends event to all clients in the list
func (b *EventBroadcaster) broadcastToClients(sessionID string, clients []chan agent.AgentEvent, event agent.AgentEvent) {
	for i, ch := range clients {
		select {
		case ch <- event:
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

func (b *EventBroadcaster) ensureCriticalEventDelivery(sessionID string, clientIndex, totalClients int, ch chan agent.AgentEvent, event agent.AgentEvent) bool {
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

func isCriticalEvent(event agent.AgentEvent) bool {
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

// RegisterClient registers a new client for a session
func (b *EventBroadcaster) RegisterClient(sessionID string, ch chan agent.AgentEvent) {
	b.clientsMu.Lock()
	defer b.clientsMu.Unlock()

	current := b.loadClients()
	updated := cloneClientMap(current)
	updated[sessionID] = append(updated[sessionID], ch)
	b.clients.Store(updated)
	b.metrics.incrementConnections()
	b.logger.Info("Client registered for session %s (total: %d)", sessionID, len(updated[sessionID]))
}

// UnregisterClient removes a client from the session
func (b *EventBroadcaster) UnregisterClient(sessionID string, ch chan agent.AgentEvent) {
	b.clientsMu.Lock()
	defer b.clientsMu.Unlock()

	current := b.loadClients()
	clients := current[sessionID]
	for i, client := range clients {
		if client == ch {
			// Remove client from list
			updated := cloneClientMap(current)
			updated[sessionID] = append(clients[:i], clients[i+1:]...)
			b.metrics.decrementConnections()
			b.logger.Info("Client unregistered from session %s (remaining: %d)", sessionID, len(updated[sessionID]))

			// Clean up empty session entries
			if len(updated[sessionID]) == 0 {
				delete(updated, sessionID)
				b.clearHighVolumeCounter(sessionID)
			}
			b.clients.Store(updated)
			break
		}
	}
}

// GetClientCount returns the number of clients subscribed to a session
func (b *EventBroadcaster) GetClientCount(sessionID string) int {
	return len(b.loadClients()[sessionID])
}

// SetSessionContext sets the session context for event extraction
// This is called when a task is started to associate events with a session
func (b *EventBroadcaster) SetSessionContext(ctx context.Context, sessionID string) context.Context {
	// Store sessionID in context using shared key from ports package
	return id.WithSessionID(ctx, sessionID)
}

// storeEventHistory stores an event in the session's history
func (b *EventBroadcaster) storeEventHistory(sessionID string, event agent.AgentEvent) {
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
}

func (b *EventBroadcaster) storeGlobalEvent(event agent.AgentEvent) {
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

func sanitizeEventForHistory(event agent.AgentEvent) agent.AgentEvent {
	if event == nil {
		return nil
	}
	if wrapper, ok := event.(agent.SubtaskWrapper); ok && wrapper != nil {
		if setter, ok := event.(interface{ SetWrappedEvent(agent.AgentEvent) }); ok {
			if sanitized := sanitizeBaseEventForHistory(wrapper.WrappedEvent()); sanitized != nil {
				setter.SetWrappedEvent(sanitized)
			}
		}
		return event
	}

	return sanitizeBaseEventForHistory(event)
}

func sanitizeBaseEventForHistory(event agent.AgentEvent) agent.AgentEvent {
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
	case *domain.WorkflowInputReceivedEvent:
		cloned := *e
		if len(e.Attachments) > 0 {
			if cleaned, ok := stripBinaryPayloadsWithStore(e.Attachments, nil).(map[string]core.Attachment); ok {
				cloned.Attachments = cleaned
			}
		}
		return &cloned
	case *domain.WorkflowDiagnosticContextSnapshotEvent:
		return base
	default:
		return base
	}
}

// StreamHistory streams stored events for a session or global scope.
func (b *EventBroadcaster) StreamHistory(ctx context.Context, filter EventHistoryFilter, fn func(agent.AgentEvent) error) error {
	if b == nil || fn == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if b.historyStore != nil {
		return b.historyStore.Stream(ctx, filter, fn)
	}

	var history []agent.AgentEvent
	if filter.SessionID == "" {
		b.globalMu.RLock()
		history = append(history, b.globalHistory...)
		b.globalMu.RUnlock()
	} else {
		b.historyMu.RLock()
		history = append(history, b.eventHistory[filter.SessionID]...)
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
func (b *EventBroadcaster) GetEventHistory(sessionID string) []agent.AgentEvent {
	var history []agent.AgentEvent
	_ = b.StreamHistory(context.Background(), EventHistoryFilter{SessionID: sessionID}, func(event agent.AgentEvent) error {
		history = append(history, event)
		return nil
	})
	if len(history) == 0 {
		return nil
	}
	return history
}

// GetGlobalHistory returns global events for diagnostics replay.
func (b *EventBroadcaster) GetGlobalHistory() []agent.AgentEvent {
	var history []agent.AgentEvent
	_ = b.StreamHistory(context.Background(), EventHistoryFilter{SessionID: ""}, func(event agent.AgentEvent) error {
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
	b.logger.Info("Cleared event history for session: %s", sessionID)
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

func shouldPersistToHistory(event agent.AgentEvent) bool {
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
	case *domain.WorkflowInputReceivedEvent, *domain.WorkflowDiagnosticContextSnapshotEvent:
		return true
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

// shouldSuppressHighVolumeLogs determines whether verbose logs should be
// suppressed for the provided event type. Assistant streaming events are very
// high volume and can flood the logs, so we only log these events in batches.
func (b *EventBroadcaster) shouldSuppressHighVolumeLogs(event agent.AgentEvent) bool {
	base := BaseAgentEvent(event)
	if base == nil {
		return false
	}
	return base.EventType() == assistantMessageEventType
}

func (b *EventBroadcaster) trackHighVolumeEvent(event agent.AgentEvent) {
	base := BaseAgentEvent(event)
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
		b.logger.Debug("[HighVolumeLogs] Processed %d '%s' events for session=%s", count, base.EventType(), sessionID)
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

// Metrics helper methods — lock-free via atomic.Int64.
func (m *broadcasterMetrics) incrementEventsSent()   { m.totalEventsSent.Add(1) }
func (m *broadcasterMetrics) incrementDroppedEvents() { m.droppedEvents.Add(1) }
func (m *broadcasterMetrics) incrementConnections() {
	m.totalConnections.Add(1)
	m.activeConnections.Add(1)
}
func (m *broadcasterMetrics) decrementConnections() { m.activeConnections.Add(-1) }

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
	totalEvents := b.metrics.totalEventsSent.Load()
	droppedEvents := b.metrics.droppedEvents.Load()
	totalConns := b.metrics.totalConnections.Load()
	activeConns := b.metrics.activeConnections.Load()

	clientsBySession := b.loadClients()

	// Calculate buffer depth per session
	bufferDepth := make(map[string]int)
	for sessionID, clients := range clientsBySession {
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
		SessionCount:      len(clientsBySession),
	}
}

func (b *EventBroadcaster) loadClients() clientMap {
	if b == nil {
		return nil
	}
	if value := b.clients.Load(); value != nil {
		return value.(clientMap)
	}
	return clientMap{}
}

func cloneClientMap(src clientMap) clientMap {
	if len(src) == 0 {
		return clientMap{}
	}
	out := make(clientMap, len(src))
	for sessionID, clients := range src {
		out[sessionID] = append([]chan agent.AgentEvent(nil), clients...)
	}
	return out
}
