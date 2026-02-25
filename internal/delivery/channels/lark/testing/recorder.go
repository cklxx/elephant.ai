package larktesting

import (
	"context"
	"fmt"
	"sync"
	"time"

	larkgw "alex/internal/delivery/channels/lark"
	agent "alex/internal/domain/agent/ports/agent"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// TraceEntry records a single interaction in a conversation trace.
type TraceEntry struct {
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
	Direction string    `json:"direction" yaml:"direction"` // "inbound", "outbound", "event"
	Type      string    `json:"type" yaml:"type"`           // "message", "reply", "reaction", "upload", "event"
	Method    string    `json:"method,omitempty" yaml:"method,omitempty"`
	SenderID  string    `json:"sender_id,omitempty" yaml:"sender_id,omitempty"`
	ChatID    string    `json:"chat_id,omitempty" yaml:"chat_id,omitempty"`
	ChatType  string    `json:"chat_type,omitempty" yaml:"chat_type,omitempty"`
	MessageID string    `json:"message_id,omitempty" yaml:"message_id,omitempty"`
	Content   string    `json:"content,omitempty" yaml:"content,omitempty"`
	MsgType   string    `json:"msg_type,omitempty" yaml:"msg_type,omitempty"`
	Emoji     string    `json:"emoji,omitempty" yaml:"emoji,omitempty"`
	EventType string    `json:"event_type,omitempty" yaml:"event_type,omitempty"`
	Error     string    `json:"error,omitempty" yaml:"error,omitempty"`
}

// ConversationTrace captures a full conversation for replay and scenario mining.
type ConversationTrace struct {
	mu      sync.Mutex
	entries []TraceEntry
}

// NewConversationTrace creates an empty trace.
func NewConversationTrace() *ConversationTrace {
	return &ConversationTrace{}
}

// Append adds an entry to the trace.
func (t *ConversationTrace) Append(entry TraceEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	t.entries = append(t.entries, entry)
}

// Entries returns a snapshot of all trace entries.
func (t *ConversationTrace) Entries() []TraceEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]TraceEntry, len(t.entries))
	copy(out, t.entries)
	return out
}

// Reset clears the trace.
func (t *ConversationTrace) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = nil
}

// RecordingEventListener wraps an EventListener to record domain events into a trace.
type RecordingEventListener struct {
	inner agent.EventListener
	trace *ConversationTrace
}

// NewRecordingEventListener wraps an inner listener and records events.
func NewRecordingEventListener(inner agent.EventListener, trace *ConversationTrace) *RecordingEventListener {
	return &RecordingEventListener{inner: inner, trace: trace}
}

// OnEvent forwards to the inner listener and records the event.
func (r *RecordingEventListener) OnEvent(event agent.AgentEvent) {
	if r.inner != nil {
		r.inner.OnEvent(event)
	}
	r.trace.Append(TraceEntry{
		Direction: "event",
		Type:      "event",
		EventType: fmt.Sprintf("%T", event),
	})
}

// TracingMessenger wraps a LarkMessenger and records all outbound calls.
type TracingMessenger struct {
	inner larkgw.LarkMessenger
	trace *ConversationTrace
}

// NewTracingMessenger wraps a messenger and records all calls to the trace.
func NewTracingMessenger(inner larkgw.LarkMessenger, trace *ConversationTrace) *TracingMessenger {
	return &TracingMessenger{inner: inner, trace: trace}
}

func (m *TracingMessenger) SendMessage(ctx context.Context, chatID, msgType, content string) (string, error) {
	id, err := m.inner.SendMessage(ctx, chatID, msgType, content)
	m.trace.Append(TraceEntry{
		Direction: "outbound",
		Type:      "message",
		Method:    "SendMessage",
		ChatID:    chatID,
		MsgType:   msgType,
		Content:   content,
		MessageID: id,
		Error:     errStr(err),
	})
	return id, err
}

func (m *TracingMessenger) ReplyMessage(ctx context.Context, replyToID, msgType, content string) (string, error) {
	id, err := m.inner.ReplyMessage(ctx, replyToID, msgType, content)
	m.trace.Append(TraceEntry{
		Direction: "outbound",
		Type:      "reply",
		Method:    "ReplyMessage",
		MessageID: id,
		MsgType:   msgType,
		Content:   content,
		Error:     errStr(err),
	})
	return id, err
}

func (m *TracingMessenger) UpdateMessage(ctx context.Context, messageID, msgType, content string) error {
	err := m.inner.UpdateMessage(ctx, messageID, msgType, content)
	m.trace.Append(TraceEntry{
		Direction: "outbound",
		Type:      "message",
		Method:    "UpdateMessage",
		MessageID: messageID,
		MsgType:   msgType,
		Content:   content,
		Error:     errStr(err),
	})
	return err
}

func (m *TracingMessenger) AddReaction(ctx context.Context, messageID, emojiType string) error {
	err := m.inner.AddReaction(ctx, messageID, emojiType)
	m.trace.Append(TraceEntry{
		Direction: "outbound",
		Type:      "reaction",
		Method:    "AddReaction",
		MessageID: messageID,
		Emoji:     emojiType,
		Error:     errStr(err),
	})
	return err
}

func (m *TracingMessenger) UploadImage(ctx context.Context, payload []byte) (string, error) {
	key, err := m.inner.UploadImage(ctx, payload)
	m.trace.Append(TraceEntry{
		Direction: "outbound",
		Type:      "upload",
		Method:    "UploadImage",
		Content:   fmt.Sprintf("(%d bytes)", len(payload)),
		Error:     errStr(err),
	})
	return key, err
}

func (m *TracingMessenger) UploadFile(ctx context.Context, payload []byte, fileName, fileType string) (string, error) {
	key, err := m.inner.UploadFile(ctx, payload, fileName, fileType)
	m.trace.Append(TraceEntry{
		Direction: "outbound",
		Type:      "upload",
		Method:    "UploadFile",
		Content:   fmt.Sprintf("%s (%s, %d bytes)", fileName, fileType, len(payload)),
		Error:     errStr(err),
	})
	return key, err
}

func (m *TracingMessenger) ListMessages(ctx context.Context, chatID string, pageSize int) ([]*larkim.Message, error) {
	items, err := m.inner.ListMessages(ctx, chatID, pageSize)
	m.trace.Append(TraceEntry{
		Direction: "outbound",
		Type:      "message",
		Method:    "ListMessages",
		ChatID:    chatID,
		Content:   fmt.Sprintf("pageSize=%d, returned=%d", pageSize, len(items)),
		Error:     errStr(err),
	})
	return items, err
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
