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

type leaderIntegrationPane struct {
	mu       sync.Mutex
	injected []string
}

func (p *leaderIntegrationPane) record(text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.injected = append(p.injected, text)
}

func (p *leaderIntegrationPane) injectedTexts() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.injected))
	copy(out, p.injected)
	return out
}

type toolCallSnapshot struct {
	tool   string
	input  string
	errMsg string
}

type leaderIntegrationRuntime struct {
	mu              sync.RWMutex
	sessions        map[string]*session.Session
	pane            *leaderIntegrationPane
	failures        map[string]string
	recentToolCalls map[string]toolCallSnapshot
	iterationCounts map[string]int
	recentEvents    map[string][]string
}

func newLeaderIntegrationRuntime() *leaderIntegrationRuntime {
	return &leaderIntegrationRuntime{
		sessions:        make(map[string]*session.Session),
		pane:            &leaderIntegrationPane{},
		failures:        make(map[string]string),
		recentToolCalls: make(map[string]toolCallSnapshot),
		iterationCounts: make(map[string]int),
		recentEvents:    make(map[string][]string),
	}
}

func (r *leaderIntegrationRuntime) startSession(id, goal string, startedAt time.Time) {
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

func (r *leaderIntegrationRuntime) setRecentToolCall(id, tool, input, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recentToolCalls[id] = toolCallSnapshot{
		tool:   tool,
		input:  input,
		errMsg: errMsg,
	}
}

func (r *leaderIntegrationRuntime) setIterationCount(id string, count int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.iterationCounts[id] = count
}

func (r *leaderIntegrationRuntime) setRecentEvents(id string, events ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := make([]string, len(events))
	copy(copied, events)
	r.recentEvents[id] = copied
}

func (r *leaderIntegrationRuntime) GetSession(id string) (session.SessionData, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	if !ok {
		return session.SessionData{}, false
	}
	return s.Snapshot(), true
}

func (r *leaderIntegrationRuntime) InjectText(_ context.Context, id, text string) error {
	r.pane.record(id + "|" + text)
	return nil
}

func (r *leaderIntegrationRuntime) MarkFailed(id, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures[id] = errMsg
	return nil
}

func (r *leaderIntegrationRuntime) GetRecentToolCall(id string) (string, string, string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot, ok := r.recentToolCalls[id]
	if !ok {
		return "", "", "", false
	}
	return snapshot.tool, snapshot.input, snapshot.errMsg, true
}

func (r *leaderIntegrationRuntime) GetIterationCount(id string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.iterationCounts[id]
}

func (r *leaderIntegrationRuntime) GetRecentEvents(id string, limit int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	events := r.recentEvents[id]
	if limit <= 0 || len(events) <= limit {
		out := make([]string, len(events))
		copy(out, events)
		return out
	}
	out := make([]string, limit)
	copy(out, events[len(events)-limit:])
	return out
}

type recentToolCallProvider interface {
	GetRecentToolCall(id string) (tool, input, errMsg string, ok bool)
}

type iterationCountProvider interface {
	GetIterationCount(id string) int
}

type recentEventsProvider interface {
	GetRecentEvents(id string, limit int) []string
}

func maybeRecentToolCall(rt any, sessionID string) (string, string, string, bool) {
	provider, ok := rt.(recentToolCallProvider)
	if !ok {
		return "", "", "", false
	}
	return provider.GetRecentToolCall(sessionID)
}

func maybeIterationCount(rt any, sessionID string) (int, bool) {
	provider, ok := rt.(iterationCountProvider)
	if !ok {
		return 0, false
	}
	return provider.GetIterationCount(sessionID), true
}

func maybeRecentEvents(rt any, sessionID string, limit int) ([]string, bool) {
	provider, ok := rt.(recentEventsProvider)
	if !ok {
		return nil, false
	}
	return provider.GetRecentEvents(sessionID, limit), true
}

func waitForLeaderEvent(t *testing.T, ch <-chan hooks.Event, want hooks.EventType) hooks.Event {
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

func waitForLeaderCondition(t *testing.T, timeout time.Duration, fn func() bool) {
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

func TestRichStallPrompt_RetryTool(t *testing.T) {
	t.Skip("waiting for implementation")

	rt := newLeaderIntegrationRuntime()
	rt.setRecentToolCall("session-1", "bash", "ls -la", "permission denied")
	rt.setIterationCount("session-1", 5)
	rt.startSession("session-1", "inspect repository state", time.Now().Add(-3*time.Minute))

	if tool, _, _, ok := maybeRecentToolCall(rt, "session-1"); !ok || tool != "bash" {
		t.Fatalf("recent tool call fixture not wired: ok=%v tool=%q", ok, tool)
	}
	if count, ok := maybeIterationCount(rt, "session-1"); !ok || count != 5 {
		t.Fatalf("iteration count fixture not wired: ok=%v count=%d", ok, count)
	}

	var (
		promptMu sync.Mutex
		prompt   string
	)
	execute := func(_ context.Context, gotPrompt, sessionID string) (string, error) {
		if sessionID != stallSessionID("session-1") {
			t.Fatalf("execute sessionID = %q, want %q", sessionID, stallSessionID("session-1"))
		}
		promptMu.Lock()
		prompt = gotPrompt
		promptMu.Unlock()
		return "RETRY_TOOL bash", nil
	}

	bus := hooks.NewInProcessBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(rt, bus, execute).Run(ctx)

	bus.Publish("session-1", hooks.Event{
		Type:      hooks.EventStalled,
		SessionID: "session-1",
		At:        time.Now(),
	})

	waitForLeaderCondition(t, time.Second, func() bool {
		promptMu.Lock()
		defer promptMu.Unlock()
		return prompt != ""
	})

	promptMu.Lock()
	capturedPrompt := prompt
	promptMu.Unlock()
	for _, want := range []string{"bash", "permission denied", "Iteration"} {
		if !strings.Contains(capturedPrompt, want) {
			t.Fatalf("prompt missing %q: %s", want, capturedPrompt)
		}
	}

	waitForLeaderCondition(t, time.Second, func() bool {
		return len(rt.pane.injectedTexts()) == 1
	})

	injected := rt.pane.injectedTexts()[0]
	if !strings.Contains(injected, "重试") || !strings.Contains(injected, "bash") {
		t.Fatalf("injected text missing retry guidance: %q", injected)
	}
}

func TestEnhancedHandoff_WithDiagnostics(t *testing.T) {
	t.Skip("waiting for implementation")

	rt := newLeaderIntegrationRuntime()
	rt.startSession("session-2", "repair stuck migration", time.Now().Add(-5*time.Minute))
	rt.setRecentToolCall("session-2", "bash", "go test ./...", "permission denied")
	rt.setRecentEvents(
		"session-2",
		"heartbeat: planner started",
		"tool: bash go test ./...",
		"error: permission denied",
	)

	if events, ok := maybeRecentEvents(rt, "session-2", 3); !ok || len(events) != 3 {
		t.Fatalf("recent events fixture not wired: ok=%v len=%d", ok, len(events))
	}

	execute := func(_ context.Context, _ string, _ string) (string, error) {
		return "INJECT keep trying", nil
	}

	bus := hooks.NewInProcessBus()
	handoffCh, cancelHandoff := bus.Subscribe("session-2")
	defer cancelHandoff()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(rt, bus, execute).Run(ctx)

	for attempt := 0; attempt <= maxStallAttempts; attempt++ {
		bus.Publish("session-2", hooks.Event{
			Type:      hooks.EventStalled,
			SessionID: "session-2",
			At:        time.Now(),
		})
		time.Sleep(80 * time.Millisecond)
	}

	handoff := waitForLeaderEvent(t, handoffCh, hooks.EventHandoffRequired)

	for _, key := range []string{"last_tool_call", "last_error", "session_tail"} {
		if _, ok := handoff.Payload[key]; !ok {
			t.Fatalf("handoff payload missing %q: %+v", key, handoff.Payload)
		}
	}

	tail, ok := handoff.Payload["session_tail"].([]string)
	if !ok {
		t.Fatalf("handoff session_tail type = %T, want []string", handoff.Payload["session_tail"])
	}
	if len(tail) != 3 {
		t.Fatalf("handoff session_tail len = %d, want 3", len(tail))
	}
}

func TestChildCompleted_SiblingProgress(t *testing.T) {
	t.Skip("waiting for implementation")

	rt := newLeaderIntegrationRuntime()
	rt.startSession("parent-1", "coordinate sibling workers", time.Now().Add(-2*time.Minute))

	var (
		promptMu sync.Mutex
		prompts  []string
	)
	execute := func(_ context.Context, prompt, sessionID string) (string, error) {
		if sessionID != "parent-1" {
			t.Fatalf("execute sessionID = %q, want parent-1", sessionID)
		}
		promptMu.Lock()
		prompts = append(prompts, prompt)
		promptMu.Unlock()
		return "INJECT summarize sibling progress", nil
	}

	bus := hooks.NewInProcessBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(rt, bus, execute).Run(ctx)

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

	waitForLeaderCondition(t, time.Second, func() bool {
		promptMu.Lock()
		defer promptMu.Unlock()
		return len(prompts) >= 1
	})

	promptMu.Lock()
	firstPrompt := prompts[0]
	promptMu.Unlock()
	if !strings.Contains(firstPrompt, "1/3") && !strings.Contains(firstPrompt, "1 of 3") {
		t.Fatalf("first prompt missing sibling progress: %s", firstPrompt)
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

	waitForLeaderCondition(t, time.Second, func() bool {
		promptMu.Lock()
		defer promptMu.Unlock()
		return len(prompts) >= 2
	})

	promptMu.Lock()
	secondPrompt := prompts[1]
	promptMu.Unlock()
	if !strings.Contains(secondPrompt, "所有子任务") && !strings.Contains(secondPrompt, "汇总") {
		t.Fatalf("second prompt missing completion summary cue: %s", secondPrompt)
	}
}

func TestFullLifecycle_StallRecoveryAndEscalation(t *testing.T) {
	t.Skip("waiting for implementation")

	rt := newLeaderIntegrationRuntime()
	rt.setRecentToolCall("session-4", "bash", "go test ./...", "permission denied")
	rt.setIterationCount("session-4", 5)
	rt.startSession("session-4", "unstick multi-step repair", time.Now().Add(-10*time.Minute))

	var (
		decisionMu sync.Mutex
		decisions  []string
	)
	queue := []string{
		"INJECT continue from the last successful step",
		"RETRY_TOOL bash",
		"SWITCH_STRATEGY summarize blockers and re-plan",
	}
	execute := func(_ context.Context, prompt, sessionID string) (string, error) {
		if sessionID != stallSessionID("session-4") {
			t.Fatalf("execute sessionID = %q, want %q", sessionID, stallSessionID("session-4"))
		}
		decisionMu.Lock()
		defer decisionMu.Unlock()
		decisions = append(decisions, prompt)
		if len(decisions) > len(queue) {
			return "ESCALATE", nil
		}
		return queue[len(decisions)-1], nil
	}

	bus := hooks.NewInProcessBus()
	handoffCh, cancelHandoff := bus.Subscribe("session-4")
	defer cancelHandoff()

	agent := New(rt, bus, execute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go agent.Run(ctx)

	bus.Publish("session-4", hooks.Event{
		Type:      hooks.EventStalled,
		SessionID: "session-4",
		At:        time.Now(),
	})
	waitForLeaderCondition(t, time.Second, func() bool {
		return len(rt.pane.injectedTexts()) >= 1
	})

	bus.Publish("session-4", hooks.Event{
		Type:      hooks.EventHeartbeat,
		SessionID: "session-4",
		At:        time.Now(),
	})
	time.Sleep(80 * time.Millisecond)

	bus.Publish("session-4", hooks.Event{
		Type:      hooks.EventStalled,
		SessionID: "session-4",
		At:        time.Now(),
	})
	time.Sleep(80 * time.Millisecond)
	bus.Publish("session-4", hooks.Event{
		Type:      hooks.EventStalled,
		SessionID: "session-4",
		At:        time.Now().Add(100 * time.Millisecond),
	})
	time.Sleep(80 * time.Millisecond)
	bus.Publish("session-4", hooks.Event{
		Type:      hooks.EventStalled,
		SessionID: "session-4",
		At:        time.Now().Add(200 * time.Millisecond),
	})

	handoff := waitForLeaderEvent(t, handoffCh, hooks.EventHandoffRequired)
	if handoff.Type != hooks.EventHandoffRequired {
		t.Fatalf("expected handoff event, got %+v", handoff)
	}

	history := agent.decisions.Get("session-4").Last(3)
	if len(history) != 3 {
		t.Fatalf("decision history len = %d, want 3", len(history))
	}

	gotOutcomes := []string{history[0].Outcome, history[1].Outcome, history[2].Outcome}
	wantOutcomes := []string{"recovered", "still_stalled", "still_stalled"}
	for i := range wantOutcomes {
		if gotOutcomes[i] != wantOutcomes[i] {
			t.Fatalf("history[%d].Outcome = %q, want %q", i, gotOutcomes[i], wantOutcomes[i])
		}
	}
}
