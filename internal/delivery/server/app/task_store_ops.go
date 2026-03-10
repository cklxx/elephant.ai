package app

import (
	"context"
	"fmt"
	"time"

	"alex/internal/delivery/server/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

// SetStatus updates task status
func (s *InMemoryTaskStore) SetStatus(ctx context.Context, taskID string, status ports.TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	task.Status = status

	// Update timestamps and termination reason based on status
	now := time.Now()
	switch status {
	case ports.TaskStatusRunning:
		if task.StartedAt == nil {
			task.StartedAt = &now
		}
	case ports.TaskStatusCompleted:
		if task.CompletedAt == nil {
			task.CompletedAt = &now
		}
		if task.TerminationReason == ports.TerminationReasonNone {
			task.TerminationReason = ports.TerminationReasonCompleted
		}
		delete(s.owners, taskID)
		delete(s.leases, taskID)
	case ports.TaskStatusCancelled:
		if task.CompletedAt == nil {
			task.CompletedAt = &now
		}
		if task.TerminationReason == ports.TerminationReasonNone {
			task.TerminationReason = ports.TerminationReasonCancelled
		}
		delete(s.owners, taskID)
		delete(s.leases, taskID)
	case ports.TaskStatusFailed:
		if task.CompletedAt == nil {
			task.CompletedAt = &now
		}
		if task.TerminationReason == ports.TerminationReasonNone {
			task.TerminationReason = ports.TerminationReasonError
		}
		delete(s.owners, taskID)
		delete(s.leases, taskID)
	}

	s.persistLocked()
	return nil
}

// SetError records task failure
func (s *InMemoryTaskStore) SetError(ctx context.Context, taskID string, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	task.Error = err.Error()
	task.Status = ports.TaskStatusFailed
	task.TerminationReason = ports.TerminationReasonError
	now := time.Now()
	task.CompletedAt = &now
	delete(s.owners, taskID)
	delete(s.leases, taskID)

	s.persistLocked()
	return nil
}

// SetResult stores task completion result
func (s *InMemoryTaskStore) SetResult(ctx context.Context, taskID string, result *agent.TaskResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	task.Result = result
	task.Status = ports.TaskStatusCompleted
	task.TerminationReason = ports.TerminationReasonCompleted
	now := time.Now()
	task.CompletedAt = &now
	delete(s.owners, taskID)
	delete(s.leases, taskID)
	task.TotalIterations = result.Iterations
	task.TokensUsed = result.TokensUsed
	task.TotalTokens = result.TokensUsed // Total tokens = final tokens used

	task.SessionID = result.SessionID

	if result.ParentRunID != "" {
		task.ParentTaskID = result.ParentRunID
	}

	s.persistLocked()
	return nil
}

// UpdateProgress updates task execution progress
func (s *InMemoryTaskStore) UpdateProgress(ctx context.Context, taskID string, iteration int, tokensUsed int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	task.CurrentIteration = iteration
	task.TokensUsed = tokensUsed

	s.persistLocked()
	return nil
}

// SetTerminationReason sets the termination reason for a task
func (s *InMemoryTaskStore) SetTerminationReason(ctx context.Context, taskID string, reason ports.TerminationReason) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	task.TerminationReason = reason

	s.persistLocked()
	return nil
}

// TryClaimTask attempts to claim ownership for a task execution.
func (s *InMemoryTaskStore) TryClaimTask(ctx context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return false, NotFoundError(fmt.Sprintf("task %s", taskID))
	}
	if isTerminalStatus(task.Status) {
		return false, nil
	}
	if !s.isTaskClaimableLocked(taskID, ownerID, time.Now()) {
		return false, nil
	}
	s.owners[taskID] = ownerID
	s.leases[taskID] = leaseUntil
	s.persistLocked()
	return true, nil
}

// ClaimResumableTasks claims tasks by status for resume execution.
func (s *InMemoryTaskStore) ClaimResumableTasks(ctx context.Context, ownerID string, leaseUntil time.Time, limit int, statuses ...ports.TaskStatus) ([]*ports.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 100
	}
	if len(statuses) == 0 {
		return []*ports.Task{}, nil
	}
	statusSet := make(map[ports.TaskStatus]struct{}, len(statuses))
	for _, st := range statuses {
		statusSet[st] = struct{}{}
	}

	now := time.Now()
	claimed := make([]*ports.Task, 0, limit)
	for _, task := range s.tasks {
		if _, ok := statusSet[task.Status]; !ok {
			continue
		}
		if !s.isTaskClaimableLocked(task.ID, ownerID, now) {
			continue
		}
		s.owners[task.ID] = ownerID
		s.leases[task.ID] = leaseUntil
		taskCopy := *task
		claimed = append(claimed, &taskCopy)
		if len(claimed) >= limit {
			break
		}
	}
	if len(claimed) > 0 {
		s.persistLocked()
	}
	return claimed, nil
}

// RenewTaskLease refreshes lease for an already owned task.
func (s *InMemoryTaskStore) RenewTaskLease(ctx context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[taskID]; !exists {
		return false, NotFoundError(fmt.Sprintf("task %s", taskID))
	}
	if s.owners[taskID] != ownerID {
		return false, nil
	}
	s.leases[taskID] = leaseUntil
	s.persistLocked()
	return true, nil
}

// ReleaseTaskLease releases ownership for an owned task.
func (s *InMemoryTaskStore) ReleaseTaskLease(ctx context.Context, taskID, ownerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[taskID]; !exists {
		return nil
	}
	if s.owners[taskID] != ownerID {
		return nil
	}
	delete(s.owners, taskID)
	delete(s.leases, taskID)
	s.persistLocked()
	return nil
}

func (s *InMemoryTaskStore) isTaskClaimableLocked(taskID, ownerID string, now time.Time) bool {
	currentOwner := s.owners[taskID]
	currentLease := s.leases[taskID]
	return currentOwner == "" || currentOwner == ownerID || currentLease.IsZero() || currentLease.Before(now)
}
