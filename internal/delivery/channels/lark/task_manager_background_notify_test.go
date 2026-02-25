package lark

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/domain/agent/types"
)

func TestRunTask_BackgroundCompletionNotifiedAfterForegroundReturn(t *testing.T) {
	rec := NewRecordingMessenger()
	done := make(chan struct{})
	exec := &delayedBackgroundCompletionExecutor{
		taskID:   "bg-late-1",
		done:     done,
		delay:    80 * time.Millisecond,
		complete: "coding agent finished",
	}
	gw := newTestGatewayWithMessenger(exec, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	if err := gw.InjectMessage(context.Background(), "oc_bg_notify", "p2p", "ou_user", "om_trigger_1", "请帮我后台处理这个 coding 任务"); err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}

	// Foreground task returns quickly; completion is emitted later.
	gw.WaitForTasks()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for delayed completion event")
	}

	sendCalls := rec.CallsByMethod("SendMessage")
	if len(sendCalls) == 0 {
		t.Fatal("expected at least one send message call")
	}
	foundProgressSend := false
	for _, call := range sendCalls {
		if call.ChatID == "oc_bg_notify" && strings.Contains(call.Content, "正在后台处理中") {
			foundProgressSend = true
			break
		}
	}
	if !foundProgressSend {
		t.Fatalf("expected initial progress message in trigger chat, sends=%v", sendCalls)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		updates := rec.CallsByMethod("UpdateMessage")
		for _, call := range updates {
			if strings.Contains(call.Content, "task_id: bg-late-1") && strings.Contains(call.Content, "status: completed") {
				if !strings.Contains(call.Content, "coding agent finished") {
					t.Fatalf("expected completion answer in update, got %q", call.Content)
				}
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected completion update after foreground return, updates=%v", updates)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type delayedBackgroundCompletionExecutor struct {
	taskID   string
	delay    time.Duration
	complete string
	done     chan struct{}
}

func (e *delayedBackgroundCompletionExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (e *delayedBackgroundCompletionExecutor) ExecuteTask(_ context.Context, _ string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	taskID := strings.TrimSpace(e.taskID)
	if taskID == "" {
		taskID = "bg-late"
	}
	delay := e.delay
	if delay <= 0 {
		delay = 50 * time.Millisecond
	}
	answer := strings.TrimSpace(e.complete)
	if answer == "" {
		answer = "done"
	}

	listener.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, sessionID, "run-bg", "", time.Now()),
		Version:   1,
		Event:     types.EventBackgroundTaskDispatched,
		NodeKind:  "background",
		NodeID:    taskID,
		Payload: map[string]any{
			"task_id":     taskID,
			"description": "background coding task",
			"agent_type":  "codex",
		},
	})

	done := e.done
	go func() {
		time.Sleep(delay)
		listener.OnEvent(&domain.WorkflowEventEnvelope{
			BaseEvent: domain.NewBaseEvent(agent.LevelCore, sessionID, "run-bg", "", time.Now()),
			Version:   1,
			Event:     types.EventBackgroundTaskCompleted,
			NodeKind:  "background",
			NodeID:    taskID,
			Payload: map[string]any{
				"task_id": taskID,
				"status":  "completed",
				"answer":  answer,
			},
		})
		if done != nil {
			close(done)
		}
	}()

	return &agent.TaskResult{Answer: "后台任务已派发"}, nil
}
