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
				Description: "Send a text message to the current Lark chat. Use this to proactively communicate with the user during task execution — progress updates, intermediate results, or questions. Only available inside a Lark chat context.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"content": {
							Type:        "string",
							Description: "The message text to send.",
						},
					},
					Required: []string{"content"},
				},
			},
			ports.ToolMetadata{
				Name:     "lark_send_message",
				Version:  "0.1.0",
				Category: "lark",
				Tags:     []string{"lark", "chat", "send"},
			},
		),
	}
}

func (t *larkSendMessage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	for key := range call.Arguments {
		switch key {
		case "content":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_send_message is only available inside a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	client, ok := rawClient.(*lark.Client)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_send_message: invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}

	chatID := shared.LarkChatIDFromContext(ctx)
	if chatID == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_send_message: no chat_id available in context.",
			Error:   fmt.Errorf("chat_id not available in context"),
		}, nil
	}

	content, errResult := shared.RequireStringArg(call.Arguments, call.ID, "content")
	if errResult != nil {
		return errResult, nil
	}

	replyToID := strings.TrimSpace(shared.LarkMessageIDFromContext(ctx))
	payload := textPayload(content)

	if replyToID != "" {
		return t.replyMessage(ctx, client, call.ID, chatID, replyToID, payload)
	}
	return t.createMessage(ctx, client, call.ID, chatID, payload)
}

func (t *larkSendMessage) createMessage(ctx context.Context, client *lark.Client, callID, chatID, content string) (*ports.ToolResult, error) {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("text").
			Content(content).
			Build()).
		Build()

	resp, err := client.Im.Message.Create(ctx, req)
	if err != nil {
		return &ports.ToolResult{
			CallID:  callID,
			Content: fmt.Sprintf("lark_send_message: API call failed: %v", err),
			Error:   fmt.Errorf("lark API call failed: %w", err),
		}, nil
	}
	if !resp.Success() {
		return &ports.ToolResult{
			CallID:  callID,
			Content: fmt.Sprintf("lark_send_message: API error code=%d msg=%s", resp.Code, resp.Msg),
			Error:   fmt.Errorf("lark API error: code=%d msg=%s", resp.Code, resp.Msg),
		}, nil
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

func (t *larkSendMessage) replyMessage(ctx context.Context, client *lark.Client, callID, chatID, replyToID, content string) (*ports.ToolResult, error) {
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(replyToID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType("text").
			Content(content).
			Build()).
		Build()

	resp, err := client.Im.Message.Reply(ctx, req)
	if err != nil {
		return &ports.ToolResult{
			CallID:  callID,
			Content: fmt.Sprintf("lark_send_message: reply API call failed: %v", err),
			Error:   fmt.Errorf("lark reply API call failed: %w", err),
		}, nil
	}
	if !resp.Success() {
		return &ports.ToolResult{
			CallID:  callID,
			Content: fmt.Sprintf("lark_send_message: reply API error code=%d msg=%s", resp.Code, resp.Msg),
			Error:   fmt.Errorf("lark reply API error: code=%d msg=%s", resp.Code, resp.Msg),
		}, nil
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

func renderOutgoingMentions(text string) string {
	if strings.TrimSpace(text) == "" {
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
