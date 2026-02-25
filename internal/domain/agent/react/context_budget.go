package react

import "strings"

const (
	defaultModelContextWindowTokens = 128000
	gpt5ContextWindowTokens         = 256000
	claudeContextWindowTokens       = 200000
	minContextBudgetTokens          = 4096
)

func (e *ReactEngine) resolveContextTokenLimit(services Services) int {
	if e.completion.contextTokenLimit > 0 {
		return e.completion.contextTokenLimit
	}

	model := ""
	if services.LLM != nil {
		model = services.LLM.Model()
	}
	return deriveContextTokenLimit(model, e.completion.maxTokens)
}

func deriveContextTokenLimit(model string, maxOutputTokens int) int {
	window := modelContextWindowTokens(model)

	reservedForOutput := maxOutputTokens
	if reservedForOutput < 2048 {
		reservedForOutput = 2048
	}

	// Keep a small fixed buffer to reduce edge-case overflows from framing/tool metadata.
	safetyBuffer := window / 100
	if safetyBuffer < 1024 {
		safetyBuffer = 1024
	}

	limit := window - reservedForOutput - safetyBuffer
	if limit < minContextBudgetTokens {
		limit = minContextBudgetTokens
	}
	if limit > window {
		limit = window
	}
	return limit
}

func modelContextWindowTokens(model string) int {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case m == "":
		return defaultModelContextWindowTokens
	case strings.HasPrefix(m, "gpt-5"),
		strings.Contains(m, "gpt-5.2-codex"),
		strings.Contains(m, "gpt-5.3-codex"),
		strings.Contains(m, "codex-spark"):
		return gpt5ContextWindowTokens
	case strings.Contains(m, "claude"):
		return claudeContextWindowTokens
	case strings.Contains(m, "gpt-4"),
		strings.Contains(m, "deepseek"),
		strings.Contains(m, "kimi"),
		strings.Contains(m, "moonshot"),
		strings.Contains(m, "glm"),
		strings.Contains(m, "minimax"):
		return defaultModelContextWindowTokens
	default:
		return defaultModelContextWindowTokens
	}
}
