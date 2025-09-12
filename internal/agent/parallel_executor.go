package agent

import (
	"context"
	"fmt"

	"alex/internal/tools/builtin"
)

// ParallelExecutorWrapper - Wrapper to implement ParallelSubAgentExecutor interface
// This avoids circular dependency issues between agent and builtin packages
type ParallelExecutorWrapper struct {
	reactCore *ReactCore
}

// NewParallelExecutorWrapper - Creates a new wrapper for parallel execution
func NewParallelExecutorWrapper(reactCore *ReactCore) builtin.ParallelSubAgentExecutor {
	return &ParallelExecutorWrapper{
		reactCore: reactCore,
	}
}

// ExecuteTasksParallel - Implementation of ParallelSubAgentExecutor interface
func (pew *ParallelExecutorWrapper) ExecuteTasksParallel(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if pew.reactCore == nil {
		return nil, fmt.Errorf("ReactCore not available")
	}
	
	if pew.reactCore.parallelAgent == nil {
		return nil, fmt.Errorf("parallel subagent not initialized")
	}
	
	return pew.reactCore.parallelAgent.ExecuteTasksParallelFromTool(ctx, args)
}