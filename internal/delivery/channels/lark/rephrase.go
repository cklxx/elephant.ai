package lark

import (
	"context"
	"strings"

	"alex/internal/shared/utils"
)

type rephraseKind int

const (
	rephraseBackground rephraseKind = iota
	rephraseForeground

	rephraseMaxInput = 2000
)

const rephraseBackgroundSystemPrompt = `把后台任务完成结果改写为简洁自然的中文消息。
去掉 task_id、status、merge 等技术字段，保留关键结论、具体成果、耗时。
2-5 句话，用 **加粗** 标注关键结论。`

const rephraseForegroundSystemPrompt = `把 AI 回答改写为更简洁易读的版本。
结构：结论/结果在第一句，关键上下文在后，细节只保留必要的。
保留所有关键信息和文件路径，去除冗余推理过程和重复陈述。
使用 markdown 格式增强可读性：**加粗**关键结论、用列表整理要点、` + "`" + `代码` + "`" + `标注路径和命令。
不要使用标题（## ）。`

// sanitizeErrorForUser strips Go error-chain prefixes, extracts the LLM
// provider/model tag when present, and maps known technical error patterns to
// user-friendly Chinese. Callers must pass the raw error string only — do NOT
// include UI prefixes like "执行失败：".
// Used when LLM narration is unavailable (e.g. the LLM itself is failing).
func sanitizeErrorForUser(errText string) string {
	if errText == "" {
		return errText
	}

	// Iteratively strip common Go error-chain technical prefixes.
	for {
		lower := strings.ToLower(errText)
		stripped := false
		for _, p := range []string{
			"task execution failed: ",
			"think step failed: ",
			"agent run failed: ",
			"step failed: ",
			"llm call failed: ",
		} {
			if strings.HasPrefix(lower, p) {
				errText = errText[len(p):]
				stripped = true
				break
			}
		}
		if !stripped {
			break
		}
	}

	// Extract LLM provider/model tag, e.g. "[anthropic/claude-sonnet-4-6] ".
	// Keep it so the user can see which model/provider failed.
	modelTag := ""
	if strings.HasPrefix(errText, "[") {
		if idx := strings.Index(errText, "] "); idx >= 0 {
			modelTag = errText[1:idx] // e.g. "anthropic/claude-sonnet-4-6"
			errText = errText[idx+2:]
		}
	}

	// Strip trailing retry/streaming noise added by retry_client.go.
	for _, suffix := range []string{
		" Streaming request failed after",
		" Retried ",
	} {
		if i := strings.Index(errText, suffix); i > 0 {
			errText = strings.TrimSpace(errText[:i])
		}
	}

	// Format a model label for inclusion in Chinese messages, e.g. "(claude-sonnet-4-6)".
	modelLabel := ""
	if modelTag != "" {
		// Use just the model part after the slash when provider is obvious.
		label := modelTag
		if i := strings.LastIndex(modelTag, "/"); i >= 0 {
			label = modelTag[i+1:]
		}
		modelLabel = "（" + label + "）"
	}

	lower := strings.ToLower(errText)
	switch {
	case strings.Contains(lower, "authentication failed") ||
		strings.Contains(lower, "please verify your api key") ||
		strings.Contains(lower, "unauthorized"):
		return "AI 服务认证失败" + modelLabel + "，请检查 API 密钥配置"
	case strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "too many requests"):
		return "AI 服务请求频率超限" + modelLabel + "，请稍后重试"
	case strings.Contains(lower, "context_length_exceeded") ||
		strings.Contains(lower, "context window") ||
		strings.Contains(lower, "maximum context length"):
		return "输入内容超出 AI 模型上下文长度限制" + modelLabel
	case strings.Contains(lower, "model not found") ||
		strings.Contains(lower, "not_found_error"):
		return "AI 模型配置错误" + modelLabel + "，请检查模型名称设置"
	case strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "deadline exceeded"):
		return "AI 服务请求超时" + modelLabel + "，请稍后重试"
	case strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "dial tcp"):
		return "网络连接失败" + modelLabel + "，请检查网络状态后重试"
	case strings.Contains(lower, "service unavailable"):
		return "AI 服务暂时不可用" + modelLabel + "，请稍后重试"
	}

	// Unknown error: return cleaned text with model tag, capped at readable length.
	result := errText
	if modelLabel != "" {
		result = modelLabel + " " + result
	}
	if len([]rune(result)) > 150 {
		return utils.TruncateWithSuffix(result, 150, "…")
	}
	return result
}

func (g *Gateway) rephraseForUser(ctx context.Context, rawText string, kind rephraseKind) string {
	if g == nil || g.llmFactory == nil {
		return rawText
	}
	if utils.IsBlank(g.llmProfile.Provider) || utils.IsBlank(g.llmProfile.Model) {
		return rawText
	}
	if g.cfg.RephraseEnabled != nil && !*g.cfg.RephraseEnabled {
		return rawText
	}

	input := truncateForLark(rawText, rephraseMaxInput)
	if utils.IsBlank(input) {
		return rawText
	}

	var systemPrompt string
	switch kind {
	case rephraseBackground:
		systemPrompt = rephraseBackgroundSystemPrompt
	case rephraseForeground:
		systemPrompt = rephraseForegroundSystemPrompt
	default:
		return rawText
	}

	result, err := g.narrateWithLLM(ctx, systemPrompt, input, narrateOpts{
		temperature: 0.3,
		maxTokens:   400,
	})
	if err != nil || result == "" {
		return rawText
	}
	return result
}
