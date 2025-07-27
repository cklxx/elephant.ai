package utils

import (
	"testing"
)

func TestStreamHelper_CreateChunk(t *testing.T) {
	helper := NewStreamHelper("TEST")

	// 测试基本块创建
	chunk := helper.CreateChunk(ToolStart, "test content")
	
	if chunk.Type != string(ToolStart) {
		t.Errorf("Expected type '%s', got: %s", ToolStart, chunk.Type)
	}
	if chunk.Content != "test content" {
		t.Errorf("Expected content 'test content', got: %s", chunk.Content)
	}
	if chunk.Metadata["component"] != "TEST" {
		t.Errorf("Expected component 'TEST', got: %v", chunk.Metadata["component"])
	}

	// 测试带元数据的块创建
	metadata := map[string]interface{}{
		"custom_key": "custom_value",
		"iteration":  5,
	}
	chunk = helper.CreateChunk(ToolResult, "result content", metadata)
	
	if chunk.Metadata["custom_key"] != "custom_value" {
		t.Errorf("Expected custom_key 'custom_value', got: %v", chunk.Metadata["custom_key"])
	}
	if chunk.Metadata["iteration"] != 5 {
		t.Errorf("Expected iteration 5, got: %v", chunk.Metadata["iteration"])
	}
	// 应该仍然包含组件信息
	if chunk.Metadata["component"] != "TEST" {
		t.Errorf("Expected component 'TEST', got: %v", chunk.Metadata["component"])
	}
}

func TestStreamHelper_SendChunk(t *testing.T) {
	helper := NewStreamHelper("TEST")

	var receivedChunk StreamChunk
	var callbackCalled bool
	callback := func(chunk StreamChunk) {
		receivedChunk = chunk
		callbackCalled = true
	}

	helper.SendChunk(callback, Status, "test status")

	if !callbackCalled {
		t.Error("Expected callback to be called")
	}
	if receivedChunk.Type != string(Status) {
		t.Errorf("Expected type '%s', got: %s", Status, receivedChunk.Type)
	}
	if receivedChunk.Content != "test status" {
		t.Errorf("Expected content 'test status', got: %s", receivedChunk.Content)
	}

	// 测试 nil callback
	callbackCalled = false
	helper.SendChunk(nil, Status, "should not crash")
	if callbackCalled {
		t.Error("Expected callback not to be called for nil callback")
	}
}

func TestStreamHelper_SendToolStart(t *testing.T) {
	helper := NewStreamHelper("TEST")

	var receivedChunk StreamChunk
	callback := func(chunk StreamChunk) {
		receivedChunk = chunk
	}

	helper.SendToolStart(callback, "test_tool", "⏺ test_tool()")

	if receivedChunk.Type != string(ToolStart) {
		t.Errorf("Expected type '%s', got: %s", ToolStart, receivedChunk.Type)
	}
	if receivedChunk.Content != "⏺ test_tool()" {
		t.Errorf("Expected display content, got: %s", receivedChunk.Content)
	}
	if receivedChunk.Metadata["tool_name"] != "test_tool" {
		t.Errorf("Expected tool_name 'test_tool', got: %v", receivedChunk.Metadata["tool_name"])
	}
	if receivedChunk.Metadata["phase"] != "tool_start" {
		t.Errorf("Expected phase 'tool_start', got: %v", receivedChunk.Metadata["phase"])
	}
}

func TestStreamHelper_SendToolResult(t *testing.T) {
	helper := NewStreamHelper("TEST")

	var receivedChunk StreamChunk
	callback := func(chunk StreamChunk) {
		receivedChunk = chunk
	}

	// 测试成功结果
	helper.SendToolResult(callback, "test_tool", "success result", true)

	if receivedChunk.Type != string(ToolResult) {
		t.Errorf("Expected type '%s', got: %s", ToolResult, receivedChunk.Type)
	}
	if receivedChunk.Metadata["success"] != true {
		t.Errorf("Expected success true, got: %v", receivedChunk.Metadata["success"])
	}

	// 测试失败结果
	helper.SendToolResult(callback, "test_tool", "error result", false)

	if receivedChunk.Type != string(ToolError) {
		t.Errorf("Expected type '%s' for failed result, got: %s", ToolError, receivedChunk.Type)
	}
	if receivedChunk.Metadata["success"] != false {
		t.Errorf("Expected success false, got: %v", receivedChunk.Metadata["success"])
	}
}

