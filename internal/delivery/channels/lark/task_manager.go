package lark

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/workdir"
	"alex/internal/delivery/channels"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	builtinshared "alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

const larkHistoryChunkHeader = "[Lark History Chunk]\nIndexed summaries (latest first-pass context, max 50 chars per entry):"

// getOrCreateSlot returns the session slot for the given chat, creating one if needed.
func (g *Gateway) getOrCreateSlot(chatID string) *sessionSlot {
	slot, _ := g.activeSlots.LoadOrStore(chatID, &sessionSlot{})
	s := slot.(*sessionSlot)
	s.mu.Lock()
	s.lastTouched = g.currentTime()
	s.mu.Unlock()
	return s
}

// storePendingRelay adds a pending input relay to the per-chat queue.
func (g *Gateway) storePendingRelay(chatID string, relay *pendingInputRelay) {
	if relay == nil {
		return
	}
	now := g.currentTime()
	if g.cfg.PendingInputRelayTTL > 0 {
		relay.expiresAt = now.Add(g.cfg.PendingInputRelayTTL).UnixNano()
	}
	raw, _ := g.pendingInputRelays.LoadOrStore(chatID, &pendingRelayQueue{})
	queue := raw.(*pendingRelayQueue)
	queue.Push(relay)
	queue.PruneExpired(now)
	if g.cfg.PendingInputRelayMaxPerChat > 0 {
		queue.TrimToMax(g.cfg.PendingInputRelayMaxPerChat)
	}
	if g.cfg.PendingInputRelayMaxChats > 0 {
		g.prunePendingInputRelays(now)
	}
}

// tryResolveInputReply checks whether a pending input relay exists for the chat
// and, if so, resolves the oldest one (FIFO) with the user's reply. Returns true
// when the message was consumed as an input reply.
func (g *Gateway) tryResolveInputReply(ctx context.Context, chatID, content string) bool {
	raw, ok := g.pendingInputRelays.Load(chatID)
	if !ok {
		return false
	}
	queue := raw.(*pendingRelayQueue)
	relay := queue.PopOldestUnexpired(g.currentTime())
	if relay == nil {
		g.pendingInputRelays.Delete(chatID)
		return false
	}
	if queue.Len() == 0 {
		g.pendingInputRelays.Delete(chatID)
	}

	resp := buildInputResponse(relay, content)

	if responder, ok := g.agent.(agent.ExternalInputResponder); ok {
		if err := responder.ReplyExternalInput(ctx, resp); err != nil {
			g.logger.Warn("External input reply failed: %v", err)
			return false
		}
		return true
	}

	g.logger.Warn("Agent does not support ExternalInputResponder interface")
	return false
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
	return strings.EqualFold(strings.TrimSpace(trimmed), "/stop")
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
	if g == nil || g.chatSessionStore == nil {
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
	if g == nil || g.chatSessionStore == nil {
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
		UpdatedAt: time.Now(),
	})
	if err != nil {
		g.logger.Warn("Persist chat session binding failed: chat=%s session=%s err=%v", chatID, sessionID, err)
	}
}

