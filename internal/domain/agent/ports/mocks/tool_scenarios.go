package mocks

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
)

// ToolScenario represents a complete tool calling scenario for testing
type ToolScenario struct {
	Name        string
	Description string
	LLM         *MockLLMClient
	Registry    *MockToolRegistry
}

func scenarioToolRegistry(inner func(name string) (tools.ToolExecutor, error)) *MockToolRegistry {
	return &MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			switch name {
			case "plan":
				return newUIPlanExecutor(), nil
			case "clarify":
				return newUIClarifyExecutor(), nil
			default:
				if inner == nil {
					return nil, fmt.Errorf("tool not found: %s", name)
				}
				return inner(name)
			}
		},
	}
}

func newUIPlanExecutor() tools.ToolExecutor {
	return &MockToolExecutor{
		ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
			runID, _ := call.Arguments["run_id"].(string)
			runID = strings.TrimSpace(runID)
			if runID == "" {
				return &ports.ToolResult{CallID: call.ID, Content: "run_id cannot be empty", Error: fmt.Errorf("run_id cannot be empty")}, nil
			}

			goal, _ := call.Arguments["overall_goal_ui"].(string)
			goal = strings.TrimSpace(goal)
			if goal == "" {
				return &ports.ToolResult{CallID: call.ID, Content: "overall_goal_ui cannot be empty", Error: fmt.Errorf("invalid overall_goal_ui")}, nil
			}

			complexity, _ := call.Arguments["complexity"].(string)
			complexity = strings.ToLower(strings.TrimSpace(complexity))
			if complexity != "simple" && complexity != "complex" {
				return &ports.ToolResult{CallID: call.ID, Content: "complexity must be simple or complex", Error: fmt.Errorf("invalid complexity")}, nil
			}
			if complexity == "simple" && strings.ContainsAny(goal, "\r\n") {
				return &ports.ToolResult{CallID: call.ID, Content: "overall_goal_ui must be single-line when complexity=simple", Error: fmt.Errorf("invalid overall_goal_ui")}, nil
			}

			metadata := map[string]any{
				"run_id":          runID,
				"overall_goal_ui": goal,
				"complexity":      complexity,
			}
			if internalPlan, ok := call.Arguments["internal_plan"]; ok {
				metadata["internal_plan"] = internalPlan
			}

			return &ports.ToolResult{CallID: call.ID, Content: goal, Metadata: metadata}, nil
		},
	}
}

func newUIClarifyExecutor() tools.ToolExecutor {
	return &MockToolExecutor{
		ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
			runID, _ := call.Arguments["run_id"].(string)
			runID = strings.TrimSpace(runID)
			if runID == "" {
				return &ports.ToolResult{CallID: call.ID, Content: "run_id cannot be empty", Error: fmt.Errorf("run_id cannot be empty")}, nil
			}

			taskID, _ := call.Arguments["task_id"].(string)
			taskID = strings.TrimSpace(taskID)
			if taskID == "" {
				return &ports.ToolResult{CallID: call.ID, Content: "task_id cannot be empty", Error: fmt.Errorf("task_id cannot be empty")}, nil
			}

			taskGoalUI, _ := call.Arguments["task_goal_ui"].(string)
			taskGoalUI = strings.TrimSpace(taskGoalUI)
			if taskGoalUI == "" {
				return &ports.ToolResult{CallID: call.ID, Content: "task_goal_ui cannot be empty", Error: fmt.Errorf("invalid task_goal_ui")}, nil
			}

			metadata := map[string]any{
				"run_id":       runID,
				"task_id":      taskID,
				"task_goal_ui": taskGoalUI,
			}

			if raw, ok := call.Arguments["success_criteria"]; ok {
				if arr, ok := raw.([]any); ok {
					var criteria []string
					for _, item := range arr {
						if text, ok := item.(string); ok {
							if trimmed := strings.TrimSpace(text); trimmed != "" {
								criteria = append(criteria, trimmed)
							}
						}
					}
					if len(criteria) > 0 {
						metadata["success_criteria"] = criteria
					}
				}
			}

			needsUserInput := false
			if raw, ok := call.Arguments["needs_user_input"]; ok {
				if value, ok := raw.(bool); ok {
					needsUserInput = value
				}
			}
			questionToUser := ""
			if raw, ok := call.Arguments["question_to_user"]; ok {
				if value, ok := raw.(string); ok {
					questionToUser = strings.TrimSpace(value)
				}
			}
			if needsUserInput {
				if questionToUser == "" {
					return &ports.ToolResult{CallID: call.ID, Content: "question_to_user is required when needs_user_input=true", Error: fmt.Errorf("missing question_to_user")}, nil
				}
				metadata["needs_user_input"] = true
				metadata["question_to_user"] = questionToUser
			}

			content := taskGoalUI
			if needsUserInput {
				content = content + "\n" + questionToUser
			}

			return &ports.ToolResult{CallID: call.ID, Content: strings.TrimSpace(content), Metadata: metadata}, nil
		},
	}
}

