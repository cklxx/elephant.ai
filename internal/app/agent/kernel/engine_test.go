package kernel

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/shared/logging"
)

// memStore is an in-memory Store implementation for testing.
type memStore struct {
	mu          sync.Mutex
	dispatches  []kerneldomain.Dispatch
	schemaReady bool
	recoverHits int
	recoverErr  error
}

func newMemStore() *memStore {
	return &memStore{}
}

func (s *memStore) EnsureSchema(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schemaReady = true
	return nil
}

func (s *memStore) EnqueueDispatches(_ context.Context, kernelID, cycleID string, specs []kerneldomain.DispatchSpec) ([]kerneldomain.Dispatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []kerneldomain.Dispatch
	for i, sp := range specs {
		d := kerneldomain.Dispatch{
			DispatchID: fmt.Sprintf("d-%d", len(s.dispatches)+i),
			KernelID:   kernelID,
			CycleID:    cycleID,
			AgentID:    sp.AgentID,
			Prompt:     sp.Prompt,
			Priority:   sp.Priority,
			Status:     kerneldomain.DispatchPending,
			Metadata:   sp.Metadata,
		}
		s.dispatches = append(s.dispatches, d)
		out = append(out, d)
	}
	return out, nil
}

func (s *memStore) ClaimDispatches(_ context.Context, _, _ string, _ int) ([]kerneldomain.Dispatch, error) {
	return nil, nil
}

func (s *memStore) MarkDispatchRunning(_ context.Context, dispatchID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.dispatches {
		if s.dispatches[i].DispatchID == dispatchID {
			s.dispatches[i].Status = kerneldomain.DispatchRunning
		}
	}
	return nil
}

func (s *memStore) MarkDispatchDone(_ context.Context, dispatchID, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.dispatches {
		if s.dispatches[i].DispatchID == dispatchID {
			s.dispatches[i].Status = kerneldomain.DispatchDone
			s.dispatches[i].TaskID = taskID
		}
	}
	return nil
}

func (s *memStore) MarkDispatchFailed(_ context.Context, dispatchID, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.dispatches {
		if s.dispatches[i].DispatchID == dispatchID {
			s.dispatches[i].Status = kerneldomain.DispatchFailed
			s.dispatches[i].Error = errMsg
		}
	}
	return nil
}

func (s *memStore) ListActiveDispatches(_ context.Context, kernelID string) ([]kerneldomain.Dispatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []kerneldomain.Dispatch
	for _, d := range s.dispatches {
		if d.KernelID == kernelID && (d.Status == kerneldomain.DispatchPending || d.Status == kerneldomain.DispatchRunning) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *memStore) ListRecentByAgent(_ context.Context, kernelID string) (map[string]kerneldomain.Dispatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := map[string]kerneldomain.Dispatch{}
	for _, d := range s.dispatches {
		if d.KernelID == kernelID {
			if existing, ok := result[d.AgentID]; !ok || d.CreatedAt.After(existing.CreatedAt) {
				result[d.AgentID] = d
			}
		}
	}
	return result, nil
}

func (s *memStore) RecoverStaleRunning(_ context.Context, _ string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recoverHits++
	if s.recoverErr != nil {
		return 0, s.recoverErr
	}
	return 0, nil
}

func newTestEngine(t *testing.T, exec Executor) (*Engine, *memStore) {
	t.Helper()
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)
	planner := NewStaticPlanner("test-kernel", []AgentConfig{
		{AgentID: "agent-a", Prompt: "STATE: {STATE}\nDo A.", Priority: 10, Enabled: true},
		{AgentID: "agent-b", Prompt: "STATE: {STATE}\nDo B.", Priority: 5, Enabled: true},
	})
	cfg := KernelConfig{
		KernelID:      "test-kernel",
		Schedule:      "*/10 * * * *",
		SeedState:     "# STATE\n## identity\ntest\n",
		MaxConcurrent: 2,
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("test"))
	return engine, store
}

