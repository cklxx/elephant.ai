package tools

import (
	"context"

	core "alex/internal/agent/ports"
)

// ToolExecutor executes a single tool call.
type ToolExecutor interface {
	// Execute runs the tool with given arguments.
	Execute(ctx context.Context, call core.ToolCall) (*core.ToolResult, error)

	// Definition returns the tool's schema for LLM.
	Definition() core.ToolDefinition

	// Metadata returns tool metadata.
	Metadata() core.ToolMetadata
}

// ToolRegistry manages available tools.
type ToolRegistry interface {
	// Register adds a tool to the registry.
	Register(tool ToolExecutor) error

	// Get retrieves a tool by name.
	Get(name string) (ToolExecutor, error)

	// List returns all available tools.
	List() []core.ToolDefinition

	// Unregister removes a tool.
	Unregister(name string) error
}

// ToolExecutionLimiter gates tool execution concurrency.
type ToolExecutionLimiter interface {
	// Limit returns the maximum number of concurrent tool executions.
	Limit() int
}
