//go:build integration

package leader

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/runtime/session"
)

type integrationPane struct {
	mu       sync.Mutex
	injected []string
}

func (p *integrationPane) record(text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.injected = append(p.injected, text)
}

func (p *integrationPane) injectedTexts() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.injected))
	copy(out, p.injected)
	return out
}

type toolCallSummary struct {
	name   string
	args   string
	errStr string
	ok     bool
}

type integrationRuntime struct {
	mu             sync.RWMutex
	sessions       map[string]*session.Session
	pane           *integrationPane
	failures       map[string]string
	recentToolCall map[string]toolCallSummary
	iterations     map[string]int
	recentEvents   map[string][]string
}

func newIntegrationRuntime() *integrationRuntime {
	return &integrationRuntime{
		sessions:       make(map[string]*session.Session),
		pane:           &integrationPane{},
		failures:       make(map[string]string),
		recentToolCall: make(map[string]toolCallSummary),
		iterations:     make(map[string]int),
		recentEvents:   make(map[string][]string),
	}
}

func (r *integrationRuntime) startSession(id, goal string, startedAt time.Time) {
	s := session.New(id, session.MemberClaudeCode, goal, "")
	if err := s.Transition(session.StateStarting); err != nil {
		panic(err)
	}
	if err := s.Transition(session.StateRunning); err != nil {
		panic(err)
	}
	s.StartedAt = &startedAt

	r.mu.Lock()
	r.sessions[id] = s
	r.mu.Unlock()
}

func (r *integrationRuntime) setRecentToolCall(id, name, args, errStr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recentToolCall[id] = toolCallSummary{
		name:   name,
		args:   args,
		errStr: errStr,
		ok:     true,
	}
}

func (r *integrationRuntime) setIterationCount(id string, count int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.iterations[id] = count
}

func (r *integrationRuntime) setRecentEvents(id string, events ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(events))
	copy(out, events)
	r.recentEvents[id] = out
}

func (r *integrationRuntime) GetSession(id string) (session.SessionData, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	if !ok {
		return session.SessionData{}, false
	}
	return s.Snapshot(), true
}

func (r *integrationRuntime) InjectText(_ context.Context, id, text string) error {
	r.pane.record(id + "|" + text)
	return nil
}

func (r *integrationRuntime) MarkFailed(id, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures[id] = errMsg
	return nil
}

func (r *integrationRuntime) GetRecentEvents(id string, n int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	events := r.recentEvents[id]
	if n <= 0 || len(events) <= n {
		out := make([]string, len(events))
		copy(out, events)
		return out
	}
	out := make([]string, n)
	copy(out, events[len(events)-n:])
	return out
}

func (r *integrationRuntime) GetRecentToolCall(id string) (name, args, errStr string, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	call, found := r.recentToolCall[id]
	if !found {
		return "", "", "", false
	}
	return call.name, call.args, call.errStr, call.ok
}

func (r *integrationRuntime) GetIterationCount(id string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.iterations[id]
}

type executeCall struct {
	prompt    string
	sessionID string
}

type executeRecorder struct {
	mu        sync.Mutex
	calls     []executeCall
	responses []string
}

func newExecuteRecorder(responses ...string) *executeRecorder {
	return &executeRecorder{responses: responses}
}

func (r *executeRecorder) execute(_ context.Context, prompt, sessionID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, executeCall{prompt: prompt, sessionID: sessionID})
	idx := len(r.calls) - 1
	if idx < len(r.responses) {
		return r.responses[idx], nil
	}
	return "ESCALATE", nil
}

func (r *executeRecorder) snapshot() []executeCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]executeCall, len(r.calls))
	copy(out, r.calls)
	return out
}

func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func waitForEvent(t *testing.T, ch <-chan hooks.Event, want hooks.EventType) hooks.Event {
	t.Helper()

	deadline := time.After(time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Type == want {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", want)
			return hooks.Event{}
		}
	}
}

func currentStallCount(a *Agent, sessionID string) int {
	a.stallCountsMu.Lock()
	defer a.stallCountsMu.Unlock()
	return a.stallCounts[sessionID]
}

