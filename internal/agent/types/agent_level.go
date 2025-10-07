package types

import (
	"context"

	"alex/internal/agent/ports"
)

type AgentLevel = ports.AgentLevel

const (
	LevelCore     = ports.LevelCore
	LevelSubagent = ports.LevelSubagent
	LevelParallel = ports.LevelParallel
)

// AgentLevelString returns the string representation of an agent level.
func AgentLevelString(l AgentLevel) string {
	return string(l)
}

// IsSubagentLevel returns true if the level is subagent or parallel.
func IsSubagentLevel(l AgentLevel) bool {
	return l == LevelSubagent || l == LevelParallel
}

// IsCoreLevel returns true if the level is core.
func IsCoreLevel(l AgentLevel) bool {
	return l == LevelCore
}

type ToolCategory = ports.ToolCategory

const (
	CategoryFile      = ports.CategoryFile
	CategoryShell     = ports.CategoryShell
	CategorySearch    = ports.CategorySearch
	CategoryWeb       = ports.CategoryWeb
	CategoryTask      = ports.CategoryTask
	CategoryExecution = ports.CategoryExecution
	CategoryReasoning = ports.CategoryReasoning
	CategoryOther     = ports.CategoryOther
)

type OutputContext = ports.OutputContext

func WithOutputContext(ctx context.Context, outCtx *OutputContext) context.Context {
	return ports.WithOutputContext(ctx, outCtx)
}

func GetOutputContext(ctx context.Context) *OutputContext {
	return ports.GetOutputContext(ctx)
}

func WithSilentMode(ctx context.Context) context.Context {
	return ports.WithSilentMode(ctx)
}

func IsSilentMode(ctx context.Context) bool {
	return ports.IsSilentMode(ctx)
}
