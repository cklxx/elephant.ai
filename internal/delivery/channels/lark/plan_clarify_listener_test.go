package lark

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
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
	if got := extractTextContent(calls[0].Content, nil); got != "Ship feature" {
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
	if got := extractTextContent(calls[0].Content, nil); got != "Which env?" {
		t.Fatalf("expected question message, got %q", got)
	}
	if !tracker.Sent() {
		t.Fatal("expected tracker marked sent")
	}
}

func TestPlanClarifyListenerClarifyOptionsSendsInteractiveCard(t *testing.T) {
	recorder := NewRecordingMessenger()
	tracker := &awaitQuestionTracker{}
	gw := &Gateway{
		messenger: recorder,
		cfg:       Config{CardsEnabled: true},
	}
	listener := newPlanClarifyListener(context.Background(), nil, gw, "oc_chat", "om_reply", tracker)

	event := &domain.WorkflowToolCompletedEvent{
		CallID:   "call-2b",
		ToolName: "clarify",
		Metadata: map[string]any{
			"needs_user_input": true,
			"question_to_user": "Which env?",
			"options":          []string{"dev", "staging"},
		},
	}
	listener.OnEvent(event)

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) == 0 {
		t.Fatal("expected reply message")
	}
	if calls[0].MsgType != "interactive" {
		t.Fatalf("expected interactive message, got %q", calls[0].MsgType)
	}
	if !strings.Contains(calls[0].Content, `"action_tag":"await_choice_select"`) {
		t.Fatalf("expected await choice action tag, got %s", calls[0].Content)
	}
	if !tracker.Sent() {
		t.Fatal("expected tracker marked sent")
	}
}

func TestPlanClarifyListenerEnvelopePlan(t *testing.T) {
	recorder := NewRecordingMessenger()
	gw := &Gateway{messenger: recorder}
	listener := newPlanClarifyListener(context.Background(), nil, gw, "oc_chat", "om_reply", nil)

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventToolCompleted,
		NodeID:    "call-3",
		Payload: map[string]any{
			"tool_name": "plan",
			"metadata": map[string]any{
				"overall_goal_ui": "Launch build",
			},
			"result": "ignored",
		},
	}
	listener.OnEvent(event)

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) == 0 {
		t.Fatal("expected reply message")
	}
	if got := extractTextContent(calls[0].Content, nil); got != "Launch build" {
		t.Fatalf("expected goal message, got %q", got)
	}
}
