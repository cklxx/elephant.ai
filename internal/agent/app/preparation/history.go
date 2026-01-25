package preparation

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	llm "alex/internal/agent/ports/llm"
	storage "alex/internal/agent/ports/storage"
	id "alex/internal/utils/id"
)

type historyRecall struct {
	messages []ports.Message
}

func (s *ExecutionPreparationService) loadSessionHistory(ctx context.Context, session *storage.Session) []ports.Message {
	if session == nil {
		return nil
	}
	if s.historyMgr != nil {
		history, err := s.historyMgr.Replay(ctx, session.ID, 0)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("Failed to replay session history (session=%s): %v", session.ID, err)
			}
		} else if len(history) > 0 {
			return agent.CloneMessages(history)
		}
	}
	if len(session.Messages) == 0 {
		return nil
	}
	return agent.CloneMessages(session.Messages)
}

func (s *ExecutionPreparationService) recallUserHistory(ctx context.Context, client llm.LLMClient, _ string, messages []ports.Message) *historyRecall {
	if len(messages) == 0 {
		return nil
	}

	rawMessages := historyMessagesFromSession(messages)
	if len(rawMessages) == 0 {
		return nil
	}

	recall := &historyRecall{}
	if s.shouldSummarizeHistory(rawMessages) {
		summaryMessages := s.composeHistorySummary(ctx, client, rawMessages)
		if len(summaryMessages) > 0 {
			recall.messages = summaryMessages
			return recall
		}
		s.logger.Warn("History recall summary failed, falling back to raw messages")
	}

	recall.messages = rawMessages
	return recall
}

func (s *ExecutionPreparationService) composeHistorySummary(ctx context.Context, client llm.LLMClient, messages []ports.Message) []ports.Message {
	if client == nil || len(messages) == 0 {
		return nil
	}
	prompt := buildHistorySummaryPrompt(messages)
	if prompt == "" {
		return nil
	}
	requestID := id.LogIDFromContext(ctx)
	if requestID == "" {
		requestID = id.NewRequestID()
	}
	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role:    "system",
				Content: "You are a memory specialist who condenses previous assistant conversations into concise, high-signal summaries. Capture the user objectives, assistant actions, and any follow-up commitments in a neutral tone. Limit the response to 2-3 short paragraphs or bullet points.",
			},
			{
				Role:    "user",
				Content: prompt,
				Source:  ports.MessageSourceUserHistory,
			},
		},
		Temperature: 0.1,
		MaxTokens:   historySummaryMaxTokens,
		Metadata: map[string]any{
			"request_id": requestID,
			"intent":     historySummaryIntent,
		},
	}
	summaryCtx, cancel := context.WithTimeout(ctx, historySummaryLLMTimeout)
	defer cancel()
	streaming, ok := llm.EnsureStreamingClient(client).(llm.StreamingLLMClient)
	if !ok {
		resp, err := client.Complete(summaryCtx, req)
		if err != nil {
			s.logger.Warn("History summary composition failed (request_id=%s): %v", requestID, err)
			return nil
		}
		if resp == nil || strings.TrimSpace(resp.Content) == "" {
			s.logger.Warn("History summary composition returned empty response (request_id=%s)", requestID)
			return nil
		}
		summary := strings.TrimSpace(resp.Content)
		return []ports.Message{{
			Role:    "system",
			Content: summary,
			Source:  ports.MessageSourceUserHistory,
		}}
	}
	resp, err := streaming.StreamComplete(summaryCtx, req, ports.CompletionStreamCallbacks{
		OnContentDelta: func(ports.ContentDelta) {},
	})
	if err != nil {
		s.logger.Warn("History summary composition failed (request_id=%s): %v", requestID, err)
		return nil
	}
	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		s.logger.Warn("History summary composition returned empty response (request_id=%s)", requestID)
		return nil
	}
	summary := strings.TrimSpace(resp.Content)
	return []ports.Message{{
		Role:    "system",
		Content: summary,
		Source:  ports.MessageSourceUserHistory,
	}}
}