func TestEngine_RunCycle_EmptyPlan(t *testing.T) {
	exec := &mockExecutor{}
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)
	planner := NewStaticPlanner("k", nil) // no agents
	cfg := KernelConfig{
		KernelID: "k",
		Schedule: "*/10 * * * *",
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("test"))

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CycleSuccess {
		t.Errorf("expected success, got %s", result.Status)
	}
	if result.Dispatched != 0 {
		t.Errorf("expected 0 dispatched, got %d", result.Dispatched)
	}
}

func TestEngine_RunCycle_AllSucceed(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"done a", "done b"}}
	engine, store := newTestEngine(t, exec)

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CycleSuccess {
		t.Errorf("expected success, got %s", result.Status)
	}
	if result.Dispatched != 2 {
		t.Errorf("expected 2 dispatched, got %d", result.Dispatched)
	}
	if result.Succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
	if len(result.AgentSummary) != 2 {
		t.Fatalf("expected 2 agent summaries, got %d", len(result.AgentSummary))
	}
	if result.AgentSummary[0].Summary == "" || result.AgentSummary[1].Summary == "" {
		t.Fatalf("expected non-empty summaries: %#v", result.AgentSummary)
	}
	if !strings.Contains(result.AgentSummary[0].Summary, "autonomy=actionable") ||
		!strings.Contains(result.AgentSummary[1].Summary, "autonomy=actionable") {
		t.Fatalf("expected autonomy marker in summaries: %#v", result.AgentSummary)
	}
	if exec.callCount() != 2 {
		t.Errorf("expected 2 executor calls, got %d", exec.callCount())
	}
	// All dispatches should be done.
	store.mu.Lock()
	for _, d := range store.dispatches {
		if d.Status != kerneldomain.DispatchDone {
			t.Errorf("dispatch %s should be done, got %s", d.DispatchID, d.Status)
		}
	}
	store.mu.Unlock()
}

func TestWriteKernelStateFallback(t *testing.T) {
	content := "# STATE\nfallback\n"
	original, err := os.ReadFile(kernelStateFallbackPath())
	originalExists := err == nil
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read existing fallback: %v", err)
	}
	t.Cleanup(func() {
		if originalExists {
			_ = os.WriteFile(kernelStateFallbackPath(), original, 0o644)
		} else {
			_ = os.Remove(kernelStateFallbackPath())
		}
	})

	path, err := WriteKernelStateFallback(content)
	if err != nil {
		t.Fatalf("WriteKernelStateFallback: %v", err)
	}
	if path != kernelStateFallbackPath() {
		t.Fatalf("expected path %s, got %s", kernelStateFallbackPath(), path)
	}
	data, err := os.ReadFile(kernelStateFallbackPath())
	if err != nil {
		t.Fatalf("read fallback: %v", err)
	}
	if string(data) != content {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestIsSandboxPathRestriction(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "permission", err: fs.ErrPermission, want: true},
		{name: "wrapped permission", err: fmt.Errorf("open: %w", fs.ErrPermission), want: true},
		{name: "path error", err: &os.PathError{Op: "open", Path: "/restricted", Err: fs.ErrPermission}, want: true},
		{name: "permission denied string", err: errors.New("permission denied"), want: true},
		{name: "sandbox restriction", err: errors.New("sandbox path restriction: denied"), want: true},
		{name: "path guard", err: errors.New("path must stay within the working directory"), want: true},
		{name: "other", err: errors.New("disk full"), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSandboxPathRestriction(tc.err); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestRenderKernelRuntimeBlockWithHistory_IncludesFallbackPath(t *testing.T) {
	block := renderKernelRuntimeBlockWithHistory(nil, nil, time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC), nil, kernelStateFallbackPath())
	if !strings.Contains(block, "state_write_fallback") {
		t.Fatalf("expected fallback note, got %q", block)
	}
	if !strings.Contains(block, kernelStateFallbackPath()) {
		t.Fatalf("expected fallback path in block, got %q", block)
	}
}

func TestPersistSystemPromptSnapshot_WritesFallbackWhenRestricted(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)
	engine.stateWriteRestricted.Store(true)
	engine.SetSystemPromptProvider(func() string { return "kernel prompt v2" })

	original, err := os.ReadFile(kernelStateFallbackPath())
	originalExists := err == nil
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read existing fallback: %v", err)
	}
	if originalExists {
		if err := os.Remove(kernelStateFallbackPath()); err != nil {
			t.Fatalf("remove existing fallback: %v", err)
		}
	}
	t.Cleanup(func() {
		if originalExists {
			_ = os.WriteFile(kernelStateFallbackPath(), original, 0o644)
		} else {
			_ = os.Remove(kernelStateFallbackPath())
		}
	})

	engine.persistSystemPromptSnapshot()

	if _, err := os.Stat(engine.stateFile.SystemPromptPath()); err == nil {
		t.Fatalf("expected no system prompt snapshot write when restricted")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat system prompt: %v", err)
	}
	data, err := os.ReadFile(kernelStateFallbackPath())
	if err != nil {
		t.Fatalf("read fallback: %v", err)
	}
	if !strings.Contains(string(data), "SYSTEM_PROMPT.md fallback") {
		t.Fatalf("expected fallback section, got %q", string(data))
	}
	if !strings.Contains(string(data), "kernel prompt v2") {
		t.Fatalf("expected fallback prompt, got %q", string(data))
	}
}

