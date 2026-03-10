package react

import (
	"context"
	"fmt"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// CancelBackgroundTask implements agent.BackgroundTaskCanceller.
func (m *BackgroundTaskManager) CancelBackgroundTask(ctx context.Context, taskID string) error {
	return m.CancelTask(ctx, taskID)
}

// Status returns lightweight summaries for the requested task IDs.
// Pass nil or empty slice to query all tasks.
func (m *BackgroundTaskManager) Status(ids []string) []agent.BackgroundTaskSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := m.resolveTargets(ids)
	summaries := make([]agent.BackgroundTaskSummary, 0, len(targets))
	for _, bt := range targets {
		bt.mu.Lock()
		now := m.clock.Now()
		elapsed := time.Duration(0)
		if !bt.startedAt.IsZero() {
			end := bt.completedAt
			if end.IsZero() {
				end = now
			}
			elapsed = end.Sub(bt.startedAt)
			if elapsed < 0 {
				elapsed = 0
			}
		}
		s := agent.BackgroundTaskSummary{
			ID:             bt.id,
			Description:    bt.description,
			Status:         bt.status,
			AgentType:      bt.agentType,
			ExecutionMode:  bt.executionMode,
			AutonomyLevel:  bt.autonomyLevel,
			StartedAt:      bt.startedAt,
			CompletedAt:    bt.completedAt,
			Elapsed:        elapsed,
			LastActivityAt: bt.lastActivityAt,
		}
		if bt.status == agent.BackgroundTaskStatusRunning && isStale(now, bt.lastActivityAt, m.staleThreshold) {
			s.Stale = true
		}
		if bt.err != nil {
			s.Error = bt.err.Error()
		}
		if bt.progress != nil {
			progress := *bt.progress
			s.Progress = &progress
		}
		if bt.pendingInput != nil {
			pending := *bt.pendingInput
			s.PendingInput = &pending
		}
		if bt.workspace != nil {
			workspaceCopy := *bt.workspace
			s.Workspace = &workspaceCopy
		}
		if len(bt.fileScope) > 0 {
			s.FileScope = append([]string(nil), bt.fileScope...)
		}
		if len(bt.dependsOn) > 0 {
			s.DependsOn = append([]string(nil), bt.dependsOn...)
		}
		bt.mu.Unlock()
		summaries = append(summaries, s)
	}
	return summaries
}

// Collect returns full results for the requested tasks.
// When wait is true, blocks until tasks complete or timeout elapses.
func (m *BackgroundTaskManager) Collect(ids []string, wait bool, timeout time.Duration) []agent.BackgroundTaskResult {
	if wait {
		m.awaitTasks(ids, timeout)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := m.resolveTargets(ids)
	results := make([]agent.BackgroundTaskResult, 0, len(targets))
	for _, bt := range targets {
		bt.mu.Lock()
		duration := time.Duration(0)
		if !bt.completedAt.IsZero() {
			duration = bt.completedAt.Sub(bt.startedAt)
			if duration < 0 {
				duration = 0
			}
		}
		r := agent.BackgroundTaskResult{
			ID:          bt.id,
			Description: bt.description,
			Status:      bt.status,
			AgentType:   bt.agentType,
			Duration:    duration,
		}
		if bt.result != nil {
			r.Answer = bt.result.Answer
			r.RunID = bt.result.RunID
			r.Iterations = bt.result.Iterations
			r.TokensUsed = bt.result.TokensUsed
		}
		if bt.err != nil {
			r.Error = bt.err.Error()
		}
		bt.mu.Unlock()
		results = append(results, r)
	}
	return results
}

// DrainCompletions returns all newly completed task IDs without blocking.
func (m *BackgroundTaskManager) DrainCompletions() []string {
	m.completedMu.Lock()
	ids := m.completedIDs
	m.completedIDs = nil
	m.completedMu.Unlock()
	return ids
}

// AwaitAll blocks until every dispatched task has finished or the timeout elapses.
// Returns true if all tasks completed, false if the timeout was reached.
func (m *BackgroundTaskManager) AwaitAll(timeout time.Duration) bool {
	return m.awaitTasks(nil, timeout)
}

// CancelTask cancels an individual background task by its ID.
// Returns an error if the task does not exist or is already in a terminal state.
func (m *BackgroundTaskManager) CancelTask(ctx context.Context, taskID string) error {
	m.mu.RLock()
	bt := m.tasks[taskID]
	m.mu.RUnlock()
	if bt == nil {
		return fmt.Errorf("%w: task %q", ErrBackgroundTaskNotFound, taskID)
	}

	bt.mu.Lock()
	defer bt.mu.Unlock()
	switch bt.status {
	case agent.BackgroundTaskStatusCompleted, agent.BackgroundTaskStatusFailed, agent.BackgroundTaskStatusCancelled:
		return fmt.Errorf("task %q already %s", taskID, bt.status)
	}
	if bt.taskCancel != nil {
		bt.taskCancel()
	}
	return nil
}

// Shutdown cancels all remaining tasks.
func (m *BackgroundTaskManager) Shutdown() {
	m.cancelAll()
}

// TaskCount returns the number of tracked tasks.
func (m *BackgroundTaskManager) TaskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tasks)
}

// resolveTargets returns the tasks matching ids, or all tasks when ids is empty.
// Caller must hold m.mu (read lock or write lock).
func (m *BackgroundTaskManager) resolveTargets(ids []string) []*backgroundTask {
	if len(ids) == 0 {
		targets := make([]*backgroundTask, 0, len(m.tasks))
		for _, bt := range m.tasks {
			targets = append(targets, bt)
		}
		return targets
	}
	targets := make([]*backgroundTask, 0, len(ids))
	for _, tid := range ids {
		if bt, ok := m.tasks[tid]; ok {
			targets = append(targets, bt)
		}
	}
	return targets
}

// awaitTasks blocks until the specified tasks (or all tasks when ids is empty)
// are no longer pending/running, or timeout elapses. Returns true if all
// tasks completed, false if the timeout was reached first.
func (m *BackgroundTaskManager) awaitTasks(ids []string, timeout time.Duration) bool {
	deadline := m.clock.Now().Add(timeout)

	for {
		if m.clock.Now().After(deadline) {
			return false
		}

		allDone := true
		m.mu.RLock()
		targets := m.resolveTargets(ids)
		for _, bt := range targets {
			bt.mu.Lock()
			status := bt.status
			signaled := bt.completionSignaled
			if status == agent.BackgroundTaskStatusPending ||
				status == agent.BackgroundTaskStatusRunning ||
				status == agent.BackgroundTaskStatusBlocked ||
				(!signaled &&
					(status == agent.BackgroundTaskStatusCompleted ||
						status == agent.BackgroundTaskStatusFailed ||
						status == agent.BackgroundTaskStatusCancelled)) {
				allDone = false
			}
			bt.mu.Unlock()
			if !allDone {
				break
			}
		}
		m.mu.RUnlock()

		if allDone {
			return true
		}

		remaining := deadline.Sub(m.clock.Now())
		if remaining <= 0 {
			return false
		}
		if remaining > 2*time.Second {
			remaining = 2 * time.Second
		}
		select {
		case <-m.depWaitChan():
		case <-time.After(remaining):
		}
	}
}
