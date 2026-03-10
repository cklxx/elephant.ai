package app

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// broadcasterMetrics tracks broadcaster performance metrics using lock-free
// atomic counters. All fields are updated via Add/Load — no mutex required.
type broadcasterMetrics struct {
	totalEventsSent   atomic.Int64
	droppedEvents     atomic.Int64 // Events dropped due to full buffers
	noClientEvents    atomic.Int64 // Events dropped because no SSE client was subscribed
	totalConnections  atomic.Int64 // Total connections ever made
	activeConnections atomic.Int64 // Currently active connections
	dropsPerSession   *boundedSessionCounterStore
	noClientBySession *boundedSessionCounterStore
}

type sessionCounterEntry struct {
	count    int64
	lastSeen time.Time
}

type boundedSessionCounterStore struct {
	mu         sync.RWMutex
	entries    map[string]*sessionCounterEntry
	maxEntries int
	ttl        time.Duration
	ops        uint64
}

func newBroadcasterMetrics() broadcasterMetrics {
	return broadcasterMetrics{
		dropsPerSession:   newBoundedSessionCounterStore(sessionMetricMaxEntries, sessionMetricTTL),
		noClientBySession: newBoundedSessionCounterStore(sessionMetricMaxEntries, sessionMetricTTL),
	}
}

func newBoundedSessionCounterStore(maxEntries int, ttl time.Duration) *boundedSessionCounterStore {
	return &boundedSessionCounterStore{
		entries:    make(map[string]*sessionCounterEntry),
		maxEntries: maxEntries,
		ttl:        ttl,
	}
}

func (s *boundedSessionCounterStore) Increment(sessionID string) int64 {
	if s == nil {
		return 0
	}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if sessionID == "" {
		sessionID = missingSessionIDKey
	}

	entry := s.entries[sessionID]
	if entry == nil {
		entry = &sessionCounterEntry{}
		s.entries[sessionID] = entry
	}
	entry.count++
	entry.lastSeen = now

	s.ops++
	shouldPruneTTL := s.ttl > 0 && s.ops%sessionMetricPruneEveryN == 0
	shouldPruneCap := s.maxEntries > 0 && len(s.entries) > s.maxEntries
	if shouldPruneTTL || shouldPruneCap {
		s.pruneLocked(now)
	}

	return entry.count
}

func (s *boundedSessionCounterStore) Delete(sessionID string) {
	if s == nil || sessionID == "" {
		return
	}
	s.mu.Lock()
	delete(s.entries, sessionID)
	s.mu.Unlock()
}

func (s *boundedSessionCounterStore) Snapshot() map[string]int64 {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.entries) == 0 {
		return nil
	}

	out := make(map[string]int64, len(s.entries))
	for sessionID, entry := range s.entries {
		if entry == nil {
			continue
		}
		out[sessionID] = entry.count
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *boundedSessionCounterStore) pruneLocked(now time.Time) {
	if len(s.entries) == 0 {
		return
	}

	if s.ttl > 0 {
		for sessionID, entry := range s.entries {
			if entry == nil {
				delete(s.entries, sessionID)
				continue
			}
			if now.Sub(entry.lastSeen) > s.ttl {
				delete(s.entries, sessionID)
			}
		}
	}

	if s.maxEntries <= 0 || len(s.entries) <= s.maxEntries {
		return
	}

	type counterInfo struct {
		sessionID string
		lastSeen  time.Time
	}

	counters := make([]counterInfo, 0, len(s.entries))
	for sessionID, entry := range s.entries {
		lastSeen := time.Time{}
		if entry != nil {
			lastSeen = entry.lastSeen
		}
		counters = append(counters, counterInfo{
			sessionID: sessionID,
			lastSeen:  lastSeen,
		})
	}

	sort.Slice(counters, func(i, j int) bool {
		if counters[i].lastSeen.Equal(counters[j].lastSeen) {
			return counters[i].sessionID < counters[j].sessionID
		}
		return counters[i].lastSeen.Before(counters[j].lastSeen)
	})

	toEvict := len(counters) - s.maxEntries
	for i := 0; i < toEvict; i++ {
		delete(s.entries, counters[i].sessionID)
	}
}

// Metrics helper methods — lock-free via atomic.Int64.
func (m *broadcasterMetrics) incrementEventsSent() { m.totalEventsSent.Add(1) }
func (m *broadcasterMetrics) incrementDroppedEvents(sessionID string) int64 {
	m.droppedEvents.Add(1)
	return m.dropsPerSession.Increment(sessionID)
}
func (m *broadcasterMetrics) incrementNoClientEvents(sessionID string) int64 {
	if sessionID == "" {
		sessionID = missingSessionIDKey
	}
	m.noClientEvents.Add(1)
	return m.noClientBySession.Increment(sessionID)
}
func (m *broadcasterMetrics) incrementConnections() {
	m.totalConnections.Add(1)
	m.activeConnections.Add(1)
}
func (m *broadcasterMetrics) decrementConnections() { m.activeConnections.Add(-1) }

// clearSessionDrops removes the per-session drop counter (called on client unregister).
func (m *broadcasterMetrics) clearSessionDrops(sessionID string) {
	m.dropsPerSession.Delete(sessionID)
}

func (m *broadcasterMetrics) clearSessionNoClient(sessionID string) {
	m.noClientBySession.Delete(sessionID)
}

func shouldSampleCounter(count int64) bool {
	// Sample at powers of two (1,2,4,8...) to cap log volume while preserving signal.
	return count > 0 && count&(count-1) == 0
}

// BroadcasterMetrics represents broadcaster metrics for export
type BroadcasterMetrics struct {
	TotalEventsSent   int64            `json:"total_events_sent"`
	DroppedEvents     int64            `json:"dropped_events"`
	DropsPerSession   map[string]int64 `json:"drops_per_session,omitempty"` // Per-session drop counts
	NoClientEvents    int64            `json:"no_client_events"`
	NoClientBySession map[string]int64 `json:"no_client_by_session,omitempty"`
	TotalConnections  int64            `json:"total_connections"`
	ActiveConnections int64            `json:"active_connections"`
	BufferDepth       map[string]int   `json:"buffer_depth"` // Per-session buffer depth
	SessionCount      int              `json:"session_count"`
}

// GetMetrics returns current broadcaster metrics
func (b *EventBroadcaster) GetMetrics() BroadcasterMetrics {
	totalEvents := b.metrics.totalEventsSent.Load()
	droppedEvents := b.metrics.droppedEvents.Load()
	noClientEvents := b.metrics.noClientEvents.Load()
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

	dropsPerSession := b.metrics.dropsPerSession.Snapshot()
	noClientBySession := b.metrics.noClientBySession.Snapshot()

	return BroadcasterMetrics{
		TotalEventsSent:   totalEvents,
		DroppedEvents:     droppedEvents,
		DropsPerSession:   dropsPerSession,
		NoClientEvents:    noClientEvents,
		NoClientBySession: noClientBySession,
		TotalConnections:  totalConns,
		ActiveConnections: activeConns,
		BufferDepth:       bufferDepth,
		SessionCount:      len(clientsBySession),
	}
}
