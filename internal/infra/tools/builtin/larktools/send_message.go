package larktools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type larkSendMessage struct {
	shared.BaseTool
}

// NewLarkSendMessage constructs a tool that sends a text message to the current Lark chat.
func NewLarkSendMessage() tools.ToolExecutor {
	return &larkSendMessage{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "lark_send_message",
				Description: "Send a text/status message to the current Lark chat thread. Use for progress updates, decisions, and short checkpoints without file transfer. Supports rich text format (post) for styled messages with bold, links, lists. Do not use for attachments/uploads (use lark_upload_file) or for reading prior context (use lark_chat_history).",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"content": {
							Type:        "string",
							Description: "The message text to send. Supports Markdown syntax when content_format is 'post'.",
						},
						"content_format": {
							Type:        "string",
							Description: "Message format: 'text' (default, plain text) or 'post' (rich text with Markdown support).",
							Enum:        []any{"text", "post"},
						},
					},
					Required: []string{"content"},
				},
			},
			ports.ToolMetadata{
				Name:        "lark_send_message",
				Version:     "0.2.0",
				Category:    "lark",
				Tags:        []string{"lark", "chat", "send", "message", "status", "checkpoint", "richtext"},
				SafetyLevel: ports.SafetyLevelReversible,
			},
		),
	}
}

func (t *larkSendMessage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	for key := range call.Arguments {
		switch key {
		case "content", "content_format":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	chatID := shared.LarkChatIDFromContext(ctx)
	if chatID == "" {
		return missingChatIDResult(call.ID, "lark_send_message"), nil
	}

	content, errResult := shared.RequireStringArg(call.Arguments, call.ID, "content")
	if errResult != nil {
		return errResult, nil
	}

	// Determine message format: text (default) or post (rich text)
	format := shared.StringArg(call.Arguments, "content_format")
	if format == "" {
		format = "text"
	}
	format = utils.TrimLower(format)

	rawReplyToID := strings.TrimSpace(shared.LarkMessageIDFromContext(ctx))
	replyToID := rawReplyToID
	syntheticInject := isSyntheticInjectMessageID(rawReplyToID)
	if syntheticInject {
		replyToID = ""
	}

	var msgType string
	var payload string
	switch format {
	case "post", "richtext", "rich_text":
		msgType = "post"
		payload = postPayload(content)
	default:
		msgType = "text"
		payload = textPayload(content)
	}

	if messenger := shared.LarkMessengerFromContext(ctx); messenger != nil {
		if replyToID != "" {
			messageID, err := messenger.ReplyMessage(ctx, replyToID, msgType, payload)
			if err != nil {
				return &ports.ToolResult{
					CallID:  call.ID,
					Content: fmt.Sprintf("lark_send_message: messenger reply failed: %v", err),
					Error:   fmt.Errorf("lark messenger reply failed: %w", err),
				}, nil
			}
			return &ports.ToolResult{
				CallID:  call.ID,
				Content: "Message sent successfully.",
				Metadata: map[string]any{
					"message_id":          messageID,
					"chat_id":             chatID,
					"reply_to_message_id": replyToID,
					"format":              format,
				},
			}, nil
		}

		messageID, err := messenger.SendMessage(ctx, chatID, msgType, payload)
		if err != nil {
			return &ports.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("lark_send_message: messenger send failed: %v", err),
				Error:   fmt.Errorf("lark messenger send failed: %w", err),
			}, nil
		}
		metadata := map[string]any{
			"message_id": messageID,
			"chat_id":    chatID,
		}
		if syntheticInject {
			metadata["synthetic_inject"] = true
		}
		return &ports.ToolResult{
			CallID:   call.ID,
			Content:  "Message sent successfully.",
			Metadata: metadata,
		}, nil
	}

	client, errResult := requireLarkClient(ctx, call.ID)
	if errResult != nil {
		return errResult, nil
	}

	if replyToID != "" {
		return t.replyMessage(ctx, client, call.ID, chatID, replyToID, msgType, payload)
	}
	return t.createMessage(ctx, client, call.ID, chatID, msgType, payload)
}

// postPayload converts Markdown content to Lark rich text (post) format.
// Supports: **bold**, [text](url) links, and plain text.
func postPayload(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var content [][]map[string]string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var row []map[string]string

		// Process bold: **text**
		boldPattern := regexp.MustCompile(`\*\*([^*]+)\*\*`)
		// Process links: [text](url)
		linkPattern := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

		// Replace bold markers with placeholders
		tempLine := boldPattern.ReplaceAllString(line, "__BOLD_START__$1__BOLD_END__")

		// Split by bold markers and process
		parts := strings.Split(tempLine, "__BOLD_")
		for _, part := range parts {
			if strings.HasPrefix(part, "START__") {
				// Bold text
				boldText := strings.SplitN(part[7:], "__END__", 2)[0]
				row = append(row, map[string]string{
					"tag":   "text",
					"text":  boldText,
					"style": "bold",
				})
				if rest := strings.SplitN(part, "__END__", 2); len(rest) > 1 {
					part = rest[1]
				} else {
					continue
				}
			} else if strings.HasPrefix(part, "END__") {
				part = part[6:]
			}

			// Process links in remaining text
			for {
				match := linkPattern.FindStringSubmatchIndex(part)
				if match == nil {
					break
				}
				// Text before link
				if match[0] > 0 {
					row = append(row, map[string]string{
						"tag":  "text",
						"text": part[:match[0]],
					})
				}
				// Link
				linkText := part[match[2]:match[3]]
				linkURL := part[match[4]:match[5]]
				row = append(row, map[string]string{
					"tag":  "a",
					"text": linkText,
					"href": linkURL,
				})
				part = part[match[1]:]
			}

			// Remaining text
			if part != "" {
				row = append(row, map[string]string{
					"tag":  "text",
					"text": part,
				})
			}
		}

		if len(row) > 0 {
			content = append(content, row)
		}
	}

	payload := map[string]interface{}{
		"post": map[string]interface{}{
			"zh_cn": map[string]interface{}{
				"content": content,
			},
		},
	}

	result, _ := json.Marshal(payload)
	return string(result)
}

