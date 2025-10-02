package types

import "context"

// AgentLevel represents the hierarchical level of agent execution
// This is a shared type used across domain, output, and other packages
type AgentLevel string

const (
	// LevelCore represents the main agent execution
	LevelCore AgentLevel = "core"

	// LevelSubagent represents a nested subagent execution
	LevelSubagent AgentLevel = "subagent"

	// LevelParallel represents parallel subagent execution
	LevelParallel AgentLevel = "parallel"
)

// String returns the string representation of AgentLevel
func (l AgentLevel) String() string {
	return string(l)
}

// IsSubagent returns true if the level is subagent or parallel
func (l AgentLevel) IsSubagent() bool {
	return l == LevelSubagent || l == LevelParallel
}

// IsCore returns true if the level is core
func (l AgentLevel) IsCore() bool {
	return l == LevelCore
}

// ToolCategory represents different categories of tools
type ToolCategory string

const (
	CategoryFile      ToolCategory = "file"      // File operations
	CategoryShell     ToolCategory = "shell"     // Shell commands
	CategorySearch    ToolCategory = "search"    // Code/file search
	CategoryWeb       ToolCategory = "web"       // Web operations
	CategoryTask      ToolCategory = "task"      // Task management
	CategoryExecution ToolCategory = "execution" // Code execution
	CategoryReasoning ToolCategory = "reasoning" // LLM reasoning
	CategoryOther     ToolCategory = "other"     // Uncategorized
)

// OutputContext provides hierarchical context for rendering
type OutputContext struct {
	Level    AgentLevel   // Agent execution level
	Category ToolCategory // Tool category (if applicable)
	AgentID  string       // Unique agent identifier
	ParentID string       // Parent agent ID (for subagents)
	Verbose  bool         // Verbose output mode
}

// Context keys for storing values in context
type outputContextKey struct{}
type silentModeKey struct{}

// WithOutputContext returns a context with output context
func WithOutputContext(ctx context.Context, outCtx *OutputContext) context.Context {
	return context.WithValue(ctx, outputContextKey{}, outCtx)
}

// GetOutputContext retrieves the output context from the context
// Returns a default core-level context if not found
func GetOutputContext(ctx context.Context) *OutputContext {
	if val, ok := ctx.Value(outputContextKey{}).(*OutputContext); ok {
		return val
	}
	// Default to core level
	return &OutputContext{
		Level:   LevelCore,
		AgentID: "core",
		Verbose: false,
	}
}

// WithSilentMode marks the context to suppress event output
func WithSilentMode(ctx context.Context) context.Context {
	return context.WithValue(ctx, silentModeKey{}, true)
}

// IsSilentMode checks if the context is in silent mode
func IsSilentMode(ctx context.Context) bool {
	val, ok := ctx.Value(silentModeKey{}).(bool)
	return ok && val
}
