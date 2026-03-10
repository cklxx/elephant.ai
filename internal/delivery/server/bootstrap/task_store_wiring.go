package bootstrap

import (
	"fmt"

	"alex/internal/app/di"
	"alex/internal/delivery/channels/lark"
	"alex/internal/delivery/server/ports"
	"alex/internal/delivery/taskadapters"
)

func serverTaskStoreForContainer(container *di.Container) (ports.TaskStore, error) {
	if container == nil {
		return nil, fmt.Errorf("server task store requires container")
	}
	if container.TaskStore == nil {
		return nil, fmt.Errorf("server task store requires unified task store")
	}
	return taskadapters.NewServerAdapter(container.TaskStore), nil
}

func larkTaskStoreForContainer(container *di.Container) (lark.TaskStore, error) {
	if container == nil {
		return nil, fmt.Errorf("lark task store requires container")
	}
	if container.TaskStore == nil {
		return nil, fmt.Errorf("lark task store requires unified task store")
	}
	return taskadapters.NewLarkAdapter(container.TaskStore), nil
}
