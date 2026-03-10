package leader

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/runtime/session"
	"alex/internal/shared/notification"
)

// --- mocks ---

type mockRuntime struct {
	sessions     map[string]session.SessionData
	markFailedFn func(id, errMsg string) error
}

func (m *mockRuntime) GetSession(id string) (session.SessionData, bool) {
	s, ok := m.sessions[id]
	return s, ok
}

func (m *mockRuntime) InjectText(_ context.Context, _, _ string) error { return nil }
func (m *mockRuntime) MarkFailed(id, errMsg string) error {
	if m.markFailedFn != nil {
		return m.markFailedFn(id, errMsg)
	}
	return nil
}

type mockNotifier struct {
	mu       sync.Mutex
	messages []string
}

func (n *mockNotifier) Send(_ context.Context, _ notification.Target, content string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.messages = append(n.messages, content)
	return nil
}

func (n *mockNotifier) getMessages() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.messages...)
}

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

func TestStallSessionIDStable(t *testing.T) {
	// Verify that the same runtime session always maps to the same leader session.
	id1 := stallSessionID("rs-abc123")
	id2 := stallSessionID("rs-abc123")
	if id1 != id2 {
		t.Errorf("expected stable ID, got %q and %q", id1, id2)
	}
	if id1 != "leader-stall-rs-abc123" {
		t.Errorf("unexpected format: %q", id1)
	}

	// Different runtime sessions produce different leader sessions.
	id3 := stallSessionID("rs-xyz789")
	if id1 == id3 {
		t.Error("expected different IDs for different sessions")
	}
}

