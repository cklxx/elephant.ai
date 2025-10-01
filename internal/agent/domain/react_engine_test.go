package domain_test

import (
	"context"
	"fmt"
	"testing"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/ports/mocks"
)

func TestReactEngine_SolveTask_SingleIteration(t *testing.T) {
	// Arrange
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			return &ports.CompletionResponse{
				Content:    "The answer is 42. This is the final answer.",
				StopReason: "stop",
				Usage:      ports.TokenUsage{TotalTokens: 50},
			}, nil
		},
	}

	mockTools := &mocks.MockToolRegistry{}
	mockParser := &mocks.MockParser{}
	mockContext := &mocks.MockContextManager{}

	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	engine := domain.NewReactEngine(10)
	state := &domain.TaskState{}

	// Act
	result, err := engine.SolveTask(context.Background(), "What is the answer?", state, services)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if result.Iterations != 1 {
		t.Errorf("Expected 1 iteration, got %d", result.Iterations)
	}
	if result.StopReason != "final_answer" {
		t.Errorf("Expected stop reason 'final_answer', got '%s'", result.StopReason)
	}
}

func TestReactEngine_SolveTask_WithToolCall(t *testing.T) {
	// Arrange
	callCount := 0
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			callCount++
			if callCount == 1 {
				// First call: request tool
				return &ports.CompletionResponse{
					Content: "I need to read the file",
					ToolCalls: []ports.ToolCall{
						{ID: "call1", Name: "file_read", Arguments: map[string]any{"path": "test.txt"}},
					},
					StopReason: "tool_calls",
				}, nil
			}
			// Second call: final answer
			return &ports.CompletionResponse{
				Content:    "The file contains: mock content. Final answer.",
				StopReason: "stop",
			}, nil
		},
	}

	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (ports.ToolExecutor, error) {
			return &mocks.MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					return &ports.ToolResult{
						CallID:  call.ID,
						Content: "mock file content",
					}, nil
				},
			}, nil
		},
	}

	mockParser := &mocks.MockParser{}
	mockContext := &mocks.MockContextManager{}

	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	engine := domain.NewReactEngine(10)
	state := &domain.TaskState{}

	// Act
	result, err := engine.SolveTask(context.Background(), "Read test.txt", state, services)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Iterations != 2 {
		t.Errorf("Expected 2 iterations, got %d", result.Iterations)
	}
	if len(state.ToolResults) != 1 {
		t.Errorf("Expected 1 tool result, got %d", len(state.ToolResults))
	}
}

func TestReactEngine_SolveTask_MaxIterations(t *testing.T) {
	// Arrange
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			// Always request more tools (never finishes)
			return &ports.CompletionResponse{
				Content: "Let me check another thing",
				ToolCalls: []ports.ToolCall{
					{ID: "call", Name: "think", Arguments: map[string]any{"thought": "thinking"}},
				},
				StopReason: "tool_calls",
			}, nil
		},
	}

	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (ports.ToolExecutor, error) {
			return &mocks.MockToolExecutor{}, nil
		},
	}

	mockParser := &mocks.MockParser{}
	mockContext := &mocks.MockContextManager{}

	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	maxIter := 3
	engine := domain.NewReactEngine(maxIter)
	state := &domain.TaskState{}

	// Act
	result, err := engine.SolveTask(context.Background(), "Test task", state, services)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Iterations != maxIter {
		t.Errorf("Expected %d iterations, got %d", maxIter, result.Iterations)
	}
	if result.StopReason != "max_iterations" {
		t.Errorf("Expected stop reason 'max_iterations', got '%s'", result.StopReason)
	}
}

func TestReactEngine_SolveTask_ToolError(t *testing.T) {
	// Arrange
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			return &ports.CompletionResponse{
				Content: "Using tool",
				ToolCalls: []ports.ToolCall{
					{ID: "call1", Name: "nonexistent_tool", Arguments: map[string]any{}},
				},
			}, nil
		},
	}

	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (ports.ToolExecutor, error) {
			// Tool not found - return error
			return nil, fmt.Errorf("tool not found: %s", name)
		},
	}

	mockParser := &mocks.MockParser{}
	mockContext := &mocks.MockContextManager{}

	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	engine := domain.NewReactEngine(10)
	state := &domain.TaskState{}

	// Act
	result, err := engine.SolveTask(context.Background(), "Test", state, services)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	// Should stop due to tool error
	if result.StopReason != "completed" {
		t.Errorf("Expected stop reason 'completed', got '%s'", result.StopReason)
	}
}
