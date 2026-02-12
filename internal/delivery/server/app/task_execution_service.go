package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	appcontext "alex/internal/app/agent/context"
	toolcontext "alex/internal/app/toolcontext"
	serverPorts "alex/internal/delivery/server/ports"
	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/analytics"
	"alex/internal/infra/observability"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const (
	defaultTaskLeaseTTL           = 45 * time.Second
	defaultTaskLeaseRenewInterval = 15 * time.Second
	defaultTaskMaxInFlight        = 64
	defaultResumeClaimBatchSize   = 128
)

// BridgeOrphanResumer detects and processes orphaned bridge subprocesses
// left behind after a process restart. Implementations classify each orphan
// (adopt, harvest, retry, or fail) and update the task store accordingly.
type BridgeOrphanResumer interface {
	// ResumeOrphans scans workDir for orphaned bridge outputs and processes them.
	// Returns a summary of actions taken per task.
	ResumeOrphans(ctx context.Context, workDir string) []OrphanResumeResult
}

// OrphanResumeResult captures what happened to a single orphaned bridge.
type OrphanResumeResult struct {
	TaskID string
	Action string
	Error  error
}

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
	bridgeResumer BridgeOrphanResumer
	bridgeWorkDir string
	analytics     analytics.Client
	obs           *observability.Observability
	logger        logging.Logger

	cancelFuncs map[string]context.CancelCauseFunc
	cancelMu    sync.RWMutex

	ownerID              string
	leaseTTL             time.Duration
	leaseRenewInterval   time.Duration
	resumeClaimBatchSize int
	admissionSem         chan struct{}
}

// SessionTaskSummary captures task_count/last_task style metadata for a session.
type SessionTaskSummary struct {
	TaskCount int
	LastTask  string
}

// sessionTaskSummaryStore is an optional optimization interface implemented by
// task stores that can summarize multiple sessions in one pass.
type sessionTaskSummaryStore interface {
	SummarizeSessionTasks(ctx context.Context, sessionIDs []string) (map[string]SessionTaskSummary, error)
}

// NewTaskExecutionService creates a new task execution service.
func NewTaskExecutionService(
	agentCoordinator AgentExecutor,
	broadcaster *EventBroadcaster,
	taskStore serverPorts.TaskStore,
	opts ...TaskExecutionServiceOption,
) *TaskExecutionService {
	svc := &TaskExecutionService{
		agentCoordinator:     agentCoordinator,
		broadcaster:          broadcaster,
		taskStore:            taskStore,
		analytics:            analytics.NewNoopClient(),
		logger:               logging.NewComponentLogger("TaskExecutionService"),
		cancelFuncs:          make(map[string]context.CancelCauseFunc),
		ownerID:              defaultTaskOwnerID(),
		leaseTTL:             defaultTaskLeaseTTL,
		leaseRenewInterval:   defaultTaskLeaseRenewInterval,
		resumeClaimBatchSize: defaultResumeClaimBatchSize,
		admissionSem:         make(chan struct{}, defaultTaskMaxInFlight),
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

// WithBridgeResumer wires orphan bridge detection and resumption.
// workDir is the base directory where .elephant/bridge/ dirs are created.
func WithBridgeResumer(resumer BridgeOrphanResumer, workDir string) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		svc.bridgeResumer = resumer
		svc.bridgeWorkDir = workDir
	}
}

// WithTaskOwnerID sets the task-lease owner identifier for this process.
func WithTaskOwnerID(ownerID string) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		ownerID = strings.TrimSpace(ownerID)
		if ownerID == "" {
			return
		}
		svc.ownerID = ownerID
	}
}

// WithTaskLeaseConfig configures claim lease TTL and renew interval.
func WithTaskLeaseConfig(ttl, renewInterval time.Duration) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		if ttl > 0 {
			svc.leaseTTL = ttl
		}
		if renewInterval > 0 {
			svc.leaseRenewInterval = renewInterval
		}
	}
}

// WithTaskAdmissionLimit configures global in-flight task admission.
// maxInFlight <= 0 disables the admission limiter.
func WithTaskAdmissionLimit(maxInFlight int) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		if maxInFlight <= 0 {
			svc.admissionSem = nil
			return
		}
		svc.admissionSem = make(chan struct{}, maxInFlight)
	}
}

