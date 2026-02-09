package lark

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/logging"
)

func TestBackgroundProgressListener_DispatchAndTickUpdate(t *testing.T) {
	recorder := NewRecordingMessenger()
	g := &Gateway{messenger: recorder}

	ln := newBackgroundProgressListener(
		context.Background(),
		agent.NoopEventListener{},
		g,
		"chat-1",
		"om_parent",
		logging.NewComponentLogger("test"),
		50*time.Millisecond,
		10*time.Minute,
	)
	defer ln.Close()

	dispatch := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id":     "bg-1",
			"description": "desc",
			"agent_type":  "codex",
		},
	}
	ln.OnEvent(dispatch)

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) != 1 {
		t.Fatalf("expected 1 reply message, got %d", len(calls))
	}
	if calls[0].ReplyTo != "om_parent" {
		t.Fatalf("unexpected reply target: %q", calls[0].ReplyTo)
	}

	progress := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventExternalAgentProgress,
		NodeKind:  "external_agent",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id":       "bg-1",
			"agent_type":    "codex",
			"tokens_used":   123,
			"current_tool":  "assistant_output",
			"current_args":  "working...",
			"files_touched": []string{"a.txt"},
			"last_activity": time.Now(),
		},
	}
	ln.OnEvent(progress)

	deadline := time.Now().Add(750 * time.Millisecond)
	for {
		updates := recorder.CallsByMethod("UpdateMessage")
		if len(updates) > 0 {
			if !strings.Contains(updates[len(updates)-1].Content, "tokens") {
				t.Fatalf("expected update content to mention tokens, got %q", updates[len(updates)-1].Content)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for update")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBackgroundProgressListener_CodeAgentUsesThreeMinuteInterval(t *testing.T) {
	recorder := NewRecordingMessenger()
	g := &Gateway{messenger: recorder}

	ln := newBackgroundProgressListener(
		context.Background(),
		agent.NoopEventListener{},
		g,
		"chat-1",
		"om_parent",
		logging.NewComponentLogger("test"),
		30*time.Minute,
		10*time.Minute,
	)
	defer ln.Close()

	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id":     "bg-1",
			"description": "desc",
			"agent_type":  "codex",
		},
	})

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) != 1 {
		t.Fatalf("expected 1 reply message, got %d", len(calls))
	}
	if !strings.Contains(calls[0].Content, "每3分钟") {
		t.Fatalf("expected 3-minute interval for code agent, got %q", calls[0].Content)
	}
}

func TestBackgroundProgressListener_CompletionUpdatesImmediatelyAndStops(t *testing.T) {
	recorder := NewRecordingMessenger()
	g := &Gateway{messenger: recorder}

	ln := newBackgroundProgressListener(
		context.Background(),
		agent.NoopEventListener{},
		g,
		"chat-1",
		"om_parent",
		logging.NewComponentLogger("test"),
		1*time.Hour,
		10*time.Minute,
	)
	defer ln.Close()

	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id":     "bg-1",
			"description": "desc",
			"agent_type":  "codex",
		},
	})

	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskCompleted,
		NodeKind:  "background",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id": "bg-1",
			"status":  "completed",
			"answer":  "done",
		},
	})

	updates := recorder.CallsByMethod("UpdateMessage")
	if len(updates) != 1 {
		t.Fatalf("expected 1 update message, got %d", len(updates))
	}
	if !strings.Contains(updates[0].Content, "done") {
		t.Fatalf("expected completion content, got %q", updates[0].Content)
	}

	// Ensure no periodic updates fire after completion.
	time.Sleep(100 * time.Millisecond)
	updates = recorder.CallsByMethod("UpdateMessage")
	if len(updates) != 1 {
		t.Fatalf("expected no more updates after completion")
	}
}

func TestBgProgressListener_ReleaseNoTasksClosesImmediately(t *testing.T) {
	recorder := NewRecordingMessenger()
	g := &Gateway{messenger: recorder}

	ln := newBackgroundProgressListener(
		context.Background(),
		agent.NoopEventListener{},
		g,
		"chat-1",
		"om_parent",
		logging.NewComponentLogger("test"),
		1*time.Hour,
		10*time.Minute,
	)

	// Release with no tasks should close immediately.
	ln.Release()

	ln.mu.Lock()
	closed := ln.closed
	released := ln.released
	ln.mu.Unlock()

	if !closed {
		t.Fatal("expected listener to be closed after Release with no tasks")
	}
	if !released {
		t.Fatal("expected released flag to be set")
	}

	// Dispatching after release should be a no-op.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    "bg-late",
		Payload: map[string]any{
			"task_id":     "bg-late",
			"description": "should not register",
			"agent_type":  "codex",
		},
	})

	ln.mu.Lock()
	taskCount := len(ln.tasks)
	ln.mu.Unlock()
	if taskCount != 0 {
		t.Fatalf("expected 0 tasks after release+close, got %d", taskCount)
	}
}

