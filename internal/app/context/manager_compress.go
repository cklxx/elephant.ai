package context

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/logging"
	"alex/internal/shared/token"
)

// EstimateTokens counts tokens for all message components: content, tool calls,
// tool results, thinking parts, and structural overhead. Previous implementation
// only counted msg.Content, leading to severe underestimates that caused
// context_length_exceeded errors at the LLM provider.
func (m *manager) EstimateTokens(messages []ports.Message) int {
	count := 0
	for _, msg := range messages {
		count += EstimateMessageTokens(msg)
	}
	return count
}

// EstimateMessageTokens counts tokens for a single message including all
// components that contribute to the actual token count sent to the LLM.
func EstimateMessageTokens(msg ports.Message) int {
	// Per-message overhead: role tag, separators (~4 tokens).
	count := 4

	// Content (primary text).
	if msg.Content != "" {
		count += tokenutil.CountTokens(msg.Content)
	}

	// Tool calls: each call has name + JSON-serialized arguments.
	for _, call := range msg.ToolCalls {
		count += tokenutil.EstimateFast(call.Name)
		count += tokenutil.EstimateFast(call.ID)
		for key, val := range call.Arguments {
			count += tokenutil.EstimateFast(key)
			switch v := val.(type) {
			case string:
				count += tokenutil.CountTokens(v)
			default:
				// Non-string args: estimate ~10 tokens per entry.
				count += 10
			}
		}
		// Structural overhead per tool call (~8 tokens for JSON wrapper).
		count += 8
	}

	// Thinking parts.
	for _, part := range msg.Thinking.Parts {
		if part.Text != "" {
			count += tokenutil.CountTokens(part.Text)
		}
	}

	// Tool call ID (for tool result messages).
	if msg.ToolCallID != "" {
		count += tokenutil.EstimateFast(msg.ToolCallID) + 4
	}

	// Attachments: base64 image data is expensive. Use a flat estimate per
	// image since actual vision token cost is model-dependent and opaque.
	for _, att := range msg.Attachments {
		if att.Data != "" {
			count += 500 // flat estimate for base64-encoded image
		}
		if att.Description != "" {
			count += tokenutil.EstimateFast(att.Description)
		}
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

	plan := buildCompressionPlan(messages)
	if len(plan.summarySource) == 0 {
		return messages, false
	}

	// Collect compressible messages (same logic as Compress) so the flush hook
	// receives exactly the messages that will be summarized away.
	if m.flushHook != nil {
		if err := m.flushHook.OnBeforeCompaction(context.Background(), plan.summarySource); err != nil {
			logging.OrNop(m.logger).Warn("Flush-before-compaction hook failed: %v", err)
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
// BuildSummaryOnly generates a compression summary for older messages without
// replacing them. Returns the summary text and the count of messages that would
// be replaced. This is the first half of the delayed summary replacement:
// generate now, apply after N more turns.
func (m *manager) BuildSummaryOnly(messages []ports.Message) (string, int) {
	plan := buildCompressionPlan(messages)
	if len(plan.summarySource) == 0 {
		return "", 0
	}
	summary := buildCompressionSummary(plan.summarySource)
	return summary, len(plan.compressibleOriginalIndexes)
}


// Compress preserves system/important/checkpoint messages and the most recent
// conversation turn, then summarizes older conversation history when the token
// budget is exceeded. Existing compression summaries are never re-summarized.
func (m *manager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	if targetTokens <= 0 {
		return messages, nil
	}
	current := m.EstimateTokens(messages)
	if current <= targetTokens {
		return messages, nil
	}

	plan := buildCompressionPlan(messages)
	if len(plan.compressibleOriginalIndexes) == 0 || len(plan.summarySource) == 0 {
		return messages, nil
	}

	summary := buildCompressionSummary(plan.summarySource)
	if summary == "" {
		return messages, nil
	}

	summaryMessage := ports.Message{
		Role:    "assistant",
		Content: summary,
		Source:  ports.MessageSourceUserHistory,
	}

	compressed := make([]ports.Message, 0, len(messages)-len(plan.compressibleOriginalIndexes)+1)
	summaryInserted := false
	for idx, msg := range messages {
		if _, shouldCompress := plan.compressibleOriginalIndexes[idx]; shouldCompress {
			if !summaryInserted {
				compressed = append(compressed, summaryMessage)
				summaryInserted = true
			}
			continue
		}
		compressed = append(compressed, msg)
	}
	if !summaryInserted {
		compressed = append(compressed, summaryMessage)
	}

	return compressed, nil
}

func buildCompressionSummary(messages []ports.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var userCount, assistantCount, toolMentions, summarizedCount int
	var firstUser, lastUser, firstAssistant, lastAssistant string

	for _, msg := range messages {
		if isContextCompressionSummary(msg) {
			continue
		}
		summarizedCount++
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
	if summarizedCount == 0 {
		return ""
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

type compressionPlan struct {
	compressibleOriginalIndexes map[int]struct{}
	summarySource               []ports.Message
}

func buildCompressionPlan(messages []ports.Message) compressionPlan {
	plan := compressionPlan{
		compressibleOriginalIndexes: map[int]struct{}{},
	}
	if len(messages) == 0 {
		return plan
	}

	conversation := make([]ports.Message, 0, len(messages))
	conversationIndexes := make([]int, 0, len(messages))
	for idx, msg := range messages {
		if isCompressionPreservedSource(msg.Source) {
			continue
		}
		conversation = append(conversation, msg)
		conversationIndexes = append(conversationIndexes, idx)
	}
	if len(conversation) == 0 {
		return plan
	}

	keptConversation := keepRecentTurns(conversation, 1)
	compressibleCount := len(conversation) - len(keptConversation)
	if compressibleCount <= 0 {
		return plan
	}

	plan.compressibleOriginalIndexes = make(map[int]struct{}, compressibleCount)
	plan.summarySource = make([]ports.Message, 0, compressibleCount)
	for idx := 0; idx < compressibleCount; idx++ {
		plan.compressibleOriginalIndexes[conversationIndexes[idx]] = struct{}{}
		msg := conversation[idx]
		if isContextCompressionSummary(msg) {
			continue
		}
		plan.summarySource = append(plan.summarySource, msg)
	}
	return plan
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
		if isCompressionPreservedSource(msg.Source) {
			preserved = append(preserved, msg)
			continue
		}
		conversation = append(conversation, msg)
	}

	// Count turns from the end (a "turn" = one user message + following assistant/tool messages).
	kept := keepRecentTurns(conversation, maxTurns)

	result := make([]ports.Message, 0, len(preserved)+len(kept))
	result = append(result, preserved...)
	if len(kept) > 0 {
		summary := buildCompressionSummary(conversation[:len(conversation)-len(kept)])
		if summary != "" {
			result = append(result, ports.Message{
				Role:    "assistant",
				Content: summary,
				Source:  ports.MessageSourceUserHistory,
			})
		}
		result = append(result, kept...)
	}
	return result
}

func isCompressionPreservedSource(source ports.MessageSource) bool {
	switch source {
	case ports.MessageSourceSystemPrompt, ports.MessageSourceImportant, ports.MessageSourceCheckpoint:
		return true
	default:
		return false
	}
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
		// No user messages: degrade to keeping only the most recent maxTurns
		// entries so fallback paths still reduce context size.
		if len(messages) <= maxTurns {
			return messages
		}
		return messages[len(messages)-maxTurns:]
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
