package taskstore

import (
	"context"
	"sort"

	"alex/internal/domain/task"
)

// ListBySession returns tasks for a session, newest first.
func (s *LocalStore) ListBySession(_ context.Context, sessionID string, limit int) ([]*task.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*task.Task
	for _, t := range s.tasks {
		if t.SessionID == sessionID {
			result = append(result, s.copyTask(t))
		}
	}
	sortByCreatedDesc(result)
	return applyLimit(result, limit), nil
}

// ListByChat returns tasks for a chat, optionally filtered to active-only.
func (s *LocalStore) ListByChat(_ context.Context, chatID string, activeOnly bool, limit int) ([]*task.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*task.Task
	for _, t := range s.tasks {
		if t.ChatID != chatID {
			continue
		}
		if activeOnly && t.Status.IsTerminal() {
			continue
		}
		result = append(result, s.copyTask(t))
	}
	sortByCreatedDesc(result)
	return applyLimit(result, limit), nil
}

// ListByStatus returns tasks matching any of the given statuses.
func (s *LocalStore) ListByStatus(_ context.Context, statuses ...task.Status) ([]*task.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set := make(map[task.Status]struct{}, len(statuses))
	for _, st := range statuses {
		set[st] = struct{}{}
	}

	var result []*task.Task
	for _, t := range s.tasks {
		if _, ok := set[t.Status]; ok {
			result = append(result, s.copyTask(t))
		}
	}
	sortByCreatedDesc(result)
	return result, nil
}

// ListActive returns all non-terminal tasks.
func (s *LocalStore) ListActive(_ context.Context) ([]*task.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*task.Task
	for _, t := range s.tasks {
		if !t.Status.IsTerminal() {
			result = append(result, s.copyTask(t))
		}
	}
	sortByCreatedDesc(result)
	return result, nil
}

// List returns paginated tasks, newest first. Returns (tasks, total, error).
func (s *LocalStore) List(_ context.Context, limit int, offset int) ([]*task.Task, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]*task.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		all = append(all, s.copyTask(t))
	}
	sortByCreatedDesc(all)

	total := len(all)
	if offset >= total {
		return nil, total, nil
	}
	all = all[offset:]
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, total, nil
}

func sortByCreatedDesc(tasks []*task.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
}

func applyLimit(tasks []*task.Task, limit int) []*task.Task {
	if limit > 0 && len(tasks) > limit {
		return tasks[:limit]
	}
	return tasks
}
