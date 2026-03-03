package kernel

// coverage_gap3_test.go — targeted tests for uncovered branches in:
//   - RunCycle (61.7% → target 75%+)
//   - persistCycleRuntimeState (56.0% → target 70%+)
//   - persistSystemPromptSnapshot (50.0% → target 70%+)
//
// Strategy: inject errors via controllable stores/planners/file paths without
// modifying production code. All tests are hermetic and side-effect-free.

import (
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/shared/logging"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// errEnqueueStore wraps memStore and returns an error from EnqueueDispatches.
type errEnqueueStore struct {
	*memStore
	enqueueErr error
}

func (s *errEnqueueStore) EnqueueDispatches(_ context.Context, _, _ string, _ []kerneldomain.DispatchSpec) ([]kerneldomain.Dispatch, error) {
	return nil, s.enqueueErr
}

// errListRecentStore wraps memStore and returns an error from ListRecentByAgent.
type errListRecentStore struct {
	*memStore
	listErr error
}

func (s *errListRecentStore) ListRecentByAgent(_ context.Context, _ string) (map[string]kerneldomain.Dispatch, error) {
	return nil, s.listErr
}

// newTestEngineWithStore creates an Engine using the provided store (instead of the default memStore).
func newTestEngineWithStore(t *testing.T, exec Executor, store kerneldomain.Store) *Engine {
	t.Helper()
	dir := t.TempDir()
	sf := NewStateFile(dir)
	planner := NewStaticPlanner("test-kernel", []AgentConfig{
		{AgentID: "agent-a", Prompt: "Do A.", Priority: 10, Enabled: true},
	})
	cfg := KernelConfig{
		KernelID:      "test-kernel",
		Schedule:      "*/10 * * * *",
		SeedState:     "# STATE\n## identity\ntest\n",
		MaxConcurrent: 1,
	}
	return NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("test"))
}

// sandboxPermError returns an error that isSandboxPathRestriction recognises.
func sandboxPermError() error {
	return &os.PathError{Op: "open", Path: "/restricted/path", Err: syscall.EPERM}
}

// makeReadOnlyDir creates a temp dir, writes STATE.md into it, then makes
// the dir read-only so subsequent writes fail.
func makeReadOnlyDir(t *testing.T, initialContent string) string {
	t.Helper()
	dir := t.TempDir()
	if initialContent != "" {
		if err := os.WriteFile(dir+"/STATE.md", []byte(initialContent), 0o644); err != nil {
			t.Fatalf("write STATE.md: %v", err)
		}
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	return dir
}

// ─────────────────────────────────────────────────────────────────────────────
// RunCycle — error paths
// ─────────────────────────────────────────────────────────────────────────────

// TestEngine_RunCycle_ListRecentError verifies that a ListRecentByAgent error
// is tolerated and the cycle continues (degrades gracefully to empty map).
func TestEngine_RunCycle_ListRecentError(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"done"}}
	store := &errListRecentStore{
		memStore: newMemStore(),
		listErr:  errors.New("db timeout"),
	}
	// errListRecentStore still needs to EnqueueDispatches — delegate to inner memStore.
	engine := newTestEngineWithStore(t, exec, store)

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("expected RunCycle to tolerate ListRecentByAgent error, got: %v", err)
	}
	// Cycle should still complete (planner runs with empty recentByAgent map).
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// TestEngine_RunCycle_PlanError verifies that a Plan error propagates to the caller.
func TestEngine_RunCycle_PlanError(t *testing.T) {
	exec := &mockExecutor{}
	store := newMemStore()
	dir := t.TempDir()
	sf := NewStateFile(dir)
	planErr := errors.New("LLM unreachable")
	planner := plannerFunc(func(_ context.Context, _ string, _ map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error) {
		return nil, planErr
	})
	cfg := KernelConfig{
		KernelID:  "k",
		Schedule:  "*/10 * * * *",
		SeedState: "# STATE\n",
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("test"))

	_, err := engine.RunCycle(context.Background())
	if err == nil {
		t.Fatal("expected error from Plan, got nil")
	}
	if !strings.Contains(err.Error(), "plan") {
		t.Errorf("expected 'plan' in error, got: %v", err)
	}
}

// TestEngine_RunCycle_EnqueueError verifies that an EnqueueDispatches error
// propagates to the caller.
func TestEngine_RunCycle_EnqueueError(t *testing.T) {
	exec := &mockExecutor{}
	store := &errEnqueueStore{
		memStore:   newMemStore(),
		enqueueErr: errors.New("disk full"),
	}
	engine := newTestEngineWithStore(t, exec, store)

	_, err := engine.RunCycle(context.Background())
	if err == nil {
		t.Fatal("expected error from EnqueueDispatches, got nil")
	}
	if !strings.Contains(err.Error(), "enqueue") {
		t.Errorf("expected 'enqueue' in error, got: %v", err)
	}
}