func TestRichStallPrompt_RetryTool(t *testing.T) {
	rt := newIntegrationRuntime()
	rt.setRecentToolCall("session-1", "bash", "ls -la", "permission denied")
	rt.setIterationCount("session-1", 5)
	rt.startSession("session-1", "inspect repository state", time.Now().Add(-3*time.Minute))

	recorder := newExecuteRecorder("RETRY_TOOL bash")
	bus := hooks.NewInProcessBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go New(rt, bus, recorder.execute).Run(ctx)
	time.Sleep(10 * time.Millisecond)

	bus.Publish("session-1", hooks.Event{
		Type:      hooks.EventStalled,
		SessionID: "session-1",
		At:        time.Now(),
	})

	waitForCondition(t, time.Second, func() bool {
		return len(recorder.snapshot()) == 1
	})

	call := recorder.snapshot()[0]
	if call.sessionID != stallSessionID("session-1") {
		t.Fatalf("sessionID = %q, want %q", call.sessionID, stallSessionID("session-1"))
	}
	for _, want := range []string{
		"Last tool call: bash(ls -la)",
		"Last error:     permission denied",
		"Iteration:      5 tool calls so far",
		"RETRY_TOOL <tool_name>",
	} {
		if !strings.Contains(call.prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, call.prompt)
		}
	}

	waitForCondition(t, time.Second, func() bool {
		return len(rt.pane.injectedTexts()) == 1
	})

	injected := rt.pane.injectedTexts()[0]
	if !strings.Contains(injected, "重试工具 bash") {
		t.Fatalf("retry injection missing tool name: %q", injected)
	}
	if !strings.Contains(injected, "permission denied") {
		t.Fatalf("retry injection missing tool error: %q", injected)
	}
}

func TestEnhancedHandoff_WithDiagnostics(t *testing.T) {
	rt := newIntegrationRuntime()
	rt.startSession("session-2", "repair stuck migration", time.Now().Add(-5*time.Minute))
	rt.setRecentToolCall("session-2", "bash", "go test ./...", "permission denied")
	rt.setRecentEvents(
		"session-2",
		"heartbeat: planner started",
		"tool: bash go test ./...",
		"error: permission denied",
	)

	recorder := newExecuteRecorder(
		"INJECT keep trying",
		"INJECT inspect the failing command",
		"INJECT collect more diagnostics",
	)
	bus := hooks.NewInProcessBus()
	handoffCh, cancelHandoff := bus.Subscribe("session-2")
	defer cancelHandoff()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(rt, bus, recorder.execute).Run(ctx)
	time.Sleep(10 * time.Millisecond)

	for i := 0; i <= maxStallAttempts; i++ {
		bus.Publish("session-2", hooks.Event{
			Type:      hooks.EventStalled,
			SessionID: "session-2",
			At:        time.Now(),
		})
		time.Sleep(80 * time.Millisecond)
	}

	handoff := waitForEvent(t, handoffCh, hooks.EventHandoffRequired)

	lastToolCall, ok := handoff.Payload["last_tool_call"].(string)
	if !ok || lastToolCall != "bash: go test ./..." {
		t.Fatalf("last_tool_call = %v, want %q", handoff.Payload["last_tool_call"], "bash: go test ./...")
	}
	lastError, ok := handoff.Payload["last_error"].(string)
	if !ok || lastError != "permission denied" {
		t.Fatalf("last_error = %v, want %q", handoff.Payload["last_error"], "permission denied")
	}
	tail, ok := handoff.Payload["session_tail"].([]string)
	if !ok {
		t.Fatalf("session_tail type = %T, want []string", handoff.Payload["session_tail"])
	}
	if len(tail) != 3 {
		t.Fatalf("session_tail len = %d, want 3", len(tail))
	}
	if tail[2] != "error: permission denied" {
		t.Fatalf("session_tail[2] = %q, want %q", tail[2], "error: permission denied")
	}
}

func TestChildCompleted_SiblingProgress(t *testing.T) {
	rt := newIntegrationRuntime()
	rt.startSession("parent-1", "coordinate sibling workers", time.Now().Add(-2*time.Minute))

	recorder := newExecuteRecorder(
		"INJECT summarize sibling progress",
		"INJECT summarize all sibling outputs",
	)
	bus := hooks.NewInProcessBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(rt, bus, recorder.execute).Run(ctx)
	time.Sleep(10 * time.Millisecond)

	bus.Publish("parent-1", hooks.Event{
		Type:      hooks.EventChildCompleted,
		SessionID: "parent-1",
		At:        time.Now(),
		Payload: map[string]any{
			"child_id":          "child-1",
			"child_goal":        "finish part 1",
			"child_answer":      "done",
			"sibling_total":     3,
			"sibling_completed": 1,
		},
	})

	waitForCondition(t, time.Second, func() bool {
		return len(recorder.snapshot()) >= 1
	})

	first := recorder.snapshot()[0]
	if first.sessionID != "parent-1" {
		t.Fatalf("first sessionID = %q, want parent-1", first.sessionID)
	}
	if !strings.Contains(first.prompt, "子任务进度: 1/3 已完成") {
		t.Fatalf("first prompt missing sibling progress: %s", first.prompt)
	}

	bus.Publish("parent-1", hooks.Event{
		Type:      hooks.EventChildCompleted,
		SessionID: "parent-1",
		At:        time.Now(),
		Payload: map[string]any{
			"child_id":          "child-3",
			"child_goal":        "finish part 3",
			"child_answer":      "done",
			"sibling_total":     3,
			"sibling_completed": 3,
		},
	})

	waitForCondition(t, time.Second, func() bool {
		return len(recorder.snapshot()) >= 2
	})

	second := recorder.snapshot()[1]
	if !strings.Contains(second.prompt, "子任务进度: 3/3 已完成") {
		t.Fatalf("second prompt missing all-done progress: %s", second.prompt)
	}
	if !strings.Contains(second.prompt, "所有子任务已完成，请汇总结果。") {
		t.Fatalf("second prompt missing summary hint: %s", second.prompt)
	}
}