// WithResumeClaimBatchSize configures max claimed tasks per resume pass.
func WithResumeClaimBatchSize(batchSize int) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		if batchSize <= 0 {
			return
		}
		svc.resumeClaimBatchSize = batchSize
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
	taskID = taskRecord.ID
	ctx = id.WithIDs(ctx, id.IDs{SessionID: confirmedSessionID, RunID: taskID, ParentRunID: parentRunID})

	if svc.broadcaster == nil {
		logger.Error("[TaskExecutionService] Broadcaster is nil!")
		_ = svc.taskStore.SetError(context.Background(), taskID, UnavailableError("broadcaster not initialized"))
		return taskRecord, UnavailableError("broadcaster not initialized")
	}

	releaseAdmission, err := svc.acquireAdmission(ctx)
	if err != nil {
		admissionErr := UnavailableError("task admission timed out")
		logger.Warn("[TaskExecutionService] Admission wait failed for task %s: %v", taskID, err)
		_ = svc.taskStore.SetError(context.Background(), taskID, admissionErr)
		return taskRecord, admissionErr
	}

	leaseUntil := svc.nextLeaseDeadline(time.Now())
	claimed, err := svc.taskStore.TryClaimTask(ctx, taskID, svc.ownerID, leaseUntil)
	if err != nil {
		releaseAdmission()
		logger.Error("[TaskExecutionService] Failed to claim task %s: %v", taskID, err)
		_ = svc.taskStore.SetError(context.Background(), taskID, fmt.Errorf("failed to claim task: %w", err))
		return taskRecord, fmt.Errorf("claim task ownership: %w", err)
	}
	if !claimed {
		claimErr := ConflictError("task already claimed by another worker")
		logger.Warn("[TaskExecutionService] Claim rejected for task %s", taskID)
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

	logger.Info("[TaskExecutionService] Task created: taskID=%s, sessionID=%s, returning immediately", taskID, taskSessionID)
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
	logger := logging.FromContext(ctx, svc.logger)
	stopLeaseRenew := svc.startTaskLeaseRenewer(ctx, taskID)

	defer func() {
		stopLeaseRenew()
		if releaseAdmission != nil {
			releaseAdmission()
		}
		if err := svc.taskStore.ReleaseTaskLease(context.Background(), taskID, svc.ownerID); err != nil {
			logger.Warn("[Background] Failed to release lease for task %s: %v", taskID, err)
		}

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

	ctx = toolcontext.WithParentListener(ctx, listener)
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

	event := domain.NewInputReceivedEvent(level, sessionID, taskID, parentRunID, task, attachmentMap, time.Now())
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

// SummarizeSessionTasks returns task_count/last_task for each requested session.
// It uses an optional batched task-store capability when available.
func (svc *TaskExecutionService) SummarizeSessionTasks(ctx context.Context, sessionIDs []string) (map[string]SessionTaskSummary, error) {
	summaries := make(map[string]SessionTaskSummary, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return summaries, nil
	}

	if batchStore, ok := svc.taskStore.(sessionTaskSummaryStore); ok {
		return batchStore.SummarizeSessionTasks(ctx, sessionIDs)
	}

	seen := make(map[string]struct{}, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if sessionID == "" {
			continue
		}
		if _, exists := seen[sessionID]; exists {
			continue
		}
		seen[sessionID] = struct{}{}

		tasks, err := svc.taskStore.ListBySession(ctx, sessionID)
		if err != nil {
			return nil, err
		}

		summary := SessionTaskSummary{TaskCount: len(tasks)}
		if len(tasks) > 0 {
			// ListBySession returns tasks sorted newest-first.
			summary.LastTask = tasks[0].Description
		}
		summaries[sessionID] = summary
	}

	return summaries, nil
}

// ResumePendingTasks re-dispatches persisted pending/running tasks after restart.
// It first detects and processes orphaned bridge subprocesses (adopt/harvest/fail),
// then re-dispatches any remaining pending/running tasks.
func (svc *TaskExecutionService) ResumePendingTasks(ctx context.Context) (int, error) {
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	logger := logging.FromContext(ctx, svc.logger)
	if svc.agentCoordinator == nil {
		return 0, UnavailableError("agent coordinator not initialized")
	}
	if svc.broadcaster == nil {
		return 0, UnavailableError("broadcaster not initialized")
	}

	// Phase 1: Detect and process orphaned bridge subprocesses.
	svc.resumeOrphanedBridges(ctx, logger)

	leaseUntil := svc.nextLeaseDeadline(time.Now())
	tasks, err := svc.taskStore.ClaimResumableTasks(
		ctx,
		svc.ownerID,
		leaseUntil,
		svc.resumeClaimBatchSize,
		serverPorts.TaskStatusPending,
		serverPorts.TaskStatusRunning,
	)
	if err != nil {
		return 0, fmt.Errorf("claim resumable tasks: %w", err)
	}
	if len(tasks) == 0 {
		logger.Info("[Resume] no pending/running tasks to resume")
		return 0, nil
	}

	resumed := 0
	skipped := 0
	for _, task := range tasks {
		if task == nil || task.ID == "" {
			skipped++
			continue
		}
		if task.Description == "" {
			logger.Warn("[Resume] skipping task %s: empty description", task.ID)
			svc.releaseTaskLease(task.ID, logger)
			skipped++
			continue
		}
		if task.SessionID == "" {
			logger.Warn("[Resume] skipping task %s: empty session_id", task.ID)
			svc.releaseTaskLease(task.ID, logger)
			skipped++
			continue
		}

		session, err := svc.agentCoordinator.GetSession(ctx, task.SessionID)
		if err != nil {
			logger.Warn("[Resume] skipping task %s: failed to load session %s: %v", task.ID, task.SessionID, err)
			svc.releaseTaskLease(task.ID, logger)
			skipped++
			continue
		}
		if svc.stateStore != nil {
			if err := svc.stateStore.Init(ctx, session.ID); err != nil {
				logger.Warn("[Resume] state store init failed for session %s: %v", session.ID, err)
			}
		}

		svc.cancelMu.RLock()
		_, alreadyRunning := svc.cancelFuncs[task.ID]
		svc.cancelMu.RUnlock()
		if alreadyRunning {
			logger.Warn("[Resume] skipping task %s: already has active cancel function", task.ID)
			skipped++
			continue
		}

		taskCtx := id.WithIDs(context.Background(), id.IDs{
			SessionID:   session.ID,
			RunID:       task.ID,
			ParentRunID: task.ParentTaskID,
		})
		taskCtx, _ = id.EnsureLogID(taskCtx, id.NewLogID)
		taskCtx = context.WithoutCancel(taskCtx)

		cancelCtx, cancelFunc := context.WithCancelCause(taskCtx)
		svc.cancelMu.Lock()
		svc.cancelFuncs[task.ID] = cancelFunc
		svc.cancelMu.Unlock()

		taskID := task.ID
		description := task.Description
		agentPreset := task.AgentPreset
		toolPreset := task.ToolPreset
		resumeSessionID := session.ID
		async.Go(svc.logger, "server.resumeTask", func() {
			svc.executeTaskInBackground(cancelCtx, taskID, description, resumeSessionID, agentPreset, toolPreset, nil)
		})

		logger.Info("[Resume] resumed task taskID=%s sessionID=%s", taskID, resumeSessionID)
		resumed++
	}

	logger.Info("[Resume] complete: claimed=%d resumed=%d skipped=%d", len(tasks), resumed, skipped)
	return resumed, nil
}

func defaultTaskOwnerID() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", host, os.Getpid())
}

func (svc *TaskExecutionService) nextLeaseDeadline(now time.Time) time.Time {
	if svc.leaseTTL <= 0 {
		return now.Add(defaultTaskLeaseTTL)
	}
	return now.Add(svc.leaseTTL)
}

func (svc *TaskExecutionService) acquireAdmission(ctx context.Context) (func(), error) {
	if svc.admissionSem == nil {
		return func() {}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case svc.admissionSem <- struct{}{}:
		var once sync.Once
		return func() {
			once.Do(func() {
				<-svc.admissionSem
			})
		}, nil
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	}
}

func (svc *TaskExecutionService) startTaskLeaseRenewer(ctx context.Context, taskID string) func() {
	if svc.leaseTTL <= 0 || svc.leaseRenewInterval <= 0 || taskID == "" {
		return func() {}
	}
	logger := logging.FromContext(ctx, svc.logger)
	stop := make(chan struct{})
	var stopOnce sync.Once

	async.Go(svc.logger, "server.taskLeaseRenewer", func() {
		ticker := time.NewTicker(svc.leaseRenewInterval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				renewUntil := svc.nextLeaseDeadline(time.Now())
				ok, err := svc.taskStore.RenewTaskLease(context.Background(), taskID, svc.ownerID, renewUntil)
				if err != nil {
					logger.Warn("[Lease] renew failed for task %s: %v", taskID, err)
					continue
				}
				if !ok {
					logger.Warn("[Lease] ownership lost for task %s, cancelling local execution", taskID)
					svc.cancelTaskExecution(taskID, fmt.Errorf("task lease lost"))
					return
				}
			}
		}
	})

	return func() {
		stopOnce.Do(func() {
			close(stop)
		})
	}
}

