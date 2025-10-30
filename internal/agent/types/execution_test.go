package types

import (
	"encoding/json"
	"testing"
)

func TestAgentTaskJSONEncodingIncludesIdentifiers(t *testing.T) {
	completedAt := "2024-05-01T12:34:56Z"
	task := AgentTask{
		TaskID:       "task-123",
		SessionID:    "session-456",
		ParentTaskID: "task-parent",
		Status:       "completed",
		CreatedAt:    "2024-05-01T12:00:00Z",
		CompletedAt:  &completedAt,
		Error:        "",
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("failed to marshal agent task: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal agent task json: %v", err)
	}

	if decoded["task_id"] != task.TaskID {
		t.Fatalf("expected task_id %s, got %v", task.TaskID, decoded["task_id"])
	}
	if decoded["session_id"] != task.SessionID {
		t.Fatalf("expected session_id %s, got %v", task.SessionID, decoded["session_id"])
	}
	if decoded["parent_task_id"] != task.ParentTaskID {
		t.Fatalf("expected parent_task_id %s, got %v", task.ParentTaskID, decoded["parent_task_id"])
	}
	if decoded["completed_at"] != completedAt {
		t.Fatalf("expected completed_at %s, got %v", completedAt, decoded["completed_at"])
	}
}

func TestExecutionEventOmitsEmptyParentTaskID(t *testing.T) {
	event := ExecutionEvent{
		EventType: "tool_start",
		Timestamp: "2024-05-01T12:30:00Z",
		SessionID: "session-123",
		TaskID:    "task-456",
		Payload:   map[string]any{"tool": "subagent"},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal execution event: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal execution event json: %v", err)
	}

	if _, ok := decoded["parent_task_id"]; ok {
		t.Fatalf("expected parent_task_id to be omitted, got %v", decoded["parent_task_id"])
	}
	if decoded["session_id"] != event.SessionID {
		t.Fatalf("expected session_id %s, got %v", event.SessionID, decoded["session_id"])
	}
	if decoded["task_id"] != event.TaskID {
		t.Fatalf("expected task_id %s, got %v", event.TaskID, decoded["task_id"])
	}
}