func (t *larkSendMessage) createMessage(ctx context.Context, client *lark.Client, callID, chatID, content, msgType string) (*ports.ToolResult, error) {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build()

	resp, err := client.Im.Message.Create(ctx, req)
	if err != nil {
		return sdkCallErr(callID, "lark_send_message", err), nil
	}
	if !resp.Success() {
		return sdkRespErr(callID, "lark_send_message", resp.Code, resp.Msg), nil
	}

	messageID := ""
	if resp.Data != nil && resp.Data.MessageId != nil {
		messageID = *resp.Data.MessageId
	}

	return &ports.ToolResult{
		CallID:  callID,
		Content: "Message sent successfully.",
		Metadata: map[string]any{
			"message_id": messageID,
			"chat_id":    chatID,
		},
	}, nil
}

func (t *larkSendMessage) replyMessage(ctx context.Context, client *lark.Client, callID, chatID, replyToID, content, msgType string) (*ports.ToolResult, error) {
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(replyToID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(msgType).
			Content(content).
			Build()).
		Build()

	resp, err := client.Im.Message.Reply(ctx, req)
	if err != nil {
		return sdkCallErr(callID, "lark_send_message: reply", err), nil
	}
	if !resp.Success() {
		return sdkRespErr(callID, "lark_send_message: reply", resp.Code, resp.Msg), nil
	}

	messageID := ""
	if resp.Data != nil && resp.Data.MessageId != nil {
		messageID = *resp.Data.MessageId
	}

	return &ports.ToolResult{
		CallID:  callID,
		Content: "Message sent successfully.",
		Metadata: map[string]any{
			"message_id":          messageID,
			"chat_id":             chatID,
			"reply_to_message_id": replyToID,
		},
	}, nil
}

// textPayload builds the JSON content payload for a Lark text message.
func textPayload(text string) string {
	text = renderOutgoingMentions(text)
	payload, _ := json.Marshal(map[string]string{"text": text})
	return string(payload)
}

var outgoingMentionPattern = regexp.MustCompile(`@([^@()<>\n\r\t]+)\((ou_[A-Za-z0-9]+|all)\)`)

func isSyntheticInjectMessageID(messageID string) bool {
	return strings.HasPrefix(strings.TrimSpace(messageID), "inject_")
}

func renderOutgoingMentions(text string) string {
	if utils.IsBlank(text) {
		return text
	}
	return outgoingMentionPattern.ReplaceAllStringFunc(text, func(raw string) string {
		sub := outgoingMentionPattern.FindStringSubmatch(raw)
		if len(sub) != 3 {
			return raw
		}
		name := strings.TrimSpace(sub[1])
		userID := strings.TrimSpace(sub[2])
		if userID == "" {
			return raw
		}
		if userID == "all" && (name == "" || strings.EqualFold(name, "all")) {
			name = "所有人"
		}
		if name == "" {
			return raw
		}
		return `<at user_id="` + userID + `">` + name + `</at>`
	})
}
