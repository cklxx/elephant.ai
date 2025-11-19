package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	agentApp "alex/internal/agent/app"
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/analytics"
	"alex/internal/analytics/journal"
	"alex/internal/observability"
	serverPorts "alex/internal/server/ports"
	sessionstate "alex/internal/session/state_store"
	"alex/internal/tools/builtin"
	"alex/internal/utils"
	id "alex/internal/utils/id"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
	stateStore       sessionstate.Store
	taskStore        serverPorts.TaskStore
	logger           *utils.Logger
	analytics        analytics.Client
	journalReader    journal.Reader
	obs              *observability.Observability

	// Cancel function map for task cancellation support
	cancelFuncs map[string]context.CancelCauseFunc
	cancelMu    sync.RWMutex
}

// ContextSnapshotRecord captures a snapshot of the messages sent to the LLM.
type ContextSnapshotRecord struct {
	SessionID    string
	TaskID       string
	ParentTaskID string
	RequestID    string
	Iteration    int
	Timestamp    time.Time
	Messages     []ports.Message
	Excluded     []ports.Message
}

// NewServerCoordinator creates a new server coordinator
func NewServerCoordinator(
	agentCoordinator AgentExecutor,
	broadcaster *EventBroadcaster,
	sessionStore ports.SessionStore,
	taskStore serverPorts.TaskStore,
	stateStore sessionstate.Store,
	opts ...ServerCoordinatorOption,
) *ServerCoordinator {
	coordinator := &ServerCoordinator{
		agentCoordinator: agentCoordinator,
		broadcaster:      broadcaster,
		sessionStore:     sessionStore,
		stateStore:       stateStore,
		taskStore:        taskStore,
		logger:           utils.NewComponentLogger("ServerCoordinator"),
		analytics:        analytics.NewNoopClient(),
		cancelFuncs:      make(map[string]context.CancelCauseFunc),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(coordinator)
		}
	}

	return coordinator
}

// ServerCoordinatorOption configures optional behavior for the server coordinator.
type ServerCoordinatorOption func(*ServerCoordinator)

// WithAnalyticsClient attaches an analytics client to the coordinator.
func WithAnalyticsClient(client analytics.Client) ServerCoordinatorOption {
	return func(coordinator *ServerCoordinator) {
		if client == nil {
			coordinator.analytics = analytics.NewNoopClient()
			return
		}
		coordinator.analytics = client
	}
}

// WithJournalReader wires a journal reader used for replay operations.
func WithJournalReader(reader journal.Reader) ServerCoordinatorOption {
	return func(coordinator *ServerCoordinator) {
		coordinator.journalReader = reader
	}
}

