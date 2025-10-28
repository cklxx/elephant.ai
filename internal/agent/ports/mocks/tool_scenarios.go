package mocks

import (
	"alex/internal/agent/ports"
	"context"
	"fmt"
)

// ToolScenario represents a complete tool calling scenario for testing
type ToolScenario struct {
	Name        string
	Description string
	LLM         *MockLLMClient
	Registry    *MockToolRegistry
}

// NewFileReadScenario creates a scenario where agent reads a file
func NewFileReadScenario() ToolScenario {
	callCount := 0
	return ToolScenario{
		Name:        "file_read",
		Description: "Agent reads a file and provides answer based on content",
		LLM: &MockLLMClient{
			CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
				callCount++
				if callCount == 1 {
					return &ports.CompletionResponse{
						Content: "I need to read the configuration file to answer the question.",
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
						Usage:      ports.TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
					}, nil
				}
				return &ports.CompletionResponse{
					Content:    "Based on the configuration file, the API endpoint is https://api.example.com/v1",
					StopReason: "stop",
					Usage:      ports.TokenUsage{PromptTokens: 200, CompletionTokens: 80, TotalTokens: 280},
				}, nil
			},
		},
		Registry: &MockToolRegistry{
			GetFunc: func(name string) (ports.ToolExecutor, error) {
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
			},
		},
	}
}

// NewMultipleToolCallsScenario creates a scenario with multiple sequential tool calls
func NewMultipleToolCallsScenario() ToolScenario {
	callCount := 0
	return ToolScenario{
		Name:        "multiple_tools",
		Description: "Agent uses multiple tools sequentially (read file, search code, execute bash)",
		LLM: &MockLLMClient{
			CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
				callCount++
				switch callCount {
				case 1:
					// First: read file
					return &ports.CompletionResponse{
						Content: "Let me first read the main.go file",
						ToolCalls: []ports.ToolCall{
							{ID: "call_001", Name: "file_read", Arguments: map[string]any{"path": "main.go"}},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140},
					}, nil
				case 2:
					// Second: search for function
					return &ports.CompletionResponse{
						Content: "Now let me search for the init function",
						ToolCalls: []ports.ToolCall{
							{ID: "call_002", Name: "ripgrep", Arguments: map[string]any{"pattern": "func init"}},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 300, CompletionTokens: 45, TotalTokens: 345},
					}, nil
				case 3:
					// Third: run tests
					return &ports.CompletionResponse{
						Content: "Let me run the tests",
						ToolCalls: []ports.ToolCall{
							{ID: "call_003", Name: "bash", Arguments: map[string]any{"command": "go test -v"}},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 500, CompletionTokens: 50, TotalTokens: 550},
					}, nil
				default:
					// Final answer
					return &ports.CompletionResponse{
						Content:    "All tests passed successfully. The init function is properly implemented.",
						StopReason: "stop",
						Usage:      ports.TokenUsage{PromptTokens: 800, CompletionTokens: 60, TotalTokens: 860},
					}, nil
				}
			},
		},
		Registry: &MockToolRegistry{
			GetFunc: func(name string) (ports.ToolExecutor, error) {
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
			},
		},
	}
}

// NewParallelToolCallsScenario creates a scenario with parallel tool calls
func NewParallelToolCallsScenario() ToolScenario {
	callCount := 0
	return ToolScenario{
		Name:        "parallel_tools",
		Description: "Agent uses multiple tools in parallel (read multiple files)",
		LLM: &MockLLMClient{
			CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
				callCount++
				if callCount == 1 {
					return &ports.CompletionResponse{
						Content: "I need to read multiple configuration files to compare them",
						ToolCalls: []ports.ToolCall{
							{ID: "call_001", Name: "file_read", Arguments: map[string]any{"path": "config/dev.json"}},
							{ID: "call_002", Name: "file_read", Arguments: map[string]any{"path": "config/prod.json"}},
							{ID: "call_003", Name: "file_read", Arguments: map[string]any{"path": "config/test.json"}},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 150, CompletionTokens: 70, TotalTokens: 220},
					}, nil
				}
				return &ports.CompletionResponse{
					Content:    "The configurations differ in API endpoints: dev uses localhost, test uses staging.example.com, prod uses api.example.com",
					StopReason: "stop",
					Usage:      ports.TokenUsage{PromptTokens: 500, CompletionTokens: 90, TotalTokens: 590},
				}, nil
			},
		},
		Registry: &MockToolRegistry{
			GetFunc: func(name string) (ports.ToolExecutor, error) {
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
			},
		},
	}
}

