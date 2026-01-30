package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/analytics"
	"alex/internal/async"
	"alex/internal/logging"
	"alex/internal/observability"
	serverPorts "alex/internal/server/ports"
	"alex/internal/tools/builtin/shared"
	id "alex/internal/utils/id"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// ExecuteTaskAsync executes a task asynchronously and streams events via SSE
// Returns immediately with the task record, spawns background goroutine for execution
func (s *ServerCoordinator) ExecuteTaskAsync(ctx context.Context, task string, sessionID string, agentPreset string, toolPreset string) (*serverPorts.Task, error) {
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	logger := logging.FromContext(ctx, s.logger)
	logger.Info("[ServerCoordinator] ExecuteTaskAsync called: task='%s', sessionID='%s', agentPreset='%s', toolPreset='%s'", task, sessionID, agentPreset, toolPreset)

	// CRITICAL FIX: Get or create session SYNCHRONOUSLY before creating task
	// This ensures we have a confirmed session ID for the task record and broadcaster mapping
	session, err := s.agentCoordinator.GetSession(ctx, sessionID)
	if err != nil {
		logger.Error("[ServerCoordinator] Failed to get/create session: %v", err)
		return nil, fmt.Errorf("failed to get/create session: %w", err)
	}
	if s.stateStore != nil {
		if err := s.stateStore.Init(ctx, session.ID); err != nil {
			logger.Warn("[ServerCoordinator] Failed to initialize state store: %v", err)
		}
	}
	confirmedSessionID := session.ID
	logger.Info("[ServerCoordinator] Session confirmed: %s (original: '%s')", confirmedSessionID, sessionID)

	// Preallocate a run ID so we can emit workflow.input.received before hitting slower stores.
	taskID := id.NewRunID()
	ctx = id.WithRunID(ctx, taskID)

	// Emit workflow.input.received event immediately so the frontend gets instant feedback.
	s.emitWorkflowInputReceivedEvent(ctx, confirmedSessionID, taskID, task)

	// Create task record with confirmed session ID and preallocated task ID from context
	taskRecord, err := s.taskStore.Create(ctx, confirmedSessionID, task, agentPreset, toolPreset)
	if err != nil {
		logger.Error("[ServerCoordinator] Failed to create task: %v", err)
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	parentRunID := id.ParentRunIDFromContext(ctx)
	if parentRunID != "" {
		taskRecord.ParentTaskID = parentRunID
	}

	taskSessionID := taskRecord.SessionID
	ctx = id.WithIDs(ctx, id.IDs{SessionID: confirmedSessionID, RunID: taskRecord.ID, ParentRunID: parentRunID})

	// Verify broadcaster is initialized
	if s.broadcaster == nil {
		logger.Error("[ServerCoordinator] Broadcaster is nil!")
		_ = s.taskStore.SetError(ctx, taskRecord.ID, fmt.Errorf("broadcaster not initialized"))
		return taskRecord, fmt.Errorf("broadcaster not initialized")
	}

	// Create a detached context so the task keeps running after the HTTP handler returns
	// while keeping request-scoped values for logging/metrics via context.WithoutCancel
	// Explicit cancellation still flows through the stored cancel function
	taskCtx, cancelFunc := context.WithCancelCause(context.WithoutCancel(ctx))

	// Store cancel function to enable explicit cancellation via CancelTask API
	s.cancelMu.Lock()
	s.cancelFuncs[taskID] = cancelFunc
	s.cancelMu.Unlock()

	// Create a snapshot of the task record before the background goroutine begins
	// mutating the original entry stored in the task store. Returning a copy avoids
	// exposing callers to shared mutable state and prevents data races when the
	// goroutine later calls SetResult/SetStatus on the same underlying task.
	taskCopy := *taskRecord
	async.Go(s.logger, "server.executeTask", func() {
		s.executeTaskInBackground(taskCtx, taskID, task, confirmedSessionID, agentPreset, toolPreset)
	})

	logger.Info("[ServerCoordinator] Task created: taskID=%s, sessionID=%s, returning immediately", taskID, taskSessionID)
	return &taskCopy, nil
}

// executeTaskInBackground runs the actual task execution in a background goroutine
func (s *ServerCoordinator) executeTaskInBackground(ctx context.Context, taskID string, task string, sessionID string, agentPreset string, toolPreset string) {
	logger := logging.FromContext(ctx, s.logger)
	defer func() {
		// Clean up cancel function from map
		s.cancelMu.Lock()
		delete(s.cancelFuncs, taskID)
		s.cancelMu.Unlock()

		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("[Background] PANIC in task execution (taskID=%s, sessionID=%s): %v", taskID, sessionID, r)

			// Log to file (use %s to avoid linter warning)
			logger.Error("%s", errMsg)

			// CRITICAL: Also print to stderr so server operator can see it
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)

			// Update task status to error
			_ = s.taskStore.SetError(ctx, taskID, fmt.Errorf("panic: %v", r))
		}
	}()

	logger.Info("[Background] Starting task execution: taskID=%s, sessionID=%s", taskID, sessionID)

	parentRunID := id.ParentRunIDFromContext(ctx)
	startTime := time.Now()
	status := "success"
	var spanErr error
	if s.obs != nil {
		if s.obs.Tracer != nil {
			attrs := append(observability.SessionAttrs(sessionID), attribute.String(observability.AttrRunID, taskID))
			ctxWithSpan, span := s.obs.Tracer.StartSpan(ctx, observability.SpanSessionSolveTask, attrs...)
			ctx = ctxWithSpan
			defer func() {
				if spanErr != nil {
					span.RecordError(spanErr)
					span.SetStatus(codes.Error, spanErr.Error())
				}
				span.End()
			}()
		}
		s.obs.Metrics.IncrementActiveSessions(ctx)
		defer s.obs.Metrics.DecrementActiveSessions(ctx)
		defer func() {
			s.obs.Metrics.RecordTaskExecution(ctx, status, time.Since(startTime))
		}()
	}

	// Defensive validation: Ensure agentCoordinator is initialized
	if s.agentCoordinator == nil {
		errMsg := fmt.Sprintf("[Background] CRITICAL: agentCoordinator is nil (taskID=%s)", taskID)
		logger.Error("%s", errMsg)
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)
		err := fmt.Errorf("agent coordinator not initialized")
		spanErr = err
		status = "error"
		_ = s.taskStore.SetError(ctx, taskID, err)
		return
	}

	// Set session context in broadcaster
	ctx = s.broadcaster.SetSessionContext(ctx, sessionID)

	// Register run-session mapping for progress tracking
	if s.progressTracker != nil {
		s.progressTracker.RegisterRunSession(sessionID, taskID)
		defer s.progressTracker.UnregisterRunSession(sessionID)
	}

	// Update task status to running
	_ = s.taskStore.SetStatus(ctx, taskID, serverPorts.TaskStatusRunning)

	// Add presets to context for the agent coordinator
	if agentPreset != "" || toolPreset != "" {
		ctx = context.WithValue(ctx, appcontext.PresetContextKey{}, appcontext.PresetConfig{
			AgentPreset: agentPreset,
			ToolPreset:  toolPreset,
		})
		logger.Info("[Background] Using presets: agent=%s, tool=%s", agentPreset, toolPreset)
	}

	// Execute task with broadcaster as event listener
	logger.Info("[Background] Calling AgentCoordinator.ExecuteTask...")

	// Compose the event listener: broadcaster for SSE clients + optional progress tracker
	var listener agent.EventListener = s.broadcaster
	if s.progressTracker != nil {
		listener = NewMultiEventListener(s.broadcaster, s.progressTracker)
	}

	// Ensure subagent tool invocations forward their events to the main listener
	ctx = shared.WithParentListener(ctx, listener)

	result, err := s.agentCoordinator.ExecuteTask(ctx, task, sessionID, listener)

	// Check if context was cancelled
	if ctx.Err() != nil {
		logger.Info("[Background] Task cancelled: taskID=%s, sessionID=%s, reason=%v", taskID, sessionID, context.Cause(ctx))
		status = "cancelled"
		if cause := context.Cause(ctx); cause != nil {
			spanErr = cause
		}

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
		props := map[string]any{
			"run_id":             taskID,
			"session_id":         sessionID,
			"termination_reason": string(terminationReason),
			"duration_ms":        time.Since(startTime).Milliseconds(),
		}
		if parentRunID != "" {
			props["parent_run_id"] = parentRunID
		}
		if agentPreset != "" {
			props["agent_preset"] = agentPreset
		}
		if toolPreset != "" {
			props["tool_preset"] = toolPreset
		}
		s.captureAnalytics(ctx, sessionID, analytics.EventTaskExecutionCancelled, props)
		return
	}

	if err != nil {
		errMsg := fmt.Sprintf("[Background] Task execution failed (taskID=%s, sessionID=%s): %v", taskID, sessionID, err)
		status = "error"
		spanErr = err

		// Log to file (use %s to avoid linter warning)
		logger.Error("%s", errMsg)

		// CRITICAL: Also print to stderr so server operator can see it
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)

		// Update task status
		_ = s.taskStore.SetError(ctx, taskID, err)
		props := map[string]any{
			"run_id":      taskID,
			"session_id":  sessionID,
			"duration_ms": time.Since(startTime).Milliseconds(),
			"error":       err.Error(),
		}
		if parentRunID != "" {
			props["parent_run_id"] = parentRunID
		}
		if agentPreset != "" {
			props["agent_preset"] = agentPreset
		}
		if toolPreset != "" {
			props["tool_preset"] = toolPreset
		}
		s.captureAnalytics(ctx, sessionID, analytics.EventTaskExecutionFailed, props)
		return
	}

	// Update task with result
	_ = s.taskStore.SetResult(ctx, taskID, result)

	logger.Info("[Background] Task execution completed: taskID=%s", taskID)

	props := map[string]any{
		"run_id":      taskID,
		"session_id":  sessionID,
		"duration_ms": time.Since(startTime).Milliseconds(),
		"iterations":  result.Iterations,
	}
	if parentRunID != "" {
		props["parent_run_id"] = parentRunID
	}
	if agentPreset != "" {
		props["agent_preset"] = agentPreset
	}
	if toolPreset != "" {
		props["tool_preset"] = toolPreset
	}
	if result.StopReason != "" {
		props["stop_reason"] = result.StopReason
	}
	s.captureAnalytics(ctx, sessionID, analytics.EventTaskExecutionCompleted, props)
}

