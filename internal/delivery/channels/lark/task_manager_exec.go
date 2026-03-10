package lark

import (
	"context"
	"fmt"
	"strings"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/workdir"
	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	builtinshared "alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"
)

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
			reply = "会话初始化失败，请稍后重试，或回复\u201c诊断\u201d让我输出可定位信息。"
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

	result, execErr := g.agent.ExecuteTask(execCtx, taskContent, sessionID, listener)

	// Remove the processing reaction now that the task has completed.
	g.removeProcessingReaction(execCtx, msg.messageID, processingReactionID)

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
