package types

import (
	"context"

	agent "alex/internal/domain/agent/ports/agent"
)

type AgentLevel = agent.AgentLevel

const (
	LevelCore     = agent.LevelCore
	LevelSubagent = agent.LevelSubagent
	LevelParallel = agent.LevelParallel
)

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

