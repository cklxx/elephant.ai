//go:build integration

package adapter_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/runtime/adapter"
	"alex/internal/runtime/panel"
	"alex/internal/runtime/session"
)

// ---------------------------------------------------------------------------
// Recording mock pane — captures ordered call records for sequence verification
// ---------------------------------------------------------------------------

type callKind string

const (
	callSend       callKind = "Send"
	callSendKey    callKind = "SendKey"
	callInject     callKind = "InjectText"
	callSubmit     callKind = "Submit"
	callActivate   callKind = "Activate"
	callKill       callKind = "Kill"
	callCapture    callKind = "CaptureOutput"
)

type callRecord struct {
	Kind callKind
	Arg  string // first string argument (empty for no-arg calls)
}

// recordingPane records every method call in order.
type recordingPane struct {
	mu      sync.Mutex
	id      int
	calls   []callRecord
	killed  bool

	// Configurable responses.
	captureText string
	captureErr  error
}

func newRecordingPane(id int) *recordingPane { return &recordingPane{id: id} }

func (p *recordingPane) PaneID() int { return p.id }

func (p *recordingPane) record(kind callKind, arg string) {
	p.calls = append(p.calls, callRecord{Kind: kind, Arg: arg})
}

func (p *recordingPane) InjectText(_ context.Context, text string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.record(callInject, text)
	return nil
}

func (p *recordingPane) Submit(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.record(callSubmit, "")
	return nil
}

func (p *recordingPane) Send(_ context.Context, text string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.record(callSend, text)
	return nil
}

func (p *recordingPane) SendKey(_ context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.record(callSendKey, key)
	return nil
}

func (p *recordingPane) CaptureOutput(_ context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.record(callCapture, "")
	return p.captureText, p.captureErr
}

func (p *recordingPane) Activate(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.record(callActivate, "")
	return nil
}

func (p *recordingPane) Kill(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.killed = true
	p.record(callKill, "")
	return nil
}

func (p *recordingPane) getCalls() []callRecord {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]callRecord, len(p.calls))
	copy(out, p.calls)
	return out
}

var _ panel.PaneIface = (*recordingPane)(nil)

// recordingManager returns a pre-configured recording pane on Split.
type recordingManager struct {
	mu       sync.Mutex
	nextPane *recordingPane
	splits   []panel.SplitOpts
}

func (m *recordingManager) Split(_ context.Context, opts panel.SplitOpts) (panel.PaneIface, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.splits = append(m.splits, opts)
	return m.nextPane, nil
}

func (m *recordingManager) List(_ context.Context) (string, error) { return "", nil }

var _ panel.ManagerIface = (*recordingManager)(nil)

// recordingSink records lifecycle callbacks with thread safety.
type recordingSink struct {
	mu         sync.Mutex
	heartbeats []string
	completed  []struct{ id, answer string }
	failed     []struct{ id, err string }
}

func (s *recordingSink) OnHeartbeat(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeats = append(s.heartbeats, sessionID)
}

func (s *recordingSink) OnCompleted(sessionID, answer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completed = append(s.completed, struct{ id, answer string }{sessionID, answer})
}

func (s *recordingSink) OnFailed(sessionID, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, struct{ id, err string }{sessionID, errMsg})
}

func (s *recordingSink) OnNeedsInput(sessionID, prompt string) {}

var _ adapter.HookSink = (*recordingSink)(nil)

// ---------------------------------------------------------------------------
// Test 1: ClaudeCodeAdapter Start sequence verification
// ---------------------------------------------------------------------------

