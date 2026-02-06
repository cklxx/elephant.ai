package ui

import (
	"context"
	"strings"
	"testing"

	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
)

func TestClarifyExecuteSourcesRunIDFromContext(t *testing.T) {
	tool := NewClarify()

	ctx := id.WithIDs(context.Background(), id.IDs{RunID: "ctx-run-789"})
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"task_goal_ui": "Sub-task goal",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	runID, ok := result.Metadata["run_id"].(string)
	if !ok || runID != "ctx-run-789" {
		t.Fatalf("expected run_id=ctx-run-789 from context, got %q", runID)
	}
}

func TestClarifyExecuteAutoGeneratesTaskID(t *testing.T) {
	tool := NewClarify()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-42",
		Arguments: map[string]any{
			"task_goal_ui": "Auto task",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	taskID, ok := result.Metadata["task_id"].(string)
	if !ok || taskID != "task-call-42" {
		t.Fatalf("expected task_id=task-call-42, got %q", taskID)
	}
}

func TestClarifyExecuteUsesProvidedTaskID(t *testing.T) {
	tool := NewClarify()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-5",
		Arguments: map[string]any{
			"task_id":      "my-custom-task",
			"task_goal_ui": "Custom task",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	taskID, ok := result.Metadata["task_id"].(string)
	if !ok || taskID != "my-custom-task" {
		t.Fatalf("expected task_id=my-custom-task, got %q", taskID)
	}
}

func TestClarifyExecuteIgnoresLLMSuppliedRunID(t *testing.T) {
	tool := NewClarify()

	ctx := id.WithIDs(context.Background(), id.IDs{RunID: "ctx-run-999"})
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-6",
		Arguments: map[string]any{
			"run_id":       "llm-bogus-id",
			"task_goal_ui": "Test",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	runID, ok := result.Metadata["run_id"].(string)
	if !ok || runID != "ctx-run-999" {
		t.Fatalf("expected run_id=ctx-run-999 from context, got %q", runID)
	}
}

func TestClarifyExecuteNeedsUserInput(t *testing.T) {
	tool := NewClarify()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-7",
		Arguments: map[string]any{
			"task_goal_ui":     "Need input",
			"needs_user_input": true,
			"question_to_user": "What color?",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	if needs, ok := result.Metadata["needs_user_input"].(bool); !ok || !needs {
		t.Fatalf("expected needs_user_input=true in metadata")
	}
	if !strings.Contains(result.Content, "What color?") {
		t.Fatalf("expected question in content, got %q", result.Content)
	}
}
