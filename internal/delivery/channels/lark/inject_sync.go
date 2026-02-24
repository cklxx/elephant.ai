package lark

import (
	"context"
	"fmt"
	"sync"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// InjectSyncRequest is the input for InjectMessageSync.
type InjectSyncRequest struct {
	ChatID   string        `json:"chat_id"`
	ChatType string        `json:"chat_type"`  // default "p2p"
	SenderID string        `json:"sender_id"`  // default "ou_inject_user"
	Text     string        `json:"text"`
	Timeout  time.Duration `json:"timeout"` // default 5min
}

// InjectSyncResponse captures the bot's replies after processing completes.
type InjectSyncResponse struct {
	Replies  []MessengerCall `json:"replies"`
	Duration time.Duration   `json:"duration"`
	Error    string          `json:"error,omitempty"`
}

const defaultInjectTimeout = 5 * time.Minute

// InjectMessageSync injects a message and blocks until the task completes,
// capturing all outbound messenger calls for the target chat.
func (g *Gateway) InjectMessageSync(ctx context.Context, req InjectSyncRequest) *InjectSyncResponse {
	start := g.currentTime()

	// Apply defaults.
	if req.ChatType == "" {
		req.ChatType = "p2p"
	}
	if req.SenderID == "" {
		req.SenderID = "ou_inject_user"
	}
	if req.Timeout <= 0 {
		req.Timeout = defaultInjectTimeout
	}
	if req.ChatID == "" {
		req.ChatID = fmt.Sprintf("inject-%d", start.UnixMilli())
	}

	// Install a tee messenger that captures outbound calls for this chatID.
	tee := newTeeMessenger(g.messenger, req.ChatID)
	original := g.messenger
	g.messenger = tee
	defer func() { g.messenger = original }()

	// Generate a unique message ID for dedup.
	messageID := fmt.Sprintf("inject_%s_%d", req.ChatID, start.UnixNano())

	// Inject the message through the normal pipeline.
	if err := g.InjectMessage(ctx, req.ChatID, req.ChatType, req.SenderID, messageID, req.Text); err != nil {
		return &InjectSyncResponse{
			Duration: g.currentTime().Sub(start),
			Error:    fmt.Sprintf("inject failed: %v", err),
		}
	}

	// Wait for the task to complete: poll slot phase until it returns to idle/awaiting.
	deadline := start.Add(req.Timeout)
	if err := g.waitForSlotIdle(ctx, req.ChatID, deadline); err != nil {
		return &InjectSyncResponse{
			Replies:  tee.captured(),
			Duration: g.currentTime().Sub(start),
			Error:    fmt.Sprintf("wait failed: %v", err),
		}
	}

	return &InjectSyncResponse{
		Replies:  tee.captured(),
		Duration: g.currentTime().Sub(start),
	}
}

// waitForSlotIdle polls the active slot for chatID until the phase is no longer
// slotRunning, or until the deadline or context is cancelled.
func (g *Gateway) waitForSlotIdle(ctx context.Context, chatID string, deadline time.Time) error {
	const pollInterval = 200 * time.Millisecond

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if g.currentTime().After(deadline) {
				return fmt.Errorf("timeout waiting for task to complete (chat=%s)", chatID)
			}
			raw, ok := g.activeSlots.Load(chatID)
			if !ok {
				// No slot means no task was started or it already cleaned up.
				return nil
			}
			slot := raw.(*sessionSlot)
			slot.mu.Lock()
			phase := slot.phase
			slot.mu.Unlock()
			if phase != slotRunning {
				return nil
			}
		}
	}
}

// teeMessenger wraps a real LarkMessenger, forwarding all calls to the inner
// messenger while capturing calls that target a specific chatID.
type teeMessenger struct {
	inner  LarkMessenger
	chatID string
	mu     sync.Mutex
	calls  []MessengerCall
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
	t.calls = append(t.calls, call)
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

func (t *teeMessenger) AddReaction(ctx context.Context, messageID, emojiType string) error {
	err := t.inner.AddReaction(ctx, messageID, emojiType)
	t.record(MessengerCall{Method: "AddReaction", MsgID: messageID, Emoji: emojiType})
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