func TestBgProgressListener_ReleaseAfterDispatchStaysAlive(t *testing.T) {
	recorder := NewRecordingMessenger()
	g := &Gateway{messenger: recorder}

	ln := newBackgroundProgressListener(
		context.Background(),
		agent.NoopEventListener{},
		g,
		"chat-1",
		"om_parent",
		logging.NewComponentLogger("test"),
		50*time.Millisecond,
		10*time.Minute,
	)

	// Dispatch a task first.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id":     "bg-1",
			"description": "test task",
			"agent_type":  "codex",
		},
	})

	// Release — should NOT close because a task is active.
	ln.Release()

	ln.mu.Lock()
	closed := ln.closed
	released := ln.released
	ln.mu.Unlock()

	if closed {
		t.Fatal("expected listener to stay open after Release with active tasks")
	}
	if !released {
		t.Fatal("expected released flag to be set")
	}

	// Progress events should still be processed.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventExternalAgentProgress,
		NodeKind:  "external_agent",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id":      "bg-1",
			"tokens_used":  500,
			"current_tool": "bash",
			"current_args": "running tests",
		},
	})

	// Wait for a tick to fire.
	deadline := time.Now().Add(750 * time.Millisecond)
	for {
		updates := recorder.CallsByMethod("UpdateMessage")
		if len(updates) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for progress update after Release")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Clean up.
	ln.Close()
}

func TestBgProgressListener_AutoCloseOnLastTaskComplete(t *testing.T) {
	recorder := NewRecordingMessenger()
	g := &Gateway{messenger: recorder}

	ln := newBackgroundProgressListener(
		context.Background(),
		agent.NoopEventListener{},
		g,
		"chat-1",
		"om_parent",
		logging.NewComponentLogger("test"),
		1*time.Hour,
		10*time.Minute,
	)

	// Dispatch two tasks.
	for _, id := range []string{"bg-1", "bg-2"} {
		ln.OnEvent(&domain.WorkflowEventEnvelope{
			BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
			Version:   1,
			Event:     types.EventBackgroundTaskDispatched,
			NodeKind:  "background",
			NodeID:    id,
			Payload: map[string]any{
				"task_id":     id,
				"description": "task " + id,
				"agent_type":  "codex",
			},
		})
	}

	// Release — 2 tasks remain, should not close.
	ln.Release()

	ln.mu.Lock()
	closed := ln.closed
	ln.mu.Unlock()
	if closed {
		t.Fatal("should not be closed with 2 active tasks")
	}

	// Complete first task.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskCompleted,
		NodeKind:  "background",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id": "bg-1",
			"status":  "completed",
			"answer":  "done 1",
		},
	})

	ln.mu.Lock()
	closed = ln.closed
	remaining := len(ln.tasks)
	ln.mu.Unlock()
	if closed {
		t.Fatal("should not be closed with 1 active task remaining")
	}
	if remaining != 1 {
		t.Fatalf("expected 1 remaining task, got %d", remaining)
	}

	// Complete second task — should auto-close.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskCompleted,
		NodeKind:  "background",
		NodeID:    "bg-2",
		Payload: map[string]any{
			"task_id": "bg-2",
			"status":  "completed",
			"answer":  "done 2",
		},
	})

	ln.mu.Lock()
	closed = ln.closed
	remaining = len(ln.tasks)
	ln.mu.Unlock()
	if !closed {
		t.Fatal("expected listener to auto-close after last task completed")
	}
	if remaining != 0 {
		t.Fatalf("expected 0 remaining tasks, got %d", remaining)
	}
}

func TestBgProgressListener_CancelledCtxDoesNotBreakAPICalls(t *testing.T) {
	recorder := NewRecordingMessenger()
	g := &Gateway{messenger: recorder}

	// Create listener with a cancellable context (simulating foreground task context).
	parentCtx, cancel := context.WithCancel(context.Background())

	ln := newBackgroundProgressListener(
		parentCtx,
		agent.NoopEventListener{},
		g,
		"chat-1",
		"om_parent",
		logging.NewComponentLogger("test"),
		1*time.Hour,
		10*time.Minute,
	)
	defer ln.Close()

	// Dispatch a task.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id":     "bg-1",
			"description": "test task",
			"agent_type":  "codex",
		},
	})

	// Cancel the parent context (simulates foreground task returning).
	cancel()

	// The listener's internal ctx should be detached via WithoutCancel,
	// so completing the task should still succeed in sending Lark messages.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskCompleted,
		NodeKind:  "background",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id": "bg-1",
			"status":  "completed",
			"answer":  "finished after cancel",
		},
	})

	// Verify the completion update was sent despite parent context cancellation.
	updates := recorder.CallsByMethod("UpdateMessage")
	if len(updates) == 0 {
		t.Fatal("expected at least 1 update message after parent context cancelled")
	}
	lastUpdate := updates[len(updates)-1]
	if !strings.Contains(lastUpdate.Content, "finished after cancel") {
		t.Fatalf("expected completion content, got %q", lastUpdate.Content)
	}
}

func TestBackgroundProgressListener_InputRequestUpdatesImmediately(t *testing.T) {
	recorder := NewRecordingMessenger()
	g := &Gateway{messenger: recorder}

	ln := newBackgroundProgressListener(
		context.Background(),
		agent.NoopEventListener{},
		g,
		"chat-1",
		"om_parent",
		logging.NewComponentLogger("test"),
		1*time.Hour,
		10*time.Minute,
	)
	defer ln.Close()

	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id":     "bg-1",
			"description": "desc",
			"agent_type":  "claude_code",
		},
	})

	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventExternalInputRequested,
		NodeKind:  "external_input",
		NodeID:    "bg-1",
		Payload: map[string]any{
			"task_id":    "bg-1",
			"request_id": "req-1",
			"summary":    "need approval",
		},
	})

	updates := recorder.CallsByMethod("UpdateMessage")
	if len(updates) != 1 {
		t.Fatalf("expected 1 update message, got %d", len(updates))
	}
	if !strings.Contains(updates[0].Content, "need approval") {
		t.Fatalf("expected input request content, got %q", updates[0].Content)
	}
}
