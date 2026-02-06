package larktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

const (
	defaultPageSize = 20
	maxPageSize     = 50
)

type larkChatHistory struct {
	shared.BaseTool
}

// NewLarkChatHistory constructs a tool for querying Lark conversation history.
func NewLarkChatHistory() tools.ToolExecutor {
	return &larkChatHistory{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "lark_chat_history",
				Description: "Retrieve recent messages from the current Lark chat. Returns messages in chronological order formatted as '[timestamp] sender: content'. Only available when running inside a Lark chat context.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"page_size": {
							Type:        "integer",
							Description: "Number of messages to retrieve (default 20, max 50).",
						},
						"start_time": {
							Type:        "string",
							Description: "Start time as Unix timestamp in seconds. Only messages after this time are returned.",
						},
						"end_time": {
							Type:        "string",
							Description: "End time as Unix timestamp in seconds. Only messages before this time are returned.",
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:        "lark_chat_history",
				Version:     "0.1.0",
				Category:    "lark",
				Tags:        []string{"lark", "chat", "history"},
				SafetyLevel: ports.SafetyLevelReadOnly,
			},
		),
	}
}

func (t *larkChatHistory) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_chat_history is only available inside a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	client, ok := rawClient.(*lark.Client)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_chat_history: invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}

	chatID := shared.LarkChatIDFromContext(ctx)
	if chatID == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_chat_history: no chat_id available in context.",
			Error:   fmt.Errorf("chat_id not available in context"),
		}, nil
	}

	pageSize := clampPageSize(call.Arguments)
	startTime := shared.StringArg(call.Arguments, "start_time")
	endTime := shared.StringArg(call.Arguments, "end_time")

	builder := larkim.NewListMessageReqBuilder().
		ContainerIdType("chat").
		ContainerId(chatID).
		SortType("ByCreateTimeDesc").
		PageSize(pageSize)

	if startTime != "" {
		builder.StartTime(startTime)
	}
	if endTime != "" {
		builder.EndTime(endTime)
	}

	resp, err := client.Im.Message.List(ctx, builder.Build())
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_chat_history: API call failed: %v", err),
			Error:   fmt.Errorf("lark API call failed: %w", err),
		}, nil
	}
	if !resp.Success() {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_chat_history: API error code=%d msg=%s", resp.Code, resp.Msg),
			Error:   fmt.Errorf("lark API error: code=%d msg=%s", resp.Code, resp.Msg),
		}, nil
	}

	if resp.Data == nil || len(resp.Data.Items) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No messages found in this chat.",
		}, nil
	}

	formatted := formatMessages(resp.Data.Items)

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: formatted,
		Metadata: map[string]any{
			"message_count": len(resp.Data.Items),
			"chat_id":       chatID,
		},
	}, nil
}

// clampPageSize extracts and clamps page_size from arguments.
func clampPageSize(args map[string]any) int {
	ps, ok := shared.IntArg(args, "page_size")
	if !ok || ps <= 0 {
		return defaultPageSize
	}
	if ps > maxPageSize {
		return maxPageSize
	}
	return ps
}

// formatMessages converts Lark message items into chronological "[timestamp] sender: content" lines.
// Items from the API are in descending order; this reverses them to chronological.
func formatMessages(items []*larkim.Message) string {
	// Reverse to chronological order.
	reversed := make([]*larkim.Message, len(items))
	for i, msg := range items {
		reversed[len(items)-1-i] = msg
	}

	var sb strings.Builder
	for i, msg := range reversed {
		if msg == nil {
			continue
		}
		if i > 0 {
			sb.WriteByte('\n')
		}
		ts := formatTimestamp(deref(msg.CreateTime))
		sender := formatSender(msg.Sender)
		content := extractMessageContent(deref(msg.MsgType), msg.Body)
		fmt.Fprintf(&sb, "[%s] %s: %s", ts, sender, content)
	}
	return sb.String()
}

// formatTimestamp converts a millisecond Unix timestamp string to a human-readable format.
func formatTimestamp(ms string) string {
	if ms == "" {
		return "unknown"
	}
	var msInt int64
	for _, c := range ms {
		if c < '0' || c > '9' {
			return ms
		}
		msInt = msInt*10 + int64(c-'0')
	}
	t := time.Unix(msInt/1000, (msInt%1000)*int64(time.Millisecond))
	return t.Format("2006-01-02 15:04:05")
}

// formatSender returns a display string for the message sender.
func formatSender(s *larkim.Sender) string {
	if s == nil {
		return "unknown"
	}
	senderType := deref(s.SenderType)
	senderID := deref(s.Id)
	switch senderType {
	case "app":
		return "bot(" + senderID + ")"
	case "user":
		return "user(" + senderID + ")"
	default:
		if senderID != "" {
			return senderType + "(" + senderID + ")"
		}
		return senderType
	}
}

// extractMessageContent extracts readable content from a message body based on type.
func extractMessageContent(msgType string, body *larkim.MessageBody) string {
	if body == nil || body.Content == nil {
		return "[empty]"
	}
	raw := *body.Content
	switch msgType {
	case "text":
		return extractTextContent(raw)
	case "post":
		return "[rich text message]"
	case "image":
		return "[image]"
	case "file":
		return "[file]"
	case "audio":
		return "[audio]"
	case "media":
		return "[media]"
	case "sticker":
		return "[sticker]"
	case "interactive":
		return "[interactive card]"
	case "share_chat":
		return "[shared chat]"
	case "share_user":
		return "[shared user]"
	case "system":
		return "[system message]"
	default:
		return fmt.Sprintf("[%s]", msgType)
	}
}

// extractTextContent parses a Lark text message content JSON: {"text":"..."}.
func extractTextContent(raw string) string {
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return strings.TrimSpace(raw)
	}
	text := strings.TrimSpace(parsed.Text)
	if text == "" {
		return "[empty]"
	}
	return text
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
