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

func TestFormatCycleNotification_Success(t *testing.T) {
	result := &kerneldomain.CycleResult{
		CycleID:    "cycle-1",
		KernelID:   "default",
		Status:     kerneldomain.CycleSuccess,
		Dispatched: 2,
		Succeeded:  2,
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-a", Status: kerneldomain.DispatchDone, Summary: "已完成 A"},
			{AgentID: "agent-b", Status: kerneldomain.DispatchDone, Summary: "已完成 B"},
		},
		Duration: 3200 * time.Millisecond,
	}
	got := FormatCycleNotification("default", result, nil)
	want := "Kernel[default] 周期完成总结\n- cycle_id: cycle-1\n- 状态: success\n- 任务总数: 2\n- 已完成: 2\n- 失败: 0\n- 完成率: 100.0%\n- 失败任务: (none)\n- 主动性: actionable=2/2, auto_recovered=0, blocked_awaiting_input=0, blocked_no_action=0\n- 执行总结:\n  - [agent-a|done] 已完成 A\n  - [agent-b|done] 已完成 B\n- 耗时: 3.2s"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCycleNotification_PartialFailure(t *testing.T) {
	result := &kerneldomain.CycleResult{
		CycleID:      "cycle-2",
		KernelID:     "default",
		Status:       kerneldomain.CyclePartialSuccess,
		Dispatched:   3,
		Succeeded:    2,
		Failed:       1,
		FailedAgents: []string{"agent-b"},
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-a", Status: kerneldomain.DispatchDone, Summary: "修复配置"},
			{AgentID: "agent-b", Status: kerneldomain.DispatchFailed, Error: "rate limit"},
		},
		Duration: 5100 * time.Millisecond,
	}
	got := FormatCycleNotification("default", result, nil)
	want := "Kernel[default] 周期完成总结\n- cycle_id: cycle-2\n- 状态: partial_success\n- 任务总数: 3\n- 已完成: 2\n- 失败: 1\n- 完成率: 66.7%\n- 失败任务: agent-b\n- 主动性: actionable=1/2, auto_recovered=0, blocked_awaiting_input=0, blocked_no_action=0\n- 执行总结:\n  - [agent-a|done] 修复配置\n  - [agent-b|failed] rate limit\n- 耗时: 5.1s"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCycleNotification_AllFailed(t *testing.T) {
	result := &kerneldomain.CycleResult{
		CycleID:      "cycle-3",
		KernelID:     "default",
		Status:       kerneldomain.CycleFailed,
		Dispatched:   2,
		Failed:       2,
		FailedAgents: []string{"agent-a", "agent-b"},
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-a", Status: kerneldomain.DispatchFailed, Error: "a failed"},
			{AgentID: "agent-b", Status: kerneldomain.DispatchFailed, Error: "b failed"},
		},
		Duration: 1500 * time.Millisecond,
	}
	got := FormatCycleNotification("default", result, nil)
	want := "Kernel[default] 周期完成总结\n- cycle_id: cycle-3\n- 状态: failed\n- 任务总数: 2\n- 已完成: 0\n- 失败: 2\n- 完成率: 0.0%\n- 失败任务: agent-a,agent-b\n- 主动性: actionable=0/2, auto_recovered=0, blocked_awaiting_input=0, blocked_no_action=0\n- 执行总结:\n  - [agent-a|failed] a failed\n  - [agent-b|failed] b failed\n- 耗时: 1.5s"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCycleNotification_CycleError(t *testing.T) {
	got := FormatCycleNotification("default", nil, fmt.Errorf("read state: file not found"))
	want := "Kernel[default] 周期异常\n- 错误: read state: file not found"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCycleNotification_AutonomySignals(t *testing.T) {
	result := &kerneldomain.CycleResult{
		CycleID:    "cycle-4",
		KernelID:   "default",
		Status:     kerneldomain.CyclePartialSuccess,
		Dispatched: 3,
		Succeeded:  1,
		Failed:     2,
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{
				AgentID: "agent-a",
				Status:  kerneldomain.DispatchDone,
				Summary: "[autonomy=actionable, attempts=2, recovered_from=awaiting_input] 已执行",
			},
			{
				AgentID: "agent-b",
				Status:  kerneldomain.DispatchFailed,
				Error:   errKernelAwaitingUserConfirmation.Error(),
			},
			{
				AgentID: "agent-c",
				Status:  kerneldomain.DispatchFailed,
				Error:   errKernelNoRealToolAction.Error(),
			},
		},
		Duration: 2500 * time.Millisecond,
	}

	got := FormatCycleNotification("default", result, nil)
	wantLine := "- 主动性: actionable=1/3, auto_recovered=1, blocked_awaiting_input=1, blocked_no_action=1"
	if !strings.Contains(got, wantLine) {
		t.Fatalf("expected autonomy signals line %q in notification:\n%s", wantLine, got)
	}
}

