package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
)

// ── test doubles ──

// inMemoryHistoryStore implements EventHistoryStore for testing.
type inMemoryHistoryStore struct {
	mu     sync.Mutex
	events []agent.AgentEvent

	appendErr         error
	deleteErr         error
	hasEventsResult   bool
	hasEventsErr      error
	streamErr         error
	appendDelay       time.Duration
	appendCallCount   atomic.Int64
	deletedSessionIDs []string
}

func (s *inMemoryHistoryStore) Append(_ context.Context, event agent.AgentEvent) error {
	s.appendCallCount.Add(1)
	if s.appendDelay > 0 {
		time.Sleep(s.appendDelay)
	}
	if s.appendErr != nil {
		return s.appendErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *inMemoryHistoryStore) Stream(_ context.Context, _ EventHistoryFilter, fn func(agent.AgentEvent) error) error {
	if s.streamErr != nil {
		return s.streamErr
	}
	s.mu.Lock()
	snapshot := make([]agent.AgentEvent, len(s.events))
	copy(snapshot, s.events)
	s.mu.Unlock()
	for _, e := range snapshot {
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

func (s *inMemoryHistoryStore) DeleteSession(_ context.Context, sessionID string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletedSessionIDs = append(s.deletedSessionIDs, sessionID)
	return nil
}

func (s *inMemoryHistoryStore) HasSessionEvents(_ context.Context, _ string) (bool, error) {
	return s.hasEventsResult, s.hasEventsErr
}

func (s *inMemoryHistoryStore) eventCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

// batchHistoryStore extends inMemoryHistoryStore with AppendBatch support.
type batchHistoryStore struct {
	inMemoryHistoryStore
	batchCallCount atomic.Int64
}

func (s *batchHistoryStore) AppendBatch(_ context.Context, events []agent.AgentEvent) error {
	s.batchCallCount.Add(1)
	if s.appendErr != nil {
		return s.appendErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, events...)
	return nil
}

type failAfterStore struct {
	mu        sync.Mutex
	events    []agent.AgentEvent
	failAt    int
	callCount int
	failErr   error
}

func (s *failAfterStore) Append(_ context.Context, event agent.AgentEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callCount++
	if s.failAt > 0 && s.callCount == s.failAt {
		if s.failErr == nil {
			s.failErr = errors.New("append failed")
		}
		return s.failErr
	}
	s.events = append(s.events, event)
	return nil
}

func (s *failAfterStore) Stream(_ context.Context, _ EventHistoryFilter, fn func(agent.AgentEvent) error) error {
	s.mu.Lock()
	snapshot := append([]agent.AgentEvent(nil), s.events...)
	s.mu.Unlock()
	for _, e := range snapshot {
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

func (s *failAfterStore) DeleteSession(_ context.Context, _ string) error { return nil }

func (s *failAfterStore) HasSessionEvents(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (s *failAfterStore) eventCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

// ── helpers ──

func makeTestEvent(sessionID string) agent.AgentEvent {
	return &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, sessionID, "run-1", "", time.Now()),
		Version:   1,
		Event:     "test.event",
	}
}

func newTestStore(inner EventHistoryStore, opts ...AsyncEventHistoryStoreOption) *AsyncEventHistoryStore {
	defaults := []AsyncEventHistoryStoreOption{
		WithAsyncHistoryFlushInterval(10 * time.Millisecond),
		WithAsyncHistoryAppendTimeout(50 * time.Millisecond),
	}
	return NewAsyncEventHistoryStore(inner, append(defaults, opts...)...)
}

// ── Append ──

func TestAsyncAppendEnqueuesEvent(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := newTestStore(inner)
	defer store.Close()

	if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Flush to ensure event reaches inner store.
	if err := store.flush(context.Background()); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if got := inner.eventCount(); got != 1 {
		t.Fatalf("expected 1 event, got %d", got)
	}
}

func TestAsyncAppendNilEventIsIgnored(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := newTestStore(inner)
	defer store.Close()

	if err := store.Append(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := store.flush(context.Background()); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if got := inner.eventCount(); got != 0 {
		t.Fatalf("expected 0 events, got %d", got)
	}
}

func TestAsyncAppendNilStoreIsNoop(t *testing.T) {
	store := NewAsyncEventHistoryStore(nil)
	defer store.Close()

	if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAsyncAppendQueueFullReturnsError(t *testing.T) {
	// Create a store with a tiny channel and no background goroutine so the
	// channel fills up deterministically.
	store := &AsyncEventHistoryStore{
		inner:         &inMemoryHistoryStore{},
		ch:            make(chan agent.AgentEvent, 2),
		flushRequests: make(chan chan error, 16),
		done:          make(chan struct{}),
		batchSize:     200,
		flushInterval: time.Hour,
		appendTimeout: 1 * time.Millisecond,
	}
	defer close(store.done)

	_ = store.Append(context.Background(), makeTestEvent("s1"))
	_ = store.Append(context.Background(), makeTestEvent("s1"))

	// Channel is full. Next append should timeout.
	err := store.Append(context.Background(), makeTestEvent("s1"))
	if err == nil {
		t.Fatal("expected error from full queue")
	}
	if !errors.Is(err, ErrAsyncHistoryQueueFull) {
		t.Fatalf("expected ErrAsyncHistoryQueueFull, got %v", err)
	}
}

func TestAsyncAppendContextCancellation(t *testing.T) {
	// Tiny channel, no background goroutine — deterministic fill.
	store := &AsyncEventHistoryStore{
		inner:         &inMemoryHistoryStore{},
		ch:            make(chan agent.AgentEvent, 1),
		flushRequests: make(chan chan error, 16),
		done:          make(chan struct{}),
		batchSize:     200,
		flushInterval: time.Hour,
		appendTimeout: 5 * time.Second,
	}
	defer close(store.done)

	_ = store.Append(context.Background(), makeTestEvent("s1")) // fill the 1-slot buffer

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := store.Append(ctx, makeTestEvent("s1"))
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// ── Stream ──

func TestAsyncStreamFlushesBeforeRead(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := newTestStore(inner)
	defer store.Close()

	// Append an event, then immediately stream — the flush-before-read
	// guarantee must make the event visible.
	if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
		t.Fatalf("append error: %v", err)
	}

	var collected []agent.AgentEvent
	err := store.Stream(context.Background(), EventHistoryFilter{SessionID: "s1"}, func(e agent.AgentEvent) error {
		collected = append(collected, e)
		return nil
	})
	if err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if len(collected) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collected))
	}
}

func TestAsyncStreamNilStoreReturnsError(t *testing.T) {
	store := NewAsyncEventHistoryStore(nil)
	defer store.Close()

	err := store.Stream(context.Background(), EventHistoryFilter{}, func(agent.AgentEvent) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error from nil store")
	}
}

func TestAsyncStreamPropagatesInnerError(t *testing.T) {
	streamErr := errors.New("stream failure")
	inner := &inMemoryHistoryStore{streamErr: streamErr}
	store := newTestStore(inner)
	defer store.Close()

	err := store.Stream(context.Background(), EventHistoryFilter{}, func(agent.AgentEvent) error {
		return nil
	})
	if !errors.Is(err, streamErr) {
		t.Fatalf("expected stream failure, got %v", err)
	}
}

// ── DeleteSession ──

func TestAsyncDeleteSessionFlushesFirst(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := newTestStore(inner)
	defer store.Close()

	// Append, then delete — the flush must happen before delete.
	if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
		t.Fatalf("append error: %v", err)
	}

	if err := store.DeleteSession(context.Background(), "s1"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	inner.mu.Lock()
	deleted := inner.deletedSessionIDs
	inner.mu.Unlock()

	if len(deleted) != 1 || deleted[0] != "s1" {
		t.Fatalf("expected delete for s1, got %v", deleted)
	}

	// The event should have been flushed to inner before delete was called.
	if got := inner.eventCount(); got != 1 {
		t.Fatalf("expected 1 event flushed before delete, got %d", got)
	}
}

func TestAsyncDeleteSessionNilStoreReturnsError(t *testing.T) {
	store := NewAsyncEventHistoryStore(nil)
	defer store.Close()

	err := store.DeleteSession(context.Background(), "s1")
	if err == nil {
		t.Fatal("expected error from nil store")
	}
}

func TestAsyncDeleteSessionPropagatesError(t *testing.T) {
	deleteErr := errors.New("delete failed")
	inner := &inMemoryHistoryStore{deleteErr: deleteErr}
	store := newTestStore(inner)
	defer store.Close()

	err := store.DeleteSession(context.Background(), "s1")
	if !errors.Is(err, deleteErr) {
		t.Fatalf("expected delete error, got %v", err)
	}
}

// ── HasSessionEvents ──

func TestAsyncHasSessionEventsFlushesFirst(t *testing.T) {
	inner := &inMemoryHistoryStore{hasEventsResult: true}
	store := newTestStore(inner)
	defer store.Close()

	// Append so there's something to flush.
	if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
		t.Fatalf("append error: %v", err)
	}

	has, err := store.HasSessionEvents(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !has {
		t.Fatal("expected true")
	}

	// Verify event was flushed.
	if got := inner.eventCount(); got != 1 {
		t.Fatalf("expected 1 event flushed, got %d", got)
	}
}

func TestAsyncHasSessionEventsNilStoreReturnsError(t *testing.T) {
	store := NewAsyncEventHistoryStore(nil)
	defer store.Close()

	_, err := store.HasSessionEvents(context.Background(), "s1")
	if err == nil {
		t.Fatal("expected error from nil store")
	}
}

func TestAsyncHasSessionEventsPropagatesError(t *testing.T) {
	hasErr := errors.New("has check failed")
	inner := &inMemoryHistoryStore{hasEventsErr: hasErr}
	store := newTestStore(inner)
	defer store.Close()

	_, err := store.HasSessionEvents(context.Background(), "s1")
	if !errors.Is(err, hasErr) {
		t.Fatalf("expected has check error, got %v", err)
	}
}

// ── Close ──

func TestAsyncCloseFlushesRemainingEvents(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := newTestStore(inner)

	for i := 0; i < 5; i++ {
		if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
			t.Fatalf("append %d error: %v", i, err)
		}
	}

	if err := store.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	if got := inner.eventCount(); got != 5 {
		t.Fatalf("expected 5 events flushed on close, got %d", got)
	}
}

func TestAsyncCloseIsIdempotent(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := newTestStore(inner)

	if err := store.Close(); err != nil {
		t.Fatalf("first close error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second close error: %v", err)
	}
}

func TestAsyncCloseNilStoreIsNoop(t *testing.T) {
	store := NewAsyncEventHistoryStore(nil)
	if err := store.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── Batch path ──

func TestAsyncBatchAppendPath(t *testing.T) {
	inner := &batchHistoryStore{}
	store := newTestStore(&inner.inMemoryHistoryStore,
		WithAsyncHistoryBatchSize(5),
	)
	// Replace inner with the batch store after construction to test the batchEventAppender path.
	store.inner = inner
	defer store.Close()

	for i := 0; i < 5; i++ {
		if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
			t.Fatalf("append %d error: %v", i, err)
		}
	}

	if err := store.flush(context.Background()); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if got := inner.eventCount(); got != 5 {
		t.Fatalf("expected 5 events, got %d", got)
	}
	if inner.batchCallCount.Load() == 0 {
		t.Fatal("expected AppendBatch to be called")
	}
	if inner.appendCallCount.Load() != 0 {
		t.Fatal("expected single Append NOT to be called when batch path is available")
	}
}

func TestAsyncSingleAppendFallback(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := newTestStore(inner)
	defer store.Close()

	for i := 0; i < 3; i++ {
		if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
			t.Fatalf("append %d error: %v", i, err)
		}
	}

	if err := store.flush(context.Background()); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if got := inner.eventCount(); got != 3 {
		t.Fatalf("expected 3 events, got %d", got)
	}
	if inner.appendCallCount.Load() != 3 {
		t.Fatalf("expected 3 individual Append calls, got %d", inner.appendCallCount.Load())
	}
}

// ── Batch size triggers flush ──

func TestAsyncBatchSizeTriggerFlush(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := newTestStore(inner,
		WithAsyncHistoryBatchSize(3),
		WithAsyncHistoryFlushInterval(10*time.Second), // long interval so only batch size triggers
	)
	defer store.Close()

	// Append exactly batchSize events.
	for i := 0; i < 3; i++ {
		if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
			t.Fatalf("append %d error: %v", i, err)
		}
	}

	// Give the run loop time to process the batch.
	time.Sleep(100 * time.Millisecond)

	if got := inner.eventCount(); got != 3 {
		t.Fatalf("expected 3 events auto-flushed by batch size, got %d", got)
	}
}

