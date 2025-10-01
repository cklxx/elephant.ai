package app

import (
	"alex/internal/agent/domain"
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
)

// SubAgentOrchestrator handles task decomposition and delegation
type SubAgentOrchestrator struct {
	coordinator *AgentCoordinator
	maxWorkers  int
}

func NewSubAgentOrchestrator(coordinator *AgentCoordinator, maxWorkers int) *SubAgentOrchestrator {
	return &SubAgentOrchestrator{
		coordinator: coordinator,
		maxWorkers:  maxWorkers,
	}
}

// SubTask represents a delegated sub-task
type SubTask struct {
	ID          string
	Description string
	Context     string
}

// ExecuteParallel runs sub-tasks concurrently
func (o *SubAgentOrchestrator) ExecuteParallel(
	ctx context.Context,
	tasks []SubTask,
) ([]domain.TaskResult, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(o.maxWorkers)

	results := make([]domain.TaskResult, len(tasks))

	for i, task := range tasks {
		i, task := i, task // Capture loop variables

		g.Go(func() error {
			result, err := o.coordinator.ExecuteTask(ctx, task.Description, "")
			if err != nil {
				return fmt.Errorf("task %s failed: %w", task.ID, err)
			}
			// Convert ports.TaskResult to domain.TaskResult
			results[i] = domain.TaskResult{
				Answer:     result.Answer,
				Messages:   nil, // SubAgent doesn't need full message history
				Iterations: result.Iterations,
				TokensUsed: result.TokensUsed,
				StopReason: result.StopReason,
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// ExecuteSerial runs sub-tasks sequentially
func (o *SubAgentOrchestrator) ExecuteSerial(
	ctx context.Context,
	tasks []SubTask,
	sessionID string,
) ([]domain.TaskResult, error) {
	results := make([]domain.TaskResult, len(tasks))

	for i, task := range tasks {
		result, err := o.coordinator.ExecuteTask(ctx, task.Description, sessionID)
		if err != nil {
			return nil, fmt.Errorf("task %s failed: %w", task.ID, err)
		}
		// Convert ports.TaskResult to domain.TaskResult
		results[i] = domain.TaskResult{
			Answer:     result.Answer,
			Messages:   nil, // SubAgent doesn't need full message history
			Iterations: result.Iterations,
			TokensUsed: result.TokensUsed,
			StopReason: result.StopReason,
		}
	}

	return results, nil
}
