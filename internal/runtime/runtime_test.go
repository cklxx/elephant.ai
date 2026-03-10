package runtime

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"alex/internal/runtime/adapter"
	"alex/internal/runtime/hooks"
	"alex/internal/runtime/panel"
	"alex/internal/runtime/pool"
	"alex/internal/runtime/session"
	"alex/internal/runtime/store"
)

// ---------------------------------------------------------------------------
// Mocks (white-box: package runtime, so we access unexported fields)
// ---------------------------------------------------------------------------

// mockPane implements panel.PaneIface.
type mockPane struct {
	mu        sync.Mutex
	id        int
	injected  []string
	submitted int
	sent      []string
	sentKeys  []string
	activated int
	killed    bool
}

func newMockPane(id int) *mockPane { return &mockPane{id: id} }

func (m *mockPane) PaneID() int { return m.id }

func (m *mockPane) InjectText(_ context.Context, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.injected = append(m.injected, text)
	return nil
}

func (m *mockPane) Submit(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.submitted++
	return nil
}

func (m *mockPane) Send(_ context.Context, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, text)
	return nil
}

func (m *mockPane) SendKey(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentKeys = append(m.sentKeys, key)
	return nil
}

func (m *mockPane) CaptureOutput(_ context.Context) (string, error) {
	return "", nil
}

func (m *mockPane) Activate(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activated++
	return nil
}

func (m *mockPane) Kill(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.killed = true
	return nil
}

var _ panel.PaneIface = (*mockPane)(nil)

// mockManager implements panel.ManagerIface.
type mockManager struct {
	mu       sync.Mutex
	nextPane *mockPane
	splits   []panel.SplitOpts
	splitErr error
}

func (m *mockManager) Split(_ context.Context, opts panel.SplitOpts) (panel.PaneIface, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.splits = append(m.splits, opts)
	if m.splitErr != nil {
		return nil, m.splitErr
	}
	return m.nextPane, nil
}

func (m *mockManager) List(_ context.Context) (string, error) { return "", nil }

var _ panel.ManagerIface = (*mockManager)(nil)

// mockAdapter implements adapter.Adapter.
type mockAdapter struct {
	mu       sync.Mutex
	started  []adapter.StartOpts
	injected []struct{ id, text string }
	stopped  []struct {
		id       string
		poolPane bool
	}
	startErr  error
	injectErr error
	stopErr   error
}

func (m *mockAdapter) Start(_ context.Context, opts adapter.StartOpts) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = append(m.started, opts)
	return m.startErr
}

func (m *mockAdapter) Inject(_ context.Context, sessionID, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.injected = append(m.injected, struct{ id, text string }{sessionID, text})
	return m.injectErr
}

func (m *mockAdapter) Stop(_ context.Context, sessionID string, poolPane bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = append(m.stopped, struct {
		id       string
		poolPane bool
	}{sessionID, poolPane})
	return m.stopErr
}

var _ adapter.Adapter = (*mockAdapter)(nil)

// mockFactory is a Factory replacement that returns a mockAdapter.
// Since adapter.Factory is a concrete struct with an unexported New method,
// we can't directly mock it. Instead, we create a real Factory with a mock
// panel manager and a mock codex executor. The CC adapter it creates will
// use the mock panel manager.
//
// For faster tests (avoiding CC sleep), we use codex adapters.
type mockCodexExecutor struct {
	answer string
	err    error
	block  chan struct{} // if non-nil, Execute blocks until this channel is closed
}

func (m *mockCodexExecutor) Execute(ctx context.Context, _, _ string, onProgress func()) (string, error) {
	if onProgress != nil {
		onProgress()
	}
	if m.block != nil {
		select {
		case <-m.block:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return m.answer, m.err
}

var _ adapter.CodexExecutor = (*mockCodexExecutor)(nil)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestRuntime creates a Runtime with a real store (temp dir), real bus,
// and a mock panel manager. The factory is NOT set by default — call
// rt.SetFactory() to wire one.
//
// The store directory is created with os.MkdirTemp and cleaned up via
// t.Cleanup with a short delay to allow in-flight goroutines (e.g. Codex
// executor) to finish writing before removal.
func newTestRuntime(t *testing.T) *Runtime {
	t.Helper()

	storeDir, err := os.MkdirTemp("", "runtime-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		// Brief delay so any in-flight goroutines finish writing to the store.
		time.Sleep(50 * time.Millisecond)
		os.RemoveAll(storeDir)
	})

	st, err := store.New(storeDir)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}

	bus := hooks.NewInProcessBus()
	pm := &mockManager{nextPane: newMockPane(100)}

	rt := &Runtime{
		sessions: make(map[string]*session.Session),
		adapters: make(map[string]adapter.Adapter),
		panel:    pm,
		store:    st,
		bus:      bus,
	}
	return rt
}