func TestEngine_RunCycle_PassesRoutingMeta(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"done a", "done b"}}
	engine, _ := newTestEngine(t, exec)
	engine.config.Channel = "lark"
	engine.config.ChatID = "oc_chat"
	engine.config.UserID = "ou_user"

	if _, err := engine.RunCycle(context.Background()); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	calls := exec.getCalls()
	if len(calls) == 0 {
		t.Fatal("expected at least one executor call")
	}
	for _, call := range calls {
		if got := call.Meta["channel"]; got != "lark" {
			t.Fatalf("expected channel=lark, got %q", got)
		}
		if got := call.Meta["chat_id"]; got != "oc_chat" {
			t.Fatalf("expected chat_id=oc_chat, got %q", got)
		}
		if got := call.Meta["user_id"]; got != "ou_user" {
			t.Fatalf("expected user_id=ou_user, got %q", got)
		}
	}
}

func TestEngine_RunCycle_PartialFailure(t *testing.T) {
	failExec := &failingExecutor{
		inner:     &mockExecutor{summaries: []string{"ok"}},
		failAgent: "agent-b",
	}
	engine, _ := newTestEngine(t, failExec)

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CyclePartialSuccess {
		t.Errorf("expected partial_success, got %s", result.Status)
	}
	if result.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %d", result.Succeeded)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
	if len(result.FailedAgents) != 1 || result.FailedAgents[0] != "agent-b" {
		t.Errorf("expected failed agent-b, got %v", result.FailedAgents)
	}
	if len(result.AgentSummary) != 2 {
		t.Fatalf("expected 2 agent summaries, got %d", len(result.AgentSummary))
	}
}

func TestEngine_RunCycle_AllFail(t *testing.T) {
	exec := &mockExecutor{err: fmt.Errorf("always fail")}
	engine, _ := newTestEngine(t, exec)

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CycleFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if result.Failed != 2 {
		t.Errorf("expected 2 failed, got %d", result.Failed)
	}
}

