package ui

import (
	"context"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	id "alex/internal/shared/utils/id"
)

func TestAskUserClarifySourcesRunIDFromContext(t *testing.T) {
	tool := NewAskUser()

	ctx := id.WithIDs(context.Background(), id.IDs{RunID: "ctx-run-789"})
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"action":       "clarify",
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

func TestAskUserClarifyAutoGeneratesTaskID(t *testing.T) {
	tool := NewAskUser()

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

func TestAskUserClarifyNeedsUserInput(t *testing.T) {
	tool := NewAskUser()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-7",
		Arguments: map[string]any{
			"action":           "clarify",
			"task_goal_ui":     "Need input",
			"needs_user_input": true,
			"question_to_user": "What color?",
			"options":          []any{"red", "blue"},
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
	options, ok := result.Metadata["options"].([]string)
	if !ok || len(options) != 2 || options[0] != "red" || options[1] != "blue" {
		t.Fatalf("expected options in metadata, got %#v", result.Metadata["options"])
	}
	if !strings.Contains(result.Content, "What color?") {
		t.Fatalf("expected question in content, got %q", result.Content)
	}
}

func TestAskUserRequestWithOptions(t *testing.T) {
	tool := NewAskUser()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"action":  "request",
			"message": "请选择部署环境",
			"title":   "需要你的选择",
			"options": []any{"dev", "staging", "prod"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Error != nil {
		t.Fatalf("expected success result, got %#v", result)
	}
	if needs, ok := result.Metadata["needs_user_input"].(bool); !ok || !needs {
		t.Fatalf("expected needs_user_input=true in metadata")
	}
	options, ok := result.Metadata["options"].([]string)
	if !ok || len(options) != 3 || options[0] != "dev" || options[1] != "staging" || options[2] != "prod" {
		t.Fatalf("expected options in metadata, got %#v", result.Metadata["options"])
	}
	if !strings.Contains(result.Content, "需要你的选择") {
		t.Fatalf("expected title in content, got %q", result.Content)
	}
}

func TestAskUserDefaultActionIsClarify(t *testing.T) {
	tool := NewAskUser()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-10",
		Arguments: map[string]any{
			"task_goal_ui": "Default action test",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if action, ok := result.Metadata["action"].(string); !ok || action != "clarify" {
		t.Fatalf("expected action=clarify, got %q", result.Metadata["action"])
	}
}

func TestAskUserRejectsInvalidAction(t *testing.T) {
	tool := NewAskUser()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-11",
		Arguments: map[string]any{
			"action":  "invalid",
			"message": "test",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Error == nil {
		t.Fatalf("expected tool error result, got %#v", result)
	}
	if !strings.Contains(result.Error.Error(), "action must be") {
		t.Fatalf("unexpected error: %v", result.Error)
	}
}
