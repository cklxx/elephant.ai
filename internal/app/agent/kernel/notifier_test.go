package kernel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	kerneldomain "alex/internal/domain/kernel"
)

// ---------------------------------------------------------------------------
// FormatAggregatedSummary tests
// ---------------------------------------------------------------------------

func TestFormatAggregatedSummary_AllSuccess(t *testing.T) {
	s := WindowSummary{
		WindowStart:     time.Date(2026, 2, 24, 15, 0, 0, 0, time.UTC),
		WindowEnd:       time.Date(2026, 2, 24, 15, 30, 0, 0, time.UTC),
		TotalCycles:     1,
		TotalDispatched: 2,
		TotalSucceeded:  2,
		UniqueAgents:    []string{"founder-operator"},
	}
	got := FormatAggregatedSummary("default", s)
	want := "Kernel[default] Summary (15:00–15:30)\n- cycles: 1, all ok\n- tasks: 2 dispatched, 2 succeeded (100%)\n- agents: founder-operator"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatAggregatedSummary_WithFailures(t *testing.T) {
	s := WindowSummary{
		WindowStart:      time.Date(2026, 2, 24, 9, 0, 0, 0, time.UTC),
		WindowEnd:        time.Date(2026, 2, 24, 9, 30, 0, 0, time.UTC),
		TotalCycles:      2,
		TotalDispatched:  4,
		TotalSucceeded:   3,
		TotalFailed:      1,
		ImmediateAlerts:  1,
		NoteworthyEvents: []string{"[cyc-3] failed — agent-b — rate limit"},
		UniqueAgents:     []string{"agent-a", "agent-b"},
	}
	got := FormatAggregatedSummary("default", s)
	for _, want := range []string{"3 ok, 1 failed", "alerts sent: 1", "events:", "agent-a, agent-b"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in:\n%s", want, got)
		}
	}
}

func TestFormatAggregatedSummary_EmptyWindow(t *testing.T) {
	s := WindowSummary{
		WindowStart: time.Date(2026, 2, 24, 3, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 2, 24, 3, 30, 0, 0, time.UTC),
	}
	got := FormatAggregatedSummary("default", s)
	if !strings.Contains(got, "cycles: 0") {
		t.Errorf("expected zero cycles:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// FormatImmediateAlert tests
// ---------------------------------------------------------------------------

func TestFormatImmediateAlert_Error(t *testing.T) {
	got := FormatImmediateAlert("default", nil, fmt.Errorf("state read failed"))
	want := "Kernel[default] ⚠ Alert\n- error: state read failed"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatImmediateAlert_CycleFailed(t *testing.T) {
	result := &kerneldomain.CycleResult{
		CycleID:    "abc123",
		Status:     kerneldomain.CycleFailed,
		Dispatched: 1,
		Failed:     1,
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-b", Status: kerneldomain.DispatchFailed, Error: "rate limit exceeded"},
		},
		Duration: 5100 * time.Millisecond,
	}
	got := FormatImmediateAlert("default", result, nil)
	for _, want := range []string{"⚠ Alert", "cycle: abc123", "agent-b — rate limit exceeded", "elapsed: 5.1s"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in:\n%s", want, got)
		}
	}
}

func TestFormatImmediateAlert_PartialSuccess(t *testing.T) {
	result := &kerneldomain.CycleResult{
		CycleID:    "cyc-ps",
		Status:     kerneldomain.CyclePartialSuccess,
		Dispatched: 2,
		Succeeded:  1,
		Failed:     1,
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-a", Status: kerneldomain.DispatchDone, Summary: "ok"},
			{AgentID: "agent-b", Status: kerneldomain.DispatchFailed, Error: "timeout"},
		},
		Duration: 3 * time.Second,
	}
	got := FormatImmediateAlert("default", result, nil)
	for _, want := range []string{"partial_success", "agent-b — timeout"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in:\n%s", want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// CycleAggregator behavioral tests
// ---------------------------------------------------------------------------

type testSender struct {
	mu    sync.Mutex
	texts []string
}

func (s *testSender) send(_ context.Context, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.texts = append(s.texts, text)
}

func (s *testSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.texts)
}

func (s *testSender) last() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.texts) == 0 {
		return ""
	}
	return s.texts[len(s.texts)-1]
}

func (s *testSender) all() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, len(s.texts))
	copy(cp, s.texts)
	return cp
}

func makeRoutineResult(cycleID string) *kerneldomain.CycleResult {
	return &kerneldomain.CycleResult{
		CycleID:    cycleID,
		Status:     kerneldomain.CycleSuccess,
		Dispatched: 2,
		Succeeded:  2,
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "founder-operator", Status: kerneldomain.DispatchDone, Summary: "done"},
		},
		Duration: 10 * time.Second,
	}
}

