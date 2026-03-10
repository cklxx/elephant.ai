package lark

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// --- injectCaptureHub LarkMessenger implementation ---

func (h *injectCaptureHub) SendMessage(ctx context.Context, chatID, msgType, content string) (string, error) {
	chatID = strings.TrimSpace(chatID)
	h.mu.RLock()
	synthetic := h.syntheticChat[chatID]
	h.mu.RUnlock()
	if synthetic {
		h.mu.Lock()
		id := h.nextSyntheticMessageIDLocked(chatID)
		h.mu.Unlock()
		h.recordByChat(chatID, MessengerCall{Method: "SendMessage", ChatID: chatID, MsgType: msgType, Content: content})
		h.recordSyntheticSend(chatID, id, msgType, content, time.Now())
		return id, nil
	}

	id, err := h.inner.SendMessage(ctx, chatID, msgType, content)
	h.recordByChat(chatID, MessengerCall{Method: "SendMessage", ChatID: chatID, MsgType: msgType, Content: content})
	h.recordSyntheticSend(chatID, id, msgType, content, time.Now())
	return id, err
}

func (h *injectCaptureHub) ReplyMessage(ctx context.Context, replyToID, msgType, content string) (string, error) {
	replyToID = strings.TrimSpace(replyToID)
	h.mu.RLock()
	chatID := h.messageToChat[replyToID]
	synthetic := h.syntheticChat[chatID]
	h.mu.RUnlock()
	if synthetic {
		h.mu.Lock()
		id := h.nextSyntheticMessageIDLocked(chatID)
		h.mu.Unlock()
		h.recordAll(MessengerCall{Method: "ReplyMessage", ReplyTo: replyToID, MsgType: msgType, Content: content})
		h.recordSyntheticReply(replyToID, id, msgType, content, time.Now())
		return id, nil
	}

	id, err := h.inner.ReplyMessage(ctx, replyToID, msgType, content)
	h.recordAll(MessengerCall{Method: "ReplyMessage", ReplyTo: replyToID, MsgType: msgType, Content: content})
	h.recordSyntheticReply(replyToID, id, msgType, content, time.Now())
	return id, err
}

func (h *injectCaptureHub) UpdateMessage(ctx context.Context, messageID, msgType, content string) error {
	messageID = strings.TrimSpace(messageID)
	h.mu.RLock()
	chatID := h.messageToChat[messageID]
	synthetic := h.syntheticChat[chatID]
	h.mu.RUnlock()
	if synthetic {
		h.recordAll(MessengerCall{Method: "UpdateMessage", MsgID: messageID, MsgType: msgType, Content: content})
		h.recordSyntheticUpdate(messageID, msgType, content, time.Now())
		return nil
	}

	err := h.inner.UpdateMessage(ctx, messageID, msgType, content)
	h.recordAll(MessengerCall{Method: "UpdateMessage", MsgID: messageID, MsgType: msgType, Content: content})
	h.recordSyntheticUpdate(messageID, msgType, content, time.Now())
	return err
}

func (h *injectCaptureHub) AddReaction(ctx context.Context, messageID, emojiType string) (string, error) {
	messageID = strings.TrimSpace(messageID)
	h.mu.RLock()
	chatID := h.messageToChat[messageID]
	synthetic := h.syntheticChat[chatID]
	h.mu.RUnlock()
	if synthetic {
		h.recordAll(MessengerCall{Method: "AddReaction", MsgID: messageID, Emoji: emojiType})
		return fmt.Sprintf("synthetic_reaction_%s_%s", messageID, emojiType), nil
	}

	reactionID, err := h.inner.AddReaction(ctx, messageID, emojiType)
	h.recordAll(MessengerCall{Method: "AddReaction", MsgID: messageID, Emoji: emojiType})
	return reactionID, err
}