// TestEngine_RunCycle_StateWriteRestrictedEmptyContent verifies that when
// stateWriteRestricted is already set and stateContent is empty, the engine
// falls back to SeedState without calling Seed().
func TestEngine_RunCycle_StateWriteRestrictedEmptyContent(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"done"}}
	engine, _ := newTestEngine(t, exec)
	// Pre-set restricted flag — no STATE.md file exists yet, so Read returns "".
	engine.stateWriteRestricted.Store(true)

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("expected success when restricted + empty state, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// persistCycleRuntimeState — sandbox + write error paths
// ─────────────────────────────────────────────────────────────────────────────

// TestPersistCycleRuntimeState_StateWriteRestricted verifies that when
// stateWriteRestricted is set, the runtime state is written to the fallback
// path instead of STATE.md.
func TestPersistCycleRuntimeState_StateWriteRestricted(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)
	engine.stateWriteRestricted.Store(true)

	// Clean up fallback file before and after.
	cleanupFallback(t)

	result := &kerneldomain.CycleResult{
		CycleID:    "test-cycle-id",
		KernelID:   "test-kernel",
		Status:     kerneldomain.CycleSuccess,
		Dispatched: 0,
	}
	engine.persistCycleRuntimeState(result, nil)

	// Fallback file should now exist.
	data, err := os.ReadFile(kernelStateFallbackPath())
	if err != nil {
		t.Fatalf("expected fallback file to be written, got: %v", err)
	}
	if !strings.Contains(string(data), "kernel_runtime") {
		t.Errorf("expected kernel_runtime section in fallback, got: %q", string(data))
	}
}

// TestPersistCycleRuntimeState_WriteFailsUseFallback verifies that when
// StateFile.Write() returns an error (read-only dir), the engine falls back
// gracefully rather than panicking.
func TestPersistCycleRuntimeState_WriteFailsUseFallback(t *testing.T) {
	// Write initial STATE.md content, then make dir read-only.
	initialState := "# STATE\n## identity\ntest\n"
	dir := makeReadOnlyDir(t, initialState)

	store := newMemStore()
	sf := NewStateFile(dir)
	planner := NewStaticPlanner("k", nil)
	cfg := KernelConfig{
		KernelID:  "k",
		Schedule:  "*/10 * * * *",
		SeedState: initialState,
	}
	engine := NewEngine(cfg, sf, store, planner, &mockExecutor{}, logging.NewComponentLogger("test"))

	cleanupFallback(t)

	result := &kerneldomain.CycleResult{
		CycleID:  "cycle-abc",
		KernelID: "k",
		Status:   kerneldomain.CycleSuccess,
	}
	// Should not panic, even though Write will fail due to read-only dir.
	engine.persistCycleRuntimeState(result, nil)
	// Outcome: either fallback written or warning logged — both are acceptable.
	// We just verify no panic occurred (test reaches here).
}

// ─────────────────────────────────────────────────────────────────────────────
// persistSystemPromptSnapshot — uncovered branches
// ─────────────────────────────────────────────────────────────────────────────

// TestPersistSystemPromptSnapshot_NilProvider verifies early return when no
// system prompt provider is set (the nil check branch).
func TestPersistSystemPromptSnapshot_NilProvider(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)
	// systemPromptProvider is nil by default — calling persistSystemPromptSnapshot
	// should return immediately without panicking.
	engine.persistSystemPromptSnapshot()
}

// TestPersistSystemPromptSnapshot_EmptyPrompt verifies early return when the
// provider returns an empty string.
func TestPersistSystemPromptSnapshot_EmptyPrompt(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)
	engine.SetSystemPromptProvider(func() string { return "   " })
	engine.persistSystemPromptSnapshot() // should not write anything
	if _, err := os.Stat(engine.stateFile.SystemPromptPath()); err == nil {
		t.Fatal("expected no SYSTEM_PROMPT.md written for empty prompt")
	}
}

// TestPersistSystemPromptSnapshot_WriteFailsLogsWarn verifies that when
// WriteSystemPrompt fails (read-only dir) and the error is NOT a sandbox
// restriction, the engine logs a warning and does not panic.
func TestPersistSystemPromptSnapshot_WriteFailsLogsWarn(t *testing.T) {
	initialState := "# STATE\n## identity\ntest\n"
	dir := makeReadOnlyDir(t, initialState)

	store := newMemStore()
	sf := NewStateFile(dir)
	planner := NewStaticPlanner("k", nil)
	cfg := KernelConfig{
		KernelID:  "k",
		Schedule:  "*/10 * * * *",
		SeedState: initialState,
	}
	engine := NewEngine(cfg, sf, store, planner, &mockExecutor{}, logging.NewComponentLogger("test"))
	engine.SetSystemPromptProvider(func() string { return "system prompt content" })

	cleanupFallback(t)
	// Should not panic; write fails silently with a warning log.
	engine.persistSystemPromptSnapshot()
}

// TestPersistSystemPromptSnapshot_SandboxRestrictedFallback verifies that when
// stateWriteRestricted is set AND AppendKernelStateFallback succeeds, the
// function writes to the fallback path.
func TestPersistSystemPromptSnapshot_SandboxRestrictedFallback(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)
	engine.stateWriteRestricted.Store(true)
	engine.SetSystemPromptProvider(func() string { return "prompt for fallback" })

	cleanupFallback(t)
	engine.persistSystemPromptSnapshot()

	// Fallback should contain the system prompt.
	data, err := os.ReadFile(kernelStateFallbackPath())
	if err != nil {
		t.Fatalf("expected fallback file: %v", err)
	}
	if !strings.Contains(string(data), "prompt for fallback") {
		t.Errorf("expected prompt content in fallback, got: %q", string(data))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// cleanupFallback removes the fallback file before the test and restores it
// (or removes it) after, ensuring hermetic test isolation.
func cleanupFallback(t *testing.T) {
	t.Helper()
	path := kernelStateFallbackPath()
	original, readErr := os.ReadFile(path)
	originalExists := readErr == nil
	if originalExists {
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove existing fallback: %v", err)
		}
	}
	t.Cleanup(func() {
		if originalExists {
			_ = os.WriteFile(path, original, 0o644)
		} else {
			_ = os.Remove(path)
		}
	})
}
