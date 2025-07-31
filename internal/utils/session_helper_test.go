package utils

import (
	"bytes"
	"context"
	"log"
	"os"
	"testing"

	"alex/internal/llm"
	agentsession "alex/internal/session"
)

// setupTestLogger sets up log output to a buffer for testing
func setupTestLogger() func() {
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	return func() {
		log.SetOutput(os.Stderr)
	}
}

func TestSessionHelper_GetSessionWithFallback(t *testing.T) {
	defer setupTestLogger()()
	helper := NewSessionHelper("TEST")

	// 创建测试会话
	session1 := &agentsession.Session{ID: "session1"}
	session2 := &agentsession.Session{ID: "session2"}

	// 测试主会话存在的情况
	result := helper.GetSessionWithFallback(session1, session2)
	if result != session1 {
		t.Errorf("Expected session1, got: %v", result)
	}

	// 测试主会话为 nil，使用回退会话
	result = helper.GetSessionWithFallback(nil, session2)
	if result != session2 {
		t.Errorf("Expected session2, got: %v", result)
	}

	// 测试两个会话都为 nil
	result = helper.GetSessionWithFallback(nil, nil)
	if result != nil {
		t.Errorf("Expected nil, got: %v", result)
	}
}

func TestSessionHelper_ValidateSession(t *testing.T) {
	defer setupTestLogger()()
	helper := NewSessionHelper("TEST")

	// 测试有效会话
	validSession := &agentsession.Session{ID: "valid_session"}
	if !helper.ValidateSession(validSession) {
		t.Error("Expected valid session to pass validation")
	}

	// 测试 nil 会话
	if helper.ValidateSession(nil) {
		t.Error("Expected nil session to fail validation")
	}

	// 测试空 ID 会话
	emptyIDSession := &agentsession.Session{ID: ""}
	if helper.ValidateSession(emptyIDSession) {
		t.Error("Expected session with empty ID to fail validation")
	}
}

func TestSessionHelper_AddMessageToSession(t *testing.T) {
	defer setupTestLogger()()
	helper := NewSessionHelper("TEST")

	// 创建测试会话
	session := &agentsession.Session{
		ID:       "test_session",
		Messages: make([]*agentsession.Message, 0),
	}

	// 创建测试 LLM 消息
	llmMsg := &llm.Message{
		Role:    "assistant",
		Content: "test content",
	}

	// 测试添加消息
	helper.AddMessageToSession(llmMsg, session, nil)

	if len(session.Messages) != 1 {
		t.Fatalf("Expected 1 message, got: %d", len(session.Messages))
	}

	addedMsg := session.Messages[0]
	if addedMsg.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got: %s", addedMsg.Role)
	}
	if addedMsg.Content != "test content" {
		t.Errorf("Expected content 'test content', got: %s", addedMsg.Content)
	}

	// 检查元数据
	if addedMsg.Metadata["source"] != "llm_response" {
		t.Errorf("Expected source 'llm_response', got: %v", addedMsg.Metadata["source"])
	}
}

func TestSessionHelper_AddMessageToSession_WithToolCalls(t *testing.T) {
	defer setupTestLogger()()
	helper := NewSessionHelper("TEST")

	session := &agentsession.Session{
		ID:       "test_session",
		Messages: make([]*agentsession.Message, 0),
	}

	// 创建带工具调用的 LLM 消息
	llmMsg := &llm.Message{
		Role:    "assistant",
		Content: "test content",
		ToolCalls: []llm.ToolCall{
			{
				ID: "call_123",
				Function: llm.Function{
					Name:      "test_tool",
					Arguments: `{"param": "value"}`,
				},
			},
		},
	}

	helper.AddMessageToSession(llmMsg, session, nil)

	if len(session.Messages) != 1 {
		t.Fatalf("Expected 1 message, got: %d", len(session.Messages))
	}

	addedMsg := session.Messages[0]
	if len(addedMsg.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got: %d", len(addedMsg.ToolCalls))
	}

	toolCall := addedMsg.ToolCalls[0]
	if toolCall.ID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got: %s", toolCall.ID)
	}
	if toolCall.Function.Name != "test_tool" {
		t.Errorf("Expected tool call name 'test_tool', got: %s", toolCall.Function.Name)
	}

	// 检查工具调用元数据
	if addedMsg.Metadata["has_tool_calls"] != true {
		t.Errorf("Expected has_tool_calls true, got: %v", addedMsg.Metadata["has_tool_calls"])
	}
	if addedMsg.Metadata["tool_count"] != 1 {
		t.Errorf("Expected tool_count 1, got: %v", addedMsg.Metadata["tool_count"])
	}
}

