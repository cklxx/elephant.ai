package adapter_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"alex/internal/runtime/adapter"
	"alex/internal/runtime/panel"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// mockPane implements panel.PaneIface and records all method calls.
type mockPane struct {
	mu        sync.Mutex
	id        int
	injected  []string
	submitted int
	sent      []string
	sentKeys  []string
	activated int
	killed    bool

	injectErr  error
	submitErr  error
	sendErr    error
	sendKeyErr error
	activateErr error
	killErr    error

	captureText string
	captureErr  error
}

func newMockPane(id int) *mockPane { return &mockPane{id: id} }

func (m *mockPane) PaneID() int { return m.id }

func (m *mockPane) InjectText(_ context.Context, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.injected = append(m.injected, text)
	return m.injectErr
}

func (m *mockPane) Submit(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.submitted++
	return m.submitErr
}

func (m *mockPane) Send(_ context.Context, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, text)
	return m.sendErr
}

func (m *mockPane) SendKey(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentKeys = append(m.sentKeys, key)
	return m.sendKeyErr
}

func (m *mockPane) CaptureOutput(_ context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.captureText, m.captureErr
}

func (m *mockPane) Activate(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activated++
	return m.activateErr
}

func (m *mockPane) Kill(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.killed = true
	return m.killErr
}

// Compile-time interface check.
var _ panel.PaneIface = (*mockPane)(nil)

