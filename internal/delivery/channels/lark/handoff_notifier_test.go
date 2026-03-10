package lark

import (
	"strings"
	"testing"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/runtime/leader"
	"alex/internal/shared/logging"
)

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

func TestFormatHandoffMessage_MinimalContext(t *testing.T) {
	ctx := leader.HandoffContext{}
	msg := FormatHandoffMessage(ctx)
	if msg == "" {
		t.Fatal("expected non-empty message for zero-value context")
	}
	if !strings.Contains(msg, "需要你的帮助") {
		t.Errorf("expected header in minimal message:\n%s", msg)
	}
}

func TestFormatHandoffMessage_ProvideInputRecommendation(t *testing.T) {
	ctx := leader.HandoffContext{
		RecommendedAction: "provide_input",
	}
	msg := FormatHandoffMessage(ctx)
	if !strings.Contains(msg, "直接回复消息提供输入") {
		t.Errorf("expected provide_input recommendation:\n%s", msg)
	}
}

func TestFormatHandoffMessage_RetryRecommendation(t *testing.T) {
	ctx := leader.HandoffContext{
		RecommendedAction: "retry",
	}
	msg := FormatHandoffMessage(ctx)
	if !strings.Contains(msg, "/retry") {
		t.Errorf("expected retry recommendation:\n%s", msg)
	}
}

func TestHandoffNotifier_HandleHandoff_DispatchesMessage(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := &Gateway{
		messenger: rec,
		logger:    logging.OrNop(nil),
	}
	// Register a slot so resolveChatID finds the chat.
	gw.activeSlots.Store("chat-123", &sessionSlot{
		sessionID: "sess-42",
	})

	bus := hooks.NewInProcessBus()
	notifier := NewHandoffNotifier(gw, bus, "")

	ctx, cancel := t.Context(), func() {}
	_ = cancel
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
	text := extractTextContent(calls[0].Content, nil)
	if !strings.Contains(text, "deploy service") {
		t.Errorf("expected goal in message, got %q", text)
	}
	if !strings.Contains(text, "终止任务") {
		t.Errorf("expected abort recommendation, got %q", text)
	}
}

func TestHandoffNotifier_NoChatID_Drops(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := &Gateway{
		messenger: rec,
		logger:    logging.OrNop(nil),
	}
	// No active slots — no chat resolution possible.

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
