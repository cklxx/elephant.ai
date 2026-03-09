package leader

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/runtime/session"
)

// --- mocks ---

type mockRuntime struct {
	sessions map[string]session.SessionData
}

func (m *mockRuntime) GetSession(id string) (session.SessionData, bool) {
	s, ok := m.sessions[id]
	return s, ok
}

func (m *mockRuntime) InjectText(_ context.Context, _, _ string) error { return nil }
func (m *mockRuntime) MarkFailed(_, _ string) error                    { return nil }

type mockBus struct {
	mu      sync.Mutex
	subs    []chan hooks.Event
	events  []hooks.Event
	eventMu sync.Mutex
}

func newMockBus() *mockBus {
	return &mockBus{}
}

func (b *mockBus) Publish(_ string, ev hooks.Event) {
	b.eventMu.Lock()
	b.events = append(b.events, ev)
	b.eventMu.Unlock()
}

func (b *mockBus) Subscribe(_ string) (<-chan hooks.Event, func()) {
	ch := make(chan hooks.Event, 1024)
	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	return ch, func() {}
}

func (b *mockBus) SubscribeAll() (<-chan hooks.Event, func()) {
	ch := make(chan hooks.Event, 1024)
	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	return ch, func() {}
}

func (b *mockBus) send(ev hooks.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (b *mockBus) publishedEvents() []hooks.Event {
	b.eventMu.Lock()
	defer b.eventMu.Unlock()
	return append([]hooks.Event(nil), b.events...)
}

// --- tests ---

func TestHandleStallDeduplication(t *testing.T) {
	var callCount atomic.Int32

	rt := &mockRuntime{
		sessions: map[string]session.SessionData{
			"sess-1": {ID: "sess-1", Goal: "test goal"},
		},
	}
	bus := newMockBus()
	executeFn := func(ctx context.Context, prompt, _ string) (string, error) {
		callCount.Add(1)
		time.Sleep(50 * time.Millisecond) // simulate LLM latency
		return "INJECT keep going", nil
	}

	a := New(rt, bus, executeFn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)

	// Give the goroutine time to subscribe.
	time.Sleep(10 * time.Millisecond)

	// Fire 10 stall events for the same session rapidly.
	for i := 0; i < 10; i++ {
		bus.send(hooks.Event{
			Type:      hooks.EventStalled,
			SessionID: "sess-1",
			At:        time.Now(),
		})
	}

	time.Sleep(200 * time.Millisecond)

	// Only 1-2 should have executed (first + possibly one after release).
	count := callCount.Load()
	if count > 3 {
		t.Errorf("expected at most 3 concurrent executions for same session, got %d", count)
	}
	t.Logf("execute called %d times for 10 rapid stall events (same session)", count)
}

func TestHandleStallMultipleSessions(t *testing.T) {
	var callCount atomic.Int32

	sessions := make(map[string]session.SessionData)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("sess-%d", i)
		sessions[id] = session.SessionData{ID: id, Goal: "goal"}
	}

	rt := &mockRuntime{sessions: sessions}
	bus := newMockBus()
	executeFn := func(ctx context.Context, prompt, _ string) (string, error) {
		callCount.Add(1)
		time.Sleep(20 * time.Millisecond)
		return "INJECT ok", nil
	}

	a := New(rt, bus, executeFn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)

	// Give the goroutine time to subscribe.
	time.Sleep(10 * time.Millisecond)

	// Fire events for 5 different sessions — all should execute concurrently.
	for i := 0; i < 5; i++ {
		bus.send(hooks.Event{
			Type:      hooks.EventStalled,
			SessionID: fmt.Sprintf("sess-%d", i),
			At:        time.Now(),
		})
	}

	time.Sleep(300 * time.Millisecond)

	count := callCount.Load()
	if count != 5 {
		t.Errorf("expected 5 executions (one per session), got %d", count)
	}
}

