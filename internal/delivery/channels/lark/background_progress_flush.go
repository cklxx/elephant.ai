package lark

import (
	"fmt"
	"strings"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/utils"
	"alex/internal/shared/uxphrases"
)

func (l *backgroundProgressListener) flush(t *bgTaskTracker, force bool) {
	if t == nil {
		return
	}

	t.mu.Lock()
	messageID := t.progressMsgID
	status := t.status
	startedAt := t.startedAt
	description := t.description
	pending := t.pendingSummary
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
	desc := strings.TrimSpace(description)

	switch status {
	case taskStatusWaitingInput:
		if desc != "" {
			b.WriteString(fmt.Sprintf("「%s」需要你确认，已等待 %s", truncateForLark(desc, 80), formatElapsed(elapsed)))
		} else {
			b.WriteString(fmt.Sprintf("后台任务需要你确认，已等待 %s", formatElapsed(elapsed)))
		}
		if utils.HasContent(pending) {
			b.WriteString("\n\n")
			b.WriteString(pending)
		}
	case taskStatusCompleted:
		// Build factual block, then let LLM narrate.
		var raw strings.Builder
		raw.WriteString("状态：完成\n")
		if desc != "" {
			raw.WriteString(fmt.Sprintf("任务：%s\n", truncateForLark(desc, 120)))
		}
		raw.WriteString(fmt.Sprintf("耗时：%s\n", formatElapsed(elapsed)))
		if utils.HasContent(pending) {
			raw.WriteString(fmt.Sprintf("结果：%s\n", pending))
		}
		b.WriteString(l.g.rephraseForUser(l.ctx, raw.String(), rephraseBackground))
	case taskStatusFailed:
		// Build factual block, then let LLM narrate.
		var raw strings.Builder
		raw.WriteString("状态：失败\n")
		if desc != "" {
			raw.WriteString(fmt.Sprintf("任务：%s\n", truncateForLark(desc, 120)))
		}
		raw.WriteString(fmt.Sprintf("已进行：%s\n", formatElapsed(elapsed)))
		if utils.HasContent(pending) {
			raw.WriteString(fmt.Sprintf("原因：%s\n", pending))
		}
		b.WriteString(l.g.rephraseForUser(l.ctx, raw.String(), rephraseBackground))
	case taskStatusCancelled:
		var raw strings.Builder
		raw.WriteString("状态：已取消\n")
		if desc != "" {
			raw.WriteString(fmt.Sprintf("任务：%s\n", truncateForLark(desc, 120)))
		}
		raw.WriteString(fmt.Sprintf("已进行：%s\n", formatElapsed(elapsed)))
		b.WriteString(l.g.rephraseForUser(l.ctx, raw.String(), rephraseBackground))
	default:
		// Running state — updated frequently, use template directly.
		if desc != "" {
			b.WriteString(fmt.Sprintf("正在处理「%s」，已进行 %s", truncateForLark(desc, 80), formatElapsed(elapsed)))
		} else {
			b.WriteString(fmt.Sprintf("后台任务处理中，已进行 %s", formatElapsed(elapsed)))
		}
		if last.currentTool != "" {
			phrase := uxphrases.ToolPhrase(last.currentTool, int(elapsed.Seconds()))
			b.WriteString(fmt.Sprintf("，目前%s", phrase))
		}
	}

	text := strings.TrimRight(b.String(), "\n")

	if err := l.g.updateMessage(l.ctx, messageID, "text", textContent(text)); err != nil {
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
	switch utils.TrimLower(agentType) {
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