// ── Options ──

func TestAsyncOptionsApplied(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := NewAsyncEventHistoryStore(inner,
		WithAsyncHistoryBatchSize(50),
		WithAsyncHistoryFlushInterval(500*time.Millisecond),
		WithAsyncHistoryAppendTimeout(100*time.Millisecond),
	)
	defer store.Close()

	if store.batchSize != 50 {
		t.Fatalf("expected batchSize 50, got %d", store.batchSize)
	}
	if store.flushInterval != 500*time.Millisecond {
		t.Fatalf("expected flushInterval 500ms, got %v", store.flushInterval)
	}
	if store.appendTimeout != 100*time.Millisecond {
		t.Fatalf("expected appendTimeout 100ms, got %v", store.appendTimeout)
	}
}

func TestAsyncOptionsIgnoreInvalid(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := NewAsyncEventHistoryStore(inner,
		WithAsyncHistoryBatchSize(0),
		WithAsyncHistoryBatchSize(-1),
		WithAsyncHistoryFlushInterval(0),
		WithAsyncHistoryFlushInterval(-1),
		WithAsyncHistoryAppendTimeout(0),
		WithAsyncHistoryAppendTimeout(-1),
		nil, // nil option should not panic
	)
	defer store.Close()

	// Defaults should remain.
	if store.batchSize != 200 {
		t.Fatalf("expected default batchSize 200, got %d", store.batchSize)
	}
	if store.flushInterval != 250*time.Millisecond {
		t.Fatalf("expected default flushInterval 250ms, got %v", store.flushInterval)
	}
	if store.appendTimeout != 50*time.Millisecond {
		t.Fatalf("expected default appendTimeout 50ms, got %v", store.appendTimeout)
	}
}

