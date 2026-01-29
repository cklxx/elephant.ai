package llm

import (
	"strings"

	"alex/internal/agent/ports"
)

const thinkingPromptHeader = "Thinking (previous):"

func thinkingPromptText(thinking ports.Thinking) string {
	if len(thinking.Parts) == 0 {
		return ""
	}
	var lines []string
	for _, part := range thinking.Parts {
		text := strings.TrimSpace(part.Text)
		if text == "" {
			if part.Encrypted != "" || part.Signature != "" {
				text = "[redacted]"
			} else {
				continue
			}
		}
		kind := strings.TrimSpace(part.Kind)
		if kind != "" && kind != "thinking" {
			text = kind + ": " + text
		}
		lines = append(lines, text)
	}
	if len(lines) == 0 {
		return ""
	}
	return thinkingPromptHeader + "\n" + strings.Join(lines, "\n")
}

func appendThinkingToText(content string, thinking ports.Thinking) string {
	extra := thinkingPromptText(thinking)
	if extra == "" {
		return content
	}
	if strings.TrimSpace(content) == "" {
		return extra
	}
	return content + "\n\n" + extra
}

func appendThinkingPart(thinking *ports.Thinking, part ports.ThinkingPart) {
	if thinking == nil {
		return
	}
	if strings.TrimSpace(part.Text) == "" && strings.TrimSpace(part.Encrypted) == "" && strings.TrimSpace(part.Signature) == "" {
		return
	}
	thinking.Parts = append(thinking.Parts, part)
}

func appendThinkingText(thinking *ports.Thinking, kind, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	appendThinkingPart(thinking, ports.ThinkingPart{Kind: kind, Text: text})
}

// isArkEndpoint returns true if the base URL points to a ByteDance ARK endpoint.
func isArkEndpoint(baseURL string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(baseURL)), "ark")
}

// shouldSendArkReasoning returns true when thinking is enabled and the endpoint
// is an ARK service. ARK endpoint IDs (e.g. "ep-xxx") don't reveal model type,
// so no model check is performed — non-reasoning ARK models silently ignore the
// reasoning_effort parameter.
func shouldSendArkReasoning(baseURL string, cfg ports.ThinkingConfig) bool {
	return cfg.Enabled && isArkEndpoint(baseURL)
}

func shouldSendOpenAIReasoning(baseURL, model string, cfg ports.ThinkingConfig) bool {
	if !cfg.Enabled {
		return false
	}
	lowerBase := strings.ToLower(strings.TrimSpace(baseURL))
	if lowerBase == "" {
		return false
	}
	if !(strings.Contains(lowerBase, "openai") || strings.Contains(lowerBase, "openrouter.ai") || strings.Contains(lowerBase, "api.deepseek.com")) {
		return false
	}
	modelLower := strings.ToLower(strings.TrimSpace(model))
	if modelLower == "" {
		return false
	}
	return strings.Contains(modelLower, "o1") || strings.Contains(modelLower, "o3") || strings.Contains(modelLower, "r1") || strings.Contains(modelLower, "reasoning") || strings.Contains(modelLower, "think")
}

func buildOpenAIReasoningConfig(cfg ports.ThinkingConfig) map[string]any {
	if !cfg.Enabled {
		return nil
	}
	reasoning := map[string]any{}
	effort := strings.TrimSpace(cfg.Effort)
	if effort == "" {
		effort = "medium"
	}
	reasoning["effort"] = effort
	if summary := strings.TrimSpace(cfg.Summary); summary != "" {
		reasoning["summary"] = summary
	}
	if len(reasoning) == 0 {
		return nil
	}
	return reasoning
}

func buildAnthropicThinkingConfig(cfg ports.ThinkingConfig) map[string]any {
	if !cfg.Enabled {
		return nil
	}
	budget := cfg.BudgetTokens
	if budget <= 0 {
		budget = 1024
	}
	return map[string]any{
		"type":          "enabled",
		"budget_tokens": budget,
	}
}

func shouldSendAnthropicThinking(model string, cfg ports.ThinkingConfig) bool {
	if !cfg.Enabled {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(model))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "3-7") || strings.Contains(lower, "3.7") || strings.Contains(lower, "thinking")
}
