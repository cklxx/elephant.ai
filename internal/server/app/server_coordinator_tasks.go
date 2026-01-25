package app

import (
	"context"
	"fmt"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/analytics"
	serverPorts "alex/internal/server/ports"
)

func (s *ServerCoordinator) emitWorkflowResultCancelledEvent(ctx context.Context, task *serverPorts.Task, reason, requestedBy string) {
	if s.broadcaster == nil || task == nil {
		return
	}

	outCtx := agent.GetOutputContext(ctx)
	level := outCtx.Level
	if level == "" {
		level = agent.LevelCore
	}

	event := domain.NewWorkflowResultCancelledEvent(
		level,
		task.SessionID,
		task.ID,
		task.ParentTaskID,
		reason,
		requestedBy,
		time.Now(),
	)
	envelope := domain.NewWorkflowEnvelopeFromEvent(event, "workflow.result.cancelled")
	if envelope != nil {
		envelope.NodeKind = "result"
		envelope.Payload = map[string]any{
			"reason":       reason,
			"requested_by": requestedBy,
		}
		s.logger.Info("[CancelTask] Emitting workflow.result.cancelled envelope: sessionID=%s taskID=%s", task.SessionID, task.ID)
		s.broadcaster.OnEvent(envelope)
	}

	s.logger.Info("[CancelTask] Emitting workflow.result.cancelled event: sessionID=%s taskID=%s", task.SessionID, task.ID)
	s.broadcaster.OnEvent(event)
}

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

	status := "no_active_execution"
	if exists && cancelFunc != nil {
		s.logger.Info("[CancelTask] Cancelling task execution: taskID=%s", taskID)
		cancelFunc(fmt.Errorf("task cancelled by user"))
		s.emitWorkflowResultCancelledEvent(ctx, task, "cancelled", "user")
		status = "dispatched"
	} else {
		s.logger.Warn("[CancelTask] No cancel function found for taskID=%s, updating status only", taskID)
		// If no cancel function exists (task not started yet or already completed), just update status
		if err := s.taskStore.SetStatus(ctx, taskID, serverPorts.TaskStatusCancelled); err != nil {
			return err
		}
		if err := s.taskStore.SetTerminationReason(ctx, taskID, serverPorts.TerminationReasonCancelled); err != nil {
			s.logger.Warn("[CancelTask] Failed to set termination reason for taskID=%s: %v", taskID, err)
		}
		s.emitWorkflowResultCancelledEvent(ctx, task, "cancelled", "user")
	}

	props := map[string]any{
		"task_id":         task.ID,
		"session_id":      task.SessionID,
		"requested_by":    "user",
		"cancel_fn_found": exists && cancelFunc != nil,
		"status":          status,
	}
	s.captureAnalytics(ctx, task.SessionID, analytics.EventTaskCancelRequested, props)

	return nil
}
