package lark

import (
	"context"
	"strings"
	"time"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
)

const larkHistoryChunkHeader = "[Lark History Chunk]\nIndexed summaries (latest first-pass context, max 50 chars per entry):"

// getOrCreateSlot returns the session slot for the given chat, creating one if needed.
func (g *Gateway) getOrCreateSlot(chatID string) *sessionSlot {
	slot, loaded := g.activeSlots.LoadOrStore(chatID, &sessionSlot{})
	s := slot.(*sessionSlot)
	if !loaded {
		s.mu.Lock()
		s.lastTouched = g.currentTime()
		s.mu.Unlock()
	}
	return s
}

// injectUserInput forwards a message into a running task's input channel.
func (g *Gateway) injectUserInput(ch chan agent.UserInput, activeSessionID string, msg *incomingMessage) {
	if msg == nil {
		return
	}
	select {
	case ch <- agent.UserInput{Content: msg.content, SenderID: msg.senderID, MessageID: msg.messageID}:
		g.logger.Info("Injected user input into active session %s", activeSessionID)
		if msg.messageID != "" {
			emojiType := strings.TrimSpace(g.cfg.InjectionAckReactEmoji)
			if emojiType == "" {
				emojiType = "THINKING"
			}
			go func() {
				ackCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", activeSessionID, msg.senderID, msg.chatID, msg.isGroup)
				ackCtx, cancel := context.WithTimeout(ackCtx, 2*time.Second)
				defer cancel()
				g.addReaction(ackCtx, msg.messageID, emojiType)
			}()
		}
	default:
		g.logger.Warn("User input channel full for session %s; message dropped", activeSessionID)
	}
}

// handleNewSessionCommand processes a /new message, creating a fresh session
// and rebinding this chat to it. The caller must hold slot.mu; this method
// releases it.
func (g *Gateway) handleNewSessionCommand(slot *sessionSlot, msg *incomingMessage) {
	newSessionID := g.newSessionID()
	oldSessionID := slot.sessionID
	cancel := slot.taskCancel
	wasRunning := slot.phase == slotRunning && cancel != nil
	if wasRunning {
		slot.intentionalCancelToken = slot.taskToken
	}
	slot.taskToken++
	slot.sessionID = newSessionID
	slot.lastSessionID = newSessionID
	slot.phase = slotIdle
	slot.inputCh = nil
	slot.taskCancel = nil
	slot.pendingOptions = nil
	slot.lastTouched = g.currentTime()
	slot.mu.Unlock()

	if wasRunning {
		cancel()
		g.logger.Info("Lark /new: cancelled running session %s and switched to %s", oldSessionID, newSessionID)
	}

	// Also stop conversation-process workers if in that mode.
	if g.conversationProcessEnabled() {
		if slotMap := g.getOrCreateSlotMap(msg.chatID); slotMap != nil {
			slotMap.stopAll(true)
		}
	}

	execCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", newSessionID, msg.senderID, msg.chatID, msg.isGroup)
	execCtx = g.withLarkContext(execCtx, msg.chatID, msg.messageID)
	g.persistChatSessionBinding(execCtx, msg.chatID, newSessionID)
	confirmation := "已开启新会话，后续消息将使用新的上下文。"
	if wasRunning {
		confirmation = "已停止当前调用并开启新会话，后续消息将使用新的上下文。"
	}
	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(confirmation))
}

// handleResetCommand processes a /reset message. The command is deprecated; it
// no longer clears history to avoid accidental loss of context.
// The caller must hold slot.mu; this method releases it.
func (g *Gateway) handleResetCommand(slot *sessionSlot, msg *incomingMessage) {
	sessionID := slot.sessionID
	if sessionID == "" {
		sessionID = slot.lastSessionID
	}
	if sessionID == "" {
		sessionID = g.memoryIDForChat(msg.chatID)
	}
	slot.lastTouched = g.currentTime()
	slot.mu.Unlock()

	execCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", sessionID, msg.senderID, msg.chatID, msg.isGroup)
	execCtx = g.withLarkContext(execCtx, msg.chatID, msg.messageID)
	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent("`/reset` 已弃用，请使用 `/new` 开启新的会话。"))
}

