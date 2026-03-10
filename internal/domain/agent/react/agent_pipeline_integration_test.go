//go:build integration

package react

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/mocks"
	tools "alex/internal/domain/agent/ports/tools"
)

// TestReAct_ToolLoop verifies the full ReAct loop: LLM returns two successive
// tool_calls then a final_answer. The engine must execute both tools and
// produce the correct final result.
func TestReAct_ToolLoop(t *testing.T) {
	var llmCall int32

	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			call := int(atomic.AddInt32(&llmCall, 1))
			switch call {
			case 1:
				// Iteration 1: plan tool call (required by engine flow)
				return &ports.CompletionResponse{
					Content: "Let me plan the approach.",
					ToolCalls: []ports.ToolCall{
						{
							ID:   "call_plan",
							Name: "plan",
							Arguments: map[string]any{
								"run_id":          "test-run",
								"overall_goal_ui": "Read two files and combine results.",
								"complexity":      "simple",
							},
						},
					},
					StopReason: "tool_calls",
					Usage:      ports.TokenUsage{TotalTokens: 30},
				}, nil
			case 2:
				// Iteration 2: read first file
				return &ports.CompletionResponse{
					Content: "Reading file A.",
					ToolCalls: []ports.ToolCall{
						{ID: "call_read_a", Name: "file_read", Arguments: map[string]any{"path": "a.txt"}},
					},
					StopReason: "tool_calls",
					Usage:      ports.TokenUsage{TotalTokens: 25},
				}, nil
			case 3:
				// Iteration 3: read second file
				return &ports.CompletionResponse{
					Content: "Reading file B.",
					ToolCalls: []ports.ToolCall{
						{ID: "call_read_b", Name: "file_read", Arguments: map[string]any{"path": "b.txt"}},
					},
					StopReason: "tool_calls",
					Usage:      ports.TokenUsage{TotalTokens: 25},
				}, nil
			default:
				// Iteration 4: final answer
				return &ports.CompletionResponse{
					Content:    "File A says hello, file B says world. Combined: hello world.",
					StopReason: "stop",
					Usage:      ports.TokenUsage{TotalTokens: 40},
				}, nil
			}
		},
	}

	var toolExecCount int32
	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			return &mocks.MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					atomic.AddInt32(&toolExecCount, 1)
					switch call.Name {
					case "plan":
						return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
					case "file_read":
						path, _ := call.Arguments["path"].(string)
						if path == "a.txt" {
							return &ports.ToolResult{CallID: call.ID, Content: "hello"}, nil
						}
						return &ports.ToolResult{CallID: call.ID, Content: "world"}, nil
					default:
						return &ports.ToolResult{CallID: call.ID, Content: "unknown tool"}, nil
					}
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

	result, err := engine.SolveTask(context.Background(), "Read a.txt and b.txt, combine results", state, services)
	if err != nil {
		t.Fatalf("SolveTask returned error: %v", err)
	}

	// 4 iterations: plan + read_a + read_b + final_answer
	if result.Iterations != 4 {
		t.Errorf("expected 4 iterations, got %d", result.Iterations)
	}
	if result.StopReason != "final_answer" {
		t.Errorf("expected stop reason 'final_answer', got %q", result.StopReason)
	}

	// 3 tool executions: plan + file_read(a) + file_read(b)
	execCount := int(atomic.LoadInt32(&toolExecCount))
	if execCount != 3 {
		t.Errorf("expected 3 tool executions, got %d", execCount)
	}

	// Verify tool results are recorded in state
	if len(state.ToolResults) != 3 {
		t.Errorf("expected 3 tool results in state, got %d", len(state.ToolResults))
	}

	// Final answer should reference combined content
	if result.Answer == "" {
		t.Error("expected non-empty final answer")
	}
}

