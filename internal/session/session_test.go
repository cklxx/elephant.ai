package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/llm"
)

// TestSession_Creation 测试会话创建
func TestSession_Creation(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	sessionID := "test-session-" + time.Now().Format("20060102-150405")

	session, err := manager.StartSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	if session == nil {
		t.Fatal("Expected non-nil session")
	}

	if session.ID != sessionID {
		t.Errorf("Expected ID %s, got %s", sessionID, session.ID)
	}

	if session.Created.IsZero() {
		t.Error("Expected non-zero Created time")
	}

	if session.Updated.IsZero() {
		t.Error("Expected non-zero Updated time")
	}

	if session.Messages == nil {
		t.Fatal("Expected non-nil Messages slice")
	}

	if len(session.Messages) != 0 {
		t.Fatal("Expected empty Messages slice for new session")
	}
}

// TestSession_AddMessage 测试添加消息
func TestSession_AddMessage(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	session, err := manager.StartSession("test-add-msg")
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// 添加用户消息
	userMsg := &Message{
		Role:      "user",
		Content:   "Hello, this is a test message",
		Timestamp: time.Now(),
	}

	session.AddMessage(userMsg)

	if len(session.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(session.Messages))
	}

	if session.Messages[0].Role != "user" {
		t.Errorf("Expected role 'user', got %s", session.Messages[0].Role)
	}

	if session.Messages[0].Content != "Hello, this is a test message" {
		t.Errorf("Expected specific content, got %s", session.Messages[0].Content)
	}

	// 添加助手消息
	assistantMsg := &Message{
		Role:      "assistant",
		Content:   "Hello! I received your test message.",
		Timestamp: time.Now(),
	}

	session.AddMessage(assistantMsg)

	if len(session.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(session.Messages))
	}

	if session.Messages[1].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got %s", session.Messages[1].Role)
	}
}

// TestSession_GetMessages 测试获取消息
func TestSession_GetMessages(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	session, err := manager.StartSession("test-get-msg")
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// 添加几条消息
	messages := []*Message{
		{Role: "user", Content: "Message 1", Timestamp: time.Now()},
		{Role: "assistant", Content: "Response 1", Timestamp: time.Now()},
		{Role: "user", Content: "Message 2", Timestamp: time.Now()},
		{Role: "assistant", Content: "Response 2", Timestamp: time.Now()},
	}

	for _, msg := range messages {
		session.AddMessage(msg)
	}

	// 获取所有消息
	allMessages := session.GetMessages()
	if len(allMessages) != 4 {
		t.Errorf("Expected 4 messages, got %d", len(allMessages))
	}

	// 验证消息内容
	for i, msg := range allMessages {
		if msg.Content != messages[i].Content {
			t.Errorf("Message %d content mismatch: expected %s, got %s",
				i, messages[i].Content, msg.Content)
		}
	}
}

// TestSession_SetConfig 测试配置设置
func TestSession_SetConfig(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	session, err := manager.StartSession("test-config")
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// 设置配置
	session.SetConfig("test_key", "test_value")
	session.SetConfig("number_key", 42)
	session.SetConfig("bool_key", true)

	// 验证配置
	if session.Config == nil {
		t.Error("Expected non-nil Config map")
	}

	if session.Config["test_key"] != "test_value" {
		t.Errorf("Expected test_key = test_value, got %v", session.Config["test_key"])
	}

	if session.Config["number_key"] != 42 {
		t.Errorf("Expected number_key = 42, got %v", session.Config["number_key"])
	}

	if session.Config["bool_key"] != true {
		t.Errorf("Expected bool_key = true, got %v", session.Config["bool_key"])
	}
}

// TestSession_GetConfig 测试配置获取
func TestSession_GetConfig(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	session, err := manager.StartSession("test-get-config")
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// 设置配置
	session.SetConfig("existing_key", "existing_value")

	// 获取存在的配置
	value, exists := session.GetConfig("existing_key")
	if !exists {
		t.Error("Expected existing_key to exist")
	}
	if value != "existing_value" {
		t.Errorf("Expected existing_value, got %v", value)
	}

	// 获取不存在的配置
	_, exists = session.GetConfig("non_existing_key")
	if exists {
		t.Error("Expected non_existing_key to not exist")
	}
}

// TestManager_Creation 测试会话管理器创建
func TestManager_Creation(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Expected non-nil session manager")
	}

	if manager.sessionsDir == "" {
		t.Fatal("Expected non-empty sessions directory")
	}

	if manager.sessions == nil {
		t.Fatal("Expected non-nil sessions map")
	}
}

// TestManager_CreateSession 测试创建会话
func TestManager_CreateSession(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	sessionID := "test-create-" + time.Now().Format("20060102-150405")

	session, err := manager.StartSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	if session == nil {
		t.Fatal("Expected non-nil session")
	}

	if session.ID != sessionID {
		t.Errorf("Expected ID %s, got %s", sessionID, session.ID)
	}

	// 验证会话已创建成功（通过检查内部状态）
	if len(manager.sessions) == 0 {
		t.Fatal("Expected session to be stored in manager")
	}
}