// ── Multiple events and ordering ──

func TestAsyncStreamPreservesOrder(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := newTestStore(inner)
	defer store.Close()

	for i := 0; i < 10; i++ {
		ev := &domain.WorkflowEventEnvelope{
			BaseEvent: domain.NewBaseEvent(agent.LevelCore, "s1", "run-1", "", time.Now()),
			Version:   1,
			Event:     "test.event",
			NodeID:    string(rune('A' + i)),
		}
		if err := store.Append(context.Background(), ev); err != nil {
			t.Fatalf("append %d error: %v", i, err)
		}
	}

	var collected []*domain.WorkflowEventEnvelope
	err := store.Stream(context.Background(), EventHistoryFilter{SessionID: "s1"}, func(e agent.AgentEvent) error {
		collected = append(collected, e.(*domain.WorkflowEventEnvelope))
		return nil
	})
	if err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if len(collected) != 10 {
		t.Fatalf("expected 10 events, got %d", len(collected))
	}
	for i, ev := range collected {
		expected := string(rune('A' + i))
		if ev.NodeID != expected {
			t.Fatalf("event %d: expected NodeID %q, got %q", i, expected, ev.NodeID)
		}
	}
}

// ── Flush context cancellation ──

func TestAsyncFlushContextCancellation(t *testing.T) {
	inner := &inMemoryHistoryStore{}
	store := newTestStore(inner)
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Stream(ctx, EventHistoryFilter{}, func(agent.AgentEvent) error {
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// ── Inner append error in single-event fallback ──

func TestAsyncSingleAppendErrorIsLogged(t *testing.T) {
	appendErr := errors.New("append failed")
	inner := &inMemoryHistoryStore{appendErr: appendErr}
	store := newTestStore(inner)
	defer store.Close()

	// Events will be enqueued but inner.Append will fail.
	if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
		t.Fatalf("enqueue should succeed: %v", err)
	}

	// Trigger a flush — the error propagates through the flush response.
	err := store.flush(context.Background())
	if err == nil {
		t.Fatal("expected flush to propagate inner append error")
	}
	if !errors.Is(err, appendErr) {
		t.Fatalf("expected append error, got %v", err)
	}
}

// ── Batch append error propagates ──

func TestAsyncBatchAppendErrorPropagates(t *testing.T) {
	batchErr := errors.New("batch failed")
	inner := &batchHistoryStore{}
	inner.appendErr = batchErr
	store := newTestStore(&inner.inMemoryHistoryStore)
	store.inner = inner // swap to batch-capable store
	defer store.Close()

	if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
		t.Fatalf("enqueue should succeed: %v", err)
	}

	err := store.flush(context.Background())
	if err == nil {
		t.Fatal("expected flush to propagate batch error")
	}
	if !errors.Is(err, batchErr) {
		t.Fatalf("expected batch error, got %v", err)
	}
}

func TestAsyncFlushRetainsBufferOnFailure(t *testing.T) {
	inner := &failAfterStore{failAt: 2, failErr: errors.New("append failed")}
	store := NewAsyncEventHistoryStore(inner,
		WithAsyncHistoryFlushInterval(time.Hour),
	)
	defer store.Close()

	for i := 0; i < 3; i++ {
		if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
			t.Fatalf("append %d error: %v", i, err)
		}
	}

	err := store.flush(context.Background())
	if err == nil {
		t.Fatal("expected flush error")
	}
	if got := inner.eventCount(); got != 1 {
		t.Fatalf("expected 1 event after failed flush, got %d", got)
	}

	inner.failAt = 0
	inner.failErr = nil

	if err := store.flush(context.Background()); err != nil {
		t.Fatalf("flush retry error: %v", err)
	}
	if got := inner.eventCount(); got != 3 {
		t.Fatalf("expected 3 events after retry, got %d", got)
	}
}

func TestAsyncFlushBackoffSkipsRetries(t *testing.T) {
	inner := &inMemoryHistoryStore{appendErr: errors.New("append failed")}
	store := NewAsyncEventHistoryStore(inner,
		WithAsyncHistoryFlushInterval(5*time.Millisecond),
	)
	defer store.Close()

	if err := store.Append(context.Background(), makeTestEvent("s1")); err != nil {
		t.Fatalf("append error: %v", err)
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for inner.appendCallCount.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	firstCount := inner.appendCallCount.Load()
	if firstCount == 0 {
		t.Fatal("expected at least one flush attempt")
	}

	time.Sleep(50 * time.Millisecond)
	if got := inner.appendCallCount.Load(); got != firstCount {
		t.Fatalf("expected backoff to suppress retries, got %d then %d", firstCount, got)
	}
}