func (h *injectCaptureHub) DeleteReaction(ctx context.Context, messageID, reactionID string) error {
	messageID = strings.TrimSpace(messageID)
	h.mu.RLock()
	chatID := h.messageToChat[messageID]
	synthetic := h.syntheticChat[chatID]
	h.mu.RUnlock()
	if synthetic {
		h.recordAll(MessengerCall{Method: "DeleteReaction", MsgID: messageID, ReactionID: reactionID})
		return nil
	}

	err := h.inner.DeleteReaction(ctx, messageID, reactionID)
	h.recordAll(MessengerCall{Method: "DeleteReaction", MsgID: messageID, ReactionID: reactionID})
	return err
}

func (h *injectCaptureHub) UploadImage(ctx context.Context, payload []byte) (string, error) {
	return h.inner.UploadImage(ctx, payload)
}

func (h *injectCaptureHub) UploadFile(ctx context.Context, payload []byte, fileName, fileType string) (string, error) {
	return h.inner.UploadFile(ctx, payload, fileName, fileType)
}

func (h *injectCaptureHub) ListMessages(ctx context.Context, chatID string, pageSize int) ([]*larkim.Message, error) {
	synthetic := h.syntheticMessages(chatID, pageSize)
	items, err := h.inner.ListMessages(ctx, chatID, pageSize)
	if err != nil {
		if len(synthetic) > 0 {
			return synthetic, nil
		}
		return nil, err
	}
	if len(synthetic) == 0 {
		return items, nil
	}
	return mergeMessageHistoryDesc(items, synthetic, pageSize), nil
}

// --- teeMessenger ---

// teeMessenger wraps a real LarkMessenger, forwarding all calls to the inner
// messenger while capturing calls that target a specific chatID.
// Once disabled, it continues forwarding but stops recording.
type teeMessenger struct {
	inner   LarkMessenger
	chatID  string
	mu      sync.Mutex
	calls   []MessengerCall
	stopped bool
}

func newTeeMessenger(inner LarkMessenger, chatID string) *teeMessenger {
	return &teeMessenger{inner: inner, chatID: chatID}
}

func (t *teeMessenger) captured() []MessengerCall {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]MessengerCall, len(t.calls))
	copy(out, t.calls)
	return out
}

func (t *teeMessenger) record(call MessengerCall) {
	t.mu.Lock()
	if !t.stopped {
		t.calls = append(t.calls, call)
	}
	t.mu.Unlock()
}

func (t *teeMessenger) SendMessage(ctx context.Context, chatID, msgType, content string) (string, error) {
	id, err := t.inner.SendMessage(ctx, chatID, msgType, content)
	if chatID == t.chatID {
		t.record(MessengerCall{Method: "SendMessage", ChatID: chatID, MsgType: msgType, Content: content})
	}
	return id, err
}

func (t *teeMessenger) ReplyMessage(ctx context.Context, replyToID, msgType, content string) (string, error) {
	id, err := t.inner.ReplyMessage(ctx, replyToID, msgType, content)
	// ReplyMessage doesn't carry chatID; always capture since we're in inject context.
	t.record(MessengerCall{Method: "ReplyMessage", ReplyTo: replyToID, MsgType: msgType, Content: content})
	return id, err
}

func (t *teeMessenger) UpdateMessage(ctx context.Context, messageID, msgType, content string) error {
	err := t.inner.UpdateMessage(ctx, messageID, msgType, content)
	t.record(MessengerCall{Method: "UpdateMessage", MsgID: messageID, MsgType: msgType, Content: content})
	return err
}

func (t *teeMessenger) AddReaction(ctx context.Context, messageID, emojiType string) (string, error) {
	reactionID, err := t.inner.AddReaction(ctx, messageID, emojiType)
	t.record(MessengerCall{Method: "AddReaction", MsgID: messageID, Emoji: emojiType})
	return reactionID, err
}

func (t *teeMessenger) DeleteReaction(ctx context.Context, messageID, reactionID string) error {
	err := t.inner.DeleteReaction(ctx, messageID, reactionID)
	t.record(MessengerCall{Method: "DeleteReaction", MsgID: messageID, ReactionID: reactionID})
	return err
}

func (t *teeMessenger) UploadImage(ctx context.Context, payload []byte) (string, error) {
	return t.inner.UploadImage(ctx, payload)
}