func TestFullLifecycle_StallRecoveryAndEscalation(t *testing.T) {
	rt := newIntegrationRuntime()
	rt.startSession("session-4", "unstick multi-step repair", time.Now().Add(-10*time.Minute))
	rt.setRecentToolCall("session-4", "bash", "go test ./...", "permission denied")
	rt.setIterationCount("session-4", 5)
	rt.setRecentEvents("session-4", "tool: bash go test ./...", "error: permission denied", "waiting for next action")

	recorder := newExecuteRecorder(
		"INJECT continue from the last successful step",
		"RETRY_TOOL bash",
		"SWITCH_STRATEGY summarize blockers and re-plan",
		"INJECT final nudge before escalation",
	)
	bus := hooks.NewInProcessBus()
	handoffCh, cancelHandoff := bus.Subscribe("session-4")
	defer cancelHandoff()

	agent := New(rt, bus, recorder.execute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go agent.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	bus.Publish("session-4", hooks.Event{
		Type:      hooks.EventStalled,
		SessionID: "session-4",
		At:        time.Now(),
	})
	waitForCondition(t, time.Second, func() bool {
		return len(recorder.snapshot()) >= 1 && len(rt.pane.injectedTexts()) >= 1
	})

	initialHistory := agent.decisions.Get("session-4")
	waitForCondition(t, time.Second, func() bool {
		return initialHistory.Len() == 1
	})

	bus.Publish("session-4", hooks.Event{
		Type:      hooks.EventHeartbeat,
		SessionID: "session-4",
		At:        time.Now(),
	})
	waitForCondition(t, time.Second, func() bool {
		records := initialHistory.Last(1)
		return len(records) == 1 && records[0].Outcome == "recovered" && currentStallCount(agent, "session-4") == 0
	})

	waitForCondition(t, time.Second, func() bool {
		return agent.decisions.Get("session-4").Len() == 0
	})

	for i := 0; i < 3; i++ {
		bus.Publish("session-4", hooks.Event{
			Type:      hooks.EventStalled,
			SessionID: "session-4",
			At:        time.Now().Add(time.Duration(i) * 100 * time.Millisecond),
		})
		time.Sleep(80 * time.Millisecond)
	}

	waitForCondition(t, time.Second, func() bool {
		return len(recorder.snapshot()) == 4 && len(rt.pane.injectedTexts()) >= 4
	})

	bus.Publish("session-4", hooks.Event{
		Type:      hooks.EventStalled,
		SessionID: "session-4",
		At:        time.Now().Add(500 * time.Millisecond),
	})

	handoff := waitForEvent(t, handoffCh, hooks.EventHandoffRequired)
	if handoff.Type != hooks.EventHandoffRequired {
		t.Fatalf("expected handoff event, got %+v", handoff)
	}

	currentHistory := agent.decisions.Get("session-4").Last(3)
	if len(currentHistory) != 3 {
		t.Fatalf("current history len = %d, want 3", len(currentHistory))
	}

	if currentHistory[0].Action != "RETRY_TOOL" || currentHistory[0].Outcome != "still_stalled" {
		t.Fatalf("history[0] = %+v, want RETRY_TOOL/still_stalled", currentHistory[0])
	}
	if currentHistory[1].Action != "SWITCH_STRATEGY" || currentHistory[1].Outcome != "still_stalled" {
		t.Fatalf("history[1] = %+v, want SWITCH_STRATEGY/still_stalled", currentHistory[1])
	}
	if currentHistory[2].Action != "INJECT" || currentHistory[2].Outcome != "" {
		t.Fatalf("history[2] = %+v, want INJECT/pending", currentHistory[2])
	}

	injected := rt.pane.injectedTexts()
	if !strings.Contains(injected[1], "重试工具 bash") {
		t.Fatalf("retry tool injection missing: %q", injected[1])
	}
	if !strings.Contains(injected[2], "换一种方式完成任务") {
		t.Fatalf("switch strategy injection missing: %q", injected[2])
	}
}
