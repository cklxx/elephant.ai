package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/uxphrases"
)

// --- test helpers ---

// testClock provides a thread-safe controllable clock for tests.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock(t time.Time) *testClock {
	return &testClock{now: t}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type spySender struct {
	mu      sync.Mutex
	sends   []string // text payloads from SendProgress
	updates []struct {
		messageID string
		text      string
	}
	nextID string
	err    error
}

func (s *spySender) SendProgress(_ context.Context, text string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sends = append(s.sends, text)
	if s.err != nil {
		return "", s.err
	}
	id := s.nextID
	if id == "" {
		id = fmt.Sprintf("om_progress_%d", len(s.sends))
	}
	return id, nil
}

func (s *spySender) UpdateProgress(_ context.Context, messageID, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, struct {
		messageID string
		text      string
	}{messageID, text})
	return s.err
}

func (s *spySender) sendCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sends)
}

func (s *spySender) updateCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.updates)
}

func (s *spySender) lastSendText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sends) == 0 {
		return ""
	}
	return s.sends[len(s.sends)-1]
}

func (s *spySender) lastUpdateText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.updates) == 0 {
		return ""
	}
	return s.updates[len(s.updates)-1].text
}

type spyListener struct {
	mu     sync.Mutex
	events []agent.AgentEvent
}

func (l *spyListener) OnEvent(event agent.AgentEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *spyListener) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.events)
}

func makeToolStarted(callID, toolName string) *domain.Event {
	return domain.NewToolStartedEvent(
		domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		0, callID, toolName, nil,
	)
}

func makeToolCompleted(callID, toolName string, dur time.Duration, err error) *domain.Event {
	return domain.NewToolCompletedEvent(
		domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		callID, toolName, "", err, dur, nil, nil,
	)
}

func makeEnvelopeToolStarted(callID, toolName string) *domain.WorkflowEventEnvelope {
	return &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventToolStarted,
		NodeID:    callID,
		Payload: map[string]any{
			"call_id":   callID,
			"tool_name": toolName,
		},
	}
}

func makeEnvelopeToolCompleted(callID, toolName string, dur time.Duration, err error) *domain.WorkflowEventEnvelope {
	payload := map[string]any{
		"call_id":   callID,
		"tool_name": toolName,
		"duration":  dur.Milliseconds(),
		"result":    "ok",
	}
	if err != nil {
		payload["error"] = err.Error()
	}
	return &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventToolCompleted,
		NodeID:    callID,
		Payload:   payload,
	}
}

func makeNodeStarted(iteration int) *domain.Event {
	return domain.NewNodeStartedEvent(
		domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		iteration, 0, 0, "", nil, nil,
	)
}

func makeEnvelopeNodeStarted(iteration int) *domain.WorkflowEventEnvelope {
	return &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventNodeStarted,
		Payload: map[string]any{
			"iteration": iteration,
		},
	}
}

// --- tests ---

func TestProgressListenerForwardsEvents(t *testing.T) {
	inner := &spyListener{}
	sender := &spySender{nextID: "om_1"}
	pl := newProgressListener(context.Background(), inner, sender, nil)
	defer pl.Close()

	ev := makeToolStarted("call-1", "web_search")
	pl.OnEvent(ev)

	if inner.count() != 1 {
		t.Fatalf("expected 1 forwarded event, got %d", inner.count())
	}
}

func TestProgressListenerForwardsNonToolEvents(t *testing.T) {
	inner := &spyListener{}
	sender := &spySender{nextID: "om_1"}
	pl := newProgressListener(context.Background(), inner, sender, nil)
	defer pl.Close()

	// Send a non-tool event.
	ev := domain.NewInputReceivedEvent(
		agent.LevelCore, "sess", "run", "", "test", nil, time.Now(),
	)
	pl.OnEvent(ev)

	if inner.count() != 1 {
		t.Fatalf("expected 1 forwarded event, got %d", inner.count())
	}
	// No tool activity, so no sends should happen.
	time.Sleep(50 * time.Millisecond)
	if sender.sendCount() != 0 {
		t.Fatalf("expected 0 sends for non-tool event, got %d", sender.sendCount())
	}
}

