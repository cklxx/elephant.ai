package coordinator

import (
	"strings"
	"sync"

	"alex/internal/agent/domain/react"
)

type backgroundTaskRegistry struct {
	mu       sync.Mutex
	managers map[string]*react.BackgroundTaskManager
}

func newBackgroundTaskRegistry() *backgroundTaskRegistry {
	return &backgroundTaskRegistry{
		managers: make(map[string]*react.BackgroundTaskManager),
	}
}

func (r *backgroundTaskRegistry) Get(sessionID string, create func() *react.BackgroundTaskManager) *react.BackgroundTaskManager {
	if r == nil {
		return nil
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if mgr := r.managers[sessionID]; mgr != nil {
		return mgr
	}
	if create == nil {
		return nil
	}
	mgr := create()
	if mgr != nil {
		r.managers[sessionID] = mgr
	}
	return mgr
}