// TestManager_RestoreSession 测试会话恢复
func TestManager_RestoreSession(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	sessionID := "test-restore-" + time.Now().Format("20060102-150405")

	// 创建会话
	session, err := manager.StartSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// 保存会话
	err = manager.SaveSession(session)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// 恢复会话
	restoredSession, err := manager.RestoreSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to restore session: %v", err)
	}

	if restoredSession.ID != sessionID {
		t.Errorf("Expected ID %s, got %s", sessionID, restoredSession.ID)
	}
}

// TestManager_SaveAndLoad 测试会话保存和加载
func TestManager_SaveAndLoad(t *testing.T) {
	// 使用临时目录
	tempDir := t.TempDir()

	// 创建管理器使用临时目录
	manager := &Manager{
		sessionsDir: tempDir,
		sessions:    make(map[string]*Session),
	}

	sessionID := "test-save-load-" + time.Now().Format("20060102-150405")

	// 创建会话
	session, err := manager.StartSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// 添加消息
	session.AddMessage(&Message{
		Role:      "user",
		Content:   "Test message for save/load",
		Timestamp: time.Now(),
	})

	// 设置配置
	session.SetConfig("test_save", "save_value")

	// 保存会话
	err = manager.SaveSession(session)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// 验证文件存在
	sessionFile := filepath.Join(tempDir, sessionID+".json")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Error("Expected session file to exist after save")
	}

	// 清除内存中的会话
	delete(manager.sessions, sessionID)

	// 恢复会话
	loadedSession, err := manager.RestoreSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to restore session: %v", err)
	}

	// 验证加载的会话
	if loadedSession.ID != sessionID {
		t.Errorf("Expected ID %s, got %s", sessionID, loadedSession.ID)
	}

	if len(loadedSession.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(loadedSession.Messages))
	}

	if loadedSession.Messages[0].Content != "Test message for save/load" {
		t.Errorf("Message content mismatch")
	}

	value, exists := loadedSession.GetConfig("test_save")
	if !exists || value != "save_value" {
		t.Errorf("Config not preserved during save/load")
	}
}

// TestManager_ListSessions 测试列出会话
func TestManager_ListSessions(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	// 创建几个会话
	sessionIDs := []string{
		"test-list-1-" + time.Now().Format("20060102-150405"),
		"test-list-2-" + time.Now().Format("20060102-150405"),
		"test-list-3-" + time.Now().Format("20060102-150405"),
	}

	for _, id := range sessionIDs {
		session, err := manager.StartSession(id)
		if err != nil {
			t.Fatalf("Failed to start session %s: %v", id, err)
		}
		// 保存会话到磁盘以便列出
		err = manager.SaveSession(session)
		if err != nil {
			t.Fatalf("Failed to save session %s: %v", id, err)
		}
	}

	// 列出会话
	sessionList, err := manager.ListSessions()
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	// 验证至少包含我们创建的会话
	for _, expectedID := range sessionIDs {
		found := false
		for _, sessionID := range sessionList {
			if sessionID == expectedID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find session %s in list", expectedID)
		}
	}
}

// TestMessage_ToolCalls 测试工具调用消息
func TestMessage_ToolCalls(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	session, err := manager.StartSession("test-tool-calls")
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// 创建带工具调用的消息
	toolCall := llm.ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: llm.Function{
			Name:        "test_tool",
			Arguments:   `{"param1": "value1", "param2": 42}`,
			Description: "Test tool description",
			Parameters: map[string]interface{}{
				"param1": "string",
				"param2": "number",
			},
		},
	}

	message := &Message{
		Role:      "assistant",
		Content:   "I need to use a tool",
		ToolCalls: []llm.ToolCall{toolCall},
		Timestamp: time.Now(),
	}

	session.AddMessage(message)

	// 验证工具调用保存正确
	savedMsg := session.Messages[0]
	if len(savedMsg.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(savedMsg.ToolCalls))
	}

	savedToolCall := savedMsg.ToolCalls[0]
	if savedToolCall.ID != "call_123" {
		t.Errorf("Expected tool call ID call_123, got %s", savedToolCall.ID)
	}

	if savedToolCall.Function.Name != "test_tool" {
		t.Errorf("Expected tool name test_tool, got %s", savedToolCall.Function.Name)
	}

	if savedToolCall.Function.Arguments != `{"param1": "value1", "param2": 42}` {
		t.Errorf("Expected param1 = value1, got %v", savedToolCall.Function.Arguments)
	}
}

// TestSession_ThreadSafety 测试线程安全
func TestSession_ThreadSafety(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	session, err := manager.StartSession("test-thread-safety")
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// 并发添加消息
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			message := &Message{
				Role:      "user",
				Content:   fmt.Sprintf("Concurrent message %d", id),
				Timestamp: time.Now(),
			}
			session.AddMessage(message)
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证所有消息都已添加
	if len(session.Messages) != 10 {
		t.Errorf("Expected 10 messages, got %d", len(session.Messages))
	}
}
