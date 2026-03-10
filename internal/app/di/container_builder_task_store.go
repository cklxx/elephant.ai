package di

import (
	"path/filepath"

	taskdomain "alex/internal/domain/task"
	taskstoreinfra "alex/internal/infra/taskstore"
)

func (b *containerBuilder) buildTaskStore() (taskdomain.Store, error) {
	path := filepath.Join(b.sessionDir, "_tasks", "tasks.json")
	return taskstoreinfra.New(taskstoreinfra.WithFilePath(path)), nil
}
