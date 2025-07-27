package utils

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"alex/pkg/types"
)

func TestToolExecutor_ExecuteToolWithRecovery(t *testing.T) {
	executor := NewToolExecutor("TEST")

	// 测试成功执行
	t.Run("successful execution", func(t *testing.T) {
		toolCall := &types.ReactToolCall{
			Name:      "test_tool",
			Arguments: map[string]interface{}{"arg1": "value1"},
			CallID:    "call_123",
		}

		mockExecutor := func(ctx context.Context, toolName string, args map[string]interface{}, callID string) (*types.ReactToolResult, error) {
			return &types.ReactToolResult{
				Success:  true,
				Content:  "test result",
				ToolName: toolName,
				ToolArgs: args,
				CallID:   callID,
			}, nil
		}

		var callbackCalled bool
		callback := func(chunk StreamChunk) {
			callbackCalled = true
			if chunk.Type != "tool_result" {
				t.Errorf("Expected tool_result chunk type, got: %s", chunk.Type)
			}
		}

		result := executor.ExecuteToolWithRecovery(context.Background(), toolCall, mockExecutor, callback)

		if !result.Success {
			t.Error("Expected successful result")
		}
		if result.Content != "test result" {
			t.Errorf("Expected 'test result', got: %s", result.Content)
		}
		if result.CallID != "call_123" {
			t.Errorf("Expected 'call_123', got: %s", result.CallID)
		}
		if !callbackCalled {
			t.Error("Expected callback to be called")
		}
	})

	// 测试执行错误
	t.Run("execution error", func(t *testing.T) {
		toolCall := &types.ReactToolCall{
			Name:   "failing_tool",
			CallID: "call_456",
		}

		mockExecutor := func(ctx context.Context, toolName string, args map[string]interface{}, callID string) (*types.ReactToolResult, error) {
			return nil, errors.New("execution failed")
		}

		var errorCallbackCalled bool
		callback := func(chunk StreamChunk) {
			if chunk.Type == "tool_error" {
				errorCallbackCalled = true
			}
		}

		result := executor.ExecuteToolWithRecovery(context.Background(), toolCall, mockExecutor, callback)

		if result.Success {
			t.Error("Expected failed result")
		}
		if result.Error != "execution failed" {
			t.Errorf("Expected 'execution failed', got: %s", result.Error)
		}
		if !errorCallbackCalled {
			t.Error("Expected error callback to be called")
		}
	})

	// 测试 panic 恢复
	t.Run("panic recovery", func(t *testing.T) {
		toolCall := &types.ReactToolCall{
			Name:   "panicking_tool",
			CallID: "call_789",
		}

		mockExecutor := func(ctx context.Context, toolName string, args map[string]interface{}, callID string) (*types.ReactToolResult, error) {
			panic("test panic")
		}

		var panicCallbackCalled bool
		callback := func(chunk StreamChunk) {
			if chunk.Type == "tool_error" && chunk.Content == "panicking_tool: panic occurred" {
				panicCallbackCalled = true
			}
		}

		result := executor.ExecuteToolWithRecovery(context.Background(), toolCall, mockExecutor, callback)

		if result.Success {
			t.Error("Expected failed result after panic")
		}
		if !panicCallbackCalled {
			t.Error("Expected panic callback to be called")
		}
	})
}

func TestToolExecutor_ValidateAndFixResult(t *testing.T) {
	executor := NewToolExecutor("TEST")

	originalCall := &types.ReactToolCall{
		Name:      "test_tool",
		Arguments: map[string]interface{}{"key": "value"},
		CallID:    "original_call",
	}

	t.Run("nil result fix", func(t *testing.T) {
		result := executor.ValidateAndFixResult(nil, originalCall)
		
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.Success {
			t.Error("Expected failed result for nil input")
		}
		if result.CallID != "original_call" {
			t.Errorf("Expected 'original_call', got: %s", result.CallID)
		}
	})

	t.Run("callid mismatch fix", func(t *testing.T) {
		result := &types.ReactToolResult{
			Success:  true,
			Content:  "test",
			CallID:   "wrong_call_id",
			ToolName: "test_tool",
		}

		fixed := executor.ValidateAndFixResult(result, originalCall)
		
		if fixed.CallID != "original_call" {
			t.Errorf("Expected CallID to be fixed to 'original_call', got: %s", fixed.CallID)
		}
	})

	t.Run("missing tool name fix", func(t *testing.T) {
		result := &types.ReactToolResult{
			Success: true,
			Content: "test",
			CallID:  "original_call",
		}

		fixed := executor.ValidateAndFixResult(result, originalCall)
		
		if fixed.ToolName != "test_tool" {
			t.Errorf("Expected ToolName to be set to 'test_tool', got: %s", fixed.ToolName)
		}
	})
}