// mockManager implements panel.ManagerIface.
type mockManager struct {
	mu       sync.Mutex
	nextPane *mockPane
	splitErr error
	splits   []panel.SplitOpts
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

// mockHookSink records lifecycle callbacks from adapters.
type mockHookSink struct {
	mu         sync.Mutex
	heartbeats []string
	completed  []struct{ id, answer string }
	failed     []struct{ id, err string }
	needsInput []struct{ id, prompt string }
}

func (m *mockHookSink) OnHeartbeat(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heartbeats = append(m.heartbeats, sessionID)
}

func (m *mockHookSink) OnCompleted(sessionID, answer string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completed = append(m.completed, struct{ id, answer string }{sessionID, answer})
}

func (m *mockHookSink) OnFailed(sessionID, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failed = append(m.failed, struct{ id, err string }{sessionID, errMsg})
}

func (m *mockHookSink) OnNeedsInput(sessionID, prompt string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.needsInput = append(m.needsInput, struct{ id, prompt string }{sessionID, prompt})
}

var _ adapter.HookSink = (*mockHookSink)(nil)

// mockCodexExecutor implements adapter.CodexExecutor.
type mockCodexExecutor struct {
	answer string
	err    error
}

func (m *mockCodexExecutor) Execute(_ context.Context, _, _ string, onProgress func()) (string, error) {
	if onProgress != nil {
		onProgress()
	}
	return m.answer, m.err
}

var _ adapter.CodexExecutor = (*mockCodexExecutor)(nil)

// ---------------------------------------------------------------------------
// ClaudeCodeAdapter tests
// ---------------------------------------------------------------------------

func TestClaudeCode_Inject(t *testing.T) {
	t.Parallel()

	mp := newMockPane(10)
	mm := &mockManager{nextPane: mp}
	sink := &mockHookSink{}
	cc := adapter.NewClaudeCodeAdapter(mm, sink, "http://localhost:9999")

	ctx := context.Background()

	// Inject before Start — should fail because no pane is stored.
	err := cc.Inject(ctx, "s1", "hello")
	if err == nil {
		t.Fatal("expected error for unknown session, got nil")
	}

	// Manually register a pane by calling Start with ModeSplit.
	// This will sleep ~3.6s total due to CC timers, but we need it to register the pane.
	err = cc.Start(ctx, adapter.StartOpts{
		SessionID: "s1",
		Goal:      "test goal",
		WorkDir:   "/tmp",
		PaneID:    1,
		Mode:      adapter.ModeSplit,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Now inject should succeed.
	err = cc.Inject(ctx, "s1", "follow-up")
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	// InjectText should have been called for goal + follow-up.
	found := false
	for _, txt := range mp.injected {
		if txt == "follow-up" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'follow-up' in injected texts, got %v", mp.injected)
	}
}

func TestClaudeCode_Inject_UnknownSession(t *testing.T) {
	t.Parallel()

	mm := &mockManager{nextPane: newMockPane(1)}
	cc := adapter.NewClaudeCodeAdapter(mm, &mockHookSink{}, "http://localhost:9999")

	err := cc.Inject(context.Background(), "nonexistent", "text")
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestClaudeCode_Stop_Kill(t *testing.T) {
	t.Parallel()

	mp := newMockPane(20)
	mm := &mockManager{nextPane: mp}
	cc := adapter.NewClaudeCodeAdapter(mm, &mockHookSink{}, "http://localhost:9999")

	ctx := context.Background()

	// Start to register the pane.
	err := cc.Start(ctx, adapter.StartOpts{
		SessionID: "s2",
		Goal:      "goal",
		WorkDir:   "/tmp",
		PaneID:    1,
		Mode:      adapter.ModeSplit,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop with poolPane=false should kill.
	err = cc.Stop(ctx, "s2", false)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	if !mp.killed {
		t.Fatal("expected pane to be killed")
	}
}

func TestClaudeCode_Stop_PoolPane(t *testing.T) {
	t.Parallel()

	mp := newMockPane(30)
	mm := &mockManager{nextPane: mp}
	cc := adapter.NewClaudeCodeAdapter(mm, &mockHookSink{}, "http://localhost:9999")

	ctx := context.Background()

	err := cc.Start(ctx, adapter.StartOpts{
		SessionID: "s3",
		Goal:      "goal",
		WorkDir:   "/tmp",
		PaneID:    1,
		Mode:      adapter.ModeSplit,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop with poolPane=true should send /exit + C-c, not kill.
	err = cc.Stop(ctx, "s3", true)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	if mp.killed {
		t.Fatal("pool pane should not be killed")
	}

	foundExit := false
	for _, s := range mp.sent {
		if s == "/exit" {
			foundExit = true
			break
		}
	}
	if !foundExit {
		t.Fatalf("expected '/exit' in sent commands, got %v", mp.sent)
	}

	foundCtrlC := false
	for _, k := range mp.sentKeys {
		if k == "C-c" {
			foundCtrlC = true
			break
		}
	}
	if !foundCtrlC {
		t.Fatalf("expected 'C-c' in sent keys, got %v", mp.sentKeys)
	}
}

func TestClaudeCode_Stop_NoPane(t *testing.T) {
	t.Parallel()

	mm := &mockManager{nextPane: newMockPane(1)}
	cc := adapter.NewClaudeCodeAdapter(mm, &mockHookSink{}, "http://localhost:9999")

	// Stop a session that was never started — should not error.
	err := cc.Stop(context.Background(), "nonexistent", false)
	if err != nil {
		t.Fatalf("Stop for nonexistent session should not error, got: %v", err)
	}
}

func TestClaudeCode_Start_SplitMode(t *testing.T) {
	t.Parallel()

	mp := newMockPane(50)
	mm := &mockManager{nextPane: mp}
	sink := &mockHookSink{}
	cc := adapter.NewClaudeCodeAdapter(mm, sink, "http://localhost:9999")

	ctx := context.Background()
	err := cc.Start(ctx, adapter.StartOpts{
		SessionID: "split-test",
		Goal:      "implement feature X",
		WorkDir:   "/tmp",
		PaneID:    5,
		Mode:      adapter.ModeSplit,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify Split was called.
	mm.mu.Lock()
	if len(mm.splits) != 1 {
		t.Fatalf("expected 1 split call, got %d", len(mm.splits))
	}
	so := mm.splits[0]
	mm.mu.Unlock()

	if so.ParentPaneID != 5 {
		t.Errorf("ParentPaneID = %d, want 5", so.ParentPaneID)
	}
	if so.Direction != "bottom" {
		t.Errorf("Direction = %q, want bottom", so.Direction)
	}

	// Verify Activate was called.
	mp.mu.Lock()
	if mp.activated < 1 {
		t.Error("expected Activate to be called")
	}

	// Verify goal was injected.
	found := false
	for _, txt := range mp.injected {
		if txt == "implement feature X" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected goal text in injected, got %v", mp.injected)
	}

	// Verify submit was called after inject.
	if mp.submitted < 1 {
		t.Error("expected Submit to be called after InjectText")
	}
	mp.mu.Unlock()
}

func TestClaudeCode_Start_SplitError(t *testing.T) {
	t.Parallel()

	mm := &mockManager{splitErr: errors.New("split failed")}
	cc := adapter.NewClaudeCodeAdapter(mm, &mockHookSink{}, "http://localhost:9999")

	err := cc.Start(context.Background(), adapter.StartOpts{
		SessionID: "err-test",
		Goal:      "goal",
		WorkDir:   "/tmp",
		PaneID:    1,
		Mode:      adapter.ModeSplit,
	})
	if err == nil {
		t.Fatal("expected error from split failure")
	}
}

// ---------------------------------------------------------------------------
// CodexAdapter tests
// ---------------------------------------------------------------------------

func TestCodex_Start(t *testing.T) {
	t.Parallel()

	mp := newMockPane(40)
	mm := &mockManager{nextPane: mp}
	sink := &mockHookSink{}
	exec := &mockCodexExecutor{answer: "done"}
	ca := adapter.NewCodexAdapter(mm, exec, sink)

	ctx := context.Background()
	err := ca.Start(ctx, adapter.StartOpts{
		SessionID: "cx1",
		Goal:      "fix bug",
		WorkDir:   "/tmp",
		PaneID:    2,
		Mode:      adapter.ModeSplit,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify split was called.
	mm.mu.Lock()
	if len(mm.splits) != 1 {
		t.Fatalf("expected 1 split, got %d", len(mm.splits))
	}
	so := mm.splits[0]
	mm.mu.Unlock()

	if so.Percent != 40 {
		t.Errorf("Percent = %d, want 40", so.Percent)
	}

	// Verify pane was activated.
	mp.mu.Lock()
	if mp.activated < 1 {
		t.Error("expected Activate to be called")
	}
	mp.mu.Unlock()
}

func TestCodex_Start_SplitError(t *testing.T) {
	t.Parallel()

	mm := &mockManager{splitErr: errors.New("no pane")}
	exec := &mockCodexExecutor{}
	ca := adapter.NewCodexAdapter(mm, exec, &mockHookSink{})

	err := ca.Start(context.Background(), adapter.StartOpts{
		SessionID: "cx-err",
		Goal:      "goal",
		WorkDir:   "/tmp",
		PaneID:    1,
	})
	if err == nil {
		t.Fatal("expected error from split failure")
	}
}

func TestCodex_Inject_NotSupported(t *testing.T) {
	t.Parallel()

	mm := &mockManager{nextPane: newMockPane(1)}
	ca := adapter.NewCodexAdapter(mm, &mockCodexExecutor{}, &mockHookSink{})

	err := ca.Inject(context.Background(), "cx1", "text")
	if err == nil {
		t.Fatal("expected error for inject not supported")
	}
}

func TestCodex_Stop(t *testing.T) {
	t.Parallel()

	mm := &mockManager{nextPane: newMockPane(1)}
	ca := adapter.NewCodexAdapter(mm, &mockCodexExecutor{}, &mockHookSink{})

	// Stop should be a no-op — no error.
	err := ca.Stop(context.Background(), "cx1", false)
	if err != nil {
		t.Fatalf("Stop should be no-op, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Factory tests
// ---------------------------------------------------------------------------

func TestFactory_New_ClaudeCode(t *testing.T) {
	t.Parallel()

	mm := &mockManager{nextPane: newMockPane(1)}
	sink := &mockHookSink{}
	f := adapter.NewFactory(mm, sink, "http://localhost:9999", nil)

	a, err := f.New("claude_code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
	// Type check: should be *ClaudeCodeAdapter (but we can't import unexported).
	// Just verify it implements Adapter by calling a method.
	_ = a.Stop(context.Background(), "test", false) // no-op for unregistered session
}

func TestFactory_New_Codex(t *testing.T) {
	t.Parallel()

	mm := &mockManager{nextPane: newMockPane(1)}
	sink := &mockHookSink{}
	exec := &mockCodexExecutor{}
	f := adapter.NewFactory(mm, sink, "http://localhost:9999", exec)

	a, err := f.New("codex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
}

func TestFactory_New_Codex_NilExecutor(t *testing.T) {
	t.Parallel()

	mm := &mockManager{nextPane: newMockPane(1)}
	f := adapter.NewFactory(mm, &mockHookSink{}, "http://localhost:9999", nil)

	_, err := f.New("codex")
	if err == nil {
		t.Fatal("expected error when codex executor is nil")
	}
}

func TestFactory_New_Unknown(t *testing.T) {
	t.Parallel()

	mm := &mockManager{nextPane: newMockPane(1)}
	f := adapter.NewFactory(mm, &mockHookSink{}, "http://localhost:9999", nil)

	_, err := f.New("unknown_member")
	if err == nil {
		t.Fatal("expected error for unknown member type")
	}
}

// ---------------------------------------------------------------------------
// hasBashPrompt (exported test via Start+watchForCompletion is indirect;
// test the helper directly if it were exported — since it's not, we test via
// integration in the CC adapter tests above)
// ---------------------------------------------------------------------------

func TestClaudeCode_Start_DirectPaneMode(t *testing.T) {
	// ModeDirectPane calls panel.NewPane() which creates a real Pane
	// (bypassing the mock manager). This test requires a running Kaku
	// terminal and is skipped in CI / headless environments.
	t.Skip("requires live Kaku terminal — panel.NewPane creates a real pane, not mockable via ManagerIface")
}
