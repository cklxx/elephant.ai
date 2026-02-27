package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/llmclient"
	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	ports "alex/internal/domain/agent/ports"
	"alex/internal/domain/agent/types"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

const (
	defaultBackgroundProgressInterval       = 10 * time.Minute
	defaultBackgroundProgressWindow         = 10 * time.Minute
	codeBackgroundProgressInterval          = 3 * time.Minute
	maxBackgroundListenerLifetime           = 4 * time.Hour
	defaultTeamCompletionSummaryLLMTimeout  = 10 * time.Second
	teamCompletionSummaryMaxPromptChars     = 3000
	teamCompletionSummaryMaxReplyChars      = 1200
	teamCompletionSummaryMinTasks           = 2
)

// completedTaskRecord captures the final state of a completed background task
// for team-level summary generation.
type completedTaskRecord struct {
	taskID      string
	description string
	status      string
	answer      string
	errText     string
	duration    time.Duration
}

type progressRecord struct {
	ts          time.Time
	currentTool string
	currentArgs string
	tokensUsed  int
	files       []string
	activity    time.Time
}

type bgTaskTracker struct {
	mu sync.Mutex

	taskID      string
	description string
	agentType   string
	startedAt   time.Time

	status         string
	progressMsgID  string
	pendingSummary string
	mergeStatus    string

	interval time.Duration
	window   time.Duration

	lastProgress progressRecord
	recent       []progressRecord

	stopCh chan struct{}
	doneCh chan struct{}
}

func (t *bgTaskTracker) stop() {
	if t == nil {
		return
	}
	select {
	case <-t.doneCh:
		return
	default:
	}
	select {
	case <-t.stopCh:
		// already closed
	default:
		close(t.stopCh)
	}
	<-t.doneCh
}

type backgroundProgressListener struct {
	inner     agent.EventListener
	ctx       context.Context
	g         *Gateway
	chatID    string
	replyToID string
	logger    logging.Logger
	now       func() time.Time
	interval  time.Duration
	window    time.Duration

	mu             sync.Mutex
	tasks          map[string]*bgTaskTracker
	completedTasks []completedTaskRecord
	closed         bool
	released       bool
	pollerInterval time.Duration // configurable for testing; defaults to 30s
	doneCh         chan struct{} // closed in Close() to stop the poller
	doneOnce       sync.Once
}

func newBackgroundProgressListener(ctx context.Context, inner agent.EventListener, g *Gateway, chatID, replyToID string, logger logging.Logger, interval, window time.Duration) *backgroundProgressListener {
	if interval <= 0 {
		interval = defaultBackgroundProgressInterval
	}
	if window <= 0 {
		window = defaultBackgroundProgressWindow
	}
	return &backgroundProgressListener{
		inner:          inner,
		ctx:            context.WithoutCancel(ctx),
		g:              g,
		chatID:         chatID,
		replyToID:      replyToID,
		logger:         logging.OrNop(logger),
		now:            time.Now,
		interval:       interval,
		window:         window,
		tasks:          make(map[string]*bgTaskTracker),
		pollerInterval: 30 * time.Second,
		doneCh:         make(chan struct{}),
	}
}

func (l *backgroundProgressListener) Close() {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.closed = true
	tasks := make([]*bgTaskTracker, 0, len(l.tasks))
	for _, t := range l.tasks {
		tasks = append(tasks, t)
	}
	l.mu.Unlock()

	// Signal the completion poller to stop.
	l.doneOnce.Do(func() { close(l.doneCh) })

	for _, t := range tasks {
		t.stop()
	}
}

