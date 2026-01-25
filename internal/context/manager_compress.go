package context

import (
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/logging"
)

// EstimateTokens approximates token usage by dividing rune count.
func (m *manager) EstimateTokens(messages []ports.Message) int {
	count := 0
	for _, msg := range messages {
		count += len(msg.Content) / 4
	}
	return count
}

// ShouldCompress indicates whether the context needs to be compacted.
func (m *manager) ShouldCompress(messages []ports.Message, limit int) bool {
	if limit <= 0 {
		return false
	}
	threshold := m.compressionThreshold()
	return float64(m.EstimateTokens(messages)) > float64(limit)*threshold
}

// AutoCompact applies compression when the configured threshold is exceeded.
// It returns the (possibly) compacted messages alongside a flag indicating
// whether compaction was performed.
func (m *manager) AutoCompact(messages []ports.Message, limit int) ([]ports.Message, bool) {
	if !m.ShouldCompress(messages, limit) {
		return messages, false
	}

	threshold := m.compressionThreshold()
	target := int(float64(limit) * threshold)
	compressed, err := m.Compress(messages, target)
	if err != nil {
		logging.OrNop(m.logger).Warn("Auto compaction failed: %v", err)
		return messages, false
	}

	return compressed, true
}

// Compress preserves all system prompts and summarizes everything else when the
// token budget is exceeded. The summary is inserted where non-system content
// was first removed so that later system prompts stay in their original order.
// This keeps governance instructions intact while still giving the model
// awareness of the trimmed conversation.
func (m *manager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	if targetTokens <= 0 {
		return messages, nil
	}
	current := m.EstimateTokens(messages)
	if current <= targetTokens {
		return messages, nil
	}

	var (
		compressed            []ports.Message
		compressible          []ports.Message
		summaryInsertionIndex = -1
	)

	for _, msg := range messages {
		if msg.Source == ports.MessageSourceSystemPrompt || msg.Source == ports.MessageSourceImportant {
			compressed = append(compressed, msg)
			continue
		}
		if summaryInsertionIndex == -1 {
			summaryInsertionIndex = len(compressed)
		}
		compressible = append(compressible, msg)
	}

	if len(compressible) == 0 {
		return messages, nil
	}

	if summary := buildCompressionSummary(compressible); summary != "" {
		compressed = append(compressed, ports.Message{
			Role:    "system",
			Content: summary,
			Source:  ports.MessageSourceSystemPrompt,
		})
		if summaryInsertionIndex >= 0 && summaryInsertionIndex < len(compressed)-1 {
			insert := compressed[len(compressed)-1]
			copy(compressed[summaryInsertionIndex+1:], compressed[summaryInsertionIndex:])
			compressed[summaryInsertionIndex] = insert
		}
	} else {
		compressed = append(compressed, compressible...)
	}

	return compressed, nil
}

func buildCompressionSummary(messages []ports.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var userCount, assistantCount, toolMentions int
	var firstUser, lastUser, firstAssistant, lastAssistant string

	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		snippet := buildCompressionSnippet(msg.Content, 140)
		switch role {
		case "user":
			userCount++
			if firstUser == "" {
				firstUser = snippet
			}
			lastUser = snippet
		case "assistant":
			assistantCount++
			toolMentions += len(msg.ToolCalls)
			if firstAssistant == "" {
				firstAssistant = snippet
			}
			lastAssistant = snippet
		case "tool":
			toolMentions++
		}
		toolMentions += len(msg.ToolResults)
	}

	parts := []string{fmt.Sprintf("Earlier conversation had %d user message(s) and %d assistant response(s)", userCount, assistantCount)}
	if toolMentions > 0 {
		parts = append(parts, fmt.Sprintf("tools were referenced %d time(s)", toolMentions))
	}

	var contextParts []string
	if firstUser != "" {
		contextParts = append(contextParts, fmt.Sprintf("user first asked: %s", firstUser))
	}
	if firstAssistant != "" {
		contextParts = append(contextParts, fmt.Sprintf("assistant first replied: %s", firstAssistant))
	}
	if lastUser != "" && lastUser != firstUser {
		contextParts = append(contextParts, fmt.Sprintf("recent user request: %s", lastUser))
	}
	if lastAssistant != "" && lastAssistant != firstAssistant {
		contextParts = append(contextParts, fmt.Sprintf("recent assistant reply: %s", lastAssistant))
	}
	if len(contextParts) > 0 {
		parts = append(parts, strings.Join(contextParts, " | "))
	}

	return fmt.Sprintf("[Earlier context compressed] %s.", strings.Join(parts, "; "))
}

func buildCompressionSnippet(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || limit <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}
