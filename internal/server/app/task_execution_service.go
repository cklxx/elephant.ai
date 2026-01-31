package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
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

// TaskExecutionService handles asynchronous task execution, cancellation,
// and task store queries. Extracted from ServerCoordinator.
type TaskExecutionService struct {
	agentCoordinator AgentExecutor
	broadcaster      *EventBroadcaster
	progressTracker  *TaskProgressTracker
	taskStore        serverPorts.TaskStore
	stateStore       interface {
		Init(ctx context.Context, sessionID string) error
	}
	analytics analytics.Client
	obs       *observability.Observability
	logger    logging.Logger

	cancelFuncs map[string]context.CancelCauseFunc
	cancelMu    sync.RWMutex
}

// NewTaskExecutionService creates a new task execution service.
func NewTaskExecutionService(
	agentCoordinator AgentExecutor,
	broadcaster *EventBroadcaster,
	taskStore serverPorts.TaskStore,
	opts ...TaskExecutionServiceOption,
) *TaskExecutionService {
	svc := &TaskExecutionService{
		agentCoordinator: agentCoordinator,
		broadcaster:      broadcaster,
		taskStore:        taskStore,
		analytics:        analytics.NewNoopClient(),
		logger:           logging.NewComponentLogger("TaskExecutionService"),
		cancelFuncs:      make(map[string]context.CancelCauseFunc),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

// TaskExecutionServiceOption configures optional behavior.
type TaskExecutionServiceOption func(*TaskExecutionService)

// WithTaskAnalytics attaches an analytics client.
func WithTaskAnalytics(client analytics.Client) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		if client == nil {
			svc.analytics = analytics.NewNoopClient()
			return
		}
		svc.analytics = client
	}
}

// WithTaskObservability wires observability.
func WithTaskObservability(obs *observability.Observability) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		svc.obs = obs
	}
}

// WithTaskProgressTracker wires a progress tracker.
func WithTaskProgressTracker(tracker *TaskProgressTracker) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		svc.progressTracker = tracker
	}
}

// WithTaskStateStore wires a state store for session init.
func WithTaskStateStore(store interface {
	Init(ctx context.Context, sessionID string) error
}) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		svc.stateStore = store
	}
}

// ExecuteTaskAsync executes a task asynchronously and streams events via SSE.
// Returns immediately with the task record, spawns background goroutine for execution.
func (svc *TaskExecutionService) ExecuteTaskAsync(ctx context.Context, task string, sessionID string, agentPreset string, toolPreset string) (*serverPorts.Task, error) {
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	logger := logging.FromContext(ctx, svc.logger)
	logger.Info("[TaskExecutionService] ExecuteTaskAsync called: task='%s', sessionID='%s', agentPreset='%s', toolPreset='%s'", task, sessionID, agentPreset, toolPreset)

	session, err := svc.agentCoordinator.GetSession(ctx, sessionID)
	if err != nil {
		logger.Error("[TaskExecutionService] Failed to get/create session: %v", err)
		return nil, fmt.Errorf("failed to get/create session: %w", err)
	}
	if svc.stateStore != nil {
		if err := svc.stateStore.Init(ctx, session.ID); err != nil {
			logger.Warn("[TaskExecutionService] Failed to initialize state store: %v", err)
		}
	}
	confirmedSessionID := session.ID
	logger.Info("[TaskExecutionService] Session confirmed: %s (original: '%s')", confirmedSessionID, sessionID)

	taskID := id.NewRunID()
	ctx = id.WithRunID(ctx, taskID)

	svc.emitWorkflowInputReceivedEvent(ctx, confirmedSessionID, taskID, task)

	taskRecord, err := svc.taskStore.Create(ctx, confirmedSessionID, task, agentPreset, toolPreset)
	if err != nil {
		logger.Error("[TaskExecutionService] Failed to create task: %v", err)
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	parentRunID := id.ParentRunIDFromContext(ctx)
	if parentRunID != "" {
		taskRecord.ParentTaskID = parentRunID
	}

	taskSessionID := taskRecord.SessionID
	ctx = id.WithIDs(ctx, id.IDs{SessionID: confirmedSessionID, RunID: taskRecord.ID, ParentRunID: parentRunID})

	if svc.broadcaster == nil {
		logger.Error("[TaskExecutionService] Broadcaster is nil!")
		_ = svc.taskStore.SetError(ctx, taskRecord.ID, UnavailableError("broadcaster not initialized"))
		return taskRecord, UnavailableError("broadcaster not initialized")
	}

	taskCtx, cancelFunc := context.WithCancelCause(context.WithoutCancel(ctx))

	svc.cancelMu.Lock()
	svc.cancelFuncs[taskID] = cancelFunc
	svc.cancelMu.Unlock()

	taskCopy := *taskRecord
	async.Go(svc.logger, "server.executeTask", func() {
		svc.executeTaskInBackground(taskCtx, taskID, task, confirmedSessionID, agentPreset, toolPreset)
	})

	logger.Info("[TaskExecutionService] Task created: taskID=%s, sessionID=%s, returning immediately", taskID, taskSessionID)
	return &taskCopy, nil
}

// executeTaskInBackground runs the actual task execution in a background goroutine.
func (svc *TaskExecutionService) executeTaskInBackground(ctx context.Context, taskID string, task string, sessionID string, agentPreset string, toolPreset string) {
	logger := logging.FromContext(ctx, svc.logger)
	defer func() {
		svc.cancelMu.Lock()
		delete(svc.cancelFuncs, taskID)
		svc.cancelMu.Unlock()

		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("[Background] PANIC in task execution (taskID=%s, sessionID=%s): %v", taskID, sessionID, r)
			logger.Error("%s", errMsg)
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)
			_ = svc.taskStore.SetError(ctx, taskID, fmt.Errorf("panic: %v", r))
		}
	}()

	logger.Info("[Background] Starting task execution: taskID=%s, sessionID=%s", taskID, sessionID)

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
		errMsg := fmt.Sprintf("[Background] CRITICAL: agentCoordinator is nil (taskID=%s)", taskID)
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
		logger.Info("[Background] Using presets: agent=%s, tool=%s", agentPreset, toolPreset)
	}

	logger.Info("[Background] Calling AgentCoordinator.ExecuteTask...")

	var listener agent.EventListener = svc.broadcaster
	if svc.progressTracker != nil {
		listener = NewMultiEventListener(svc.broadcaster, svc.progressTracker)
	}

	ctx = shared.WithParentListener(ctx, listener)
	result, err := svc.agentCoordinator.ExecuteTask(ctx, task, sessionID, listener)

	if ctx.Err() != nil {
		logger.Info("[Background] Task cancelled: taskID=%s, sessionID=%s, reason=%v", taskID, sessionID, context.Cause(ctx))
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
		errMsg := fmt.Sprintf("[Background] Task execution failed (taskID=%s, sessionID=%s): %v", taskID, sessionID, err)
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
	svc.captureAnalytics(ctx, sessionID, analytics.EventTaskExecutionCompleted, props)
}

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
		logger.Debug("[Analytics] failed to capture event %s: %v", event, err)
	}
}

