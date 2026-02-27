package telegram

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"

	"github.com/mymmrac/telego"
)

// incomingMessage is the parsed representation of a Telegram update.
type incomingMessage struct {
	chatID    int64
	messageID int
	senderID  int64
	username  string
	firstName string
	content   string
	isGroup   bool
}

// handleUpdate dispatches a Telegram update to the appropriate handler.
func (g *Gateway) handleUpdate(ctx context.Context, update telego.Update) {
	switch {
	case update.Message != nil:
		g.handleMessage(ctx, update.Message)
	case update.EditedMessage != nil:
		// Ignore edits for now — treat them the same as new messages would
		// cause confusion with dedup and slot state.
	case update.CallbackQuery != nil:
		g.handleCallbackQuery(ctx, update.CallbackQuery)
	}
}

// handleMessage processes an incoming Telegram message.
func (g *Gateway) handleMessage(ctx context.Context, msg *telego.Message) {
	incoming := g.parseIncomingMessage(msg)
	if incoming == nil {
		return
	}

	// Dedup.
	if g.isDuplicate(incoming.messageID) {
		return
	}

	// Chat type filtering.
	if incoming.isGroup && !g.cfg.AllowGroups {
		return
	}
	if !incoming.isGroup && !g.cfg.AllowDirect {
		return
	}
	if incoming.isGroup && !g.isGroupAllowed(incoming.chatID) {
		return
	}

	// Empty content — nothing to do.
	content := strings.TrimSpace(incoming.content)
	if content == "" {
		return
	}

	// Slot management.
	slot := g.getOrCreateSlot(incoming.chatID)
	slot.mu.Lock()

	// Command routing (slot locked).
	lower := strings.ToLower(content)
	switch {
	case lower == "/new" || lower == "/reset":
		slot.sessionID = ""
		if slot.phase == slotRunning && slot.taskCancel != nil {
			slot.taskCancel()
		}
		slot.phase = slotIdle
		slot.mu.Unlock()
		g.sendReply(ctx, incoming.chatID, incoming.messageID, "会话已重置。")
		return

	case lower == "/stop":
		if slot.phase == slotRunning && slot.taskCancel != nil {
			slot.taskCancel()
			slot.mu.Unlock()
			g.sendReply(ctx, incoming.chatID, incoming.messageID, "正在停止当前任务...")
		} else {
			slot.mu.Unlock()
			g.sendReply(ctx, incoming.chatID, incoming.messageID, "当前没有运行中的任务。")
		}
		return

	case lower == "/status":
		phase := slot.phase
		sid := slot.sessionID
		slot.mu.Unlock()
		status := "空闲"
		switch phase {
		case slotRunning:
			status = "执行中"
		case slotAwaitingInput:
			status = "等待输入"
		}
		g.sendReply(ctx, incoming.chatID, incoming.messageID, fmt.Sprintf("状态: %s\n会话: %s", status, sid))
		return
	}

	// Input injection: if a task is running, feed the message into its input channel.
	if slot.phase == slotRunning && slot.inputCh != nil {
		inputCh := slot.inputCh
		slot.mu.Unlock()
		select {
		case inputCh <- agent.UserInput{Content: content}:
			g.logger.Info("Telegram: injected input into running task for chat %d", incoming.chatID)
		default:
			g.logger.Warn("Telegram: input channel full for chat %d, dropping message", incoming.chatID)
		}
		return
	}

	// Awaiting input → resume session.
	isResume := false
	sessionID := slot.sessionID
	if slot.phase == slotAwaitingInput && sessionID != "" {
		isResume = true
	}
	if sessionID == "" {
		sessionID = g.newSessionID()
	}

	// Transition to running.
	inputCh := make(chan agent.UserInput, 8)
	token := g.nextToken()
	slot.phase = slotRunning
	slot.inputCh = inputCh
	slot.taskCancel = nil
	slot.taskToken = token
	slot.sessionID = sessionID
	slot.mu.Unlock()

	// Spawn task goroutine.
	g.taskWG.Add(1)
	go func() {
		defer g.taskWG.Done()
		g.runTask(ctx, incoming, sessionID, inputCh, isResume, token)
	}()
}