func TestProgressListenerKeyToolSendsNewMessage(t *testing.T) {
	sender := &spySender{nextID: "om_progress"}
	pl := newProgressListener(context.Background(), nil, sender, nil)

	// Key tool → sends a new message (not edit-in-place).
	pl.OnEvent(makeToolStarted("call-1", "web_search"))
	time.Sleep(100 * time.Millisecond)

	if sender.sendCount() != 1 {
		t.Fatalf("expected 1 send, got %d", sender.sendCount())
	}
	// No updates — each progress is a separate new message.
	if sender.updateCount() != 0 {
		t.Fatalf("expected 0 updates (no edit-in-place), got %d", sender.updateCount())
	}
	// MessageID always returns empty (no edit-in-place).
	if id := pl.MessageID(); id != "" {
		t.Fatalf("expected empty MessageID, got %q", id)
	}
	pl.Close()
}

func TestProgressListenerNonKeyToolSilent(t *testing.T) {
	sender := &spySender{nextID: "om_nonkey"}
	pl := newProgressListener(context.Background(), nil, sender, nil)

	// Non-key tool → no progress message.
	pl.OnEvent(makeToolStarted("call-1", "read_file"))
	time.Sleep(100 * time.Millisecond)
	if sender.sendCount() != 0 {
		t.Fatalf("expected 0 sends for non-key tool, got %d", sender.sendCount())
	}
	pl.Close()
}

func TestProgressListenerMultipleKeyToolsEachGetMessage(t *testing.T) {
	clk := newTestClock(time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC))
	sender := &spySender{nextID: "om_multi"}
	pl := newProgressListener(context.Background(), nil, sender, nil)
	pl.now = clk.Now

	// First key tool.
	pl.OnEvent(makeToolStarted("call-1", "web_search"))
	time.Sleep(100 * time.Millisecond)
	if sender.sendCount() != 1 {
		t.Fatalf("expected 1 send, got %d", sender.sendCount())
	}

	// Second key tool after rate limit.
	clk.Advance(3 * time.Second)
	pl.OnEvent(makeToolStarted("call-2", "shell_exec"))
	time.Sleep(100 * time.Millisecond)

	// Should have 2 sends (each as a new message), 0 updates.
	if sender.sendCount() != 2 {
		t.Fatalf("expected 2 sends, got %d", sender.sendCount())
	}
	if sender.updateCount() != 0 {
		t.Fatalf("expected 0 updates, got %d", sender.updateCount())
	}
	pl.Close()
}

func TestProgressListenerDuplicateToolStart(t *testing.T) {
	sender := &spySender{nextID: "om_dup"}
	pl := newProgressListener(context.Background(), nil, sender, nil)
	defer pl.Close()

	pl.OnEvent(makeToolStarted("call-1", "web_search"))
	pl.OnEvent(makeToolStarted("call-1", "web_search")) // duplicate

	pl.mu.Lock()
	count := len(pl.tools)
	pl.mu.Unlock()

	if count != 1 {
		t.Fatalf("expected 1 tool entry, got %d", count)
	}
}

func TestProgressListenerBuildTextFormat(t *testing.T) {
	clk := newTestClock(time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC))
	pl := &progressListener{
		now:       clk.Now,
		toolIndex: make(map[string]*toolStatus),
	}

	// Empty tools → thinking phrase.
	text := pl.buildText()
	if !isNaturalThinkingPhrase(text) {
		t.Fatalf("expected thinking phrase, got %q", text)
	}

	now := clk.Now()
	pl.tools = []*toolStatus{
		{callID: "c1", toolName: "web_search", started: now.Add(-2 * time.Second), done: true},
		{callID: "c2", toolName: "seedream", started: now.Add(-3 * time.Second)}, // active key tool
	}

	text = pl.buildText()
	// No jargon in phrase.
	if strings.Contains(strings.ToLower(text), "search") || strings.Contains(strings.ToLower(text), "tool") {
		t.Fatalf("expected conversational phrase, got jargon: %q", text)
	}
	if strings.Contains(text, "\n") {
		t.Fatalf("expected single-line, got: %q", text)
	}
}

func TestProgressListenerNilInner(t *testing.T) {
	sender := &spySender{nextID: "om_nil"}
	pl := newProgressListener(context.Background(), nil, sender, nil)
	defer pl.Close()

	// Should not panic with nil inner listener.
	pl.OnEvent(makeToolStarted("call-1", "web_search"))
	time.Sleep(100 * time.Millisecond)
	if sender.sendCount() != 1 {
		t.Fatalf("expected 1 send, got %d", sender.sendCount())
	}
}