func (t *teeMessenger) UploadFile(ctx context.Context, payload []byte, fileName, fileType string) (string, error) {
	return t.inner.UploadFile(ctx, payload, fileName, fileType)
}

func (t *teeMessenger) ListMessages(ctx context.Context, chatID string, pageSize int) ([]*larkim.Message, error) {
	return t.inner.ListMessages(ctx, chatID, pageSize)
}

// --- Helper functions ---

func mergeMessageHistoryDesc(primary, extra []*larkim.Message, limit int) []*larkim.Message {
	if len(primary) == 0 && len(extra) == 0 {
		return nil
	}
	combined := make([]*larkim.Message, 0, len(primary)+len(extra))
	combined = append(combined, primary...)
	combined = append(combined, extra...)

	sort.SliceStable(combined, func(i, j int) bool {
		ti := messageCreateMillis(combined[i])
		tj := messageCreateMillis(combined[j])
		if ti == tj {
			return strings.TrimSpace(deref(combined[i].MessageId)) > strings.TrimSpace(deref(combined[j].MessageId))
		}
		return ti > tj
	})

	out := make([]*larkim.Message, 0, len(combined))
	seen := make(map[string]struct{}, len(combined))
	for _, item := range combined {
		if item == nil {
			continue
		}
		msgID := strings.TrimSpace(deref(item.MessageId))
		if msgID != "" {
			if _, ok := seen[msgID]; ok {
				continue
			}
			seen[msgID] = struct{}{}
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func messageCreateMillis(msg *larkim.Message) int64 {
	if msg == nil {
		return 0
	}
	raw := strings.TrimSpace(deref(msg.CreateTime))
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func buildInjectHistoryMessage(messageID, msgType, content, senderType, senderID string, ts time.Time) *larkim.Message {
	messageID = strings.TrimSpace(messageID)
	msgType = strings.TrimSpace(msgType)
	if msgType == "" {
		msgType = "text"
	}
	senderType = strings.TrimSpace(senderType)
	if senderType == "" {
		senderType = "user"
	}
	senderID = strings.TrimSpace(senderID)
	if senderID == "" {
		senderID = "ou_inject_user"
	}

	contentCopy := content
	msgTypeCopy := msgType
	messageIDCopy := messageID
	createTime := formatMillis(ts.UnixMilli())
	senderTypeCopy := senderType
	senderIDCopy := senderID

	return &larkim.Message{
		MessageId:  &messageIDCopy,
		MsgType:    &msgTypeCopy,
		CreateTime: &createTime,
		Body:       &larkim.MessageBody{Content: &contentCopy},
		Sender: &larkim.Sender{
			SenderType: &senderTypeCopy,
			Id:         &senderIDCopy,
		},
	}
}

func cloneInjectHistoryMessage(msg *larkim.Message) *larkim.Message {
	if msg == nil {
		return nil
	}
	messageID := strings.TrimSpace(deref(msg.MessageId))
	msgType := strings.TrimSpace(deref(msg.MsgType))
	createTime := strings.TrimSpace(deref(msg.CreateTime))
	content := ""
	if msg.Body != nil {
		content = deref(msg.Body.Content)
	}
	senderType := ""
	senderID := ""
	if msg.Sender != nil {
		senderType = strings.TrimSpace(deref(msg.Sender.SenderType))
		senderID = strings.TrimSpace(deref(msg.Sender.Id))
	}
	msgTypeCopy := msgType
	messageIDCopy := messageID
	createTimeCopy := createTime
	contentCopy := content
	senderTypeCopy := senderType
	senderIDCopy := senderID
	return &larkim.Message{
		MessageId:  &messageIDCopy,
		MsgType:    &msgTypeCopy,
		CreateTime: &createTimeCopy,
		Body:       &larkim.MessageBody{Content: &contentCopy},
		Sender: &larkim.Sender{
			SenderType: &senderTypeCopy,
			Id:         &senderIDCopy,
		},
	}
}

func formatMillis(v int64) string {
	return strconv.FormatInt(v, 10)
}
