package coordinator

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"alex/internal/domain/agent/react"
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

// CancelTask searches all managers for the given task ID and cancels it.
func (r *backgroundTaskRegistry) CancelTask(ctx context.Context, taskID string) error {
	if r == nil {
		return fmt.Errorf("background task registry not available")
	}

	r.mu.Lock()
	managers := make([]*react.BackgroundTaskManager, 0, len(r.managers))
	for _, mgr := range r.managers {
		managers = append(managers, mgr)
	}
	r.mu.Unlock()

	for _, mgr := range managers {
		err := mgr.CancelTask(ctx, taskID)
		if err == nil {
			return nil
		}
		// "not found" means try next manager; other errors are real failures
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
	}
	return fmt.Errorf("task %q not found in any session", taskID)
}
