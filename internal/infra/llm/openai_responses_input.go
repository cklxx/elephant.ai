package llm

import (
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/json"
)

// Responses input item shapes follow OpenAI Responses API.
// Source: opencode dev branch
// - packages/opencode/src/provider/sdk/openai-compatible/src/responses/openai-responses-api-types.ts
// - packages/opencode/src/provider/sdk/openai-compatible/src/responses/convert-to-openai-responses-input.ts
func (c *openAIResponsesClient) buildResponsesInputAndInstructions(msgs []ports.Message) ([]map[string]any, string) {
	items := make([]map[string]any, 0, len(msgs))
	var instructionsParts []string
	collectInstructions := c.isCodexEndpoint()
	for _, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		switch role {
		case "system", "developer":
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			if collectInstructions {
				instructionsParts = append(instructionsParts, msg.Content)
				continue
			}
			items = append(items, map[string]any{
				"role":    role,
				"content": msg.Content,
			})
		case "user":
			parts := buildResponsesUserContent(msg, shouldEmbedAttachmentsInContent(msg))
			if len(parts) == 0 {
				continue
			}
			items = append(items, map[string]any{
				"role":    "user",
				"content": parts,
			})
		case "assistant":
			if parts := buildResponsesAssistantContent(msg); len(parts) > 0 {
				items = append(items, map[string]any{
					"role":    "assistant",
					"content": parts,
				})
			}
			for _, call := range msg.ToolCalls {
				if !isValidToolName(call.Name) {
					continue
				}
				callID := strings.TrimSpace(call.ID)
				if callID == "" {
					continue
				}
				args := "{}"
				if len(call.Arguments) > 0 {
					if data, err := jsonx.Marshal(call.Arguments); err == nil {
						args = string(data)
					}
				}
				items = append(items, map[string]any{
					"type":      "function_call",
					"call_id":   callID,
					"name":      call.Name,
					"arguments": args,
				})
			}
		case "tool":
			callID := strings.TrimSpace(msg.ToolCallID)
			if callID == "" {
				continue
			}
			items = append(items, map[string]any{
				"type":    "function_call_output",
				"call_id": callID,
				"output":  msg.Content,
			})
		}
	}
	return items, strings.Join(instructionsParts, "\n\n")
}

// pruneOrphanFunctionCallOutputs removes Responses API function_call_output
// items that don't have a preceding function_call with the same call_id.
// This prevents upstream 400 rejections when history compaction/snapshot
// pruning leaves stale tool outputs in context.
func pruneOrphanFunctionCallOutputs(items []map[string]any) ([]map[string]any, []string) {
	if len(items) == 0 {
		return items, nil
	}

	seenCalls := make(map[string]struct{}, 8)
	droppedSet := make(map[string]struct{}, 4)
	dropped := make([]string, 0, 4)
	filtered := make([]map[string]any, 0, len(items))

	for _, item := range items {
		kind := strings.TrimSpace(itemString(item["type"]))
		switch kind {
		case "function_call":
			callID := strings.TrimSpace(itemString(item["call_id"]))
			if callID != "" {
				seenCalls[callID] = struct{}{}
			}
			filtered = append(filtered, item)
		case "function_call_output":
			callID := strings.TrimSpace(itemString(item["call_id"]))
			if callID == "" {
				if _, exists := droppedSet["<empty_call_id>"]; !exists {
					droppedSet["<empty_call_id>"] = struct{}{}
					dropped = append(dropped, "<empty_call_id>")
				}
				continue
			}
			if _, ok := seenCalls[callID]; !ok {
				if _, exists := droppedSet[callID]; !exists {
					droppedSet[callID] = struct{}{}
					dropped = append(dropped, callID)
				}
				continue
			}
			filtered = append(filtered, item)
		default:
			filtered = append(filtered, item)
		}
	}

	return filtered, dropped
}

func itemString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func buildResponsesAssistantContent(msg ports.Message) []map[string]any {
	parts := make([]map[string]any, 0, 2)
	if strings.TrimSpace(msg.Content) != "" {
		parts = append(parts, map[string]any{
			"type": "output_text",
			"text": msg.Content,
		})
	}
	if thinkingText := thinkingPromptText(msg.Thinking); thinkingText != "" {
		parts = append(parts, map[string]any{
			"type": "output_text",
			"text": thinkingText,
		})
	}
	if len(parts) == 0 {
		return nil
	}
	return parts
}

func buildResponsesUserContent(msg ports.Message, embedAttachments bool) []map[string]any {
	if len(msg.Attachments) == 0 || !embedAttachments {
		if msg.Content == "" {
			return nil
		}
		return []map[string]any{
			{"type": "input_text", "text": msg.Content},
		}
	}

	var parts []map[string]any

	appendText := func(text string) {
		if text == "" {
			return
		}
		parts = append(parts, map[string]any{
			"type": "input_text",
			"text": text,
		})
	}

	appendURLImage := func(att ports.Attachment, _ string) bool {
		url := ports.AttachmentReferenceValue(att)
		if url == "" {
			return false
		}
		parts = append(parts, map[string]any{
			"type":      "input_image",
			"image_url": url,
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
			parts = append(parts, map[string]any{
				"type":      "input_image",
				"image_url": url,
			})
			return true
		},
	)

	return parts
}
