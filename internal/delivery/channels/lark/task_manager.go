package lark

import (
	"context"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	toolcontext "alex/internal/app/toolcontext"
	"alex/internal/app/workdir"
	"alex/internal/delivery/channels"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	id "alex/internal/shared/utils/id"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// getOrCreateSlot returns the session slot for the given chat, creating one if needed.
func (g *Gateway) getOrCreateSlot(chatID string) *sessionSlot {
	slot, _ := g.activeSlots.LoadOrStore(chatID, &sessionSlot{})
	return slot.(*sessionSlot)
}

// storePendingRelay adds a pending input relay to the per-chat queue.
func (g *Gateway) storePendingRelay(chatID string, relay *pendingInputRelay) {
	raw, _ := g.pendingInputRelays.LoadOrStore(chatID, &pendingRelayQueue{})
	queue := raw.(*pendingRelayQueue)
	queue.Push(relay)
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
	relay := queue.PopOldest()
	if relay == nil {
		return false
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
	slot.sessionID = newSessionID
	slot.lastSessionID = newSessionID
	slot.phase = slotIdle
	slot.pendingOptions = nil
	slot.mu.Unlock()

	execCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", newSessionID, msg.senderID, msg.chatID, msg.isGroup)
	execCtx = toolcontext.WithLarkClient(execCtx, g.client)
	execCtx = toolcontext.WithLarkChatID(execCtx, msg.chatID)
	execCtx = toolcontext.WithLarkMessageID(execCtx, msg.messageID)
	g.persistChatSessionBinding(execCtx, msg.chatID, newSessionID)
	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent("已开启新会话，后续消息将使用新的上下文。"))
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
	slot.mu.Unlock()

	execCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", sessionID, msg.senderID, msg.chatID, msg.isGroup)
	execCtx = toolcontext.WithLarkClient(execCtx, g.client)
	execCtx = toolcontext.WithLarkChatID(execCtx, msg.chatID)
	execCtx = toolcontext.WithLarkMessageID(execCtx, msg.messageID)
	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent("`/reset` 已弃用，请使用 `/new` 开启新的会话。"))
}

