package lark

import (
	"fmt"
	"strings"
	"sync"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type injectCaptureSession struct {
	chatID  string
	mu      sync.Mutex
	calls   []MessengerCall
	stopped bool
}

func (s *injectCaptureSession) disable() {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
}

func (s *injectCaptureSession) captured() []MessengerCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]MessengerCall, len(s.calls))
	copy(out, s.calls)
	return out
}

func (s *injectCaptureSession) record(call MessengerCall) {
	s.mu.Lock()
	if !s.stopped {
		s.calls = append(s.calls, call)
	}
	s.mu.Unlock()
}

type injectCaptureHandle struct {
	hub *injectCaptureHub
	id  uint64
}

func (h *injectCaptureHandle) disable() {
	if h.hub == nil || h.id == 0 {
		return
	}
	h.hub.disable(h.id)
}

func (h *injectCaptureHandle) captured() []MessengerCall {
	if h.hub == nil || h.id == 0 {
		return nil
	}
	return h.hub.captured(h.id)
}

func (h *injectCaptureHandle) close() {
	if h.hub == nil || h.id == 0 {
		return
	}
	h.hub.close(h.id)
}

// injectCaptureHub is a single long-lived messenger wrapper. Each inject call
// opens one capture session and closes it on return, avoiding wrapper stacking.
type injectCaptureHub struct {
	inner         LarkMessenger
	mu            sync.RWMutex
	nextID        uint64
	nextSynthetic uint64
	sessions      map[uint64]*injectCaptureSession
	chatHistory   map[string]*injectChatHistory // chat_id -> synthetic transcript (chronological)
	messageToChat map[string]string             // message_id -> chat_id
	syntheticChat map[string]bool               // chat_id -> synthetic inject source
}

func newInjectCaptureHub(inner LarkMessenger) *injectCaptureHub {
	return &injectCaptureHub{
		inner:         inner,
		sessions:      map[uint64]*injectCaptureSession{},
		chatHistory:   map[string]*injectChatHistory{},
		messageToChat: map[string]string{},
		syntheticChat: map[string]bool{},
	}
}

type injectChatHistory struct {
	messages []*larkim.Message // chronological (oldest -> newest)
	index    map[string]int
}

func (h *injectChatHistory) upsertLocked(msg *larkim.Message) {
	if msg == nil {
		return
	}
	msgID := strings.TrimSpace(deref(msg.MessageId))
	if msgID != "" {
		if idx, ok := h.index[msgID]; ok && idx >= 0 && idx < len(h.messages) {
			h.messages[idx] = msg
			return
		}
	}
	h.messages = append(h.messages, msg)
	if len(h.messages) > maxInjectHistoryPerChat {
		h.messages = h.messages[len(h.messages)-maxInjectHistoryPerChat:]
	}
	h.rebuildIndexLocked()
}

func (h *injectChatHistory) updateLocked(messageID, msgType, content string, ts time.Time) bool {
	idx, ok := h.index[strings.TrimSpace(messageID)]
	if !ok || idx < 0 || idx >= len(h.messages) {
		return false
	}
	msg := h.messages[idx]
	if msg == nil {
		return false
	}
	if msgType != "" {
		msgTypeCopy := msgType
		msg.MsgType = &msgTypeCopy
	}
	contentCopy := content
	msg.Body = &larkim.MessageBody{Content: &contentCopy}
	createTime := formatMillis(ts.UnixMilli())
	msg.CreateTime = &createTime
	h.messages[idx] = msg
	return true
}

func (h *injectChatHistory) rebuildIndexLocked() {
	idx := make(map[string]int, len(h.messages))
	for i, msg := range h.messages {
		if msg == nil {
			continue
		}
		msgID := strings.TrimSpace(deref(msg.MessageId))
		if msgID == "" {
			continue
		}
		idx[msgID] = i
	}
	h.index = idx
}

func (h *injectCaptureHub) ensureChatHistoryLocked(chatID string) *injectChatHistory {
	history, ok := h.chatHistory[chatID]
	if ok && history != nil {
		return history
	}
	history = &injectChatHistory{
		index: map[string]int{},
	}
	h.chatHistory[chatID] = history
	return history
}

func (h *injectCaptureHub) nextSyntheticMessageIDLocked(chatID string) string {
	h.nextSynthetic++
	return fmt.Sprintf("inject_local_%s_%d", chatID, h.nextSynthetic)
}

func (h *injectCaptureHub) appendSyntheticMessageLocked(chatID string, msg *larkim.Message) {
	if strings.TrimSpace(chatID) == "" || msg == nil {
		return
	}
	history := h.ensureChatHistoryLocked(chatID)
	history.upsertLocked(msg)
	msgID := strings.TrimSpace(deref(msg.MessageId))
	if msgID != "" {
		h.messageToChat[msgID] = chatID
	}
}