// Release marks the foreground caller as done. If no background tasks are
// tracked, it closes the listener immediately. Otherwise, it defers closing
// until the last tracked task completes. A safety-net timer prevents leaks
// if a completion event is lost.
func (l *backgroundProgressListener) Release() {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.released = true
	shouldClose := len(l.tasks) == 0
	l.mu.Unlock()

	if shouldClose {
		l.Close()
		return
	}

	// Start the completion poller — polls TaskStore as a safety net in case
	// both the normal and direct event paths fail (e.g. process crash/OOM).
	go l.pollForCompletions()

	// Safety net: prevent leaks if completion event is lost.
	go func() {
		t := time.NewTimer(maxBackgroundListenerLifetime)
		defer t.Stop()
		select {
		case <-t.C:
			l.logger.Warn("backgroundProgressListener force-closing after max lifetime")
			l.Close()
		case <-l.doneCh:
		}
	}()
}

func (l *backgroundProgressListener) OnEvent(event agent.AgentEvent) {
	if l.inner != nil {
		l.inner.OnEvent(event)
	}

	switch e := event.(type) {
	case *domain.WorkflowEventEnvelope:
		l.onEnvelope(e)
	case *domain.Event:
		// Direct bypass path: BackgroundTaskManager sends completion events
		// directly here when the SerializingEventListener queue may be dead.
		// Dedup is safe: getTask returns nil after the first handler deletes the task.
		if e.Kind == types.EventBackgroundTaskCompleted {
			l.onRawCompleted(e)
		}
	}
}

func (l *backgroundProgressListener) onEnvelope(env *domain.WorkflowEventEnvelope) {
	if env == nil {
		return
	}

	switch strings.TrimSpace(env.Event) {
	case types.EventBackgroundTaskDispatched:
		l.onBackgroundDispatched(env)
	case types.EventExternalAgentProgress:
		l.onExternalProgress(env)
	case types.EventExternalInputRequested:
		l.onExternalInputRequested(env)
	case types.EventBackgroundTaskCompleted:
		l.onBackgroundCompleted(env)
	}
}

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
	text := l.buildHumanHeader(tracker, "正在后台处理中…")
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
		l.logger.Info("handleCompletion: task %s not found (already processed or not tracked)", taskID)
		return
	}
	l.logger.Info("handleCompletion: processing task %s status=%s", taskID, status)

	normalizedStatus := normalizeCompletionTaskStatus(status, errText)
	if raw := strings.TrimSpace(status); raw != "" && normalizeTaskStatus(raw) != normalizedStatus {
		l.logger.Warn("Lark background completion normalized non-terminal status: task_id=%s raw_status=%s normalized=%s", taskID, raw, normalizedStatus)
	}
	mergeStatus = normalizeMergeStatus(mergeStatus)

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
	remaining := len(l.tasks)
	completed := len(l.completedTasks)
	released := l.released
	l.mu.Unlock()

	l.logger.Info("handleCompletion: task %s done, remaining=%d completed=%d released=%v shouldClose=%v", taskID, remaining, completed, released, shouldClose)

	t.stop()

	if shouldClose {
		l.logger.Info("handleCompletion: all tasks done, sending team summary (completed=%d)", completed)
		l.sendTeamCompletionSummary()
		l.Close()
	}
}