// runTask executes an agent task for a Telegram message.
func (g *Gateway) runTask(ctx context.Context, msg *incomingMessage, sessionID string, inputCh chan agent.UserInput, isResume bool, taskToken uint64) {
	slot := g.getSlot(msg.chatID)
	if slot == nil {
		return
	}

	// Build execution context using shared channel helpers.
	execCtx := channels.BuildBaseContext(
		g.cfg.BaseConfig,
		"telegram",
		sessionID,
		chatIDStr(msg.senderID),
		chatIDStr(msg.chatID),
		msg.isGroup,
	)
	execCtx = channels.ApplyPresets(execCtx, g.cfg.BaseConfig)
	execCtx, cancel := channels.ApplyTimeout(execCtx, g.cfg.BaseConfig)
	defer cancel()

	// Attach task cancel to slot.
	taskCtx, taskCancel := context.WithCancel(execCtx)
	defer taskCancel()
	slot.mu.Lock()
	slot.taskCancel = taskCancel
	slot.mu.Unlock()

	// Ensure session exists.
	if _, err := g.agent.EnsureSession(taskCtx, sessionID); err != nil {
		g.logger.Warn("Telegram: EnsureSession failed: %v", err)
		g.sendReply(ctx, msg.chatID, msg.messageID, fmt.Sprintf("会话初始化失败: %v", err))
		g.resetSlotToIdle(slot, taskToken)
		return
	}

	// Build event listener chain.
	var listener agent.EventListener = g.eventListener

	// Progress listener (tool progress updates via message editing).
	var progressLis *progressListener
	if g.cfg.ShowToolProgress {
		sender := &telegramProgressSender{
			gateway: g,
			chatID:  msg.chatID,
		}
		progressLis = newProgressListener(taskCtx, listener, sender, g.logger)
		listener = progressLis
	}

	// Execute task.
	content := strings.TrimSpace(msg.content)
	result, execErr := g.agent.ExecuteTask(taskCtx, content, sessionID, listener)

	// Close progress listener.
	if progressLis != nil {
		progressLis.Close()
	}

	// Build and send reply.
	reply := channels.BuildReplyCore(g.cfg.BaseConfig, result, execErr)
	if reply != "" {
		g.sendReply(ctx, msg.chatID, msg.messageID, reply)
	}

	// Check for await_user_input stop reason.
	isAwaiting := result != nil && result.StopReason == "await_user_input"
	slot.mu.Lock()
	if slot.taskToken != taskToken {
		slot.mu.Unlock()
		return
	}
	if isAwaiting {
		slot.phase = slotAwaitingInput
		slot.inputCh = nil
		slot.taskCancel = nil
	} else {
		slot.phase = slotIdle
		slot.inputCh = nil
		slot.taskCancel = nil
	}
	slot.lastTouched = g.clock()
	slot.mu.Unlock()
}

// resetSlotToIdle transitions a slot back to idle if the token matches.
func (g *Gateway) resetSlotToIdle(slot *sessionSlot, token uint64) {
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.taskToken != token {
		return
	}
	slot.phase = slotIdle
	slot.inputCh = nil
	slot.taskCancel = nil
	slot.lastTouched = g.clock()
}

// handleCallbackQuery processes inline keyboard button presses (for plan review).
func (g *Gateway) handleCallbackQuery(ctx context.Context, cq *telego.CallbackQuery) {
	if cq == nil || cq.Data == "" {
		return
	}

	// Plan review callback handling — delegated to plan_review.go.
	g.handlePlanReviewCallback(ctx, cq)
}

// sendReply sends a text reply to a chat.
func (g *Gateway) sendReply(ctx context.Context, chatID int64, replyToMsgID int, text string) {
	if g.messenger == nil || text == "" {
		return
	}
	if _, err := g.messenger.SendText(ctx, chatID, text, replyToMsgID); err != nil {
		g.logger.Warn("Telegram: send reply failed: %v", err)
	}
}

// parseIncomingMessage extracts a normalized message from a Telegram message.
func (g *Gateway) parseIncomingMessage(msg *telego.Message) *incomingMessage {
	if msg == nil {
		return nil
	}

	// Extract text content.
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		text = strings.TrimSpace(msg.Caption)
	}
	if text == "" {
		return nil
	}

	chatType := msg.Chat.Type
	isGroup := chatType == telego.ChatTypeGroup || chatType == telego.ChatTypeSupergroup

	// Strip bot mention from group messages (e.g. "/new@mybot" → "/new").
	if isGroup {
		text = stripBotMention(text)
	}

	var senderID int64
	var username, firstName string
	if msg.From != nil {
		senderID = msg.From.ID
		username = msg.From.Username
		firstName = msg.From.FirstName
	}

	return &incomingMessage{
		chatID:    msg.Chat.ID,
		messageID: msg.MessageID,
		senderID:  senderID,
		username:  username,
		firstName: firstName,
		content:   text,
		isGroup:   isGroup,
	}
}

// stripBotMention removes @botname suffix from commands.
func stripBotMention(text string) string {
	if !strings.HasPrefix(text, "/") {
		return text
	}
	parts := strings.SplitN(text, " ", 2)
	cmd := parts[0]
	if idx := strings.Index(cmd, "@"); idx > 0 {
		cmd = cmd[:idx]
	}
	if len(parts) > 1 {
		return cmd + " " + parts[1]
	}
	return cmd
}

