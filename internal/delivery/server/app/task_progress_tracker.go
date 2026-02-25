package app

import (
	"context"
	"sync"

	serverPorts "alex/internal/delivery/server/ports"
	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"
)

// RunTracker checks if a session has an active run.
type RunTracker interface {
	GetActiveRunID(sessionID string) string
}

// TaskProgressTracker implements agent.EventListener, updating task progress
// in TaskStore based on workflow events. It is decoupled from EventBroadcaster
// and composed with it via MultiEventListener at wiring time.
type TaskProgressTracker struct {
	taskStore    serverPorts.TaskStore
	sessionToRun map[string]string // sessionID → runID
	mu           sync.RWMutex
	logger       logging.Logger
}

// NewTaskProgressTracker creates a tracker that writes progress to the given task store.
func NewTaskProgressTracker(taskStore serverPorts.TaskStore) *TaskProgressTracker {
	return &TaskProgressTracker{
		taskStore:    taskStore,
		sessionToRun: make(map[string]string),
		logger:       logging.NewComponentLogger("TaskProgressTracker"),
	}
}

// OnEvent implements agent.EventListener.
func (t *TaskProgressTracker) OnEvent(event agent.AgentEvent) {
	if event == nil {
		return
	}

	base := BaseAgentEvent(event)
	if base == nil {
		return
	}

	sessionID := base.GetSessionID()
	if sessionID == "" {
		return
	}

	t.mu.RLock()
	taskID, ok := t.sessionToRun[sessionID]
	t.mu.RUnlock()

	if !ok {
		return
	}

	ctx := id.WithSessionID(context.Background(), sessionID)
	ctx = id.WithRunID(ctx, taskID)

	switch e := base.(type) {
	case *domain.WorkflowEventEnvelope:
		iter := intFromPayload(e.Payload, "iteration")
		switch e.EventType() {
		case types.EventNodeStarted:
			task, err := t.taskStore.Get(ctx, taskID)
			if err == nil {
				_ = t.taskStore.UpdateProgress(ctx, taskID, iter, task.TokensUsed)
			}
		case types.EventNodeCompleted:
			tokens := intFromPayload(e.Payload, "tokens_used")
			_ = t.taskStore.UpdateProgress(ctx, taskID, iter, tokens)
		case types.EventResultFinal:
			totalIters := intFromPayload(e.Payload, "total_iterations")
			totalTokens := intFromPayload(e.Payload, "total_tokens")
			_ = t.taskStore.UpdateProgress(ctx, taskID, totalIters, totalTokens)
		}
	case *domain.Event:
		switch e.Kind {
		case types.EventResultFinal:
			_ = t.taskStore.UpdateProgress(ctx, taskID, e.Data.TotalIterations, e.Data.TotalTokens)
		case types.EventNodeCompleted:
			_ = t.taskStore.UpdateProgress(ctx, taskID, e.Data.Iteration, e.Data.TokensUsed)
		case types.EventNodeStarted:
			task, err := t.taskStore.Get(ctx, taskID)
			if err == nil {
				_ = t.taskStore.UpdateProgress(ctx, taskID, e.Data.Iteration, task.TokensUsed)
			}
		}
	}
}

// RegisterRunSession associates a runID with a sessionID for progress tracking.
func (t *TaskProgressTracker) RegisterRunSession(sessionID, runID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionToRun[sessionID] = runID
	t.logger.Info("Registered run-session mapping: sessionID=%s, runID=%s", sessionID, runID)
}

// UnregisterRunSession removes the runID-sessionID mapping.
func (t *TaskProgressTracker) UnregisterRunSession(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.sessionToRun, sessionID)
	t.logger.Info("Unregistered run-session mapping: sessionID=%s", sessionID)
}

// GetActiveRunID returns the runID currently associated with the given session,
// or an empty string if no run is active.
func (t *TaskProgressTracker) GetActiveRunID(sessionID string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.sessionToRun[sessionID]
}
