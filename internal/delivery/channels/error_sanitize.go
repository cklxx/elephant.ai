package channels

import "strings"

// SanitizeErrorForUser strips Go error-chain prefixes, extracts the LLM
// provider/model tag when present, and maps known technical error patterns to
// user-friendly Chinese messages.
//
// Callers must pass the raw error string only — do NOT include UI prefixes
// like "执行失败：". Used when LLM narration is unavailable (e.g. the LLM
// itself is failing).
func SanitizeErrorForUser(errText string) string {
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

	// Strip "task execution failed (task_id=..., session_id=...): " with IDs.
	if lower := strings.ToLower(errText); strings.HasPrefix(lower, "task execution failed (") {
		if idx := strings.Index(errText, "): "); idx >= 0 {
			errText = errText[idx+3:]
		}
	}

	// Extract LLM provider/model tag, e.g. "[anthropic/claude-sonnet-4-6] ".
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

	// Format a model label for inclusion in Chinese messages.
	modelLabel := ""
	if modelTag != "" {
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
		strings.Contains(lower, "dial tcp") ||
		strings.Contains(lower, "failed to reach"):
		return "网络连接失败" + modelLabel + "，请检查网络状态后重试"

	case strings.Contains(lower, "unavailable") ||
		strings.Contains(lower, "bad gateway") ||
		strings.Contains(lower, "request was rejected"):
		return "AI 服务暂时不可用" + modelLabel + "，请稍后重试"

	case strings.Contains(lower, "request rejected"):
		return "AI 服务请求被拒绝" + modelLabel + "，请检查请求参数"

	case strings.Contains(lower, "nil response") ||
		strings.Contains(lower, "empty response"):
		return "AI 服务返回空结果" + modelLabel + "，请重试"

	case strings.Contains(lower, "responses input is empty"):
		return "消息内容为空，无法发送给 AI" + modelLabel
	}

	// Unknown error: return cleaned text with model tag, capped at readable length.
	const maxErrorLen = 150
	result := errText
	if modelLabel != "" {
		result = modelLabel + " " + result
	}
	if len(result) > maxErrorLen {
		return result[:maxErrorLen] + "…"
	}
	return result
}
