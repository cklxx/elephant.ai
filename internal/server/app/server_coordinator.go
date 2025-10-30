package app

import (
	"context"
	"fmt"
	"os"
	"sync"

	agentApp "alex/internal/agent/app"
	"alex/internal/agent/ports"
	serverPorts "alex/internal/server/ports"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

// AgentExecutor defines the interface for agent task execution
// This allows for easier testing and mocking
type AgentExecutor interface {
	GetSession(ctx context.Context, id string) (*ports.Session, error)
	ExecuteTask(ctx context.Context, task string, sessionID string, listener ports.EventListener) (*ports.TaskResult, error)
}

// Ensure AgentCoordinator implements AgentExecutor
var _ AgentExecutor = (*agentApp.AgentCoordinator)(nil)

// ServerCoordinator coordinates task execution for the server
// It wraps AgentExecutor and integrates with EventBroadcaster
type ServerCoordinator struct {
	agentCoordinator AgentExecutor
	broadcaster      *EventBroadcaster
	sessionStore     ports.SessionStore
	taskStore        serverPorts.TaskStore
	logger           *utils.Logger

	// Cancel function map for task cancellation support
	cancelFuncs map[string]context.CancelCauseFunc
	cancelMu    sync.RWMutex
}

// NewServerCoordinator creates a new server coordinator
func NewServerCoordinator(
	agentCoordinator AgentExecutor,
	broadcaster *EventBroadcaster,
	sessionStore ports.SessionStore,
	taskStore serverPorts.TaskStore,
) *ServerCoordinator {
	return &ServerCoordinator{
		agentCoordinator: agentCoordinator,
		broadcaster:      broadcaster,
		sessionStore:     sessionStore,
		taskStore:        taskStore,
		logger:           utils.NewComponentLogger("ServerCoordinator"),
		cancelFuncs:      make(map[string]context.CancelCauseFunc),
	}
}

// ExecuteTaskAsync executes a task asynchronously and streams events via SSE
// Returns immediately with the task record, spawns background goroutine for execution
func (s *ServerCoordinator) ExecuteTaskAsync(ctx context.Context, task string, sessionID string, agentPreset string, toolPreset string) (*serverPorts.Task, error) {
	s.logger.Info("[ServerCoordinator] ExecuteTaskAsync called: task='%s', sessionID='%s', agentPreset='%s', toolPreset='%s'", task, sessionID, agentPreset, toolPreset)

	// CRITICAL FIX: Get or create session SYNCHRONOUSLY before creating task
	// This ensures we have a confirmed session ID for the task record and broadcaster mapping
	session, err := s.agentCoordinator.GetSession(ctx, sessionID)
	if err != nil {
		s.logger.Error("[ServerCoordinator] Failed to get/create session: %v", err)
		return nil, fmt.Errorf("failed to get/create session: %w", err)
	}
	confirmedSessionID := session.ID
	s.logger.Info("[ServerCoordinator] Session confirmed: %s (original: '%s')", confirmedSessionID, sessionID)

	// Create task record with confirmed session ID
	taskRecord, err := s.taskStore.Create(ctx, confirmedSessionID, task, agentPreset, toolPreset)
	if err != nil {
		s.logger.Error("[ServerCoordinator] Failed to create task: %v", err)
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	parentTaskID := id.ParentTaskIDFromContext(ctx)
	if parentTaskID != "" {
		taskRecord.ParentTaskID = parentTaskID
	}

	ctx = id.WithIDs(ctx, id.IDs{SessionID: confirmedSessionID, TaskID: taskRecord.ID, ParentTaskID: parentTaskID})

	// Verify broadcaster is initialized
	if s.broadcaster == nil {
		s.logger.Error("[ServerCoordinator] Broadcaster is nil!")
		_ = s.taskStore.SetError(ctx, taskRecord.ID, fmt.Errorf("broadcaster not initialized"))
		return taskRecord, fmt.Errorf("broadcaster not initialized")
	}

	// Create a detached context so the task keeps running after the HTTP handler returns
	// while keeping request-scoped values for logging/metrics via context.WithoutCancel
	// Explicit cancellation still flows through the stored cancel function
	taskCtx, cancelFunc := context.WithCancelCause(context.WithoutCancel(ctx))

	// Store cancel function to enable explicit cancellation via CancelTask API
	s.cancelMu.Lock()
	s.cancelFuncs[taskRecord.ID] = cancelFunc
	s.cancelMu.Unlock()

	// Spawn background goroutine to execute task with confirmed session ID
	go s.executeTaskInBackground(taskCtx, taskRecord.ID, task, confirmedSessionID, agentPreset, toolPreset)

	// Return immediately with the task record (now has correct session_id)
	s.logger.Info("[ServerCoordinator] Task created: taskID=%s, sessionID=%s, returning immediately", taskRecord.ID, taskRecord.SessionID)
	return taskRecord, nil
}

// executeTaskInBackground runs the actual task execution in a background goroutine
func (s *ServerCoordinator) executeTaskInBackground(ctx context.Context, taskID string, task string, sessionID string, agentPreset string, toolPreset string) {
	defer func() {
		// Clean up cancel function from map
		s.cancelMu.Lock()
		delete(s.cancelFuncs, taskID)
		s.cancelMu.Unlock()

		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("[Background] PANIC in task execution (taskID=%s, sessionID=%s): %v", taskID, sessionID, r)

			// Log to file (use %s to avoid linter warning)
			s.logger.Error("%s", errMsg)

			// CRITICAL: Also print to stderr so server operator can see it
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)

			// Update task status to error
			_ = s.taskStore.SetError(ctx, taskID, fmt.Errorf("panic: %v", r))
		}
	}()

	s.logger.Info("[Background] Starting task execution: taskID=%s, sessionID=%s", taskID, sessionID)

	// Defensive validation: Ensure agentCoordinator is initialized
	if s.agentCoordinator == nil {
		errMsg := fmt.Sprintf("[Background] CRITICAL: agentCoordinator is nil (taskID=%s)", taskID)
		s.logger.Error("%s", errMsg)
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)
		_ = s.taskStore.SetError(ctx, taskID, fmt.Errorf("agent coordinator not initialized"))
		return
	}

	// Set session context in broadcaster
	ctx = s.broadcaster.SetSessionContext(ctx, sessionID)

	// Register task-session mapping for progress tracking
	s.broadcaster.RegisterTaskSession(sessionID, taskID)
	defer s.broadcaster.UnregisterTaskSession(sessionID)

	// Update task status to running
	_ = s.taskStore.SetStatus(ctx, taskID, serverPorts.TaskStatusRunning)

	// Add presets to context for the agent coordinator
	if agentPreset != "" || toolPreset != "" {
		ctx = context.WithValue(ctx, agentApp.PresetContextKey{}, agentApp.PresetConfig{
			AgentPreset: agentPreset,
			ToolPreset:  toolPreset,
		})
		s.logger.Info("[Background] Using presets: agent=%s, tool=%s", agentPreset, toolPreset)
	}

	// Execute task with broadcaster as event listener
	s.logger.Info("[Background] Calling AgentCoordinator.ExecuteTask...")
	result, err := s.agentCoordinator.ExecuteTask(ctx, task, sessionID, s.broadcaster)

	// Check if context was cancelled
	if ctx.Err() != nil {
		s.logger.Info("[Background] Task cancelled: taskID=%s, sessionID=%s, reason=%v", taskID, sessionID, context.Cause(ctx))

		// Determine termination reason from context
		cause := context.Cause(ctx)
		var terminationReason serverPorts.TerminationReason
		if cause != nil {
			switch cause {
			case context.DeadlineExceeded:
				terminationReason = serverPorts.TerminationReasonTimeout
			case context.Canceled:
				terminationReason = serverPorts.TerminationReasonCancelled
			default:
				// Check if it's a custom cancellation reason
				terminationReason = serverPorts.TerminationReasonCancelled
			}
		} else {
			terminationReason = serverPorts.TerminationReasonCancelled
		}

		// Update task status to cancelled with termination reason
		_ = s.taskStore.SetStatus(ctx, taskID, serverPorts.TaskStatusCancelled)
		_ = s.taskStore.SetTerminationReason(context.Background(), taskID, terminationReason)
		return
	}

	if err != nil {
		errMsg := fmt.Sprintf("[Background] Task execution failed (taskID=%s, sessionID=%s): %v", taskID, sessionID, err)

		// Log to file (use %s to avoid linter warning)
		s.logger.Error("%s", errMsg)

		// CRITICAL: Also print to stderr so server operator can see it
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)

		// Update task status
		_ = s.taskStore.SetError(ctx, taskID, err)
		return
	}

	// Update task with result
	_ = s.taskStore.SetResult(ctx, taskID, result)

	s.logger.Info("[Background] Task execution completed: taskID=%s, sessionID=%s, iterations=%d", taskID, result.SessionID, result.Iterations)
}

