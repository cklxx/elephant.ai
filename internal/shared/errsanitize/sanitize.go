package errsanitize

import "strings"

// errorMapping maps a set of lower-case substrings to a user-facing message
// template. The template may contain "%s" which is replaced with modelLabel.
type errorMapping struct {
	patterns []string
	message  string // use %s for modelLabel insertion point
}

var userErrorMappings = []errorMapping{
	{
		patterns: []string{"authentication failed", "please verify your api key", "unauthorized"},
		message:  "AI 服务认证失败%s，请检查 API 密钥配置",
	},
	{
		patterns: []string{"rate limit", "too many requests"},
		message:  "AI 服务请求频率超限%s，系统正在尝试备用模型，请稍后重试",
	},
	{
		patterns: []string{"context_length_exceeded", "context window", "maximum context length"},
		message:  "输入内容超出 AI 模型上下文长度限制%s",
	},
	{
		patterns: []string{"model not found", "not_found_error"},
		message:  "AI 模型配置错误%s，请检查模型名称设置",
	},
	{
		patterns: []string{"timeout", "timed out", "deadline exceeded"},
		message:  "AI 服务请求超时%s，请稍后重试",
	},
	{
		patterns: []string{"connection refused", "no such host", "dial tcp", "failed to reach"},
		message:  "网络连接失败%s，请检查网络状态后重试",
	},
	{
		patterns: []string{"unavailable", "overloaded", "bad gateway", "request was rejected"},
		message:  "AI 服务暂时不可用%s，请稍后重试",
	},
	{
		patterns: []string{"request rejected"},
		message:  "AI 服务请求被拒绝%s，请检查请求参数",
	},
	{
		patterns: []string{"nil response", "empty response"},
		message:  "AI 服务返回空结果%s，请重试",
	},
	{
		patterns: []string{"responses input is empty"},
		message:  "消息内容为空，无法发送给 AI%s",
	},
}

// chainPrefixes are Go error-chain prefixes stripped iteratively.
var chainPrefixes = []string{
	"task execution failed: ",
	"think step failed: ",
	"agent run failed: ",
	"step failed: ",
	"llm call failed: ",
}

// ForUser strips Go error-chain prefixes, extracts the LLM
// provider/model tag when present, and maps known technical error patterns to
// user-friendly Chinese messages.
//
// Callers must pass the raw error string only — do NOT include UI prefixes
// like "执行失败：". Used when LLM narration is unavailable (e.g. the LLM
// itself is failing).
func ForUser(errText string) string {
	if errText == "" {
		return errText
	}

	errText = stripChainPrefixes(errText)

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
			modelTag = errText[1:idx]
			errText = errText[idx+2:]
		}
	}

	// Strip trailing retry/streaming noise.
	for _, suffix := range []string{" Streaming request failed after", " Retried "} {
		if i := strings.Index(errText, suffix); i > 0 {
			errText = strings.TrimSpace(errText[:i])
		}
	}

	modelLabel := formatModelLabel(modelTag)

	// Match against known error patterns.
	lower := strings.ToLower(errText)
	for _, m := range userErrorMappings {
		if containsAny(lower, m.patterns) {
			return strings.ReplaceAll(m.message, "%s", modelLabel)
		}
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

func stripChainPrefixes(s string) string {
	for {
		lower := strings.ToLower(s)
		stripped := false
		for _, p := range chainPrefixes {
			if strings.HasPrefix(lower, p) {
				s = s[len(p):]
				stripped = true
				break
			}
		}
		if !stripped {
			return s
		}
	}
}

func formatModelLabel(modelTag string) string {
	if modelTag == "" {
		return ""
	}
	label := modelTag
	if i := strings.LastIndex(modelTag, "/"); i >= 0 {
		label = modelTag[i+1:]
	}
	return "（" + label + "）"
}

func containsAny(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