// WithObservability wires the observability provider into the coordinator.
func WithObservability(obs *observability.Observability) ServerCoordinatorOption {
	return func(coordinator *ServerCoordinator) {
		coordinator.obs = obs
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
	if s.stateStore != nil {
		if err := s.stateStore.Init(ctx, session.ID); err != nil {
			s.logger.Warn("[ServerCoordinator] Failed to initialize state store: %v", err)
		}
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

	taskID := taskRecord.ID
	taskSessionID := taskRecord.SessionID
	ctx = id.WithIDs(ctx, id.IDs{SessionID: confirmedSessionID, TaskID: taskID, ParentTaskID: parentTaskID})

	// Verify broadcaster is initialized
	if s.broadcaster == nil {
		s.logger.Error("[ServerCoordinator] Broadcaster is nil!")
		_ = s.taskStore.SetError(ctx, taskID, fmt.Errorf("broadcaster not initialized"))
		return taskRecord, fmt.Errorf("broadcaster not initialized")
	}

	// Emit user_task event immediately so the frontend gets instant feedback.
	s.emitUserTaskEvent(ctx, confirmedSessionID, taskRecord.ID, task)

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
	go s.executeTaskInBackground(taskCtx, taskID, task, confirmedSessionID, agentPreset, toolPreset)

	s.logger.Info("[ServerCoordinator] Task created: taskID=%s, sessionID=%s, returning immediately", taskID, taskSessionID)
	return &taskCopy, nil
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

	parentTaskID := id.ParentTaskIDFromContext(ctx)
	startTime := time.Now()
	status := "success"
	var spanErr error
	if s.obs != nil {
		if s.obs.Tracer != nil {
			attrs := append(observability.SessionAttrs(sessionID), attribute.String(observability.AttrTaskID, taskID))
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
		s.logger.Error("%s", errMsg)
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)
		err := fmt.Errorf("agent coordinator not initialized")
		spanErr = err
		status = "error"
		_ = s.taskStore.SetError(ctx, taskID, err)
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

	// Ensure subagent tool invocations forward their events to the main listener
	ctx = builtin.WithParentListener(ctx, s.broadcaster)

	result, err := s.agentCoordinator.ExecuteTask(ctx, task, sessionID, s.broadcaster)

	// Check if context was cancelled
	if ctx.Err() != nil {
		s.logger.Info("[Background] Task cancelled: taskID=%s, sessionID=%s, reason=%v", taskID, sessionID, context.Cause(ctx))
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
			"task_id":            taskID,
			"session_id":         sessionID,
			"termination_reason": string(terminationReason),
			"duration_ms":        time.Since(startTime).Milliseconds(),
		}
		if parentTaskID != "" {
			props["parent_task_id"] = parentTaskID
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
		s.logger.Error("%s", errMsg)

		// CRITICAL: Also print to stderr so server operator can see it
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)

		// Update task status
		_ = s.taskStore.SetError(ctx, taskID, err)
		props := map[string]any{
			"task_id":     taskID,
			"session_id":  sessionID,
			"duration_ms": time.Since(startTime).Milliseconds(),
			"error":       err.Error(),
		}
		if parentTaskID != "" {
			props["parent_task_id"] = parentTaskID
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

	s.logger.Info("[Background] Task execution completed: taskID=%s, sessionID=%s, iterations=%d", taskID, result.SessionID, result.Iterations)

	props := map[string]any{
		"task_id":     taskID,
		"session_id":  sessionID,
		"duration_ms": time.Since(startTime).Milliseconds(),
		"iterations":  result.Iterations,
	}
	if parentTaskID != "" {
		props["parent_task_id"] = parentTaskID
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

// ListSnapshots returns paginated session snapshots for API consumers.
func (s *ServerCoordinator) ListSnapshots(ctx context.Context, sessionID string, cursor string, limit int) ([]sessionstate.SnapshotMetadata, string, error) {
	if s.stateStore == nil {
		return nil, "", fmt.Errorf("state store not configured")
	}
	return s.stateStore.ListSnapshots(ctx, sessionID, cursor, limit)
}

// GetSnapshot fetches a specific turn snapshot.
func (s *ServerCoordinator) GetSnapshot(ctx context.Context, sessionID string, turnID int) (sessionstate.Snapshot, error) {
	if s.stateStore == nil {
		return sessionstate.Snapshot{}, fmt.Errorf("state store not configured")
	}
	return s.stateStore.GetSnapshot(ctx, sessionID, turnID)
}

// ReplaySession rehydrates the snapshot store from persisted turn journal entries.
func (s *ServerCoordinator) ReplaySession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}
	if s.journalReader == nil {
		return fmt.Errorf("journal reader not configured")
	}
	if s.stateStore == nil {
		return fmt.Errorf("state store not configured")
	}
	var snapshots []sessionstate.Snapshot
	streamErr := s.journalReader.Stream(ctx, sessionID, func(entry journal.TurnJournalEntry) error {
		snapshot := sessionstate.Snapshot{
			SessionID:     entry.SessionID,
			TurnID:        entry.TurnID,
			LLMTurnSeq:    entry.LLMTurnSeq,
			Summary:       entry.Summary,
			Plans:         entry.Plans,
			Beliefs:       entry.Beliefs,
			World:         entry.World,
			Diff:          entry.Diff,
			Messages:      entry.Messages,
			Feedback:      entry.Feedback,
			KnowledgeRefs: entry.KnowledgeRefs,
		}
		if snapshot.SessionID == "" {
			snapshot.SessionID = sessionID
		}
		if entry.Timestamp.IsZero() {
			snapshot.CreatedAt = time.Now().UTC()
		} else {
			snapshot.CreatedAt = entry.Timestamp
		}
		snapshots = append(snapshots, snapshot)
		return nil
	})
	if streamErr != nil {
		return fmt.Errorf("replay journal: %w", streamErr)
	}
	if len(snapshots) == 0 {
		return fmt.Errorf("no journal entries for session %s", sessionID)
	}
	if err := s.stateStore.ClearSession(ctx, sessionID); err != nil {
		return fmt.Errorf("clear state store: %w", err)
	}
	if err := s.stateStore.Init(ctx, sessionID); err != nil {
		return fmt.Errorf("init state store: %w", err)
	}
	for _, snapshot := range snapshots {
		if err := s.stateStore.SaveSnapshot(ctx, snapshot); err != nil {
			return fmt.Errorf("save snapshot: %w", err)
		}
	}
	if s.logger != nil {
		s.logger.Info("[Replay] Rehydrated %d turn(s) for session %s", len(snapshots), sessionID)
	}
	return nil
}

func (s *ServerCoordinator) captureAnalytics(ctx context.Context, distinctID string, event string, props map[string]any) {
	if s.analytics == nil {
		return
	}

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
		s.logger.Debug("[Analytics] failed to capture event %s: %v", event, err)
	}
}

func (s *ServerCoordinator) emitUserTaskEvent(ctx context.Context, sessionID, taskID, task string) {
	if s.broadcaster == nil {
		return
	}

	parentTaskID := id.ParentTaskIDFromContext(ctx)
	level := ports.GetOutputContext(ctx).Level
	attachments := agentApp.GetUserAttachments(ctx)
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
			attachmentMap[name] = sanitized
		}
	}

	event := domain.NewUserTaskEvent(level, sessionID, taskID, parentTaskID, task, attachmentMap, time.Now())
	s.logger.Debug("[Background] Emitting user_task event for session=%s task=%s", sessionID, taskID)
	s.broadcaster.OnEvent(event)

	attachmentCount := len(attachmentMap)
	props := map[string]any{
		"task_id":          taskID,
		"session_id":       sessionID,
		"level":            level,
		"has_parent_task":  parentTaskID != "",
		"has_attachments":  attachmentCount > 0,
		"attachment_count": attachmentCount,
	}
	if parentTaskID != "" {
		props["parent_task_id"] = parentTaskID
	}

	s.captureAnalytics(ctx, sessionID, analytics.EventTaskExecutionStarted, props)
}

// GetContextSnapshots retrieves context snapshots captured during LLM calls for a session.
func (s *ServerCoordinator) GetContextSnapshots(sessionID string) []ContextSnapshotRecord {
	if s.broadcaster == nil || sessionID == "" {
		return nil
	}

	events := s.broadcaster.GetEventHistory(sessionID)
	if len(events) == 0 {
		return nil
	}

	snapshots := make([]ContextSnapshotRecord, 0)
	for _, event := range events {
		snapshot, ok := event.(*domain.ContextSnapshotEvent)
		if !ok {
			continue
		}
		record := ContextSnapshotRecord{
			SessionID:    sessionID,
			TaskID:       snapshot.GetTaskID(),
			ParentTaskID: snapshot.GetParentTaskID(),
			RequestID:    snapshot.RequestID,
			Iteration:    snapshot.Iteration,
			Timestamp:    snapshot.Timestamp(),
			Messages:     cloneMessages(snapshot.Messages),
			Excluded:     cloneMessages(snapshot.Excluded),
		}
		snapshots = append(snapshots, record)
	}
	return snapshots
}

func (s *ServerCoordinator) emitTaskCancelledEvent(ctx context.Context, task *serverPorts.Task, reason, requestedBy string) {
	if s.broadcaster == nil || task == nil {
		return
	}

	outCtx := ports.GetOutputContext(ctx)
	level := outCtx.Level
	if level == "" {
		level = ports.LevelCore
	}

	event := domain.NewTaskCancelledEvent(
		level,
		task.SessionID,
		task.ID,
		task.ParentTaskID,
		reason,
		requestedBy,
		time.Now(),
	)
	s.logger.Info("[CancelTask] Emitting task_cancelled event: sessionID=%s taskID=%s", task.SessionID, task.ID)
	s.broadcaster.OnEvent(event)
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

	status := "no_active_execution"
	if exists && cancelFunc != nil {
		s.logger.Info("[CancelTask] Cancelling task execution: taskID=%s", taskID)
		cancelFunc(fmt.Errorf("task cancelled by user"))
		s.emitTaskCancelledEvent(ctx, task, "cancelled", "user")
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
		s.emitTaskCancelledEvent(ctx, task, "cancelled", "user")
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

func cloneMessages(messages []ports.Message) []ports.Message {
	if len(messages) == 0 {
		return nil
	}
	cloned := make([]ports.Message, len(messages))
	for i, msg := range messages {
		cloned[i] = cloneMessage(msg)
	}
	return cloned
}

func cloneMessage(msg ports.Message) ports.Message {
	cloned := msg
	if len(msg.ToolCalls) > 0 {
		cloned.ToolCalls = append([]ports.ToolCall(nil), msg.ToolCalls...)
	}
	if len(msg.ToolResults) > 0 {
		cloned.ToolResults = make([]ports.ToolResult, len(msg.ToolResults))
		for i, result := range msg.ToolResults {
			cloned.ToolResults[i] = cloneToolResult(result)
		}
	}
	if len(msg.Metadata) > 0 {
		metadata := make(map[string]any, len(msg.Metadata))
		for key, value := range msg.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(msg.Attachments) > 0 {
		cloned.Attachments = cloneAttachmentsMap(msg.Attachments)
	}
	return cloned
}

func cloneToolResult(result ports.ToolResult) ports.ToolResult {
	cloned := result
	if len(result.Metadata) > 0 {
		metadata := make(map[string]any, len(result.Metadata))
		for key, value := range result.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(result.Attachments) > 0 {
		cloned.Attachments = cloneAttachmentsMap(result.Attachments)
	}
	return cloned
}

func cloneAttachmentsMap(values map[string]ports.Attachment) map[string]ports.Attachment {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]ports.Attachment, len(values))
	for key, att := range values {
		cloned[key] = att
	}
	return cloned
}
