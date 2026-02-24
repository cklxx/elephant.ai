package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// InjectSyncRequest is the input for InjectMessageSync.
type InjectSyncRequest struct {
	ChatID             string        `json:"chat_id"`
	ChatType           string        `json:"chat_type"`             // default "p2p"
	SenderID           string        `json:"sender_id"`             // default "ou_inject_user"
	Text               string        `json:"text"`
	Timeout            time.Duration `json:"timeout"`               // default 5min
	AutoReply          bool          `json:"auto_reply"`            // enable auto-reply on await_user_input
	MaxAutoReplyRounds int           `json:"max_auto_reply_rounds"` // default 3
}

// InjectSyncResponse captures the bot's replies after processing completes.
type InjectSyncResponse struct {
	Replies     []MessengerCall `json:"replies"`
	Duration    time.Duration   `json:"duration"`
	Error       string          `json:"error,omitempty"`
	AutoReplies int             `json:"auto_replies,omitempty"` // actual auto-reply count
}

const (
	defaultInjectTimeout       = 5 * time.Minute
	defaultMaxAutoReplyRounds  = 3
	llmAutoReplyTimeout        = 10 * time.Second
)

// InjectMessageSync injects a message and blocks until the task completes,
// capturing all outbound messenger calls for the target chat.
// When AutoReply is enabled, the method automatically generates replies to
// agent clarification questions, resuming the slot up to MaxAutoReplyRounds.
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
	maxRounds := req.MaxAutoReplyRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxAutoReplyRounds
	}

	// Install a tee messenger that captures outbound calls for this chatID.
	// The tee forwards all calls to the real messenger and records matching ones.
	// We never swap g.messenger back — instead we disable recording when done.
	// This avoids a data race with detached goroutines (e.g. addReaction) that
	// read g.messenger after the task goroutine tracked by taskWG has finished.
	tee := newTeeMessenger(g.messenger, req.ChatID)
	g.messenger = tee

	// Generate a unique message ID for dedup.
	messageID := fmt.Sprintf("inject_%s_%d", req.ChatID, start.UnixNano())

	// Inject the message through the normal pipeline.
	if err := g.InjectMessage(ctx, req.ChatID, req.ChatType, req.SenderID, messageID, req.Text); err != nil {
		tee.disable()
		return &InjectSyncResponse{
			Duration: g.currentTime().Sub(start),
			Error:    fmt.Sprintf("inject failed: %v", err),
		}
	}

	autoReplies := 0
	for {
		// Each round gets an independent timeout.
		deadline := g.currentTime().Add(req.Timeout)
		waitErr := g.waitForSlotIdle(ctx, req.ChatID, deadline)

		if waitErr != nil {
			time.Sleep(500 * time.Millisecond)
			tee.disable()
			return &InjectSyncResponse{
				Replies:     tee.captured(),
				Duration:    g.currentTime().Sub(start),
				Error:       fmt.Sprintf("wait failed: %v", waitErr),
				AutoReplies: autoReplies,
			}
		}

		if !req.AutoReply || autoReplies >= maxRounds {
			break
		}

		// Check if the slot is awaiting user input.
		phase, options := g.getSlotPhaseAndOptions(req.ChatID)
		if phase != slotAwaitingInput {
			break // task completed normally
		}

		// Extract the agent's clarification question from captured calls.
		question := extractLastReplyText(tee.captured())
		replyText := g.generateAutoReply(ctx, req.Text, question, options)
		autoReplies++

		// Inject the auto-reply through the normal message pipeline.
		autoMsgID := fmt.Sprintf("inject_auto_%s_%d_%d", req.ChatID, start.UnixNano(), autoReplies)
		if err := g.InjectMessage(ctx, req.ChatID, req.ChatType, req.SenderID, autoMsgID, replyText); err != nil {
			break
		}
	}

	// Allow a short grace period for detached goroutines (e.g. addReaction)
	// to complete their messenger calls before we stop recording.
	time.Sleep(500 * time.Millisecond)
	tee.disable()

	return &InjectSyncResponse{
		Replies:     tee.captured(),
		Duration:    g.currentTime().Sub(start),
		AutoReplies: autoReplies,
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

// getSlotPhaseAndOptions atomically reads the slot's phase and pendingOptions.
func (g *Gateway) getSlotPhaseAndOptions(chatID string) (slotPhase, []string) {
	raw, ok := g.activeSlots.Load(chatID)
	if !ok {
		return slotIdle, nil
	}
	slot := raw.(*sessionSlot)
	slot.mu.Lock()
	phase := slot.phase
	opts := make([]string, len(slot.pendingOptions))
	copy(opts, slot.pendingOptions)
	slot.mu.Unlock()
	return phase, opts
}

// generateAutoReply uses LLM to generate an auto-reply; falls back to
// heuristic when the LLM factory is unavailable or the call fails.
func (g *Gateway) generateAutoReply(ctx context.Context, originalText, question string, options []string) string {
	if g.llmFactory != nil {
		if reply, err := g.llmAutoReply(ctx, originalText, question, options); err == nil {
			return reply
		}
	}
	return heuristicAutoReply(options)
}

// heuristicAutoReply returns a simple rule-based reply:
// pick the first option if any, otherwise a fixed "just do it" instruction.
func heuristicAutoReply(options []string) string {
	if len(options) > 0 {
		return "1"
	}
	return "请直接执行，不需要进一步确认"
}

// llmAutoReply calls a lightweight LLM to generate a context-aware reply,
// using the shared runtime LLM profile.
func (g *Gateway) llmAutoReply(ctx context.Context, originalText, question string, options []string) (string, error) {
	profile := g.llmProfile
	if strings.TrimSpace(profile.Provider) == "" || strings.TrimSpace(profile.Model) == "" {
		return "", fmt.Errorf("no LLM profile configured")
	}

	client, _, err := llmclient.GetClientFromProfile(g.llmFactory, profile, nil, false)
	if err != nil {
		return "", err
	}

	systemPrompt := `你是一个自动回复助手。用户给 AI 下了一个指令，AI 提出了澄清问题。
请根据原始指令生成一个简短的回复让 AI 继续执行，而不是继续追问。
只输出回复内容，不加任何解释。如果 AI 给出了编号选项，只回复最合适的选项编号。`

	userPrompt := fmt.Sprintf("原始指令: %s\n\nAI 的澄清问题: %s", originalText, question)
	if len(options) > 0 {
		userPrompt += "\n\n选项:\n"
		for i, opt := range options {
			userPrompt += fmt.Sprintf("[%d] %s\n", i+1, opt)
		}
	}

	callCtx, cancel := context.WithTimeout(ctx, llmAutoReplyTimeout)
	defer cancel()

	resp, err := client.Complete(callCtx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2,
		MaxTokens:   50,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

// extractLastReplyText extracts the text from the last non-reaction reply
// in a list of captured messenger calls.
func extractLastReplyText(calls []MessengerCall) string {
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].Method == "AddReaction" {
			continue
		}
		if text := extractTextFromContent(calls[i].Content); text != "" {
			return text
		}
	}
	return ""
}

// extractTextFromContent parses the "text" field from a Lark message JSON
// content string, falling back to the raw trimmed content.
func extractTextFromContent(content string) string {
	var obj struct {
		Text string `json:"text"`
	}
	if json.Unmarshal([]byte(content), &obj) == nil && obj.Text != "" {
		return obj.Text
	}
	return strings.TrimSpace(content)
}

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

// disable stops recording new calls. The tee continues to forward to inner.
func (t *teeMessenger) disable() {
	t.mu.Lock()
	t.stopped = true
	t.mu.Unlock()
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