// GetSession retrieves a session by ID
func (s *ServerCoordinator) GetSession(ctx context.Context, id string) (*ports.Session, error) {
	return s.sessionStore.Get(ctx, id)
}

// ListSessions returns all session IDs
func (s *ServerCoordinator) ListSessions(ctx context.Context) ([]string, error) {
	return s.sessionStore.List(ctx)
}

// DeleteSession removes a session
func (s *ServerCoordinator) DeleteSession(ctx context.Context, id string) error {
	return s.sessionStore.Delete(ctx, id)
}

// GetBroadcaster returns the event broadcaster
func (s *ServerCoordinator) GetBroadcaster() *EventBroadcaster {
	return s.broadcaster
}

// Task management methods

// GetTask retrieves a task by ID
func (s *ServerCoordinator) GetTask(ctx context.Context, taskID string) (*serverPorts.Task, error) {
	return s.taskStore.Get(ctx, taskID)
}

// ListTasks returns all tasks with pagination
func (s *ServerCoordinator) ListTasks(ctx context.Context, limit int, offset int) ([]*serverPorts.Task, int, error) {
	return s.taskStore.List(ctx, limit, offset)
}

// ListSessionTasks returns all tasks for a session
func (s *ServerCoordinator) ListSessionTasks(ctx context.Context, sessionID string) ([]*serverPorts.Task, error) {
	return s.taskStore.ListBySession(ctx, sessionID)
}

