package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	serverPorts "alex/internal/delivery/server/ports"
	domain "alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agentports "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/analytics"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"
)

func (svc *TaskExecutionService) captureAnalytics(ctx context.Context, distinctID string, event string, props map[string]any) {
	if svc.analytics == nil {
		return
	}
	logger := logging.FromContext(ctx, svc.logger)

	payload := map[string]any{
		"source": "server",
	}

	for key, value := range props {
		if value == nil {
			continue
		}
		payload[key] = value
	}

	if err := svc.analytics.Capture(ctx, distinctID, event, payload); err != nil {
		logger.Debug("Analytics capture failed for event %s: %v", event, err)
	}
}

func (svc *TaskExecutionService) emitWorkflowInputReceivedEvent(ctx context.Context, sessionID, taskID, task string) {
	if svc.broadcaster == nil {
		return
	}

	parentRunID := id.ParentRunIDFromContext(ctx)
	level := agentports.GetOutputContext(ctx).Level
	attachments := appcontext.GetUserAttachments(ctx)
	var attachmentMap map[string]ports.Attachment
	if len(attachments) > 0 {
		attachmentMap = make(map[string]ports.Attachment, len(attachments))
		for _, att := range attachments {
			name := strings.TrimSpace(att.Name)
			if name == "" {
				continue
			}
			sanitized := att
			sanitized.Name = name
			if sanitized.Source == "" {
				sanitized.Source = "user_upload"
			}
			sanitized.URI = strings.TrimSpace(sanitized.URI)
			sanitized.Data = ""
			if sanitized.URI == "" || strings.HasPrefix(strings.ToLower(sanitized.URI), "data:") {
				continue
			}
			attachmentMap[name] = sanitized
		}
	}

	event := domain.NewInputReceivedEvent(level, sessionID, taskID, parentRunID, task, attachmentMap, time.Now())
	if logID := id.LogIDFromContext(ctx); logID != "" {
		event.SetLogID(logID)
	}
	svc.broadcaster.OnEvent(event)

	attachmentCount := len(attachmentMap)
	props := map[string]any{
		"run_id":           taskID,
		"session_id":       sessionID,
		"level":            level,
		"has_parent_run":   parentRunID != "",
		"has_attachments":  attachmentCount > 0,
		"attachment_count": attachmentCount,
	}
	if parentRunID != "" {
		props["parent_run_id"] = parentRunID
	}

	svc.captureAnalytics(ctx, sessionID, analytics.EventTaskExecutionStarted, props)
}

func (svc *TaskExecutionService) emitWorkflowResultCancelledEvent(ctx context.Context, task *serverPorts.Task, reason, requestedBy string) {
	if svc.broadcaster == nil || task == nil {
		return
	}

	outCtx := agentports.GetOutputContext(ctx)
	level := outCtx.Level
	if level == "" {
		level = agentports.LevelCore
	}

	event := domain.NewResultCancelledEvent(
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
		svc.broadcaster.OnEvent(envelope)
	}
	svc.broadcaster.OnEvent(event)
}

// CancelTask cancels a running task.
func (svc *TaskExecutionService) CancelTask(ctx context.Context, taskID string) error {
	task, err := svc.taskStore.Get(ctx, taskID)
	if err != nil {
		return err
	}
	logger := logging.FromContext(ctx, svc.logger)

	if task.Status != serverPorts.TaskStatusPending && task.Status != serverPorts.TaskStatusRunning {
		return ConflictError(fmt.Sprintf("cannot cancel task in status: %s", task.Status))
	}

	svc.cancelMu.RLock()
	cancelFunc, exists := svc.cancelFuncs[taskID]
	svc.cancelMu.RUnlock()

	status := "no_active_execution"
	if exists && cancelFunc != nil {
		logger.Info("Cancelling task execution: task_id=%s", taskID)
		cancelFunc(fmt.Errorf("task cancelled by user"))
		svc.emitWorkflowResultCancelledEvent(ctx, task, "cancelled", "user")
		status = "dispatched"
	} else {
		logger.Warn("No cancel function found for task %s; updating status only", taskID)
		if err := svc.taskStore.SetStatus(ctx, taskID, serverPorts.TaskStatusCancelled); err != nil {
			return err
		}
		if err := svc.taskStore.SetTerminationReason(ctx, taskID, serverPorts.TerminationReasonCancelled); err != nil {
			logger.Warn("Failed to set termination reason for task %s: %v", taskID, err)
		}
		svc.emitWorkflowResultCancelledEvent(ctx, task, "cancelled", "user")
	}

	props := map[string]any{
		"task_id":         task.ID,
		"session_id":      task.SessionID,
		"requested_by":    "user",
		"cancel_fn_found": exists && cancelFunc != nil,
		"status":          status,
	}
	svc.captureAnalytics(ctx, task.SessionID, analytics.EventTaskCancelRequested, props)

	return nil
}