func TestEngine_RunCycle_SeedState(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"done a", "done b"}}
	engine, _ := newTestEngine(t, exec)

	// First cycle should seed the state file.
	_, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	// Verify state was seeded.
	content, err := engine.stateFile.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(content, engine.config.SeedState) {
		t.Errorf("state missing seed content: got %q", content)
	}
	if !strings.Contains(content, kernelRuntimeSectionStart) || !strings.Contains(content, kernelRuntimeSectionEnd) {
		t.Fatalf("state missing kernel runtime markers: %q", content)
	}
	if !strings.Contains(content, "## kernel_runtime") {
		t.Fatalf("state missing kernel runtime section: %q", content)
	}
	if !strings.Contains(content, "- latest_agent_summary: ") {
		t.Fatalf("state missing agent summary line: %q", content)
	}
	if !strings.Contains(content, "### cycle_history") {
		t.Fatalf("state missing cycle history section: %q", content)
	}
}

func TestEngine_RunCycle_RuntimeSectionUpsertedOnce(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)

	if _, err := engine.RunCycle(context.Background()); err != nil {
		t.Fatalf("first RunCycle: %v", err)
	}
	if _, err := engine.RunCycle(context.Background()); err != nil {
		t.Fatalf("second RunCycle: %v", err)
	}

	content, err := engine.stateFile.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got := strings.Count(content, kernelRuntimeSectionStart); got != 1 {
		t.Fatalf("expected exactly one runtime section start marker, got %d: %q", got, content)
	}
	if got := strings.Count(content, kernelRuntimeSectionEnd); got != 1 {
		t.Fatalf("expected exactly one runtime section end marker, got %d: %q", got, content)
	}
}

func TestEngine_RunCycle_RefreshesSystemPromptSnapshot(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)
	engine.SetSystemPromptProvider(func() string {
		return "kernel prompt v2"
	})

	if _, err := engine.RunCycle(context.Background()); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	content, err := engine.stateFile.ReadSystemPrompt()
	if err != nil {
		t.Fatalf("ReadSystemPrompt: %v", err)
	}
	if !strings.Contains(content, "kernel prompt v2") {
		t.Fatalf("expected system prompt snapshot refresh, got: %q", content)
	}
}

func TestEngine_RunCycle_RunningAgentNotReDispatched(t *testing.T) {
	exec := &mockExecutor{}
	engine, store := newTestEngine(t, exec)

	// Simulate agent-a as still running.
	store.mu.Lock()
	store.dispatches = append(store.dispatches, kerneldomain.Dispatch{
		DispatchID: "prev-1",
		KernelID:   "test-kernel",
		AgentID:    "agent-a",
		Status:     kerneldomain.DispatchRunning,
	})
	store.mu.Unlock()

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Only agent-b should be dispatched.
	if result.Dispatched != 1 {
		t.Errorf("expected 1 dispatched, got %d", result.Dispatched)
	}
	calls := exec.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].AgentID != "agent-b" {
		t.Errorf("expected agent-b, got %s", calls[0].AgentID)
	}
}

func TestEngine_SetNotifier(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)

	if engine.notifier != nil {
		t.Fatal("notifier should be nil by default")
	}

	called := false
	engine.SetNotifier(func(_ context.Context, _ *kerneldomain.CycleResult, _ error) {
		called = true
	})
	if engine.notifier == nil {
		t.Fatal("notifier should be set after SetNotifier")
	}

	// Simulate what Run() does: call RunCycle, then invoke notifier on non-empty cycle.
	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Dispatched > 0 {
		engine.notifier(context.Background(), result, nil)
	}
	if !called {
		t.Error("notifier should have been called for non-empty cycle")
	}
}

func TestEngine_NotifierNotCalledOnEmptyCycle(t *testing.T) {
	exec := &mockExecutor{}
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)
	planner := NewStaticPlanner("k", nil) // no agents → empty cycle
	cfg := KernelConfig{
		KernelID:  "k",
		Schedule:  "*/10 * * * *",
		SeedState: "# test\n",
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("test"))

	called := false
	engine.SetNotifier(func(_ context.Context, _ *kerneldomain.CycleResult, _ error) {
		called = true
	})

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Simulate Run()'s logic: only notify when dispatched > 0 or error.
	if err != nil || (result != nil && result.Dispatched > 0) {
		engine.notifier(context.Background(), result, err)
	}
	if called {
		t.Error("notifier should NOT be called for empty cycle")
	}
}

