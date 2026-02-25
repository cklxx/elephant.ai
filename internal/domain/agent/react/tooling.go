package react

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
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
		normalized.TaskID = state.RunID
	}
	if normalized.ParentTaskID == "" {
		normalized.ParentTaskID = state.ParentRunID
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
	e.emitEvent(domain.NewToolCompletedEvent(
		e.newBaseEvent(ctx, state.SessionID, state.RunID, state.ParentRunID),
		result.CallID, tc.Name, result.Content, result.Error, duration,
		result.Metadata, result.Attachments,
	))
}

// parseToolCalls extracts tool calls from assistant message
func (e *ReactEngine) parseToolCalls(msg Message, parser agent.FunctionCallParser) []ToolCall {

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

// truncateToolResultContent caps content at maxToolResultContentChars,
// cutting at the nearest preceding line boundary to avoid splitting a line
// in half.  When truncation occurs, a hint is appended telling the LLM how
// much content remains and suggesting it request a specific range.
func truncateToolResultContent(content string, limit int) string {
	return truncateToolResultWithMetadata(content, limit, nil)
}

// truncateToolResultWithMetadata is the metadata-aware variant.  When the
// tool result carries structured metadata (e.g. read_file with total_lines
// and file_size_bytes), it produces a file-specific hint with line ranges
// and next-range suggestions.  For all other tools it falls back to a
// generic hint.
func truncateToolResultWithMetadata(content string, limit int, metadata map[string]any) string {
	if len(content) <= limit {
		return content
	}

	// Find the last newline at or before the limit so we cut at a line boundary.
	cut := strings.LastIndex(content[:limit], "\n")
	if cut <= 0 {
		cut = limit
	}

	shownLines := strings.Count(content[:cut], "\n") + 1
	truncated := content[:cut]

	// File-specific hint when rich metadata is available.
	toolName, _ := metadata["tool_name"].(string)
	totalLinesM, hasLines := metadata["total_lines"].(int)
	fileSizeM, hasSize := metadata["file_size_bytes"].(int)

	if toolName == "read_file" && hasLines && hasSize {
		rangeStart := 0
		if sr, ok := metadata["shown_range"].([2]int); ok {
			rangeStart = sr[0]
		}
		nextStart := rangeStart + shownLines
		truncated += fmt.Sprintf(
			"\n\n[Content truncated: showing lines %d-%d of %d total (%d/%d bytes). "+
				"File size: %d bytes. "+
				"Use start_line=%d end_line=%d to continue reading.]",
			rangeStart, rangeStart+shownLines-1, totalLinesM,
			cut, len(content), fileSizeM,
			nextStart, min(nextStart+200, totalLinesM),
		)
		return truncated
	}

	// Generic hint for other tools.
	totalLines := strings.Count(content, "\n") + 1
	truncated += fmt.Sprintf(
		"\n\n[Content truncated: showing %d/%d lines (%d/%d chars). "+
			"Use start_line/end_line parameters to view remaining content, e.g. start_line=%d.]",
		shownLines, totalLines, cut, len(content), shownLines,
	)
	return truncated
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

		content = strings.TrimSpace(content)
		content = truncateToolResultWithMetadata(content, maxToolResultContentChars, result.Metadata)

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
