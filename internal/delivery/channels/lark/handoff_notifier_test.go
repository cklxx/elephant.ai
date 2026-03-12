package lark

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/runtime/leader"
	"alex/internal/shared/logging"
)

func TestFormatHandoffCard_Complete(t *testing.T) {
	ctx := leader.HandoffContext{
		SessionID:         "sess-42",
		Member:            "claude_code",
		Goal:              "fix the login bug",
		Reason:            "session stalled 3 times, escalating to human",
		StallCount:        3,
		Elapsed:           "5m30s",
		RecommendedAction: "abort",
	}
	card := FormatHandoffCard(ctx)

	// Must be valid JSON.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(card), &parsed); err != nil {
		t.Fatalf("card is not valid JSON: %v\n%s", err, card)
	}

	// Header present with red template (abort).
	header, _ := parsed["header"].(map[string]any)
	if header == nil {
		t.Fatal("expected header in card")
	}
	tmpl, _ := header["template"].(string)
	if tmpl != "red" {
		t.Errorf("expected header template=red for abort, got %q", tmpl)
	}
	title, _ := header["title"].(map[string]any)
	content, _ := title["content"].(string)
	if !strings.Contains(content, "需要你的帮助") {
		t.Errorf("header title missing '需要你的帮助': %q", content)
	}

	// Elements contain markdown + actions.
	elements, _ := parsed["elements"].([]any)
	if len(elements) < 2 {
		t.Fatalf("expected at least 2 elements (markdown + action), got %d", len(elements))
	}

	// First element is markdown with diagnostic fields.
	md, _ := elements[0].(map[string]any)
	mdContent, _ := md["content"].(string)
	for _, expected := range []string{
		"fix the login bug",
		"claude_code",
		"session stalled 3 times",
		"3 次",
		"5m30s",
	} {
		if !strings.Contains(mdContent, expected) {
			t.Errorf("card markdown missing %q:\n%s", expected, mdContent)
		}
	}

	// Last element is action with 3 buttons.
	actionElem, _ := elements[len(elements)-1].(map[string]any)
	if actionElem["tag"] != "action" {
		t.Fatalf("expected last element tag=action, got %v", actionElem["tag"])
	}
	actions, _ := actionElem["actions"].([]any)
	if len(actions) != 3 {
		t.Fatalf("expected 3 action buttons, got %d", len(actions))
	}
}

func TestFormatHandoffCard_YellowForNonAbort(t *testing.T) {
	ctx := leader.HandoffContext{RecommendedAction: "provide_input"}
	card := FormatHandoffCard(ctx)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(card), &parsed); err != nil {
		t.Fatal(err)
	}
	header, _ := parsed["header"].(map[string]any)
	if header["template"] != "yellow" {
		t.Errorf("expected yellow template for non-abort, got %v", header["template"])
	}
}

func TestFormatHandoffCard_ButtonsContainSessionID(t *testing.T) {
	ctx := leader.HandoffContext{SessionID: "sess-99"}
	card := FormatHandoffCard(ctx)
	var parsed map[string]any
	_ = json.Unmarshal([]byte(card), &parsed)

	elements, _ := parsed["elements"].([]any)
	actionElem, _ := elements[len(elements)-1].(map[string]any)
	actions, _ := actionElem["actions"].([]any)

	for i, btn := range actions {
		b, _ := btn.(map[string]any)
		val, _ := b["value"].(map[string]any)
		if val["session_id"] != "sess-99" {
			t.Errorf("button %d missing session_id=sess-99: %v", i, val)
		}
	}
}

func TestFormatHandoffCard_WithDiagnostics(t *testing.T) {
	ctx := leader.HandoffContext{
		LastToolCall: "bash: make deploy",
		LastError:    "connection refused",
		SessionTail:  []string{"user: deploy it", "tool: bash failed"},
	}
	card := FormatHandoffCard(ctx)
	for _, expected := range []string{
		"最后工具调用",
		"bash: make deploy",
		"最后错误",
		"connection refused",
		"最近消息",
		"user: deploy it",
		"tool: bash failed",
	} {
		if !strings.Contains(card, expected) {
			t.Errorf("card missing %q", expected)
		}
	}
}

