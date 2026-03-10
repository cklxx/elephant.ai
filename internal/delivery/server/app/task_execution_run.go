package app

import (
	"context"
	"fmt"
	"os"
	"time"

	appcontext "alex/internal/app/agent/context"
	builtinshared "alex/internal/infra/tools/builtin/shared"
	serverPorts "alex/internal/delivery/server/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/analytics"
	"alex/internal/infra/observability"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// ExecuteTaskAsync executes a task asynchronously and streams events via SSE.
// Returns immediately with the task record, spawns background goroutine for execution.
func (svc *TaskExecutionService) ExecuteTaskAsync(ctx context.Context, task string, sessionID string, agentPreset string, toolPreset string) (*serverPorts.Task, error) {
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	logger := logging.FromContext(ctx, svc.logger)
	logger.Debug("ExecuteTaskAsync called: session_id=%s agent_preset=%s tool_preset=%s", sessionID, agentPreset, toolPreset)

	session, err := svc.agentCoordinator.GetSession(ctx, sessionID)
	if err != nil {
		logger.Error("Failed to get/create session: %v", err)
		return nil, fmt.Errorf("failed to get/create session: %w", err)
	}
	if svc.stateStore != nil {
		if err := svc.stateStore.Init(ctx, session.ID); err != nil {
			logger.Warn("Failed to initialize state store: %v", err)
		}
	}
	confirmedSessionID := session.ID

	taskID := id.NewRunID()
	ctx = id.WithRunID(ctx, taskID)

	svc.emitWorkflowInputReceivedEvent(ctx, confirmedSessionID, taskID, task)

	taskRecord, err := svc.taskStore.Create(ctx, confirmedSessionID, task, agentPreset, toolPreset)
	if err != nil {
		logger.Error("Failed to create task: %v", err)
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	parentRunID := id.ParentRunIDFromContext(ctx)
	if parentRunID != "" {
		taskRecord.ParentTaskID = parentRunID
	}

	taskSessionID := taskRecord.SessionID
	taskID = taskRecord.ID
	ctx = id.WithIDs(ctx, id.IDs{SessionID: confirmedSessionID, RunID: taskID, ParentRunID: parentRunID})

	if svc.broadcaster == nil {
		logger.Error("Broadcaster is nil")
		_ = svc.taskStore.SetError(context.Background(), taskID, UnavailableError("broadcaster not initialized"))
		return taskRecord, UnavailableError("broadcaster not initialized")
	}

	releaseAdmission, err := svc.acquireAdmission(ctx)
	if err != nil {
		admissionErr := UnavailableError("task admission timed out")
		logger.Warn("Admission wait failed for task %s: %v", taskID, err)
		_ = svc.taskStore.SetError(context.Background(), taskID, admissionErr)
		return taskRecord, admissionErr
	}

	leaseUntil := svc.nextLeaseDeadline(time.Now())
	claimed, err := svc.taskStore.TryClaimTask(ctx, taskID, svc.ownerID, leaseUntil)
	if err != nil {
		releaseAdmission()
		logger.Error("Failed to claim task %s: %v", taskID, err)
		_ = svc.taskStore.SetError(context.Background(), taskID, fmt.Errorf("failed to claim task: %w", err))
		return taskRecord, fmt.Errorf("claim task ownership: %w", err)
	}
	if !claimed {
		claimErr := ConflictError("task already claimed by another worker")
		logger.Warn("Claim rejected for task %s", taskID)
		releaseAdmission()
		return taskRecord, claimErr
	}

	taskCtx, cancelFunc := context.WithCancelCause(context.WithoutCancel(ctx))

	svc.cancelMu.Lock()
	svc.cancelFuncs[taskID] = cancelFunc
	svc.cancelMu.Unlock()

	taskCopy := *taskRecord
	async.Go(svc.logger, "server.executeTask", func() {
		svc.executeTaskInBackground(taskCtx, taskID, task, confirmedSessionID, agentPreset, toolPreset, releaseAdmission)
	})

	logger.Debug("Task created: task_id=%s session_id=%s", taskID, taskSessionID)
	return &taskCopy, nil
}

// executeTaskInBackground runs the actual task execution in a background goroutine.
func (svc *TaskExecutionService) executeTaskInBackground(
	ctx context.Context,
	taskID string,
	task string,
	sessionID string,
	agentPreset string,
	toolPreset string,
	releaseAdmission func(),
) {
	svc.taskWg.Add(1)
	defer svc.taskWg.Done()

	logger := logging.FromContext(ctx, svc.logger)
	stopLeaseRenew := svc.startTaskLeaseRenewer(ctx, taskID)

	defer func() {
		stopLeaseRenew()
		if releaseAdmission != nil {
			releaseAdmission()
		}
		if err := svc.taskStore.ReleaseTaskLease(context.Background(), taskID, svc.ownerID); err != nil {
			logger.Warn("Failed to release lease for task %s: %v", taskID, err)
		}

		svc.cancelMu.Lock()
		delete(svc.cancelFuncs, taskID)
		svc.cancelMu.Unlock()

		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("panic in task execution (task_id=%s, session_id=%s): %v", taskID, sessionID, r)
			logger.Error("%s", errMsg)
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)
			_ = svc.taskStore.SetError(ctx, taskID, fmt.Errorf("panic: %v", r))
		}
	}()

	if releaseAdmission == nil {
		acquiredRelease, err := svc.acquireAdmission(ctx)
		if err != nil {
			if ctx.Err() != nil {
				_ = svc.taskStore.SetStatus(context.Background(), taskID, serverPorts.TaskStatusCancelled)
				_ = svc.taskStore.SetTerminationReason(context.Background(), taskID, serverPorts.TerminationReasonCancelled)
			} else {
				_ = svc.taskStore.SetError(context.Background(), taskID, UnavailableError("task admission failed"))
			}
			return
		}
		releaseAdmission = acquiredRelease
	}

	logger.Debug("Starting task execution: task_id=%s session_id=%s", taskID, sessionID)

	parentRunID := id.ParentRunIDFromContext(ctx)
	startTime := time.Now()
	status := "success"
	var spanErr error
	if svc.obs != nil {
		if svc.obs.Tracer != nil {
			attrs := append(observability.SessionAttrs(sessionID), attribute.String(observability.AttrRunID, taskID))
			ctxWithSpan, span := svc.obs.Tracer.StartSpan(ctx, observability.SpanSessionSolveTask, attrs...)
			ctx = ctxWithSpan
			defer func() {
				if spanErr != nil {
					span.RecordError(spanErr)
					span.SetStatus(codes.Error, spanErr.Error())
				}
				span.End()
			}()
		}
		svc.obs.Metrics.IncrementActiveSessions(ctx)
		defer svc.obs.Metrics.DecrementActiveSessions(ctx)
		defer func() {
			svc.obs.Metrics.RecordTaskExecution(ctx, status, time.Since(startTime))
		}()
	}

	if svc.agentCoordinator == nil {
		errMsg := fmt.Sprintf("agent coordinator is nil (task_id=%s)", taskID)
		logger.Error("%s", errMsg)
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)
		err := UnavailableError("agent coordinator not initialized")
		spanErr = err
		status = "error"
		_ = svc.taskStore.SetError(ctx, taskID, err)
		return
	}

	ctx = svc.broadcaster.SetSessionContext(ctx, sessionID)

	if svc.progressTracker != nil {
		svc.progressTracker.RegisterRunSession(sessionID, taskID)
		defer svc.progressTracker.UnregisterRunSession(sessionID)
	}

	_ = svc.taskStore.SetStatus(ctx, taskID, serverPorts.TaskStatusRunning)

	if agentPreset != "" || toolPreset != "" {
		ctx = context.WithValue(ctx, appcontext.PresetContextKey{}, appcontext.PresetConfig{
			AgentPreset: agentPreset,
			ToolPreset:  toolPreset,
		})
		logger.Debug("Using presets: agent=%s tool=%s", agentPreset, toolPreset)
	}

	var listener agent.EventListener = svc.broadcaster
	if svc.progressTracker != nil {
		listener = NewMultiEventListener(svc.broadcaster, svc.progressTracker)
	}

	ctx = builtinshared.WithParentListener(ctx, listener)
	result, err := svc.agentCoordinator.ExecuteTask(ctx, task, sessionID, listener)

	if ctx.Err() != nil {
		logger.Info("Task cancelled: task_id=%s session_id=%s reason=%v", taskID, sessionID, context.Cause(ctx))
		status = "cancelled"
		if cause := context.Cause(ctx); cause != nil {
			spanErr = cause
		}

		cause := context.Cause(ctx)
		var terminationReason serverPorts.TerminationReason
		if cause != nil {
			switch cause {
			case context.DeadlineExceeded:
				terminationReason = serverPorts.TerminationReasonTimeout
			case context.Canceled:
				terminationReason = serverPorts.TerminationReasonCancelled
			default:
				terminationReason = serverPorts.TerminationReasonCancelled
			}
		} else {
			terminationReason = serverPorts.TerminationReasonCancelled
		}

		_ = svc.taskStore.SetStatus(ctx, taskID, serverPorts.TaskStatusCancelled)
		_ = svc.taskStore.SetTerminationReason(context.Background(), taskID, terminationReason)
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
		svc.captureAnalytics(ctx, sessionID, analytics.EventTaskExecutionCancelled, props)
		return
	}

	if err != nil {
		errMsg := fmt.Sprintf("task execution failed (task_id=%s, session_id=%s): %v", taskID, sessionID, err)
		status = "error"
		spanErr = err
		logger.Error("%s", errMsg)
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)
		_ = svc.taskStore.SetError(ctx, taskID, err)
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
		svc.captureAnalytics(ctx, sessionID, analytics.EventTaskExecutionFailed, props)
		return
	}

	_ = svc.taskStore.SetResult(ctx, taskID, result)

	logger.Info("Task execution completed: task_id=%s", taskID)

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
	svc.captureAnalytics(ctx, sessionID, analytics.EventTaskExecutionCompleted, props)
}