func TestClaudeCodeAdapter_StartSequence(t *testing.T) {
	t.Parallel()

	rp := newRecordingPane(100)
	rm := &recordingManager{nextPane: rp}
	sink := &recordingSink{}
	cc := adapter.NewClaudeCodeAdapter(rm, sink, "http://localhost:8888")

	ctx := context.Background()
	err := cc.Start(ctx, adapter.StartOpts{
		SessionID: "seq-1",
		Goal:      "implement feature Y",
		WorkDir:   "/tmp",
		PaneID:    10,
		Mode:      adapter.ModeSplit,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Retrieve the ordered call log.
	calls := rp.getCalls()

	// Expected sequence (indices relative to first occurrence):
	//   Activate
	//   Send("export RUNTIME_SESSION_ID=... RUNTIME_HOOKS_URL=...")
	//   Send("unset CLAUDECODE && claude --dangerously-skip-permissions")
	//   InjectText("implement feature Y")
	//   Submit
	//
	// We verify ordering and content of the key calls.

	type expected struct {
		kind    callKind
		substr  string // substring that must appear in Arg
	}

	sequence := []expected{
		{callActivate, ""},
		{callSend, "export RUNTIME_SESSION_ID"},
		{callSend, "unset CLAUDECODE && claude"},
		{callInject, "implement feature Y"},
		{callSubmit, ""},
	}

	idx := 0
	for _, exp := range sequence {
		found := false
		for ; idx < len(calls); idx++ {
			c := calls[idx]
			if c.Kind == exp.kind && (exp.substr == "" || strings.Contains(c.Arg, exp.substr)) {
				found = true
				idx++
				break
			}
		}
		if !found {
			t.Fatalf("expected %s(%q) in call sequence but not found after scanning remaining calls.\nFull call log: %+v", exp.kind, exp.substr, calls)
		}
	}

	// Verify the env export contains both vars.
	for _, c := range calls {
		if c.Kind == callSend && strings.Contains(c.Arg, "export RUNTIME_SESSION_ID") {
			if !strings.Contains(c.Arg, "RUNTIME_HOOKS_URL") {
				t.Errorf("env export missing RUNTIME_HOOKS_URL: %s", c.Arg)
			}
			if !strings.Contains(c.Arg, "seq-1") {
				t.Errorf("env export missing session ID: %s", c.Arg)
			}
			if !strings.Contains(c.Arg, "http://localhost:8888") {
				t.Errorf("env export missing hooks URL: %s", c.Arg)
			}
			break
		}
	}

	// Verify Split was called with correct opts.
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if len(rm.splits) != 1 {
		t.Fatalf("expected 1 split, got %d", len(rm.splits))
	}
	so := rm.splits[0]
	if so.ParentPaneID != 10 {
		t.Errorf("ParentPaneID = %d, want 10", so.ParentPaneID)
	}
	if so.Direction != "bottom" {
		t.Errorf("Direction = %q, want bottom", so.Direction)
	}
	if so.Percent != 65 {
		t.Errorf("Percent = %d, want 65", so.Percent)
	}
	if so.WorkDir != "/tmp" {
		t.Errorf("WorkDir = %q, want /tmp", so.WorkDir)
	}
}

// ---------------------------------------------------------------------------
// Test 2: Completion detection via shell prompt polling
// ---------------------------------------------------------------------------

func TestClaudeCodeAdapter_CompletionDetection(t *testing.T) {
	t.Parallel()

	rp := newRecordingPane(200)
	// Initially no prompt — CC is still running.
	rp.captureText = "Working on task...\n❯ "
	rm := &recordingManager{nextPane: rp}
	sink := &recordingSink{}
	cc := adapter.NewClaudeCodeAdapter(rm, sink, "http://localhost:8888")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := cc.Start(ctx, adapter.StartOpts{
		SessionID: "detect-1",
		Goal:      "test goal",
		WorkDir:   "/tmp",
		PaneID:    1,
		Mode:      adapter.ModeSplit,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// After Start, the watchForCompletion goroutine is polling.
	// Simulate CC exiting by changing capture output to show a bash prompt.
	// Wait a bit then set the prompt.
	time.Sleep(500 * time.Millisecond)
	rp.mu.Lock()
	rp.captureText = "Task completed.\nuser@host:~$ "
	rp.mu.Unlock()

	// Wait for the poller to detect the prompt (polls every 3s).
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for OnCompleted callback")
		default:
		}

		sink.mu.Lock()
		n := len(sink.completed)
		sink.mu.Unlock()

		if n > 0 {
			sink.mu.Lock()
			if sink.completed[0].id != "detect-1" {
				t.Errorf("completed session ID = %q, want detect-1", sink.completed[0].id)
			}
			sink.mu.Unlock()
			return // success
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Pool mode stop — /exit instead of kill
// ---------------------------------------------------------------------------

func TestClaudeCodeAdapter_PoolMode(t *testing.T) {
	t.Parallel()

	rp := newRecordingPane(300)
	rm := &recordingManager{nextPane: rp}
	sink := &recordingSink{}
	cc := adapter.NewClaudeCodeAdapter(rm, sink, "http://localhost:8888")

	ctx := context.Background()
	err := cc.Start(ctx, adapter.StartOpts{
		SessionID: "pool-1",
		Goal:      "pool test",
		WorkDir:   "/tmp",
		PaneID:    1,
		Mode:      adapter.ModeSplit,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop with poolPane=true: should send /exit + C-c, NOT kill.
	err = cc.Stop(ctx, "pool-1", true)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	calls := rp.getCalls()

	// Verify /exit was sent.
	foundExit := false
	foundCtrlC := false
	for _, c := range calls {
		if c.Kind == callSend && c.Arg == "/exit" {
			foundExit = true
		}
		if c.Kind == callSendKey && c.Arg == "C-c" {
			foundCtrlC = true
		}
	}
	if !foundExit {
		t.Errorf("expected Send(/exit) in pool mode stop, calls: %+v", calls)
	}
	if !foundCtrlC {
		t.Errorf("expected SendKey(C-c) in pool mode stop, calls: %+v", calls)
	}

	// Must NOT be killed.
	rp.mu.Lock()
	if rp.killed {
		t.Error("pane should not be killed in pool mode")
	}
	rp.mu.Unlock()

	// Now test non-pool mode: kill the pane.
	// Start a new session first.
	rp2 := newRecordingPane(301)
	rm.mu.Lock()
	rm.nextPane = rp2
	rm.mu.Unlock()

	err = cc.Start(ctx, adapter.StartOpts{
		SessionID: "pool-2",
		Goal:      "kill test",
		WorkDir:   "/tmp",
		PaneID:    1,
		Mode:      adapter.ModeSplit,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = cc.Stop(ctx, "pool-2", false)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	rp2.mu.Lock()
	if !rp2.killed {
		t.Error("pane should be killed in non-pool mode")
	}
	rp2.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Test 4: CodexAdapter execute and report
// ---------------------------------------------------------------------------

type recordingCodexExecutor struct {
	mu         sync.Mutex
	answer     string
	err        error
	called     bool
	progressN  int
}

func (e *recordingCodexExecutor) Execute(_ context.Context, _, _ string, onProgress func()) (string, error) {
	e.mu.Lock()
	e.called = true
	e.mu.Unlock()

	// Fire progress a few times.
	if onProgress != nil {
		for i := 0; i < 3; i++ {
			onProgress()
			e.mu.Lock()
			e.progressN++
			e.mu.Unlock()
		}
	}
	return e.answer, e.err
}

var _ adapter.CodexExecutor = (*recordingCodexExecutor)(nil)

func TestCodexAdapter_ExecuteAndReport(t *testing.T) {
	t.Parallel()

	rp := newRecordingPane(400)
	rm := &recordingManager{nextPane: rp}
	sink := &recordingSink{}
	exec := &recordingCodexExecutor{answer: "all tests pass"}
	ca := adapter.NewCodexAdapter(rm, exec, sink)

	ctx := context.Background()
	err := ca.Start(ctx, adapter.StartOpts{
		SessionID: "codex-1",
		Goal:      "run tests",
		WorkDir:   "/tmp",
		PaneID:    5,
		Mode:      adapter.ModeSplit,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// The executor runs in a goroutine. Wait for completion.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for Codex completion")
		default:
		}

		sink.mu.Lock()
		n := len(sink.completed)
		sink.mu.Unlock()

		if n > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify completion callback.
	sink.mu.Lock()
	if sink.completed[0].id != "codex-1" {
		t.Errorf("completed session ID = %q, want codex-1", sink.completed[0].id)
	}
	if sink.completed[0].answer != "all tests pass" {
		t.Errorf("completed answer = %q, want 'all tests pass'", sink.completed[0].answer)
	}
	sink.mu.Unlock()

	// Verify heartbeats were fired (3 progress callbacks).
	sink.mu.Lock()
	hb := len(sink.heartbeats)
	sink.mu.Unlock()
	if hb < 3 {
		t.Errorf("expected at least 3 heartbeats, got %d", hb)
	}

	// Verify executor was called.
	exec.mu.Lock()
	if !exec.called {
		t.Error("executor should have been called")
	}
	exec.mu.Unlock()

	// Verify pane was activated and status shown.
	calls := rp.getCalls()
	foundActivate := false
	foundStatus := false
	for _, c := range calls {
		if c.Kind == callActivate {
			foundActivate = true
		}
		if c.Kind == callSend && strings.Contains(c.Arg, "[Codex]") {
			foundStatus = true
		}
	}
	if !foundActivate {
		t.Error("expected Activate call on visual pane")
	}
	if !foundStatus {
		t.Error("expected status line sent to pane")
	}
}

// ---------------------------------------------------------------------------
// Test 5: Factory member routing
// ---------------------------------------------------------------------------

func TestAdapterFactory_MemberRouting(t *testing.T) {
	t.Parallel()

	rp := newRecordingPane(500)
	rm := &recordingManager{nextPane: rp}
	sink := &recordingSink{}
	exec := &recordingCodexExecutor{answer: "ok"}
	f := adapter.NewFactory(rm, sink, "http://localhost:8888", exec)

	// claude_code → ClaudeCodeAdapter
	ccAdapter, err := f.New(session.MemberClaudeCode)
	if err != nil {
		t.Fatalf("New(claude_code) error: %v", err)
	}
	if ccAdapter == nil {
		t.Fatal("expected non-nil adapter for claude_code")
	}
	// Verify it behaves like CC adapter: Stop on unknown session is no-op.
	if err := ccAdapter.Stop(context.Background(), "x", false); err != nil {
		t.Errorf("CC adapter Stop should be no-op for unknown session: %v", err)
	}

	// codex → CodexAdapter
	codexAdapter, err := f.New(session.MemberCodex)
	if err != nil {
		t.Fatalf("New(codex) error: %v", err)
	}
	if codexAdapter == nil {
		t.Fatal("expected non-nil adapter for codex")
	}
	// Verify it behaves like Codex adapter: Inject returns error.
	if err := codexAdapter.Inject(context.Background(), "x", "text"); err == nil {
		t.Error("Codex adapter Inject should return error")
	}

	// unknown → error
	_, err = f.New("unknown_type")
	if err == nil {
		t.Fatal("expected error for unknown member type")
	}

	// codex with nil executor → error
	f2 := adapter.NewFactory(rm, sink, "http://localhost:8888", nil)
	_, err = f2.New(session.MemberCodex)
	if err == nil {
		t.Fatal("expected error when codex executor is nil")
	}
}