func TestSessionHelper_AddMessageToSession_Fallback(t *testing.T) {
	defer setupTestLogger()()
	helper := NewSessionHelper("TEST")

	// 主会话为 nil，使用回退会话
	fallbackSession := &agentsession.Session{
		ID:       "fallback_session",
		Messages: make([]*agentsession.Message, 0),
	}

	llmMsg := &llm.Message{
		Role:    "user",
		Content: "fallback test",
	}

	helper.AddMessageToSession(llmMsg, nil, fallbackSession)

	if len(fallbackSession.Messages) != 1 {
		t.Fatalf("Expected 1 message in fallback session, got: %d", len(fallbackSession.Messages))
	}

	if fallbackSession.Messages[0].Content != "fallback test" {
		t.Errorf("Expected 'fallback test', got: %s", fallbackSession.Messages[0].Content)
	}
}

func TestSessionHelper_AddMessageToSession_InvalidSession(t *testing.T) {
	defer setupTestLogger()()
	helper := NewSessionHelper("TEST")

	llmMsg := &llm.Message{
		Role:    "user",
		Content: "should not be added",
	}

	// 测试无效会话 - 不应该崩溃
	helper.AddMessageToSession(llmMsg, nil, nil)

	// 测试空 ID 会话
	invalidSession := &agentsession.Session{ID: ""}
	helper.AddMessageToSession(llmMsg, invalidSession, nil)

	// 如果没有 panic，测试通过
}

func TestSessionHelper_AddToolResultToSession(t *testing.T) {
	defer setupTestLogger()()
	helper := NewSessionHelper("TEST")

	session := &agentsession.Session{
		ID:       "test_session",
		Messages: make([]*agentsession.Message, 0),
	}

	helper.AddToolResultToSession("tool result", "test_tool", "call_123", session, nil)

	if len(session.Messages) != 1 {
		t.Fatalf("Expected 1 message, got: %d", len(session.Messages))
	}

	addedMsg := session.Messages[0]
	if addedMsg.Role != "tool" {
		t.Errorf("Expected role 'tool', got: %s", addedMsg.Role)
	}
	if addedMsg.Content != "tool result" {
		t.Errorf("Expected content 'tool result', got: %s", addedMsg.Content)
	}

	// 检查元数据
	if addedMsg.Metadata["source"] != "tool_result" {
		t.Errorf("Expected source 'tool_result', got: %v", addedMsg.Metadata["source"])
	}
	if addedMsg.Metadata["tool_call_id"] != "call_123" {
		t.Errorf("Expected tool_call_id 'call_123', got: %v", addedMsg.Metadata["tool_call_id"])
	}
	if addedMsg.Metadata["tool_name"] != "test_tool" {
		t.Errorf("Expected tool_name 'test_tool', got: %v", addedMsg.Metadata["tool_name"])
	}
}

