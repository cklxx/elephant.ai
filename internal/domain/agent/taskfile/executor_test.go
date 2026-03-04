package taskfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

type mockDispatcher struct {
	mu          sync.Mutex
	dispatched  []agent.BackgroundDispatchRequest
	statusFn    func(ids []string) []agent.BackgroundTaskSummary
	collectFn   func(ids []string, wait bool, timeout time.Duration) []agent.BackgroundTaskResult
}

func (m *mockDispatcher) Dispatch(_ context.Context, req agent.BackgroundDispatchRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dispatched = append(m.dispatched, req)
	return nil
}

func (m *mockDispatcher) Status(ids []string) []agent.BackgroundTaskSummary {
	if m.statusFn != nil {
		return m.statusFn(ids)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []agent.BackgroundTaskSummary
	for _, req := range m.dispatched {
		out = append(out, agent.BackgroundTaskSummary{
			ID:     req.TaskID,
			Status: agent.BackgroundTaskStatusCompleted,
		})
	}
	return out
}

func (m *mockDispatcher) Collect(ids []string, wait bool, timeout time.Duration) []agent.BackgroundTaskResult {
	if m.collectFn != nil {
		return m.collectFn(ids, wait, timeout)
	}
	var out []agent.BackgroundTaskResult
	for _, id := range ids {
		out = append(out, agent.BackgroundTaskResult{
			ID:     id,
			Status: agent.BackgroundTaskStatusCompleted,
			Answer: "done",
		})
	}
	return out
}

func TestExecutor_DispatchesInTopoOrder(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "exec-test",
		Tasks: []TaskSpec{
			{ID: "b", Prompt: "do B", DependsOn: []string{"a"}},
			{ID: "a", Prompt: "do A"},
		},
	}

	mock := &mockDispatcher{}
	statusPath := filepath.Join(t.TempDir(), "test.status.yaml")
	exec := NewExecutor(mock, ModeTeam, DefaultSwarmConfig())

	result, err := exec.Execute(context.Background(), tf, "cause-1", statusPath)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(mock.dispatched) != 2 {
		t.Fatalf("expected 2 dispatches, got %d", len(mock.dispatched))
	}
	if mock.dispatched[0].TaskID != "a" {
		t.Errorf("first dispatch should be 'a', got %q", mock.dispatched[0].TaskID)
	}
	if mock.dispatched[1].TaskID != "b" {
		t.Errorf("second dispatch should be 'b', got %q", mock.dispatched[1].TaskID)
	}
	if result.PlanID != "exec-test" {
		t.Errorf("plan_id: got %q", result.PlanID)
	}

	// Check status file was written.
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(string(data), "exec-test") {
		t.Error("status file should contain plan_id")
	}
}

func TestExecutor_ValidationError(t *testing.T) {
	tf := &TaskFile{Version: "1"} // no tasks

	mock := &mockDispatcher{}
	exec := NewExecutor(mock, ModeTeam, DefaultSwarmConfig())

	_, err := exec.Execute(context.Background(), tf, "cause-1", "/tmp/test.status.yaml")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "validate") {
		t.Errorf("error should mention validate: %v", err)
	}
}

func TestExecutor_ExecuteAndWait(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "wait-test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
		},
	}

	mock := &mockDispatcher{}
	statusPath := filepath.Join(t.TempDir(), "wait.status.yaml")
	exec := NewExecutor(mock, ModeTeam, DefaultSwarmConfig())

	result, err := exec.ExecuteAndWait(context.Background(), tf, "cause-1", statusPath, 5*time.Second)
	if err != nil {
		t.Fatalf("ExecuteAndWait: %v", err)
	}
	if len(result.TaskIDs) != 1 {
		t.Errorf("expected 1 task, got %d", len(result.TaskIDs))
	}
}

func TestExecutor_CausationIDPropagated(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "cause-test",
		Tasks:   []TaskSpec{{ID: "a", Prompt: "do A"}},
	}

	mock := &mockDispatcher{}
	statusPath := filepath.Join(t.TempDir(), "cause.status.yaml")
	exec := NewExecutor(mock, ModeTeam, DefaultSwarmConfig())

	_, err := exec.Execute(context.Background(), tf, "my-causation", statusPath)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if mock.dispatched[0].CausationID != "my-causation" {
		t.Errorf("CausationID: got %q, want %q", mock.dispatched[0].CausationID, "my-causation")
	}
}

func TestExecutor_SwarmMode(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "swarm-exec-test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B"},
			{ID: "c", Prompt: "do C", DependsOn: []string{"a", "b"}},
		},
	}

	mock := &mockDispatcher{}
	statusPath := filepath.Join(t.TempDir(), "swarm.status.yaml")
	exec := NewExecutor(mock, ModeSwarm, DefaultSwarmConfig())

	result, err := exec.Execute(context.Background(), tf, "cause-swarm", statusPath)
	if err != nil {
		t.Fatalf("Execute (swarm): %v", err)
	}
	if len(result.TaskIDs) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result.TaskIDs))
	}
	// Swarm should clear DependsOn before dispatch.
	for _, req := range mock.dispatched {
		if len(req.DependsOn) > 0 {
			t.Errorf("swarm dispatch of %q should have cleared DependsOn", req.TaskID)
		}
	}
}

func TestExecutor_AutoMode_SelectsSwarm(t *testing.T) {
	// All independent → auto should select swarm
	tf := &TaskFile{
		Version: "1",
		PlanID:  "auto-swarm-test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B"},
			{ID: "c", Prompt: "do C"},
		},
	}

	mock := &mockDispatcher{}
	statusPath := filepath.Join(t.TempDir(), "auto.status.yaml")
	exec := NewExecutor(mock, ModeAuto, DefaultSwarmConfig())

	result, err := exec.Execute(context.Background(), tf, "cause-auto", statusPath)
	if err != nil {
		t.Fatalf("Execute (auto): %v", err)
	}
	if len(result.TaskIDs) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result.TaskIDs))
	}
}