func (l *backgroundProgressListener) flush(t *bgTaskTracker, force bool) {
	if t == nil {
		return
	}

	t.mu.Lock()
	messageID := t.progressMsgID
	status := t.status
	startedAt := t.startedAt
	taskID := t.taskID
	description := t.description
	pending := t.pendingSummary
	mergeStatus := t.mergeStatus
	last := t.lastProgress
	t.mu.Unlock()

	if messageID == "" {
		return
	}

	now := l.clock()
	elapsed := now.Sub(startedAt)
	if elapsed < 0 {
		elapsed = 0
	}

	var b strings.Builder

	switch status {
	case taskStatusWaitingInput:
		b.WriteString("后台任务等待中\n")
		l.writeDescription(&b, description)
		b.WriteString(fmt.Sprintf("⏱ 已进行 %s\n", formatElapsed(elapsed)))
		b.WriteString("\n")
		b.WriteString(pending)
	case taskStatusCompleted:
		b.WriteString("任务已完成\n")
		l.writeCompletionMeta(&b, taskID, status, mergeStatus)
		l.writeDescription(&b, description)
		b.WriteString(fmt.Sprintf("⏱ 共耗时 %s\n", formatElapsed(elapsed)))
		if utils.HasContent(pending) {
			b.WriteString("\n")
			b.WriteString(pending)
		}
	case taskStatusFailed:
		b.WriteString("任务出错了\n")
		l.writeCompletionMeta(&b, taskID, status, mergeStatus)
		l.writeDescription(&b, description)
		b.WriteString(fmt.Sprintf("⏱ 已进行 %s\n", formatElapsed(elapsed)))
		if utils.HasContent(pending) {
			b.WriteString("\n")
			b.WriteString(pending)
		}
	case taskStatusCancelled:
		b.WriteString("任务已取消\n")
		l.writeCompletionMeta(&b, taskID, status, mergeStatus)
		l.writeDescription(&b, description)
		b.WriteString(fmt.Sprintf("⏱ 已进行 %s\n", formatElapsed(elapsed)))
	default:
		// Running state.
		b.WriteString("正在后台处理中…\n")
		l.writeDescription(&b, description)
		b.WriteString(fmt.Sprintf("⏱ 已进行 %s\n", formatElapsed(elapsed)))

		// Show current activity as a friendly phrase.
		if last.currentTool != "" {
			phrase := toolPhraseForBackground(last.currentTool, int(elapsed.Seconds()))
			b.WriteString("\n最近动态：\n")
			b.WriteString(phrase)
		}
	}

	text := strings.TrimRight(b.String(), "\n")

	if err := l.g.updateMessage(l.ctx, messageID, text); err != nil {
		// If updating fails (some chats restrict updates for replies), fall back to sending a new reply.
		newID, sendErr := l.g.dispatchMessage(l.ctx, l.chatID, l.replyToID, "text", textContent(text))
		if sendErr != nil {
			l.logger.Warn("Lark background progress update failed: %v", err)
			return
		}
		t.mu.Lock()
		t.progressMsgID = newID
		t.mu.Unlock()
	}

	_ = force // reserved for future: immediate flush paths already call flush()
}

// writeDescription writes the task description line (if non-empty).
func (l *backgroundProgressListener) writeDescription(b *strings.Builder, description string) {
	if desc := strings.TrimSpace(description); desc != "" {
		b.WriteString("📋 ")
		b.WriteString(truncateForLark(desc, 120))
		b.WriteString("\n")
	}
}

func (l *backgroundProgressListener) writeCompletionMeta(b *strings.Builder, taskID, status, mergeStatus string) {
	taskID = strings.TrimSpace(taskID)
	status = normalizeTaskStatus(status)
	mergeStatus = normalizeMergeStatus(mergeStatus)
	if taskID != "" {
		b.WriteString(fmt.Sprintf("task_id: %s\n", taskID))
	}
	if status != "" {
		b.WriteString(fmt.Sprintf("status: %s\n", status))
	}
	if mergeStatus != "" {
		b.WriteString(fmt.Sprintf("merge: %s\n", mergeStatus))
	}
}

// formatElapsed formats a duration into a human-friendly Chinese string.
func formatElapsed(d time.Duration) string {
	if d < time.Minute {
		secs := int(d.Seconds())
		if secs <= 0 {
			secs = 1
		}
		return fmt.Sprintf("%d 秒", secs)
	}
	mins := int(d.Minutes())
	if mins < 60 {
		return fmt.Sprintf("%d 分钟", mins)
	}
	hours := mins / 60
	remainMins := mins % 60
	if remainMins == 0 {
		return fmt.Sprintf("%d 小时", hours)
	}
	return fmt.Sprintf("%d 小时 %d 分钟", hours, remainMins)
}

func (l *backgroundProgressListener) getTask(taskID string) *bgTaskTracker {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	return l.tasks[taskID]
}