func TestSessionHelper_GetTodoFromSession(t *testing.T) {
	defer setupTestLogger()()
	helper := NewSessionHelper("TEST")

	// 创建有效会话
	session := &agentsession.Session{ID: "test_session"}

	// 调用方法 - 当前实现返回空字符串（占位符）
	result := helper.GetTodoFromSession(context.Background(), session, nil, nil)

	// 验证返回空字符串（符合当前实现）
	if result != "" {
		t.Errorf("Expected empty string for placeholder implementation, got: %s", result)
	}

	// 测试无效会话
	result = helper.GetTodoFromSession(context.Background(), nil, nil, nil)
	if result != "" {
		t.Errorf("Expected empty string for invalid session, got: %s", result)
	}
}

func TestSessionManager_ProcessLLMMessages(t *testing.T) {
	defer setupTestLogger()()
	manager := NewSessionManager("TEST")

	session := &agentsession.Session{
		ID:       "test_session",
		Messages: make([]*agentsession.Message, 0),
	}

	// 创建测试消息列表
	messages := []llm.Message{
		{Role: "system", Content: "system message"}, // 应该被跳过
		{Role: "assistant", Content: "assistant message"},
		{Role: "tool", Content: "tool result", Name: "test_tool", ToolCallId: "call_123"},
		{Role: "user", Content: "user message"}, // 不被处理，因为只处理assistant和tool
	}

	manager.ProcessLLMMessages(messages, session, nil)

	// 只有 assistant 和 tool 消息会被处理，system 和 user 消息被跳过
	if len(session.Messages) != 2 {
		t.Fatalf("Expected 2 messages (only assistant and tool processed), got: %d", len(session.Messages))
	}

	// 检查消息类型 - 只有 assistant 和 tool
	expectedRoles := []string{"assistant", "tool"}
	for i, expectedRole := range expectedRoles {
		if session.Messages[i].Role != expectedRole {
			t.Errorf("Expected role '%s' at index %d, got: %s", expectedRole, i, session.Messages[i].Role)
		}
	}

	// 检查工具消息的元数据
	toolMsg := session.Messages[1] // tool 消息
	if toolMsg.Metadata["tool_call_id"] != "call_123" {
		t.Errorf("Expected tool_call_id 'call_123', got: %v", toolMsg.Metadata["tool_call_id"])
	}
	if toolMsg.Metadata["tool_name"] != "test_tool" {
		t.Errorf("Expected tool_name 'test_tool', got: %v", toolMsg.Metadata["tool_name"])
	}
}

func TestGlobalSessionHelpers(t *testing.T) {
	defer setupTestLogger()()
	// 测试全局会话助手是否正确初始化
	helpers := []*SessionHelper{
		CoreSessionHelper,
		ReactSessionHelper,
		SubAgentSessionHelper,
	}

	for i, helper := range helpers {
		if helper == nil {
			t.Errorf("Global session helper %d is nil", i)
			continue
		}
		// 注意：我们无法直接访问 componentName，但可以验证 helper 不为 nil
	}
}

func TestNewSessionHelper(t *testing.T) {
	defer setupTestLogger()()
	helper := NewSessionHelper("CUSTOM")

	if helper == nil {
		t.Fatal("Expected non-nil session helper")
	}

	// 验证日志器已设置
	if helper.logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestNewSessionManager(t *testing.T) {
	defer setupTestLogger()()
	manager := NewSessionManager("CUSTOM")

	if manager == nil {
		t.Fatal("Expected non-nil session manager")
	}

	if manager.helper == nil {
		t.Error("Expected helper to be set")
	}
}

func BenchmarkSessionHelper_GetSessionWithFallback(b *testing.B) {
	helper := NewSessionHelper("BENCH")
	session1 := &agentsession.Session{ID: "session1"}
	session2 := &agentsession.Session{ID: "session2"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		helper.GetSessionWithFallback(session1, session2)
	}
}

func BenchmarkSessionHelper_ValidateSession(b *testing.B) {
	helper := NewSessionHelper("BENCH")
	session := &agentsession.Session{ID: "valid_session"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		helper.ValidateSession(session)
	}
}