// TestReAct_ToolRetry verifies that when a tool execution fails on the first
// attempt, the LLM observes the error and retries. The second attempt succeeds.
func TestReAct_ToolRetry(t *testing.T) {
	var llmCall int32

	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			call := int(atomic.AddInt32(&llmCall, 1))
			switch call {
			case 1:
				// First: plan
				return &ports.CompletionResponse{
					Content: "Planning.",
					ToolCalls: []ports.ToolCall{
						{ID: "call_plan", Name: "plan", Arguments: map[string]any{
							"run_id": "test", "overall_goal_ui": "test", "complexity": "simple",
						}},
					},
					StopReason: "tool_calls",
				}, nil
			case 2:
				// Second: attempt to read file (will fail)
				return &ports.CompletionResponse{
					Content: "Reading the file.",
					ToolCalls: []ports.ToolCall{
						{ID: "call_read_1", Name: "file_read", Arguments: map[string]any{"path": "/tmp/data.txt"}},
					},
					StopReason: "tool_calls",
				}, nil
			case 3:
				// Third: LLM sees the error, retries the same tool
				return &ports.CompletionResponse{
					Content: "The file read failed. Let me retry.",
					ToolCalls: []ports.ToolCall{
						{ID: "call_read_2", Name: "file_read", Arguments: map[string]any{"path": "/tmp/data.txt"}},
					},
					StopReason: "tool_calls",
				}, nil
			default:
				// Fourth: final answer with the data
				return &ports.CompletionResponse{
					Content:    "The file contains: success data",
					StopReason: "stop",
				}, nil
			}
		},
	}

	var toolAttempts int32
	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			return &mocks.MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					if call.Name == "plan" {
						return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
					}
					attempt := int(atomic.AddInt32(&toolAttempts, 1))
					if attempt == 1 {
						// First file_read attempt fails
						return &ports.ToolResult{
							CallID:  call.ID,
							Content: "error: permission denied",
							Error:   fmt.Errorf("permission denied"),
						}, nil
					}
					// Second file_read attempt succeeds
					return &ports.ToolResult{
						CallID:  call.ID,
						Content: "success data",
					}, nil
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

	result, err := engine.SolveTask(context.Background(), "Read /tmp/data.txt", state, services)
	if err != nil {
		t.Fatalf("SolveTask returned error: %v", err)
	}

	if result.StopReason != "final_answer" {
		t.Errorf("expected stop reason 'final_answer', got %q", result.StopReason)
	}

	// Verify the tool was attempted twice (once failed, once succeeded)
	attempts := int(atomic.LoadInt32(&toolAttempts))
	if attempts != 2 {
		t.Errorf("expected 2 file_read attempts, got %d", attempts)
	}

	// Verify the error was observed — check that state has tool results
	// including the failed one.
	var errorResults, successResults int
	for _, tr := range state.ToolResults {
		if tr.Error != nil {
			errorResults++
		} else if tr.Content == "success data" {
			successResults++
		}
	}
	if errorResults < 1 {
		t.Errorf("expected at least 1 error tool result, got %d", errorResults)
	}
	if successResults < 1 {
		t.Errorf("expected at least 1 success tool result, got %d", successResults)
	}
}

// fallbackLLMClient wraps a primary and fallback LLM client. If the primary
// returns an error, it transparently falls back to the secondary — mimicking
// the production pinnedRateLimitFallbackClient behavior.
type fallbackLLMClient struct {
	primary      *mocks.MockLLMClient
	fallback     *mocks.MockLLMClient
	useFallback  atomic.Bool
	switchedOver atomic.Bool
}

func (f *fallbackLLMClient) Model() string {
	if f.useFallback.Load() {
		return f.fallback.Model()
	}
	return f.primary.Model()
}

func (f *fallbackLLMClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if f.useFallback.Load() {
		return f.fallback.Complete(ctx, req)
	}
	resp, err := f.primary.Complete(ctx, req)
	if err != nil {
		f.useFallback.Store(true)
		f.switchedOver.Store(true)
		return f.fallback.Complete(ctx, req)
	}
	return resp, nil
}

func (f *fallbackLLMClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	if f.useFallback.Load() {
		return f.fallback.StreamComplete(ctx, req, callbacks)
	}
	resp, err := f.primary.StreamComplete(ctx, req, callbacks)
	if err != nil {
		f.useFallback.Store(true)
		f.switchedOver.Store(true)
		return f.fallback.StreamComplete(ctx, req, callbacks)
	}
	return resp, nil
}

// TestReAct_LLMFallback verifies that when the primary LLM returns a 5xx
// error, the engine transparently falls back to a secondary LLM and completes
// the task successfully.
func TestReAct_LLMFallback(t *testing.T) {
	// Primary LLM: always returns server error
	primaryLLM := &mocks.MockLLMClient{
		ModelFunc: func() string { return "primary-model" },
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			return nil, fmt.Errorf("500 Internal Server Error: service unavailable")
		},
	}

	// Fallback LLM: returns a valid response
	var fallbackCalls int32
	fallbackLLM := &mocks.MockLLMClient{
		ModelFunc: func() string { return "fallback-model" },
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			call := int(atomic.AddInt32(&fallbackCalls, 1))
			if call == 1 {
				// Plan call
				return &ports.CompletionResponse{
					Content: "Planning with fallback model.",
					ToolCalls: []ports.ToolCall{
						{ID: "call_plan", Name: "plan", Arguments: map[string]any{
							"run_id": "test", "overall_goal_ui": "answer question", "complexity": "simple",
						}},
					},
					StopReason: "tool_calls",
				}, nil
			}
			return &ports.CompletionResponse{
				Content:    "The answer from the fallback model is 42.",
				StopReason: "stop",
				Usage:      ports.TokenUsage{TotalTokens: 30},
			}, nil
		},
	}

	client := &fallbackLLMClient{primary: primaryLLM, fallback: fallbackLLM}

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
		LLM:          client,
		ToolExecutor: mockTools,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 5,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
	})
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "What is the answer?", state, services)
	if err != nil {
		t.Fatalf("SolveTask returned error: %v", err)
	}

	// Verify fallback was activated
	if !client.switchedOver.Load() {
		t.Error("expected LLM fallback to have been activated")
	}

	// Task should complete successfully via fallback
	if result.StopReason != "final_answer" {
		t.Errorf("expected stop reason 'final_answer', got %q", result.StopReason)
	}

	// Verify fallback model served the requests
	fbCalls := int(atomic.LoadInt32(&fallbackCalls))
	if fbCalls < 2 {
		t.Errorf("expected at least 2 fallback LLM calls (plan + final), got %d", fbCalls)
	}

	if result.Answer == "" {
		t.Error("expected non-empty final answer from fallback model")
	}
}
