package types

import (
	"encoding/json"
	"testing"
)

func TestAgentTaskJSONEncodingIncludesIdentifiers(t *testing.T) {
	completedAt := "2024-05-01T12:34:56Z"
	task := AgentTask{
		RunID:       "task-123",
		SessionID:   "session-456",
		ParentRunID: "task-parent",
		Status:      "completed",
		CreatedAt:   "2024-05-01T12:00:00Z",
		CompletedAt: &completedAt,
		Error:       "",
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("failed to marshal agent task: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal agent task json: %v", err)
	}

	if decoded["run_id"] != task.RunID {
		t.Fatalf("expected run_id %s, got %v", task.RunID, decoded["run_id"])
	}
	if decoded["session_id"] != task.SessionID {
		t.Fatalf("expected session_id %s, got %v", task.SessionID, decoded["session_id"])
	}
	if decoded["parent_run_id"] != task.ParentRunID {
		t.Fatalf("expected parent_run_id %s, got %v", task.ParentRunID, decoded["parent_run_id"])
	}
	if decoded["completed_at"] != completedAt {
		t.Fatalf("expected completed_at %s, got %v", completedAt, decoded["completed_at"])
	}
}
