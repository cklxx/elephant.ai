package taskfmt

import (
	"time"

	"alex/internal/domain/task"
)

// InWindow returns true if the task was updated or completed within the window.
func InWindow(t *task.Task, from time.Time) bool {
	if t.CompletedAt != nil && t.CompletedAt.After(from) {
		return true
	}
	return t.UpdatedAt.After(from)
}

// CountFailed returns the number of failed tasks in a slice.
func CountFailed(tasks []*task.Task) int {
	n := 0
	for _, t := range tasks {
		if t.Status == task.StatusFailed {
			n++
		}
	}
	return n
}