// resolveSessionForNewTask decides whether to reuse the awaiting session or
// create a fresh one. Must be called while slot.mu is held.
func (g *Gateway) resolveSessionForNewTask(ctx context.Context, chatID string, slot *sessionSlot) (sessionID string, isResume bool) {
	if slot.phase == slotAwaitingInput && slot.sessionID != "" {
		return slot.sessionID, true
	}
	// Reuse the last session to preserve conversation history across turns.
	if slot.lastSessionID != "" {
		return slot.lastSessionID, false
	}
	if persisted := g.loadPersistedChatSessionBinding(ctx, chatID); persisted != "" {
		return persisted, false
	}
	return g.newSessionID(), false
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
func (g *Gateway) runTask(msg *incomingMessage, sessionID string, inputCh chan agent.UserInput, isResume bool) bool {
	execCtx := g.buildExecContext(msg, sessionID, inputCh)

	// Inject CompletionNotifier so BackgroundTaskManager writes TaskStore directly.
	execCtx = agent.WithCompletionNotifier(execCtx, g)

	session, err := g.agent.EnsureSession(execCtx, sessionID)
	if err != nil {
		g.logger.Warn("Lark ensure session failed: %v", err)
		reply := g.buildReply(nil, fmt.Errorf("ensure session: %w", err))
		if reply == "" {
			reply = "（无可用回复）"
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
	execCtx = toolcontext.WithParentListener(execCtx, listener)

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

	startEmoji, endEmoji := g.pickReactionEmojis()
	if msg.messageID != "" && startEmoji != "" {
		go g.addReaction(execCtx, msg.messageID, "Get")
	}

	result, execErr := g.agent.ExecuteTask(execCtx, taskContent, sessionID, listener)

	if msg.messageID != "" && endEmoji != "" {
		go g.addReaction(execCtx, msg.messageID, endEmoji)
	}

	// Close the progress listener before reading MessageID to ensure
	// no timer-fired flushes can race with the edit-in-place operation.
	// Close is idempotent; the deferred cleanupListeners will no-op.
	var progressMsgID string
	if progressLn != nil {
		progressLn.Close()
		progressMsgID = progressLn.MessageID()
	}

	g.dispatchResult(execCtx, msg, result, execErr, awaitTracker, progressMsgID)

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
func (g *Gateway) buildExecContext(msg *incomingMessage, sessionID string, inputCh chan agent.UserInput) context.Context {
	execCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", sessionID, msg.senderID, msg.chatID, msg.isGroup)
	execCtx = toolcontext.WithLarkClient(execCtx, g.client)
	execCtx = toolcontext.WithLarkChatID(execCtx, msg.chatID)
	execCtx = toolcontext.WithLarkMessageID(execCtx, msg.messageID)
	if calendarID := strings.TrimSpace(g.cfg.TenantCalendarID); calendarID != "" {
		execCtx = toolcontext.WithLarkTenantCalendarID(execCtx, calendarID)
	}
	if g.oauth != nil {
		execCtx = toolcontext.WithLarkOAuth(execCtx, g.oauth)
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
	execCtx = toolcontext.WithAutoUploadConfig(execCtx, toolcontext.AutoUploadConfig{
		Enabled:   g.cfg.AutoUploadFiles,
		MaxBytes:  autoUploadMaxBytes,
		AllowExts: normalizeExtensions(g.cfg.AutoUploadAllowExt),
	})

	execCtx = g.applyPinnedLarkLLMSelection(execCtx, msg)

	return execCtx
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
		cleanups = append(cleanups, bgLn.Close)
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

	listener = newFinalAnswerReviewReactionListener(
		execCtx,
		listener,
		g,
		msg.messageID,
		strings.TrimSpace(g.cfg.FinalAnswerReviewReactEmoji),
	)

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
	if hasPlanReview {
		return taskContent + "\n\n[近期对话]\n" + chatHistory
	}
	return "[近期对话]\n" + chatHistory + "\n\n" + taskContent
}

// dispatchResult builds the reply from the execution result and sends it to
// the Lark chat, including any attachments. When progressMsgID is non-empty,
// the progress message is edited in-place to become the final reply, avoiding
// message fragmentation.
func (g *Gateway) dispatchResult(execCtx context.Context, msg *incomingMessage, result *agent.TaskResult, execErr error, awaitTracker *awaitQuestionTracker, progressMsgID string) {
	isAwait := execErr == nil && isResultAwaitingInput(result)
	awaitPrompt, hasAwaitPrompt := agent.AwaitUserInputPrompt{}, false
	if isAwait && result != nil {
		awaitPrompt, hasAwaitPrompt = agent.ExtractAwaitUserInputPrompt(result.Messages)
	}

	reply := ""
	replyContent := ""

	if isAwait && g.cfg.PlanReviewEnabled {
		reply, _, replyContent = g.buildPlanReviewReplyContent(execCtx, msg, result)
	}

	skipReply := isAwait && awaitTracker.Sent()

	if replyContent == "" && !skipReply {
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
			reply = g.buildReply(result, execErr)
		}
		if reply == "" {
			reply = "（无可用回复）"
		}
		if summary := buildAttachmentSummary(result); summary != "" {
			reply += "\n\n" + summary
		}
		replyContent = textContent(reply)
	}

	if !skipReply {
		// When a progress message exists, edit it into the final reply
		// so the user sees a single message that transitions from
		// "在思考…" → final answer, rather than two separate messages.
		edited := false
		if progressMsgID != "" {
			if err := g.updateMessage(execCtx, progressMsgID, reply); err != nil {
				g.logger.Warn("Lark progress→reply edit failed, falling back to new message: %v", err)
			} else {
				edited = true
			}
		}
		if !edited {
			g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", replyContent)
		}
		g.sendAttachments(execCtx, msg.chatID, msg.messageID, result)
	}
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

	chatType = strings.ToLower(strings.TrimSpace(chatType))
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

// buildReply constructs the reply string from the agent result.
func (g *Gateway) buildReply(result *agent.TaskResult, execErr error) string {
	reply := channels.BuildReplyCore(g.cfg.BaseConfig, result, execErr)
	if reply == "" && result != nil {
		// Lark-specific fallback: check thinking content for models that reason but produce no text.
		if fallback := extractThinkingFallback(result.Messages); fallback != "" {
			reply = fallback
			if g.cfg.ReplyPrefix != "" {
				reply = g.cfg.ReplyPrefix + reply
			}
		}
	}
	return reply
}

// extractThinkingFallback scans messages in reverse for the last assistant
// message with non-empty thinking content. This is a safety net for models
// that reason but produce no text output.
func extractThinkingFallback(msgs []ports.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if msg.Role != "assistant" {
			continue
		}
		for _, part := range msg.Thinking.Parts {
			text := strings.TrimSpace(part.Text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}
