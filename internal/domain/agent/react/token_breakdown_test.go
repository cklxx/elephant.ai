package react

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/domain/agent/ports/mocks"
	tools "alex/internal/domain/agent/ports/tools"
)

func TestTokenBreakdown_SingleIteration(t *testing.T) {
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			return &ports.CompletionResponse{
				Content:    "Final answer.",
				StopReason: "stop",
				Usage: ports.TokenUsage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
			}, nil
		},
	}

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "question", state, services)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tb := result.TokenBreakdown
	if tb.LLMCalls != 1 {
		t.Errorf("expected 1 LLM call, got %d", tb.LLMCalls)
	}
	if tb.ThinkPromptTokens != 100 {
		t.Errorf("expected ThinkPromptTokens=100, got %d", tb.ThinkPromptTokens)
	}
	if tb.ThinkCompletionTokens != 50 {
		t.Errorf("expected ThinkCompletionTokens=50, got %d", tb.ThinkCompletionTokens)
	}
	if tb.TotalPromptTokens != 100 {
		t.Errorf("expected TotalPromptTokens=100, got %d", tb.TotalPromptTokens)
	}
	if tb.TotalCompletionTokens != 50 {
		t.Errorf("expected TotalCompletionTokens=50, got %d", tb.TotalCompletionTokens)
	}
	if tb.TotalTokens != 150 {
		t.Errorf("expected TotalTokens=150, got %d", tb.TotalTokens)
	}
}

func TestTokenBreakdown_MultiIterationAccumulation(t *testing.T) {
	callCount := 0
	usages := []ports.TokenUsage{
		{PromptTokens: 200, CompletionTokens: 80, TotalTokens: 280},  // iter 1: plan
		{PromptTokens: 350, CompletionTokens: 120, TotalTokens: 470}, // iter 2: tool call
		{PromptTokens: 500, CompletionTokens: 60, TotalTokens: 560},  // iter 3: final answer
	}

	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			idx := callCount
			callCount++
			switch idx {
			case 0:
				return &ports.CompletionResponse{
					Content: "Let me plan.",
					ToolCalls: []ports.ToolCall{
						{ID: "c1", Name: "plan", Arguments: map[string]any{"overall_goal_ui": "test", "complexity": "simple"}},
					},
					StopReason: "tool_calls",
					Usage:      usages[0],
				}, nil
			case 1:
				return &ports.CompletionResponse{
					Content: "Reading file.",
					ToolCalls: []ports.ToolCall{
						{ID: "c2", Name: "file_read", Arguments: map[string]any{"path": "x.txt"}},
					},
					StopReason: "tool_calls",
					Usage:      usages[1],
				}, nil
			default:
				return &ports.CompletionResponse{
					Content:    "Done.",
					StopReason: "stop",
					Usage:      usages[2],
				}, nil
			}
		},
	}

	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			return &mocks.MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
				},
			}, nil
		},
	}

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "do something", state, services)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Iterations != 3 {
		t.Fatalf("expected 3 iterations, got %d", result.Iterations)
	}

	tb := result.TokenBreakdown

	// Should accumulate all 3 LLM calls.
	if tb.LLMCalls != 3 {
		t.Errorf("expected 3 LLM calls, got %d", tb.LLMCalls)
	}

	wantPrompt := 200 + 350 + 500
	wantCompletion := 80 + 120 + 60
	wantTotal := 280 + 470 + 560

	if tb.ThinkPromptTokens != wantPrompt {
		t.Errorf("ThinkPromptTokens: want %d, got %d", wantPrompt, tb.ThinkPromptTokens)
	}
	if tb.ThinkCompletionTokens != wantCompletion {
		t.Errorf("ThinkCompletionTokens: want %d, got %d", wantCompletion, tb.ThinkCompletionTokens)
	}
	if tb.TotalPromptTokens != wantPrompt {
		t.Errorf("TotalPromptTokens: want %d, got %d", wantPrompt, tb.TotalPromptTokens)
	}
	if tb.TotalCompletionTokens != wantCompletion {
		t.Errorf("TotalCompletionTokens: want %d, got %d", wantCompletion, tb.TotalCompletionTokens)
	}
	if tb.TotalTokens != wantTotal {
		t.Errorf("TotalTokens: want %d, got %d", wantTotal, tb.TotalTokens)
	}

	// Also verify state has the same breakdown.
	stb := state.TokenBreakdown
	if stb.TotalTokens != wantTotal {
		t.Errorf("state.TokenBreakdown.TotalTokens: want %d, got %d", wantTotal, stb.TotalTokens)
	}
}

func TestTokenBreakdown_ZeroUsageWhenLLMReportsNone(t *testing.T) {
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			return &ports.CompletionResponse{
				Content:    "Answer.",
				StopReason: "stop",
				// Usage left as zero value
			}, nil
		},
	}

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(1)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "q", state, services)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tb := result.TokenBreakdown
	if tb.LLMCalls != 1 {
		t.Errorf("expected 1 LLM call even with zero usage, got %d", tb.LLMCalls)
	}
	if tb.TotalTokens != 0 {
		t.Errorf("expected TotalTokens=0 when LLM reports nothing, got %d", tb.TotalTokens)
	}
}
