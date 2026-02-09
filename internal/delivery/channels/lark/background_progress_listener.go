package lark

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/logging"
)

const (
	defaultBackgroundProgressInterval = 10 * time.Minute
	defaultBackgroundProgressWindow   = 10 * time.Minute
	codeBackgroundProgressInterval    = 3 * time.Minute
	maxBackgroundListenerLifetime     = 4 * time.Hour
)

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
	case *domain.BackgroundTaskCompletedEvent:
		// Direct bypass path: BackgroundTaskManager sends completion events
		// directly here when the SerializingEventListener queue may be dead.
		// Dedup is safe: getTask returns nil after the first handler deletes the task.
		l.onRawCompleted(e)
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
		status:      "running",
		interval:    l.taskInterval(agentType),
		window:      l.taskWindow(agentType),
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}
	l.tasks[taskID] = tracker
	l.mu.Unlock()

	// Send initial message (reply to original message when replyToID is provided).
	intervalLabel := formatMinutes(tracker.interval)
	windowLabel := formatMinutes(tracker.window)
	text := l.buildHeader(tracker, "[后台任务进行中]") + "\n\n" + fmt.Sprintf("已启动，未结束前每%s自动更新一次（最近%s窗口）。", intervalLabel, windowLabel)
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

	rec := progressRecord{
		ts:          env.Timestamp(),
		currentTool: asString(env.Payload["current_tool"]),
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
	if t.status == "completed" || t.status == "failed" || t.status == "cancelled" {
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
	l.syncTaskStatus(taskID, "running", WithTokensUsed(tokensUsed))
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
	if t.status != "completed" && t.status != "failed" && t.status != "cancelled" {
		t.status = "waiting_input"
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
	status := strings.TrimSpace(asString(env.Payload["status"]))
	answer := asString(env.Payload["answer"])
	errText := asString(env.Payload["error"])
	tokensUsed := asInt(env.Payload["tokens_used"])
	l.handleCompletion(taskID, status, answer, errText, tokensUsed)
}

// onRawCompleted handles BackgroundTaskCompletedEvent delivered directly by
// BackgroundTaskManager (bypassing SerializingEventListener). Dedup is safe
// because getTask returns nil after the first handler deletes the task.
func (l *backgroundProgressListener) onRawCompleted(e *domain.BackgroundTaskCompletedEvent) {
	if e == nil {
		return
	}
	l.handleCompletion(e.TaskID, e.Status, e.Answer, e.Error, e.TokensUsed)
}

// handleCompletion is the shared completion handler for both envelope and raw event paths.
func (l *backgroundProgressListener) handleCompletion(taskID, status, answer, errText string, tokensUsed int) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}

	t := l.getTask(taskID)
	if t == nil {
		return
	}

	status = strings.TrimSpace(status)

	t.mu.Lock()
	if status != "" {
		t.status = status
	} else {
		t.status = "completed"
	}
	// Stash result in pendingSummary so flush can format without racing payload.
	if errText != "" {
		t.pendingSummary = truncateForLark(errText, 1500)
	} else {
		t.pendingSummary = truncateForLark(answer, 1500)
	}
	t.mu.Unlock()

	l.flush(t, true)

	// Sync final status to TaskStore.
	finalStatus := status
	if finalStatus == "" {
		finalStatus = "completed"
	}
	var updateOpts []TaskUpdateOption
	if errText != "" {
		updateOpts = append(updateOpts, WithErrorText(truncateForLark(errText, 1500)))
	}
	if answer != "" {
		updateOpts = append(updateOpts, WithAnswerPreview(truncateForLark(answer, 1500)))
	}
	if tokensUsed > 0 {
		updateOpts = append(updateOpts, WithTokensUsed(tokensUsed))
	}
	l.syncTaskStatus(taskID, finalStatus, updateOpts...)

	l.mu.Lock()
	delete(l.tasks, taskID)
	shouldClose := l.released && len(l.tasks) == 0
	l.mu.Unlock()

	t.stop()

	if shouldClose {
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
	description := t.description
	agentType := t.agentType
	pending := t.pendingSummary
	last := t.lastProgress
	recent := append([]progressRecord(nil), t.recent...)
	window := t.window
	t.mu.Unlock()

	if messageID == "" {
		return
	}

	now := l.clock()
	elapsed := now.Sub(startedAt)
	if elapsed < 0 {
		elapsed = 0
	}

	title := "[后台任务进行中]"
	if status == "waiting_input" {
		title = "[后台任务等待输入]"
	} else if status == "completed" {
		title = "[后台任务已完成]"
	} else if status == "failed" {
		title = "[后台任务失败]"
	} else if status == "cancelled" {
		title = "[后台任务已取消]"
	}

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("task_id=%s agent=%s\n", t.taskID, nonEmpty(agentType, "unknown")))
	if strings.TrimSpace(description) != "" {
		b.WriteString(fmt.Sprintf("desc=%s\n", truncateForLark(description, 120)))
	}
	b.WriteString(fmt.Sprintf("elapsed=%.0fm\n", elapsed.Minutes()))

	if status == "waiting_input" {
		b.WriteString("\n需要输入：\n")
		b.WriteString(pending)
		b.WriteString("\n")
	} else if status == "completed" || status == "failed" || status == "cancelled" {
		b.WriteString("\n结果：\n")
		b.WriteString(pending)
		b.WriteString("\n")
	} else {
		if window <= 0 {
			window = l.window
		}
		windowStart := now.Add(-window)
		windowRecords := make([]progressRecord, 0, len(recent))
		for _, r := range recent {
			if !r.ts.Before(windowStart) {
				windowRecords = append(windowRecords, r)
			}
		}

		b.WriteString("\n最近")
		b.WriteString(formatMinutes(window))
		b.WriteString("：\n")
		if len(windowRecords) == 0 {
			b.WriteString("- 无新增进展\n")
		} else {
			first := windowRecords[0]
			lastRec := windowRecords[len(windowRecords)-1]

			delta := 0
			if first.tokensUsed > 0 && lastRec.tokensUsed >= first.tokensUsed {
				delta = lastRec.tokensUsed - first.tokensUsed
			}
			if lastRec.tokensUsed > 0 {
				b.WriteString(fmt.Sprintf("- tokens: +%d / total %d\n", delta, lastRec.tokensUsed))
			}

			files := mergeFiles(windowRecords)
			if len(files) > 0 {
				b.WriteString("- files: ")
				b.WriteString(strings.Join(files, ", "))
				b.WriteString("\n")
			}

			tools := mergeTools(windowRecords)
			if len(tools) > 0 {
				b.WriteString("- tools: ")
				b.WriteString(strings.Join(tools, ", "))
				b.WriteString("\n")
			}

			if strings.TrimSpace(lastRec.currentArgs) != "" {
				b.WriteString("- last: ")
				b.WriteString(lastRec.currentArgs)
				b.WriteString("\n")
			}
		}

		b.WriteString("\n当前状态：\n")
		if last.currentTool != "" {
			b.WriteString(fmt.Sprintf("- %s", last.currentTool))
			if strings.TrimSpace(last.currentArgs) != "" {
				b.WriteString(": ")
				b.WriteString(last.currentArgs)
			}
			b.WriteString("\n")
		}
		if !last.activity.IsZero() {
			b.WriteString(fmt.Sprintf("- last_activity=%s\n", last.activity.Format(time.RFC3339)))
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

func (l *backgroundProgressListener) getTask(taskID string) *bgTaskTracker {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	return l.tasks[taskID]
}

func (l *backgroundProgressListener) buildHeader(t *bgTaskTracker, title string) string {
	elapsed := l.clock().Sub(t.startedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("task_id=%s agent=%s\n", t.taskID, nonEmpty(t.agentType, "unknown")))
	if strings.TrimSpace(t.description) != "" {
		b.WriteString(fmt.Sprintf("desc=%s\n", truncateForLark(t.description, 120)))
	}
	b.WriteString(fmt.Sprintf("elapsed=%.0fm", elapsed.Minutes()))
	return b.String()
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

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
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

func mergeFiles(records []progressRecord) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, r := range records {
		for _, f := range r.files {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			out = append(out, f)
		}
	}
	sort.Strings(out)
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func mergeTools(records []progressRecord) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, r := range records {
		tool := strings.TrimSpace(r.currentTool)
		if tool == "" {
			continue
		}
		if _, ok := seen[tool]; ok {
			continue
		}
		seen[tool] = struct{}{}
		out = append(out, tool)
	}
	sort.Strings(out)
	if len(out) > 6 {
		out = out[:6]
	}
	return out
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

func formatMinutes(d time.Duration) string {
	if d <= 0 {
		return "1分钟"
	}
	mins := int(math.Ceil(d.Minutes()))
	if mins <= 0 {
		mins = 1
	}
	return fmt.Sprintf("%d分钟", mins)
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
		Status:      "running",
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
		if rec.Status == "completed" || rec.Status == "failed" || rec.Status == "cancelled" {
			l.logger.Info("Poller detected completed task %s (status=%s), delivering notification", taskID, rec.Status)
			l.handleCompletion(taskID, rec.Status, rec.AnswerPreview, rec.Error, rec.TokensUsed)
		}
	}
}