func (svc *TaskExecutionService) cancelTaskExecution(taskID string, cause error) {
	if taskID == "" || cause == nil {
		return
	}
	svc.cancelMu.RLock()
	cancelFunc := svc.cancelFuncs[taskID]
	svc.cancelMu.RUnlock()
	if cancelFunc != nil {
		cancelFunc(cause)
	}
}

func (svc *TaskExecutionService) releaseTaskLease(taskID string, logger logging.Logger) {
	if taskID == "" {
		return
	}
	if err := svc.taskStore.ReleaseTaskLease(context.Background(), taskID, svc.ownerID); err != nil {
		logger.Warn("[Lease] release failed for task %s: %v", taskID, err)
	}
}

// ListActiveTasks returns all currently running tasks.
func (svc *TaskExecutionService) ListActiveTasks(ctx context.Context) ([]*serverPorts.Task, error) {
	return svc.taskStore.ListByStatus(ctx, serverPorts.TaskStatusPending, serverPorts.TaskStatusRunning)
}

// TaskStats returns aggregated task metrics.
type TaskStats struct {
	ActiveCount    int     `json:"active_count"`
	PendingCount   int     `json:"pending_count"`
	RunningCount   int     `json:"running_count"`
	CompletedCount int     `json:"completed_count"`
	FailedCount    int     `json:"failed_count"`
	CancelledCount int     `json:"cancelled_count"`
	TotalCount     int     `json:"total_count"`
	TotalTokens    int     `json:"total_tokens"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
}

// GetTaskStats computes aggregated task metrics.
func (svc *TaskExecutionService) GetTaskStats(ctx context.Context) (*TaskStats, error) {
	tasks, total, err := svc.taskStore.List(ctx, 10000, 0)
	if err != nil {
		return nil, err
	}

	stats := &TaskStats{TotalCount: total}
	for _, t := range tasks {
		switch t.Status {
		case serverPorts.TaskStatusPending:
			stats.PendingCount++
			stats.ActiveCount++
		case serverPorts.TaskStatusRunning:
			stats.RunningCount++
			stats.ActiveCount++
		case serverPorts.TaskStatusCompleted:
			stats.CompletedCount++
		case serverPorts.TaskStatusFailed:
			stats.FailedCount++
		case serverPorts.TaskStatusCancelled:
			stats.CancelledCount++
		}
		stats.TotalTokens += t.TokensUsed
	}

	return stats, nil
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

// resumeOrphanedBridges detects and processes orphaned bridge subprocesses.
// This runs as the first step of ResumePendingTasks to adopt running bridges,
// harvest completed results, and mark dead bridges as failed before re-dispatching
// persisted tasks.
func (svc *TaskExecutionService) resumeOrphanedBridges(ctx context.Context, logger logging.Logger) {
	if svc.bridgeResumer == nil || svc.bridgeWorkDir == "" {
		return
	}

	results := svc.bridgeResumer.ResumeOrphans(ctx, svc.bridgeWorkDir)
	if len(results) == 0 {
		return
	}

	adopted, harvested, failed := 0, 0, 0
	for _, r := range results {
		switch r.Action {
		case "adopt":
			adopted++
		case "harvest":
			harvested++
		default:
			failed++
		}
		if r.Error != nil {
			logger.Warn("[Resume] orphan %s (%s): %v", r.TaskID, r.Action, r.Error)
		}
	}

	logger.Info("[Resume] orphan bridge scan: adopted=%d harvested=%d failed=%d", adopted, harvested, failed)
}
