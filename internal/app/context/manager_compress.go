package context

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/logging"
	"alex/internal/shared/token"
)

// EstimateTokens counts tokens using tiktoken (cl100k_base).
func (m *manager) EstimateTokens(messages []ports.Message) int {
	count := 0
	for _, msg := range messages {
		count += tokenutil.CountTokens(msg.Content)
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

	// Collect compressible messages (same logic as Compress) so the flush hook
	// receives exactly the messages that will be summarized away.
	if m.flushHook != nil {
		var compressible []ports.Message
		for _, msg := range messages {
			if msg.Source == ports.MessageSourceSystemPrompt || msg.Source == ports.MessageSourceImportant {
				continue
			}
			compressible = append(compressible, msg)
		}
		if len(compressible) > 0 {
			if err := m.flushHook.OnBeforeCompaction(context.Background(), compressible); err != nil {
				logging.OrNop(m.logger).Warn("Flush-before-compaction hook failed: %v", err)
			}
		}
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

// ---------------------------------------------------------------------------
// Budget helpers
// ---------------------------------------------------------------------------

// BudgetSignal indicates which token-budget threshold was crossed.
type BudgetSignal int

const (
	BudgetOK             BudgetSignal = iota // under all thresholds
	BudgetCompress                           // crossed compression threshold
	BudgetAggressiveTrim                     // crossed aggressive-trim threshold
)

// BudgetCheck evaluates the budget thresholds and returns the appropriate
// signal. The caller may choose to apply different compaction strategies based
// on the result.
func BudgetCheck(tokenCount, limit int, compressionThreshold, aggressiveTrimThreshold float64) BudgetSignal {
	if limit <= 0 {
		return BudgetOK
	}
	ratio := float64(tokenCount) / float64(limit)
	if ratio >= aggressiveTrimThreshold {
		return BudgetAggressiveTrim
	}
	if ratio >= compressionThreshold {
		return BudgetCompress
	}
	return BudgetOK
}

// AggressiveTrim retains only the most recent maxTurns user+assistant
// exchanges plus all system/important messages.
func AggressiveTrim(messages []ports.Message, maxTurns int) []ports.Message {
	if maxTurns <= 0 {
		maxTurns = 6
	}

	// Separate preserved messages from conversation turns.
	var preserved, conversation []ports.Message
	for _, msg := range messages {
		switch msg.Source {
		case ports.MessageSourceSystemPrompt, ports.MessageSourceImportant:
			preserved = append(preserved, msg)
		default:
			conversation = append(conversation, msg)
		}
	}

	// Count turns from the end (a "turn" = one user message + following assistant/tool messages).
	kept := keepRecentTurns(conversation, maxTurns)

	result := make([]ports.Message, 0, len(preserved)+len(kept))
	result = append(result, preserved...)
	if len(kept) > 0 {
		summary := buildCompressionSummary(conversation[:len(conversation)-len(kept)])
		if summary != "" {
			result = append(result, ports.Message{
				Role:    "system",
				Content: summary,
				Source:  ports.MessageSourceSystemPrompt,
			})
		}
		result = append(result, kept...)
	}
	return result
}

// keepRecentTurns returns the last N user-initiated turns from a conversation
// slice. A turn starts with a user message and includes all following
// assistant/tool messages until the next user message.
func keepRecentTurns(messages []ports.Message, maxTurns int) []ports.Message {
	if len(messages) == 0 || maxTurns <= 0 {
		return nil
	}

	// Find turn boundaries (indices of user messages).
	var turnStarts []int
	for i, msg := range messages {
		if strings.EqualFold(strings.TrimSpace(msg.Role), "user") {
			turnStarts = append(turnStarts, i)
		}
	}

	if len(turnStarts) == 0 {
		return messages // no user messages — keep everything
	}

	// Keep the last maxTurns.
	start := 0
	if len(turnStarts) > maxTurns {
		start = turnStarts[len(turnStarts)-maxTurns]
	} else {
		start = turnStarts[0]
	}

	return messages[start:]
}
