package app

import (
	"context"

	serverPorts "alex/internal/delivery/server/ports"
)

// GetTask retrieves a task by ID.
func (svc *TaskExecutionService) GetTask(ctx context.Context, taskID string) (*serverPorts.Task, error) {
	return svc.taskStore.Get(ctx, taskID)
}

// ListTasks returns all tasks with pagination.
func (svc *TaskExecutionService) ListTasks(ctx context.Context, limit int, offset int) ([]*serverPorts.Task, int, error) {
	return svc.taskStore.List(ctx, limit, offset)
}

// ListSessionTasks returns all tasks for a session.
func (svc *TaskExecutionService) ListSessionTasks(ctx context.Context, sessionID string) ([]*serverPorts.Task, error) {
	return svc.taskStore.ListBySession(ctx, sessionID)
}

// SummarizeSessionTasks returns task_count/last_task for each requested session.
// It uses an optional batched task-store capability when available.
func (svc *TaskExecutionService) SummarizeSessionTasks(ctx context.Context, sessionIDs []string) (map[string]SessionTaskSummary, error) {
	summaries := make(map[string]SessionTaskSummary, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return summaries, nil
	}

	if batchStore, ok := svc.taskStore.(sessionTaskSummaryStore); ok {
		return batchStore.SummarizeSessionTasks(ctx, sessionIDs)
	}

	seen := make(map[string]struct{}, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if sessionID == "" {
			continue
		}
		if _, exists := seen[sessionID]; exists {
			continue
		}
		seen[sessionID] = struct{}{}

		tasks, err := svc.taskStore.ListBySession(ctx, sessionID)
		if err != nil {
			return nil, err
		}

		summary := SessionTaskSummary{TaskCount: len(tasks)}
		if len(tasks) > 0 {
			// ListBySession returns tasks sorted newest-first.
			summary.LastTask = tasks[0].Description
		}
		summaries[sessionID] = summary
	}

	return summaries, nil
}

// ListActiveTasks returns all currently running tasks.
func (svc *TaskExecutionService) ListActiveTasks(ctx context.Context) ([]*serverPorts.Task, error) {
	return svc.taskStore.ListByStatus(ctx, serverPorts.TaskStatusPending, serverPorts.TaskStatusRunning)
}

// TaskStats returns aggregated task metrics.
type TaskStats struct {
	ActiveCount    int     `json:"active_count"`
	PendingCount   int     `json:"pending_count"`
	RunningCount   int     `json:"running_count"`
	CompletedCount int     `json:"completed_count"`
	FailedCount    int     `json:"failed_count"`
	CancelledCount int     `json:"cancelled_count"`
	TotalCount     int     `json:"total_count"`
	TotalTokens    int     `json:"total_tokens"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
}

// GetTaskStats computes aggregated task metrics.
func (svc *TaskExecutionService) GetTaskStats(ctx context.Context) (*TaskStats, error) {
	tasks, total, err := svc.taskStore.List(ctx, 10000, 0)
	if err != nil {
		return nil, err
	}

	stats := &TaskStats{TotalCount: total}
	for _, t := range tasks {
		switch t.Status {
		case serverPorts.TaskStatusPending:
			stats.PendingCount++
			stats.ActiveCount++
		case serverPorts.TaskStatusRunning:
			stats.RunningCount++
			stats.ActiveCount++
		case serverPorts.TaskStatusCompleted:
			stats.CompletedCount++
		case serverPorts.TaskStatusFailed:
			stats.FailedCount++
		case serverPorts.TaskStatusCancelled:
			stats.CancelledCount++
		}
		stats.TotalTokens += t.TokensUsed
	}

	return stats, nil
}
