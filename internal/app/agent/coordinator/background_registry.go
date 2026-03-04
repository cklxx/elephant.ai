package coordinator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/react"
)

const (
	defaultBackgroundRegistryCleanupInterval = 3 * time.Minute
	defaultBackgroundRegistryIdleTTL         = 15 * time.Minute
	defaultBackgroundRegistryMaxEntryAge     = 1 * time.Hour
)

type backgroundRegistryEntry struct {
	manager    *react.BackgroundTaskManager
	lastAccess time.Time
}

type backgroundTaskRegistry struct {
	mu              sync.Mutex
	managers        map[string]backgroundRegistryEntry
	nowFn           func() time.Time
	shutdownFn      func(*react.BackgroundTaskManager)
	cleanupInterval time.Duration
	idleTTL         time.Duration
	maxEntryAge     time.Duration
	lastCleanup     time.Time
}

func newBackgroundTaskRegistry() *backgroundTaskRegistry {
	return &backgroundTaskRegistry{
		managers:        make(map[string]backgroundRegistryEntry),
		nowFn:           time.Now,
		shutdownFn:      func(mgr *react.BackgroundTaskManager) { mgr.Shutdown() },
		cleanupInterval: defaultBackgroundRegistryCleanupInterval,
		idleTTL:         defaultBackgroundRegistryIdleTTL,
		maxEntryAge:     defaultBackgroundRegistryMaxEntryAge,
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

	now := r.now()
	r.mu.Lock()
	r.cleanupLocked(now)

	if entry, ok := r.managers[sessionID]; ok && entry.manager != nil {
		entry.lastAccess = now
		r.managers[sessionID] = entry
		r.mu.Unlock()
		return entry.manager
	}
	if create == nil {
		r.mu.Unlock()
		return nil
	}
	mgr := create()
	if mgr != nil {
		r.managers[sessionID] = backgroundRegistryEntry{
			manager:    mgr,
			lastAccess: now,
		}
	}
	r.mu.Unlock()
	return mgr
}

// CancelTask searches all managers for the given task ID and cancels it.
func (r *backgroundTaskRegistry) CancelTask(ctx context.Context, taskID string) error {
	if r == nil {
		return fmt.Errorf("background task registry not available")
	}

	now := r.now()
	r.mu.Lock()
	r.cleanupLocked(now)
	type managerEntry struct {
		sessionID string
		manager   *react.BackgroundTaskManager
	}
	managers := make([]managerEntry, 0, len(r.managers))
	for sessionID, entry := range r.managers {
		if entry.manager == nil {
			continue
		}
		managers = append(managers, managerEntry{
			sessionID: sessionID,
			manager:   entry.manager,
		})
	}
	r.mu.Unlock()

	for _, entry := range managers {
		err := entry.manager.CancelTask(ctx, taskID)
		if err == nil {
			r.touch(entry.sessionID)
			return nil
		}
		if !errors.Is(err, react.ErrBackgroundTaskNotFound) {
			return err
		}
	}
	return fmt.Errorf("task %q not found in any session", taskID)
}

func (r *backgroundTaskRegistry) cleanupLocked(now time.Time) {
	if r.cleanupInterval > 0 && !r.lastCleanup.IsZero() && now.Sub(r.lastCleanup) < r.cleanupInterval {
		return
	}
	r.lastCleanup = now

	for sessionID, entry := range r.managers {
		if entry.manager == nil {
			delete(r.managers, sessionID)
			continue
		}
		// Hard TTL: force-remove entries older than maxEntryAge regardless of
		// task terminal state. This prevents indefinite accumulation from stuck tasks.
		if r.maxEntryAge > 0 && now.Sub(entry.lastAccess) > r.maxEntryAge {
			r.shutdown(entry.manager)
			delete(r.managers, sessionID)
			continue
		}
		if r.idleTTL > 0 && now.Sub(entry.lastAccess) < r.idleTTL {
			continue
		}
		if !managerTasksTerminal(entry.manager) {
			continue
		}
		r.shutdown(entry.manager)
		delete(r.managers, sessionID)
	}
}

func (r *backgroundTaskRegistry) touch(sessionID string) {
	if sessionID == "" {
		return
	}
	now := r.now()
	r.mu.Lock()
	entry, ok := r.managers[sessionID]
	if ok && entry.manager != nil {
		entry.lastAccess = now
		r.managers[sessionID] = entry
	}
	r.mu.Unlock()
}

func (r *backgroundTaskRegistry) now() time.Time {
	if r.nowFn != nil {
		return r.nowFn()
	}
	return time.Now()
}

func (r *backgroundTaskRegistry) shutdown(mgr *react.BackgroundTaskManager) {
	if mgr == nil {
		return
	}
	if r.shutdownFn != nil {
		r.shutdownFn(mgr)
		return
	}
	mgr.Shutdown()
}

func managerTasksTerminal(mgr *react.BackgroundTaskManager) bool {
	if mgr == nil {
		return true
	}
	for _, summary := range mgr.Status(nil) {
		switch summary.Status {
		case agent.BackgroundTaskStatusCompleted, agent.BackgroundTaskStatusFailed, agent.BackgroundTaskStatusCancelled:
		default:
			return false
		}
	}
	return true
}
