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
	messages    []string
	err         error
}

func (m *mockLarkNotifier) SendNotification(_ context.Context, chatID, text string) error {
	m.lastChatID = chatID
	m.lastMessage = text
	m.messages = append(m.messages, text)
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

// postHook is a test helper that sends a hook event and returns the recorder.
func postHook(bridge *HooksBridge, body string, token string, queryParams string) *httptest.ResponseRecorder {
	url := "/api/hooks/claude-code"
	if queryParams != "" {
		url += "?" + queryParams
	}
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)
	return w
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
	w := postHook(bridge, string(body), "test-token", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Flush aggregated buffer.
	bridge.Close(context.Background())

	if notifier.lastChatID != "chat-123" {
		t.Errorf("expected chatID=chat-123, got %q", notifier.lastChatID)
	}
	// Single tool use is sent as-is (no aggregation wrapper).
	if !containsAny(notifier.lastMessage, "运算", "执行", "实验") {
		t.Errorf("message should contain shell phrase, got: %s", notifier.lastMessage)
	}
	if !strings.Contains(notifier.lastMessage, "npm test") {
		t.Errorf("message should contain command detail, got: %s", notifier.lastMessage)
	}
}

func TestHooksBridge_HookEventNameToolUse(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-123", nil)

	body := `{"hook_event_name":"tool-use","tool_name":"Bash","tool_input":{"command":"echo hello"}}`
	w := postHook(bridge, body, "", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	bridge.Close(context.Background())

	if !containsAny(notifier.lastMessage, "运算", "执行", "实验") {
		t.Errorf("message should contain shell phrase, got: %s", notifier.lastMessage)
	}
	if !strings.Contains(notifier.lastMessage, "echo hello") {
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
	w := postHook(bridge, string(body), "", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	bridge.Close(context.Background())

	if !containsAny(notifier.lastMessage, "翻阅", "研读", "查阅") {
		t.Errorf("message should contain file read phrase, got: %s", notifier.lastMessage)
	}
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
	w := postHook(bridge, string(body), "", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	bridge.Close(context.Background())

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
	w := postHook(bridge, string(body), "", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(notifier.lastMessage, "任务已完成") {
		t.Errorf("message should contain completion text, got: %s", notifier.lastMessage)
	}
	if !strings.Contains(notifier.lastMessage, "Task completed successfully.") {
		t.Errorf("message should contain answer, got: %s", notifier.lastMessage)
	}
}

func TestHooksBridge_StopFallsBackToOutput(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-456", nil)

	payload := hookPayload{
		Event:      "Stop",
		StopReason: "end_turn",
		Output:     "Final output text.",
	}
	body, _ := json.Marshal(payload)
	w := postHook(bridge, string(body), "", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(notifier.lastMessage, "任务已完成") {
		t.Errorf("message should contain completion text, got: %s", notifier.lastMessage)
	}
	if !strings.Contains(notifier.lastMessage, "Final output text.") {
		t.Errorf("message should contain output fallback, got: %s", notifier.lastMessage)
	}
}

func TestHooksBridge_StopFromNestedEventAndFinalAnswer(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-456", nil)

	body := `{"event":{"name":"Stop"},"stop_reason":"end_turn","final_answer":"Done from final_answer"}`
	w := postHook(bridge, body, "", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(notifier.lastMessage, "任务已完成") {
		t.Errorf("message should contain completion text, got: %s", notifier.lastMessage)
	}
	if !strings.Contains(notifier.lastMessage, "Done from final_answer") {
		t.Errorf("message should contain final answer fallback, got: %s", notifier.lastMessage)
	}
}

func TestHooksBridge_NullFieldsNoLongerReturnInvalidJSON(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-456", nil)

	body := `{"event":null,"tool_name":null,"answer":null}`
	w := postHook(bridge, body, "", "")

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for empty/unknown event, got %d", w.Code)
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
	w := postHook(bridge, string(body), "", "")

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
	w := postHook(bridge, string(body), "wrong-token", "")

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
	w := postHook(bridge, string(body), "", "chat_id=override-chat")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
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
	w := postHook(bridge, string(body), "", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
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
	w := postHook(bridge, string(body), "", "chat_id=query-chat")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
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
	w := postHook(bridge, string(body), "", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if notifier.lastChatID != "default-chat" {
		t.Errorf("expected chatID=default-chat (fallback on error), got %q", notifier.lastChatID)
	}
}

func TestHooksBridge_UnknownEvent(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-123", nil)

	payload := hookPayload{Event: "UnknownEvent"}
	body, _ := json.Marshal(payload)
	w := postHook(bridge, string(body), "", "")

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
	w := postHook(bridge, string(body), "", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	bridge.Close(context.Background())

	if !containsAny(notifier.lastMessage, "撰写", "书写", "落笔") {
		t.Errorf("message should contain write phrase, got: %s", notifier.lastMessage)
	}
	if !strings.Contains(notifier.lastMessage, "output.txt") {
		t.Errorf("message should contain filename, got: %s", notifier.lastMessage)
	}
}

func TestHooksBridge_PreToolUseIncludesThinking(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-123", nil)

	payload := hookPayload{
		Event:     "PreToolUse",
		ToolName:  "Bash",
		Thinking:  "先确认当前目录结构，再执行命令。",
		ToolInput: json.RawMessage(`{"command":"ls -la"}`),
	}
	body, _ := json.Marshal(payload)
	w := postHook(bridge, string(body), "", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	bridge.Close(context.Background())

	if !strings.Contains(notifier.lastMessage, "思路：先确认当前目录结构，再执行命令。") {
		t.Errorf("message should contain thinking line, got: %s", notifier.lastMessage)
	}
	if !strings.Contains(notifier.lastMessage, "ls -la") {
		t.Errorf("message should contain command detail, got: %s", notifier.lastMessage)
	}
}

func TestDecodeHookPayload_ExtractsThinking(t *testing.T) {
	body := `{"event":"PreToolUse","tool_name":"Bash","thinking":{"parts":[{"text":"先定位入口"},{"text":"再调用工具"}]}}`
	p, err := decodeHookPayload([]byte(body))
	if err != nil {
		t.Fatalf("decodeHookPayload returned error: %v", err)
	}
	if p.Thinking != "先定位入口 再调用工具" {
		t.Fatalf("expected extracted thinking, got %q", p.Thinking)
	}
}

func TestDecodeHookPayload_ExtractsReasoningAlias(t *testing.T) {
	body := `{"event":"PreToolUse","tool_name":"Bash","reasoning":"  先搜文档\n再跑命令  "}`
	p, err := decodeHookPayload([]byte(body))
	if err != nil {
		t.Fatalf("decodeHookPayload returned error: %v", err)
	}
	if p.Thinking != "先搜文档 再跑命令" {
		t.Fatalf("expected compacted reasoning text, got %q", p.Thinking)
	}
}

// ---------------------------------------------------------------------------
// Aggregation-specific tests
// ---------------------------------------------------------------------------

func TestHooksBridge_AggregatesMultipleToolUses(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-123", nil)

	// Send 3 tool events — all buffered, none sent yet.
	tools := []hookPayload{
		{Event: "PostToolUse", ToolName: "read_file", ToolInput: json.RawMessage(`{"path":"a.go"}`)},
		{Event: "PostToolUse", ToolName: "Bash", ToolInput: json.RawMessage(`{"command":"go test"}`)},
		{Event: "PostToolUse", ToolName: "write_file", ToolInput: json.RawMessage(`{"path":"b.go"}`)},
	}
	for _, p := range tools {
		body, _ := json.Marshal(p)
		postHook(bridge, string(body), "", "")
	}

	if len(notifier.messages) != 0 {
		t.Fatalf("expected 0 sends while buffering, got %d", len(notifier.messages))
	}

	bridge.Close(context.Background())

	if len(notifier.messages) != 1 {
		t.Fatalf("expected 1 aggregated send after Close, got %d", len(notifier.messages))
	}
	msg := notifier.lastMessage
	if !strings.Contains(msg, "3 步操作") {
		t.Errorf("expected '3 tool calls' in aggregated message, got: %s", msg)
	}
}

func TestHooksBridge_StopFlushesBuffer(t *testing.T) {
	notifier := &mockLarkNotifier{}
	bridge := NewHooksBridge(notifier, nil, "", "chat-123", nil)

	// Buffer a tool event.
	p := hookPayload{Event: "PostToolUse", ToolName: "Bash", ToolInput: json.RawMessage(`{"command":"ls"}`)}
	body, _ := json.Marshal(p)
	postHook(bridge, string(body), "", "")

	if len(notifier.messages) != 0 {
		t.Fatalf("expected 0 sends while buffering, got %d", len(notifier.messages))
	}

	// Stop should flush the buffered tool event first, then send Stop.
	stop := hookPayload{Event: "Stop", Answer: "all done"}
	body, _ = json.Marshal(stop)
	postHook(bridge, string(body), "", "")

	if len(notifier.messages) != 2 {
		t.Fatalf("expected 2 sends (flushed tool + stop), got %d: %v", len(notifier.messages), notifier.messages)
	}
	if !strings.Contains(notifier.messages[1], "任务已完成") {
		t.Errorf("second message should be stop, got: %s", notifier.messages[1])
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
