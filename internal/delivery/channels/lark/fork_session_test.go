package lark

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/logging"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// trackingExecutor records every ExecuteTask call and can return configured
// answers so tests can verify both the child and parent invocations.
type trackingExecutor struct {
	mu      sync.Mutex
	calls   []string // task content, in order
	answers map[string]string // task content → answer
	err     error
}

func newTrackingExecutor() *trackingExecutor {
	return &trackingExecutor{answers: make(map[string]string)}
}

func (e *trackingExecutor) setAnswer(taskContent, answer string) {
	e.mu.Lock()
	e.answers[taskContent] = answer
	e.mu.Unlock()
}

func (e *trackingExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "test-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (e *trackingExecutor) ExecuteTask(_ context.Context, task string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	e.mu.Lock()
	e.calls = append(e.calls, task)
	ans := e.answers[task]
	e.mu.Unlock()
	if e.err != nil {
		return nil, e.err
	}
	return &agent.TaskResult{Answer: ans}, nil
}

func (e *trackingExecutor) callCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.calls)
}

func (e *trackingExecutor) allCalls() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.calls))
	copy(out, e.calls)
	return out
}

// newBtwGateway builds a minimal gateway with BtwEnabled=true and the given executor.
func newBtwGateway(t *testing.T, exec AgentExecutor, autoInject bool) *Gateway {
	t.Helper()
	rec := NewRecordingMessenger()
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{
				SessionPrefix: "test",
				AllowDirect:   true,
			},
			AppID:     "test-app",
			AppSecret: "test-secret",
			BtwEnabled:          true,
			BtwAutoInjectResult: &autoInject,
			BtwResultPrefix:     "[btw]",
		},
		agent:     exec,
		logger:    logging.OrNop(nil),
		messenger: wrapInjectCaptureHub(rec),
		dedup:     newEventDedup(nil),
		now:       time.Now,
	}
	return gw
}

// ---------------------------------------------------------------------------
// Unit tests for fork_session.go helpers
// ---------------------------------------------------------------------------

func TestGenerateChildSessionID_ContainsBtw(t *testing.T) {
	parent := "lark-session-abc123"
	child := generateChildSessionID(parent)

	if !strings.Contains(child, "/btw/") {
		t.Fatalf("expected child ID to contain /btw/, got %q", child)
	}
	if !strings.HasPrefix(child, parent) {
		t.Fatalf("expected child ID to start with parent %q, got %q", parent, child)
	}
}

func TestGenerateChildSessionID_NoDoubleNesting(t *testing.T) {
	parent := "lark-session-abc123"
	child1 := generateChildSessionID(parent)
	// Simulating a fork of a fork: the result should still be one level deep.
	child2 := generateChildSessionID(child1)

	if strings.Count(child2, "/btw/") != 1 {
		t.Fatalf("expected exactly one /btw/ in nested child ID, got %q", child2)
	}
	if !strings.HasPrefix(child2, parent) {
		t.Fatalf("expected nested child to re-root to original parent, got %q", child2)
	}
}

func TestBtwAutoInjectEnabled_NilFalse(t *testing.T) {
	gw := &Gateway{cfg: Config{BtwAutoInjectResult: nil}}
	if gw.btwAutoInjectEnabled() {
		t.Fatal("expected btwAutoInjectEnabled to return false when nil")
	}
}

func TestBtwAutoInjectEnabled_ExplicitTrue(t *testing.T) {
	b := true
	gw := &Gateway{cfg: Config{BtwAutoInjectResult: &b}}
	if !gw.btwAutoInjectEnabled() {
		t.Fatal("expected btwAutoInjectEnabled to return true")
	}
}

func TestBtwInjectionPrefix_Default(t *testing.T) {
	gw := &Gateway{cfg: Config{BtwResultPrefix: ""}}
	if gw.btwInjectionPrefix() != "[btw result] " {
		t.Fatalf("unexpected default prefix: %q", gw.btwInjectionPrefix())
	}
}

func TestBtwInjectionPrefix_Custom(t *testing.T) {
	gw := &Gateway{cfg: Config{BtwResultPrefix: "[aside]"}}
	if gw.btwInjectionPrefix() != "[aside] " {
		t.Fatalf("unexpected custom prefix: %q", gw.btwInjectionPrefix())
	}
}

// ---------------------------------------------------------------------------
// Integration tests via InjectMessageSync / InjectMessage
// ---------------------------------------------------------------------------

