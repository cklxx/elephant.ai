package app

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"
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
	eventHistory map[string]*sessionHistory // sessionID -> events
	historyMu    sync.RWMutex
	maxHistory   int // Maximum events to keep per session
	maxSessions  int
	sessionTTL   time.Duration
	historyStore EventHistoryStore
	pruneCounter atomic.Int64

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
	missingSessionIDKey       = "__missing__"
	pruneEveryN               = 100
	sessionMetricMaxEntries   = 2048
	sessionMetricTTL          = 30 * time.Minute
	sessionMetricPruneEveryN  = 256
)

type sessionHistory struct {
	events   []agent.AgentEvent
	lastSeen time.Time
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

// WithMaxSessions caps the number of sessions retained in memory (0 = unlimited).
func WithMaxSessions(max int) EventBroadcasterOption {
	return func(b *EventBroadcaster) {
		if max >= 0 {
			b.maxSessions = max
		}
	}
}

// WithSessionTTL configures how long to retain idle session history (0 = disabled).
func WithSessionTTL(ttl time.Duration) EventBroadcasterOption {
	return func(b *EventBroadcaster) {
		if ttl >= 0 {
			b.sessionTTL = ttl
		}
	}
}

const (
	defaultMaxHistoryWithStore    = 1000
	defaultMaxHistoryWithoutStore = 200
)

// NewEventBroadcaster creates a new event broadcaster
func NewEventBroadcaster(opts ...EventBroadcasterOption) *EventBroadcaster {
	b := &EventBroadcaster{
		eventHistory:       make(map[string]*sessionHistory),
		highVolumeCounters: make(map[string]int),
		maxHistory:         defaultMaxHistoryWithStore,
		logger:             logging.NewComponentLogger("EventBroadcaster"),
		metrics:            newBroadcasterMetrics(),
	}
	b.clients.Store(clientMap{})
	for _, opt := range opts {
		if opt != nil {
			opt(b)
		}
	}
	// When no persistent history store is configured (dev/fallback mode),
	// all events accumulate in memory. Cap more aggressively to prevent
	// memory explosion during long agent runs.
	if b.historyStore == nil && b.maxHistory > defaultMaxHistoryWithoutStore {
		b.maxHistory = defaultMaxHistoryWithoutStore
		b.logger.Warn("No event history store configured; capping in-memory history to %d events per session", b.maxHistory)
	}
	return b
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
	b.logger.Debug("Client registered for session %s (total: %d)", sessionID, len(updated[sessionID]))
}

// UnregisterClient removes a client from the session
func (b *EventBroadcaster) UnregisterClient(sessionID string, ch chan agent.AgentEvent) {
	b.clientsMu.Lock()
	defer b.clientsMu.Unlock()

	current := b.loadClients()
	clients := current[sessionID]
	for i, client := range clients {
		if client == ch {
			// Clone first, then operate on the cloned slice to avoid
			// mutating the original slice's backing array.
			updated := cloneClientMap(current)
			clonedClients := updated[sessionID]
			updated[sessionID] = append(clonedClients[:i], clonedClients[i+1:]...)
			b.metrics.decrementConnections()
			b.logger.Debug("Client unregistered from session %s (remaining: %d)", sessionID, len(updated[sessionID]))

			// Clean up empty session entries
			if len(updated[sessionID]) == 0 {
				delete(updated, sessionID)
				b.clearHighVolumeCounter(sessionID)
				b.metrics.clearSessionDrops(sessionID)
				b.metrics.clearSessionNoClient(sessionID)
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

func (b *EventBroadcaster) pruneExpiredSessionsLocked(now time.Time) {
	if b.sessionTTL <= 0 || len(b.eventHistory) == 0 {
		return
	}
	for sessionID, history := range b.eventHistory {
		if history == nil {
			delete(b.eventHistory, sessionID)
			continue
		}
		if now.Sub(history.lastSeen) > b.sessionTTL {
			delete(b.eventHistory, sessionID)
			b.clearHighVolumeCounter(sessionID)
			b.metrics.clearSessionDrops(sessionID)
			b.metrics.clearSessionNoClient(sessionID)
		}
	}
}

func (b *EventBroadcaster) enforceMaxSessionsLocked() {
	if b.maxSessions <= 0 || len(b.eventHistory) <= b.maxSessions {
		return
	}

	type sessionInfo struct {
		id       string
		lastSeen time.Time
	}
	sessions := make([]sessionInfo, 0, len(b.eventHistory))
	for sessionID, history := range b.eventHistory {
		lastSeen := time.Time{}
		if history != nil {
			lastSeen = history.lastSeen
		}
		sessions = append(sessions, sessionInfo{id: sessionID, lastSeen: lastSeen})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].lastSeen.Before(sessions[j].lastSeen)
	})

	toEvict := len(sessions) - b.maxSessions
	for i := 0; i < toEvict; i++ {
		sessionID := sessions[i].id
		delete(b.eventHistory, sessionID)
		b.clearHighVolumeCounter(sessionID)
		b.metrics.clearSessionDrops(sessionID)
		b.metrics.clearSessionNoClient(sessionID)
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
