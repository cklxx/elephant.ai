package ports

import "context"

// ToolExecutor executes a single tool call
type ToolExecutor interface {
	// Execute runs the tool with given arguments
	Execute(ctx context.Context, call ToolCall) (*ToolResult, error)

	// Definition returns the tool's schema for LLM
	Definition() ToolDefinition

	// Metadata returns tool metadata
	Metadata() ToolMetadata
}

// AgentCoordinator represents the main agent coordinator for subagent delegation
type AgentCoordinator interface {
	// ExecuteTask executes a task and returns the result
	ExecuteTask(ctx context.Context, task string, sessionID string) (*TaskResult, error)

	// ListSessions lists all available sessions
	ListSessions(ctx context.Context) ([]string, error)
}

// TaskResult represents the result of task execution
type TaskResult struct {
	Answer     string
	Iterations int
	TokensUsed int
	StopReason string
}

// StreamCallback is called during task execution to stream events
type StreamCallback func(event StreamEvent)

// StreamEvent represents different types of events during execution
type StreamEvent struct {
	Type    string // "tool_start", "tool_end", "thought", "error"
	Tool    string
	Args    map[string]any
	Result  string
	Error   error
	Content string
}

// ToolRegistry manages available tools
type ToolRegistry interface {
	// Register adds a tool to the registry
	Register(tool ToolExecutor) error

	// Get retrieves a tool by name
	Get(name string) (ToolExecutor, error)

	// List returns all available tools
	List() []ToolDefinition

	// Unregister removes a tool
	Unregister(name string) error
}

// ToolCall represents a request to execute a tool
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolResult is the execution result
type ToolResult struct {
	CallID   string         `json:"call_id"`
	Content  string         `json:"content"`
	Error    error          `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ToolDefinition describes a tool for the LLM
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  ParameterSchema `json:"parameters"`
}

// ToolMetadata contains tool information
type ToolMetadata struct {
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Category  string   `json:"category"`
	Tags      []string `json:"tags"`
	Dangerous bool     `json:"dangerous"`
}

// ParameterSchema defines tool parameters (JSON Schema format)
type ParameterSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property defines a single parameter
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Enum        []any  `json:"enum,omitempty"`
}
