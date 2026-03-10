package testutil

import (
	"path/filepath"
	"testing"

	"alex/internal/domain/task"
	"alex/internal/infra/taskstore"
)

// NewTestTaskStore creates a temporary file-backed task store for tests.
// The store is automatically closed when the test finishes.
func NewTestTaskStore(t *testing.T) task.Store {
	t.Helper()
	fp := filepath.Join(t.TempDir(), "tasks.json")
	s := taskstore.New(taskstore.WithFilePath(fp))
	t.Cleanup(func() { s.Close() })
	return s
}

// MakeTask creates a minimal task for testing with the given ID, description,
// and status. SessionID is set to "s1" and Channel to "test".
func MakeTask(id, desc string, status task.Status) *task.Task {
	return &task.Task{
		TaskID:      id,
		SessionID:   "s1",
		Description: desc,
		Status:      status,
		Channel:     "test",
	}
}