// runTask executes a full task lifecycle: context setup, session ensure,
// listener wiring, content preparation, execution, and reply dispatch.
// isResume indicates this task resumes from a prior await_user_input stop.
// Returns true if the result indicates await_user_input.
func (g *Gateway) runTask(taskCtx context.Context, msg *incomingMessage, sessionID string, inputCh chan agent.UserInput, isResume bool, taskToken uint64) bool {
	execCtx, cancelExec := g.buildExecContext(taskCtx, msg, sessionID, inputCh)
	defer cancelExec()

	// Inject CompletionNotifier so BackgroundTaskManager writes TaskStore directly.
	execCtx = agent.WithCompletionNotifier(execCtx, g)

	session, err := g.agent.EnsureSession(execCtx, sessionID)
	if err != nil {
		g.logger.Warn("Lark ensure session failed: %v", err)
		reply := g.buildReply(execCtx, nil, fmt.Errorf("ensure session: %w", err))
		if reply == "" {
			reply = "会话初始化失败，请稍后重试，或回复“诊断”让我输出可定位信息。"
		}
		g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(reply))
		return false
	}
	if session != nil && session.ID != "" && session.ID != sessionID {
		sessionID = session.ID
		execCtx = id.WithSessionID(execCtx, sessionID)
	}
	g.persistChatSessionBinding(execCtx, msg.chatID, sessionID)

	// Reconcile in-memory isResume with persisted session metadata.
	// This handles the cold-start case where the gateway restarted while
	// a session was awaiting input — the slot is slotIdle but the session
	// metadata still records the await flag.
	if !isResume && sessionHasAwaitFlag(session) {
		isResume = true
	}

	execCtx = channels.ApplyPresets(execCtx, g.cfg.BaseConfig)
	execCtx, cancelTimeout := channels.ApplyTimeout(execCtx, g.cfg.BaseConfig)
	defer cancelTimeout()

	awaitTracker := &awaitQuestionTracker{}
	listener, cleanupListeners, progressLn := g.setupListeners(execCtx, msg, awaitTracker)
	defer cleanupListeners()
	guardListener, guardState := newToolFailureGuardListener(listener, g.cfg.ToolFailureAbortThreshold, cancelExec)
	listener = guardListener
	execCtx = builtinshared.WithParentListener(execCtx, listener)

	// Resolve task content from three distinct concerns:
	// 1. Plan review feedback (if any pending plan review exists)
	taskContent, hasPlanReview := g.resolvePlanReviewFeedback(execCtx, session, msg)

	// 2. Await resume: seed user reply into inputCh; task content becomes empty
	//    so the ReAct loop reads input from the channel instead.
	if isResume && !hasPlanReview {
		g.seedAwaitResumeInput(inputCh, msg, sessionID)
		taskContent = ""
	}

	// 3. Chat context enrichment from IM recent rounds (only when there is content)
	taskContent = g.enrichWithChatContext(execCtx, taskContent, msg, hasPlanReview)

	// Add processing reaction to indicate the task is in progress.
	processingReactionID := g.addProcessingReaction(execCtx, msg.messageID)

	_, endEmoji := g.pickReactionEmojis()

	result, execErr := g.agent.ExecuteTask(execCtx, taskContent, sessionID, listener)

	// Remove the processing reaction now that the task has completed.
	g.removeProcessingReaction(execCtx, msg.messageID, processingReactionID)

	if msg.messageID != "" && endEmoji != "" {
		// Use context.WithoutCancel so the end-emoji reaction is not cancelled
		// when execCtx is torn down by the deferred cancelExec() in runTask.
		go g.addReaction(context.WithoutCancel(execCtx), msg.messageID, endEmoji)
	}

	// Close the progress listener before reading MessageID to ensure
	// no timer-fired flushes can race with the edit-in-place operation.
	// Close is idempotent; the deferred cleanupListeners will no-op.
	var progressMsgID string
	if progressLn != nil {
		progressLn.Close()
		progressMsgID = progressLn.MessageID()
	}

	g.dispatchResult(execCtx, msg, result, execErr, awaitTracker, progressMsgID, taskToken, guardState)

	// Notify AI chat coordinator that this bot's turn is complete
	if g.aiCoordinator != nil && msg.aiChatSessionActive {
		if nextBotID, shouldContinue := g.aiCoordinator.AdvanceTurn(msg.chatID, g.cfg.AppID); shouldContinue {
			g.logger.Info("AI chat: advancing to next bot %s in chat %s", nextBotID, msg.chatID)
			// Optionally trigger the next bot here if needed
		} else {
			g.logger.Info("AI chat: session ended for chat %s", msg.chatID)
		}
	}

	return execErr == nil && isResultAwaitingInput(result)
}

