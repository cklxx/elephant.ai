package lark

import (
	"strings"
	"time"

	"alex/internal/delivery/channels"
	domain "alex/internal/domain/agent"
)

func (l *backgroundProgressListener) onBackgroundDispatched(env *domain.WorkflowEventEnvelope) {
	taskID := strings.TrimSpace(env.NodeID)
	if taskID == "" {
		taskID = asString(env.Payload["task_id"])
	}
	if taskID == "" {
		return
	}

	description := asString(env.Payload["description"])
	agentType := asString(env.Payload["agent_type"])
	startedAt := env.Timestamp()
	if startedAt.IsZero() {
		startedAt = l.clock()
	}

	l.mu.Lock()
	if l.closed || l.released {
		l.mu.Unlock()
		return
	}
	if _, exists := l.tasks[taskID]; exists {
		l.mu.Unlock()
		return
	}
	tracker := &bgTaskTracker{
		taskID:      taskID,
		description: description,
		agentType:   agentType,
		startedAt:   startedAt,
		status:      taskStatusRunning,
		interval:    l.taskInterval(agentType),
		window:      l.taskWindow(agentType),
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}
	l.tasks[taskID] = tracker
	l.mu.Unlock()

	// Send initial message (reply to original message when replyToID is provided).
	text := l.buildHumanHeader(tracker, "后台任务处理中")
	msgID, err := l.g.dispatchMessage(l.ctx, l.chatID, l.replyToID, "text", textContent(text))
	if err != nil {
		l.logger.Warn("Lark background progress initial send failed: %v", err)
		return
	}
	tracker.mu.Lock()
	tracker.progressMsgID = msgID
	tracker.mu.Unlock()

	// Persist task record to TaskStore.
	l.syncTaskSave(taskID, description, agentType, startedAt)

	go l.runTicker(tracker)
}

func (l *backgroundProgressListener) runTicker(t *bgTaskTracker) {
	defer close(t.doneCh)

	interval := t.interval
	if interval <= 0 {
		interval = l.interval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.flush(t, false)
		case <-t.stopCh:
			return
		}
	}
}

func (l *backgroundProgressListener) onExternalProgress(env *domain.WorkflowEventEnvelope) {
	taskID := asString(env.Payload["task_id"])
	if taskID == "" {
		taskID = strings.TrimSpace(env.NodeID)
	}
	if taskID == "" {
		return
	}

	t := l.getTask(taskID)
	if t == nil {
		return
	}

	currentTool := asString(env.Payload["current_tool"])

	// Heartbeat events keep the SerializingEventListener queue alive but
	// should not be recorded as progress or displayed to the user.
	if currentTool == "__heartbeat__" {
		return
	}

	rec := progressRecord{
		ts:          env.Timestamp(),
		currentTool: currentTool,
		currentArgs: truncateForLark(asString(env.Payload["current_args"]), 200),
		tokensUsed:  asInt(env.Payload["tokens_used"]),
		files:       asStringSlice(env.Payload["files_touched"]),
		activity:    asTime(env.Payload["last_activity"]),
	}
	if rec.ts.IsZero() {
		rec.ts = l.clock()
	}
	if rec.activity.IsZero() {
		rec.activity = rec.ts
	}

	t.mu.Lock()
	if isTerminalTaskStatus(t.status) {
		t.mu.Unlock()
		return
	}
	t.lastProgress = rec
	t.recent = append(t.recent, rec)
	window := t.window
	if window <= 0 {
		window = l.window
	}
	cutoff := l.clock().Add(-window)
	idx := 0
	for idx < len(t.recent) {
		if !t.recent[idx].ts.Before(cutoff) {
			break
		}
		idx++
	}
	if idx > 0 {
		t.recent = append([]progressRecord(nil), t.recent[idx:]...)
	}
	tokensUsed := rec.tokensUsed
	t.mu.Unlock()

	// Sync running status to TaskStore.
	l.syncTaskStatus(taskID, taskStatusRunning, WithTokensUsed(tokensUsed))
}

