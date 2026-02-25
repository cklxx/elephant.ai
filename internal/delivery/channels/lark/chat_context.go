package lark

import (
	"context"
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

const larkHistorySummaryChars = 50

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
		fmt.Fprintf(
			&sb,
			"%d | role=%s | sender=%s | time=%s | summary=%s",
			i+1,
			normalizeChatRole(line.SenderType),
			line.Sender,
			line.Timestamp,
			summarizeChatContent(line.Content, larkHistorySummaryChars),
		)
	}
	return sb.String()
}

func normalizeChatRole(senderType string) string {
	switch strings.ToLower(strings.TrimSpace(senderType)) {
	case "user":
		return "user"
	case "app":
		return "assistant"
	default:
		trimmed := strings.ToLower(strings.TrimSpace(senderType))
		if trimmed == "" {
			return "message"
		}
		return trimmed
	}
}

func summarizeChatContent(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "[empty]"
	}
	normalized := strings.Join(strings.Fields(trimmed), " ")
	if limit <= 0 {
		return normalized
	}
	runes := []rune(normalized)
	if len(runes) <= limit {
		return normalized
	}
	return string(runes[:limit]) + "…"
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
	parsed, ok := parseLarkPostPayload(raw)
	if !ok {
		return "[rich text message]"
	}

	result := flattenLarkPostPayload(
		parsed,
		func(el larkPostElement) string {
			return el.Text
		},
		func(el larkPostElement) string {
			if name := strings.TrimSpace(el.UserName); name != "" {
				return "@" + name
			}
			return ""
		},
	)
	if result == "" {
		return "[rich text message]"
	}
	return result
}

// extractChatTextContent parses a Lark text message content JSON: {"text":"..."}.
func extractChatTextContent(raw string) string {
	text, ok := parseLarkTextPayload(raw)
	if !ok {
		return strings.TrimSpace(raw)
	}
	if text == "" {
		return "[empty]"
	}
	return text
}