// ---------------------------------------------------------------------------
// FormatAggregatedSummary tests
// ---------------------------------------------------------------------------

func TestFormatAggregatedSummary_AllSuccess(t *testing.T) {
	s := WindowSummary{
		WindowStart:    time.Date(2026, 2, 24, 15, 0, 0, 0, time.UTC),
		WindowEnd:      time.Date(2026, 2, 24, 18, 0, 0, 0, time.UTC),
		TotalCycles:    6,
		TotalDispatched: 12,
		TotalSucceeded:  12,
		UniqueAgents:   []string{"founder-operator"},
	}
	got := FormatAggregatedSummary("default", s)
	want := "Kernel[default] 阶段总结 (15:00–18:00)\n- 周期: 6, 全部成功\n- 任务: 12 dispatched, 12 succeeded (100%)\n- 代理: founder-operator"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatAggregatedSummary_WithFailures(t *testing.T) {
	s := WindowSummary{
		WindowStart:      time.Date(2026, 2, 24, 9, 0, 0, 0, time.UTC),
		WindowEnd:        time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC),
		TotalCycles:      6,
		TotalDispatched:  12,
		TotalSucceeded:   10,
		TotalFailed:      2,
		ImmediateAlerts:  1,
		NoteworthyEvents: []string{"[cyc-3] failed — agent-b — rate limit"},
		UniqueAgents:     []string{"agent-a", "agent-b"},
	}
	got := FormatAggregatedSummary("default", s)
	if !strings.Contains(got, "10 成功, 2 失败") {
		t.Errorf("expected failure counts in summary:\n%s", got)
	}
	if !strings.Contains(got, "即时告警: 1") {
		t.Errorf("expected immediate alerts count:\n%s", got)
	}
	if !strings.Contains(got, "异常事件:") {
		t.Errorf("expected noteworthy events section:\n%s", got)
	}
	if !strings.Contains(got, "agent-a, agent-b") {
		t.Errorf("expected agents list:\n%s", got)
	}
}