func TestToolExecutor_ExecuteSerialToolsWithRecovery(t *testing.T) {
	executor := NewToolExecutor("TEST")

	// 测试空工具调用列表
	t.Run("empty tool calls", func(t *testing.T) {
		results := executor.ExecuteSerialToolsWithRecovery(
			context.Background(),
			[]*types.ReactToolCall{},
			nil,
			nil,
			nil,
		)

		if len(results) != 1 {
			t.Fatalf("Expected 1 result for empty input, got: %d", len(results))
		}
		if results[0].Success {
			t.Error("Expected failed result for empty input")
		}
	})

	// 测试多个工具调用
	t.Run("multiple tool calls", func(t *testing.T) {
		toolCalls := []*types.ReactToolCall{
			{Name: "tool1", CallID: "call1"},
			{Name: "tool2", CallID: "call2"},
		}

		mockExecutor := func(ctx context.Context, toolName string, args map[string]interface{}, callID string) (*types.ReactToolResult, error) {
			return &types.ReactToolResult{
				Success:  true,
				Content:  "result for " + toolName,
				ToolName: toolName,
				CallID:   callID,
			}, nil
		}

		var callbackCount int
		callback := func(chunk StreamChunk) {
			callbackCount++
		}

		formatter := func(toolName string, args map[string]interface{}) string {
			return toolName + "()"
		}

		results := executor.ExecuteSerialToolsWithRecovery(
			context.Background(),
			toolCalls,
			mockExecutor,
			callback,
			formatter,
		)

		if len(results) != 2 {
			t.Fatalf("Expected 2 results, got: %d", len(results))
		}
		
		for i, result := range results {
			if !result.Success {
				t.Errorf("Expected successful result %d", i)
			}
			expectedName := toolCalls[i].Name
			if result.ToolName != expectedName {
				t.Errorf("Expected tool name '%s', got: %s", expectedName, result.ToolName)
			}
		}

		if callbackCount == 0 {
			t.Error("Expected callback to be called")
		}
	})
}

func TestGenerateCallID(t *testing.T) {
	callID1 := GenerateCallID("test_tool")
	// Add a small delay to ensure different timestamps
	time.Sleep(1 * time.Millisecond)
	callID2 := GenerateCallID("test_tool")

	if callID1 == callID2 {
		t.Error("Expected different call IDs for same tool")
	}

	if !contains(callID1, "test_tool") {
		t.Errorf("Expected call ID to contain tool name, got: %s", callID1)
	}

	if !contains(callID1, "fallback_") {
		t.Errorf("Expected call ID to contain 'fallback_', got: %s", callID1)
	}
}

func TestToolDisplayFormatter_Format(t *testing.T) {
	formatter := NewToolDisplayFormatter(32) // Green color

	// 测试无参数格式化
	result := formatter.Format("test_tool", nil)
	if !contains(result, "test_tool()") {
		t.Errorf("Expected 'test_tool()' in result, got: %s", result)
	}

	// 测试带参数格式化
	args := map[string]interface{}{
		"param1": "value1",
		"param2": 42,
	}
	result = formatter.Format("test_tool", args)
	
	if !contains(result, "test_tool") {
		t.Errorf("Expected tool name in result, got: %s", result)
	}
	// 注意：参数顺序可能不确定，所以只检查包含工具名
}

func TestToolDisplayFormatter_DefaultColor(t *testing.T) {
	formatter := NewToolDisplayFormatter() // 默认绿色
	result := formatter.Format("test", nil)
	
	// 应该包含默认的绿色 ANSI 代码
	if !contains(result, "\033[32m⏺\033[0m") {
		t.Errorf("Expected default green color in result, got: %s", result)
	}
}

// 辅助函数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestToolExecutor_FormatToolResultContent(t *testing.T) {
	executor := NewToolExecutor("TEST")

	// 测试 file_read 长内容截断和对齐
	t.Run("file_read long content truncation and alignment", func(t *testing.T) {
		longContent := "     Line 1 with leading spaces\n  Line 2\n    Line 3\nLine 4\nLine 5\nLine 6"
		result := executor.formatToolResultContent("file_read", longContent)
		
		if !contains(result, "Line 1 with leading spaces") {
			t.Errorf("Expected first line to be preserved, got: %s", result)
		}
		if !contains(result, "(6 total lines)") {
			t.Errorf("Expected total line count, got: %s", result)
		}
		
		lines := strings.Split(result, "\n")
		if len(lines) > 4 { // 3 content lines + summary line
			t.Errorf("Expected at most 4 lines for file_read, got: %d", len(lines))
		}
		
		// Check that lines are aligned (no leading spaces after first line processing)
		for i, line := range lines[:len(lines)-1] { // Skip summary line
			if i > 0 && strings.HasPrefix(line, "  ") {
				t.Errorf("Expected line %d to be aligned (no leading spaces), got: '%s'", i, line)
			}
		}
	})

	// 测试其他工具的一般处理
	t.Run("other tools general handling", func(t *testing.T) {
		content := "Result line 1\nResult line 2"
		result := executor.formatToolResultContent("other_tool", content)
		
		if result != content {
			t.Errorf("Expected content unchanged for short content, got: %s", result)
		}
	})

	// 测试空内容
	t.Run("empty content", func(t *testing.T) {
		result := executor.formatToolResultContent("any_tool", "")
		if result != "" {
			t.Errorf("Expected empty string for empty content, got: %s", result)
		}
	})

	// 测试前导空格清理
	t.Run("leading space cleanup", func(t *testing.T) {
		content := "     \n     Line with many leading spaces\nNormal line"
		result := executor.formatToolResultContent("bash", content)
		
		if strings.HasPrefix(result, "     ") {
			t.Errorf("Expected leading spaces to be cleaned up, got: %s", result)
		}
	})
}

func BenchmarkToolExecutor_ExecuteToolWithRecovery(b *testing.B) {
	executor := NewToolExecutor("BENCH")
	toolCall := &types.ReactToolCall{
		Name:   "bench_tool",
		CallID: "bench_call",
	}

	mockExecutor := func(ctx context.Context, toolName string, args map[string]interface{}, callID string) (*types.ReactToolResult, error) {
		return &types.ReactToolResult{
			Success:  true,
			Content:  "bench result",
			ToolName: toolName,
			CallID:   callID,
		}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.ExecuteToolWithRecovery(context.Background(), toolCall, mockExecutor, nil)
	}
}