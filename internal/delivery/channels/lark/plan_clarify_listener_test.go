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

	event := domain.NewToolCompletedEvent(
		domain.NewBaseEvent(agent.LevelCore, "", "", "", time.Now()),
		"call-1", "plan", "fallback goal", nil, 0,
		map[string]any{"overall_goal_ui": "Ship feature"},
		nil,
	)
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

	event := domain.NewToolCompletedEvent(
		domain.NewBaseEvent(agent.LevelCore, "", "", "", time.Now()),
		"call-2", "clarify", "task\nWhich env?", nil, 0,
		map[string]any{"needs_user_input": true, "question_to_user": "Which env?"},
		nil,
	)
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

func TestPlanClarifyListenerClarifyOptionsSendsNumberedText(t *testing.T) {
	recorder := NewRecordingMessenger()
	tracker := &awaitQuestionTracker{}
	gw := &Gateway{
		messenger: recorder,
	}
	listener := newPlanClarifyListener(context.Background(), nil, gw, "oc_chat", "om_reply", tracker)

	event := domain.NewToolCompletedEvent(
		domain.NewBaseEvent(agent.LevelCore, "", "", "", time.Now()),
		"call-2b", "clarify", "", nil, 0,
		map[string]any{"needs_user_input": true, "question_to_user": "Which env?", "options": []string{"dev", "staging"}},
		nil,
	)
	listener.OnEvent(event)

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) == 0 {
		t.Fatal("expected reply message")
	}
	if calls[0].MsgType != "text" {
		t.Fatalf("expected text message, got %q", calls[0].MsgType)
	}
	content := extractTextContent(calls[0].Content, nil)
	if !strings.Contains(content, "Which env?") {
		t.Fatalf("expected question in text, got %s", content)
	}
	if !strings.Contains(content, "[1] dev") {
		t.Fatalf("expected numbered option [1] dev, got %s", content)
	}
	if !strings.Contains(content, "[2] staging") {
		t.Fatalf("expected numbered option [2] staging, got %s", content)
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
