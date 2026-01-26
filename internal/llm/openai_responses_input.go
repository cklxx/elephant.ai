package llm

import (
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/jsonx"
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
			if strings.TrimSpace(msg.Content) != "" {
				items = append(items, map[string]any{
					"role": "assistant",
					"content": []map[string]any{
						{"type": "output_text", "text": msg.Content},
					},
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

func buildResponsesUserContent(msg ports.Message, embedAttachments bool) []map[string]any {
	if len(msg.Attachments) == 0 || !embedAttachments {
		if msg.Content == "" {
			return nil
		}
		return []map[string]any{
			{"type": "input_text", "text": msg.Content},
		}
	}

	index := buildAttachmentIndex(msg.Attachments)

	var parts []map[string]any
	used := make(map[string]bool)

	appendText := func(text string) {
		if text == "" {
			return
		}
		parts = append(parts, map[string]any{
			"type": "input_text",
			"text": text,
		})
	}

	appendImage := func(url string) {
		if url == "" {
			return
		}
		parts = append(parts, map[string]any{
			"type":      "input_image",
			"image_url": url,
		})
	}

	content := msg.Content
	cursor := 0
	matches := attachmentPlaceholderPattern.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		if match[0] > cursor {
			appendText(content[cursor:match[0]])
		}
		placeholderToken := content[match[0]:match[1]]
		appendText(placeholderToken)

		name := strings.TrimSpace(content[match[2]:match[3]])
		if name == "" {
			cursor = match[1]
			continue
		}
		if att, key, ok := index.resolve(name); ok && isImageAttachment(att, key) && !used[key] {
			if url := ports.AttachmentReferenceValue(att); url != "" {
				appendImage(url)
				used[key] = true
			}
		}
		cursor = match[1]
	}
	if cursor < len(content) {
		appendText(content[cursor:])
	}

	for _, desc := range orderedImageAttachments(content, msg.Attachments) {
		key := desc.Placeholder
		if key == "" || used[key] {
			continue
		}
		if url := ports.AttachmentReferenceValue(desc.Attachment); url != "" {
			appendText("[" + key + "]")
			appendImage(url)
			used[key] = true
		}
	}

	return parts
}