func (g *Gateway) isStopCommand(trimmed string) bool {
	lower := strings.ToLower(strings.TrimSpace(trimmed))
	return lower == "/stop" || strings.HasPrefix(lower, "/stop #")
}

// handleStopCommand processes /stop message. It cancels an in-flight foreground
// task for this chat when one exists.
// The caller must hold slot.mu; this method releases it.
func (g *Gateway) handleStopCommand(slot *sessionSlot, msg *incomingMessage) {
	sessionID := slot.sessionID
	if sessionID == "" {
		sessionID = slot.lastSessionID
	}
	if sessionID == "" {
		sessionID = g.memoryIDForChat(msg.chatID)
	}
	cancel := slot.taskCancel
	running := slot.phase == slotRunning && cancel != nil
	if running {
		slot.intentionalCancelToken = slot.taskToken
	}
	slot.lastTouched = g.currentTime()
	slot.mu.Unlock()

	execCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", sessionID, msg.senderID, msg.chatID, msg.isGroup)
	execCtx = g.withLarkContext(execCtx, msg.chatID, msg.messageID)

	// Also stop conversation-process workers if in that mode.
	if g.conversationProcessEnabled() {
		if slotMap := g.getOrCreateSlotMap(msg.chatID); slotMap != nil {
			trimmedContent := strings.TrimSpace(msg.content)
			fields := strings.Fields(trimmedContent)
			taskIDArg := ""
			if len(fields) >= 2 {
				taskIDArg = fields[1]
			}
			if taskIDArg != "" {
				slotMap.stopByTaskID(taskIDArg)
			} else {
				slotMap.stopAll(true)
			}
		}
	}

	if !running {
		g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent("当前没有正在执行的调用。"))
		return
	}

	cancel()
	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent("已停止当前调用。"))
}

// resolveSessionForNewTask decides whether to reuse the awaiting session or
// create a fresh one. Must be called while slot.mu is held.
func (g *Gateway) resolveSessionForNewTask(ctx context.Context, chatID string, slot *sessionSlot) (sessionID string, isResume bool) {
	if slot.phase == slotAwaitingInput && slot.sessionID != "" {
		g.logger.Info("Lark session routing: chat=%s source=awaiting_input session=%s", chatID, slot.sessionID)
		return slot.sessionID, true
	}
	// Reuse the last session to preserve conversation history across turns.
	if slot.lastSessionID != "" {
		g.logger.Info("Lark session routing: chat=%s source=last_session session=%s", chatID, slot.lastSessionID)
		return slot.lastSessionID, false
	}
	if persisted := g.loadPersistedChatSessionBinding(ctx, chatID); persisted != "" {
		g.logger.Info("Lark session routing: chat=%s source=persisted_binding session=%s", chatID, persisted)
		return persisted, false
	}
	fresh := g.newSessionID()
	g.logger.Info("Lark session routing: chat=%s source=new_session session=%s", chatID, fresh)
	return fresh, false
}

func (g *Gateway) loadPersistedChatSessionBinding(ctx context.Context, chatID string) string {
	if g.chatSessionStore == nil {
		return ""
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return ""
	}
	binding, ok, err := g.chatSessionStore.GetBinding(ctx, chatSessionBindingChannel, chatID)
	if err != nil {
		g.logger.Warn("Load chat session binding failed: chat=%s err=%v", chatID, err)
		return ""
	}
	if !ok {
		return ""
	}
	return strings.TrimSpace(binding.SessionID)
}

func (g *Gateway) persistChatSessionBinding(ctx context.Context, chatID, sessionID string) {
	if g.chatSessionStore == nil {
		return
	}
	chatID = strings.TrimSpace(chatID)
	sessionID = strings.TrimSpace(sessionID)
	if chatID == "" || sessionID == "" {
		return
	}
	storeCtx := context.WithoutCancel(ctx)
	err := g.chatSessionStore.SaveBinding(storeCtx, ChatSessionBinding{
		Channel:   chatSessionBindingChannel,
		ChatID:    chatID,
		SessionID: sessionID,
		UpdatedAt: g.currentTime(),
	})
	if err != nil {
		g.logger.Warn("Persist chat session binding failed: chat=%s session=%s err=%v", chatID, sessionID, err)
	}
}
