package app

import (
	"context"
	"errors"
	"testing"

	agent "alex/internal/agent/ports/agent"
	serverPorts "alex/internal/server/ports"
	id "alex/internal/utils/id"
)

func TestInMemoryTaskStore_Create(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	task, err := store.Create(ctx, "session-1", "Test task", "", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	if task.ID == "" {
		t.Error("Task ID should not be empty")
	}

	if task.SessionID != "session-1" {
		t.Errorf("Expected session ID 'session-1', got '%s'", task.SessionID)
	}

	if task.Description != "Test task" {
		t.Errorf("Expected description 'Test task', got '%s'", task.Description)
	}

	if task.Status != serverPorts.TaskStatusPending {
		t.Errorf("Expected status 'pending', got '%s'", task.Status)
	}

	if task.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestInMemoryTaskStore_CreateCapturesParentTaskID(t *testing.T) {
	ctx := id.WithParentRunID(context.Background(), "parent-123")
	store := NewInMemoryTaskStore()

	task, err := store.Create(ctx, "session-1", "Test task", "", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	if task.ParentTaskID != "parent-123" {
		t.Errorf("Expected parent task ID 'parent-123', got '%s'", task.ParentTaskID)
	}
}

func TestInMemoryTaskStore_Get(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	// Create a task
	created, err := store.Create(ctx, "session-1", "Test task", "", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Retrieve the task
	retrieved, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("Expected task ID '%s', got '%s'", created.ID, retrieved.ID)
	}

	// Try to get non-existent task
	_, err = store.Get(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent task")
	}
}

func TestInMemoryTaskStore_SetStatus(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	// Test status transitions
	tests := []struct {
		name           string
		status         serverPorts.TaskStatus
		checkStartedAt bool
		checkCompleted bool
	}{
		{"Running", serverPorts.TaskStatusRunning, true, false},
		{"Completed", serverPorts.TaskStatusCompleted, false, true},
		{"Failed", serverPorts.TaskStatusFailed, false, true},
		{"Cancelled", serverPorts.TaskStatusCancelled, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh task for each test
			freshTask, _ := store.Create(ctx, "session-1", "Test task", "", "")

			err := store.SetStatus(ctx, freshTask.ID, tt.status)
			if err != nil {
				t.Fatalf("Failed to set status: %v", err)
			}

			updated, _ := store.Get(ctx, freshTask.ID)
			if updated.Status != tt.status {
				t.Errorf("Expected status '%s', got '%s'", tt.status, updated.Status)
			}

			if tt.checkStartedAt && updated.StartedAt == nil {
				t.Error("StartedAt should be set for running status")
			}

			if tt.checkCompleted && updated.CompletedAt == nil {
				t.Error("CompletedAt should be set for terminal status")
			}
		})
	}
}

func TestInMemoryTaskStore_SetError(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	task, _ := store.Create(ctx, "session-1", "Test task", "", "")

	testErr := errors.New("Test error message")
	err := store.SetError(ctx, task.ID, testErr)
	if err != nil {
		t.Fatalf("Failed to set error: %v", err)
	}

	updated, _ := store.Get(ctx, task.ID)
	if updated.Error != testErr.Error() {
		t.Errorf("Expected error '%s', got '%s'", testErr.Error(), updated.Error)
	}

	if updated.Status != serverPorts.TaskStatusFailed {
		t.Errorf("Expected status 'failed', got '%s'", updated.Status)
	}

	if updated.CompletedAt == nil {
		t.Error("CompletedAt should be set when error occurs")
	}
}

func TestInMemoryTaskStore_SetResult(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	task, _ := store.Create(ctx, "session-1", "Test task", "", "")

	result := &agent.TaskResult{
		Answer:      "Task completed",
		Iterations:  5,
		TokensUsed:  1000,
		StopReason:  "final_answer",
		SessionID:   "session-1",
		ParentRunID: "parent-xyz",
	}

	err := store.SetResult(ctx, task.ID, result)
	if err != nil {
		t.Fatalf("Failed to set result: %v", err)
	}

	updated, _ := store.Get(ctx, task.ID)
	if updated.Status != serverPorts.TaskStatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", updated.Status)
	}

	if updated.Result == nil {
		t.Fatal("Result should be set")
	}

	if updated.Result.Answer != result.Answer {
		t.Errorf("Expected answer '%s', got '%s'", result.Answer, updated.Result.Answer)
	}

	if updated.TotalIterations != 5 {
		t.Errorf("Expected 5 iterations, got %d", updated.TotalIterations)
	}

	if updated.TokensUsed != 1000 {
		t.Errorf("Expected 1000 tokens, got %d", updated.TokensUsed)
	}

	if updated.ParentTaskID != "parent-xyz" {
		t.Errorf("Expected parent task ID 'parent-xyz', got '%s'", updated.ParentTaskID)
	}
}

// TestInMemoryTaskStore_SetResult_UpdatesSessionID tests the P0 blocker fix:
// When a task is created without a session ID, SetResult should update it
// with the session ID from the result.
func TestInMemoryTaskStore_SetResult_UpdatesSessionID(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	// Create task with empty session ID (simulating POST /api/tasks without session_id)
	task, _ := store.Create(ctx, "", "Test task", "", "")

	// Verify task was created with empty session ID
	if task.SessionID != "" {
		t.Fatalf("Expected empty session ID initially, got '%s'", task.SessionID)
	}

	// Execute task and get result with generated session ID
	result := &agent.TaskResult{
		Answer:     "Task completed",
		Iterations: 3,
		TokensUsed: 500,
		StopReason: "final_answer",
		SessionID:  "generated-session-abc123", // Generated by AgentCoordinator
	}

	err := store.SetResult(ctx, task.ID, result)
	if err != nil {
		t.Fatalf("Failed to set result: %v", err)
	}

	// Verify task now has the generated session ID
	updated, _ := store.Get(ctx, task.ID)
	if updated.SessionID != "generated-session-abc123" {
		t.Errorf("Expected session ID 'generated-session-abc123', got '%s'", updated.SessionID)
	}

	// Verify result also has the session ID
	if updated.Result == nil {
		t.Fatal("Result should be set")
	}
	if updated.Result.SessionID != "generated-session-abc123" {
		t.Errorf("Expected result session ID 'generated-session-abc123', got '%s'", updated.Result.SessionID)
	}
}

// TestInMemoryTaskStore_SetResult_PreservesExistingSessionID tests that
// when a task already has a session ID, SetResult doesn't overwrite it
// unless the result has a different session ID.
func TestInMemoryTaskStore_SetResult_PreservesExistingSessionID(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	// Create task with explicit session ID
	task, _ := store.Create(ctx, "existing-session-123", "Test task", "", "")

	// Execute task and get result with same session ID
	result := &agent.TaskResult{
		Answer:     "Task completed",
		Iterations: 2,
		TokensUsed: 300,
		StopReason: "final_answer",
		SessionID:  "existing-session-123",
	}

	err := store.SetResult(ctx, task.ID, result)
	if err != nil {
		t.Fatalf("Failed to set result: %v", err)
	}

	// Verify session ID is preserved
	updated, _ := store.Get(ctx, task.ID)
	if updated.SessionID != "existing-session-123" {
		t.Errorf("Expected session ID 'existing-session-123', got '%s'", updated.SessionID)
	}
}

func TestInMemoryTaskStore_UpdateProgress(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	task, _ := store.Create(ctx, "session-1", "Test task", "", "")

	err := store.UpdateProgress(ctx, task.ID, 3, 500)
	if err != nil {
		t.Fatalf("Failed to update progress: %v", err)
	}

	updated, _ := store.Get(ctx, task.ID)
	if updated.CurrentIteration != 3 {
		t.Errorf("Expected current iteration 3, got %d", updated.CurrentIteration)
	}

	if updated.TokensUsed != 500 {
		t.Errorf("Expected 500 tokens used, got %d", updated.TokensUsed)
	}
}

func TestInMemoryTaskStore_List(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	// Create multiple tasks
	for i := 0; i < 15; i++ {
		_, _ = store.Create(ctx, "session-1", "Test task", "", "")
	}

	// Test pagination
	tasks, total, err := store.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if total != 15 {
		t.Errorf("Expected total 15, got %d", total)
	}

	if len(tasks) != 10 {
		t.Errorf("Expected 10 tasks, got %d", len(tasks))
	}

	// Test second page
	tasks, total, err = store.List(ctx, 10, 10)
	if err != nil {
		t.Fatalf("Failed to list tasks (page 2): %v", err)
	}

	if len(tasks) != 5 {
		t.Errorf("Expected 5 tasks on page 2, got %d", len(tasks))
	}

	if total != 15 {
		t.Errorf("Expected total 15, got %d", total)
	}
}

func TestInMemoryTaskStore_ListBySession(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	// Create tasks for different sessions
	_, _ = store.Create(ctx, "session-1", "Task 1", "", "")
	_, _ = store.Create(ctx, "session-1", "Task 2", "", "")
	_, _ = store.Create(ctx, "session-2", "Task 3", "", "")
	_, _ = store.Create(ctx, "session-1", "Task 4", "", "")

	// Get tasks for session-1
	tasks, err := store.ListBySession(ctx, "session-1")
	if err != nil {
		t.Fatalf("Failed to list tasks by session: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks for session-1, got %d", len(tasks))
	}

	// Verify all tasks belong to session-1
	for _, task := range tasks {
		if task.SessionID != "session-1" {
			t.Errorf("Expected task to belong to session-1, got '%s'", task.SessionID)
		}
	}
}

func TestInMemoryTaskStore_Delete(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	task, _ := store.Create(ctx, "session-1", "Test task", "", "")

	// Delete the task
	err := store.Delete(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	// Verify task is deleted
	_, err = store.Get(ctx, task.ID)
	if err == nil {
		t.Error("Expected error when getting deleted task")
	}

	// Try to delete non-existent task
	err = store.Delete(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error when deleting non-existent task")
	}
}

func TestInMemoryTaskStore_SetTerminationReason(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	// Test setting each termination reason
	tests := []struct {
		name   string
		reason serverPorts.TerminationReason
	}{
		{"Completed", serverPorts.TerminationReasonCompleted},
		{"Cancelled", serverPorts.TerminationReasonCancelled},
		{"Timeout", serverPorts.TerminationReasonTimeout},
		{"Error", serverPorts.TerminationReasonError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			freshTask, _ := store.Create(ctx, "session-1", "Test task", "", "")

			err := store.SetTerminationReason(ctx, freshTask.ID, tt.reason)
			if err != nil {
				t.Fatalf("Failed to set termination reason: %v", err)
			}

			updated, _ := store.Get(ctx, freshTask.ID)
			if updated.TerminationReason != tt.reason {
				t.Errorf("Expected termination reason '%s', got '%s'", tt.reason, updated.TerminationReason)
			}
		})
	}
}

func TestInMemoryTaskStore_TerminationReasonAutoSet(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	// Test that SetStatus automatically sets termination reason
	tests := []struct {
		name            string
		status          serverPorts.TaskStatus
		expectedReason  serverPorts.TerminationReason
		shouldSetReason bool
	}{
		{"Running", serverPorts.TaskStatusRunning, serverPorts.TerminationReasonNone, false},
		{"Completed", serverPorts.TaskStatusCompleted, serverPorts.TerminationReasonCompleted, true},
		{"Failed", serverPorts.TaskStatusFailed, serverPorts.TerminationReasonError, true},
		{"Cancelled", serverPorts.TaskStatusCancelled, serverPorts.TerminationReasonCancelled, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, _ := store.Create(ctx, "session-1", "Test task", "", "")

			err := store.SetStatus(ctx, task.ID, tt.status)
			if err != nil {
				t.Fatalf("Failed to set status: %v", err)
			}

			updated, _ := store.Get(ctx, task.ID)
			if tt.shouldSetReason {
				if updated.TerminationReason != tt.expectedReason {
					t.Errorf("Expected termination reason '%s', got '%s'", tt.expectedReason, updated.TerminationReason)
				}
			} else {
				if updated.TerminationReason != serverPorts.TerminationReasonNone {
					t.Errorf("Expected no termination reason, got '%s'", updated.TerminationReason)
				}
			}
		})
	}
}

