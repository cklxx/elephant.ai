package lark

import (
	"context"
	"testing"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
	"alex/internal/channels"
)

func TestFinalAnswerReviewReactionListener_ReactsOnce(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(&stubExecutor{}, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	listener := newFinalAnswerReviewReactionListener(context.Background(), agent.NoopEventListener{}, gw, "om_msg_1", "")
	listener.OnEvent(&domain.WorkflowEventEnvelope{
		Event:  types.EventToolStarted,
		NodeID: "final_answer_review:1",
		Payload: map[string]any{
			"tool_name": "final_answer_review",
		},
	})
	listener.OnEvent(&domain.WorkflowEventEnvelope{
		Event:  types.EventToolStarted,
		NodeID: "final_answer_review:1",
		Payload: map[string]any{
			"tool_name": "final_answer_review",
		},
	})

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(rec.CallsByMethod("AddReaction")) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	reactions := rec.CallsByMethod("AddReaction")
	if len(reactions) != 1 {
		t.Fatalf("expected 1 AddReaction call, got %d", len(reactions))
	}
	if reactions[0].MsgID != "om_msg_1" {
		t.Fatalf("expected reaction on msg_id=om_msg_1, got %q", reactions[0].MsgID)
	}
	if reactions[0].Emoji != "GLANCE" {
		t.Fatalf("expected GLANCE reaction, got %q", reactions[0].Emoji)
	}
}

func TestFinalAnswerReviewReactionListener_IgnoresOtherTools(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(&stubExecutor{}, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	listener := newFinalAnswerReviewReactionListener(context.Background(), nil, gw, "om_msg_1", "GLANCE")
	listener.OnEvent(&domain.WorkflowEventEnvelope{
		Event:  types.EventToolStarted,
		NodeID: "call_001",
		Payload: map[string]any{
			"tool_name": "todo_read",
		},
	})

	time.Sleep(50 * time.Millisecond)
	reactions := rec.CallsByMethod("AddReaction")
	if len(reactions) != 0 {
		t.Fatalf("expected no reactions, got %d", len(reactions))
	}
}