func TestFormatHandoffCard_NoDiagnostics(t *testing.T) {
	ctx := leader.HandoffContext{Goal: "test"}
	card := FormatHandoffCard(ctx)
	if strings.Contains(card, "最后工具调用") {
		t.Error("should not contain tool call when empty")
	}
	if strings.Contains(card, "最后错误") {
		t.Error("should not contain error when empty")
	}
	if strings.Contains(card, "最近消息") {
		t.Error("should not contain session tail when empty")
	}
}

// FormatHandoffMessage is retained for fallback — verify it still works.
func TestFormatHandoffMessage_Complete(t *testing.T) {
	ctx := leader.HandoffContext{
		SessionID:         "sess-42",
		Member:            "claude_code",
		Goal:              "fix the login bug",
		Reason:            "session stalled 3 times, escalating to human",
		StallCount:        3,
		Elapsed:           "5m30s",
		RecommendedAction: "abort",
	}
	msg := FormatHandoffMessage(ctx)
	for _, expected := range []string{
		"需要你的帮助",
		"fix the login bug",
		"claude_code",
		"session stalled 3 times",
		"3 次",
		"5m30s",
		"终止任务",
	} {
		if !strings.Contains(msg, expected) {
			t.Errorf("message missing %q:\n%s", expected, msg)
		}
	}
}

func TestHandoffNotifier_HandleHandoff_DispatchesInteractiveCard(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := &Gateway{
		messenger: rec,
		logger:    logging.OrNop(nil),
	}
	gw.activeSlots.Store("chat-123", &sessionSlot{
		sessionID: "sess-42",
	})

	bus := hooks.NewInProcessBus()
	notifier := NewHandoffNotifier(gw, bus, "")

	ctx := t.Context()
	go notifier.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	payload := leader.HandoffContext{
		SessionID:         "sess-42",
		Member:            "claude_code",
		Goal:              "deploy service",
		Reason:            "max stalls reached",
		StallCount:        3,
		RecommendedAction: "abort",
	}
	bus.Publish("sess-42", hooks.Event{
		Type:      hooks.EventHandoffRequired,
		SessionID: "sess-42",
		At:        time.Now(),
		Payload:   payload.ToPayload(),
	})
	time.Sleep(100 * time.Millisecond)

	calls := rec.CallsByMethod(MethodSendMessage)
	if len(calls) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(calls))
	}
	if calls[0].ChatID != "chat-123" {
		t.Errorf("expected chat_id=chat-123, got %q", calls[0].ChatID)
	}
	// Content type must be interactive.
	if calls[0].MsgType != "interactive" {
		t.Errorf("expected msg_type=interactive, got %q", calls[0].MsgType)
	}
	// Content must be valid card JSON containing the goal.
	if !strings.Contains(calls[0].Content, "deploy service") {
		t.Errorf("expected card to contain goal, got %q", calls[0].Content)
	}
	if !strings.Contains(calls[0].Content, "handoff_retry") {
		t.Errorf("expected card to contain retry action")
	}
}

func TestHandoffNotifier_NoChatID_Drops(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := &Gateway{
		messenger: rec,
		logger:    logging.OrNop(nil),
	}

	bus := hooks.NewInProcessBus()
	notifier := NewHandoffNotifier(gw, bus, "")

	go notifier.Run(t.Context())
	time.Sleep(10 * time.Millisecond)
	bus.Publish("unknown-sess", hooks.Event{
		Type:      hooks.EventHandoffRequired,
		SessionID: "unknown-sess",
		At:        time.Now(),
		Payload:   map[string]any{"reason": "test"},
	})
	time.Sleep(100 * time.Millisecond)

	calls := rec.CallsByMethod(MethodSendMessage)
	if len(calls) != 0 {
		t.Fatalf("expected 0 send calls when no chat ID, got %d", len(calls))
	}
}

