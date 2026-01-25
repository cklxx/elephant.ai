package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
)

func (e *ReactEngine) normalizeToolResult(tc ToolCall, state *TaskState, result ToolResult) ToolResult {
	normalized := result
	if normalized.CallID == "" {
		normalized.CallID = tc.ID
	}
	if normalized.SessionID == "" {
		normalized.SessionID = state.SessionID
	}
	if normalized.TaskID == "" {
		normalized.TaskID = state.TaskID
	}
	if normalized.ParentTaskID == "" {
		normalized.ParentTaskID = state.ParentTaskID
	}
	if strings.EqualFold(tc.Name, "a2ui_emit") {
		normalized.Attachments = nil
		if len(normalized.Metadata) > 0 {
			delete(normalized.Metadata, "attachment_mutations")
			delete(normalized.Metadata, "attachments_mutations")
			delete(normalized.Metadata, "attachmentMutations")
			delete(normalized.Metadata, "attachmentsMutations")
			if len(normalized.Metadata) == 0 {
				normalized.Metadata = nil
			}
		}
	}
	return normalized
}

func (e *ReactEngine) emitWorkflowToolCompletedEvent(ctx context.Context, state *TaskState, tc ToolCall, result ToolResult, duration time.Duration) {
	e.emitEvent(&WorkflowToolCompletedEvent{
		BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
		CallID:      result.CallID,
		ToolName:    tc.Name,
		Result:      result.Content,
		Error:       result.Error,
		Duration:    duration,
		Metadata:    result.Metadata,
		Attachments: result.Attachments,
	})
}

// parseToolCalls extracts tool calls from assistant message
func (e *ReactEngine) parseToolCalls(msg Message, parser ports.FunctionCallParser) []ToolCall {

	if len(msg.ToolCalls) > 0 {
		e.logger.Debug("Using native tool calls from message: count=%d", len(msg.ToolCalls))
		return msg.ToolCalls
	}

	e.logger.Debug("Parsing tool calls from content: length=%d", len(msg.Content))
	parsed, err := parser.Parse(msg.Content)
	if err != nil {
		e.logger.Warn("Failed to parse tool calls from content: %v", err)
		return nil
	}

	calls := make([]ToolCall, 0, len(parsed))
	calls = append(calls, parsed...)

	e.logger.Debug("Parsed %d tool calls from content", len(calls))
	return calls
}

// buildToolMessages converts tool results into messages sent back to the LLM.
func (e *ReactEngine) buildToolMessages(results []ToolResult) []Message {
	messages := make([]Message, 0, len(results))

	for _, result := range results {
		var content string
		if result.Error != nil {
			content = fmt.Sprintf("Tool %s failed: %v", result.CallID, result.Error)
		} else if trimmed := strings.TrimSpace(result.Content); trimmed != "" {
			content = trimmed
		} else {
			content = fmt.Sprintf("Tool %s completed successfully.", result.CallID)
		}

		content = ensureToolAttachmentReferences(content, result.Attachments)

		msg := Message{
			Role:        "tool",
			Content:     content,
			ToolCallID:  result.CallID,
			ToolResults: []ToolResult{result},
			Source:      ports.MessageSourceToolResult,
		}

		msg.Attachments = normalizeToolAttachments(result.Attachments)

		messages = append(messages, msg)
	}

	return messages
}

// cleanToolCallMarkers removes leaked tool call XML markers from content
func (e *ReactEngine) cleanToolCallMarkers(content string) string {
	cleaned := content
	for _, re := range cleanToolCallPatterns {
		cleaned = re.ReplaceAllString(cleaned, "")
	}
	return strings.TrimSpace(cleaned)
}