func TestProgressListenerMultipleCloseIdempotent(t *testing.T) {
	sender := &spySender{nextID: "om_mc"}
	pl := newProgressListener(context.Background(), nil, sender, nil)

	pl.OnEvent(makeToolStarted("call-1", "web_search"))
	time.Sleep(100 * time.Millisecond)

	pl.Close()
	pl.Close() // Should not panic or double-flush.

	if sender.sendCount() != 1 {
		t.Fatalf("expected exactly 1 send, got %d", sender.sendCount())
	}
}

func TestProgressListenerNodeStartedEvent(t *testing.T) {
	clk := newTestClock(time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC))
	sender := &spySender{nextID: "om_node"}
	pl := newProgressListener(context.Background(), nil, sender, nil)
	pl.now = clk.Now

	// NodeStarted with no tools → thinking phrase.
	pl.OnEvent(makeNodeStarted(1))
	time.Sleep(100 * time.Millisecond)

	if sender.sendCount() != 1 {
		t.Fatalf("expected 1 send after NodeStarted, got %d", sender.sendCount())
	}
	text := sender.lastSendText()
	if !isNaturalThinkingPhrase(text) {
		t.Fatalf("expected natural thinking phrase on NodeStarted, got %q", text)
	}

	pl.Close()
}

func TestProgressListenerEnvelopeNodeStartedEvent(t *testing.T) {
	clk := newTestClock(time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC))
	sender := &spySender{nextID: "om_env_node"}
	pl := newProgressListener(context.Background(), nil, sender, nil)
	pl.now = clk.Now

	// Envelope NodeStarted → thinking phrase.
	pl.OnEvent(makeEnvelopeNodeStarted(1))
	time.Sleep(100 * time.Millisecond)

	if sender.sendCount() != 1 {
		t.Fatalf("expected 1 send after envelope NodeStarted, got %d", sender.sendCount())
	}
	text := sender.lastSendText()
	if !isNaturalThinkingPhrase(text) {
		t.Fatalf("expected natural thinking phrase, got %q", text)
	}

	pl.Close()
}

func TestProgressListenerMessageIDAlwaysEmpty(t *testing.T) {
	sender := &spySender{nextID: "om_msgid"}
	pl := newProgressListener(context.Background(), nil, sender, nil)

	// MessageID is always empty — no edit-in-place.
	pl.OnEvent(makeToolStarted("call-1", "web_search"))
	time.Sleep(100 * time.Millisecond)
	if id := pl.MessageID(); id != "" {
		t.Fatalf("expected empty MessageID (no edit-in-place), got %q", id)
	}
	pl.Close()
}

// --- helpers ---

func isNaturalThinkingPhrase(text string) bool {
	for _, p := range naturalThinkingPhrases {
		if strings.Contains(text, strings.TrimSuffix(p, "…")) {
			return true
		}
	}
	return false
}

func TestProgressListenerToolPhraseMapping(t *testing.T) {
	tests := []struct {
		toolName string
		keywords []string
	}{
		{"web_search", []string{"搜索", "探索", "挖掘"}},
		{"read_file", []string{"翻阅", "研读", "查阅"}},
		{"write_file", []string{"撰写", "书写", "落笔"}},
		{"shell_exec", []string{"运算", "执行", "实验"}},
		{"browser_navigate", []string{"浏览", "查看", "观察"}},
		{"seedream", []string{"创作", "绘制", "构图"}},
		{"lark_send_message", []string{"联络", "查询", "协调"}},
		{"plan", []string{"规划", "梳理", "分析"}},
		{"task", []string{"深入", "调研", "拆解"}},
		{"unknown_tool_xyz", []string{"处理", "分析", "洞察"}},
	}

	for _, tc := range tests {
		t.Run(tc.toolName, func(t *testing.T) {
			phrase := uxphrases.ToolPhrase(tc.toolName, 0)
			found := false
			for _, kw := range tc.keywords {
				if strings.Contains(phrase, kw) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("uxphrases.ToolPhrase(%q) = %q, expected one of keywords %v", tc.toolName, phrase, tc.keywords)
			}
		})
	}
}
