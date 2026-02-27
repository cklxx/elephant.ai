package lark

import (
	"context"
	"strings"
	"sync"
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
			lastContent := updates[len(updates)-1].Content
			// New format uses friendly phrases and human-readable status.
			if !strings.Contains(lastContent, "正在后台处理中") {
				t.Fatalf("expected human-friendly progress header, got %q", lastContent)
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
	// Verify the initial message uses the humanized header.
	if !strings.Contains(calls[0].Content, "正在后台处理中") {
		t.Fatalf("expected humanized header for code agent, got %q", calls[0].Content)
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
	if !strings.Contains(updates[0].Content, "任务已完成") {
		t.Fatalf("expected completion header, got %q", updates[0].Content)
	}

	// Ensure no periodic updates fire after completion.
	time.Sleep(100 * time.Millisecond)
	updates = recorder.CallsByMethod("UpdateMessage")
	if len(updates) != 1 {
		t.Fatalf("expected no more updates after completion")
	}
}

func TestBackgroundProgressListener_CompletionUppercaseStatusIsNormalized(t *testing.T) {
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
		NodeID:    "bg-upper-1",
		Payload: map[string]any{
			"task_id":     "bg-upper-1",
			"description": "desc",
			"agent_type":  "codex",
		},
	})

	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskCompleted,
		NodeKind:  "background",
		NodeID:    "bg-upper-1",
		Payload: map[string]any{
			"task_id": "bg-upper-1",
			"status":  "COMPLETED",
			"answer":  "done",
		},
	})

	updates := recorder.CallsByMethod("UpdateMessage")
	if len(updates) != 1 {
		t.Fatalf("expected 1 update message, got %d", len(updates))
	}
	if !strings.Contains(updates[0].Content, "任务已完成") {
		t.Fatalf("expected completion header, got %q", updates[0].Content)
	}
	if !strings.Contains(updates[0].Content, "done") {
		t.Fatalf("expected answer in completion content, got %q", updates[0].Content)
	}
}

func TestBackgroundProgressListener_CompletionNonTerminalStatusWithErrorBecomesFailed(t *testing.T) {
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
		NodeID:    "bg-fail-1",
		Payload: map[string]any{
			"task_id":     "bg-fail-1",
			"description": "desc",
			"agent_type":  "codex",
		},
	})

	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskCompleted,
		NodeKind:  "background",
		NodeID:    "bg-fail-1",
		Payload: map[string]any{
			"task_id": "bg-fail-1",
			"status":  "running",
			"error":   "boom",
		},
	})

	updates := recorder.CallsByMethod("UpdateMessage")
	if len(updates) != 1 {
		t.Fatalf("expected 1 update message, got %d", len(updates))
	}
	if !strings.Contains(updates[0].Content, "任务出错了") {
		t.Fatalf("expected failed header, got %q", updates[0].Content)
	}
	if !strings.Contains(updates[0].Content, "status: failed") {
		t.Fatalf("expected normalized failed status in completion content, got %q", updates[0].Content)
	}
	if !strings.Contains(updates[0].Content, "boom") {
		t.Fatalf("expected error content, got %q", updates[0].Content)
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
	if !strings.Contains(lastUpdate.Content, "任务已完成") {
		t.Fatalf("expected completion header, got %q", lastUpdate.Content)
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

// --- Batch 1: Dedup ---

// TestDuplicateCompletionIsIdempotent sends completion via both the envelope
// path and the raw event path. Only the first should trigger a Lark update;
// the second should be silently dropped because getTask returns nil.
func TestDuplicateCompletionIsIdempotent(t *testing.T) {
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

	// Dispatch a task.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    "bg-dup",
		Payload: map[string]any{
			"task_id":     "bg-dup",
			"description": "dedup test",
			"agent_type":  "codex",
		},
	})

	// Path 1: Completion via envelope (normal chain).
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskCompleted,
		NodeKind:  "background",
		NodeID:    "bg-dup",
		Payload: map[string]any{
			"task_id": "bg-dup",
			"status":  "completed",
			"answer":  "first-path",
		},
	})

	// Path 2: Completion via raw event (direct bypass).
	ln.OnEvent(domain.NewBackgroundTaskCompletedEvent(
		domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		"bg-dup", "", "completed", "second-path", "", 0, 0, 100,
	))

	// Only one update should have fired (from the first path).
	updates := recorder.CallsByMethod("UpdateMessage")
	if len(updates) != 1 {
		t.Fatalf("expected exactly 1 completion update (dedup), got %d", len(updates))
	}
	if !strings.Contains(updates[0].Content, "first-path") {
		t.Fatalf("expected first-path content, got %q", updates[0].Content)
	}

	// Task should be removed from the listener.
	ln.mu.Lock()
	taskCount := len(ln.tasks)
	ln.mu.Unlock()
	if taskCount != 0 {
		t.Fatalf("expected 0 tasks after completion, got %d", taskCount)
	}
}

