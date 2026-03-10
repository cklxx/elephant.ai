package taskstore

import (
	"context"
	"fmt"
	"time"

	"alex/internal/domain/task"
)

// TryClaimTask tries to claim task ownership. Returns true when the claim succeeds.
func (s *LocalStore) TryClaimTask(_ context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return false, fmt.Errorf("task %s not found", taskID)
	}

	// If already owned by another live lease, reject.
	if existing, claimed := s.owners[taskID]; claimed && existing != ownerID {
		if exp, hasLease := s.leases[taskID]; hasLease && time.Now().Before(exp) {
			return false, nil
		}
	}

	s.owners[taskID] = ownerID
	s.leases[taskID] = leaseUntil
	t.UpdatedAt = time.Now()
	s.persistLocked()
	return true, nil
}

// ClaimResumableTasks atomically claims tasks in the given statuses and returns them.
func (s *LocalStore) ClaimResumableTasks(_ context.Context, ownerID string, leaseUntil time.Time, limit int, statuses ...task.Status) ([]*task.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	set := make(map[task.Status]struct{}, len(statuses))
	for _, st := range statuses {
		set[st] = struct{}{}
	}

	now := time.Now()
	var claimed []*task.Task
	for id, t := range s.tasks {
		if _, match := set[t.Status]; !match {
			continue
		}
		// Skip if owned by another live lease.
		if existing, ok := s.owners[id]; ok && existing != ownerID {
			if exp, hasLease := s.leases[id]; hasLease && now.Before(exp) {
				continue
			}
		}
		s.owners[id] = ownerID
		s.leases[id] = leaseUntil
		t.UpdatedAt = now
		claimed = append(claimed, s.copyTask(t))
		if limit > 0 && len(claimed) >= limit {
			break
		}
	}

	if len(claimed) > 0 {
		s.persistLocked()
	}
	return claimed, nil
}

// RenewTaskLease extends the lease for a task owned by ownerID.
func (s *LocalStore) RenewTaskLease(_ context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[taskID]; !ok {
		return false, fmt.Errorf("task %s not found", taskID)
	}

	existing, ok := s.owners[taskID]
	if !ok || existing != ownerID {
		return false, nil
	}

	s.leases[taskID] = leaseUntil
	s.persistLocked()
	return true, nil
}

// ReleaseTaskLease releases ownership for a task.
func (s *LocalStore) ReleaseTaskLease(_ context.Context, taskID, ownerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[taskID]; !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	existing, ok := s.owners[taskID]
	if !ok || existing != ownerID {
		return nil // Not owned by this owner — no-op.
	}

	delete(s.owners, taskID)
	delete(s.leases, taskID)
	s.persistLocked()
	return nil
}

// Transitions returns the audit trail for a task.
func (s *LocalStore) Transitions(_ context.Context, taskID string) ([]task.Transition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.tasks[taskID]; !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	trs := s.transitions[taskID]
	out := make([]task.Transition, len(trs))
	copy(out, trs)
	return out, nil
}

// MarkStaleRunning marks all running/pending tasks as failed with the given reason.
func (s *LocalStore) MarkStaleRunning(_ context.Context, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	changed := false
	for id, t := range s.tasks {
		if t.Status != task.StatusRunning && t.Status != task.StatusPending {
			continue
		}
		from := t.Status
		t.Status = task.StatusFailed
		t.TerminationReason = task.TerminationError
		t.Error = reason
		t.UpdatedAt = now
		if t.CompletedAt == nil {
			t.CompletedAt = &now
		}
		delete(s.owners, id)
		delete(s.leases, id)
		s.addTransitionLocked(id, from, task.StatusFailed, task.TransitionParams{Reason: reason})
		changed = true
	}

	if changed {
		s.persistLocked()
	}
	return nil
}

// DeleteExpired removes tasks completed before the given time.
func (s *LocalStore) DeleteExpired(_ context.Context, before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for id, t := range s.tasks {
		if !t.Status.IsTerminal() {
			continue
		}
		if t.CompletedAt != nil && t.CompletedAt.Before(before) {
			delete(s.tasks, id)
			delete(s.owners, id)
			delete(s.leases, id)
			delete(s.transitions, id)
			changed = true
		}
	}

	if changed {
		s.persistLocked()
	}
	return nil
}