// NewWebSearchScenario creates a scenario with web search (requires network mock)
func NewWebSearchScenario() ToolScenario {
	callCount := 0
	return ToolScenario{
		Name:        "web_search",
		Description: "Agent performs web search and web fetch",
		LLM: &MockLLMClient{
			CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
				callCount++
				if callCount == 1 {
					return &ports.CompletionResponse{
						Content: "Let me search for information about Go 1.22 features",
						ToolCalls: []ports.ToolCall{
							{
								ID:   "call_001",
								Name: "web_search",
								Arguments: map[string]any{
									"query": "Go 1.22 new features release notes",
								},
							},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 80, CompletionTokens: 45, TotalTokens: 125},
					}, nil
				}
				if callCount == 2 {
					return &ports.CompletionResponse{
						Content: "Let me fetch detailed information from the official blog",
						ToolCalls: []ports.ToolCall{
							{
								ID:   "call_002",
								Name: "web_fetch",
								Arguments: map[string]any{
									"url": "https://go.dev/blog/go1.22",
								},
							},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 300, CompletionTokens: 50, TotalTokens: 350},
					}, nil
				}
				return &ports.CompletionResponse{
					Content:    "Go 1.22 introduces enhanced for-range loops, improved profile-guided optimization, and better workspace management.",
					StopReason: "stop",
					Usage:      ports.TokenUsage{PromptTokens: 600, CompletionTokens: 80, TotalTokens: 680},
				}, nil
			},
		},
		Registry: &MockToolRegistry{
			GetFunc: func(name string) (ports.ToolExecutor, error) {
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
			},
		},
	}
}

// NewCodeEditScenario creates a scenario where agent edits code files
func NewCodeEditScenario() ToolScenario {
	callCount := 0
	return ToolScenario{
		Name:        "code_edit",
		Description: "Agent reads, edits, and tests code changes",
		LLM: &MockLLMClient{
			CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
				callCount++
				switch callCount {
				case 1:
					return &ports.CompletionResponse{
						Content: "Let me read the file first",
						ToolCalls: []ports.ToolCall{
							{ID: "call_001", Name: "file_read", Arguments: map[string]any{"path": "utils.go"}},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140},
					}, nil
				case 2:
					return &ports.CompletionResponse{
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
					}, nil
				case 3:
					return &ports.CompletionResponse{
						Content: "Let me run the tests to verify",
						ToolCalls: []ports.ToolCall{
							{ID: "call_003", Name: "bash", Arguments: map[string]any{"command": "go test ./..."}},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 500, CompletionTokens: 45, TotalTokens: 545},
					}, nil
				default:
					return &ports.CompletionResponse{
						Content:    "Successfully added error handling and all tests pass.",
						StopReason: "stop",
						Usage:      ports.TokenUsage{PromptTokens: 700, CompletionTokens: 55, TotalTokens: 755},
					}, nil
				}
			},
		},
		Registry: &MockToolRegistry{
			GetFunc: func(name string) (ports.ToolExecutor, error) {
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
			},
		},
	}
}

