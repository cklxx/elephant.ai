package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type chatMessageLine struct {
	Timestamp  string
	Sender     string
	Content    string
	SenderType string
}

// fetchRecentChatMessages retrieves recent messages from a Lark chat via the
// gateway's messenger and returns them formatted as chronological
// "[timestamp] sender: content" lines.
func (g *Gateway) fetchRecentChatMessages(ctx context.Context, chatID string, pageSize int) (string, error) {
	if g.messenger == nil {
		return "", fmt.Errorf("lark messenger is nil")
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

	items, err := g.messenger.ListMessages(ctx, chatID, pageSize)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", nil
	}

	return formatChatMessages(items), nil
}

// fetchRecentChatRounds retrieves recent messages from a Lark chat and returns
// the latest N user-initiated chat rounds as chronological lines.
func (g *Gateway) fetchRecentChatRounds(ctx context.Context, chatID, excludeMessageID string, pageSize, maxRounds int) (string, error) {
	if g.messenger == nil {
		return "", fmt.Errorf("lark messenger is nil")
	}
	if chatID == "" {
		return "", fmt.Errorf("chat_id is empty")
	}
	if maxRounds <= 0 {
		maxRounds = defaultRecentChatMaxRounds
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	minRequired := maxRounds * 4
	if pageSize < minRequired {
		pageSize = minRequired
	}
	if pageSize > 50 {
		pageSize = 50
	}

	items, err := g.messenger.ListMessages(ctx, chatID, pageSize)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", nil
	}

	lines := mapChatMessagesChronological(items, excludeMessageID)
	lines = keepRecentChatRounds(lines, maxRounds)
	return formatChatMessageLines(lines), nil
}

// formatChatMessages converts Lark message items (descending order from API)
// into chronological "[timestamp] sender: content" lines.
func formatChatMessages(items []*larkim.Message) string {
	return formatChatMessageLines(mapChatMessagesChronological(items, ""))
}

func mapChatMessagesChronological(items []*larkim.Message, excludeMessageID string) []chatMessageLine {
	var lines []chatMessageLine
	for i := len(items) - 1; i >= 0; i-- {
		msg := items[i]
		if msg == nil {
			continue
		}
		if excludeMessageID != "" && strings.TrimSpace(deref(msg.MessageId)) == strings.TrimSpace(excludeMessageID) {
			continue
		}
		msgType := strings.ToLower(strings.TrimSpace(deref(msg.MsgType)))
		senderType := ""
		if msg.Sender != nil {
			senderType = strings.ToLower(strings.TrimSpace(deref(msg.Sender.SenderType)))
		}
		lines = append(lines, chatMessageLine{
			Timestamp:  formatChatTimestamp(deref(msg.CreateTime)),
			Sender:     formatChatSender(msg.Sender),
			Content:    extractChatMessageContent(msgType, msg.Body),
			SenderType: senderType,
		})
	}
	return lines
}

func keepRecentChatRounds(lines []chatMessageLine, maxRounds int) []chatMessageLine {
	if len(lines) == 0 || maxRounds <= 0 {
		return nil
	}
	var userRoundStarts []int
	for i, line := range lines {
		if line.SenderType == "user" {
			userRoundStarts = append(userRoundStarts, i)
		}
	}
	if len(userRoundStarts) == 0 {
		return lines
	}
	start := userRoundStarts[0]
	if len(userRoundStarts) > maxRounds {
		start = userRoundStarts[len(userRoundStarts)-maxRounds]
	}
	return lines[start:]
}

func formatChatMessageLines(lines []chatMessageLine) string {
	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		fmt.Fprintf(&sb, "[%s] %s: %s", line.Timestamp, line.Sender, line.Content)
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
		return extractChatPostContent(raw)
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
		return "[interactive message]"
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

// extractChatPostContent parses a Lark post message content JSON and flattens
// text elements into a readable string. The content field has the structure:
// {"title":"...","content":[[{"tag":"text","text":"..."},{"tag":"at","user_name":"..."}]]}
func extractChatPostContent(raw string) string {
	type postElement struct {
		Tag      string `json:"tag"`
		Text     string `json:"text"`
		UserName string `json:"user_name"`
	}
	type postPayload struct {
		Title   string          `json:"title"`
		Content [][]postElement `json:"content"`
	}

	var parsed postPayload
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "[rich text message]"
	}

	var sb strings.Builder
	if title := strings.TrimSpace(parsed.Title); title != "" {
		sb.WriteString(title)
	}
	for _, line := range parsed.Content {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		for _, el := range line {
			switch el.Tag {
			case "text":
				sb.WriteString(el.Text)
			case "at":
				if name := strings.TrimSpace(el.UserName); name != "" {
					sb.WriteString("@" + name)
				}
			default:
				if el.Text != "" {
					sb.WriteString(el.Text)
				}
			}
		}
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "[rich text message]"
	}
	return result
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
