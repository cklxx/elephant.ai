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

func TestProgressListenerToolLifecycle(t *testing.T) {
	clk := newTestClock(time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC))
	sender := &spySender{nextID: "om_progress"}
	pl := newProgressListener(context.Background(), nil, sender, nil)
	pl.now = clk.Now

	// Start a key tool (web_search).
	pl.OnEvent(makeToolStarted("call-1", "web_search"))
	time.Sleep(100 * time.Millisecond)

	if sender.sendCount() != 1 {
		t.Fatalf("expected 1 send after key tool start, got %d", sender.sendCount())
	}
	text := sender.lastSendText()
	// Conversational phrase — must not contain tool names.
	if isJargon(text) {
		t.Fatalf("expected conversational phrase, got jargon: %q", text)
	}

	clk.Advance(3 * time.Second)
	pl.OnEvent(makeToolCompleted("call-1", "web_search", 1200*time.Millisecond, nil))
	time.Sleep(100 * time.Millisecond)

	if sender.updateCount() != 1 {
		t.Fatalf("expected 1 update after tool completion, got %d", sender.updateCount())
	}
	if !isNaturalWrappingPhrase(sender.lastUpdateText()) {
		t.Fatalf("expected wrapping phrase, got %q", sender.lastUpdateText())
	}
	pl.Close()
}

func TestProgressListenerEnvelopeLifecycle(t *testing.T) {
	clk := newTestClock(time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC))
	sender := &spySender{nextID: "om_progress"}
	pl := newProgressListener(context.Background(), nil, sender, nil)
	pl.now = clk.Now

	pl.OnEvent(makeEnvelopeToolStarted("call-1", "web_search"))
	time.Sleep(100 * time.Millisecond)
	if sender.sendCount() != 1 {
		t.Fatalf("expected 1 send, got %d", sender.sendCount())
	}

	clk.Advance(3 * time.Second)
	pl.OnEvent(makeEnvelopeToolCompleted("call-1", "web_search", 1200*time.Millisecond, nil))
	time.Sleep(100 * time.Millisecond)
	if !isNaturalWrappingPhrase(sender.lastUpdateText()) {
		t.Fatalf("expected wrapping phrase, got %q", sender.lastUpdateText())
	}
}

func TestProgressListenerNonKeyToolNoProgress(t *testing.T) {
	sender := &spySender{nextID: "om_nonkey"}
	pl := newProgressListener(context.Background(), nil, sender, nil)

	// Non-key tool (read_file) should not trigger progress.
	pl.OnEvent(makeToolStarted("call-1", "read_file"))
	time.Sleep(100 * time.Millisecond)
	if sender.sendCount() != 0 {
		t.Fatalf("expected 0 sends for non-key tool, got %d", sender.sendCount())
	}

	// Non-key errored tool also should not trigger.
	pl.OnEvent(makeToolCompleted("call-1", "read_file", 500*time.Millisecond, fmt.Errorf("failed")))
	time.Sleep(100 * time.Millisecond)
	if sender.updateCount() != 0 {
		t.Fatalf("expected 0 updates for non-key tool, got %d", sender.updateCount())
	}
	pl.Close()
}

func TestProgressListenerRateLimiting(t *testing.T) {
	clk := newTestClock(time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC))
	sender := &spySender{nextID: "om_rl"}
	pl := newProgressListener(context.Background(), nil, sender, nil)
	pl.now = clk.Now

	// First key tool starts: should trigger immediate flush.
	pl.OnEvent(makeToolStarted("call-1", "web_search"))
	time.Sleep(100 * time.Millisecond)
	if sender.sendCount() != 1 {
		t.Fatalf("expected 1 send, got %d", sender.sendCount())
	}

	// Second key tool starts within rate limit window: should NOT flush immediately.
	clk.Advance(500 * time.Millisecond)
	pl.OnEvent(makeToolStarted("call-2", "shell_exec"))
	time.Sleep(100 * time.Millisecond)
	if sender.updateCount() != 0 {
		t.Fatalf("expected 0 updates within rate limit, got %d", sender.updateCount())
	}

	// After rate limit passes, the timer fires.
	clk.Advance(2 * time.Second)
	time.Sleep(2 * time.Second)
	if sender.updateCount() < 1 {
		t.Fatalf("expected at least 1 update after rate limit, got %d", sender.updateCount())
	}
	pl.Close()
}