func TestEngine_NotifierCalledOnError(t *testing.T) {
	exec := &mockExecutor{}
	store := newMemStore()
	badFile, err := os.CreateTemp("", "kernel-state-file")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	badPath := badFile.Name()
	t.Cleanup(func() {
		_ = os.Remove(badPath)
	})
	if err := badFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	sf := NewStateFile(badPath)
	planner := NewStaticPlanner("k", []AgentConfig{
		{AgentID: "a", Prompt: "p", Enabled: true},
	})
	cfg := KernelConfig{
		KernelID: "k",
		Schedule: "*/10 * * * *",
		// No SeedState → state file read will return empty, then Seed will fail on bad dir.
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("test"))

	var gotErr error
	engine.SetNotifier(func(_ context.Context, _ *kerneldomain.CycleResult, err error) {
		gotErr = err
	})

	result, cycleErr := engine.RunCycle(context.Background())
	// Simulate Run()'s logic.
	if cycleErr != nil || (result != nil && result.Dispatched > 0) {
		engine.notifier(context.Background(), result, cycleErr)
	}
	if cycleErr == nil {
		t.Fatal("expected RunCycle error on bad state dir")
	}
	if gotErr == nil {
		t.Error("notifier should have received the error")
	}
}

func TestEngine_StopIdempotent(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)
	// Should not panic when called multiple times.
	engine.Stop()
	engine.Stop()
}

func TestEngine_RunCycle_RecoversStaleDispatchesWhenSupported(t *testing.T) {
	exec := &mockExecutor{}
	engine, store := newTestEngine(t, exec)

	if _, err := engine.RunCycle(context.Background()); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.recoverHits == 0 {
		t.Fatal("expected stale recovery hook to be called")
	}
}

func TestEngine_RunCycle_RollingHistory(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"done a", "done b"}}
	engine, _ := newTestEngine(t, exec)
	engine.config.MaxCycleHistory = 10

	// Run 3 cycles.
	for i := 0; i < 3; i++ {
		exec.mu.Lock()
		exec.idx = 0
		exec.mu.Unlock()
		if _, err := engine.RunCycle(context.Background()); err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
	}

	content, err := engine.stateFile.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	// Parse the history table — should have 3 rows.
	history := parseCycleHistory(content)
	if len(history) != 3 {
		t.Fatalf("expected 3 history rows, got %d", len(history))
	}
	// Most recent should be first.
	if history[0].Status != "success" {
		t.Errorf("expected latest status=success, got %q", history[0].Status)
	}
}

func TestEngine_RunCycle_RollingHistoryTruncation(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"done a", "done b"}}
	engine, _ := newTestEngine(t, exec)
	engine.config.MaxCycleHistory = 5

	// Run 7 cycles.
	for i := 0; i < 7; i++ {
		exec.mu.Lock()
		exec.idx = 0
		exec.mu.Unlock()
		if _, err := engine.RunCycle(context.Background()); err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
	}

	content, err := engine.stateFile.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	history := parseCycleHistory(content)
	if len(history) != 5 {
		t.Fatalf("expected 5 history rows (truncated), got %d", len(history))
	}
}

// failingExecutor wraps another executor and fails for a specific agent.
type failingExecutor struct {
	inner     Executor
	failAgent string
}

func (f *failingExecutor) Execute(ctx context.Context, agentID, prompt string, meta map[string]string) (ExecutionResult, error) {
	if agentID == f.failAgent {
		return ExecutionResult{}, fmt.Errorf("simulated failure for %s", agentID)
	}
	return f.inner.Execute(ctx, agentID, prompt, meta)
}
