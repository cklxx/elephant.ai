package http

import (
	"testing"
	"time"

	serverPorts "alex/internal/delivery/server/ports"
)

func TestToTaskStatusResponse(t *testing.T) {
	createdAt := time.Date(2026, 2, 1, 10, 20, 30, 0, time.UTC)
	completedAt := createdAt.Add(2 * time.Minute)
	task := &serverPorts.Task{
		ID:           "run-1",
		SessionID:    "sess-1",
		ParentTaskID: "run-0",
		Status:       serverPorts.TaskStatusCompleted,
		CreatedAt:    createdAt,
		CompletedAt:  &completedAt,
		Error:        "boom",
	}

	response := toTaskStatusResponse(task)

	if response.RunID != "run-1" {
		t.Fatalf("expected run_id run-1, got %q", response.RunID)
	}
	if response.ParentRunID != "run-0" {
		t.Fatalf("expected parent_run_id run-0, got %q", response.ParentRunID)
	}
	if response.Status != string(serverPorts.TaskStatusCompleted) {
		t.Fatalf("expected status %q, got %q", serverPorts.TaskStatusCompleted, response.Status)
	}
	if response.CreatedAt != createdAt.Format(time.RFC3339) {
		t.Fatalf("unexpected created_at: %q", response.CreatedAt)
	}
	if response.CompletedAt == nil || *response.CompletedAt != completedAt.Format(time.RFC3339) {
		t.Fatalf("unexpected completed_at: %v", response.CompletedAt)
	}
	if response.Error != "boom" {
		t.Fatalf("expected error boom, got %q", response.Error)
	}
}

func TestToTaskStatusResponsesPreservesOrdering(t *testing.T) {
	createdAt := time.Date(2026, 2, 1, 10, 20, 30, 0, time.UTC)
	tasks := []*serverPorts.Task{
		{
			ID:        "run-1",
			SessionID: "sess-1",
			Status:    serverPorts.TaskStatusPending,
			CreatedAt: createdAt,
		},
		{
			ID:        "run-2",
			SessionID: "sess-2",
			Status:    serverPorts.TaskStatusRunning,
			CreatedAt: createdAt.Add(time.Minute),
		},
	}

	responses := toTaskStatusResponses(tasks)

	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
	if responses[0].RunID != "run-1" || responses[1].RunID != "run-2" {
		t.Fatalf("unexpected response ordering: %+v", responses)
	}
}
