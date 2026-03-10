package mocks

import (
	"context"
	"fmt"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
)

// NewToolErrorScenario creates a scenario where tool execution fails
func NewToolErrorScenario() ToolScenario {
	llmScript := []ports.CompletionResponse{
		newPlanCompletion(
			"尝试读取文件，如果失败会寻找替代方案。",
			"读取指定文件，并在失败时寻找替代文件。",
			"simple",
			ports.TokenUsage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140},
			nil,
		),
		newToolCallCompletion(
			"Let me try to read the file",
			ports.ToolCall{
				ID:   "call_001",
				Name: "file_read",
				Arguments: map[string]any{
					"path": "/nonexistent/file.txt",
				},
			},
			ports.TokenUsage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140},
		),
		// After error, agent should try alternative approach
		newToolCallCompletion(
			"The file doesn't exist. Let me search for similar files",
			ports.ToolCall{
				ID:   "call_002",
				Name: "find",
				Arguments: map[string]any{
					"pattern": "*.txt",
				},
			},
			ports.TokenUsage{PromptTokens: 250, CompletionTokens: 50, TotalTokens: 300},
		),
		newFinalCompletion(
			"The file you requested doesn't exist, but I found these similar files: data.txt, config.txt",
			ports.TokenUsage{PromptTokens: 400, CompletionTokens: 60, TotalTokens: 460},
		),
	}

	return ToolScenario{
		Name:        "tool_error",
		Description: "Agent handles tool execution errors gracefully",
		LLM:         newScriptedLLMClient(llmScript...),
		Registry: scenarioToolRegistry(func(name string) (tools.ToolExecutor, error) {
			return &MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					if call.Name == "file_read" {
						return &ports.ToolResult{
							CallID:  call.ID,
							Content: "",
							Error:   fmt.Errorf("file not found: /nonexistent/file.txt"),
						}, nil
					}
					// find succeeds
					return &ports.ToolResult{
						CallID:  call.ID,
						Content: "./data.txt\n./config.txt",
					}, nil
				},
			}, nil
		}),
	}
}

// NewTodoManagementScenario creates a scenario with todo operations
func NewTodoManagementScenario() ToolScenario {
	todos := []string{}

	return ToolScenario{
		Name:        "todo_management",
		Description: "Agent reads and updates todo list",
		LLM: newScriptedLLMClient(
			ports.CompletionResponse{
				Content: "读取并更新 todo 列表。",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_plan",
						Name: "plan",
						Arguments: map[string]any{
							"run_id":          "test-run",
							"overall_goal_ui": "维护 todo 列表：添加任务并标记完成。",
							"complexity":      "simple",
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 80, CompletionTokens: 35, TotalTokens: 115},
			},
			ports.CompletionResponse{
				Content: "Let me check the current todo list",
				ToolCalls: []ports.ToolCall{
					{ID: "call_001", Name: "todo_read", Arguments: map[string]any{}},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 80, CompletionTokens: 35, TotalTokens: 115},
			},
			ports.CompletionResponse{
				Content: "I'll add the new tasks to the list",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_002",
						Name: "todo_update",
						Arguments: map[string]any{
							"action": "add",
							"items":  []any{"Write tests", "Update docs", "Fix bug #123"},
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 200, CompletionTokens: 45, TotalTokens: 245},
			},
			ports.CompletionResponse{
				Content: "Let me mark the first task as complete",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_003",
						Name: "todo_update",
						Arguments: map[string]any{
							"action": "complete",
							"index":  0,
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 300, CompletionTokens: 40, TotalTokens: 340},
			},
			ports.CompletionResponse{
				Content:    "Successfully updated todo list: 1 completed, 2 pending tasks.",
				StopReason: "stop",
				Usage:      ports.TokenUsage{PromptTokens: 400, CompletionTokens: 50, TotalTokens: 450},
			},
		),
		Registry: scenarioToolRegistry(func(name string) (tools.ToolExecutor, error) {
			return &MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					switch call.Name {
					case "todo_read":
						result := "Current todos:\n"
						for i, todo := range todos {
							result += fmt.Sprintf("%d. %s\n", i+1, todo)
						}
						if len(todos) == 0 {
							result = "No todos found"
						}
						return &ports.ToolResult{CallID: call.ID, Content: result}, nil
					case "todo_update":
						action := call.Arguments["action"].(string)
						switch action {
						case "add":
							items := call.Arguments["items"].([]any)
							for _, item := range items {
								if text, ok := item.(string); ok {
									todos = append(todos, text)
								}
							}
							return &ports.ToolResult{
								CallID:  call.ID,
								Content: fmt.Sprintf("Added %d tasks", len(items)),
							}, nil
						case "complete":
							idx := call.Arguments["index"].(int)
							if idx >= 0 && idx < len(todos) {
								completed := todos[idx]
								todos = append(todos[:idx], todos[idx+1:]...)
								return &ports.ToolResult{
									CallID:  call.ID,
									Content: fmt.Sprintf("Completed: %s", completed),
								}, nil
							}
						}
						return &ports.ToolResult{CallID: call.ID, Content: "OK"}, nil
					default:
						return nil, fmt.Errorf("unknown tool: %s", call.Name)
					}
				},
			}, nil
		}),
	}
}