func TestStallStress10000Events(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const (
		totalEvents   = 10000
		numSessions   = 20
		maxGoroutines = 500 // safety cap
	)

	var (
		callCount    atomic.Int32
		peakRoutines atomic.Int32
		activeCount  atomic.Int32
	)

	sessions := make(map[string]session.SessionData)
	for i := 0; i < numSessions; i++ {
		id := fmt.Sprintf("stress-sess-%d", i)
		sessions[id] = session.SessionData{ID: id, Goal: "stress test"}
	}

	rt := &mockRuntime{sessions: sessions}
	bus := newMockBus()
	executeFn := func(ctx context.Context, prompt, _ string) (string, error) {
		callCount.Add(1)
		current := activeCount.Add(1)
		defer activeCount.Add(-1)

		// Track peak concurrent goroutines.
		for {
			peak := peakRoutines.Load()
			if current <= peak || peakRoutines.CompareAndSwap(peak, current) {
				break
			}
		}

		time.Sleep(time.Millisecond) // minimal LLM simulation
		return "INJECT continue", nil
	}

	a := New(rt, bus, executeFn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)

	// Give the goroutine time to subscribe.
	time.Sleep(10 * time.Millisecond)

	// Record goroutines before.
	goroutinesBefore := runtime.NumGoroutine()

	// Fire 10000 events across 20 sessions.
	for i := 0; i < totalEvents; i++ {
		sessID := fmt.Sprintf("stress-sess-%d", i%numSessions)
		bus.send(hooks.Event{
			Type:      hooks.EventStalled,
			SessionID: sessID,
			At:        time.Now(),
		})
		// Small jitter to avoid pure burst.
		if i%100 == 0 {
			time.Sleep(time.Millisecond)
		}
	}

	// Wait for all to complete.
	time.Sleep(500 * time.Millisecond)
	cancel()

	goroutinesAfter := runtime.NumGoroutine()
	peakConcurrent := peakRoutines.Load()
	totalCalls := callCount.Load()

	t.Logf("Results:")
	t.Logf("  Total events sent:       %d", totalEvents)
	t.Logf("  Total execute calls:     %d", totalCalls)
	t.Logf("  Peak concurrent calls:   %d", peakConcurrent)
	t.Logf("  Goroutines before/after: %d / %d", goroutinesBefore, goroutinesAfter)

	// With dedup, at most numSessions can be concurrent at once.
	if peakConcurrent > int32(numSessions) {
		t.Errorf("peak concurrent (%d) exceeded session count (%d) — dedup broken",
			peakConcurrent, numSessions)
	}

	// Goroutine count should not explode.
	goroutineGrowth := goroutinesAfter - goroutinesBefore
	if goroutineGrowth > maxGoroutines {
		t.Errorf("goroutine growth too high: %d (before=%d after=%d)",
			goroutineGrowth, goroutinesBefore, goroutinesAfter)
	}

	// Should have far fewer calls than events due to dedup.
	if totalCalls > int32(totalEvents/2) {
		t.Errorf("expected significant dedup reduction, got %d calls for %d events",
			totalCalls, totalEvents)
	}
}

func TestApplyDecisionINJECT(t *testing.T) {
	rt := &mockRuntime{sessions: map[string]session.SessionData{}}
	bus := newMockBus()
	a := New(rt, bus, nil)

	// Should not panic.
	a.applyDecision(context.Background(), "sess-1", "INJECT try a different approach")
}

func TestApplyDecisionESCALATE(t *testing.T) {
	rt := &mockRuntime{sessions: map[string]session.SessionData{}}
	bus := newMockBus()
	agent := New(rt, bus, nil)

	agent.applyDecision(context.Background(), "sess-1", "ESCALATE")

	events := bus.publishedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != hooks.EventHandoffRequired {
		t.Errorf("expected EventHandoffRequired, got %v", events[0].Type)
	}
}

func TestBuildStallPrompt(t *testing.T) {
	prompt := buildStallPrompt("s1", "backend", "fix the bug", 90*time.Second, hooks.EventStalled)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	// Should contain session info.
	for _, expected := range []string{"s1", "backend", "fix the bug", "stalled", "1m30s"} {
		if !contains(prompt, expected) {
			t.Errorf("prompt missing %q", expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