func TestProgressListenerCloseFlush(t *testing.T) {
	clk := newTestClock(time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC))
	sender := &spySender{nextID: "om_close"}
	pl := newProgressListener(context.Background(), nil, sender, nil)
	pl.now = clk.Now

	// Start two key tools rapidly.
	pl.OnEvent(makeToolStarted("call-1", "web_search"))
	time.Sleep(100 * time.Millisecond)

	clk.Advance(100 * time.Millisecond)
	pl.OnEvent(makeToolStarted("call-2", "shell_exec"))

	pl.Close()
	totalOps := sender.sendCount() + sender.updateCount()
	if totalOps < 2 {
		t.Fatalf("expected at least 2 total operations, got %d", totalOps)
	}
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

	// Empty tools → natural thinking phrase.
	text := pl.buildText()
	if !isNaturalThinkingPhrase(text) {
		t.Fatalf("expected natural thinking phrase for empty tools, got %q", text)
	}

	now := clk.Now()
	// Add tools: one done (key), one running (key), one done non-key.
	pl.tools = []*toolStatus{
		{callID: "c1", toolName: "web_search", started: now.Add(-2 * time.Second), done: true, duration: 1200 * time.Millisecond},
		{callID: "c2", toolName: "seedream", started: now.Add(-3 * time.Second)},
		{callID: "c3", toolName: "read_file", started: now.Add(-1 * time.Second), done: true, duration: 500 * time.Millisecond},
	}

	text = pl.buildText()
	// Has active key tool → conversational phrase, no jargon.
	if isJargon(text) {
		t.Fatalf("expected conversational phrase, got jargon: %q", text)
	}
	if strings.Contains(text, "\n") {
		t.Fatalf("expected single-line phrase, got multi-line: %q", text)
	}

	// All tools done → wrapping phrase.
	pl.tools[1].done = true
	pl.tools[1].duration = 3 * time.Second
	text = pl.buildText()
	if !isNaturalWrappingPhrase(text) {
		t.Fatalf("expected wrapping phrase when all done, got %q", text)
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

func TestProgressListenerMessageID(t *testing.T) {
	sender := &spySender{nextID: "om_msgid"}
	pl := newProgressListener(context.Background(), nil, sender, nil)

	// Before any send, MessageID should be empty.
	if id := pl.MessageID(); id != "" {
		t.Fatalf("expected empty MessageID before send, got %q", id)
	}

	pl.OnEvent(makeToolStarted("call-1", "web_search"))
	time.Sleep(100 * time.Millisecond)

	// After send, MessageID should be set.
	if id := pl.MessageID(); id != "om_msgid" {
		t.Fatalf("expected MessageID=om_msgid, got %q", id)
	}

	pl.Close()
}

// --- natural phrase matchers for progress listener tests ---

func isNaturalThinkingPhrase(text string) bool {
	for _, p := range naturalThinkingPhrases {
		if strings.Contains(text, strings.TrimSuffix(p, "…")) {
			return true
		}
	}
	return false
}

// isJargon returns true if the text contains English tool names or technical terms.
func isJargon(text string) bool {
	lower := strings.ToLower(text)
	jargonTerms := []string{"search", "shell", "exec", "write", "read", "tool", "browser", "file", "command"}
	for _, term := range jargonTerms {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func isNaturalWrappingPhrase(text string) bool {
	for _, p := range naturalWrappingPhrases {
		if strings.Contains(text, strings.TrimSuffix(p, "…")) {
			return true
		}
	}
	return false
}

func TestProgressListenerToolPhraseMapping(t *testing.T) {
	tests := []struct {
		toolName string
		keywords []string // at least one must appear in the phrase
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