// --- Batch 3: Completion poller ---

// TestCompletionPollerCatchesMissedEvents simulates a scenario where the
// event chain is broken (no completion event arrives) but TaskStore has been
// updated by CompletionNotifier. The poller should detect this and deliver
// the completion message to Lark.
func TestCompletionPollerCatchesMissedEvents(t *testing.T) {
	recorder := NewRecordingMessenger()
	store := newInMemoryTaskStore()
	g := &Gateway{messenger: recorder, taskStore: store}

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
	// Use a very short poller interval for testing.
	ln.pollerInterval = 50 * time.Millisecond

	// Dispatch a task.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    "bg-poll",
		Payload: map[string]any{
			"task_id":     "bg-poll",
			"description": "poller test",
			"agent_type":  "codex",
		},
	})

	// Release the foreground — this starts the poller.
	ln.Release()

	// Simulate CompletionNotifier writing directly to TaskStore (Batch 2),
	// without any event reaching the listener.
	_ = store.UpdateStatus(context.Background(), "bg-poll", "completed",
		WithAnswerPreview("polled-answer"),
		WithMergeStatus(agent.MergeStatusMerged))

	// Wait for poller to detect the completed task.
	deadline := time.Now().Add(2 * time.Second)
	for {
		updates := recorder.CallsByMethod("UpdateMessage")
		for _, u := range updates {
			if strings.Contains(u.Content, "已完成") {
				// Success: poller delivered the completion.
				goto done
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for poller to deliver completion message")
		}
		time.Sleep(20 * time.Millisecond)
	}
done:
	updates := recorder.CallsByMethod("UpdateMessage")
	found := false
	for _, u := range updates {
		if strings.Contains(u.Content, "任务已完成") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected poller completion update with '任务已完成' header")
	}

	// Listener should auto-close after poller delivers completion.
	deadline = time.Now().Add(1 * time.Second)
	for {
		ln.mu.Lock()
		closed := ln.closed
		ln.mu.Unlock()
		if closed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for listener to auto-close after poller completion")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// --- Batch 4: Heartbeat filtering ---

// TestHeartbeatEventsFilteredFromProgress verifies that __heartbeat__ progress
// events are silently dropped by onExternalProgress and don't appear in
// the Lark progress display.
func TestHeartbeatEventsFilteredFromProgress(t *testing.T) {
	recorder := NewRecordingMessenger()
	g := &Gateway{messenger: recorder}

	ln := newBackgroundProgressListener(
		context.Background(),
		agent.NoopEventListener{},
		g,
		"chat-1",
		"om_parent",
		logging.NewComponentLogger("test"),
		50*time.Millisecond, // short interval to trigger flush
		10*time.Minute,
	)
	defer ln.Close()

	// Dispatch a task.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    "bg-hb",
		Payload: map[string]any{
			"task_id":     "bg-hb",
			"description": "heartbeat filter test",
			"agent_type":  "codex",
		},
	})

	// Send a heartbeat event.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventExternalAgentProgress,
		NodeKind:  "external_agent",
		NodeID:    "bg-hb",
		Payload: map[string]any{
			"task_id":      "bg-hb",
			"current_tool": "__heartbeat__",
			"tokens_used":  0,
		},
	})

	// Send a real progress event.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventExternalAgentProgress,
		NodeKind:  "external_agent",
		NodeID:    "bg-hb",
		Payload: map[string]any{
			"task_id":      "bg-hb",
			"current_tool": "Bash",
			"current_args": "ls -la",
			"tokens_used":  50,
		},
	})

	// Wait for a tick to fire a progress update.
	deadline := time.Now().Add(750 * time.Millisecond)
	for {
		updates := recorder.CallsByMethod("UpdateMessage")
		if len(updates) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for progress update")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify the update contains a human-friendly phrase (not raw tool name)
	// and does NOT contain the heartbeat marker.
	updates := recorder.CallsByMethod("UpdateMessage")
	lastUpdate := updates[len(updates)-1].Content
	if strings.Contains(lastUpdate, "__heartbeat__") {
		t.Fatalf("heartbeat should be filtered out from progress display, got %q", lastUpdate)
	}
	// Bash maps to execution-related phrases (在运算/在执行/在实验).
	if !strings.Contains(lastUpdate, "运算") && !strings.Contains(lastUpdate, "执行") && !strings.Contains(lastUpdate, "实验") {
		t.Fatalf("expected execution-related phrase for Bash tool, got %q", lastUpdate)
	}
}