func TestStallSessionIDUsedByExecute(t *testing.T) {
	// Verify that handleStall passes the stable session ID to execute.
	var capturedSessionIDs []string
	var mu sync.Mutex

	rt := &mockRuntime{
		sessions: map[string]session.SessionData{
			"rs-test": {ID: "rs-test", Goal: "test"},
		},
	}
	bus := newMockBus()
	executeFn := func(_ context.Context, _, sid string) (string, error) {
		mu.Lock()
		capturedSessionIDs = append(capturedSessionIDs, sid)
		mu.Unlock()
		return "INJECT ok", nil
	}

	a := New(rt, bus, executeFn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Send 3 stall events for the same session (with gaps to allow processing).
	for i := 0; i < 3; i++ {
		bus.send(hooks.Event{
			Type:      hooks.EventStalled,
			SessionID: "rs-test",
			At:        time.Now(),
		})
		time.Sleep(80 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(capturedSessionIDs) == 0 {
		t.Fatal("expected at least 1 execute call")
	}

	// All calls should use the same stable session ID.
	expected := "leader-stall-rs-test"
	for i, sid := range capturedSessionIDs {
		if sid != expected {
			t.Errorf("call %d: expected session ID %q, got %q", i, expected, sid)
		}
	}
	t.Logf("captured %d calls, all using stable session ID %q", len(capturedSessionIDs), expected)
}

func TestMaxStallAttemptsEscalates(t *testing.T) {
	rt := &mockRuntime{
		sessions: map[string]session.SessionData{
			"sess-1": {ID: "sess-1", Goal: "stuck task"},
		},
	}
	bus := newMockBus()
	var callCount atomic.Int32
	executeFn := func(_ context.Context, _, _ string) (string, error) {
		callCount.Add(1)
		return "INJECT try again", nil
	}

	a := New(rt, bus, executeFn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Send maxStallAttempts + 2 events with gaps.
	for i := 0; i < maxStallAttempts+2; i++ {
		bus.send(hooks.Event{
			Type:      hooks.EventStalled,
			SessionID: "sess-1",
			At:        time.Now(),
		})
		time.Sleep(80 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	// Should have called execute exactly maxStallAttempts times.
	calls := callCount.Load()
	if calls != int32(maxStallAttempts) {
		t.Errorf("expected %d execute calls (max attempts), got %d", maxStallAttempts, calls)
	}

	// Should have published escalation events for attempts beyond max.
	events := bus.publishedEvents()
	escalations := 0
	for _, ev := range events {
		if ev.Type == hooks.EventHandoffRequired {
			escalations++
		}
	}
	if escalations == 0 {
		t.Error("expected at least one escalation event after max stall attempts")
	}
	t.Logf("execute calls: %d, escalations: %d", calls, escalations)
}

func TestHeartbeatResetsStallCount(t *testing.T) {
	rt := &mockRuntime{
		sessions: map[string]session.SessionData{
			"sess-1": {ID: "sess-1", Goal: "task"},
		},
	}
	bus := newMockBus()
	var callCount atomic.Int32
	executeFn := func(_ context.Context, _, _ string) (string, error) {
		callCount.Add(1)
		return "INJECT ok", nil
	}

	a := New(rt, bus, executeFn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Exhaust maxStallAttempts - 1.
	for i := 0; i < maxStallAttempts-1; i++ {
		bus.send(hooks.Event{Type: hooks.EventStalled, SessionID: "sess-1", At: time.Now()})
		time.Sleep(80 * time.Millisecond)
	}

	// Heartbeat resets the counter.
	bus.send(hooks.Event{Type: hooks.EventHeartbeat, SessionID: "sess-1", At: time.Now()})
	time.Sleep(20 * time.Millisecond)

	// Now we should be able to do maxStallAttempts more.
	for i := 0; i < maxStallAttempts; i++ {
		bus.send(hooks.Event{Type: hooks.EventStalled, SessionID: "sess-1", At: time.Now()})
		time.Sleep(80 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	calls := callCount.Load()
	expected := int32(maxStallAttempts-1) + int32(maxStallAttempts)
	if calls != expected {
		t.Errorf("expected %d execute calls after heartbeat reset, got %d", expected, calls)
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
	prompt := buildStallPrompt("s1", "backend", "fix the bug", 90*time.Second, hooks.EventStalled, 2)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	// Should contain session info and attempt count.
	for _, expected := range []string{"s1", "backend", "fix the bug", "stalled", "1m30s", "2 of 3"} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt missing %q", expected)
		}
	}
}

func TestMarkFailedRetrySuccess(t *testing.T) {
	// MarkFailed fails twice then succeeds on 3rd attempt.
	var attempts atomic.Int32
	rt := &mockRuntime{
		sessions: map[string]session.SessionData{},
		markFailedFn: func(_, _ string) error {
			n := attempts.Add(1)
			if n < 3 {
				return errors.New("transient db error")
			}
			return nil
		},
	}
	bus := newMockBus()
	notif := &mockNotifier{}
	a := New(rt, bus, nil)
	a.SetNotifier(notif)

	a.markFailedWithRetry("sess-retry", "test reason")

	if got := attempts.Load(); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
	// No escalation event — it succeeded.
	events := bus.publishedEvents()
	for _, ev := range events {
		if ev.Type == hooks.EventHandoffRequired {
			t.Error("unexpected escalation event — MarkFailed eventually succeeded")
		}
	}
	// No notification sent.
	if msgs := notif.getMessages(); len(msgs) != 0 {
		t.Errorf("expected no notifications, got %d", len(msgs))
	}
}

func TestMarkFailedRetryExhaustedEscalates(t *testing.T) {
	// MarkFailed always fails — should exhaust retries, escalate, and notify.
	var attempts atomic.Int32
	rt := &mockRuntime{
		sessions: map[string]session.SessionData{},
		markFailedFn: func(_, _ string) error {
			attempts.Add(1)
			return errors.New("persistent error")
		},
	}
	bus := newMockBus()
	notif := &mockNotifier{}
	a := New(rt, bus, nil)
	a.SetNotifier(notif)

	a.markFailedWithRetry("sess-fail", "session stuck")

	// Should have attempted exactly markFailedRetries times.
	if got := attempts.Load(); got != int32(markFailedRetries) {
		t.Errorf("expected %d attempts, got %d", markFailedRetries, got)
	}

	// Should have published an escalation event.
	events := bus.publishedEvents()
	escalations := 0
	for _, ev := range events {
		if ev.Type == hooks.EventHandoffRequired {
			escalations++
			reason, _ := ev.Payload["reason"].(string)
			if !strings.Contains(reason, "CRITICAL") || !strings.Contains(reason, "sess-fail") {
				t.Errorf("escalation reason missing expected content: %q", reason)
			}
		}
	}
	if escalations != 1 {
		t.Errorf("expected 1 escalation event, got %d", escalations)
	}

	// Should have sent a notification.
	msgs := notif.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0], "CRITICAL") || !strings.Contains(msgs[0], "sess-fail") {
		t.Errorf("notification message missing expected content: %q", msgs[0])
	}
}

func TestMarkFailedRetryExhaustedNoNotifierNosPanic(t *testing.T) {
	// MarkFailed always fails, no notifier set — should not panic.
	rt := &mockRuntime{
		sessions: map[string]session.SessionData{},
		markFailedFn: func(_, _ string) error {
			return errors.New("fail")
		},
	}
	bus := newMockBus()
	a := New(rt, bus, nil)
	// No notifier set — should still escalate via bus without panic.

	a.markFailedWithRetry("sess-no-notifier", "reason")

	events := bus.publishedEvents()
	if len(events) != 1 || events[0].Type != hooks.EventHandoffRequired {
		t.Errorf("expected 1 escalation event, got %d events", len(events))
	}
}

func TestApplyDecisionFAILUsesRetry(t *testing.T) {
	// Verify that applyDecision("FAIL ...") goes through markFailedWithRetry.
	var attempts atomic.Int32
	rt := &mockRuntime{
		sessions: map[string]session.SessionData{},
		markFailedFn: func(_, _ string) error {
			attempts.Add(1)
			return errors.New("transient")
		},
	}
	bus := newMockBus()
	a := New(rt, bus, nil)

	a.applyDecision(context.Background(), "sess-1", "FAIL something broke")

	// Should have retried markFailedRetries times.
	if got := attempts.Load(); got != int32(markFailedRetries) {
		t.Errorf("expected %d MarkFailed attempts, got %d", markFailedRetries, got)
	}

	// Should have escalated after exhaustion.
	events := bus.publishedEvents()
	found := false
	for _, ev := range events {
		if ev.Type == hooks.EventHandoffRequired {
			found = true
		}
	}
	if !found {
		t.Error("expected escalation event after retry exhaustion")
	}
}
