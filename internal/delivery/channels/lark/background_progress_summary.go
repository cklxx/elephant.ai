package lark

import (
	"context"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/utils"
)

// sendTeamCompletionSummary fires a goroutine to generate and send a
// team-level summary message when all tracked background tasks have finished.
// Follows the slowProgressSummaryListener pattern: lightweight LLM call +
// dispatchMessage, no session pollution.
func (l *backgroundProgressListener) sendTeamCompletionSummary() {
	if l.g == nil {
		return
	}
	if !l.isTeamSummaryEnabled() {
		return
	}

	l.mu.Lock()
	tasks := make([]completedTaskRecord, len(l.completedTasks))
	copy(tasks, l.completedTasks)
	l.mu.Unlock()

	if len(tasks) < teamCompletionSummaryMinTasks {
		return
	}

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
		"包含：总任务数、成功/失败数、总耗时、每个任务一句话结果。不使用 Markdown 和 emoji。"
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
	b.WriteString(fmt.Sprintf("分配的 %d 个后台任务全部结束了。", len(tasks)))
	if failed == 0 && cancelled == 0 {
		b.WriteString("全部成功完成")
	} else {
		parts := []string{fmt.Sprintf("%d 个成功", succeeded)}
		if failed > 0 {
			parts = append(parts, fmt.Sprintf("%d 个失败", failed))
		}
		if cancelled > 0 {
			parts = append(parts, fmt.Sprintf("%d 个取消", cancelled))
		}
		b.WriteString(strings.Join(parts, "，"))
	}
	b.WriteString(fmt.Sprintf("，总耗时约 %s。\n\n", formatElapsed(maxDuration(tasks))))

	for _, t := range tasks {
		desc := strings.TrimSpace(t.description)
		if desc == "" {
			desc = t.taskID
		}
		switch t.status {
		case taskStatusCompleted:
			if t.answer != "" {
				b.WriteString(fmt.Sprintf("%s：%s\n", truncateForLark(desc, 80), truncateForLark(t.answer, 100)))
			} else {
				b.WriteString(fmt.Sprintf("%s：已完成\n", truncateForLark(desc, 80)))
			}
		case taskStatusFailed:
			b.WriteString(fmt.Sprintf("%s：失败（%s）\n", truncateForLark(desc, 80), truncateForLark(t.errText, 100)))
		case taskStatusCancelled:
			b.WriteString(fmt.Sprintf("%s：已取消\n", truncateForLark(desc, 80)))
		default:
			b.WriteString(fmt.Sprintf("%s：已完成\n", truncateForLark(desc, 80)))
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
