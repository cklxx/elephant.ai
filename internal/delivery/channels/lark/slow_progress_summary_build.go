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

func (l *slowProgressSummaryListener) buildSummary(signals []slowProgressSignal, elapsed time.Duration) string {
	fallback := l.buildFallbackSummary(signals, elapsed)
	toolLines := buildHumanToolSignalLines(signals, 3)
	if l.gateway == nil || l.gateway.llmFactory == nil {
		return appendHumanToolSection(fallback, toolLines)
	}
	profile := l.resolveProfile()
	if utils.IsBlank(profile.Provider) || utils.IsBlank(profile.Model) {
		return appendHumanToolSection(fallback, toolLines)
	}

	baseCtx := context.Background()
	if l.ctx != nil {
		baseCtx = context.WithoutCancel(l.ctx)
	}
	llmCtx, cancel := context.WithTimeout(baseCtx, defaultSlowProgressSummaryLLMTimeout)
	defer cancel()

	summary, err := l.generateLLMSummary(llmCtx, signals, elapsed)
	if err != nil {
		if l.gateway.logger != nil {
			l.gateway.logger.Warn("Lark slow progress summary LLM fallback: %v", err)
		}
		return appendHumanToolSection(fallback, toolLines)
	}
	summary = strings.TrimSpace(summary)
	if !isValidSlowProgressLLMSummary(summary) {
		return appendHumanToolSection(fallback, toolLines)
	}
	return truncateForLark(summary, slowProgressSummaryMaxReplyChars)
}

// resolveProfile returns the pinned subscription profile from the task context
// if available, otherwise falls back to the gateway's shared runtime profile.
func (l *slowProgressSummaryListener) resolveProfile() runtimeconfig.LLMProfile {
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
	return l.gateway.llmProfile
}

func (l *slowProgressSummaryListener) generateLLMSummary(
	ctx context.Context,
	signals []slowProgressSignal,
	elapsed time.Duration,
) (string, error) {
	client, _, err := llmclient.GetClientFromProfile(l.gateway.llmFactory, l.resolveProfile(), nil, false)
	if err != nil {
		return "", err
	}
	systemPrompt := "你是运行进展播报助手。把事件整理成自然中文进展同步，输出 1 段 2-3 句，不使用列表。不要出现内部节点ID或键名（如 react:iter:...、call_xxx、step_description、payload），只描述已发生进展与当前状态，不给最终结论。"
	userPrompt := l.buildLLMPrompt(signals, elapsed)

	resp, err := client.Complete(ctx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2,
		MaxTokens:   220,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func (l *slowProgressSummaryListener) buildLLMPrompt(signals []slowProgressSignal, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString("任务还在执行中。\n")
	b.WriteString("已运行时长：")
	b.WriteString(formatDuration(elapsed))
	b.WriteString("\n最近事件：\n")
	if len(signals) == 0 {
		b.WriteString("- 暂无可用事件细节（可能仍在准备上下文或等待工具返回）。\n")
	} else {
		for i, signal := range tailSignals(signals, 8) {
			offset := signal.at.Sub(l.startedAt)
			if offset < 0 {
				offset = 0
			}
			b.WriteString(fmt.Sprintf("%d. [t+%s] %s\n", i+1, formatDuration(offset), signal.text))
		}
	}
	toolLines := buildHumanToolSignalLines(signals, 3)
	if len(toolLines) > 0 {
		b.WriteString("最近工具调用（人话）：\n")
		for i, line := range toolLines {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, line))
		}
	}
	b.WriteString("请输出一段对用户的中文同步，语气自然口语化。")
	prompt := b.String()
	if len(prompt) > slowProgressSummaryMaxPromptChars {
		return prompt[:slowProgressSummaryMaxPromptChars]
	}
	return prompt
}

func (l *slowProgressSummaryListener) buildFallbackSummary(signals []slowProgressSignal, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString("任务已运行 ")
	b.WriteString(formatDuration(elapsed))
	b.WriteString("，仍在执行中。\n")
	if len(signals) == 0 {
		b.WriteString("最近进展：正在准备上下文或等待工具返回。\n")
	} else {
		b.WriteString("最近进展：")
		parts := make([]string, 0, 3)
		for _, signal := range tailSignals(signals, 3) {
			parts = append(parts, signal.text)
		}
		b.WriteString(strings.Join(parts, "；"))
		b.WriteString("\n")
	}
	b.WriteString("我会在完成后继续给你最终结果。")
	return truncateForLark(appendHumanToolSection(b.String(), buildHumanToolSignalLines(signals, 3)), slowProgressSummaryMaxReplyChars)
}

func tailSignals(signals []slowProgressSignal, n int) []slowProgressSignal {
	if len(signals) <= n {
		out := make([]slowProgressSignal, len(signals))
		copy(out, signals)
		return out
	}
	out := make([]slowProgressSignal, n)
	copy(out, signals[len(signals)-n:])
	return out
}

func buildSlowSummaryIntervals(first time.Duration) []time.Duration {
	if first <= 0 {
		first = defaultSlowProgressSummaryDelay
	}
	second := first * 2
	third := first * 6
	if second <= 0 {
		second = first
	}
	if third < second {
		third = second
	}
	return []time.Duration{first, second, third}
}

func appendHumanToolSection(base string, lines []string) string {
	base = strings.TrimSpace(base)
	if len(lines) == 0 {
		return base
	}
	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\n最近工具调用（人话）：\n")
	for _, line := range lines {
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildHumanToolSignalLines(signals []slowProgressSignal, max int) []string {
	if max <= 0 || len(signals) == 0 {
		return nil
	}
	lines := make([]string, 0, max)
	tail := tailSignals(signals, 16)
	for i := len(tail) - 1; i >= 0 && len(lines) < max; i-- {
		name, state, errText, ok := parseToolSignalLine(tail[i].text)
		if !ok {
			continue
		}
		selector := len(lines) + int(tail[i].at.Unix()%7)
		phrase := toolPhraseForBackground(name, selector)
		name = strings.TrimSpace(name)
		switch state {
		case "started":
			lines = append(lines, fmt.Sprintf("%s（%s）", phrase, name))
		case "completed":
			lines = append(lines, fmt.Sprintf("已完成 %s（%s）", phrase, name))
		case "failed":
			if errText != "" {
				lines = append(lines, fmt.Sprintf("%s（%s）失败：%s", phrase, name, truncateForLark(errText, 80)))
			} else {
				lines = append(lines, fmt.Sprintf("%s（%s）执行失败", phrase, name))
			}
		}
	}
	return lines
}

func parseToolSignalLine(text string) (name string, state string, errText string, ok bool) {
	text = strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(text, "开始工具："):
		return strings.TrimSpace(strings.TrimPrefix(text, "开始工具：")), "started", "", true
	case strings.HasPrefix(text, "完成工具："):
		return strings.TrimSpace(strings.TrimPrefix(text, "完成工具：")), "completed", "", true
	case strings.HasPrefix(text, "工具失败："):
		body := strings.TrimSpace(strings.TrimPrefix(text, "工具失败："))
		if body == "" {
			return "", "", "", false
		}
		name = body
		if idx := strings.Index(body, "（"); idx >= 0 {
			name = strings.TrimSpace(body[:idx])
			rest := strings.TrimSpace(body[idx+len("（"):])
			rest = strings.TrimSuffix(rest, ")")
			rest = strings.TrimSuffix(rest, "）")
			errText = strings.TrimSpace(rest)
		}
		return name, "failed", errText, true
	default:
		return "", "", "", false
	}
}