func (l *backgroundProgressListener) onExternalInputRequested(env *domain.WorkflowEventEnvelope) {
	taskID := asString(env.Payload["task_id"])
	if taskID == "" {
		taskID = strings.TrimSpace(env.NodeID)
	}
	if taskID == "" {
		return
	}

	t := l.getTask(taskID)
	if t == nil {
		return
	}

	summary := asString(env.Payload["summary"])
	if summary == "" {
		summary = "需要你确认/补充信息后继续。"
	}

	t.mu.Lock()
	if !isTerminalTaskStatus(t.status) {
		t.status = taskStatusWaitingInput
		t.pendingSummary = truncateForLark(summary, 400)
	}
	t.mu.Unlock()

	l.flush(t, true)
}

func (l *backgroundProgressListener) onBackgroundCompleted(env *domain.WorkflowEventEnvelope) {
	taskID := asString(env.Payload["task_id"])
	if taskID == "" {
		taskID = strings.TrimSpace(env.NodeID)
	}
	status := asString(env.Payload["status"])
	answer := asString(env.Payload["answer"])
	errText := asString(env.Payload["error"])
	mergeStatus := asString(env.Payload["merge_status"])
	tokensUsed := asInt(env.Payload["tokens_used"])
	l.handleCompletion(taskID, status, answer, errText, mergeStatus, tokensUsed)
}

// onRawCompleted handles background task completed events delivered directly by
// BackgroundTaskManager (bypassing SerializingEventListener). Dedup is safe
// because getTask returns nil after the first handler deletes the task.
func (l *backgroundProgressListener) onRawCompleted(e *domain.Event) {
	if e == nil {
		return
	}
	l.handleCompletion(e.Data.TaskID, e.Data.Status, e.Data.Answer, e.Data.ErrorStr, e.Data.MergeStatus, e.Data.TokensUsed)
}

// handleCompletion is the shared completion handler for both envelope and raw event paths.
func (l *backgroundProgressListener) handleCompletion(taskID, status, answer, errText, mergeStatus string, tokensUsed int) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}

	t := l.getTask(taskID)
	if t == nil {
		return
	}

	normalizedStatus := normalizeCompletionTaskStatus(status, errText)
	if raw := strings.TrimSpace(status); raw != "" && normalizeTaskStatus(raw) != normalizedStatus {
		l.logger.Warn("Lark background completion normalized non-terminal status: task_id=%s raw_status=%s normalized=%s", taskID, raw, normalizedStatus)
	}
	mergeStatus = normalizeMergeStatus(mergeStatus)

	// Sanitize error text before any user-facing use so raw Go error chains
	// are never shown verbatim in Lark messages.
	if errText != "" {
		errText = channels.SanitizeErrorForUser(errText)
	}

	t.mu.Lock()
	t.status = normalizedStatus
	t.mergeStatus = mergeStatus
	// Stash result in pendingSummary so flush can format without racing payload.
	if errText != "" {
		t.pendingSummary = truncateForLark(errText, 1500)
	} else {
		t.pendingSummary = truncateForLark(answer, 1500)
	}
	t.mu.Unlock()

	l.flush(t, true)

	// Sync final status to TaskStore.
	finalStatus := normalizedStatus
	var updateOpts []TaskUpdateOption
	if errText != "" {
		updateOpts = append(updateOpts, WithErrorText(truncateForLark(errText, 1500)))
	}
	if answer != "" {
		updateOpts = append(updateOpts, WithAnswerPreview(truncateForLark(answer, 1500)))
	}
	if mergeStatus != "" {
		updateOpts = append(updateOpts, WithMergeStatus(mergeStatus))
	}
	if tokensUsed > 0 {
		updateOpts = append(updateOpts, WithTokensUsed(tokensUsed))
	}
	l.syncTaskStatus(taskID, finalStatus, updateOpts...)

	elapsed := l.clock().Sub(t.startedAt)
	if elapsed < 0 {
		elapsed = 0
	}

	l.mu.Lock()
	l.completedTasks = append(l.completedTasks, completedTaskRecord{
		taskID:      taskID,
		description: t.description,
		status:      normalizedStatus,
		answer:      truncateForLark(answer, 500),
		errText:     truncateForLark(errText, 500),
		duration:    elapsed,
	})
	delete(l.tasks, taskID)
	shouldClose := l.released && len(l.tasks) == 0
	completedCount := len(l.completedTasks)
	l.mu.Unlock()

	t.stop()

	if shouldClose {
		l.logger.Info("All %d background tasks completed, generating team summary", completedCount)
		l.sendTeamCompletionSummary()
		l.Close()
	}
}
