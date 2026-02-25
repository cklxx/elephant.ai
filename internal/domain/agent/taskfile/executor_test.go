package taskfile

import (
	"context"
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
	exec := NewExecutor(mock)

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
	exec := NewExecutor(mock)

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
	exec := NewExecutor(mock)

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
	exec := NewExecutor(mock)

	_, err := exec.Execute(context.Background(), tf, "my-causation", statusPath)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if mock.dispatched[0].CausationID != "my-causation" {
		t.Errorf("CausationID: got %q, want %q", mock.dispatched[0].CausationID, "my-causation")
	}
}