// ---------------------------------------------------------------------------
// Session lifecycle tests
// ---------------------------------------------------------------------------

func TestRuntime_CreateSession(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, err := rt.CreateSession(session.MemberClaudeCode, "implement X", "/tmp", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if s.Snapshot().Goal != "implement X" {
		t.Errorf("Goal = %q, want 'implement X'", s.Snapshot().Goal)
	}
	if s.Snapshot().State != session.StateCreated {
		t.Errorf("State = %q, want 'created'", s.Snapshot().State)
	}
	if s.Snapshot().Member != session.MemberClaudeCode {
		t.Errorf("Member = %q, want 'claude_code'", s.Snapshot().Member)
	}

	// Should be retrievable.
	snap, ok := rt.GetSession(s.Snapshot().ID)
	if !ok {
		t.Fatal("GetSession returned false for just-created session")
	}
	if snap.Goal != "implement X" {
		t.Errorf("GetSession Goal = %q, want 'implement X'", snap.Goal)
	}
}

func TestRuntime_CreateSession_WithParent(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, err := rt.CreateSession(session.MemberCodex, "sub-task", "/tmp", "parent-123")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	snap := s.Snapshot()
	if snap.ParentSessionID != "parent-123" {
		t.Errorf("ParentSessionID = %q, want 'parent-123'", snap.ParentSessionID)
	}
}

func TestRuntime_StartSession_NotFound(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	err := rt.StartSession(context.Background(), "nonexistent", 1)
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestRuntime_StartSession_LegacyPath(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	mp := newMockPane(42)
	rt.panel = &mockManager{nextPane: mp}

	s, err := rt.CreateSession(session.MemberClaudeCode, "goal", "/tmp", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// No factory set — uses legacy path with panel.Split.
	err = rt.StartSession(context.Background(), s.Snapshot().ID, 1)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	snap := s.Snapshot()
	if snap.State != session.StateRunning {
		t.Errorf("State = %q, want 'running'", snap.State)
	}
	if snap.PaneID != 42 {
		t.Errorf("PaneID = %d, want 42", snap.PaneID)
	}
}

func TestRuntime_StartSession_WithFactory(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	mm := &mockManager{nextPane: newMockPane(55)}

	// Use a blocking executor so the goroutine doesn't outlive the test.
	block := make(chan struct{})
	exec := &mockCodexExecutor{answer: "done", block: block}
	f := adapter.NewFactory(mm, rt, "http://localhost:9999", exec)
	rt.SetFactory(f)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := rt.CreateSession(session.MemberCodex, "fix bug", "/tmp", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	err = rt.StartSession(ctx, s.Snapshot().ID, 1)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	snap := s.Snapshot()
	if snap.State != session.StateRunning {
		t.Errorf("State = %q, want 'running'", snap.State)
	}

	// Cancel to unblock the executor goroutine so it doesn't outlive the test.
	cancel()
}

func TestRuntime_StartSession_PoolMode(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	mm := &mockManager{nextPane: newMockPane(70)}
	block := make(chan struct{})
	exec := &mockCodexExecutor{answer: "pool-done", block: block}
	f := adapter.NewFactory(mm, rt, "http://localhost:9999", exec)
	rt.SetFactory(f)

	p := pool.New()
	p.Register([]int{200, 201})
	rt.SetPool(p)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := rt.CreateSession(session.MemberCodex, "pool task", "/tmp", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// parentPaneID = -1 triggers pool mode.
	err = rt.StartSession(ctx, s.Snapshot().ID, -1)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	snap := s.Snapshot()
	if snap.State != session.StateRunning {
		t.Errorf("State = %q, want 'running'", snap.State)
	}
	if !snap.PoolPane {
		t.Error("expected PoolPane = true")
	}
	if snap.PaneID < 0 {
		t.Errorf("PaneID = %d, expected >= 0 (from pool)", snap.PaneID)
	}

	// Cancel to unblock the executor goroutine.
	cancel()
}

func TestRuntime_StopSession(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	mm := &mockManager{nextPane: newMockPane(80)}
	block := make(chan struct{})
	exec := &mockCodexExecutor{answer: "done", block: block}
	f := adapter.NewFactory(mm, rt, "http://localhost:9999", exec)
	rt.SetFactory(f)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := rt.CreateSession(session.MemberCodex, "stop-test", "/tmp", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	err = rt.StartSession(ctx, s.Snapshot().ID, 1)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	err = rt.StopSession(ctx, s.Snapshot().ID)
	if err != nil {
		t.Fatalf("StopSession: %v", err)
	}

	snap := s.Snapshot()
	if snap.State != session.StateCancelled {
		t.Errorf("State = %q, want 'cancelled'", snap.State)
	}

	// Cancel to unblock executor goroutine.
	cancel()
}

func TestRuntime_StopSession_NotFound(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	err := rt.StopSession(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

// ---------------------------------------------------------------------------
// Completion/failure hooks
// ---------------------------------------------------------------------------

func TestRuntime_MarkCompleted(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, _ := rt.CreateSession(session.MemberClaudeCode, "complete-test", "/tmp", "")
	id := s.Snapshot().ID

	// Move to running state first.
	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)

	// Subscribe to events.
	ch, cancel := rt.bus.Subscribe(id)
	defer cancel()

	err := rt.MarkCompleted(id, "the answer")
	if err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}

	snap := s.Snapshot()
	if snap.State != session.StateCompleted {
		t.Errorf("State = %q, want 'completed'", snap.State)
	}
	if snap.Answer != "the answer" {
		t.Errorf("Answer = %q, want 'the answer'", snap.Answer)
	}

	// Check event was published.
	select {
	case ev := <-ch:
		if ev.Type != hooks.EventCompleted {
			t.Errorf("event type = %q, want 'completed'", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for completed event")
	}
}

func TestRuntime_MarkCompleted_WithParent(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	// Create parent session.
	parent, _ := rt.CreateSession(session.MemberClaudeCode, "leader", "/tmp", "")
	parentID := parent.Snapshot().ID

	// Create child session with parent.
	child, _ := rt.CreateSession(session.MemberCodex, "child-task", "/tmp", parentID)
	childID := child.Snapshot().ID

	// Move child to running.
	_ = child.Transition(session.StateStarting)
	_ = child.Transition(session.StateRunning)

	// Subscribe to parent events.
	parentCh, cancel := rt.bus.Subscribe(parentID)
	defer cancel()

	err := rt.MarkCompleted(childID, "child answer")
	if err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}

	// Parent should receive EventChildCompleted.
	select {
	case ev := <-parentCh:
		if ev.Type != hooks.EventChildCompleted {
			t.Errorf("parent event type = %q, want 'child_completed'", ev.Type)
		}
		if ev.Payload["child_id"] != childID {
			t.Errorf("child_id = %v, want %s", ev.Payload["child_id"], childID)
		}
		if ev.Payload["child_answer"] != "child answer" {
			t.Errorf("child_answer = %v, want 'child answer'", ev.Payload["child_answer"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for child_completed event on parent")
	}
}

func TestRuntime_MarkFailed(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, _ := rt.CreateSession(session.MemberClaudeCode, "fail-test", "/tmp", "")
	id := s.Snapshot().ID

	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)

	ch, cancel := rt.bus.Subscribe(id)
	defer cancel()

	err := rt.MarkFailed(id, "something broke")
	if err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	snap := s.Snapshot()
	if snap.State != session.StateFailed {
		t.Errorf("State = %q, want 'failed'", snap.State)
	}
	if snap.ErrorMsg != "something broke" {
		t.Errorf("ErrorMsg = %q, want 'something broke'", snap.ErrorMsg)
	}

	select {
	case ev := <-ch:
		if ev.Type != hooks.EventFailed {
			t.Errorf("event type = %q, want 'failed'", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for failed event")
	}
}

func TestRuntime_MarkFailed_WithParent(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	parent, _ := rt.CreateSession(session.MemberClaudeCode, "leader", "/tmp", "")
	parentID := parent.Snapshot().ID

	child, _ := rt.CreateSession(session.MemberCodex, "child-fail", "/tmp", parentID)
	childID := child.Snapshot().ID

	_ = child.Transition(session.StateStarting)
	_ = child.Transition(session.StateRunning)

	parentCh, cancel := rt.bus.Subscribe(parentID)
	defer cancel()

	_ = rt.MarkFailed(childID, "child error")

	select {
	case ev := <-parentCh:
		if ev.Type != hooks.EventChildCompleted {
			t.Errorf("parent event type = %q, want 'child_completed'", ev.Type)
		}
		if ev.Payload["child_error"] != "child error" {
			t.Errorf("child_error = %v, want 'child error'", ev.Payload["child_error"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for child_completed event on parent")
	}
}

func TestRuntime_MarkCompleted_NotFound(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	err := rt.MarkCompleted("nonexistent", "answer")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestRuntime_MarkFailed_NotFound(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	err := rt.MarkFailed("nonexistent", "error")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestRuntime_MarkCompleted_ReleasesPoolPane(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	p := pool.New()
	p.Register([]int{300})
	rt.SetPool(p)

	s, _ := rt.CreateSession(session.MemberClaudeCode, "pool-complete", "/tmp", "")
	id := s.Snapshot().ID

	// Simulate pool acquisition.
	paneID, _ := p.Acquire(context.Background(), id)
	s.SetPane(paneID, paneID)
	s.SetPoolPane(true)

	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)

	_ = rt.MarkCompleted(id, "done")

	// Pool slot should be released (idle again).
	slots := p.Slots()
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(slots))
	}
	if slots[0].State != pool.SlotIdle {
		t.Errorf("slot state = %q, want 'idle'", slots[0].State)
	}
}

// ---------------------------------------------------------------------------
// HookSink implementation tests
// ---------------------------------------------------------------------------

func TestRuntime_OnHeartbeat(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, _ := rt.CreateSession(session.MemberClaudeCode, "hb-test", "/tmp", "")
	id := s.Snapshot().ID

	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)

	ch, cancel := rt.bus.Subscribe(id)
	defer cancel()

	rt.OnHeartbeat(id)

	snap := s.Snapshot()
	if snap.LastHeartbeat == nil {
		t.Fatal("expected LastHeartbeat to be set")
	}

	select {
	case ev := <-ch:
		if ev.Type != hooks.EventHeartbeat {
			t.Errorf("event type = %q, want 'heartbeat'", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for heartbeat event")
	}
}

func TestRuntime_OnCompleted(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, _ := rt.CreateSession(session.MemberClaudeCode, "on-complete", "/tmp", "")
	id := s.Snapshot().ID

	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)

	rt.OnCompleted(id, "final answer")

	snap := s.Snapshot()
	if snap.State != session.StateCompleted {
		t.Errorf("State = %q, want 'completed'", snap.State)
	}
	if snap.Answer != "final answer" {
		t.Errorf("Answer = %q, want 'final answer'", snap.Answer)
	}
}

func TestRuntime_OnFailed(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, _ := rt.CreateSession(session.MemberClaudeCode, "on-fail", "/tmp", "")
	id := s.Snapshot().ID

	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)

	rt.OnFailed(id, "crash")

	snap := s.Snapshot()
	if snap.State != session.StateFailed {
		t.Errorf("State = %q, want 'failed'", snap.State)
	}
	if snap.ErrorMsg != "crash" {
		t.Errorf("ErrorMsg = %q, want 'crash'", snap.ErrorMsg)
	}
}

func TestRuntime_OnNeedsInput(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, _ := rt.CreateSession(session.MemberClaudeCode, "input-test", "/tmp", "")
	id := s.Snapshot().ID

	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)

	ch, cancel := rt.bus.Subscribe(id)
	defer cancel()

	rt.OnNeedsInput(id, "confirm?")

	snap := s.Snapshot()
	if snap.State != session.StateNeedsInput {
		t.Errorf("State = %q, want 'needs_input'", snap.State)
	}

	select {
	case ev := <-ch:
		if ev.Type != hooks.EventNeedsInput {
			t.Errorf("event type = %q, want 'needs_input'", ev.Type)
		}
		if ev.Payload["prompt"] != "confirm?" {
			t.Errorf("prompt = %v, want 'confirm?'", ev.Payload["prompt"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for needs_input event")
	}
}

// ---------------------------------------------------------------------------
// Query methods
// ---------------------------------------------------------------------------

func TestRuntime_ListSessions(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	rt.CreateSession(session.MemberClaudeCode, "goal-1", "/tmp", "")
	rt.CreateSession(session.MemberCodex, "goal-2", "/tmp", "")

	list := rt.ListSessions()
	if len(list) != 2 {
		t.Fatalf("ListSessions returned %d, want 2", len(list))
	}

	goals := map[string]bool{}
	for _, s := range list {
		goals[s.Goal] = true
	}
	if !goals["goal-1"] || !goals["goal-2"] {
		t.Errorf("missing expected goals in %v", goals)
	}
}

func TestRuntime_GetSession(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, _ := rt.CreateSession(session.MemberClaudeCode, "get-test", "/tmp", "")
	id := s.Snapshot().ID

	snap, ok := rt.GetSession(id)
	if !ok {
		t.Fatal("GetSession returned false")
	}
	if snap.Goal != "get-test" {
		t.Errorf("Goal = %q, want 'get-test'", snap.Goal)
	}
}

func TestRuntime_GetSession_NotFound(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	_, ok := rt.GetSession("nonexistent")
	if ok {
		t.Fatal("GetSession should return false for nonexistent session")
	}
}

// ---------------------------------------------------------------------------
// Stall detection
// ---------------------------------------------------------------------------

func TestRuntime_ScanStalled(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	// Create two running sessions.
	s1, _ := rt.CreateSession(session.MemberClaudeCode, "stall-1", "/tmp", "")
	s2, _ := rt.CreateSession(session.MemberClaudeCode, "stall-2", "/tmp", "")

	_ = s1.Transition(session.StateStarting)
	_ = s1.Transition(session.StateRunning)
	_ = s2.Transition(session.StateStarting)
	_ = s2.Transition(session.StateRunning)

	// Give s1 a recent heartbeat.
	s1.RecordHeartbeat()

	// s2 has no heartbeat — its StartedAt was set moments ago.
	// Use a very short threshold to detect s2 as stalled.
	// We need to wait or manipulate time. Since IsStalled checks time.Since(StartedAt),
	// and StartedAt was just set, with threshold=0 both would be stalled.
	// With threshold=1h, neither is stalled. Let's use threshold=0.
	stalled := rt.ScanStalled(0)

	// Both should be "stalled" with threshold=0 since even a nanosecond has passed.
	if len(stalled) < 1 {
		t.Fatalf("expected at least 1 stalled session, got %d", len(stalled))
	}
}

func TestRuntime_ScanStalled_TerminalSessionsExcluded(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, _ := rt.CreateSession(session.MemberClaudeCode, "completed", "/tmp", "")
	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)
	_ = s.Transition(session.StateCompleted)

	stalled := rt.ScanStalled(0)
	if len(stalled) != 0 {
		t.Fatalf("completed session should not be stalled, got %v", stalled)
	}
}

// ---------------------------------------------------------------------------
// InjectText
// ---------------------------------------------------------------------------

func TestRuntime_InjectText_NoAdapter(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, _ := rt.CreateSession(session.MemberClaudeCode, "inject-test", "/tmp", "")
	id := s.Snapshot().ID

	err := rt.InjectText(context.Background(), id, "hello")
	if err == nil {
		t.Fatal("expected error when no adapter is registered")
	}
}

func TestRuntime_InjectText_WithAdapter(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	s, _ := rt.CreateSession(session.MemberClaudeCode, "inject-test", "/tmp", "")
	id := s.Snapshot().ID

	ma := &mockAdapter{}
	rt.mu.Lock()
	rt.adapters[id] = ma
	rt.mu.Unlock()

	err := rt.InjectText(context.Background(), id, "hello")
	if err != nil {
		t.Fatalf("InjectText: %v", err)
	}

	ma.mu.Lock()
	defer ma.mu.Unlock()
	if len(ma.injected) != 1 || ma.injected[0].text != "hello" {
		t.Errorf("expected inject('hello'), got %v", ma.injected)
	}
}

// ---------------------------------------------------------------------------
// Bus accessor
// ---------------------------------------------------------------------------

func TestRuntime_Bus(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	if rt.Bus() == nil {
		t.Fatal("Bus() should not return nil")
	}
}

// ---------------------------------------------------------------------------
// SetFactory / SetPool
// ---------------------------------------------------------------------------

func TestRuntime_SetFactory(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	if rt.factory != nil {
		t.Fatal("factory should be nil initially")
	}

	mm := &mockManager{nextPane: newMockPane(1)}
	f := adapter.NewFactory(mm, rt, "http://localhost:9999", nil)
	rt.SetFactory(f)

	if rt.factory == nil {
		t.Fatal("factory should be set after SetFactory")
	}
}

func TestRuntime_SetPool(t *testing.T) {
	t.Parallel()
	rt := newTestRuntime(t)

	if rt.Pool() != nil {
		t.Fatal("pool should be nil initially")
	}

	p := pool.New()
	rt.SetPool(p)

	if rt.Pool() == nil {
		t.Fatal("pool should be set after SetPool")
	}
}
