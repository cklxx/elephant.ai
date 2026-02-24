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
	ChatType           string        `json:"chat_type"` // default "p2p"
	SenderID           string        `json:"sender_id"` // default "ou_inject_user"
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
	defaultInjectTimeout      = 5 * time.Minute
	defaultMaxAutoReplyRounds = 3
	llmAutoReplyTimeout       = 10 * time.Second
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

	// Reuse a single hub messenger and open a per-request capture session.
	// This avoids stacking one tee layer per inject call under load.
	captureHub, err := g.ensureInjectCaptureHub()
	if err != nil {
		return &InjectSyncResponse{
			Duration: g.currentTime().Sub(start),
			Error:    fmt.Sprintf("inject failed: %v", err),
		}
	}
	capture := captureHub.startCapture(req.ChatID)
	defer capture.close()

	// Generate a unique message ID for dedup.
	messageID := fmt.Sprintf("inject_%s_%d", req.ChatID, start.UnixNano())

	// Inject the message through the normal pipeline.
	if err := g.InjectMessage(ctx, req.ChatID, req.ChatType, req.SenderID, messageID, req.Text); err != nil {
		capture.disable()
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
			// Prevent timed-out inject runs from leaving a runaway task
			// goroutine consuming resources in the background.
			if g.cancelRunningTask(req.ChatID) {
				_ = g.waitForSlotIdle(context.Background(), req.ChatID, g.currentTime().Add(3*time.Second))
			}
			time.Sleep(200 * time.Millisecond)
			capture.disable()
			return &InjectSyncResponse{
				Replies:     capture.captured(),
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
		question := extractLastReplyText(capture.captured())
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
	capture.disable()

	return &InjectSyncResponse{
		Replies:     capture.captured(),
		Duration:    g.currentTime().Sub(start),
		AutoReplies: autoReplies,
	}
}

func (g *Gateway) ensureInjectCaptureHub() (*injectCaptureHub, error) {
	if g == nil || g.messenger == nil {
		return nil, fmt.Errorf("lark messenger not initialized")
	}
	if hub, ok := g.messenger.(*injectCaptureHub); ok {
		return hub, nil
	}
	// Fallback for tests that construct Gateway literals without Start()/SetMessenger().
	hub := newInjectCaptureHub(g.messenger)
	g.messenger = hub
	return hub, nil
}

// cancelRunningTask cancels the currently running task for chatID when present.
func (g *Gateway) cancelRunningTask(chatID string) bool {
	raw, ok := g.activeSlots.Load(chatID)
	if !ok {
		return false
	}
	slot := raw.(*sessionSlot)

	slot.mu.Lock()
	phase := slot.phase
	cancel := slot.taskCancel
	slot.mu.Unlock()

	if phase != slotRunning || cancel == nil {
		return false
	}
	cancel()
	return true
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
	return "Proceed directly, no further confirmation needed."
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

	systemPrompt := `You are an auto-reply assistant. The user gave the AI an instruction, and the AI asked a clarification question.
Generate a short reply based on the original instruction that lets the AI continue executing, rather than asking more questions.
Output only the reply content with no explanation. If the AI presented numbered options, reply with the most appropriate option number only.`

	userPrompt := fmt.Sprintf("Original instruction: %s\n\nAI's clarification question: %s", originalText, question)
	if len(options) > 0 {
		userPrompt += "\n\nOptions:\n"
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

type injectCaptureSession struct {
	chatID  string
	mu      sync.Mutex
	calls   []MessengerCall
	stopped bool
}

func (s *injectCaptureSession) disable() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
}

func (s *injectCaptureSession) captured() []MessengerCall {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]MessengerCall, len(s.calls))
	copy(out, s.calls)
	return out
}

func (s *injectCaptureSession) record(call MessengerCall) {
	if s == nil {
		return
	}
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
	if h == nil || h.hub == nil || h.id == 0 {
		return
	}
	h.hub.disable(h.id)
}

func (h *injectCaptureHandle) captured() []MessengerCall {
	if h == nil || h.hub == nil || h.id == 0 {
		return nil
	}
	return h.hub.captured(h.id)
}

func (h *injectCaptureHandle) close() {
	if h == nil || h.hub == nil || h.id == 0 {
		return
	}
	h.hub.close(h.id)
}

// injectCaptureHub is a single long-lived messenger wrapper. Each inject call
// opens one capture session and closes it on return, avoiding wrapper stacking.
type injectCaptureHub struct {
	inner    LarkMessenger
	mu       sync.RWMutex
	nextID   uint64
	sessions map[uint64]*injectCaptureSession
}

func newInjectCaptureHub(inner LarkMessenger) *injectCaptureHub {
	return &injectCaptureHub{
		inner:    inner,
		sessions: map[uint64]*injectCaptureSession{},
	}
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

func (h *injectCaptureHub) SendMessage(ctx context.Context, chatID, msgType, content string) (string, error) {
	id, err := h.inner.SendMessage(ctx, chatID, msgType, content)
	h.recordByChat(chatID, MessengerCall{Method: "SendMessage", ChatID: chatID, MsgType: msgType, Content: content})
	return id, err
}

func (h *injectCaptureHub) ReplyMessage(ctx context.Context, replyToID, msgType, content string) (string, error) {
	id, err := h.inner.ReplyMessage(ctx, replyToID, msgType, content)
	h.recordAll(MessengerCall{Method: "ReplyMessage", ReplyTo: replyToID, MsgType: msgType, Content: content})
	return id, err
}

func (h *injectCaptureHub) UpdateMessage(ctx context.Context, messageID, msgType, content string) error {
	err := h.inner.UpdateMessage(ctx, messageID, msgType, content)
	h.recordAll(MessengerCall{Method: "UpdateMessage", MsgID: messageID, MsgType: msgType, Content: content})
	return err
}

func (h *injectCaptureHub) AddReaction(ctx context.Context, messageID, emojiType string) error {
	err := h.inner.AddReaction(ctx, messageID, emojiType)
	h.recordAll(MessengerCall{Method: "AddReaction", MsgID: messageID, Emoji: emojiType})
	return err
}

func (h *injectCaptureHub) UploadImage(ctx context.Context, payload []byte) (string, error) {
	return h.inner.UploadImage(ctx, payload)
}

func (h *injectCaptureHub) UploadFile(ctx context.Context, payload []byte, fileName, fileType string) (string, error) {
	return h.inner.UploadFile(ctx, payload, fileName, fileType)
}

func (h *injectCaptureHub) ListMessages(ctx context.Context, chatID string, pageSize int) ([]*larkim.Message, error) {
	return h.inner.ListMessages(ctx, chatID, pageSize)
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
