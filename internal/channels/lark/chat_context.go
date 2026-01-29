package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// fetchRecentChatMessages retrieves recent messages from a Lark chat and
// returns them formatted as chronological "[timestamp] sender: content" lines.
// This is used for auto chat context injection before task execution.
func fetchRecentChatMessages(ctx context.Context, client *lark.Client, chatID string, pageSize int) (string, error) {
	if client == nil {
		return "", fmt.Errorf("lark client is nil")
	}
	if chatID == "" {
		return "", fmt.Errorf("chat_id is empty")
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}

	req := larkim.NewListMessageReqBuilder().
		ContainerIdType("chat").
		ContainerId(chatID).
		SortType("ByCreateTimeDesc").
		PageSize(pageSize).
		Build()

	resp, err := client.Im.Message.List(ctx, req)
	if err != nil {
		return "", fmt.Errorf("lark chat history API call failed: %w", err)
	}
	if !resp.Success() {
		return "", fmt.Errorf("lark chat history API error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || len(resp.Data.Items) == 0 {
		return "", nil
	}

	return formatChatMessages(resp.Data.Items), nil
}

// formatChatMessages converts Lark message items (descending order from API)
// into chronological "[timestamp] sender: content" lines.
func formatChatMessages(items []*larkim.Message) string {
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
		ts := formatChatTimestamp(deref(msg.CreateTime))
		sender := formatChatSender(msg.Sender)
		content := extractChatMessageContent(deref(msg.MsgType), msg.Body)
		fmt.Fprintf(&sb, "[%s] %s: %s", ts, sender, content)
	}
	return sb.String()
}

// formatChatTimestamp converts a millisecond Unix timestamp string to a readable format.
func formatChatTimestamp(ms string) string {
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

// formatChatSender returns a display string for the message sender.
func formatChatSender(s *larkim.Sender) string {
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

// extractChatMessageContent extracts readable content from a message body based on type.
func extractChatMessageContent(msgType string, body *larkim.MessageBody) string {
	if body == nil || body.Content == nil {
		return "[empty]"
	}
	raw := *body.Content
	switch msgType {
	case "text":
		return extractChatTextContent(raw)
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

// extractChatTextContent parses a Lark text message content JSON: {"text":"..."}.
func extractChatTextContent(raw string) string {
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
