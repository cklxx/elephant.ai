package lark

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// parseIncomingMessage validates the event and extracts key fields.
// Returns nil if the message should be skipped (unsupported type, disallowed
// chat, empty content, duplicate, etc.).
func (g *Gateway) parseIncomingMessage(event *larkim.P2MessageReceiveV1, opts messageProcessingOptions) *incomingMessage {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}
	raw := event.Event.Message

	msgType := strings.ToLower(strings.TrimSpace(deref(raw.MessageType)))
	if msgType != "text" && msgType != "post" {
		return nil
	}

	chatType := strings.ToLower(strings.TrimSpace(deref(raw.ChatType)))
	isGroup := chatType != "" && chatType != "p2p"
	if isGroup && !g.cfg.AllowGroups {
		return nil
	}
	if !isGroup && !g.cfg.AllowDirect {
		return nil
	}

	content := g.extractMessageContent(msgType, deref(raw.Content), raw.Mentions)
	if content == "" {
		return nil
	}

	chatID := deref(raw.ChatId)
	if chatID == "" {
		g.logger.Warn("Lark message has empty chat_id; skipping")
		return nil
	}

	messageID := deref(raw.MessageId)
	if messageID != "" && !opts.skipDedup && g.isDuplicateMessage(messageID) {
		g.logger.Warn("Lark duplicate message skipped (WS re-delivery): msg_id=%s", messageID)
		return nil
	}

	return &incomingMessage{
		chatID:    chatID,
		chatType:  chatType,
		messageID: messageID,
		senderID:  extractSenderID(event),
		content:   content,
		isGroup:   isGroup,
		isFromBot: isBotSender(event),
	}
}

// extractMessageContent parses the JSON content from a Lark message.
// Supports "text" and "post" message types, returning a trimmed string.
func (g *Gateway) extractMessageContent(msgType, raw string, mentions []*larkim.MentionEvent) string {
	switch msgType {
	case "text":
		return extractTextContent(raw, mentions)
	case "post":
		return extractPostContent(raw, mentions)
	default:
		return ""
	}
}

type mentionInfo struct {
	Name string
	ID   string
}

func mentionKeyMap(mentions []*larkim.MentionEvent) map[string]mentionInfo {
	if len(mentions) == 0 {
		return nil
	}
	out := make(map[string]mentionInfo, len(mentions))
	for _, mention := range mentions {
		if mention == nil {
			continue
		}
		key := strings.TrimSpace(deref(mention.Key))
		if key == "" {
			continue
		}
		name := strings.TrimSpace(deref(mention.Name))
		id := ""
		if mention.Id != nil {
			id = strings.TrimSpace(deref(mention.Id.OpenId))
			if id == "" {
				id = strings.TrimSpace(deref(mention.Id.UserId))
			}
			if id == "" {
				id = strings.TrimSpace(deref(mention.Id.UnionId))
			}
		}
		out[key] = mentionInfo{Name: name, ID: id}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func withAtPrefix(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "@") {
		return trimmed
	}
	return "@" + trimmed
}

func formatReadableMention(name, id, fallback string) string {
	name = strings.TrimSpace(name)
	id = strings.TrimSpace(id)
	fallback = strings.TrimSpace(fallback)

	if name != "" {
		atName := withAtPrefix(name)
		if id != "" && id != name {
			return atName + "(" + id + ")"
		}
		return atName
	}
	if id != "" {
		return withAtPrefix(id)
	}
	if fallback != "" {
		return withAtPrefix(fallback)
	}
	return ""
}

func renderIncomingMentionPlaceholders(text string, mentionMap map[string]mentionInfo) string {
	if strings.TrimSpace(text) == "" || len(mentionMap) == 0 {
		return text
	}

	keys := make([]string, 0, len(mentionMap))
	for key := range mentionMap {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return text
	}

	// Replace longer keys first to avoid `@_user_1` corrupting `@_user_10`.
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] > keys[j]
	})

	out := text
	for _, key := range keys {
		info := mentionMap[key]
		repl := formatReadableMention(info.Name, info.ID, key)
		if repl == "" || repl == key {
			continue
		}
		out = strings.ReplaceAll(out, key, repl)
	}
	return out
}

