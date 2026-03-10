package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/utils"
)

// InjectSyncRequest is the input for InjectMessageSync.
type InjectSyncRequest struct {
	ChatID             string        `json:"chat_id"`
	ChatType           string        `json:"chat_type"` // default "p2p"
	SenderID           string        `json:"sender_id"` // default "ou_inject_user"
	Text               string        `json:"text"`
	ToolMessageRounds  int           `json:"tool_message_rounds,omitempty"` // heuristic: force N tool-driven progress messages before final answer
	Timeout            time.Duration `json:"timeout"`                       // default 5min
	AutoReply          bool          `json:"auto_reply"`                    // enable auto-reply on await_user_input
	MaxAutoReplyRounds int           `json:"max_auto_reply_rounds"`         // default 3
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
	maxInjectHistoryPerChat   = 400
	injectBotSenderID         = "cli_inject_bot"
)

// InjectMessageSync injects a message and blocks until the task completes,
// capturing all outbound messenger calls for the target chat.
// When AutoReply is enabled, the method automatically generates replies to
// agent clarification questions, resuming the slot up to MaxAutoReplyRounds.
func (g *Gateway) InjectMessageSync(ctx context.Context, req InjectSyncRequest) *InjectSyncResponse {
	start := g.currentTime()
	originalText := req.Text

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
	taskText := buildInjectToolMessageTask(originalText, req.ToolMessageRounds)

	// Inject the message through the normal pipeline.
	if err := g.InjectMessage(ctx, req.ChatID, req.ChatType, req.SenderID, messageID, taskText); err != nil {
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
		replyText := g.generateAutoReply(ctx, originalText, question, options)
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

func buildInjectToolMessageTask(task string, rounds int) string {
	task = strings.TrimSpace(task)
	if task == "" {
		return ""
	}
	if rounds <= 0 {
		return task
	}
	var builder strings.Builder
	builder.WriteString("[Inject Tool Heuristic]\n")
	builder.WriteString("This is an inject-chain verification run.\n")
	builder.WriteString(fmt.Sprintf("Before the final answer, send exactly %d progress updates to the current Lark chat via tools.\n", rounds))
	builder.WriteString(fmt.Sprintf("Each progress update must start with \"[过程 i/%d]\" where i is 1..%d.\n", rounds, rounds))
	builder.WriteString("Use shell_exec and call python3 skills/feishu-cli/run.py for progress messages.\n")
	builder.WriteString("Only use this CLI skill path for progress updates.\n")
	builder.WriteString("Avoid request_user, plan, clarify, run_tasks, or any unrelated tools.\n")
	builder.WriteString("If a send tool call fails, continue to the next i without retry loops.\n")
	builder.WriteString("Do not ask for confirmation during these progress updates.\n\n")
	builder.WriteString("User task:\n")
	builder.WriteString(task)
	return builder.String()
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
	if phase == slotRunning && cancel != nil {
		slot.intentionalCancelToken = slot.taskToken
	}
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

// resolveAutoReplyProfile returns the pinned subscription profile from context
// if available, otherwise falls back to the gateway's shared runtime profile.
func (g *Gateway) resolveAutoReplyProfile(ctx context.Context) runtimeconfig.LLMProfile {
	if ctx != nil {
		if selection, ok := appcontext.GetLLMSelection(ctx); ok {
			if utils.HasContent(selection.Provider) && utils.HasContent(selection.Model) {
				return runtimeconfig.LLMProfile{
					Provider: selection.Provider,
					Model:    selection.Model,
					APIKey:   selection.APIKey,
					BaseURL:  selection.BaseURL,
					Headers:  selection.Headers,
				}
			}
		}
	}
	return g.llmProfile
}

// llmAutoReply calls a lightweight LLM to generate a context-aware reply.
// It prefers the pinned subscription from context, falling back to the gateway profile.
func (g *Gateway) llmAutoReply(ctx context.Context, originalText, question string, options []string) (string, error) {
	profile := g.resolveAutoReplyProfile(ctx)
	if utils.IsBlank(profile.Provider) || utils.IsBlank(profile.Model) {
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
		if calls[i].Method == MethodAddReaction || calls[i].Method == MethodDeleteReaction {
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
