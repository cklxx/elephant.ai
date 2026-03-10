package taskstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"alex/internal/domain/task"
)

// Create persists a new task.
func (s *LocalStore) Create(_ context.Context, t *task.Task) error {
	if t == nil || t.TaskID == "" {
		return fmt.Errorf("task or task_id is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[t.TaskID]; exists {
		return fmt.Errorf("task %s already exists", t.TaskID)
	}

	cp := *t
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now()
	}
	cp.UpdatedAt = time.Now()
	s.tasks[cp.TaskID] = &cp
	s.persistLocked()
	return nil
}

// Get retrieves a task by ID.
func (s *LocalStore) Get(_ context.Context, taskID string) (*task.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return s.copyTask(t), nil
}

// SetStatus updates the task status and writes a transition record atomically.
func (s *LocalStore) SetStatus(_ context.Context, taskID string, status task.Status, opts ...task.TransitionOption) error {
	params := task.ApplyTransitionOptions(opts)

	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	from := t.Status
	t.Status = status
	t.UpdatedAt = time.Now()

	now := time.Now()
	switch status {
	case task.StatusRunning:
		if t.StartedAt == nil {
			t.StartedAt = &now
		}
	case task.StatusCompleted, task.StatusFailed, task.StatusCancelled:
		if t.CompletedAt == nil {
			t.CompletedAt = &now
		}
		delete(s.owners, taskID)
		delete(s.leases, taskID)
	}

	if status == task.StatusCompleted && t.TerminationReason == "" {
		t.TerminationReason = task.TerminationCompleted
	}
	if status == task.StatusCancelled && t.TerminationReason == "" {
		t.TerminationReason = task.TerminationCancelled
	}
	if status == task.StatusFailed && t.TerminationReason == "" {
		t.TerminationReason = task.TerminationError
	}

	if params.AnswerPreview != nil {
		t.AnswerPreview = *params.AnswerPreview
	}
	if params.ErrorText != nil {
		t.Error = *params.ErrorText
	}
	if params.TokensUsed != nil {
		t.TokensUsed = *params.TokensUsed
	}

	s.addTransitionLocked(taskID, from, status, params)
	s.persistLocked()
	return nil
}

// UpdateProgress updates iteration and token counts.
func (s *LocalStore) UpdateProgress(_ context.Context, taskID string, iteration int, tokensUsed int, costUSD float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	t.CurrentIteration = iteration
	t.TokensUsed = tokensUsed
	t.CostUSD = costUSD
	t.UpdatedAt = time.Now()
	s.persistLocked()
	return nil
}

// SetResult stores the completion result.
func (s *LocalStore) SetResult(_ context.Context, taskID string, answer string, resultJSON json.RawMessage, tokensUsed int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	t.AnswerPreview = answer
	t.ResultJSON = resultJSON
	t.TokensUsed = tokensUsed
	t.UpdatedAt = time.Now()
	s.persistLocked()
	return nil
}

// SetError records a task failure.
func (s *LocalStore) SetError(_ context.Context, taskID string, errText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	t.Error = errText
	t.UpdatedAt = time.Now()
	s.persistLocked()
	return nil
}

// SetBridgeMeta persists bridge checkpoint data.
func (s *LocalStore) SetBridgeMeta(_ context.Context, taskID string, meta task.BridgeMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	t.BridgeMeta = &meta
	t.UpdatedAt = time.Now()
	s.persistLocked()
	return nil
}

// Delete removes a task.
func (s *LocalStore) Delete(_ context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[taskID]; !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	delete(s.tasks, taskID)
	delete(s.owners, taskID)
	delete(s.leases, taskID)
	delete(s.transitions, taskID)
	s.persistLocked()
	return nil
}
