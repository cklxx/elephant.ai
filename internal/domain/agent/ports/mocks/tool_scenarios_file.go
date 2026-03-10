package mocks

import (
	"context"
	"fmt"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
)

// NewFileReadScenario creates a scenario where agent reads a file
func NewFileReadScenario() ToolScenario {
	return ToolScenario{
		Name:        "file_read",
		Description: "Agent reads a file and provides answer based on content",
		LLM: newScriptedLLMClient(
			ports.CompletionResponse{
				Content: "通过读取配置来回答这个问题。",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_plan",
						Name: "plan",
						Arguments: map[string]any{
							"run_id":          "test-run",
							"overall_goal_ui": "回答问题，并在需要时读取配置文件。",
							"complexity":      "simple",
							"internal_plan": map[string]any{
								"overall_goal": "Answer question using config",
								"branches":     []any{},
							},
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
			},
			ports.CompletionResponse{
				Content: "我将查看配置文件内容。",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_001",
						Name: "file_read",
						Arguments: map[string]any{
							"path": "/config/settings.json",
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 200, CompletionTokens: 50, TotalTokens: 250},
			},
			ports.CompletionResponse{
				Content:    "Based on the configuration file, the API endpoint is https://api.example.com/v1",
				StopReason: "stop",
				Usage:      ports.TokenUsage{PromptTokens: 300, CompletionTokens: 80, TotalTokens: 380},
			},
		),
		Registry: scenarioToolRegistry(func(name string) (tools.ToolExecutor, error) {
			if name == "file_read" {
				return &MockToolExecutor{
					ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
						return &ports.ToolResult{
							CallID:  call.ID,
							Content: `{"api_endpoint": "https://api.example.com/v1", "timeout": 30}`,
						}, nil
					},
				}, nil
			}
			return nil, fmt.Errorf("tool not found: %s", name)
		}),
	}
}

// NewMultipleToolCallsScenario creates a scenario with multiple sequential tool calls
func NewMultipleToolCallsScenario() ToolScenario {
	return ToolScenario{
		Name:        "multiple_tools",
		Description: "Agent uses multiple tools sequentially (read file, search code, execute bash)",
		LLM: newScriptedLLMClient(
			ports.CompletionResponse{
				Content: "检查测试是否通过，并在需要时读取/搜索代码与运行命令。",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_plan",
						Name: "plan",
						Arguments: map[string]any{
							"run_id":          "test-run",
							"overall_goal_ui": "检查测试是否通过，并定位相关代码位置。",
							"complexity":      "complex",
							"internal_plan": map[string]any{
								"overall_goal": "Check tests and locate init",
								"branches":     []any{},
							},
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140},
			},
			newAskUserCompletion(
				"读取 main.go、搜索 init，并运行测试。",
				"读取 main.go、搜索 init，并运行测试。",
				ports.TokenUsage{PromptTokens: 150, CompletionTokens: 45, TotalTokens: 195},
				map[string]any{
					"success_criteria": []any{
						"拿到 init 定义位置",
						"确认测试运行结果",
					},
				},
			),
			// First action: read file
			ports.CompletionResponse{
				Content: "Let me first read the main.go file",
				ToolCalls: []ports.ToolCall{
					{ID: "call_001", Name: "file_read", Arguments: map[string]any{"path": "main.go"}},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140},
			},
			// Second: search for function
			ports.CompletionResponse{
				Content: "Now let me search for the init function",
				ToolCalls: []ports.ToolCall{
					{ID: "call_002", Name: "ripgrep", Arguments: map[string]any{"pattern": "func init"}},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 300, CompletionTokens: 45, TotalTokens: 345},
			},
			// Third: run tests
			ports.CompletionResponse{
				Content: "Let me run the tests",
				ToolCalls: []ports.ToolCall{
					{ID: "call_003", Name: "bash", Arguments: map[string]any{"command": "go test -v"}},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 500, CompletionTokens: 50, TotalTokens: 550},
			},
			// Final answer
			ports.CompletionResponse{
				Content:    "All tests passed successfully. The init function is properly implemented.",
				StopReason: "stop",
				Usage:      ports.TokenUsage{PromptTokens: 800, CompletionTokens: 60, TotalTokens: 860},
			},
		),
		Registry: scenarioToolRegistry(func(name string) (tools.ToolExecutor, error) {
			return &MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					switch call.Name {
					case "file_read":
						return &ports.ToolResult{CallID: call.ID, Content: "package main\n\nfunc main() {...}"}, nil
					case "ripgrep":
						return &ports.ToolResult{CallID: call.ID, Content: "init.go:10:func init() {"}, nil
					case "bash":
						return &ports.ToolResult{CallID: call.ID, Content: "PASS\nok  \tpackage\t0.123s"}, nil
					default:
						return nil, fmt.Errorf("unknown tool: %s", call.Name)
					}
				},
			}, nil
		}),
	}
}

// NewParallelToolCallsScenario creates a scenario with parallel tool calls
func NewParallelToolCallsScenario() ToolScenario {
	return ToolScenario{
		Name:        "parallel_tools",
		Description: "Agent uses multiple tools in parallel (read multiple files)",
		LLM: newScriptedLLMClient(
			ports.CompletionResponse{
				Content: "比较多个配置文件的差异。",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_plan",
						Name: "plan",
						Arguments: map[string]any{
							"run_id":          "test-run",
							"overall_goal_ui": "读取并对比多个配置文件。",
							"complexity":      "simple",
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 150, CompletionTokens: 70, TotalTokens: 220},
			},
			ports.CompletionResponse{
				Content: "Reading dev config",
				ToolCalls: []ports.ToolCall{
					{ID: "call_001", Name: "file_read", Arguments: map[string]any{"path": "config/dev.json"}},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 200, CompletionTokens: 40, TotalTokens: 240},
			},
			ports.CompletionResponse{
				Content: "Reading prod config",
				ToolCalls: []ports.ToolCall{
					{ID: "call_002", Name: "file_read", Arguments: map[string]any{"path": "config/prod.json"}},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 250, CompletionTokens: 40, TotalTokens: 290},
			},
			ports.CompletionResponse{
				Content: "Reading test config",
				ToolCalls: []ports.ToolCall{
					{ID: "call_003", Name: "file_read", Arguments: map[string]any{"path": "config/test.json"}},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 300, CompletionTokens: 40, TotalTokens: 340},
			},
			ports.CompletionResponse{
				Content:    "The configurations differ in API endpoints: dev uses localhost, test uses staging.example.com, prod uses api.example.com",
				StopReason: "stop",
				Usage:      ports.TokenUsage{PromptTokens: 500, CompletionTokens: 90, TotalTokens: 590},
			},
		),
		Registry: scenarioToolRegistry(func(name string) (tools.ToolExecutor, error) {
			return &MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					path := call.Arguments["path"].(string)
					var content string
					switch path {
					case "config/dev.json":
						content = `{"endpoint": "http://localhost:8080"}`
					case "config/prod.json":
						content = `{"endpoint": "https://api.example.com"}`
					case "config/test.json":
						content = `{"endpoint": "https://staging.example.com"}`
					}
					return &ports.ToolResult{CallID: call.ID, Content: content}, nil
				},
			}, nil
		}),
	}
}
