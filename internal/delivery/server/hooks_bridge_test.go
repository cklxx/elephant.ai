package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockLarkNotifier struct {
	lastChatID  string
	lastMessage string
	err         error
}

func (m *mockLarkNotifier) SendNotification(_ context.Context, chatID, text string) error {
	m.lastChatID = chatID
	m.lastMessage = text
	return m.err
}

func TestHooksBridge_PostToolUse(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, "test-token", "chat-123", nil)

	payload := hookPayload{
		Event:    "PostToolUse",
		ToolName: "Bash",
		ToolInput: json.RawMessage(`{"command":"npm test"}`),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if notifier.lastChatID != "chat-123" {
		t.Errorf("expected chatID=chat-123, got %q", notifier.lastChatID)
	}
	if !strings.Contains(notifier.lastMessage, "Bash") {
		t.Errorf("message should contain tool name, got: %s", notifier.lastMessage)
	}
}

func TestHooksBridge_Stop(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, "", "chat-456", nil)

	payload := hookPayload{
		Event:      "Stop",
		StopReason: "end_turn",
		Answer:     "Task completed successfully.",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(notifier.lastMessage, "任务完成") {
		t.Errorf("message should contain completion text, got: %s", notifier.lastMessage)
	}
	if !strings.Contains(notifier.lastMessage, "Task completed successfully.") {
		t.Errorf("message should contain answer, got: %s", notifier.lastMessage)
	}
}

func TestHooksBridge_Unauthorized(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, "secret", "chat-123", nil)

	payload := hookPayload{Event: "Stop"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHooksBridge_MethodNotAllowed(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, "", "chat-123", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/hooks/claude-code", nil)
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHooksBridge_ChatIDOverride(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, "", "default-chat", nil)

	payload := hookPayload{Event: "Stop", Answer: "done"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code?chat_id=override-chat", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if notifier.lastChatID != "override-chat" {
		t.Errorf("expected chatID=override-chat, got %q", notifier.lastChatID)
	}
}

func TestHooksBridge_UnknownEvent(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, "", "chat-123", nil)

	payload := hookPayload{Event: "UnknownEvent"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for unknown event, got %d", w.Code)
	}
}
