package agent

import (
	"context"
	"time"
)

// Logger provides the minimal logging contract required by the domain layer.
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// Clock represents a time source that can be replaced in tests.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts a function to the Clock interface.
type ClockFunc func() time.Time

// Now implements the Clock interface.
func (f ClockFunc) Now() time.Time {
	return f()
}

// SystemClock returns the real system time.
type SystemClock struct{}

// Now implements Clock using time.Now.
func (SystemClock) Now() time.Time {
	return time.Now()
}

// AgentLevel represents the hierarchical level of agent execution.
type AgentLevel string

const (
	// LevelCore represents the main agent execution.
	LevelCore AgentLevel = "core"
	// LevelSubagent represents a nested subagent execution.
	LevelSubagent AgentLevel = "subagent"
	// LevelParallel represents parallel subagent execution.
	LevelParallel AgentLevel = "parallel"
)

// ToolCategory represents different categories of tools.
type ToolCategory string

const (
	CategoryFile      ToolCategory = "file"
	CategoryShell     ToolCategory = "shell"
	CategorySearch    ToolCategory = "search"
	CategoryWeb       ToolCategory = "web"
	CategoryTask      ToolCategory = "task"
	CategoryExecution ToolCategory = "execution"
	CategoryReasoning ToolCategory = "reasoning"
	CategoryOther     ToolCategory = "other"
)

// OutputContext provides hierarchical context for rendering.
type OutputContext struct {
	Level        AgentLevel
	Category     ToolCategory
	AgentID      string
	ParentID     string
	Verbose      bool
	SessionID    string
	TaskID       string
	ParentTaskID string
}

type outputContextKey struct{}
type silentModeKey struct{}

// WithOutputContext returns a context with output context.
func WithOutputContext(ctx context.Context, outCtx *OutputContext) context.Context {
	return context.WithValue(ctx, outputContextKey{}, outCtx)
}

// GetOutputContext retrieves the output context from the context.
// Defaults to a core level context when not present.
func GetOutputContext(ctx context.Context) *OutputContext {
	if val, ok := ctx.Value(outputContextKey{}).(*OutputContext); ok {
		return val
	}
	return &OutputContext{
		Level:        LevelCore,
		AgentID:      "core",
		Verbose:      false,
		SessionID:    "",
		TaskID:       "",
		ParentTaskID: "",
	}
}

// WithSilentMode marks the context to suppress event output.
func WithSilentMode(ctx context.Context) context.Context {
	return context.WithValue(ctx, silentModeKey{}, true)
}

// IsSilentMode checks if the context is in silent mode.
func IsSilentMode(ctx context.Context) bool {
	val, ok := ctx.Value(silentModeKey{}).(bool)
	return ok && val
}

// NoopLogger is a Logger implementation that discards all messages.
type NoopLogger struct{}

// Debug implements Logger.
func (NoopLogger) Debug(string, ...interface{}) {}

// Info implements Logger.
func (NoopLogger) Info(string, ...interface{}) {}

// Warn implements Logger.
func (NoopLogger) Warn(string, ...interface{}) {}

// Error implements Logger.
func (NoopLogger) Error(string, ...interface{}) {}
