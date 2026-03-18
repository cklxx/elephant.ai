package signals

import (
	"context"
	"sync"
	"testing"
	"time"
)

type collectHandler struct {
	mu     sync.Mutex
	events []SignalEvent
}

func (h *collectHandler) HandleSignal(_ context.Context, event SignalEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
}

func (h *collectHandler) collected() []SignalEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]SignalEvent, len(h.events))
	copy(out, h.events)
	return out
}

func TestGraphIngestAndProcess(t *testing.T) {
	now := time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return now }
	cfg := DefaultConfig()
	cfg.BudgetMax = 0

	scorer := NewScorer(nil, 0, nowFn)
	router := NewRouter(cfg, nowFn)
	handler := &collectHandler{}
	g := NewGraph(nil, scorer, router, 10, []SignalHandler{handler})

	ctx := context.Background()
	if err := g.Start(ctx); err != nil {
		t.Fatal(err)
	}

	g.Ingest(SignalEvent{ID: "e1", Content: "hello"})
	g.Ingest(SignalEvent{ID: "e2", Content: "urgent error"})

	// Give processing loop time to consume.
	time.Sleep(50 * time.Millisecond)
	g.Stop()

	events := handler.collected()
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	recent := g.RecentSignals()
	if len(recent) < 2 {
		t.Errorf("RecentSignals() = %d, want >= 2", len(recent))
	}
}

func TestGraphStopWithoutStart(t *testing.T) {
	scorer := NewScorer(nil, 0, time.Now)
	router := NewRouter(DefaultConfig(), time.Now)
	g := NewGraph(nil, scorer, router, 10, nil)
	g.Stop() // should not panic
}
