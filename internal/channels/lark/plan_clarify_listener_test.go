package lark

import (
	"context"
	"testing"

	"alex/internal/agent/domain"
)

func TestPlanClarifyListenerPlanMessage(t *testing.T) {
	recorder := NewRecordingMessenger()
	gw := &Gateway{messenger: recorder}
	listener := newPlanClarifyListener(context.Background(), nil, gw, "oc_chat", "om_reply", nil)

	event := &domain.WorkflowToolCompletedEvent{
		CallID:   "call-1",
		ToolName: "plan",
		Result:   "fallback goal",
		Metadata: map[string]any{
			"overall_goal_ui": "Ship feature",
		},
	}
	listener.OnEvent(event)

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) == 0 {
		t.Fatal("expected reply message")
	}
	if got := extractTextContent(calls[0].Content); got != "Ship feature" {
		t.Fatalf("expected goal message, got %q", got)
	}
}

func TestPlanClarifyListenerClarifyQuestionMarksSent(t *testing.T) {
	recorder := NewRecordingMessenger()
	tracker := &awaitQuestionTracker{}
	gw := &Gateway{messenger: recorder}
	listener := newPlanClarifyListener(context.Background(), nil, gw, "oc_chat", "om_reply", tracker)

	event := &domain.WorkflowToolCompletedEvent{
		CallID:   "call-2",
		ToolName: "clarify",
		Result:   "task\nWhich env?",
		Metadata: map[string]any{
			"needs_user_input": true,
			"question_to_user": "Which env?",
		},
	}
	listener.OnEvent(event)

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) == 0 {
		t.Fatal("expected reply message")
	}
	if got := extractTextContent(calls[0].Content); got != "Which env?" {
		t.Fatalf("expected question message, got %q", got)
	}
	if !tracker.Sent() {
		t.Fatal("expected tracker marked sent")
	}
}