// extractTextContent parses a Lark text message content JSON: {"text":"..."}.
func extractTextContent(raw string, mentions []*larkim.MentionEvent) string {
	if raw == "" {
		return ""
	}
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return strings.TrimSpace(raw)
	}
	text := strings.TrimSpace(parsed.Text)
	if text == "" {
		return ""
	}
	mentionMap := mentionKeyMap(mentions)
	text = renderIncomingMentionPlaceholders(text, mentionMap)
	text = renderTextMentions(text, mentionMap)
	return strings.TrimSpace(text)
}

var larkMentionTag = regexp.MustCompile(`<at\s+user_id="([^"]+)"\s*>([^<]*)</at>`)

func renderTextMentions(text string, mentionMap map[string]mentionInfo) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	return larkMentionTag.ReplaceAllStringFunc(text, func(m string) string {
		sub := larkMentionTag.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		userID := strings.TrimSpace(sub[1])
		name := strings.TrimSpace(sub[2])

		mentionID := userID
		if mentionMap != nil {
			if info, ok := mentionMap[userID]; ok {
				if name == "" {
					name = info.Name
				}
				if strings.TrimSpace(info.ID) != "" {
					mentionID = info.ID
				}
			}
		}
		if name == "" {
			name = mentionID
		}
		if mentionID == "" || name == "" {
			return m
		}
		if mentionID == name {
			return withAtPrefix(name)
		}
		return withAtPrefix(name) + "(" + mentionID + ")"
	})
}

// extractPostContent parses a Lark post message content JSON and flattens text.
// The content field is a JSON string like:
// {"title":"...","content":[[{"tag":"text","text":"..."}]]}
func extractPostContent(raw string, mentions []*larkim.MentionEvent) string {
	if raw == "" {
		return ""
	}

	type postElement struct {
		Tag      string `json:"tag"`
		Text     string `json:"text"`
		UserID   string `json:"user_id"`
		UserName string `json:"user_name"`
	}
	type postPayload struct {
		Title   string          `json:"title"`
		Content [][]postElement `json:"content"`
	}

	var parsed postPayload
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return strings.TrimSpace(raw)
	}

	mentionMap := mentionKeyMap(mentions)
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
				sb.WriteString(renderIncomingMentionPlaceholders(el.Text, mentionMap))
			case "at":
				rawUserID := strings.TrimSpace(el.UserID)
				userID := rawUserID
				name := strings.TrimSpace(el.UserName)
				if mentionMap != nil {
					if info, ok := mentionMap[rawUserID]; ok {
						if name == "" {
							name = info.Name
						}
						if strings.TrimSpace(info.ID) != "" {
							userID = info.ID
						}
					}
				}
				if mention := formatReadableMention(name, userID, rawUserID); mention != "" {
					sb.WriteString(mention)
				}
			default:
				if el.Text != "" {
					sb.WriteString(el.Text)
				}
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

// textContent builds the JSON content payload for a Lark text message.
func textContent(text string) string {
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

func imageContent(imageKey string) string {
	payload, _ := json.Marshal(map[string]string{"image_key": imageKey})
	return string(payload)
}

func fileContent(fileKey string) string {
	payload, _ := json.Marshal(map[string]string{"file_key": fileKey})
	return string(payload)
}

// extractSenderID extracts sender identity from a Lark message event, preferring
// open_id and falling back to user_id/union_id.
func extractSenderID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Sender == nil || event.Event.Sender.SenderId == nil {
		return ""
	}
	id := strings.TrimSpace(deref(event.Event.Sender.SenderId.OpenId))
	if id != "" {
		return id
	}
	id = strings.TrimSpace(deref(event.Event.Sender.SenderId.UserId))
	if id != "" {
		return id
	}
	return strings.TrimSpace(deref(event.Event.Sender.SenderId.UnionId))
}

// extractMentions extracts mentioned user IDs from a Lark message event.
func extractMentions(event *larkim.P2MessageReceiveV1) []string {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}
	mentions := event.Event.Message.Mentions
	if len(mentions) == 0 {
		return nil
	}
	var ids []string
	for _, m := range mentions {
		if m == nil || m.Id == nil {
			continue
		}
		id := deref(m.Id.OpenId)
		if id == "" {
			id = deref(m.Id.UserId)
		}
		if id == "" {
			id = deref(m.Id.UnionId)
		}
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// isBotSender checks if the message sender is a bot (app).
func isBotSender(event *larkim.P2MessageReceiveV1) bool {
	if event == nil || event.Event == nil || event.Event.Sender == nil {
		return false
	}
	return deref(event.Event.Sender.SenderType) == "app"
}
