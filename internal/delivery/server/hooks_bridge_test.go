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

type mockNoticeLoader struct {
	chatID string
	ok     bool
	err    error
}

func (m *mockNoticeLoader) Load() (string, bool, error) {
	return m.chatID, m.ok, m.err
}

func TestHooksBridge_PostToolUse(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "test-token", "chat-123", nil)

	payload := hookPayload{
		Event:     "PostToolUse",
		ToolName:  "Bash",
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
	// Should show a friendly phrase instead of raw tool name
	if !containsAny(notifier.lastMessage, "运算", "执行", "实验") {
		t.Errorf("message should contain shell phrase, got: %s", notifier.lastMessage)
	}
	// Should show command detail
	if !strings.Contains(notifier.lastMessage, "npm test") {
		t.Errorf("message should contain command detail, got: %s", notifier.lastMessage)
	}
}

func TestHooksBridge_PostToolUseFileDetail(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-123", nil)

	payload := hookPayload{
		Event:     "PostToolUse",
		ToolName:  "read_file",
		ToolInput: json.RawMessage(`{"path":"/home/user/project/main.go"}`),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	// Should show file read phrase
	if !containsAny(notifier.lastMessage, "翻阅", "研读", "查阅") {
		t.Errorf("message should contain file read phrase, got: %s", notifier.lastMessage)
	}
	// Should show filename
	if !strings.Contains(notifier.lastMessage, "main.go") {
		t.Errorf("message should contain filename, got: %s", notifier.lastMessage)
	}
}

func TestHooksBridge_PostToolUseSearchDetail(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-123", nil)

	payload := hookPayload{
		Event:     "PostToolUse",
		ToolName:  "web_search",
		ToolInput: json.RawMessage(`{"query":"Go error handling best practices"}`),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !containsAny(notifier.lastMessage, "搜索", "探索", "挖掘") {
		t.Errorf("message should contain search phrase, got: %s", notifier.lastMessage)
	}
	if !strings.Contains(notifier.lastMessage, "Go error handling best practices") {
		t.Errorf("message should contain search query, got: %s", notifier.lastMessage)
	}
}

func TestHooksBridge_Stop(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-456", nil)

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

func TestHooksBridge_StopWithError(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-456", nil)

	payload := hookPayload{
		Event: "Stop",
		Error: "something went wrong",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(notifier.lastMessage, "出错了") {
		t.Errorf("message should contain error text, got: %s", notifier.lastMessage)
	}
}

func TestHooksBridge_Unauthorized(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "secret", "chat-123", nil)

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
	bridge := NewHooksBridge(notifier, nil, "", "chat-123", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/hooks/claude-code", nil)
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHooksBridge_ChatIDOverride(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "default-chat", nil)

	payload := hookPayload{Event: "Stop", Answer: "done"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code?chat_id=override-chat", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if notifier.lastChatID != "override-chat" {
		t.Errorf("expected chatID=override-chat, got %q", notifier.lastChatID)
	}
}

func TestHooksBridge_NoticeBindingFallback(t *testing.T) {
	notifier := &mockLarkNotifier{}
	loader := &mockNoticeLoader{chatID: "notice-chat", ok: true}
	bridge := NewHooksBridge(notifier, loader, "", "default-chat", nil)

	payload := hookPayload{Event: "Stop", Answer: "done"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	// Should use notice binding over default
	if notifier.lastChatID != "notice-chat" {
		t.Errorf("expected chatID=notice-chat (from notice binding), got %q", notifier.lastChatID)
	}
}

func TestHooksBridge_QueryParamOverridesNoticeBinding(t *testing.T) {
	notifier := &mockLarkNotifier{}
	loader := &mockNoticeLoader{chatID: "notice-chat", ok: true}
	bridge := NewHooksBridge(notifier, loader, "", "default-chat", nil)

	payload := hookPayload{Event: "Stop", Answer: "done"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code?chat_id=query-chat", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	// Query param takes highest priority
	if notifier.lastChatID != "query-chat" {
		t.Errorf("expected chatID=query-chat (from query param), got %q", notifier.lastChatID)
	}
}

func TestHooksBridge_NoticeLoaderError(t *testing.T) {
	notifier := &mockLarkNotifier{}
	loader := &mockNoticeLoader{err: context.DeadlineExceeded}
	bridge := NewHooksBridge(notifier, loader, "", "default-chat", nil)

	payload := hookPayload{Event: "Stop", Answer: "done"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	// Should fall back to default when notice loader errors
	if notifier.lastChatID != "default-chat" {
		t.Errorf("expected chatID=default-chat (fallback on error), got %q", notifier.lastChatID)
	}
}

func TestHooksBridge_UnknownEvent(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-123", nil)

	payload := hookPayload{Event: "UnknownEvent"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for unknown event, got %d", w.Code)
	}
}

func TestHooksBridge_PreToolUse(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-123", nil)

	payload := hookPayload{
		Event:     "PreToolUse",
		ToolName:  "write_file",
		ToolInput: json.RawMessage(`{"path":"/tmp/output.txt"}`),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/claude-code", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !containsAny(notifier.lastMessage, "撰写", "书写", "落笔") {
		t.Errorf("message should contain write phrase, got: %s", notifier.lastMessage)
	}
	if !strings.Contains(notifier.lastMessage, "output.txt") {
		t.Errorf("message should contain filename, got: %s", notifier.lastMessage)
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
