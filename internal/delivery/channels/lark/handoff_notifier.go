package lark

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/runtime/leader"
	"alex/internal/shared/logging"
)

// HandoffNotifier subscribes to EventHandoffRequired on the runtime bus
// and dispatches structured Lark messages with action guidance.
type HandoffNotifier struct {
	gateway    *Gateway
	bus        hooks.Bus
	defaultChatID string // fallback chat ID when session-to-chat resolution fails
	logger     logging.Logger
}

// NewHandoffNotifier creates a HandoffNotifier wired to the given gateway and bus.
func NewHandoffNotifier(gateway *Gateway, bus hooks.Bus, defaultChatID string) *HandoffNotifier {
	return &HandoffNotifier{
		gateway:       gateway,
		bus:           bus,
		defaultChatID: strings.TrimSpace(defaultChatID),
		logger:        logging.NewComponentLogger("HandoffNotifier"),
	}
}

// Run subscribes to the bus and dispatches handoff notifications.
// Blocks until ctx is cancelled.
func (n *HandoffNotifier) Run(ctx context.Context) {
	ch, cancel := n.bus.SubscribeAll()
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if ev.Type == hooks.EventHandoffRequired {
				n.handleHandoff(ctx, ev)
			}
		}
	}
}

func (n *HandoffNotifier) handleHandoff(ctx context.Context, ev hooks.Event) {
	hctx := leader.ParseHandoffContext(ev.Payload)
	// Override session_id from the event if not present in payload.
	if hctx.SessionID == "" {
		hctx.SessionID = ev.SessionID
	}
	card := FormatHandoffCard(hctx)

	chatID := n.resolveChatID(hctx.SessionID)
	if chatID == "" {
		n.logger.Warn("HandoffNotifier: no chat ID for session %s, dropping", hctx.SessionID)
		return
	}

	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	n.gateway.dispatch(sendCtx, chatID, "", "interactive", card)
}

// resolveChatID finds the Lark chat ID associated with a runtime session.
// It iterates activeSlots looking for a slot whose sessionID matches.
// Falls back to the default chat ID if no match is found.
func (n *HandoffNotifier) resolveChatID(sessionID string) string {
	if sessionID == "" {
		return n.defaultChatID
	}
	var found string
	n.gateway.activeSlots.Range(func(key, value any) bool {
		chatID, ok := key.(string)
		if !ok {
			return true
		}
		slot, ok := value.(*sessionSlot)
		if !ok || slot == nil {
			return true
		}
		slot.mu.Lock()
		match := slot.sessionID == sessionID
		slot.mu.Unlock()
		if match {
			found = chatID
			return false
		}
		return true
	})
	if found != "" {
		return found
	}
	return n.defaultChatID
}

// handoffActionRetry is the action value for the retry button.
const handoffActionRetry = "handoff_retry"

// handoffActionAbort is the action value for the abort button.
const handoffActionAbort = "handoff_abort"

// handoffActionProvideInput is the action value for the provide input button.
const handoffActionProvideInput = "handoff_provide_input"

// FormatHandoffCard builds a Lark interactive card JSON for a handoff notification.
// The card includes diagnostic fields and action buttons for retry/abort/provide_input.
func FormatHandoffCard(ctx leader.HandoffContext) string {
	headerColor := "yellow"
	if ctx.RecommendedAction == "abort" {
		headerColor = "red"
	}

	elements := buildHandoffCardElements(ctx)

	// Action buttons with session_id in the value for callback routing.
	actions := []any{
		handoffButton("🔄 重试", "primary", handoffActionRetry, ctx.SessionID),
		handoffButton("⛔ 终止", "danger", handoffActionAbort, ctx.SessionID),
		handoffButton("💬 提供输入", "default", handoffActionProvideInput, ctx.SessionID),
	}
	elements = append(elements, map[string]any{
		"tag":     "action",
		"actions": actions,
	})

	return buildLarkCard("⚠️ Leader Agent 需要你的帮助", headerColor, elements)
}

// buildHandoffCardElements constructs the markdown content elements for the card.
func buildHandoffCardElements(ctx leader.HandoffContext) []any {
	var b strings.Builder
	if ctx.Goal != "" {
		b.WriteString(fmt.Sprintf("**目标:** %s\n", ctx.Goal))
	}
	if ctx.Member != "" {
		b.WriteString(fmt.Sprintf("**成员:** %s\n", ctx.Member))
	}
	if ctx.Reason != "" {
		b.WriteString(fmt.Sprintf("**原因:** %s\n", ctx.Reason))
	}
	if ctx.StallCount > 0 {
		b.WriteString(fmt.Sprintf("**已尝试:** %d 次\n", ctx.StallCount))
	}
	if ctx.Elapsed != "" {
		b.WriteString(fmt.Sprintf("**运行时长:** %s\n", ctx.Elapsed))
	}
	if ctx.LastToolCall != "" {
		b.WriteString(fmt.Sprintf("**最后工具调用:** %s\n", ctx.LastToolCall))
	}
	if ctx.LastError != "" {
		b.WriteString(fmt.Sprintf("**最后错误:** %s\n", ctx.LastError))
	}
	if len(ctx.SessionTail) > 0 {
		b.WriteString("**最近消息:**\n")
		for _, msg := range ctx.SessionTail {
			b.WriteString(fmt.Sprintf("- %s\n", msg))
		}
	}

	elements := []any{
		map[string]any{"tag": "markdown", "content": b.String()},
	}
	return elements
}

// handoffButton builds a Lark card button element with an action value.
func handoffButton(text, btnType, action, sessionID string) map[string]any {
	return map[string]any{
		"tag":  "button",
		"text": map[string]any{"tag": "plain_text", "content": text},
		"type": btnType,
		"value": map[string]any{
			"action":     action,
			"session_id": sessionID,
		},
	}
}

// FormatHandoffMessage formats a HandoffContext into a plain-text message.
// Retained for fallback rendering when interactive cards are unavailable.
func FormatHandoffMessage(ctx leader.HandoffContext) string {
	var b strings.Builder
	b.WriteString("⚠️ Leader Agent 需要你的帮助\n\n")

	if ctx.Goal != "" {
		b.WriteString(fmt.Sprintf("目标: %s\n", ctx.Goal))
	}
	if ctx.Member != "" {
		b.WriteString(fmt.Sprintf("成员: %s\n", ctx.Member))
	}
	if ctx.Reason != "" {
		b.WriteString(fmt.Sprintf("原因: %s\n", ctx.Reason))
	}
	if ctx.StallCount > 0 {
		b.WriteString(fmt.Sprintf("已尝试: %d 次\n", ctx.StallCount))
	}
	if ctx.Elapsed != "" {
		b.WriteString(fmt.Sprintf("运行时长: %s\n", ctx.Elapsed))
	}
	if ctx.LastToolCall != "" {
		b.WriteString(fmt.Sprintf("最后工具调用: %s\n", ctx.LastToolCall))
	}
	if ctx.LastError != "" {
		b.WriteString(fmt.Sprintf("最后错误: %s\n", ctx.LastError))
	}
	if len(ctx.SessionTail) > 0 {
		b.WriteString("最近消息:\n")
		for _, msg := range ctx.SessionTail {
			b.WriteString(fmt.Sprintf("  - %s\n", msg))
		}
	}

	switch ctx.RecommendedAction {
	case "provide_input":
		b.WriteString("\n建议: 直接回复消息提供输入")
	case "retry":
		b.WriteString("\n建议: 可以重试（发送 /retry）")
	case "abort":
		b.WriteString("\n建议: 考虑终止任务（发送 /stop）")
	}

	return b.String()
}