// buildHumanHeader returns a human-friendly initial message for a background task.
func (l *backgroundProgressListener) buildHumanHeader(t *bgTaskTracker, title string) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	if desc := strings.TrimSpace(t.description); desc != "" {
		b.WriteString("📋 ")
		b.WriteString(truncateForLark(desc, 120))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (l *backgroundProgressListener) clock() time.Time {
	if l.now != nil {
		return l.now()
	}
	return time.Now()
}

func (l *backgroundProgressListener) taskInterval(agentType string) time.Duration {
	interval := l.interval
	if isCodeAgentType(agentType) {
		interval = minDuration(interval, codeBackgroundProgressInterval)
	}
	if interval <= 0 {
		return defaultBackgroundProgressInterval
	}
	return interval
}

func (l *backgroundProgressListener) taskWindow(_ string) time.Duration {
	if l.window <= 0 {
		return defaultBackgroundProgressWindow
	}
	return l.window
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

func asTime(v any) time.Time {
	t, _ := v.(time.Time)
	return t
}

func asStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case []string:
		out := make([]string, 0, len(x))
		for _, s := range x {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func normalizeMergeStatus(status string) string {
	status = strings.TrimSpace(status)
	switch strings.ToLower(status) {
	case "":
		return agent.MergeStatusNotMerged
	case "merged", "merged/success":
		return agent.MergeStatusMerged
	case "merge_failed", "merge failed":
		return agent.MergeStatusFailed
	case "not_merged", "not merged":
		return agent.MergeStatusNotMerged
	default:
		return status
	}
}

func truncateForLark(s string, max int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if max <= 0 || len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}

func isCodeAgentType(agentType string) bool {
	switch strings.ToLower(strings.TrimSpace(agentType)) {
	case "codex", "claude_code":
		return true
	default:
		return false
	}
}

// syncTaskSave persists a new task record to the TaskStore (if available).
func (l *backgroundProgressListener) syncTaskSave(taskID, description, agentType string, startedAt time.Time) {
	if l.g == nil || l.g.taskStore == nil {
		return
	}
	rec := TaskRecord{
		ChatID:      l.chatID,
		TaskID:      taskID,
		AgentType:   agentType,
		Description: description,
		Status:      taskStatusRunning,
		CreatedAt:   startedAt,
	}
	if err := l.g.taskStore.SaveTask(l.ctx, rec); err != nil {
		l.logger.Warn("Task store save failed for %s: %v", taskID, err)
	}
}

// syncTaskStatus updates a task's status in the TaskStore (if available).
func (l *backgroundProgressListener) syncTaskStatus(taskID, status string, opts ...TaskUpdateOption) {
	if l.g == nil || l.g.taskStore == nil {
		return
	}
	if err := l.g.taskStore.UpdateStatus(l.ctx, taskID, status, opts...); err != nil {
		l.logger.Warn("Task store status update failed for %s: %v", taskID, err)
	}
}

// sendTeamCompletionSummary fires a goroutine to generate and send a
// team-level summary message when all tracked background tasks have finished.
// Follows the slowProgressSummaryListener pattern: lightweight LLM call +
// dispatchMessage, no session pollution.
func (l *backgroundProgressListener) sendTeamCompletionSummary() {
	if l.g == nil {
		l.logger.Info("sendTeamCompletionSummary: skipped (gateway nil)")
		return
	}
	if !l.isTeamSummaryEnabled() {
		l.logger.Info("sendTeamCompletionSummary: skipped (disabled)")
		return
	}

	l.mu.Lock()
	tasks := make([]completedTaskRecord, len(l.completedTasks))
	copy(tasks, l.completedTasks)
	l.mu.Unlock()

	if len(tasks) < teamCompletionSummaryMinTasks {
		l.logger.Info("sendTeamCompletionSummary: skipped (only %d tasks, need %d)", len(tasks), teamCompletionSummaryMinTasks)
		return
	}

	l.logger.Info("sendTeamCompletionSummary: dispatching summary for %d tasks", len(tasks))
	go l.doSendTeamCompletionSummary(tasks)
}

func (l *backgroundProgressListener) doSendTeamCompletionSummary(tasks []completedTaskRecord) {
	summary := l.buildTeamSummary(tasks)
	if summary == "" {
		return
	}
	if _, err := l.g.dispatchMessage(l.ctx, l.chatID, l.replyToID, "text", textContent(summary)); err != nil {
		l.logger.Warn("Team completion summary send failed: %v", err)
	}
}

// buildTeamSummary tries LLM generation first, falls back to template.
func (l *backgroundProgressListener) buildTeamSummary(tasks []completedTaskRecord) string {
	if l.g.llmFactory != nil {
		timeout := l.teamSummaryLLMTimeout()
		ctx, cancel := context.WithTimeout(l.ctx, timeout)
		defer cancel()

		summary, err := l.generateTeamLLMSummary(ctx, tasks)
		if err == nil && isValidTeamSummary(summary) {
			return truncateForLark(summary, teamCompletionSummaryMaxReplyChars)
		}
		l.logger.Warn("Team completion LLM summary failed, using fallback: %v", err)
	}
	return l.buildTeamSummaryFallback(tasks)
}

func (l *backgroundProgressListener) generateTeamLLMSummary(ctx context.Context, tasks []completedTaskRecord) (string, error) {
	client, _, err := llmclient.GetClientFromProfile(l.g.llmFactory, l.resolveTeamSummaryProfile(), nil, false)
	if err != nil {
		return "", err
	}

	systemPrompt := "你是后台任务汇总助手。把多个后台任务的完成结果整理成一段自然中文汇总，简洁友好。" +
		"包含：总任务数、成功/失败数、总耗时、每个任务一句话结果。不使用 Markdown。"
	userPrompt := l.buildTeamLLMPrompt(tasks)

	resp, err := client.Complete(ctx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2,
		MaxTokens:   300,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func (l *backgroundProgressListener) buildTeamLLMPrompt(tasks []completedTaskRecord) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("共 %d 个后台任务已全部结束：\n", len(tasks)))

	var totalDuration time.Duration
	for i, t := range tasks {
		totalDuration += t.duration
		b.WriteString(fmt.Sprintf("%d. [%s] %s (耗时 %s)", i+1, t.status, t.description, formatElapsed(t.duration)))
		if t.errText != "" {
			b.WriteString(fmt.Sprintf("\n   错误：%s", truncateForLark(t.errText, 200)))
		} else if t.answer != "" {
			b.WriteString(fmt.Sprintf("\n   结果：%s", truncateForLark(t.answer, 200)))
		}
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("\n总耗时（并行）约 %s\n", formatElapsed(maxDuration(tasks))))
	b.WriteString("请输出一段对用户的中文汇总，语气自然友好。")

	prompt := b.String()
	if len(prompt) > teamCompletionSummaryMaxPromptChars {
		return prompt[:teamCompletionSummaryMaxPromptChars]
	}
	return prompt
}

func (l *backgroundProgressListener) buildTeamSummaryFallback(tasks []completedTaskRecord) string {
	var succeeded, failed, cancelled int
	for _, t := range tasks {
		switch t.status {
		case taskStatusCompleted:
			succeeded++
		case taskStatusFailed:
			failed++
		case taskStatusCancelled:
			cancelled++
		default:
			succeeded++ // treat unknown terminal as success
		}
	}

	var b strings.Builder
	b.WriteString("全部后台任务已完成\n\n")
	b.WriteString(fmt.Sprintf("共 %d 个任务", len(tasks)))

	parts := make([]string, 0, 3)
	if succeeded > 0 {
		parts = append(parts, fmt.Sprintf("成功 %d", succeeded))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("失败 %d", failed))
	}
	if cancelled > 0 {
		parts = append(parts, fmt.Sprintf("取消 %d", cancelled))
	}
	if len(parts) > 0 {
		b.WriteString("（")
		b.WriteString(strings.Join(parts, "，"))
		b.WriteString("）")
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("⏱ 总耗时 %s\n", formatElapsed(maxDuration(tasks))))

	for _, t := range tasks {
		b.WriteString("\n")
		switch t.status {
		case taskStatusCompleted:
			b.WriteString("✅ ")
		case taskStatusFailed:
			b.WriteString("❌ ")
		case taskStatusCancelled:
			b.WriteString("⏹ ")
		default:
			b.WriteString("✅ ")
		}
		desc := strings.TrimSpace(t.description)
		if desc == "" {
			desc = t.taskID
		}
		b.WriteString(truncateForLark(desc, 80))
		if t.errText != "" {
			b.WriteString(fmt.Sprintf("\n   错误：%s", truncateForLark(t.errText, 200)))
		} else if t.answer != "" {
			b.WriteString(fmt.Sprintf("\n   结果：%s", truncateForLark(t.answer, 200)))
		}
	}

	return truncateForLark(strings.TrimRight(b.String(), "\n"), teamCompletionSummaryMaxReplyChars)
}

func (l *backgroundProgressListener) isTeamSummaryEnabled() bool {
	if l.g == nil {
		return false
	}
	if l.g.cfg.TeamCompletionSummaryEnabled != nil {
		return *l.g.cfg.TeamCompletionSummaryEnabled
	}
	return true // default enabled
}

func (l *backgroundProgressListener) teamSummaryLLMTimeout() time.Duration {
	if l.g != nil && l.g.cfg.TeamCompletionSummaryLLMTimeout > 0 {
		return l.g.cfg.TeamCompletionSummaryLLMTimeout
	}
	return defaultTeamCompletionSummaryLLMTimeout
}

func (l *backgroundProgressListener) resolveTeamSummaryProfile() runtimeconfig.LLMProfile {
	if l.ctx != nil {
		if selection, ok := appcontext.GetLLMSelection(l.ctx); ok {
			if utils.HasContent(selection.Provider) && utils.HasContent(selection.Model) {
				return runtimeconfig.LLMProfile{
					Provider: selection.Provider,
					Model:    selection.Model,
					APIKey:   selection.APIKey,
					BaseURL:  selection.BaseURL,
					Headers:  selection.Headers,
				}
			}
		}
	}
	return l.g.llmProfile
}

// isValidTeamSummary checks that an LLM-generated summary is usable.
func isValidTeamSummary(summary string) bool {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "empty response:") || strings.HasPrefix(lower, "empty completion:") {
		return false
	}
	return true
}

// maxDuration returns the maximum duration across all completed tasks
// (approximation of wall-clock time since tasks run in parallel).
func maxDuration(tasks []completedTaskRecord) time.Duration {
	var max time.Duration
	for _, t := range tasks {
		if t.duration > max {
			max = t.duration
		}
	}
	return max
}

// pollForCompletions periodically checks TaskStore for tasks that completed
// but whose completion event was never received (e.g. process crash, OOM).
func (l *backgroundProgressListener) pollForCompletions() {
	interval := l.pollerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.checkTaskStoreCompletions()
		case <-l.doneCh:
			return
		}
	}
}

func (l *backgroundProgressListener) checkTaskStoreCompletions() {
	if l.g == nil || l.g.taskStore == nil {
		return
	}

	l.mu.Lock()
	taskIDs := make([]string, 0, len(l.tasks))
	for id := range l.tasks {
		taskIDs = append(taskIDs, id)
	}
	l.mu.Unlock()

	if len(taskIDs) == 0 {
		return
	}

	for _, taskID := range taskIDs {
		rec, ok, err := l.g.taskStore.GetTask(l.ctx, taskID)
		if err != nil || !ok {
			continue
		}
		if isTerminalTaskStatus(rec.Status) {
			l.logger.Info("Poller detected completed task %s (status=%s), delivering notification", taskID, rec.Status)
			l.handleCompletion(taskID, rec.Status, rec.AnswerPreview, rec.Error, rec.MergeStatus, rec.TokensUsed)
		}
	}
}
