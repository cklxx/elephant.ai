package app

import (
	"context"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/types"
)

// TestProgressTracking_Integration tests end-to-end progress tracking
// This verifies that events update task progress correctly
func TestProgressTracking_Integration(t *testing.T) {
	ctx := context.Background()

	// Setup components
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()
	broadcaster.SetTaskStore(taskStore)

	// Create a task
	task, err := taskStore.Create(ctx, "test-session-123", "Test task", "", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Register task-session mapping
	broadcaster.RegisterTaskSession("test-session-123", task.ID)
	defer broadcaster.UnregisterTaskSession("test-session-123")

	// Simulate iteration 1 start
	event1 := createIterationStartEvent("test-session-123", 1, 5)
	broadcaster.OnEvent(event1)

	// Verify progress updated
	updated, _ := taskStore.Get(ctx, task.ID)
	if updated.CurrentIteration != 1 {
		t.Errorf("Expected CurrentIteration=1 after IterationStartEvent, got %d", updated.CurrentIteration)
	}

	// Simulate iteration 1 complete with tokens
	event2 := createIterationCompleteEvent("test-session-123", 1, 150, 2)
	broadcaster.OnEvent(event2)

	// Verify tokens updated
	updated, _ = taskStore.Get(ctx, task.ID)
	if updated.TokensUsed != 150 {
		t.Errorf("Expected TokensUsed=150 after IterationCompleteEvent, got %d", updated.TokensUsed)
	}

	// Simulate iteration 2 start
	event3 := createIterationStartEvent("test-session-123", 2, 5)
	broadcaster.OnEvent(event3)

	// Verify iteration updated but tokens preserved
	updated, _ = taskStore.Get(ctx, task.ID)
	if updated.CurrentIteration != 2 {
		t.Errorf("Expected CurrentIteration=2, got %d", updated.CurrentIteration)
	}
	if updated.TokensUsed != 150 {
		t.Errorf("Expected TokensUsed=150 (preserved), got %d", updated.TokensUsed)
	}

	// Simulate iteration 2 complete with more tokens
	event4 := createIterationCompleteEvent("test-session-123", 2, 300, 1)
	broadcaster.OnEvent(event4)

	// Verify cumulative tokens
	updated, _ = taskStore.Get(ctx, task.ID)
	if updated.CurrentIteration != 2 {
		t.Errorf("Expected CurrentIteration=2, got %d", updated.CurrentIteration)
	}
	if updated.TokensUsed != 300 {
		t.Errorf("Expected TokensUsed=300 (cumulative), got %d", updated.TokensUsed)
	}

	// Simulate task complete
	event5 := createTaskCompleteEvent("test-session-123", 2, 300)
	broadcaster.OnEvent(event5)

	// Verify final progress
	updated, _ = taskStore.Get(ctx, task.ID)
	if updated.CurrentIteration != 2 {
		t.Errorf("Expected CurrentIteration=2 at completion, got %d", updated.CurrentIteration)
	}
	if updated.TokensUsed != 300 {
		t.Errorf("Expected TokensUsed=300 at completion, got %d", updated.TokensUsed)
	}
}

// TestProgressTracking_MultipleTasksIsolation verifies that progress tracking
// is isolated between different tasks/sessions
func TestProgressTracking_MultipleTasksIsolation(t *testing.T) {
	ctx := context.Background()

	// Setup
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()
	broadcaster.SetTaskStore(taskStore)

	// Create two tasks with different sessions
	task1, _ := taskStore.Create(ctx, "session-1", "Task 1", "", "")
	task2, _ := taskStore.Create(ctx, "session-2", "Task 2", "", "")

	// Register both mappings
	broadcaster.RegisterTaskSession("session-1", task1.ID)
	broadcaster.RegisterTaskSession("session-2", task2.ID)
	defer func() {
		broadcaster.UnregisterTaskSession("session-1")
		broadcaster.UnregisterTaskSession("session-2")
	}()

	// Send events for task 1
	event1 := createIterationCompleteEvent("session-1", 1, 100, 1)
	broadcaster.OnEvent(event1)

	// Send events for task 2
	event2 := createIterationCompleteEvent("session-2", 3, 500, 2)
	broadcaster.OnEvent(event2)

	// Verify task 1 has its own progress
	updated1, _ := taskStore.Get(ctx, task1.ID)
	if updated1.CurrentIteration != 1 {
		t.Errorf("Task 1: Expected CurrentIteration=1, got %d", updated1.CurrentIteration)
	}
	if updated1.TokensUsed != 100 {
		t.Errorf("Task 1: Expected TokensUsed=100, got %d", updated1.TokensUsed)
	}

	// Verify task 2 has its own progress
	updated2, _ := taskStore.Get(ctx, task2.ID)
	if updated2.CurrentIteration != 3 {
		t.Errorf("Task 2: Expected CurrentIteration=3, got %d", updated2.CurrentIteration)
	}
	if updated2.TokensUsed != 500 {
		t.Errorf("Task 2: Expected TokensUsed=500, got %d", updated2.TokensUsed)
	}
}

// TestProgressTracking_EventsWithoutMapping verifies that events for
// unregistered sessions don't cause errors
func TestProgressTracking_EventsWithoutMapping(t *testing.T) {
	ctx := context.Background()

	// Setup
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()
	broadcaster.SetTaskStore(taskStore)

	// Create task but don't register mapping
	task, _ := taskStore.Create(ctx, "session-orphan", "Orphan task", "", "")

	// Send event for unregistered session - should not panic
	event := createIterationCompleteEvent("session-orphan", 1, 100, 1)
	broadcaster.OnEvent(event)

	// Verify task progress unchanged (since no mapping exists)
	updated, _ := taskStore.Get(ctx, task.ID)
	if updated.CurrentIteration != 0 {
		t.Errorf("Expected CurrentIteration=0 (no mapping), got %d", updated.CurrentIteration)
	}
	if updated.TokensUsed != 0 {
		t.Errorf("Expected TokensUsed=0 (no mapping), got %d", updated.TokensUsed)
	}
}

// TestProgressTracking_WithoutTaskStore verifies that broadcaster works
// even when taskStore is not set (for backwards compatibility)
func TestProgressTracking_WithoutTaskStore(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	// Don't set task store

	// Send event - should not panic
	event := createIterationCompleteEvent("session-test", 1, 100, 1)
	broadcaster.OnEvent(event) // Should not panic
}

// Helper functions to create events for testing
func createIterationStartEvent(sessionID string, iteration, totalIters int) *domain.IterationStartEvent {
	evt := domain.NewTaskAnalysisEvent(
		types.LevelCore,
		sessionID,
		"progress-task",
		"",
		&ports.TaskAnalysis{ActionName: "test", Goal: "test"},
		time.Now(),
	)
	return &domain.IterationStartEvent{
		BaseEvent:  evt.BaseEvent,
		Iteration:  iteration,
		TotalIters: totalIters,
	}
}

func createIterationCompleteEvent(sessionID string, iteration, tokensUsed, toolsRun int) *domain.IterationCompleteEvent {
	evt := domain.NewTaskAnalysisEvent(
		types.LevelCore,
		sessionID,
		"progress-task",
		"",
		&ports.TaskAnalysis{ActionName: "test", Goal: "test"},
		time.Now(),
	)
	return &domain.IterationCompleteEvent{
		BaseEvent:  evt.BaseEvent,
		Iteration:  iteration,
		TokensUsed: tokensUsed,
		ToolsRun:   toolsRun,
	}
}

func createTaskCompleteEvent(sessionID string, totalIterations, totalTokens int) *domain.TaskCompleteEvent {
	evt := domain.NewTaskAnalysisEvent(
		types.LevelCore,
		sessionID,
		"progress-task",
		"",
		&ports.TaskAnalysis{ActionName: "test", Goal: "test"},
		time.Now(),
	)
	return &domain.TaskCompleteEvent{
		BaseEvent:       evt.BaseEvent,
		FinalAnswer:     "Task completed successfully",
		TotalIterations: totalIterations,
		TotalTokens:     totalTokens,
		StopReason:      "final_answer",
		Duration:        5 * time.Second,
	}
}
