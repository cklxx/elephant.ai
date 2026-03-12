package lark

import (
	"context"
	"time"

	"alex/internal/runtime/hooks"
)

// HandleHandoffAction processes a card button click from a handoff notification.
// The action and sessionID are extracted from the card button's value field.
func (g *Gateway) HandleHandoffAction(ctx context.Context, chatID, action, sessionID string) {
	switch action {
	case handoffActionRetry:
		g.handleHandoffRetry(ctx, chatID, sessionID)
	case handoffActionAbort:
		g.handleHandoffAbort(ctx, chatID, sessionID)
	case handoffActionProvideInput:
		g.handleHandoffProvideInput(chatID)
	}
}

// handleHandoffRetry publishes an EventStalled to trigger the leader agent
// to retry handling the session.
func (g *Gateway) handleHandoffRetry(ctx context.Context, chatID, sessionID string) {
	if bus := g.handoffBus(); bus != nil {
		bus.Publish(sessionID, hooks.Event{
			Type:      hooks.EventStalled,
			SessionID: sessionID,
			At:        time.Now(),
		})
	}
	g.dispatch(ctx, chatID, "", "text", textContent("已触发重试。"))
}

// handleHandoffAbort cancels the running task in the slot associated with
// the given chat, similar to the /stop command.
func (g *Gateway) handleHandoffAbort(ctx context.Context, chatID, sessionID string) {
	raw, ok := g.activeSlots.Load(chatID)
	if !ok {
		g.dispatch(ctx, chatID, "", "text", textContent("未找到活跃会话。"))
		return
	}
	slot, ok := raw.(*sessionSlot)
	if !ok || slot == nil {
		g.dispatch(ctx, chatID, "", "text", textContent("未找到活跃会话。"))
		return
	}

	slot.mu.Lock()
	cancel := slot.taskCancel
	running := slot.phase == slotRunning && cancel != nil
	if running {
		slot.intentionalCancelToken = slot.taskToken
	}
	slot.mu.Unlock()

	if !running {
		g.dispatch(ctx, chatID, "", "text", textContent("当前没有正在执行的任务。"))
		return
	}
	cancel()
	g.dispatch(ctx, chatID, "", "text", textContent("已终止任务。"))
}

// handleHandoffProvideInput switches the slot to awaitingInput so the next
// user message in this chat is routed as input to the session.
func (g *Gateway) handleHandoffProvideInput(chatID string) {
	raw, ok := g.activeSlots.Load(chatID)
	if !ok {
		return
	}
	slot, ok := raw.(*sessionSlot)
	if !ok || slot == nil {
		return
	}
	slot.mu.Lock()
	slot.phase = slotAwaitingInput
	slot.mu.Unlock()
}

// handoffBus returns the hooks.Bus used by the HandoffNotifier, if available.
// The bus is stored on the gateway when a HandoffNotifier is wired.
func (g *Gateway) handoffBus() hooks.Bus {
	if g == nil {
		return nil
	}
	return g.runtimeBus
}