// TestBtwForkSpawnsChildSession verifies that when BtwEnabled=true and a task
// is running, a second message forks a child session instead of injecting into
// the parent.
func TestBtwForkSpawnsChildSession(t *testing.T) {
	// Use an executor that blocks on a channel so we can guarantee the first
	// task is still running when the second message arrives.
	unblock := make(chan struct{})
	exec := &forkBlockingExecutor{
		unblock: unblock,
		result:  &agent.TaskResult{Answer: "parent done"},
	}
	gw := newBtwGateway(t, exec, false)

	ctx := context.Background()

	// Start the first (parent) task asynchronously — it will block.
	errCh := make(chan error, 1)
	go func() {
		errCh <- gw.InjectMessage(ctx, "oc_chat1", "p2p", "ou_user1", "om_msg1", "long running task")
	}()

	// Wait until executor has been called at least once (parent is running).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && exec.callCount() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if exec.callCount() == 0 {
		t.Fatal("timeout: executor never called for parent task")
	}

	// Inject a "btw" message while parent is running.
	if err := gw.InjectMessage(ctx, "oc_chat1", "p2p", "ou_user1", "om_msg2", "btw quick question"); err != nil {
		t.Fatalf("second InjectMessage failed: %v", err)
	}

	// Wait for the child session to start (executor should be called a second time).
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && exec.callCount() < 2 {
		time.Sleep(10 * time.Millisecond)
	}

	// Unblock the parent so all goroutines finish cleanly.
	close(unblock)

	// Wait for InjectMessage to return and all tasks to drain.
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("first InjectMessage returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for first InjectMessage to return")
	}
	gw.WaitForTasks()

	if exec.callCount() < 2 {
		t.Fatalf("expected at least 2 ExecuteTask calls (parent + child), got %d", exec.callCount())
	}

	// The child session ID should contain /btw/.
	sessionIDs := exec.allSessionIDs()
	var hasBtwChild bool
	for _, sid := range sessionIDs {
		if strings.Contains(sid, "/btw/") {
			hasBtwChild = true
		}
	}
	if !hasBtwChild {
		t.Fatalf("no child session with /btw/ suffix; sessions=%v", sessionIDs)
	}
}

// TestBtwDisabledFallsBackToInject verifies that when BtwEnabled=false the old
// behaviour is preserved: messages are injected into the parent inputCh.
func TestBtwDisabledFallsBackToInject(t *testing.T) {
	unblock := make(chan struct{})
	exec := &forkBlockingExecutor{
		unblock: unblock,
		result:  &agent.TaskResult{Answer: "ok"},
	}

	rec := NewRecordingMessenger()
	// BtwEnabled is false (default).
	gw := &Gateway{
		cfg: Config{
			BaseConfig:  channels.BaseConfig{SessionPrefix: "test", AllowDirect: true},
			AppID:       "test-app",
			AppSecret:   "test-secret",
			BtwEnabled:  false,
		},
		agent:     exec,
		logger:    logging.OrNop(nil),
		messenger: wrapInjectCaptureHub(rec),
		dedup:     newEventDedup(nil),
		now:       time.Now,
	}

	ctx := context.Background()
	go gw.InjectMessage(ctx, "oc_chat2", "p2p", "ou_user1", "om_m1", "task")

	// Wait for parent to be running.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && exec.callCount() == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	// Second message: should be injected (not forked), so NO new ExecuteTask call.
	_ = gw.InjectMessage(ctx, "oc_chat2", "p2p", "ou_user1", "om_m2", "injected msg")

	time.Sleep(100 * time.Millisecond) // give any spurious goroutine time to appear

	// Unblock parent and wait.
	close(unblock)
	gw.WaitForTasks()

	// Only 1 ExecuteTask call expected (no fork).
	if exec.callCount() != 1 {
		t.Fatalf("expected exactly 1 ExecuteTask call (no fork), got %d; calls=%v", exec.callCount(), exec.allCalls())
	}
}

// ---------------------------------------------------------------------------
// forkBlockingExecutor — executor that blocks until `unblock` is closed.
// ---------------------------------------------------------------------------

type forkBlockingExecutor struct {
	mu         sync.Mutex
	calls      []string
	sessionIDs []string
	unblock    <-chan struct{}
	result     *agent.TaskResult
	err        error
}

func (b *forkBlockingExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "test-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (b *forkBlockingExecutor) ExecuteTask(_ context.Context, task string, sessionID string, _ agent.EventListener) (*agent.TaskResult, error) {
	b.mu.Lock()
	b.calls = append(b.calls, task)
	b.sessionIDs = append(b.sessionIDs, sessionID)
	b.mu.Unlock()

	// Block until unblocked or context cancelled.
	if b.unblock != nil {
		<-b.unblock
	}
	if b.err != nil {
		return nil, b.err
	}
	return b.result, nil
}

func (b *forkBlockingExecutor) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.calls)
}

func (b *forkBlockingExecutor) allCalls() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.calls))
	copy(out, b.calls)
	return out
}

func (b *forkBlockingExecutor) allSessionIDs() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.sessionIDs))
	copy(out, b.sessionIDs)
	return out
}