func TestFormatAggregatedSummary_EmptyWindow(t *testing.T) {
	s := WindowSummary{
		WindowStart:    time.Date(2026, 2, 24, 3, 0, 0, 0, time.UTC),
		WindowEnd:      time.Date(2026, 2, 24, 6, 0, 0, 0, time.UTC),
		TotalCycles:    0,
		TotalDispatched: 0,
		TotalSucceeded:  0,
	}
	got := FormatAggregatedSummary("default", s)
	if !strings.Contains(got, "周期: 0") {
		t.Errorf("expected zero cycles:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// FormatImmediateAlert tests
// ---------------------------------------------------------------------------

func TestFormatImmediateAlert_Error(t *testing.T) {
	got := FormatImmediateAlert("default", nil, fmt.Errorf("state read failed"))
	want := "Kernel[default] ⚠ 异常\n- 错误: state read failed"
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
	if !strings.Contains(got, "⚠ 异常") {
		t.Errorf("expected alert header:\n%s", got)
	}
	if !strings.Contains(got, "cycle: abc123") {
		t.Errorf("expected cycle id:\n%s", got)
	}
	if !strings.Contains(got, "agent-b — rate limit exceeded") {
		t.Errorf("expected failed agent detail:\n%s", got)
	}
	if !strings.Contains(got, "耗时: 5.1s") {
		t.Errorf("expected duration:\n%s", got)
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
	if !strings.Contains(got, "partial_success") {
		t.Errorf("expected partial_success status:\n%s", got)
	}
	if !strings.Contains(got, "agent-b — timeout") {
		t.Errorf("expected agent-b failure:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// CycleAggregator behavioral tests
// ---------------------------------------------------------------------------

// testSender collects all sent texts for assertion.
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
		CycleID:    cycleID,
		Status:     kerneldomain.CycleFailed,
		Dispatched: 1,
		Failed:     1,
		FailedAgents: []string{"agent-b"},
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-b", Status: kerneldomain.DispatchFailed, Error: "rate limit"},
		},
		Duration: 5 * time.Second,
	}
}

func TestCycleAggregator_BuffersRoutineSuccess(t *testing.T) {
	sender := &testSender{}
	agg := NewCycleAggregator("default", 3*time.Hour, sender.send)
	// Use a fixed clock so we stay within the window.
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
	agg := NewCycleAggregator("default", 3*time.Hour, sender.send)
	baseTime := time.Date(2026, 2, 24, 15, 0, 0, 0, time.UTC)
	agg.now = func() time.Time { return baseTime }
	defer agg.Close(context.Background())

	ctx := context.Background()
	// First a routine success — no send.
	agg.HandleCycle(ctx, makeRoutineResult("c1"), nil)
	if sender.count() != 0 {
		t.Fatalf("expected 0 sends after routine cycle, got %d", sender.count())
	}

	// Then a failure — immediate send.
	agg.HandleCycle(ctx, makeFailedResult("c2"), nil)
	if sender.count() != 1 {
		t.Fatalf("expected 1 immediate send on failure, got %d", sender.count())
	}
	if !strings.Contains(sender.last(), "⚠ 异常") {
		t.Errorf("expected immediate alert format, got:\n%s", sender.last())
	}

	// Then an error — immediate send.
	agg.HandleCycle(ctx, nil, fmt.Errorf("boom"))
	if sender.count() != 2 {
		t.Fatalf("expected 2 immediate sends, got %d", sender.count())
	}
}

func TestCycleAggregator_FlushOnWindowExpiry(t *testing.T) {
	sender := &testSender{}
	agg := NewCycleAggregator("default", 3*time.Hour, sender.send)

	// Control the clock: start at 15:00.
	currentTime := time.Date(2026, 2, 24, 15, 0, 0, 0, time.UTC)
	agg.now = func() time.Time { return currentTime }

	ctx := context.Background()

	// Feed 3 routine cycles (still in window).
	agg.HandleCycle(ctx, makeRoutineResult("c1"), nil)
	currentTime = currentTime.Add(30 * time.Minute)
	agg.HandleCycle(ctx, makeRoutineResult("c2"), nil)
	currentTime = currentTime.Add(30 * time.Minute)
	agg.HandleCycle(ctx, makeRoutineResult("c3"), nil)

	if sender.count() != 0 {
		t.Fatalf("expected 0 sends within window, got %d", sender.count())
	}

	// Advance past the 3-hour window.
	currentTime = currentTime.Add(2*time.Hour + 1*time.Minute)
	agg.HandleCycle(ctx, makeRoutineResult("c4"), nil)

	// The window check in HandleCycle should have triggered a flush.
	if sender.count() < 1 {
		t.Fatalf("expected at least 1 aggregated summary after window expiry, got %d", sender.count())
	}
	lastText := sender.all()[0]
	if !strings.Contains(lastText, "阶段总结") {
		t.Errorf("expected aggregated summary format, got:\n%s", lastText)
	}
	if !strings.Contains(lastText, "周期:") {
		t.Errorf("expected cycle count in summary:\n%s", lastText)
	}
}

func TestCycleAggregator_CloseFlushes(t *testing.T) {
	sender := &testSender{}
	agg := NewCycleAggregator("default", 3*time.Hour, sender.send)
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
	if !strings.Contains(text, "阶段总结") {
		t.Errorf("expected aggregated summary on Close, got:\n%s", text)
	}
	if !strings.Contains(text, "周期: 2") {
		t.Errorf("expected 2 cycles in Close summary, got:\n%s", text)
	}
}

func TestCycleAggregator_CloseWithNoData(t *testing.T) {
	sender := &testSender{}
	agg := NewCycleAggregator("default", 3*time.Hour, sender.send)
	agg.Close(context.Background())

	if sender.count() != 0 {
		t.Fatalf("expected 0 sends on Close with no data, got %d", sender.count())
	}
}