// inMemoryTaskStore is a simple in-memory TaskStore for testing.
type inMemoryTaskStore struct {
	mu    sync.Mutex
	tasks map[string]TaskRecord
}

func newInMemoryTaskStore() *inMemoryTaskStore {
	return &inMemoryTaskStore{tasks: make(map[string]TaskRecord)}
}

func (s *inMemoryTaskStore) EnsureSchema(_ context.Context) error { return nil }

func (s *inMemoryTaskStore) SaveTask(_ context.Context, task TaskRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.TaskID] = task
	return nil
}

func (s *inMemoryTaskStore) UpdateStatus(_ context.Context, taskID, status string, opts ...TaskUpdateOption) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tasks[taskID]
	if !ok {
		rec = TaskRecord{TaskID: taskID}
	}
	rec.Status = status
	rec.UpdatedAt = time.Now()

	var o taskUpdateOptions
	for _, opt := range opts {
		opt(&o)
	}
	if o.answerPreview != nil {
		rec.AnswerPreview = *o.answerPreview
	}
	if o.errorText != nil {
		rec.Error = *o.errorText
	}
	if o.tokensUsed != nil {
		rec.TokensUsed = *o.tokensUsed
	}
	if o.mergeStatus != nil {
		rec.MergeStatus = *o.mergeStatus
	}
	s.tasks[taskID] = rec
	return nil
}

func (s *inMemoryTaskStore) GetTask(_ context.Context, taskID string) (TaskRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tasks[taskID]
	return rec, ok, nil
}

func (s *inMemoryTaskStore) ListByChat(_ context.Context, _ string, _ bool, _ int) ([]TaskRecord, error) {
	return nil, nil
}

func (s *inMemoryTaskStore) DeleteExpired(_ context.Context, _ time.Time) error { return nil }

func (s *inMemoryTaskStore) MarkStaleRunning(_ context.Context, _ string) error { return nil }

// --- Team Completion Summary Tests ---

func TestTeamCompletionSummary_SentWhenMultipleTasksComplete(t *testing.T) {
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
	for _, id := range []string{"bg-t1", "bg-t2"} {
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

	// Release foreground.
	ln.Release()

	// Complete both tasks.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskCompleted,
		NodeKind:  "background",
		NodeID:    "bg-t1",
		Payload: map[string]any{
			"task_id": "bg-t1",
			"status":  "completed",
			"answer":  "result one",
		},
	})
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskCompleted,
		NodeKind:  "background",
		NodeID:    "bg-t2",
		Payload: map[string]any{
			"task_id": "bg-t2",
			"status":  "failed",
			"error":   "build failed",
		},
	})

	// Wait for the async team summary goroutine.
	deadline := time.Now().Add(2 * time.Second)
	for {
		replies := recorder.CallsByMethod("ReplyMessage")
		found := false
		for _, r := range replies {
			if strings.Contains(r.Content, "全部后台任务已完成") {
				found = true
				break
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for team completion summary message")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Verify summary content.
	replies := recorder.CallsByMethod("ReplyMessage")
	var summaryContent string
	for _, r := range replies {
		if strings.Contains(r.Content, "全部后台任务已完成") {
			summaryContent = r.Content
			break
		}
	}
	if summaryContent == "" {
		t.Fatal("team summary message not found")
	}
	if !strings.Contains(summaryContent, "成功 1") {
		t.Fatalf("expected '成功 1' in summary, got %q", summaryContent)
	}
	if !strings.Contains(summaryContent, "失败 1") {
		t.Fatalf("expected '失败 1' in summary, got %q", summaryContent)
	}
	if !strings.Contains(summaryContent, "共 2 个任务") {
		t.Fatalf("expected '共 2 个任务' in summary, got %q", summaryContent)
	}
}

func TestTeamCompletionSummary_SkippedForSingleTask(t *testing.T) {
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

	// Dispatch only one task.
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    "bg-solo",
		Payload: map[string]any{
			"task_id":     "bg-solo",
			"description": "solo task",
			"agent_type":  "codex",
		},
	})

	ln.Release()

	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskCompleted,
		NodeKind:  "background",
		NodeID:    "bg-solo",
		Payload: map[string]any{
			"task_id": "bg-solo",
			"status":  "completed",
			"answer":  "done",
		},
	})

	// Wait briefly and verify no team summary was sent.
	time.Sleep(100 * time.Millisecond)

	replies := recorder.CallsByMethod("ReplyMessage")
	for _, r := range replies {
		if strings.Contains(r.Content, "全部后台任务已完成") {
			t.Fatalf("team summary should not be sent for single task, got %q", r.Content)
		}
	}
}