func (h *injectCaptureHub) recordInjectedIncoming(chatID, messageID, senderID, msgType, content string, ts time.Time) {
	chatID = strings.TrimSpace(chatID)
	messageID = strings.TrimSpace(messageID)
	if chatID == "" || messageID == "" {
		return
	}
	if strings.TrimSpace(senderID) == "" {
		senderID = "ou_inject_user"
	}
	msg := buildInjectHistoryMessage(messageID, msgType, content, "user", senderID, ts)

	h.mu.Lock()
	h.appendSyntheticMessageLocked(chatID, msg)
	if isInjectSyntheticMessageID(messageID) {
		h.syntheticChat[chatID] = true
	}
	h.mu.Unlock()
}

func (h *injectCaptureHub) recordSyntheticSend(chatID, messageID, msgType, content string, ts time.Time) {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return
	}
	h.mu.Lock()
	if _, tracked := h.chatHistory[chatID]; !tracked {
		h.mu.Unlock()
		return
	}
	if strings.TrimSpace(messageID) == "" {
		messageID = h.nextSyntheticMessageIDLocked(chatID)
	}
	msg := buildInjectHistoryMessage(messageID, msgType, content, "app", injectBotSenderID, ts)
	h.appendSyntheticMessageLocked(chatID, msg)
	h.mu.Unlock()
}

func (h *injectCaptureHub) recordSyntheticReply(replyToID, messageID, msgType, content string, ts time.Time) {
	replyToID = strings.TrimSpace(replyToID)
	if replyToID == "" {
		return
	}
	h.mu.Lock()
	chatID, ok := h.messageToChat[replyToID]
	if !ok || chatID == "" {
		h.mu.Unlock()
		return
	}
	if strings.TrimSpace(messageID) == "" {
		messageID = h.nextSyntheticMessageIDLocked(chatID)
	}
	msg := buildInjectHistoryMessage(messageID, msgType, content, "app", injectBotSenderID, ts)
	h.appendSyntheticMessageLocked(chatID, msg)
	h.mu.Unlock()
}

func (h *injectCaptureHub) recordSyntheticUpdate(messageID, msgType, content string, ts time.Time) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return
	}
	h.mu.Lock()
	chatID, ok := h.messageToChat[messageID]
	if !ok || chatID == "" {
		h.mu.Unlock()
		return
	}
	history := h.chatHistory[chatID]
	if history == nil {
		h.mu.Unlock()
		return
	}
	if !history.updateLocked(messageID, msgType, content, ts) {
		msg := buildInjectHistoryMessage(messageID, msgType, content, "app", injectBotSenderID, ts)
		h.appendSyntheticMessageLocked(chatID, msg)
	}
	h.mu.Unlock()
}

func (h *injectCaptureHub) syntheticMessages(chatID string, pageSize int) []*larkim.Message {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil
	}
	h.mu.RLock()
	history := h.chatHistory[chatID]
	if history == nil || len(history.messages) == 0 {
		h.mu.RUnlock()
		return nil
	}
	asc := history.messages
	limit := len(asc)
	if pageSize > 0 && limit > pageSize {
		limit = pageSize
	}
	out := make([]*larkim.Message, 0, limit)
	for i := len(asc) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, cloneInjectHistoryMessage(asc[i]))
	}
	h.mu.RUnlock()
	return out
}

func (h *injectCaptureHub) startCapture(chatID string) *injectCaptureHandle {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	id := h.nextID
	h.sessions[id] = &injectCaptureSession{chatID: chatID}
	return &injectCaptureHandle{hub: h, id: id}
}

func (h *injectCaptureHub) disable(id uint64) {
	h.mu.RLock()
	session := h.sessions[id]
	h.mu.RUnlock()
	if session != nil {
		session.disable()
	}
}

func (h *injectCaptureHub) captured(id uint64) []MessengerCall {
	h.mu.RLock()
	session := h.sessions[id]
	h.mu.RUnlock()
	if session == nil {
		return nil
	}
	return session.captured()
}

func (h *injectCaptureHub) close(id uint64) {
	h.mu.Lock()
	delete(h.sessions, id)
	h.mu.Unlock()
}

func (h *injectCaptureHub) recordByChat(chatID string, call MessengerCall) {
	h.mu.RLock()
	targets := make([]*injectCaptureSession, 0, len(h.sessions))
	for _, session := range h.sessions {
		if session.chatID == chatID {
			targets = append(targets, session)
		}
	}
	h.mu.RUnlock()
	for _, session := range targets {
		session.record(call)
	}
}

func (h *injectCaptureHub) recordAll(call MessengerCall) {
	h.mu.RLock()
	targets := make([]*injectCaptureSession, 0, len(h.sessions))
	for _, session := range h.sessions {
		targets = append(targets, session)
	}
	h.mu.RUnlock()
	for _, session := range targets {
		session.record(call)
	}
}
