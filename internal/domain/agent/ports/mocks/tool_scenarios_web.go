package mocks

import (
	"context"
	"fmt"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
)

// NewWebSearchScenario creates a scenario with web search (requires network mock)
func NewWebSearchScenario() ToolScenario {
	return ToolScenario{
		Name:        "web_search",
		Description: "Agent performs web search and web fetch",
		LLM: newScriptedLLMClient(
			newPlanCompletion(
				"检索并总结 Go 1.22 的新特性。",
				"搜索并阅读资料，总结 Go 1.22 的新特性。",
				"simple",
				ports.TokenUsage{PromptTokens: 80, CompletionTokens: 45, TotalTokens: 125},
				nil,
			),
			newToolCallCompletion(
				"Let me search for information about Go 1.22 features",
				ports.ToolCall{
					ID:   "call_001",
					Name: "web_search",
					Arguments: map[string]any{
						"query": "Go 1.22 new features release notes",
					},
				},
				ports.TokenUsage{PromptTokens: 80, CompletionTokens: 45, TotalTokens: 125},
			),
			newToolCallCompletion(
				"Let me fetch detailed information from the official blog",
				ports.ToolCall{
					ID:   "call_002",
					Name: "web_fetch",
					Arguments: map[string]any{
						"url": "https://go.dev/blog/go1.22",
					},
				},
				ports.TokenUsage{PromptTokens: 300, CompletionTokens: 50, TotalTokens: 350},
			),
			newFinalCompletion(
				"Go 1.22 introduces enhanced for-range loops, improved profile-guided optimization, and better workspace management.",
				ports.TokenUsage{PromptTokens: 600, CompletionTokens: 80, TotalTokens: 680},
			),
		),
		Registry: scenarioToolRegistry(func(name string) (tools.ToolExecutor, error) {
			return &MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					switch call.Name {
					case "web_search":
						return &ports.ToolResult{
							CallID: call.ID,
							Content: `Search results:
1. Go 1.22 Release Notes - https://go.dev/doc/go1.22
2. Go Blog: Go 1.22 is released - https://go.dev/blog/go1.22
3. What's new in Go 1.22 - Medium article`,
						}, nil
					case "web_fetch":
						return &ports.ToolResult{
							CallID: call.ID,
							Content: `Go 1.22 Release Notes
===
Major changes:
- Enhanced for-range loops with integer support
- Profile-guided optimization improvements
- Better workspace mode support
- Memory optimization in runtime`,
						}, nil
					default:
						return nil, fmt.Errorf("unknown tool: %s", call.Name)
					}
				},
			}, nil
		}),
	}
}

// NewCodeEditScenario creates a scenario where agent edits code files
func NewCodeEditScenario() ToolScenario {
	return ToolScenario{
		Name:        "code_edit",
		Description: "Agent reads, edits, and tests code changes",
		LLM: newScriptedLLMClient(
			ports.CompletionResponse{
				Content: "读取/修改代码并运行测试来验证。",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_plan",
						Name: "plan",
						Arguments: map[string]any{
							"run_id":          "test-run",
							"overall_goal_ui": "为 utils.go 增加错误处理并验证测试通过。",
							"complexity":      "complex",
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140},
			},
			ports.CompletionResponse{
				Content: "读取 utils.go、修改并运行 go test。",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_ask_user",
						Name: "ask_user",
						Arguments: map[string]any{
							"run_id":       "test-run",
							"task_id":      "task-1",
							"task_goal_ui": "读取 utils.go、修改并运行 go test。",
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 120, CompletionTokens: 35, TotalTokens: 155},
			},
			ports.CompletionResponse{
				Content: "Let me read the file first",
				ToolCalls: []ports.ToolCall{
					{ID: "call_001", Name: "file_read", Arguments: map[string]any{"path": "utils.go"}},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140},
			},
			ports.CompletionResponse{
				Content: "Now I'll add the missing error handling",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_002",
						Name: "file_edit",
						Arguments: map[string]any{
							"path":     "utils.go",
							"old_text": "return result",
							"new_text": "return result, nil",
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 300, CompletionTokens: 50, TotalTokens: 350},
			},
			ports.CompletionResponse{
				Content: "Let me run the tests to verify",
				ToolCalls: []ports.ToolCall{
					{ID: "call_003", Name: "bash", Arguments: map[string]any{"command": "go test ./..."}},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 500, CompletionTokens: 45, TotalTokens: 545},
			},
			ports.CompletionResponse{
				Content:    "Successfully added error handling and all tests pass.",
				StopReason: "stop",
				Usage:      ports.TokenUsage{PromptTokens: 700, CompletionTokens: 55, TotalTokens: 755},
			},
		),
		Registry: scenarioToolRegistry(func(name string) (tools.ToolExecutor, error) {
			return &MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					switch call.Name {
					case "file_read":
						return &ports.ToolResult{
							CallID:  call.ID,
							Content: "func Process() int {\n  result := compute()\n  return result\n}",
						}, nil
					case "file_edit":
						return &ports.ToolResult{
							CallID:  call.ID,
							Content: "✓ Successfully edited utils.go",
						}, nil
					case "bash":
						return &ports.ToolResult{
							CallID:  call.ID,
							Content: "PASS\nok  \tproject/pkg\t0.234s",
						}, nil
					default:
						return nil, fmt.Errorf("unknown tool: %s", call.Name)
					}
				},
			}, nil
		}),
	}
}
