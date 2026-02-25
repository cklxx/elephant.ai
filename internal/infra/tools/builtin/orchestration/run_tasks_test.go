package orchestration

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

type mockBGDispatcher struct {
	mu         sync.Mutex
	dispatched []agent.BackgroundDispatchRequest
}

func (m *mockBGDispatcher) Dispatch(_ context.Context, req agent.BackgroundDispatchRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dispatched = append(m.dispatched, req)
	return nil
}

func (m *mockBGDispatcher) Status(ids []string) []agent.BackgroundTaskSummary {
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

func (m *mockBGDispatcher) Collect(ids []string, wait bool, timeout time.Duration) []agent.BackgroundTaskResult {
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

func TestRunTasks_FileMode(t *testing.T) {
	// Create a temp task file.
	dir := t.TempDir()
	taskFilePath := filepath.Join(dir, "tasks.yaml")
	content := `
version: "1"
plan_id: "test-plan"
tasks:
  - id: "task-1"
    description: "first task"
    prompt: "do something"
    agent_type: "internal"
  - id: "task-2"
    description: "second task"
    prompt: "do something else"
    agent_type: "internal"
    depends_on: ["task-1"]
`
	if err := os.WriteFile(taskFilePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	mock := &mockBGDispatcher{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"file": taskFilePath,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %s", result.Content)
	}

	mock.mu.Lock()
	count := len(mock.dispatched)
	mock.mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 dispatched tasks, got %d", count)
	}
}

func TestRunTasks_MissingFileAndTemplate(t *testing.T) {
	mock := &mockBGDispatcher{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:        "call-1",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for missing file/template")
	}
}

func TestRunTasks_NoDispatcher(t *testing.T) {
	tool := NewRunTasks()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"file": "/nonexistent",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error when no dispatcher")
	}
}

func TestRunTasks_WaitMode(t *testing.T) {
	dir := t.TempDir()
	taskFilePath := filepath.Join(dir, "tasks.yaml")
	content := `
version: "1"
plan_id: "wait-plan"
tasks:
  - id: "task-1"
    prompt: "do it"
`
	if err := os.WriteFile(taskFilePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mock := &mockBGDispatcher{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"file": taskFilePath,
			"wait": true,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %s", result.Content)
	}
}

func TestRunTasks_TaskIDFilter(t *testing.T) {
	dir := t.TempDir()
	taskFilePath := filepath.Join(dir, "tasks.yaml")
	content := `
version: "1"
plan_id: "filter-plan"
tasks:
  - id: "a"
    prompt: "do A"
  - id: "b"
    prompt: "do B"
  - id: "c"
    prompt: "do C"
`
	if err := os.WriteFile(taskFilePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mock := &mockBGDispatcher{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"file":     taskFilePath,
			"task_ids": []any{"a", "c"},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	mock.mu.Lock()
	count := len(mock.dispatched)
	mock.mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 filtered tasks dispatched, got %d", count)
	}
}

func TestRunTasks_TemplateList(t *testing.T) {
	mock := &mockBGDispatcher{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{
		{
			Name:        "test-team",
			Description: "A test team",
			Roles:       []agent.TeamRoleDefinition{{Name: "worker", AgentType: "codex"}},
			Stages:      []agent.TeamStageDefinition{{Name: "do", Roles: []string{"worker"}}},
		},
	})

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"template": "list",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if result.Content == "" {
		t.Error("expected non-empty template listing")
	}
}