func TestHandoffNotifier_DefaultChatID_Fallback(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := &Gateway{
		messenger: rec,
		logger:    logging.OrNop(nil),
	}

	bus := hooks.NewInProcessBus()
	notifier := NewHandoffNotifier(gw, bus, "fallback-chat")

	go notifier.Run(t.Context())
	time.Sleep(10 * time.Millisecond)
	bus.Publish("unknown-sess", hooks.Event{
		Type:      hooks.EventHandoffRequired,
		SessionID: "unknown-sess",
		At:        time.Now(),
		Payload:   map[string]any{"reason": "fallback test"},
	})
	time.Sleep(100 * time.Millisecond)

	calls := rec.CallsByMethod(MethodSendMessage)
	if len(calls) != 1 {
		t.Fatalf("expected 1 send call with fallback, got %d", len(calls))
	}
	if calls[0].ChatID != "fallback-chat" {
		t.Errorf("expected fallback chat_id, got %q", calls[0].ChatID)
	}
}

func TestHandleHandoffAction_Retry(t *testing.T) {
	rec := NewRecordingMessenger()
	bus := hooks.NewInProcessBus()
	gw := &Gateway{
		messenger:  rec,
		logger:     logging.OrNop(nil),
		runtimeBus: bus,
	}

	ch, cancel := bus.Subscribe("sess-1")
	defer cancel()

	gw.HandleHandoffAction(context.Background(), "chat-1", handoffActionRetry, "sess-1")

	select {
	case ev := <-ch:
		if ev.Type != hooks.EventStalled {
			t.Errorf("expected EventStalled, got %s", ev.Type)
		}
		if ev.SessionID != "sess-1" {
			t.Errorf("expected session_id=sess-1, got %q", ev.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stall event")
	}

	// Should send confirmation.
	calls := rec.CallsByMethod(MethodSendMessage)
	if len(calls) != 1 {
		t.Fatalf("expected 1 confirmation message, got %d", len(calls))
	}
	text := extractTextContent(calls[0].Content, nil)
	if !strings.Contains(text, "重试") {
		t.Errorf("expected retry confirmation, got %q", text)
	}
}

func TestHandleHandoffAction_Abort(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := &Gateway{
		messenger: rec,
		logger:    logging.OrNop(nil),
	}

	cancelled := false
	gw.activeSlots.Store("chat-1", &sessionSlot{
		phase:      slotRunning,
		taskCancel: func() { cancelled = true },
		taskToken:  1,
	})

	gw.HandleHandoffAction(context.Background(), "chat-1", handoffActionAbort, "sess-1")

	if !cancelled {
		t.Error("expected task to be cancelled")
	}

	calls := rec.CallsByMethod(MethodSendMessage)
	if len(calls) != 1 {
		t.Fatalf("expected 1 message, got %d", len(calls))
	}
	text := extractTextContent(calls[0].Content, nil)
	if !strings.Contains(text, "终止") {
		t.Errorf("expected abort confirmation, got %q", text)
	}
}

func TestHandleHandoffAction_Abort_NoSlot(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := &Gateway{
		messenger: rec,
		logger:    logging.OrNop(nil),
	}

	gw.HandleHandoffAction(context.Background(), "chat-1", handoffActionAbort, "sess-1")

	calls := rec.CallsByMethod(MethodSendMessage)
	if len(calls) != 1 {
		t.Fatalf("expected 1 message, got %d", len(calls))
	}
	text := extractTextContent(calls[0].Content, nil)
	if !strings.Contains(text, "未找到") {
		t.Errorf("expected 'not found' message, got %q", text)
	}
}

func TestHandleHandoffAction_ProvideInput(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := &Gateway{
		messenger: rec,
		logger:    logging.OrNop(nil),
	}
	gw.activeSlots.Store("chat-1", &sessionSlot{
		phase: slotRunning,
	})

	gw.HandleHandoffAction(context.Background(), "chat-1", handoffActionProvideInput, "sess-1")

	raw, _ := gw.activeSlots.Load("chat-1")
	slot := raw.(*sessionSlot)
	slot.mu.Lock()
	phase := slot.phase
	slot.mu.Unlock()
	if phase != slotAwaitingInput {
		t.Errorf("expected slot phase=awaitingInput, got %d", phase)
	}
}