func TestExecutor_AutoMode_SelectsTeam(t *testing.T) {
	// inherit_context → auto should select team
	tf := &TaskFile{
		Version: "1",
		PlanID:  "auto-team-test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B", InheritContext: true, DependsOn: []string{"a"}},
		},
	}

	mock := &mockDispatcher{}
	statusPath := filepath.Join(t.TempDir(), "auto-team.status.yaml")
	exec := NewExecutor(mock, ModeAuto, DefaultSwarmConfig())

	result, err := exec.Execute(context.Background(), tf, "cause-team", statusPath)
	if err != nil {
		t.Fatalf("Execute (auto→team): %v", err)
	}
	if len(result.TaskIDs) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(result.TaskIDs))
	}
	// Team mode preserves DependsOn.
	for _, req := range mock.dispatched {
		if req.TaskID == "b" && len(req.DependsOn) == 0 {
			t.Error("team dispatch of 'b' should preserve DependsOn")
		}
	}
}

func TestExecutor_DispatchErrorMidExecution(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "dispatch-err-test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B", DependsOn: []string{"a"}},
			{ID: "c", Prompt: "do C", DependsOn: []string{"b"}},
		},
	}

	mock := &mockDispatcher{}
	failOnSecond := &failingDispatcher{
		inner:     mock,
		failAfter: 1, // fail on dispatch index 1 (the second task)
	}

	statusPath := filepath.Join(t.TempDir(), "dispatch-err.status.yaml")
	exec := NewExecutor(failOnSecond, ModeTeam, DefaultSwarmConfig())

	_, err := exec.Execute(context.Background(), tf, "cause-err", statusPath)
	if err == nil {
		t.Fatal("expected dispatch error")
	}
	if !strings.Contains(err.Error(), "dispatch task") {
		t.Errorf("error should mention dispatch task: %v", err)
	}

	// Only the first task should have been dispatched before the error.
	failOnSecond.mu.Lock()
	dispatched := len(failOnSecond.dispatched)
	failOnSecond.mu.Unlock()
	if dispatched != 1 {
		t.Errorf("expected 1 successful dispatch before error, got %d", dispatched)
	}
}

func TestExecutor_ContextCancellation(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "ctx-cancel-test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B", DependsOn: []string{"a"}},
			{ID: "c", Prompt: "do C", DependsOn: []string{"b"}},
		},
	}

	// Cancel context before executing; the executor checks ctx.Err() in the
	// dispatch loop so it should bail out.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	mock := &mockDispatcher{}
	statusPath := filepath.Join(t.TempDir(), "ctx-cancel.status.yaml")
	exec := NewExecutor(mock, ModeTeam, DefaultSwarmConfig())

	_, err := exec.Execute(ctx, tf, "cause-cancel", statusPath)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("error should mention context canceled: %v", err)
	}
	// No tasks should have been dispatched.
	if len(mock.dispatched) != 0 {
		t.Errorf("expected 0 dispatches with cancelled context, got %d", len(mock.dispatched))
	}
}

// failingDispatcher wraps a dispatcher and returns an error after failAfter
// successful dispatches.
type failingDispatcher struct {
	inner     *mockDispatcher
	failAfter int
	mu        sync.Mutex
	dispatched []agent.BackgroundDispatchRequest
	count     int
}

func (f *failingDispatcher) Dispatch(ctx context.Context, req agent.BackgroundDispatchRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.count >= f.failAfter {
		return fmt.Errorf("simulated dispatch failure for %s", req.TaskID)
	}
	f.count++
	f.dispatched = append(f.dispatched, req)
	return f.inner.Dispatch(ctx, req)
}

func (f *failingDispatcher) Status(ids []string) []agent.BackgroundTaskSummary {
	return f.inner.Status(ids)
}

func (f *failingDispatcher) Collect(ids []string, wait bool, timeout time.Duration) []agent.BackgroundTaskResult {
	return f.inner.Collect(ids, wait, timeout)
}

func TestExecutor_ExecuteAndWait_FinalSyncRehydratesStatusFile(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "rehydrate-test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B", DependsOn: []string{"a"}},
		},
	}

	mock := &mockDispatcher{
		statusFn: func(ids []string) []agent.BackgroundTaskSummary {
			out := make([]agent.BackgroundTaskSummary, 0, len(ids))
			for _, id := range ids {
				out = append(out, agent.BackgroundTaskSummary{
					ID:     id,
					Status: agent.BackgroundTaskStatusCompleted,
				})
			}
			return out
		},
	}
	statusPath := filepath.Join(t.TempDir(), "rehydrate.status.yaml")
	exec := NewExecutor(mock, ModeTeam, DefaultSwarmConfig())

	_, err := exec.ExecuteAndWait(context.Background(), tf, "cause-rehydrate", statusPath, 2*time.Second)
	if err != nil {
		t.Fatalf("ExecuteAndWait: %v", err)
	}

	sf, err := ReadStatusFile(statusPath)
	if err != nil {
		t.Fatalf("ReadStatusFile: %v", err)
	}
	if len(sf.Tasks) != 2 {
		t.Fatalf("expected 2 status rows, got %d", len(sf.Tasks))
	}
	for _, task := range sf.Tasks {
		if task.Status != string(agent.BackgroundTaskStatusCompleted) {
			t.Fatalf("task %s status = %s, want completed", task.ID, task.Status)
		}
	}
}
