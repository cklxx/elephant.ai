package task

import (
	"errors"
	"fmt"
)

// ErrTaskNotFound indicates a task lookup or mutation targeted a missing task.
var ErrTaskNotFound = errors.New("task not found")

// NotFoundError annotates ErrTaskNotFound with the missing task ID.
func NotFoundError(taskID string) error {
	return fmt.Errorf("task %s: %w", taskID, ErrTaskNotFound)
}
