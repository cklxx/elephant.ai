package types

import (
	"context"

	agent "alex/internal/agent/ports/agent"
)

type AgentLevel = agent.AgentLevel

const (
	LevelCore     = agent.LevelCore
	LevelSubagent = agent.LevelSubagent
	LevelParallel = agent.LevelParallel
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

type ToolCategory = agent.ToolCategory

const (
	CategoryFile      = agent.CategoryFile
	CategoryShell     = agent.CategoryShell
	CategorySearch    = agent.CategorySearch
	CategoryWeb       = agent.CategoryWeb
	CategoryTask      = agent.CategoryTask
	CategoryExecution = agent.CategoryExecution
	CategoryReasoning = agent.CategoryReasoning
	CategoryOther     = agent.CategoryOther
)

type OutputContext = agent.OutputContext

func WithOutputContext(ctx context.Context, outCtx *OutputContext) context.Context {
	return agent.WithOutputContext(ctx, outCtx)
}

func GetOutputContext(ctx context.Context) *OutputContext {
	return agent.GetOutputContext(ctx)
}

func WithSilentMode(ctx context.Context) context.Context {
	return agent.WithSilentMode(ctx)
}

func IsSilentMode(ctx context.Context) bool {
	return agent.IsSilentMode(ctx)
}