func TestStreamHelper_SendToolError(t *testing.T) {
	helper := NewStreamHelper("TEST")

	var receivedChunk StreamChunk
	callback := func(chunk StreamChunk) {
		receivedChunk = chunk
	}

	helper.SendToolError(callback, "test_tool", "error message")

	if receivedChunk.Type != string(ToolError) {
		t.Errorf("Expected type '%s', got: %s", ToolError, receivedChunk.Type)
	}
	if receivedChunk.Content != "test_tool: error message" {
		t.Errorf("Expected formatted error content, got: %s", receivedChunk.Content)
	}
	if receivedChunk.Metadata["tool_name"] != "test_tool" {
		t.Errorf("Expected tool_name 'test_tool', got: %v", receivedChunk.Metadata["tool_name"])
	}
}

func TestStreamHelper_SendTokenUsage(t *testing.T) {
	helper := NewStreamHelper("TEST")

	var receivedChunk StreamChunk
	callback := func(chunk StreamChunk) {
		receivedChunk = chunk
	}

	helper.SendTokenUsage(callback, 100, 500, 60, 40, 3)

	if receivedChunk.Type != string(TokenUsage) {
		t.Errorf("Expected type '%s', got: %s", TokenUsage, receivedChunk.Type)
	}
	if receivedChunk.TokensUsed != 100 {
		t.Errorf("Expected TokensUsed 100, got: %d", receivedChunk.TokensUsed)
	}
	if receivedChunk.TotalTokensUsed != 500 {
		t.Errorf("Expected TotalTokensUsed 500, got: %d", receivedChunk.TotalTokensUsed)
	}
	if receivedChunk.PromptTokens != 60 {
		t.Errorf("Expected PromptTokens 60, got: %d", receivedChunk.PromptTokens)
	}
	if receivedChunk.CompletionTokens != 40 {
		t.Errorf("Expected CompletionTokens 40, got: %d", receivedChunk.CompletionTokens)
	}
	if receivedChunk.Metadata["iteration"] != 3 {
		t.Errorf("Expected iteration 3, got: %v", receivedChunk.Metadata["iteration"])
	}
}

func TestStreamHelper_SendFinalAnswer(t *testing.T) {
	helper := NewStreamHelper("TEST")

	var receivedChunk StreamChunk
	callback := func(chunk StreamChunk) {
		receivedChunk = chunk
	}

	helper.SendFinalAnswer(callback, "final answer", 5)

	if receivedChunk.Type != string(FinalAnswer) {
		t.Errorf("Expected type '%s', got: %s", FinalAnswer, receivedChunk.Type)
	}
	if receivedChunk.Content != "final answer" {
		t.Errorf("Expected content 'final answer', got: %s", receivedChunk.Content)
	}
	if receivedChunk.Metadata["iteration"] != 5 {
		t.Errorf("Expected iteration 5, got: %v", receivedChunk.Metadata["iteration"])
	}
	if receivedChunk.Metadata["phase"] != "final_answer" {
		t.Errorf("Expected phase 'final_answer', got: %v", receivedChunk.Metadata["phase"])
	}
}

func TestStreamHelper_SendError(t *testing.T) {
	helper := NewStreamHelper("TEST")

	var receivedChunk StreamChunk
	callback := func(chunk StreamChunk) {
		receivedChunk = chunk
	}

	helper.SendError(callback, "test error")

	if receivedChunk.Type != string(Error) {
		t.Errorf("Expected type '%s', got: %s", Error, receivedChunk.Type)
	}
	if receivedChunk.Content != "❌ test error" {
		t.Errorf("Expected formatted error content, got: %s", receivedChunk.Content)
	}
	if receivedChunk.Metadata["phase"] != "error" {
		t.Errorf("Expected phase 'error', got: %v", receivedChunk.Metadata["phase"])
	}
}