func buildHistorySummaryPrompt(messages []ports.Message) string {
	if len(messages) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("Summarize the intent, assistant responses, tool outputs, and remaining follow-ups from the prior exchanges below. Focus on actionable context relevant to the current task.\n\n")
	for i, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "message"
		}
		roleLower := strings.ToLower(role)
		roleLabel := strings.ToUpper(roleLower[:1]) + roleLower[1:]
		builder.WriteString(fmt.Sprintf("%d. %s: ", i+1, roleLabel))
		builder.WriteString(condenseHistoryText(msg.Content, historyComposeSnippetLimit))
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func condenseHistoryText(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	normalized := normalizeWhitespace(trimmed)
	runes := []rune(normalized)
	if len(runes) <= limit {
		return normalized
	}
	if limit <= 1 {
		return string(runes[:1])
	}
	return string(runes[:limit-1]) + "…"
}

func normalizeWhitespace(value string) string {
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func historyMessagesFromSession(messages []ports.Message) []ports.Message {
	if len(messages) == 0 {
		return nil
	}
	filtered := make([]ports.Message, 0, len(messages))
	for _, msg := range messages {
		if !shouldRecallHistoryMessage(msg) {
			continue
		}
		filtered = append(filtered, cloneHistoryMessage(msg))
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func shouldRecallHistoryMessage(msg ports.Message) bool {
	role := strings.TrimSpace(msg.Role)
	if strings.EqualFold(role, "system") {
		return false
	}
	if msg.Source == ports.MessageSourceSystemPrompt || msg.Source == ports.MessageSourceUserHistory {
		return false
	}
	if strings.TrimSpace(msg.Content) == "" && len(msg.Attachments) == 0 && len(msg.ToolCalls) == 0 && len(msg.ToolResults) == 0 {
		return false
	}
	return true
}

func cloneHistoryMessage(msg ports.Message) ports.Message {
	cloned := msg
	cloned.Role = strings.TrimSpace(cloned.Role)
	if cloned.Role == "" {
		cloned.Role = msg.Role
	}
	cloned.Content = strings.TrimSpace(cloned.Content)
	cloned.Source = ports.MessageSourceUserHistory
	if len(msg.ToolCalls) > 0 {
		cloned.ToolCalls = append([]ports.ToolCall(nil), msg.ToolCalls...)
	}
	if len(msg.ToolResults) > 0 {
		cloned.ToolResults = make([]ports.ToolResult, len(msg.ToolResults))
		for i, result := range msg.ToolResults {
			cloned.ToolResults[i] = cloneHistoryToolResult(result)
		}
	}
	if len(msg.Metadata) > 0 {
		metadata := make(map[string]any, len(msg.Metadata))
		for key, value := range msg.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(msg.Attachments) > 0 {
		cloned.Attachments = cloneHistoryAttachments(msg.Attachments)
	}
	return cloned
}

func cloneHistoryToolResult(result ports.ToolResult) ports.ToolResult {
	cloned := result
	if len(result.Metadata) > 0 {
		metadata := make(map[string]any, len(result.Metadata))
		for key, value := range result.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(result.Attachments) > 0 {
		cloned.Attachments = cloneHistoryAttachments(result.Attachments)
	}
	return cloned
}

func cloneHistoryAttachments(values map[string]ports.Attachment) map[string]ports.Attachment {
	if len(values) == 0 {
		return nil
	}
	return ports.CloneAttachmentMap(values)
}

func (s *ExecutionPreparationService) shouldSummarizeHistory(messages []ports.Message) bool {
	if len(messages) == 0 {
		return false
	}
	limit := s.config.MaxTokens
	if limit <= 0 {
		return false
	}
	threshold := int(float64(limit) * 0.7)
	if threshold <= 0 {
		return false
	}
	return s.estimateHistoryTokens(messages) > threshold
}

func (s *ExecutionPreparationService) estimateHistoryTokens(messages []ports.Message) int {
	if len(messages) == 0 {
		return 0
	}
	if s.contextMgr != nil {
		if estimate := s.contextMgr.EstimateTokens(messages); estimate > 0 {
			return estimate
		}
	}
	total := 0
	for _, msg := range messages {
		total += len(msg.Content) / 4
	}
	return total
}