func (s *ServerCoordinator) captureAnalytics(ctx context.Context, distinctID string, event string, props map[string]any) {
	if s.analytics == nil {
		return
	}
	logger := logging.FromContext(ctx, s.logger)

	payload := map[string]any{
		"source": "server",
	}

	for key, value := range props {
		if value == nil {
			continue
		}
		payload[key] = value
	}

	if err := s.analytics.Capture(ctx, distinctID, event, payload); err != nil {
		logger.Debug("[Analytics] failed to capture event %s: %v", event, err)
	}
}

func (s *ServerCoordinator) emitWorkflowInputReceivedEvent(ctx context.Context, sessionID, taskID, task string) {
	if s.broadcaster == nil {
		return
	}
	logger := logging.FromContext(ctx, s.logger)

	parentRunID := id.ParentRunIDFromContext(ctx)
	level := agent.GetOutputContext(ctx).Level
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

	event := domain.NewWorkflowInputReceivedEvent(level, sessionID, taskID, parentRunID, task, attachmentMap, time.Now())
	if logID := id.LogIDFromContext(ctx); logID != "" {
		event.SetLogID(logID)
	}
	logger.Debug("[Background] Emitting workflow.input.received event for session=%s run=%s", sessionID, taskID)
	s.broadcaster.OnEvent(event)

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

	s.captureAnalytics(ctx, sessionID, analytics.EventTaskExecutionStarted, props)
}