func TestStreamHelper_SendMaxIterations(t *testing.T) {
	helper := NewStreamHelper("TEST")

	var receivedChunk StreamChunk
	callback := func(chunk StreamChunk) {
		receivedChunk = chunk
	}

	helper.SendMaxIterations(callback, 100)

	if receivedChunk.Type != string(MaxIterations) {
		t.Errorf("Expected type '%s', got: %s", MaxIterations, receivedChunk.Type)
	}
	if receivedChunk.Content != "⚠️ Reached maximum iterations (100)" {
		t.Errorf("Expected formatted max iterations content, got: %s", receivedChunk.Content)
	}
	if receivedChunk.Metadata["max_iterations"] != 100 {
		t.Errorf("Expected max_iterations 100, got: %v", receivedChunk.Metadata["max_iterations"])
	}
}

func TestConditionalCallback(t *testing.T) {
	var receivedChunk StreamChunk
	var callbackCalled bool
	callback := func(chunk StreamChunk) {
		receivedChunk = chunk
		callbackCalled = true
	}

	// 测试有效回调
	cc := NewConditionalCallback(callback, "TEST")
	cc.Send(Status, "test message")

	if !callbackCalled {
		t.Error("Expected callback to be called")
	}
	if receivedChunk.Type != string(Status) {
		t.Errorf("Expected type '%s', got: %s", Status, receivedChunk.Type)
	}

	// 测试 nil 回调
	callbackCalled = false
	ccNil := NewConditionalCallback(nil, "TEST")
	ccNil.Send(Status, "should not crash")

	if callbackCalled {
		t.Error("Expected no callback for nil callback")
	}
}

func TestConditionalCallback_Methods(t *testing.T) {
	var receivedChunks []StreamChunk
	callback := func(chunk StreamChunk) {
		receivedChunks = append(receivedChunks, chunk)
	}

	cc := NewConditionalCallback(callback, "TEST")

	// 测试各种方法
	cc.ToolStart("test_tool", "⏺ test_tool()")
	cc.ToolResult("test_tool", "result", true)
	cc.ToolError("test_tool", "error")
	cc.Status("status message", "test_phase")
	cc.Error("error message")

	expectedTypes := []string{
		string(ToolStart),
		string(ToolResult),
		string(ToolError),
		string(Status),
		string(Error),
	}

	if len(receivedChunks) != len(expectedTypes) {
		t.Fatalf("Expected %d chunks, got: %d", len(expectedTypes), len(receivedChunks))
	}

	for i, expectedType := range expectedTypes {
		if receivedChunks[i].Type != expectedType {
			t.Errorf("Expected type '%s' at index %d, got: %s", expectedType, i, receivedChunks[i].Type)
		}
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{5, 3, 3},
		{1, 10, 1},
		{0, 0, 0},
		{-5, -3, -5},
		{-1, 1, -1},
	}

	for _, test := range tests {
		result := minInt(test.a, test.b)
		if result != test.expected {
			t.Errorf("minInt(%d, %d) = %d, expected %d", test.a, test.b, result, test.expected)
		}
	}
}

func TestGlobalStreamHelpers(t *testing.T) {
	// 测试全局流助手是否正确初始化
	helpers := []*StreamHelper{
		CoreStreamHelper,
		ReactStreamHelper,
		SubAgentStreamHelper,
		ToolStreamHelper,
	}

	for i, helper := range helpers {
		if helper == nil {
			t.Errorf("Global stream helper %d is nil", i)
		}
	}
}

func BenchmarkStreamHelper_CreateChunk(b *testing.B) {
	helper := NewStreamHelper("BENCH")
	metadata := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		helper.CreateChunk(ToolResult, "benchmark content", metadata)
	}
}

func BenchmarkStreamHelper_SendChunk(b *testing.B) {
	helper := NewStreamHelper("BENCH")
	callback := func(chunk StreamChunk) {
		// 模拟处理
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		helper.SendChunk(callback, Status, "benchmark message")
	}
}