// NewSubagentDelegationScenario creates a scenario where the agent shells out
// to the team CLI for deeper analysis.
func NewSubagentDelegationScenario() ToolScenario {
	return ToolScenario{
		Name:        "team_cli_delegation",
		Description: "Agent delegates complex task via the team CLI",
		LLM: newScriptedLLMClient(
			ports.CompletionResponse{
				Content: "委托子代理进行分析，并汇总建议。",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_plan",
						Name: "plan",
						Arguments: map[string]any{
							"run_id":          "test-run",
							"overall_goal_ui": "通过子代理分析代码库性能问题并给出建议。",
							"complexity":      "complex",
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 150, CompletionTokens: 60, TotalTokens: 210},
			},
			ports.CompletionResponse{
				Content: "让子代理分析代码库并提出性能优化建议。",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_ask_user",
						Name: "ask_user",
						Arguments: map[string]any{
							"run_id":       "test-run",
							"task_id":      "task-1",
							"task_goal_ui": "让子代理分析代码库并提出性能优化建议。",
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 180, CompletionTokens: 45, TotalTokens: 225},
			},
			ports.CompletionResponse{
				Content: "This task is complex, I'll delegate it via the team CLI",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_001",
						Name: "bash",
						Arguments: map[string]any{
							"command": "alex team run --file tasks.yaml --wait",
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 150, CompletionTokens: 60, TotalTokens: 210},
			},
			ports.CompletionResponse{
				Content:    "The team CLI delegation completed the analysis. Main suggestions: 1) Use sync.Pool for object reuse, 2) Add caching layer, 3) Optimize database queries.",
				StopReason: "stop",
				Usage:      ports.TokenUsage{PromptTokens: 500, CompletionTokens: 90, TotalTokens: 590},
			},
		),
		Registry: scenarioToolRegistry(func(name string) (tools.ToolExecutor, error) {
			return &MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					return &ports.ToolResult{
						CallID: call.ID,
						Content: `Subagent analysis complete:

Performance Issues Found:
1. Excessive memory allocations in hot path
2. Missing database connection pooling
3. No caching for frequently accessed data

Recommendations:
1. Implement sync.Pool for temporary objects
2. Add connection pooling with max 50 connections
3. Implement Redis caching layer for read-heavy operations

Estimated improvement: 3-5x performance increase`,
					}, nil
				},
			}, nil
		}),
	}
}

// GetAllScenarios returns all available test scenarios
func GetAllScenarios() []ToolScenario {
	return []ToolScenario{
		NewFileReadScenario(),
		NewMultipleToolCallsScenario(),
		NewParallelToolCallsScenario(),
		NewWebSearchScenario(),
		NewCodeEditScenario(),
		NewToolErrorScenario(),
		NewTodoManagementScenario(),
		NewSubagentDelegationScenario(),
	}
}