func makeFailedResult(cycleID string) *kerneldomain.CycleResult {
	return &kerneldomain.CycleResult{
		CycleID:      cycleID,
		Status:       kerneldomain.CycleFailed,
		Dispatched:   1,
		Failed:       1,
		FailedAgents: []string{"agent-b"},
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-b", Status: kerneldomain.DispatchFailed, Error: "rate limit"},
		},
		Duration: 5 * time.Second,
	}
}

func TestCycleAggregator_BuffersRoutineSuccess(t *testing.T) {
	sender := &testSender{}
	agg := NewCycleAggregator("default", 30*time.Minute, sender.send)
	baseTime := time.Date(2026, 2, 24, 15, 0, 0, 0, time.UTC)
	agg.now = func() time.Time { return baseTime }
	defer agg.Close(context.Background())

	ctx := context.Background()
	agg.HandleCycle(ctx, makeRoutineResult("c1"), nil)
	agg.HandleCycle(ctx, makeRoutineResult("c2"), nil)
	agg.HandleCycle(ctx, makeRoutineResult("c3"), nil)

	if sender.count() != 0 {
		t.Fatalf("expected 0 sends for routine successes within window, got %d", sender.count())
	}
}

func TestCycleAggregator_ImmediateOnFailure(t *testing.T) {
	sender := &testSender{}
	agg := NewCycleAggregator("default", 30*time.Minute, sender.send)
	baseTime := time.Date(2026, 2, 24, 15, 0, 0, 0, time.UTC)
	agg.now = func() time.Time { return baseTime }
	defer agg.Close(context.Background())

	ctx := context.Background()
	agg.HandleCycle(ctx, makeRoutineResult("c1"), nil)
	if sender.count() != 0 {
		t.Fatalf("expected 0 sends after routine cycle, got %d", sender.count())
	}

	agg.HandleCycle(ctx, makeFailedResult("c2"), nil)
	if sender.count() != 1 {
		t.Fatalf("expected 1 immediate send on failure, got %d", sender.count())
	}
	if !strings.Contains(sender.last(), "⚠ Alert") {
		t.Errorf("expected immediate alert format, got:\n%s", sender.last())
	}

	agg.HandleCycle(ctx, nil, fmt.Errorf("boom"))
	if sender.count() != 2 {
		t.Fatalf("expected 2 immediate sends, got %d", sender.count())
	}
}

func TestCycleAggregator_FlushOnWindowExpiry(t *testing.T) {
	sender := &testSender{}
	agg := NewCycleAggregator("default", 30*time.Minute, sender.send)

	currentTime := time.Date(2026, 2, 24, 15, 0, 0, 0, time.UTC)
	agg.now = func() time.Time { return currentTime }

	ctx := context.Background()
	agg.HandleCycle(ctx, makeRoutineResult("c1"), nil)

	if sender.count() != 0 {
		t.Fatalf("expected 0 sends within window, got %d", sender.count())
	}

	// Advance past the 30-minute window.
	currentTime = currentTime.Add(31 * time.Minute)
	agg.HandleCycle(ctx, makeRoutineResult("c2"), nil)

	if sender.count() < 1 {
		t.Fatalf("expected at least 1 aggregated summary after window expiry, got %d", sender.count())
	}
	text := sender.all()[0]
	if !strings.Contains(text, "Summary") {
		t.Errorf("expected aggregated summary format, got:\n%s", text)
	}
}

func TestCycleAggregator_CloseFlushes(t *testing.T) {
	sender := &testSender{}
	agg := NewCycleAggregator("default", 30*time.Minute, sender.send)
	baseTime := time.Date(2026, 2, 24, 15, 0, 0, 0, time.UTC)
	agg.now = func() time.Time { return baseTime }

	ctx := context.Background()
	agg.HandleCycle(ctx, makeRoutineResult("c1"), nil)
	agg.HandleCycle(ctx, makeRoutineResult("c2"), nil)

	if sender.count() != 0 {
		t.Fatalf("expected 0 sends before Close, got %d", sender.count())
	}

	agg.Close(ctx)

	if sender.count() != 1 {
		t.Fatalf("expected 1 send after Close (flush), got %d", sender.count())
	}
	text := sender.last()
	if !strings.Contains(text, "Summary") {
		t.Errorf("expected aggregated summary on Close, got:\n%s", text)
	}
	if !strings.Contains(text, "cycles: 2") {
		t.Errorf("expected 2 cycles in Close summary, got:\n%s", text)
	}
}

func TestCycleAggregator_CloseWithNoData(t *testing.T) {
	sender := &testSender{}
	agg := NewCycleAggregator("default", 30*time.Minute, sender.send)
	agg.Close(context.Background())

	if sender.count() != 0 {
		t.Fatalf("expected 0 sends on Close with no data, got %d", sender.count())
	}
}
