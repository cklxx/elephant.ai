package llm

import (
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/utils"
)

func (c *openaiClient) convertMessages(msgs []ports.Message) []map[string]any {
	result := make([]map[string]any, 0, len(msgs))
	isKimi := c.kimiCompat
	seenToolCalls := make(map[string]struct{}, 8)
	droppedToolOutputs := make(map[string]struct{}, 4)
	droppedCallIDs := make([]string, 0, 4)
	recordDroppedToolOutput := func(callID string) {
		callID = strings.TrimSpace(callID)
		if callID == "" {
			callID = "<empty_call_id>"
		}
		if _, exists := droppedToolOutputs[callID]; exists {
			return
		}
		droppedToolOutputs[callID] = struct{}{}
		droppedCallIDs = append(droppedCallIDs, callID)
	}
	embedMask := attachmentEmbeddingMask(msgs)
	for idx, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}
		role := utils.TrimLower(msg.Role)
		if role == "tool" {
			callID := strings.TrimSpace(msg.ToolCallID)
			if callID == "" {
				recordDroppedToolOutput(callID)
				continue
			}
			if _, ok := seenToolCalls[callID]; !ok {
				recordDroppedToolOutput(callID)
				continue
			}
		}
		entry := map[string]any{"role": msg.Role}
		content := buildMessageContent(msg, embedMask[idx])
		entry["content"] = content
		// Kimi rejects empty user messages and empty assistant messages
		// (no content AND no tool_calls). Empty assistants arise from
		// checkpoint recovery where MessageState drops ToolCalls.
		if isKimi && isEmptyContent(content) {
			if msg.Role == "user" {
				continue
			}
			if msg.Role == "assistant" && len(msg.ToolCalls) == 0 {
				continue
			}
		}
		if msg.ToolCallID != "" {
			entry["tool_call_id"] = msg.ToolCallID
		}
		if len(msg.ToolCalls) > 0 {
			historyCalls := buildToolCallHistory(msg.ToolCalls)
			if len(historyCalls) > 0 {
				entry["tool_calls"] = historyCalls
				for _, call := range historyCalls {
					callID, _ := call["id"].(string)
					callID = strings.TrimSpace(callID)
					if callID == "" {
						continue
					}
					seenToolCalls[callID] = struct{}{}
				}
			}
		}
		// Kimi requires reasoning_content in assistant messages with tool_calls when thinking is enabled.
		// The field must always be present (even as "") to satisfy the API contract; omitting it
		// causes: "thinking is enabled but reasoning_content is missing in assistant tool call message".
		if isKimi && msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			reasoningContent := extractThinkingText(msg.Thinking)
			entry["reasoning_content"] = reasoningContent
		}
		result = append(result, entry)
	}
	if len(droppedCallIDs) > 0 && c.logger != nil {
		c.logger.Warn("Dropped %d orphan/invalid tool message(s) from chat history: %s", len(droppedCallIDs), strings.Join(droppedCallIDs, ", "))
	}
	return result
}

// isEmptyContent checks if the message content is empty (string or array).
func isEmptyContent(content any) bool {
	switch v := content.(type) {
	case string:
		return utils.IsBlank(v)
	case []map[string]any:
		return len(v) == 0
	case nil:
		return true
	default:
		return false
	}
}

func extractRequestID(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata["request_id"]; ok {
		switch v := value.(type) {
		case string:
			return strings.TrimSpace(v)
		case fmt.Stringer:
			return strings.TrimSpace(v.String())
		}
	}
	return ""
}

func buildMessageContent(msg ports.Message, embedAttachments bool) any {
	contentWithThinking := appendThinkingToText(msg.Content, msg.Thinking)
	if len(msg.Attachments) == 0 || !embedAttachments {
		return contentWithThinking
	}

	var parts []map[string]any
	hasImage := false

	appendText := func(text string) {
		if text == "" {
			return
		}
		parts = append(parts, map[string]any{
			"type": "text",
			"text": text,
		})
	}

	appendURLImage := func(att ports.Attachment, _ string) bool {
		url := ports.AttachmentReferenceValue(att)
		if url == "" {
			return false
		}
		hasImage = true
		parts = append(parts, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": url},
		})
		return true
	}

	embedAttachmentImages(msg.Content, msg.Attachments, appendText,
		appendURLImage,
		func(att ports.Attachment, key string) bool {
			url := ports.AttachmentReferenceValue(att)
			if url == "" {
				return false
			}
			appendText("[" + key + "]")
			hasImage = true
			parts = append(parts, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": url},
			})
			return true
		},
	)

	if !hasImage {
		return contentWithThinking
	}

	if thinkingText := thinkingPromptText(msg.Thinking); thinkingText != "" {
		appendText(thinkingText)
	}

	return parts
}

func (c *openaiClient) convertTools(tools []ports.ToolDefinition) []map[string]any {
	converted := convertTools(tools)
	if len(converted) != len(tools) {
		for _, tool := range tools {
			if !isValidToolName(tool.Name) {
				c.logger.Warn("Skipping tool with invalid function name for OpenAI: %s", tool.Name)
			}
		}
	}
	return converted
}