func newScriptedLLMClient(responses ...ports.CompletionResponse) *MockLLMClient {
	if len(responses) == 0 {
		panic("newScriptedLLMClient requires at least one completion response")
	}

	callCount := 0
	return &MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			callCount++

			idx := callCount - 1
			if idx >= len(responses) {
				idx = len(responses) - 1
			}

			return cloneCompletionResponse(responses[idx]), nil
		},
	}
}

func cloneCompletionResponse(template ports.CompletionResponse) *ports.CompletionResponse {
	clone := template

	if template.ToolCalls != nil {
		clone.ToolCalls = make([]ports.ToolCall, len(template.ToolCalls))
		for i, call := range template.ToolCalls {
			clone.ToolCalls[i] = cloneToolCall(call)
		}
	}

	if template.Metadata != nil {
		clone.Metadata = cloneAnyMap(template.Metadata)
	}

	return &clone
}

func cloneToolCall(template ports.ToolCall) ports.ToolCall {
	clone := template
	if template.Arguments != nil {
		clone.Arguments = cloneAnyMap(template.Arguments)
	}
	return clone
}

func cloneAnyMap(template map[string]any) map[string]any {
	clone := make(map[string]any, len(template))
	for key, value := range template {
		clone[key] = value
	}
	return clone
}

const (
	scenarioRunID  = "test-run"
	scenarioTaskID = "task-1"
)

func newToolCallCompletion(content string, call ports.ToolCall, usage ports.TokenUsage) ports.CompletionResponse {
	return ports.CompletionResponse{
		Content:    content,
		ToolCalls:  []ports.ToolCall{call},
		StopReason: "tool_calls",
		Usage:      usage,
	}
}

func newPlanCompletion(
	content string,
	overallGoalUI string,
	complexity string,
	usage ports.TokenUsage,
	internalPlan map[string]any,
) ports.CompletionResponse {
	args := map[string]any{
		"run_id":          scenarioRunID,
		"overall_goal_ui": overallGoalUI,
		"complexity":      complexity,
	}
	if internalPlan != nil {
		args["internal_plan"] = internalPlan
	}

	return newToolCallCompletion(content, ports.ToolCall{
		ID:        "call_plan",
		Name:      "plan",
		Arguments: args,
	}, usage)
}

func newClarifyCompletion(
	content string,
	taskGoalUI string,
	usage ports.TokenUsage,
	extraArgs map[string]any,
) ports.CompletionResponse {
	args := map[string]any{
		"run_id":       scenarioRunID,
		"task_id":      scenarioTaskID,
		"task_goal_ui": taskGoalUI,
	}
	for key, value := range extraArgs {
		args[key] = value
	}

	return newToolCallCompletion(content, ports.ToolCall{
		ID:        "call_clarify",
		Name:      "clarify",
		Arguments: args,
	}, usage)
}

func newFinalCompletion(content string, usage ports.TokenUsage) ports.CompletionResponse {
	return ports.CompletionResponse{
		Content:    content,
		StopReason: "stop",
		Usage:      usage,
	}
}

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
			newClarifyCompletion(
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
						ID:   "call_clarify",
						Name: "clarify",
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

// NewSubagentDelegationScenario creates a scenario with subagent tool
func NewSubagentDelegationScenario() ToolScenario {
	return ToolScenario{
		Name:        "subagent_delegation",
		Description: "Agent delegates complex task to subagent",
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
						ID:   "call_clarify",
						Name: "clarify",
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
				Content: "This task is complex, I'll delegate it to a specialized subagent",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call_001",
						Name: "subagent",
						Arguments: map[string]any{
							"task":        "Analyze the codebase and suggest performance improvements",
							"description": "Code optimization analysis",
						},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 150, CompletionTokens: 60, TotalTokens: 210},
			},
			ports.CompletionResponse{
				Content:    "The subagent completed the analysis. Main suggestions: 1) Use sync.Pool for object reuse, 2) Add caching layer, 3) Optimize database queries.",
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