// buildExecContext constructs the fully-configured execution context for a task.
// taskCtx is an optional cancellation source used to abort long-running tasks.
func (g *Gateway) buildExecContext(taskCtx context.Context, msg *incomingMessage, sessionID string, inputCh chan agent.UserInput) (context.Context, context.CancelFunc) {
	execCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", sessionID, msg.senderID, msg.chatID, msg.isGroup)
	execCtx = g.withLarkContext(execCtx, msg.chatID, msg.messageID)
	if calendarID := strings.TrimSpace(g.cfg.TenantCalendarID); calendarID != "" {
		execCtx = builtinshared.WithLarkTenantCalendarID(execCtx, calendarID)
	}
	if g.oauth != nil {
		execCtx = builtinshared.WithLarkOAuth(execCtx, g.oauth)
	}
	execCtx = appcontext.WithPlanReviewEnabled(execCtx, g.cfg.PlanReviewEnabled)
	execCtx = g.applyPlanModeToContext(execCtx, msg)
	execCtx = agent.WithUserInputCh(execCtx, inputCh)

	workspaceDir := strings.TrimSpace(g.cfg.WorkspaceDir)
	if workspaceDir == "" {
		workspaceDir = workdir.DefaultWorkingDir()
	}
	if workspaceDir != "" {
		execCtx = workdir.WithWorkingDir(execCtx, workspaceDir)
	}

	autoUploadMaxBytes := g.cfg.AutoUploadMaxBytes
	if autoUploadMaxBytes <= 0 {
		autoUploadMaxBytes = 2 * 1024 * 1024
	}
	execCtx = builtinshared.WithAutoUploadConfig(execCtx, builtinshared.AutoUploadConfig{
		Enabled:   g.cfg.AutoUploadFiles,
		MaxBytes:  autoUploadMaxBytes,
		AllowExts: normalizeExtensions(g.cfg.AutoUploadAllowExt),
	})

	execCtx = g.applyPinnedLarkLLMSelection(execCtx, msg)

	execCtx, cancel := withCancellationForward(execCtx, taskCtx)
	return execCtx, cancel
}