// NewToolErrorScenario creates a scenario where tool execution fails
func NewToolErrorScenario() ToolScenario {
	callCount := 0
	return ToolScenario{
		Name:        "tool_error",
		Description: "Agent handles tool execution errors gracefully",
		LLM: &MockLLMClient{
			CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
				callCount++
				if callCount == 1 {
					return &ports.CompletionResponse{
						Content: "Let me try to read the file",
						ToolCalls: []ports.ToolCall{
							{ID: "call_001", Name: "file_read", Arguments: map[string]any{"path": "/nonexistent/file.txt"}},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140},
					}, nil
				}
				// After error, agent should try alternative approach
				if callCount == 2 {
					return &ports.CompletionResponse{
						Content: "The file doesn't exist. Let me search for similar files",
						ToolCalls: []ports.ToolCall{
							{ID: "call_002", Name: "find", Arguments: map[string]any{"pattern": "*.txt"}},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 250, CompletionTokens: 50, TotalTokens: 300},
					}, nil
				}
				return &ports.CompletionResponse{
					Content:    "The file you requested doesn't exist, but I found these similar files: data.txt, config.txt",
					StopReason: "stop",
					Usage:      ports.TokenUsage{PromptTokens: 400, CompletionTokens: 60, TotalTokens: 460},
				}, nil
			},
		},
		Registry: &MockToolRegistry{
			GetFunc: func(name string) (ports.ToolExecutor, error) {
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
			},
		},
	}
}

// NewTodoManagementScenario creates a scenario with todo operations
func NewTodoManagementScenario() ToolScenario {
	callCount := 0
	todos := []string{}

	return ToolScenario{
		Name:        "todo_management",
		Description: "Agent reads and updates todo list",
		LLM: &MockLLMClient{
			CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
				callCount++
				switch callCount {
				case 1:
					return &ports.CompletionResponse{
						Content: "Let me check the current todo list",
						ToolCalls: []ports.ToolCall{
							{ID: "call_001", Name: "todo_read", Arguments: map[string]any{}},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 80, CompletionTokens: 35, TotalTokens: 115},
					}, nil
				case 2:
					return &ports.CompletionResponse{
						Content: "I'll add the new tasks to the list",
						ToolCalls: []ports.ToolCall{
							{
								ID:   "call_002",
								Name: "todo_update",
								Arguments: map[string]any{
									"action": "add",
									"items":  []string{"Write tests", "Update docs", "Fix bug #123"},
								},
							},
						},
						StopReason: "tool_calls",
						Usage:      ports.TokenUsage{PromptTokens: 200, CompletionTokens: 45, TotalTokens: 245},
					}, nil
				case 3:
					return &ports.CompletionResponse{
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
					}, nil
				default:
					return &ports.CompletionResponse{
						Content:    "Successfully updated todo list: 1 completed, 2 pending tasks.",
						StopReason: "stop",
						Usage:      ports.TokenUsage{PromptTokens: 400, CompletionTokens: 50, TotalTokens: 450},
					}, nil
				}
			},
		},
		Registry: &MockToolRegistry{
			GetFunc: func(name string) (ports.ToolExecutor, error) {
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
								items := call.Arguments["items"].([]string)
								todos = append(todos, items...)
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
			},
		},
	}
}

// NewSubagentDelegationScenario creates a scenario with subagent tool
func NewSubagentDelegationScenario() ToolScenario {
	callCount := 0
	return ToolScenario{
		Name:        "subagent_delegation",
		Description: "Agent delegates complex task to subagent",
		LLM: &MockLLMClient{
			CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
				callCount++
				if callCount == 1 {
					return &ports.CompletionResponse{
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
					}, nil
				}
				return &ports.CompletionResponse{
					Content:    "The subagent completed the analysis. Main suggestions: 1) Use sync.Pool for object reuse, 2) Add caching layer, 3) Optimize database queries.",
					StopReason: "stop",
					Usage:      ports.TokenUsage{PromptTokens: 500, CompletionTokens: 90, TotalTokens: 590},
				}, nil
			},
		},
		Registry: &MockToolRegistry{
			GetFunc: func(name string) (ports.ToolExecutor, error) {
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
			},
		},
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
