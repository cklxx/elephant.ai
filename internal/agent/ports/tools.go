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

// ExecutionEnvironment contains all dependencies needed to run a task
type ExecutionEnvironment struct {
	State        any // *domain.TaskState
	Services     any // domain.Services
	Session      *Session
	SystemPrompt string
	TaskAnalysis *TaskAnalysis // Pre-analysis result (action name, goal, approach)
}

// TaskAnalysis contains structured task pre-analysis result
type TaskAnalysis struct {
	ActionName string
	Goal       string
	Approach   string
}

// AgentCoordinator represents the main agent coordinator for subagent delegation
type AgentCoordinator interface {
	// ExecuteTask executes a task with optional event listener and returns the result
	ExecuteTask(ctx context.Context, task string, sessionID string, listener any) (*TaskResult, error)

	// PrepareExecution prepares the execution environment (session, state, services) without running the task
	PrepareExecution(ctx context.Context, task string, sessionID string) (*ExecutionEnvironment, error)

	// SaveSessionAfterExecution saves session state after task completion
	SaveSessionAfterExecution(ctx context.Context, session *Session, result any) error

	// ListSessions lists all available sessions
	ListSessions(ctx context.Context) ([]string, error)

	// GetConfig returns the coordinator configuration
	GetConfig() any // Returns app.Config

	// GetLLMClient returns an LLM client
	GetLLMClient() (any, error) // Returns LLMClient

	// GetToolRegistry returns the tool registry (without subagent for nested calls)
	GetToolRegistryWithoutSubagent() ToolRegistry

	// GetParser returns the function call parser
	GetParser() any // Returns FunctionCallParser

	// GetContextManager returns the context manager
	GetContextManager() any // Returns ContextManager

	// GetSystemPrompt returns the system prompt
	GetSystemPrompt() string
}

// TaskResult represents the result of task execution
type TaskResult struct {
	Answer     string
	Iterations int
	TokensUsed int
	StopReason string
	SessionID  string // The session ID used for this task
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
