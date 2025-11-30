package app

import (
	"fmt"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
)

func TestTranslateSubflowEventsIncludeAggregatedStats(t *testing.T) {
	translator := &workflowEventTranslator{
		subflowTracker: newSubflowStatsTracker(),
	}

	parentTaskID := "parent-task"

	progressEnv := translator.translateSubtaskEvent(newSubtaskWrapper(parentTaskID, 0, 2, &domain.WorkflowToolCompletedEvent{
		BaseEvent: domain.NewBaseEvent(ports.LevelSubagent, "sess-1", "task-1", parentTaskID, time.Now()),
		CallID:    "call-1",
		ToolName:  "bash",
	}))
	if len(progressEnv) != 1 {
		t.Fatalf("expected 1 progress envelope, got %d", len(progressEnv))
	}
	progress := progressEnv[0]
	if progress.Event != "workflow.subflow.progress" {
		t.Fatalf("expected progress event, got %s", progress.Event)
	}
	if got := progress.Payload["tool_calls"]; got != 1 {
		t.Fatalf("expected tool_calls=1, got %v", got)
	}
	if got := progress.Payload["completed"]; got != 0 {
		t.Fatalf("expected completed=0 on first progress event, got %v", got)
	}
	if got := progress.Payload["total"]; got != 2 {
		t.Fatalf("expected total=2, got %v", got)
	}

	completedEnv := translator.translateSubtaskEvent(newSubtaskWrapper(parentTaskID, 0, 2, &domain.WorkflowResultFinalEvent{
		BaseEvent:       domain.NewBaseEvent(ports.LevelSubagent, "sess-1", "task-1", parentTaskID, time.Now()),
		TotalTokens:     400,
		TotalIterations: 2,
	}))
	if len(completedEnv) != 1 {
		t.Fatalf("expected 1 completion envelope, got %d", len(completedEnv))
	}
	completed := completedEnv[0]
	if completed.Event != "workflow.subflow.completed" {
		t.Fatalf("expected completion event, got %s", completed.Event)
	}
	if got := completed.Payload["tokens"]; got != 400 {
		t.Fatalf("expected tokens=400, got %v", got)
	}
	if got := completed.Payload["tool_calls"]; got != 1 {
		t.Fatalf("expected tool_calls to carry forward=1, got %v", got)
	}
	if got := completed.Payload["success"]; got != 1 {
		t.Fatalf("expected success=1, got %v", got)
	}
	if got := completed.Payload["failed"]; got != 0 {
		t.Fatalf("expected failed=0, got %v", got)
	}
	if got := completed.Payload["completed"]; got != 1 {
		t.Fatalf("expected completed=1, got %v", got)
	}
	if got := completed.Payload["total"]; got != 2 {
		t.Fatalf("expected total=2, got %v", got)
	}

	cancelledEnv := translator.translateSubtaskEvent(newSubtaskWrapper(parentTaskID, 1, 2, &domain.WorkflowResultCancelledEvent{
		BaseEvent: domain.NewBaseEvent(ports.LevelSubagent, "sess-1", "task-2", parentTaskID, time.Now()),
		Reason:    "user_stop",
	}))
	if len(cancelledEnv) != 1 {
		t.Fatalf("expected 1 cancellation envelope, got %d", len(cancelledEnv))
	}
	cancelled := cancelledEnv[0]
	if cancelled.Event != "workflow.subflow.completed" {
		t.Fatalf("expected completion event for cancelled subtask, got %s", cancelled.Event)
	}
	if got := cancelled.Payload["success"]; got != 1 {
		t.Fatalf("expected success to remain 1, got %v", got)
	}
	if got := cancelled.Payload["failed"]; got != 1 {
		t.Fatalf("expected failed=1 after cancellation, got %v", got)
	}
	if got := cancelled.Payload["completed"]; got != 2 {
		t.Fatalf("expected completed=2 after second subtask, got %v", got)
	}
	if got := cancelled.Payload["total"]; got != 2 {
		t.Fatalf("expected total=2, got %v", got)
	}
	if got := cancelled.Payload["tokens"]; got != 400 {
		t.Fatalf("expected tokens to stay aggregated at 400, got %v", got)
	}
	if got := cancelled.Payload["tool_calls"]; got != 1 {
		t.Fatalf("expected tool_calls to remain 1, got %v", got)
	}
}

func newSubtaskWrapper(parentTaskID string, index, total int, wrapped ports.AgentEvent) ports.SubtaskWrapper {
	base := domain.NewBaseEvent(ports.LevelSubagent, "sess-1", fmt.Sprintf("task-%d", index), parentTaskID, time.Now())

	return &fakeSubtaskEvent{
		base: &base,
		meta: ports.SubtaskMetadata{
			Index:       index,
			Total:       total,
			Preview:     fmt.Sprintf("task-%d", index),
			MaxParallel: total,
		},
		wrapped: wrapped,
	}
}

type fakeSubtaskEvent struct {
	base    *domain.BaseEvent
	meta    ports.SubtaskMetadata
	wrapped ports.AgentEvent
}

func (e *fakeSubtaskEvent) EventType() string {
	return "subtask"
}

func (e *fakeSubtaskEvent) Timestamp() time.Time {
	if e.base == nil {
		return time.Time{}
	}
	return e.base.Timestamp()
}

func (e *fakeSubtaskEvent) GetAgentLevel() ports.AgentLevel {
	if e.base == nil {
		return ""
	}
	return e.base.GetAgentLevel()
}

func (e *fakeSubtaskEvent) GetSessionID() string {
	if e.base == nil {
		return ""
	}
	return e.base.GetSessionID()
}

func (e *fakeSubtaskEvent) GetTaskID() string {
	if e.base == nil {
		return ""
	}
	return e.base.GetTaskID()
}

func (e *fakeSubtaskEvent) GetParentTaskID() string {
	if e.base == nil {
		return ""
	}
	return e.base.GetParentTaskID()
}

func (e *fakeSubtaskEvent) SubtaskDetails() ports.SubtaskMetadata {
	return e.meta
}

func (e *fakeSubtaskEvent) WrappedEvent() ports.AgentEvent {
	return e.wrapped
}