func withCancellationForward(baseCtx, upstream context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(baseCtx)
	if upstream == nil {
		return ctx, cancel
	}
	done := upstream.Done()
	if done == nil {
		return ctx, cancel
	}
	go func() {
		select {
		case <-done:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// sessionHasAwaitFlag checks whether a session's metadata indicates a pending
// await_user_input state. Used as a cold-start fallback when the in-memory
// slot phase has not been restored after a gateway restart.
func sessionHasAwaitFlag(session *storage.Session) bool {
	if session == nil || session.Metadata == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(session.Metadata["await_user_input"]), "true")
}

// setupListeners configures the event listener chain (progress, plan clarify)
// and returns the composed listener, a cleanup function, and the progress
// listener (nil when progress is disabled). The caller uses the progress
// listener to retrieve the message ID for editing the progress message
// into the final reply.
func (g *Gateway) setupListeners(execCtx context.Context, msg *incomingMessage, awaitTracker *awaitQuestionTracker) (agent.EventListener, func(), *progressListener) {
	listener := g.eventListener
	if listener == nil {
		listener = agent.NoopEventListener{}
	}

	var cleanups []func()
	var progressLn *progressListener

	if g.cfg.ShowToolProgress {
		sender := &larkProgressSender{gateway: g, chatID: msg.chatID, messageID: msg.messageID, isGroup: msg.isGroup}
		progressLn = newProgressListener(execCtx, listener, sender, g.logger)
		cleanups = append(cleanups, progressLn.Close)
		listener = progressLn
	}
	backgroundEnabled := true
	if g.cfg.BackgroundProgressEnabled != nil {
		backgroundEnabled = *g.cfg.BackgroundProgressEnabled
	}
	if backgroundEnabled {
		replyTo := replyTarget(msg.messageID, msg.isGroup)
		bgLn := newBackgroundProgressListener(execCtx, listener, g, msg.chatID, replyTo, g.logger, g.cfg.BackgroundProgressInterval, g.cfg.BackgroundProgressWindow)
		// Release keeps the listener alive for tracked background tasks so
		// completion notifications can still be delivered after foreground return.
		cleanups = append(cleanups, bgLn.Release)
		listener = bgLn
	}
	// Input request listener bridges external agent permission/input requests to Lark.
	{
		irLn := newInputRequestListener(execCtx, listener, g, msg.chatID, replyTarget(msg.messageID, true), g.logger)
		cleanups = append(cleanups, irLn.Close)
		listener = irLn
	}
	if g.cfg.ShowPlanClarifyMessages {
		listener = newPlanClarifyListener(execCtx, listener, g, msg.chatID, replyTarget(msg.messageID, true), awaitTracker)
	}

	listener = newPreanalysisEmojiReactionListener(execCtx, listener, g, msg.messageID)

	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	return listener, cleanup, progressLn
}

// resolvePlanReviewFeedback checks for a pending plan review and, if found,
// wraps the user's reply into a plan feedback block. Returns the task content
// and whether a pending plan review was found.
func (g *Gateway) resolvePlanReviewFeedback(execCtx context.Context, session *storage.Session, msg *incomingMessage) (string, bool) {
	if !g.cfg.PlanReviewEnabled {
		return msg.content, false
	}
	pending, ok := g.loadPlanReviewPending(execCtx, session, msg.senderID, msg.chatID)
	if !ok {
		return msg.content, false
	}
	taskContent := buildPlanFeedbackBlock(pending, msg.content)
	if g.planReviewStore != nil {
		if err := g.planReviewStore.ClearPending(execCtx, msg.senderID, msg.chatID); err != nil {
			g.logger.Warn("Lark plan review pending clear failed: %v", err)
		}
	}
	return taskContent, true
}

// seedAwaitResumeInput seeds the user's reply into the input channel for an
// await-resume handoff, resolving numbered replies if pending options exist.
// Must be called while the task goroutine holds the slot in slotRunning phase.
func (g *Gateway) seedAwaitResumeInput(inputCh chan agent.UserInput, msg *incomingMessage, sessionID string) {
	resolvedContent := msg.content
	slot := g.getOrCreateSlot(msg.chatID)
	slot.mu.Lock()
	if options := slot.pendingOptions; len(options) > 0 {
		resolvedContent = parseNumberedReply(msg.content, options)
		slot.pendingOptions = nil
	}
	slot.mu.Unlock()
	select {
	case inputCh <- agent.UserInput{Content: resolvedContent, SenderID: msg.senderID, MessageID: msg.messageID}:
		g.logger.Info("Seeded pending user input for session %s", sessionID)
	default:
		g.logger.Warn("Pending user input channel full for session %s; message dropped", sessionID)
	}
}

// enrichWithChatContext prepends (or appends) recent IM chat rounds as context
// to the task content.
func (g *Gateway) enrichWithChatContext(execCtx context.Context, taskContent string, msg *incomingMessage, hasPlanReview bool) string {
	if taskContent == "" || g.messenger == nil {
		return taskContent
	}
	pageSize := g.cfg.AutoChatContextSize
	if pageSize <= 0 {
		pageSize = 50
	}
	chatHistory, err := g.fetchRecentChatRounds(execCtx, msg.chatID, msg.messageID, pageSize, defaultRecentChatMaxRounds)
	if err != nil {
		g.logger.Warn("Lark auto chat context fetch failed: %v", err)
		return taskContent
	}
	if chatHistory == "" {
		return taskContent
	}
	historyChunk := larkHistoryChunkHeader + "\n" + chatHistory
	if hasPlanReview {
		return taskContent + "\n\n" + historyChunk
	}
	return historyChunk + "\n\n" + taskContent
}

// dispatchResult builds the reply from the execution result and sends it to
// the Lark chat, including any attachments. When progressMsgID is non-empty,
// the progress message is edited in-place to become the final reply, avoiding
// message fragmentation.
func (g *Gateway) dispatchResult(execCtx context.Context, msg *incomingMessage, result *agent.TaskResult, execErr error, awaitTracker *awaitQuestionTracker, progressMsgID string, taskToken uint64, guardState *toolFailureGuardState) {
	if errors.Is(execErr, context.Canceled) && g.isIntentionalTaskCancellation(msg.chatID, taskToken) {
		g.logger.Info("Lark task cancelled intentionally: chat=%s msg=%s token=%d", msg.chatID, msg.messageID, taskToken)
		return
	}
	if guardState != nil && guardState.Tripped() {
		dispatchCtx, cancel := detachedContext(execCtx, 15*time.Second)
		defer cancel()
		g.dispatch(dispatchCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(guardState.UserNotice()))
		return
	}

	isAwait := execErr == nil && isResultAwaitingInput(result)
	awaitPrompt, hasAwaitPrompt := agent.AwaitUserInputPrompt{}, false
	if isAwait && result != nil {
		awaitPrompt, hasAwaitPrompt = agent.ExtractAwaitUserInputPrompt(result.Messages)
	}

	reply := ""
	replyContent := ""
	replyMsgType := "text"
	attachmentSummary := ""

	if isAwait && g.cfg.PlanReviewEnabled {
		reply, _, replyContent = g.buildPlanReviewReplyContent(execCtx, msg, result)
	}

	skipReply := isAwait && awaitTracker.Sent()

	if replyContent == "" && !skipReply {
		var textOnlyAttachments map[string]ports.Attachment
		if result != nil && len(result.Attachments) > 0 {
			cfg := builtinshared.GetAutoUploadConfig(execCtx)
			_, textOnlyAttachments = partitionUploadableAttachments(
				result.Attachments, normalizeExtensions(cfg.AllowExts),
			)
		}
		attachmentSummary = buildAttachmentSummary(textOnlyAttachments)
		if reply == "" && isAwait {
			if hasAwaitPrompt && len(awaitPrompt.Options) > 0 {
				reply = formatNumberedOptions(awaitPrompt.Question, awaitPrompt.Options)
				// Store pending options so numeric replies can be resolved.
				slot := g.getOrCreateSlot(msg.chatID)
				slot.mu.Lock()
				slot.pendingOptions = awaitPrompt.Options
				slot.mu.Unlock()
			} else if hasAwaitPrompt {
				reply = awaitPrompt.Question
			} else {
				reply = "需要你补充信息后继续。"
			}
		}
		if reply == "" {
			reply = g.buildReply(execCtx, result, execErr)
		}
		if reply == "" {
			switch {
			case attachmentSummary != "":
				reply = attachmentSummary
				attachmentSummary = ""
			case execErr != nil:
				reply = "执行失败：" + sanitizeErrorForUser(execErr.Error())
			case isAwait:
				reply = "还需要你补充信息后继续。请直接回复你的补充内容。"
			default:
				reply = "这次没有生成可展示的文本结果。请告诉我你希望我输出：总结、下一步计划，或重试后的关键过程。"
			}
		}
		if attachmentSummary != "" {
			reply += "\n\n" + attachmentSummary
		}
		replyMsgType, replyContent = smartContent(reply)
	}

	if !skipReply {
		// Try multi-message split for conversational delivery.
		if g.cfg.MessageSplit != nil && g.cfg.MessageSplit.Enabled && replyMsgType == "text" {
			chunks := splitMessage(reply, *g.cfg.MessageSplit)
			if len(chunks) > 1 {
				g.dispatchMultiMessageReply(execCtx, msg, result, execErr, progressMsgID, chunks)
				return
			}
		}
		intent := g.buildTerminalDeliveryIntent(execCtx, msg, result, execErr, progressMsgID, replyMsgType, replyContent)
		g.dispatchTerminalIntent(execCtx, intent)
	}
}

func detachedContext(execCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	baseCtx := context.Background()
	if execCtx != nil {
		baseCtx = context.WithoutCancel(execCtx)
	}
	return context.WithTimeout(baseCtx, timeout)
}

func (g *Gateway) buildTerminalDeliveryIntent(execCtx context.Context, msg *incomingMessage, result *agent.TaskResult, execErr error, progressMsgID, msgType, content string) DeliveryIntent {
	runID := ""
	sessionID := ""
	if result != nil {
		runID = strings.TrimSpace(result.RunID)
		sessionID = strings.TrimSpace(result.SessionID)
	}
	if runID == "" {
		runID = strings.TrimSpace(id.RunIDFromContext(execCtx))
	}
	if sessionID == "" {
		sessionID = strings.TrimSpace(id.SessionIDFromContext(execCtx))
	}
	eventType := "result_final"
	switch {
	case execErr != nil:
		eventType = "result_failed"
	case result != nil && strings.EqualFold(strings.TrimSpace(result.StopReason), "await_user_input"):
		eventType = "result_await"
	}

	intent := DeliveryIntent{
		Channel:           chatSessionBindingChannel,
		ChatID:            strings.TrimSpace(msg.chatID),
		ReplyToMessageID:  strings.TrimSpace(msg.messageID),
		ProgressMessageID: strings.TrimSpace(progressMsgID),
		SessionID:         sessionID,
		RunID:             runID,
		EventType:         eventType,
		Sequence:          1,
		MsgType:           msgType,
		Content:           content,
		Status:            DeliveryIntentPending,
	}
	if result != nil && len(result.Attachments) > 0 {
		intent.Attachments = filterReferencedAttachments(result.Attachments, content)
	}
	intent.IdempotencyKey = buildTerminalDeliveryIdempotencyKey(intent)
	return intent
}

func buildTerminalDeliveryIdempotencyKey(intent DeliveryIntent) string {
	runKey := strings.TrimSpace(intent.RunID)
	if runKey == "" {
		runKey = strings.TrimSpace(intent.ReplyToMessageID)
	}
	if runKey == "" {
		runKey = strings.TrimSpace(intent.SessionID)
	}
	if runKey == "" {
		runKey = "unknown"
	}
	return fmt.Sprintf("lark:%s:%s:%s:%d", strings.TrimSpace(intent.ChatID), runKey, strings.TrimSpace(intent.EventType), intent.Sequence)
}

func (g *Gateway) dispatchTerminalIntent(execCtx context.Context, intent DeliveryIntent) {
	mode := normalizeDeliveryMode(g.cfg.DeliveryMode)
	store := g.deliveryOutboxStore

	switch mode {
	case DeliveryModeOutbox:
		if store != nil && g.cfg.DeliveryWorker.Enabled {
			storeCtx, cancel := detachedContext(execCtx, 5*time.Second)
			enqueued, err := store.Enqueue(storeCtx, []DeliveryIntent{intent})
			cancel()
			if err == nil && len(enqueued) > 0 {
				return
			}
			if err != nil {
				g.logger.Warn("Lark outbox enqueue failed in outbox mode, fallback to direct dispatch: %v", err)
			} else {
				g.logger.Warn("Lark outbox enqueue returned empty in outbox mode, fallback to direct dispatch")
			}
		} else {
			g.logger.Warn("Lark outbox mode enabled but outbox store/worker unavailable, fallback to direct dispatch")
		}
	case DeliveryModeShadow:
		var stored DeliveryIntent
		if store != nil {
			storeCtx, cancel := detachedContext(execCtx, 5*time.Second)
			enqueued, err := store.Enqueue(storeCtx, []DeliveryIntent{intent})
			cancel()
			if err != nil {
				g.logger.Warn("Lark outbox enqueue failed in shadow mode: %v", err)
			} else if len(enqueued) > 0 {
				stored = enqueued[0]
			}
		}
		if stored.IntentID != "" && stored.Status == DeliveryIntentSent {
			return
		}
		if err := g.deliverIntent(execCtx, intent); err != nil {
			g.logger.Warn("Lark direct terminal delivery failed in shadow mode: %v", err)
			if store != nil && stored.IntentID != "" {
				storeCtx, cancel := detachedContext(execCtx, 5*time.Second)
				_ = store.MarkDead(storeCtx, stored.IntentID, err.Error())
				cancel()
			}
			return
		}
		if store != nil && stored.IntentID != "" {
			storeCtx, cancel := detachedContext(execCtx, 5*time.Second)
			_ = store.MarkSent(storeCtx, stored.IntentID, g.currentTime())
			cancel()
		}
		return
	}

	if err := g.deliverIntent(execCtx, intent); err != nil {
		g.logger.Warn("Lark direct terminal delivery failed: %v", err)
	}
}

// dispatchMultiMessageReply sends multiple chunks as separate messages,
// editing the progress message in-place for the first chunk and sending
// new reply messages for subsequent chunks with a delay between them.
func (g *Gateway) dispatchMultiMessageReply(execCtx context.Context, msg *incomingMessage, result *agent.TaskResult, execErr error, progressMsgID string, chunks []string) {
	delay := time.Duration(0)
	if g.cfg.MessageSplit != nil {
		delay = g.cfg.MessageSplit.delayBetween()
	}

	for i, chunk := range chunks {
		chunkMsgType, chunkContent := smartContent(chunk)
		intent := g.buildTerminalDeliveryIntent(execCtx, msg, result, execErr, "", chunkMsgType, chunkContent)
		intent.Sequence = int64(i + 1)
		intent.IdempotencyKey = buildTerminalDeliveryIdempotencyKey(intent)

		if i == 0 && progressMsgID != "" {
			// First chunk: edit progress message in-place.
			intent.ProgressMessageID = progressMsgID
		}

		// Last chunk: attach any attachments.
		if i == len(chunks)-1 && result != nil && len(result.Attachments) > 0 {
			intent.Attachments = filterReferencedAttachments(result.Attachments, chunkContent)
		} else {
			intent.Attachments = nil
		}

		g.dispatchTerminalIntent(execCtx, intent)

		// Delay between messages to simulate typing rhythm.
		if i < len(chunks)-1 && delay > 0 {
			time.Sleep(delay)
		}
	}
}

func (g *Gateway) isIntentionalTaskCancellation(chatID string, taskToken uint64) bool {
	if g == nil || taskToken == 0 {
		return false
	}
	raw, ok := g.activeSlots.Load(strings.TrimSpace(chatID))
	if !ok {
		return false
	}
	slot, ok := raw.(*sessionSlot)
	if !ok || slot == nil {
		return false
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return slot.intentionalCancelToken == taskToken
}

// buildPlanReviewReplyContent handles plan review marker extraction,
// pending store save, and returns the reply text, message type,
// and content payload.
func (g *Gateway) buildPlanReviewReplyContent(execCtx context.Context, msg *incomingMessage, result *agent.TaskResult) (reply, msgType, content string) {
	marker, ok := extractPlanReviewMarker(result.Messages)
	if !ok {
		return "", "", ""
	}

	reply = buildPlanReviewReply(marker, g.cfg.PlanReviewRequireConfirmation)

	if g.planReviewStore != nil {
		if err := g.planReviewStore.SavePending(execCtx, PlanReviewPending{
			UserID:        msg.senderID,
			ChatID:        msg.chatID,
			RunID:         marker.RunID,
			OverallGoalUI: marker.OverallGoalUI,
			InternalPlan:  marker.InternalPlan,
		}); err != nil {
			g.logger.Warn("Lark plan review pending save failed: %v", err)
		}
	}

	return reply, "text", ""
}

// drainAndReprocess drains any remaining messages from the input channel after
// a task finishes and reprocesses each as a new task. This handles messages that
// arrived between the last ReAct iteration drain and the task completion.
// Messages are processed sequentially in a single goroutine to preserve ordering.
func (g *Gateway) drainAndReprocess(ch chan agent.UserInput, chatID, chatType string) {
	var remaining []agent.UserInput
	for {
		select {
		case msg := <-ch:
			remaining = append(remaining, msg)
		default:
			goto done
		}
	}
done:
	if len(remaining) == 0 {
		return
	}
	g.taskWG.Add(1)
	go func() {
		defer g.taskWG.Done()
		for _, msg := range remaining {
			g.reprocessMessage(chatID, chatType, msg)
		}
	}()
}

// discardPendingInputs drains and drops remaining in-flight messages that were
// not consumed by the running task. This avoids automatically starting a new
// task round when the previous run has already produced a terminal answer.
func (g *Gateway) discardPendingInputs(ch chan agent.UserInput, chatID string) {
	dropped := 0
	for {
		select {
		case <-ch:
			dropped++
		default:
			if dropped > 0 {
				g.logger.Info("Discarded %d pending in-flight message(s) for chat %s", dropped, chatID)
			}
			return
		}
	}
}

// reprocessMessage re-injects a drained user input as if it were a fresh Lark
// message. This creates a synthetic P2MessageReceiveV1 event and feeds it back
// through handleMessage so the full pipeline (dedup, session, execution) runs.
func (g *Gateway) reprocessMessage(chatID, chatType string, input agent.UserInput) {
	msgID := input.MessageID
	content := input.Content

	g.logger.Info("Reprocessing drained message for chat %s (msg_id=%s)", chatID, msgID)

	chatType = utils.TrimLower(chatType)
	if chatType == "" {
		chatType = "p2p"
	}
	msgType := "text"
	contentJSON := textContent(content)

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageId:   &msgID,
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: &msgType,
				Content:     &contentJSON,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &input.SenderID,
				},
			},
		},
	}
	if err := g.handleMessageWithOptions(context.Background(), event, messageProcessingOptions{skipDedup: true}); err != nil {
		g.logger.Warn("Reprocess message failed for chat %s: %v", chatID, err)
	}
}

// buildReply constructs the reply string from the agent result, then rephrases
// it into natural conversational Chinese via LLM when available.
func (g *Gateway) buildReply(ctx context.Context, result *agent.TaskResult, execErr error) string {
	reply := channels.BuildReplyCore(g.cfg.BaseConfig, result, execErr)
	if result == nil {
		// No result — task failed before producing output. Sanitize the raw
		// error deterministically so Go chain prefixes are never shown to users.
		if execErr != nil {
			reply = "执行失败：" + sanitizeErrorForUser(execErr.Error())
		}
		return channels.ShapeReply7C(reply)
	}
	reply = channels.ShapeReply7C(reply)
	return g.rephraseForUser(ctx, reply, rephraseForeground)
}
