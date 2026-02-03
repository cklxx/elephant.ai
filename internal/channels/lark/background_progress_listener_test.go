package lark

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
	"alex/internal/logging"
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
