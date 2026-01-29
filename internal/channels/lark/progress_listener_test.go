package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
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

func makeToolStarted(callID, toolName string) *domain.WorkflowToolStartedEvent {
	return &domain.WorkflowToolStartedEvent{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		CallID:    callID,
		ToolName:  toolName,
	}
}

func makeToolCompleted(callID, toolName string, dur time.Duration, err error) *domain.WorkflowToolCompletedEvent {
	return &domain.WorkflowToolCompletedEvent{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		CallID:    callID,
		ToolName:  toolName,
		Duration:  dur,
		Error:     err,
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
	ev := &domain.WorkflowInputReceivedEvent{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Task:      "test",
	}
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

	// Start a tool.
	pl.OnEvent(makeToolStarted("call-1", "web_search"))

	// Wait for flush (immediate since no prior flush).
	time.Sleep(100 * time.Millisecond)

	if sender.sendCount() != 1 {
		t.Fatalf("expected 1 send after first tool start, got %d", sender.sendCount())
	}
	text := sender.lastSendText()
	if !strings.Contains(text, "[处理中...]") {
		t.Fatalf("expected progress header, got %q", text)
	}
	if !strings.Contains(text, "web_search") {
		t.Fatalf("expected tool name in progress, got %q", text)
	}
	if !strings.Contains(text, "[running") {
		t.Fatalf("expected running status, got %q", text)
	}

	// Advance time past the rate limit.
	clk.Advance(3 * time.Second)

	// Complete the tool.
	pl.OnEvent(makeToolCompleted("call-1", "web_search", 1200*time.Millisecond, nil))

	// Wait for the flush.
	time.Sleep(100 * time.Millisecond)

	if sender.updateCount() != 1 {
		t.Fatalf("expected 1 update after tool completion, got %d", sender.updateCount())
	}
	updateText := sender.lastUpdateText()
	if !strings.Contains(updateText, "[done 1.2s]") {
		t.Fatalf("expected done status with duration, got %q", updateText)
	}

	pl.Close()
}

func TestProgressListenerErroredTool(t *testing.T) {
	clk := newTestClock(time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC))
	sender := &spySender{nextID: "om_err"}
	pl := newProgressListener(context.Background(), nil, sender, nil)
	pl.now = clk.Now

	pl.OnEvent(makeToolStarted("call-err", "bad_tool"))
	time.Sleep(100 * time.Millisecond)

	clk.Advance(3 * time.Second)
	pl.OnEvent(makeToolCompleted("call-err", "bad_tool", 500*time.Millisecond, fmt.Errorf("failed")))
	time.Sleep(100 * time.Millisecond)

	text := sender.lastUpdateText()
	if !strings.Contains(text, "[error 0.5s]") {
		t.Fatalf("expected error status, got %q", text)
	}
	pl.Close()
}

func TestProgressListenerRateLimiting(t *testing.T) {
	clk := newTestClock(time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC))
	sender := &spySender{nextID: "om_rl"}
	pl := newProgressListener(context.Background(), nil, sender, nil)
	pl.now = clk.Now

	// First tool starts: should trigger immediate flush.
	pl.OnEvent(makeToolStarted("call-1", "tool_a"))
	time.Sleep(100 * time.Millisecond)
	if sender.sendCount() != 1 {
		t.Fatalf("expected 1 send, got %d", sender.sendCount())
	}

	// Second tool starts within rate limit window: should NOT flush immediately.
	clk.Advance(500 * time.Millisecond)
	pl.OnEvent(makeToolStarted("call-2", "tool_b"))
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

	// Start two tools rapidly.
	pl.OnEvent(makeToolStarted("call-1", "tool_a"))
	time.Sleep(100 * time.Millisecond) // Let first flush happen.

	clk.Advance(100 * time.Millisecond)
	pl.OnEvent(makeToolStarted("call-2", "tool_b"))
	// Don't wait for the timer to fire; Close should do a synchronous flush.

	pl.Close()

	// After Close, the update for tool_b should have been sent.
	totalOps := sender.sendCount() + sender.updateCount()
	if totalOps < 2 {
		t.Fatalf("expected at least 2 total operations (send+updates), got %d", totalOps)
	}

	// Verify the final text includes both tools.
	var finalText string
	if sender.updateCount() > 0 {
		finalText = sender.lastUpdateText()
	} else {
		finalText = sender.lastSendText()
	}
	if !strings.Contains(finalText, "tool_a") || !strings.Contains(finalText, "tool_b") {
		t.Fatalf("expected both tools in final text, got %q", finalText)
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

	// Empty tools.
	text := pl.buildText()
	if text != "[处理中...]" {
		t.Fatalf("expected header only for empty tools, got %q", text)
	}

	now := clk.Now()
	// Add tools.
	pl.tools = []*toolStatus{
		{callID: "c1", toolName: "web_search", started: now.Add(-2 * time.Second), done: true, duration: 1200 * time.Millisecond},
		{callID: "c2", toolName: "seedream", started: now.Add(-3 * time.Second)},
		{callID: "c3", toolName: "bad_tool", started: now.Add(-1 * time.Second), done: true, errored: true, duration: 500 * time.Millisecond},
	}

	text = pl.buildText()
	lines := strings.Split(text, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), text)
	}
	if lines[0] != "[处理中...]" {
		t.Fatalf("expected header line, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "web_search") || !strings.Contains(lines[1], "[done 1.2s]") {
		t.Fatalf("expected web_search done line, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "seedream") || !strings.Contains(lines[2], "[running 3s]") {
		t.Fatalf("expected seedream running line, got %q", lines[2])
	}
	if !strings.Contains(lines[3], "bad_tool") || !strings.Contains(lines[3], "[error 0.5s]") {
		t.Fatalf("expected bad_tool error line, got %q", lines[3])
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