func TestTeamCompletionSummary_SkippedWhenDisabled(t *testing.T) {
	recorder := NewRecordingMessenger()
	disabled := false
	g := &Gateway{
		messenger: recorder,
		cfg:       Config{TeamCompletionSummaryEnabled: &disabled},
	}

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
	for _, id := range []string{"bg-d1", "bg-d2"} {
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

	ln.Release()

	for _, id := range []string{"bg-d1", "bg-d2"} {
		ln.OnEvent(&domain.WorkflowEventEnvelope{
			BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
			Version:   1,
			Event:     types.EventBackgroundTaskCompleted,
			NodeKind:  "background",
			NodeID:    id,
			Payload: map[string]any{
				"task_id": id,
				"status":  "completed",
				"answer":  "done",
			},
		})
	}

	time.Sleep(100 * time.Millisecond)

	replies := recorder.CallsByMethod("ReplyMessage")
	for _, r := range replies {
		if strings.Contains(r.Content, "全部后台任务已完成") {
			t.Fatal("team summary should not be sent when disabled")
		}
	}
}

func TestTeamCompletionSummary_FallbackFormat(t *testing.T) {
	ln := &backgroundProgressListener{
		g:      &Gateway{},
		logger: logging.NewComponentLogger("test"),
		now:    time.Now,
	}

	tasks := []completedTaskRecord{
		{taskID: "t1", description: "Research task A", status: taskStatusCompleted, answer: "Found 3 papers", duration: 2 * time.Minute},
		{taskID: "t2", description: "Coding task B", status: taskStatusFailed, errText: "Build failed", duration: 5 * time.Minute},
		{taskID: "t3", description: "Analysis task C", status: taskStatusCompleted, answer: "Analysis complete", duration: 3 * time.Minute},
	}

	summary := ln.buildTeamSummaryFallback(tasks)

	if !strings.Contains(summary, "全部后台任务已完成") {
		t.Fatalf("expected header in fallback, got %q", summary)
	}
	if !strings.Contains(summary, "共 3 个任务") {
		t.Fatalf("expected task count, got %q", summary)
	}
	if !strings.Contains(summary, "成功 2") {
		t.Fatalf("expected success count, got %q", summary)
	}
	if !strings.Contains(summary, "失败 1") {
		t.Fatalf("expected failure count, got %q", summary)
	}
	if !strings.Contains(summary, "5 分钟") {
		t.Fatalf("expected max duration (5 min), got %q", summary)
	}
	if !strings.Contains(summary, "✅ Research task A") {
		t.Fatalf("expected completed task marker, got %q", summary)
	}
	if !strings.Contains(summary, "❌ Coding task B") {
		t.Fatalf("expected failed task marker, got %q", summary)
	}
	if !strings.Contains(summary, "Build failed") {
		t.Fatalf("expected error text, got %q", summary)
	}
}

func TestTeamCompletionSummary_MaxDuration(t *testing.T) {
	tasks := []completedTaskRecord{
		{duration: 1 * time.Minute},
		{duration: 7 * time.Minute},
		{duration: 3 * time.Minute},
	}
	d := maxDuration(tasks)
	if d != 7*time.Minute {
		t.Fatalf("expected 7m, got %v", d)
	}
}

func TestTeamCompletionSummary_IsValidTeamSummary(t *testing.T) {
	if isValidTeamSummary("") {
		t.Fatal("empty should be invalid")
	}
	if isValidTeamSummary("  ") {
		t.Fatal("whitespace should be invalid")
	}
	if isValidTeamSummary("Empty response: nothing") {
		t.Fatal("empty response prefix should be invalid")
	}
	if !isValidTeamSummary("所有任务已完成") {
		t.Fatal("normal text should be valid")
	}
}