// CancelTask cancels a running task
func (s *ServerCoordinator) CancelTask(ctx context.Context, taskID string) error {
	task, err := s.taskStore.Get(ctx, taskID)
	if err != nil {
		return err
	}

	// Only allow cancelling pending or running tasks
	if task.Status != serverPorts.TaskStatusPending && task.Status != serverPorts.TaskStatusRunning {
		return fmt.Errorf("cannot cancel task in status: %s", task.Status)
	}

	// Call the cancel function if it exists
	s.cancelMu.RLock()
	cancelFunc, exists := s.cancelFuncs[taskID]
	s.cancelMu.RUnlock()

	if exists && cancelFunc != nil {
		s.logger.Info("[CancelTask] Cancelling task execution: taskID=%s", taskID)
		cancelFunc(fmt.Errorf("task cancelled by user"))
	} else {
		s.logger.Warn("[CancelTask] No cancel function found for taskID=%s, updating status only", taskID)
		// If no cancel function exists (task not started yet or already completed), just update status
		return s.taskStore.SetStatus(ctx, taskID, serverPorts.TaskStatusCancelled)
	}

	return nil
}

// ForkSession creates a new session as a fork of an existing one
func (s *ServerCoordinator) ForkSession(ctx context.Context, sessionID string) (*ports.Session, error) {
	// Get original session
	originalSession, err := s.sessionStore.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("original session not found: %w", err)
	}

	// Create new session
	newSession, err := s.sessionStore.Create(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create forked session: %w", err)
	}

	// Copy messages from original session
	newSession.Messages = make([]ports.Message, len(originalSession.Messages))
	copy(newSession.Messages, originalSession.Messages)

	// Copy metadata and add fork information
	newSession.Metadata = make(map[string]string)
	for k, v := range originalSession.Metadata {
		newSession.Metadata[k] = v
	}
	newSession.Metadata["forked_from"] = sessionID

	// Save the forked session
	if err := s.sessionStore.Save(ctx, newSession); err != nil {
		return nil, fmt.Errorf("failed to save forked session: %w", err)
	}

	return newSession, nil
}