func (svc *TaskExecutionService) emitWorkflowInputReceivedEvent(ctx context.Context, sessionID, taskID, task string) {
	if svc.broadcaster == nil {
		return
	}
	logger := logging.FromContext(ctx, svc.logger)

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
	logger := logging.FromContext(ctx, svc.logger)

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
		logger.Info("[CancelTask] Emitting workflow.result.cancelled envelope: sessionID=%s taskID=%s", task.SessionID, task.ID)
		svc.broadcaster.OnEvent(envelope)
	}

	logger.Info("[CancelTask] Emitting workflow.result.cancelled event: sessionID=%s taskID=%s", task.SessionID, task.ID)
	svc.broadcaster.OnEvent(event)
}

// GetTask retrieves a task by ID.
func (svc *TaskExecutionService) GetTask(ctx context.Context, taskID string) (*serverPorts.Task, error) {
	return svc.taskStore.Get(ctx, taskID)
}

// ListTasks returns all tasks with pagination.
func (svc *TaskExecutionService) ListTasks(ctx context.Context, limit int, offset int) ([]*serverPorts.Task, int, error) {
	return svc.taskStore.List(ctx, limit, offset)
}

// ListSessionTasks returns all tasks for a session.
func (svc *TaskExecutionService) ListSessionTasks(ctx context.Context, sessionID string) ([]*serverPorts.Task, error) {
	return svc.taskStore.ListBySession(ctx, sessionID)
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
		logger.Info("[CancelTask] Cancelling task execution: taskID=%s", taskID)
		cancelFunc(fmt.Errorf("task cancelled by user"))
		svc.emitWorkflowResultCancelledEvent(ctx, task, "cancelled", "user")
		status = "dispatched"
	} else {
		logger.Warn("[CancelTask] No cancel function found for taskID=%s, updating status only", taskID)
		if err := svc.taskStore.SetStatus(ctx, taskID, serverPorts.TaskStatusCancelled); err != nil {
			return err
		}
		if err := svc.taskStore.SetTerminationReason(ctx, taskID, serverPorts.TerminationReasonCancelled); err != nil {
			logger.Warn("[CancelTask] Failed to set termination reason for taskID=%s: %v", taskID, err)
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