func TestInMemoryTaskStore_SetError_SetsTerminationReason(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	task, _ := store.Create(ctx, "session-1", "Test task", "", "")

	testErr := errors.New("Test error")
	err := store.SetError(ctx, task.ID, testErr)
	if err != nil {
		t.Fatalf("Failed to set error: %v", err)
	}

	updated, _ := store.Get(ctx, task.ID)
	if updated.TerminationReason != serverPorts.TerminationReasonError {
		t.Errorf("Expected termination reason 'error', got '%s'", updated.TerminationReason)
	}
}

func TestInMemoryTaskStore_SetResult_SetsTerminationReason(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()

	task, _ := store.Create(ctx, "session-1", "Test task", "", "")

	result := &agent.TaskResult{
		Answer:     "Task completed",
		Iterations: 5,
		TokensUsed: 1000,
		StopReason: "final_answer",
		SessionID:  "session-1",
	}

	err := store.SetResult(ctx, task.ID, result)
	if err != nil {
		t.Fatalf("Failed to set result: %v", err)
	}

	updated, _ := store.Get(ctx, task.ID)
	if updated.TerminationReason != serverPorts.TerminationReasonCompleted {
		t.Errorf("Expected termination reason 'completed', got '%s'", updated.TerminationReason)
	}
}
