package lark

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/subscription"
	"alex/internal/delivery/channels"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	toolports "alex/internal/domain/agent/ports/tools"
	larkoauth "alex/internal/infra/lark/oauth"
	artifacts "alex/internal/infra/tools/builtin/artifacts"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/json"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"

	lru "github.com/hashicorp/golang-lru/v2"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

const (
	messageDedupCacheSize = 2048
	messageDedupTTL       = 10 * time.Minute
)

// AgentExecutor is an alias for the shared channel executor interface.
type AgentExecutor = channels.AgentExecutor

// slotPhase describes the lifecycle phase of a sessionSlot.
type slotPhase int

const (
	slotIdle          slotPhase = iota // no active task
	slotRunning                        // task goroutine is active
	slotAwaitingInput                  // task ended with await_user_input, waiting for user reply
)

// sessionSlot tracks whether a task is active for a given chat and holds
// the user input channel used to inject follow-up messages into a running
// ReAct loop, plus the pending session state for await-user-input handoffs.
type sessionSlot struct {
	mu             sync.Mutex
	phase          slotPhase
	inputCh        chan agent.UserInput // non-nil only when phase == slotRunning
	sessionID      string
	lastSessionID  string
	pendingOptions []string // options awaiting numeric reply
}

// Gateway bridges Lark bot messages into the agent runtime.
type Gateway struct {
	channels.BaseGateway
	cfg                Config
	agent              AgentExecutor
	logger             logging.Logger
	client             *lark.Client
	wsClient           *larkws.Client
	messenger          LarkMessenger
	eventListener      agent.EventListener
	emojiPicker        *emojiPicker
	dedupMu            sync.Mutex
	dedupCache         *lru.Cache[string, time.Time]
	now                func() time.Time
	planReviewStore    PlanReviewStore
	oauth              *larkoauth.Service
	llmSelections      *subscription.SelectionStore
	llmResolver        *subscription.SelectionResolver
	cliCredsLoader     func() runtimeconfig.CLICredentials
	llamaResolver      func(context.Context) (subscription.LlamaServerTarget, bool)
	taskStore          TaskStore
	noticeState        *noticeStateStore
	activeSlots        sync.Map           // chatID → *sessionSlot
	pendingInputRelays sync.Map           // chatID → *pendingRelayQueue
	aiCoordinator      *AIChatCoordinator // coordinates multi-bot chat sessions
	taskWG             sync.WaitGroup     // tracks running task goroutines (for tests)
}

type awaitQuestionTracker struct {
	mu   sync.Mutex
	sent bool
}

func (t *awaitQuestionTracker) MarkSent() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.sent = true
	t.mu.Unlock()
}

func (t *awaitQuestionTracker) Sent() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sent
}

// NewGateway constructs a Lark gateway instance.
func NewGateway(cfg Config, agent AgentExecutor, logger logging.Logger) (*Gateway, error) {
	if agent == nil {
		return nil, fmt.Errorf("lark gateway requires agent executor")
	}
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "" {
		return nil, fmt.Errorf("lark gateway requires app_id and app_secret")
	}
	cfg.SessionPrefix = strings.TrimSpace(cfg.SessionPrefix)
	if cfg.SessionPrefix == "" {
		cfg.SessionPrefix = "lark"
	}
	cfg.ToolPreset = strings.TrimSpace(strings.ToLower(cfg.ToolPreset))
	if cfg.ToolPreset == "" {
		cfg.ToolPreset = "full"
	}
	if cfg.BackgroundProgressEnabled == nil {
		enabled := true
		cfg.BackgroundProgressEnabled = &enabled
	}
	dedupCache, err := lru.New[string, time.Time](messageDedupCacheSize)
	if err != nil {
		return nil, fmt.Errorf("lark message deduper init: %w", err)
	}
	logger = logging.OrNop(logger)

	// Initialize AI chat coordinator if bot IDs are configured
	var aiCoordinator *AIChatCoordinator
	if len(cfg.AIChatBotIDs) > 0 {
		aiCoordinator = NewAIChatCoordinator(logger, cfg.AIChatBotIDs)
	}

	selectionPath := subscription.ResolveSelectionStorePath(runtimeconfig.DefaultEnvLookup, nil)
	return &Gateway{
		cfg:           cfg,
		agent:         agent,
		logger:        logger,
		emojiPicker:   newEmojiPicker(time.Now().UnixNano(), resolveEmojiPool(cfg.ReactEmoji)),
		dedupCache:    dedupCache,
		now:           time.Now,
		llmSelections: subscription.NewSelectionStore(selectionPath),
		noticeState:   newNoticeStateStore(logger),
		llmResolver: subscription.NewSelectionResolver(func() runtimeconfig.CLICredentials {
			return runtimeconfig.LoadCLICredentials()
		}),
		cliCredsLoader: func() runtimeconfig.CLICredentials {
			return runtimeconfig.LoadCLICredentials()
		},
		llamaResolver: func(context.Context) (subscription.LlamaServerTarget, bool) {
			return resolveLlamaServerTarget(runtimeconfig.DefaultEnvLookup)
		},
		aiCoordinator: aiCoordinator,
	}, nil
}

// SetEventListener configures an optional listener to receive workflow events.
func (g *Gateway) SetEventListener(listener agent.EventListener) {
	if g == nil {
		return
	}
	g.eventListener = listener
}

// SetPlanReviewStore configures the pending plan review store.
func (g *Gateway) SetPlanReviewStore(store PlanReviewStore) {
	if g == nil {
		return
	}
	g.planReviewStore = store
}

// SetOAuthService configures the Lark user OAuth service used for user-scoped API calls.
func (g *Gateway) SetOAuthService(svc *larkoauth.Service) {
	if g == nil {
		return
	}
	g.oauth = svc
}

// SetMessenger replaces the default SDK messenger with a custom implementation.
// This is the primary injection point for testing.
func (g *Gateway) SetMessenger(m LarkMessenger) {
	if g == nil {
		return
	}
	g.messenger = m
}

// SetTaskStore configures the task persistence store.
func (g *Gateway) SetTaskStore(store TaskStore) {
	if g == nil {
		return
	}
	g.taskStore = store
}

// SendNotification sends a text notification to a Lark chat (no session/slot management).
// This is used by external bridges (e.g. hooks bridge) to push messages to Lark.
func (g *Gateway) SendNotification(ctx context.Context, chatID, text string) error {
	if g == nil || g.messenger == nil {
		return fmt.Errorf("lark messenger not initialized")
	}
	_, err := g.messenger.SendMessage(ctx, chatID, "text", textContent(text))
	return err
}

// SetAIChatCoordinator configures the AI chat coordinator for multi-bot conversations.
func (g *Gateway) SetAIChatCoordinator(coordinator *AIChatCoordinator) {
	if g == nil {
		return
	}
	g.aiCoordinator = coordinator
}

// NotifyCompletion implements agent.BackgroundCompletionNotifier. It writes
// the final task status directly to TaskStore, ensuring persistence even when
// the event listener chain is broken (e.g. SerializingEventListener idle timeout).
func (g *Gateway) NotifyCompletion(ctx context.Context, taskID, status, answer, errText string, tokensUsed int) {
	if g == nil || g.taskStore == nil {
		return
	}
	// Use a detached context: the caller's task context may already be cancelled
	// (e.g. for cancelled tasks), but this persistence write must still succeed.
	storeCtx := context.WithoutCancel(ctx)
	var opts []TaskUpdateOption
	if answer != "" {
		opts = append(opts, WithAnswerPreview(truncateForLark(answer, 1500)))
	}
	if errText != "" {
		opts = append(opts, WithErrorText(truncateForLark(errText, 1500)))
	}
	if tokensUsed > 0 {
		opts = append(opts, WithTokensUsed(tokensUsed))
	}
	if err := g.taskStore.UpdateStatus(storeCtx, taskID, status, opts...); err != nil {
		g.logger.Warn("CompletionNotifier: TaskStore update failed for %s: %v", taskID, err)
	}
}

// getOrCreateSlot returns the session slot for the given chat, creating one if needed.
func (g *Gateway) getOrCreateSlot(chatID string) *sessionSlot {
	slot, _ := g.activeSlots.LoadOrStore(chatID, &sessionSlot{})
	return slot.(*sessionSlot)
}

// Start creates the Lark SDK client, event dispatcher, and WebSocket client, then blocks.
func (g *Gateway) Start(ctx context.Context) error {
	if !g.cfg.Enabled {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Build the REST client for sending replies.
	var clientOpts []lark.ClientOptionFunc
	if domain := strings.TrimSpace(g.cfg.BaseDomain); domain != "" {
		clientOpts = append(clientOpts, lark.WithOpenBaseUrl(domain))
	}
	g.client = lark.NewClient(g.cfg.AppID, g.cfg.AppSecret, clientOpts...)

	// Initialize the messenger if not already set (e.g. by tests).
	if g.messenger == nil {
		g.messenger = newSDKMessenger(g.client)
	}

	// Build the event dispatcher and register event handlers.
	eventDispatcher := dispatcher.NewEventDispatcher("", "")
	eventDispatcher.OnP2MessageReceiveV1(g.handleMessage)
	eventDispatcher.OnP2MessageReactionCreatedV1(func(_ context.Context, _ *larkim.P2MessageReactionCreatedV1) error {
		return nil
	})
	eventDispatcher.OnP2ChatAccessEventBotP2pChatEnteredV1(func(_ context.Context, _ *larkim.P2ChatAccessEventBotP2pChatEnteredV1) error {
		return nil
	})
	eventDispatcher.OnP2MessageReadV1(func(_ context.Context, _ *larkim.P2MessageReadV1) error {
		return nil
	})

	// Build and start the WebSocket client.
	var wsOpts []larkws.ClientOption
	wsOpts = append(wsOpts, larkws.WithEventHandler(eventDispatcher))
	wsOpts = append(wsOpts, larkws.WithLogLevel(larkcore.LogLevelInfo))
	if domain := strings.TrimSpace(g.cfg.BaseDomain); domain != "" {
		wsOpts = append(wsOpts, larkws.WithDomain(domain))
	}
	g.wsClient = larkws.NewClient(g.cfg.AppID, g.cfg.AppSecret, wsOpts...)

	g.logger.Info("Lark gateway connecting (app_id=%s)...", g.cfg.AppID)
	return g.wsClient.Start(ctx)
}

// Stop releases resources. The WebSocket client does not expose a Stop method;
// cancelling the context passed to Start is the primary shutdown mechanism.
func (g *Gateway) Stop() {
	// The Lark WS client is stopped by cancelling its context.
}

// WaitForTasks blocks until all in-flight task goroutines complete.
// Intended for test synchronization only.
func (g *Gateway) WaitForTasks() {
	g.taskWG.Wait()
}

// incomingMessage holds the parsed fields from a Lark message event.
type incomingMessage struct {
	chatID              string
	chatType            string
	messageID           string
	senderID            string
	content             string
	isGroup             bool
	isFromBot           bool
	aiChatSessionActive bool // true if this message is part of an AI chat session
}

// isResultAwaitingInput reports whether the task result indicates an
// await_user_input stop reason.
func isResultAwaitingInput(result *agent.TaskResult) bool {
	return result != nil && strings.EqualFold(strings.TrimSpace(result.StopReason), "await_user_input")
}

type messageProcessingOptions struct {
	skipDedup bool
}

// handleMessage is the P2MessageReceiveV1 event handler.
func (g *Gateway) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	return g.handleMessageWithOptions(ctx, event, messageProcessingOptions{})
}

func (g *Gateway) handleMessageWithOptions(ctx context.Context, event *larkim.P2MessageReceiveV1, opts messageProcessingOptions) error {
	msg := g.parseIncomingMessage(event, opts)
	if msg == nil {
		return nil
	}
	g.logger.Info("Lark message received: chat_id=%s msg_id=%s sender=%s group=%t len=%d", msg.chatID, msg.messageID, msg.senderID, msg.isGroup, len(msg.content))

	// AI Chat Coordination: Check if this is a multi-bot chat scenario
	if g.aiCoordinator != nil && msg.isGroup {
		// Skip processing if this is a message from another bot in an active AI chat session
		// to prevent infinite loops
		if msg.isFromBot && g.aiCoordinator.IsMessageFromParticipantBot(msg.chatID, msg.senderID) {
			g.logger.Info("Skipping message from participant bot in AI chat session: chat=%s sender=%s", msg.chatID, msg.senderID)
			return nil
		}

		// Check if this should trigger or participate in an AI chat session
		mentions := extractMentions(event)
		shouldParticipate, waitForTurn := g.aiCoordinator.DetectAndStartSession(
			msg.chatID, msg.messageID, msg.senderID, mentions, g.cfg.AppID,
		)

		if shouldParticipate {
			if waitForTurn {
				g.logger.Info("AI chat: waiting for turn in chat=%s", msg.chatID)
				return nil
			}
			// It's our turn to respond
			g.logger.Info("AI chat: our turn to respond in chat=%s", msg.chatID)
			msg.aiChatSessionActive = true
		}
	}

	slot := g.getOrCreateSlot(msg.chatID)
	slot.mu.Lock()
	trimmedContent := strings.TrimSpace(msg.content)

	// Natural-language status query should be handled immediately, even when a
	// foreground task is currently running.
	if g.isNaturalTaskStatusQuery(trimmedContent) {
		slot.mu.Unlock()
		g.handleNaturalTaskStatusQuery(msg)
		return nil
	}
	if g.isNoticeCommand(trimmedContent) {
		slot.mu.Unlock()
		g.handleNoticeCommand(msg)
		return nil
	}

	// If a task is already running for this chat, inject the new message
	// into the running ReAct loop instead of starting a new task.
	if slot.phase == slotRunning {
		ch := slot.inputCh
		activeSessionID := slot.sessionID
		slot.mu.Unlock()
		if g.tryResolveInputReply(ctx, msg.chatID, strings.TrimSpace(msg.content)) {
			return nil
		}
		g.injectUserInput(ch, activeSessionID, msg)
		return nil
	}

	// Handle /reset outside of a running task.
	if strings.TrimSpace(msg.content) == "/reset" {
		g.handleResetCommand(slot, msg) // releases slot.mu
		return nil
	}
	if strings.HasPrefix(strings.TrimSpace(msg.content), "/model") || strings.HasPrefix(strings.TrimSpace(msg.content), "/models") {
		slot.mu.Unlock()
		g.handleModelCommand(msg)
		return nil
	}
	isPlan := g.isPlanCommand(trimmedContent)
	if g.isTaskCommand(trimmedContent) || isPlan {
		slot.mu.Unlock()
		if isPlan {
			g.handlePlanModeCommand(msg)
		} else {
			g.handleTaskCommand(msg)
		}
		return nil
	}

	// Resolve session ID: reuse the awaiting session or create a new one.
	sessionID, isResume := g.resolveSessionForNewTask(slot)
	inputCh := make(chan agent.UserInput, 16)
	slot.phase = slotRunning
	slot.inputCh = inputCh
	slot.sessionID = sessionID
	slot.lastSessionID = sessionID
	slot.mu.Unlock()

	// Run the task asynchronously so the Lark SDK event handler returns
	// immediately and can ACK the WS frame. Without this, long-running
	// tasks delay the ACK, causing the Lark server to re-deliver the
	// event and produce duplicate responses.
	g.taskWG.Add(1)
	go func() {
		defer g.taskWG.Done()
		awaitingInput := g.runTask(msg, sessionID, inputCh, isResume)

		slot.mu.Lock()
		slot.inputCh = nil
		if awaitingInput {
			slot.phase = slotAwaitingInput
			slot.lastSessionID = slot.sessionID
		} else {
			slot.phase = slotIdle
			slot.sessionID = ""
		}
		slot.mu.Unlock()
		if awaitingInput {
			g.drainAndReprocess(inputCh, msg.chatID, msg.chatType)
		} else {
			g.discardPendingInputs(inputCh, msg.chatID)
		}
	}()

	return nil
}

// parseIncomingMessage validates the event and extracts key fields.
// Returns nil if the message should be skipped (unsupported type, disallowed
// chat, empty content, duplicate, etc.).
func (g *Gateway) parseIncomingMessage(event *larkim.P2MessageReceiveV1, opts messageProcessingOptions) *incomingMessage {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}
	raw := event.Event.Message

	msgType := strings.ToLower(strings.TrimSpace(deref(raw.MessageType)))
	if msgType != "text" && msgType != "post" {
		return nil
	}

	chatType := strings.ToLower(strings.TrimSpace(deref(raw.ChatType)))
	isGroup := chatType != "" && chatType != "p2p"
	if isGroup && !g.cfg.AllowGroups {
		return nil
	}
	if !isGroup && !g.cfg.AllowDirect {
		return nil
	}

	content := g.extractMessageContent(msgType, deref(raw.Content), raw.Mentions)
	if content == "" {
		return nil
	}

	chatID := deref(raw.ChatId)
	if chatID == "" {
		g.logger.Warn("Lark message has empty chat_id; skipping")
		return nil
	}

	messageID := deref(raw.MessageId)
	if messageID != "" && !opts.skipDedup && g.isDuplicateMessage(messageID) {
		g.logger.Warn("Lark duplicate message skipped (WS re-delivery): msg_id=%s", messageID)
		return nil
	}

	return &incomingMessage{
		chatID:    chatID,
		chatType:  chatType,
		messageID: messageID,
		senderID:  extractSenderID(event),
		content:   content,
		isGroup:   isGroup,
		isFromBot: isBotSender(event),
	}
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

// handleResetCommand processes a /reset message, clearing session state and
// notifying the user. The caller must hold slot.mu; this method releases it.
func (g *Gateway) handleResetCommand(slot *sessionSlot, msg *incomingMessage) {
	resetSessionID := slot.sessionID
	if resetSessionID == "" {
		resetSessionID = slot.lastSessionID
	}
	if resetSessionID == "" {
		resetSessionID = g.memoryIDForChat(msg.chatID)
	}
	slot.sessionID = ""
	slot.lastSessionID = ""
	slot.phase = slotIdle
	slot.mu.Unlock()

	execCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", resetSessionID, msg.senderID, msg.chatID, msg.isGroup)
	execCtx = shared.WithLarkClient(execCtx, g.client)
	execCtx = shared.WithLarkChatID(execCtx, msg.chatID)
	if resetter, ok := g.agent.(interface {
		ResetSession(ctx context.Context, sessionID string) error
	}); ok {
		if err := resetter.ResetSession(execCtx, resetSessionID); err != nil {
			g.logger.Warn("Lark session reset failed: %v", err)
		}
	}
	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent("已清空对话历史，下次对话将从零开始。"))
}

// resolveSessionForNewTask decides whether to reuse the awaiting session or
// create a fresh one. Must be called while slot.mu is held.
func (g *Gateway) resolveSessionForNewTask(slot *sessionSlot) (sessionID string, isResume bool) {
	if slot.phase == slotAwaitingInput && slot.sessionID != "" {
		return slot.sessionID, true
	}
	return g.newSessionID(), false
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
	execCtx = shared.WithParentListener(execCtx, listener)

	// Resolve task content from three distinct concerns:
	// 1. Plan review feedback (if any pending plan review exists)
	taskContent, hasPlanReview := g.resolvePlanReviewFeedback(execCtx, session, msg)

	// 2. Await resume: seed user reply into inputCh; task content becomes empty
	//    so the ReAct loop reads input from the channel instead.
	if isResume && !hasPlanReview {
		g.seedAwaitResumeInput(inputCh, msg, sessionID)
		taskContent = ""
	}

	// 3. Chat context enrichment for group chats (only when there is content)
	taskContent = g.enrichWithChatContext(execCtx, taskContent, msg, hasPlanReview)

	startEmoji, endEmoji := g.pickReactionEmojis()
	if msg.messageID != "" && startEmoji != "" {
		go g.addReaction(execCtx, msg.messageID, "Get")
	}

	result, execErr := g.agent.ExecuteTask(execCtx, taskContent, sessionID, listener)

	if msg.messageID != "" && endEmoji != "" {
		go g.addReaction(execCtx, msg.messageID, endEmoji)
	}

	// Retrieve the progress message ID so dispatchResult can edit it
	// into the final reply instead of sending a separate message.
	var progressMsgID string
	if progressLn != nil {
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
	execCtx = shared.WithLarkClient(execCtx, g.client)
	execCtx = shared.WithLarkChatID(execCtx, msg.chatID)
	execCtx = shared.WithLarkMessageID(execCtx, msg.messageID)
	if calendarID := strings.TrimSpace(g.cfg.TenantCalendarID); calendarID != "" {
		execCtx = shared.WithLarkTenantCalendarID(execCtx, calendarID)
	}
	if g.oauth != nil {
		execCtx = shared.WithLarkOAuth(execCtx, g.oauth)
	}
	execCtx = appcontext.WithPlanReviewEnabled(execCtx, g.cfg.PlanReviewEnabled)
	execCtx = g.applyPlanModeToContext(execCtx, msg)
	execCtx = agent.WithUserInputCh(execCtx, inputCh)

	workspaceDir := strings.TrimSpace(g.cfg.WorkspaceDir)
	if workspaceDir == "" {
		workspaceDir = pathutil.DefaultWorkingDir()
	}
	if workspaceDir != "" {
		execCtx = pathutil.WithWorkingDir(execCtx, workspaceDir)
	}

	autoUploadMaxBytes := g.cfg.AutoUploadMaxBytes
	if autoUploadMaxBytes <= 0 {
		autoUploadMaxBytes = 2 * 1024 * 1024
	}
	execCtx = shared.WithAutoUploadConfig(execCtx, shared.AutoUploadConfig{
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

// enrichWithChatContext prepends (or appends) recent group chat messages as
// context to the task content. Only applies to group chats.
func (g *Gateway) enrichWithChatContext(execCtx context.Context, taskContent string, msg *incomingMessage, hasPlanReview bool) string {
	if taskContent == "" || g.messenger == nil || !msg.isGroup {
		return taskContent
	}
	pageSize := g.cfg.AutoChatContextSize
	if pageSize <= 0 {
		pageSize = 20
	}
	chatHistory, err := g.fetchRecentChatMessages(execCtx, msg.chatID, pageSize)
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

// InjectMessage constructs a synthetic P2MessageReceiveV1 event and feeds it
// through handleMessage. This is the primary entry point for scenario tests:
// it exercises the full pipeline (dedup, session, context, execution, reply)
// without requiring a WebSocket connection.
func (g *Gateway) InjectMessage(ctx context.Context, chatID, chatType, senderID, messageID, text string) error {
	msgType := "text"
	contentJSON := textContent(text)
	if chatType == "" {
		chatType = "p2p"
	}

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: &msgType,
				Content:     &contentJSON,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &senderID,
				},
			},
		},
	}
	return g.handleMessage(ctx, event)
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

func (g *Gateway) isDuplicateMessage(messageID string) bool {
	if messageID == "" {
		return false
	}
	g.dedupMu.Lock()
	defer g.dedupMu.Unlock()

	nowFn := g.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()

	if ts, ok := g.dedupCache.Get(messageID); ok {
		if now.Sub(ts) <= messageDedupTTL {
			return true
		}
		g.dedupCache.Remove(messageID)
	}
	g.dedupCache.Add(messageID, now)
	return false
}

// dispatchMessage sends a message to a Lark chat. When replyToID is non-empty
// the message is sent as a reply to that message; otherwise a new message is
// created in the chat identified by chatID. Returns the new message ID.
func (g *Gateway) dispatchMessage(ctx context.Context, chatID, replyToID, msgType, content string) (string, error) {
	if g.messenger == nil {
		return "", fmt.Errorf("lark messenger not initialized")
	}

	if replyToID != "" {
		return g.messenger.ReplyMessage(ctx, replyToID, msgType, content)
	}
	return g.messenger.SendMessage(ctx, chatID, msgType, content)
}

// dispatch is a fire-and-forget wrapper around dispatchMessage that logs errors.
func (g *Gateway) dispatch(ctx context.Context, chatID, replyToID, msgType, content string) {
	if _, err := g.dispatchMessage(ctx, chatID, replyToID, msgType, content); err != nil {
		g.logger.Warn("Lark dispatch message failed: %v", err)
	}
}

// replyTarget returns the message ID to reply to when allowed.
// An empty ID or disallowed replies indicates no reply target.
func replyTarget(messageID string, allowReply bool) string {
	if !allowReply || messageID == "" {
		return ""
	}
	return messageID
}

// updateMessage updates an existing text message in-place.
func (g *Gateway) updateMessage(ctx context.Context, messageID, text string) error {
	if g.messenger == nil {
		return fmt.Errorf("lark messenger not initialized")
	}
	return g.messenger.UpdateMessage(ctx, messageID, "text", textContent(text))
}

// addReaction adds an emoji reaction to the specified message.
func (g *Gateway) addReaction(ctx context.Context, messageID, emojiType string) {
	if g.messenger == nil || messageID == "" || emojiType == "" {
		g.logger.Warn("Lark add reaction skipped: messenger=%v messageID=%q emojiType=%q", g.messenger != nil, messageID, emojiType)
		return
	}
	if err := g.messenger.AddReaction(ctx, messageID, emojiType); err != nil {
		g.logger.Warn("Lark add reaction failed: %v", err)
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

// newSessionID generates a fresh session identifier for a new Lark task.
func (g *Gateway) newSessionID() string {
	prefix := strings.TrimSpace(g.cfg.SessionPrefix)
	if prefix == "" {
		prefix = "lark"
	}
	return fmt.Sprintf("%s-%s", prefix, id.NewKSUID())
}

// memoryIDForChat derives a deterministic memory identity from a chat ID.
// This stable ID is used as a fallback reset target for the chat.
func (g *Gateway) memoryIDForChat(chatID string) string {
	hash := sha1.Sum([]byte(chatID))
	return fmt.Sprintf("%s-%x", g.cfg.SessionPrefix, hash[:12])
}

// extractMessageContent parses the JSON content from a Lark message.
// Supports "text" and "post" message types, returning a trimmed string.
func (g *Gateway) extractMessageContent(msgType, raw string, mentions []*larkim.MentionEvent) string {
	switch msgType {
	case "text":
		return extractTextContent(raw, mentions)
	case "post":
		return extractPostContent(raw, mentions)
	default:
		return ""
	}
}

type mentionInfo struct {
	Name string
	ID   string
}

func mentionKeyMap(mentions []*larkim.MentionEvent) map[string]mentionInfo {
	if len(mentions) == 0 {
		return nil
	}
	out := make(map[string]mentionInfo, len(mentions))
	for _, mention := range mentions {
		if mention == nil {
			continue
		}
		key := strings.TrimSpace(deref(mention.Key))
		if key == "" {
			continue
		}
		name := strings.TrimSpace(deref(mention.Name))
		id := ""
		if mention.Id != nil {
			id = strings.TrimSpace(deref(mention.Id.OpenId))
			if id == "" {
				id = strings.TrimSpace(deref(mention.Id.UserId))
			}
			if id == "" {
				id = strings.TrimSpace(deref(mention.Id.UnionId))
			}
		}
		out[key] = mentionInfo{Name: name, ID: id}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func withAtPrefix(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "@") {
		return trimmed
	}
	return "@" + trimmed
}

func formatReadableMention(name, id, fallback string) string {
	name = strings.TrimSpace(name)
	id = strings.TrimSpace(id)
	fallback = strings.TrimSpace(fallback)

	if name != "" {
		atName := withAtPrefix(name)
		if id != "" && id != name {
			return atName + "(" + id + ")"
		}
		return atName
	}
	if id != "" {
		return withAtPrefix(id)
	}
	if fallback != "" {
		return withAtPrefix(fallback)
	}
	return ""
}

func renderIncomingMentionPlaceholders(text string, mentionMap map[string]mentionInfo) string {
	if strings.TrimSpace(text) == "" || len(mentionMap) == 0 {
		return text
	}

	keys := make([]string, 0, len(mentionMap))
	for key := range mentionMap {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return text
	}

	// Replace longer keys first to avoid `@_user_1` corrupting `@_user_10`.
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] > keys[j]
	})

	out := text
	for _, key := range keys {
		info := mentionMap[key]
		repl := formatReadableMention(info.Name, info.ID, key)
		if repl == "" || repl == key {
			continue
		}
		out = strings.ReplaceAll(out, key, repl)
	}
	return out
}

// extractTextContent parses a Lark text message content JSON: {"text":"..."}.
func extractTextContent(raw string, mentions []*larkim.MentionEvent) string {
	if raw == "" {
		return ""
	}
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return strings.TrimSpace(raw)
	}
	text := strings.TrimSpace(parsed.Text)
	if text == "" {
		return ""
	}
	mentionMap := mentionKeyMap(mentions)
	text = renderIncomingMentionPlaceholders(text, mentionMap)
	text = renderTextMentions(text, mentionMap)
	return strings.TrimSpace(text)
}

var larkMentionTag = regexp.MustCompile(`<at\s+user_id="([^"]+)"\s*>([^<]*)</at>`)

func renderTextMentions(text string, mentionMap map[string]mentionInfo) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	return larkMentionTag.ReplaceAllStringFunc(text, func(m string) string {
		sub := larkMentionTag.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		userID := strings.TrimSpace(sub[1])
		name := strings.TrimSpace(sub[2])

		mentionID := userID
		if mentionMap != nil {
			if info, ok := mentionMap[userID]; ok {
				if name == "" {
					name = info.Name
				}
				if strings.TrimSpace(info.ID) != "" {
					mentionID = info.ID
				}
			}
		}
		if name == "" {
			name = mentionID
		}
		if mentionID == "" || name == "" {
			return m
		}
		if mentionID == name {
			return withAtPrefix(name)
		}
		return withAtPrefix(name) + "(" + mentionID + ")"
	})
}

// extractPostContent parses a Lark post message content JSON and flattens text.
// The content field is a JSON string like:
// {"title":"...","content":[[{"tag":"text","text":"..."}]]}
func extractPostContent(raw string, mentions []*larkim.MentionEvent) string {
	if raw == "" {
		return ""
	}

	type postElement struct {
		Tag      string `json:"tag"`
		Text     string `json:"text"`
		UserID   string `json:"user_id"`
		UserName string `json:"user_name"`
	}
	type postPayload struct {
		Title   string          `json:"title"`
		Content [][]postElement `json:"content"`
	}

	var parsed postPayload
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return strings.TrimSpace(raw)
	}

	mentionMap := mentionKeyMap(mentions)
	var sb strings.Builder
	if title := strings.TrimSpace(parsed.Title); title != "" {
		sb.WriteString(title)
	}
	for _, line := range parsed.Content {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		for _, el := range line {
			switch el.Tag {
			case "text":
				sb.WriteString(renderIncomingMentionPlaceholders(el.Text, mentionMap))
			case "at":
				rawUserID := strings.TrimSpace(el.UserID)
				userID := rawUserID
				name := strings.TrimSpace(el.UserName)
				if mentionMap != nil {
					if info, ok := mentionMap[rawUserID]; ok {
						if name == "" {
							name = info.Name
						}
						if strings.TrimSpace(info.ID) != "" {
							userID = info.ID
						}
					}
				}
				if mention := formatReadableMention(name, userID, rawUserID); mention != "" {
					sb.WriteString(mention)
				}
			default:
				if el.Text != "" {
					sb.WriteString(el.Text)
				}
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

// textContent builds the JSON content payload for a Lark text message.
func textContent(text string) string {
	text = renderOutgoingMentions(text)
	payload, _ := json.Marshal(map[string]string{"text": text})
	return string(payload)
}

var outgoingMentionPattern = regexp.MustCompile(`@([^@()<>\n\r\t]+)\((ou_[A-Za-z0-9]+|all)\)`)

func renderOutgoingMentions(text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	return outgoingMentionPattern.ReplaceAllStringFunc(text, func(raw string) string {
		sub := outgoingMentionPattern.FindStringSubmatch(raw)
		if len(sub) != 3 {
			return raw
		}
		name := strings.TrimSpace(sub[1])
		userID := strings.TrimSpace(sub[2])
		if userID == "" {
			return raw
		}
		if userID == "all" && (name == "" || strings.EqualFold(name, "all")) {
			name = "所有人"
		}
		if name == "" {
			return raw
		}
		return `<at user_id="` + userID + `">` + name + `</at>`
	})
}

func imageContent(imageKey string) string {
	payload, _ := json.Marshal(map[string]string{"image_key": imageKey})
	return string(payload)
}

func fileContent(fileKey string) string {
	payload, _ := json.Marshal(map[string]string{"file_key": fileKey})
	return string(payload)
}

const (
	planReviewMarkerStart = "<plan_review_pending>"
	planReviewMarkerEnd   = "</plan_review_pending>"
)

type planReviewMarker struct {
	RunID         string
	OverallGoalUI string
	InternalPlan  any
}

func (g *Gateway) loadPlanReviewPending(ctx context.Context, session *storage.Session, userID, chatID string) (PlanReviewPending, bool) {
	if g == nil || userID == "" || chatID == "" {
		return PlanReviewPending{}, false
	}
	if g.planReviewStore != nil {
		pending, ok, err := g.planReviewStore.GetPending(ctx, userID, chatID)
		if err != nil {
			g.logger.Warn("Lark plan review pending load failed: %v", err)
			return PlanReviewPending{}, false
		}
		if ok {
			return pending, true
		}
		return PlanReviewPending{}, false
	}
	if session == nil || len(session.Messages) == 0 {
		return PlanReviewPending{}, false
	}
	if marker, ok := extractPlanReviewMarker(session.Messages); ok {
		return PlanReviewPending{
			UserID:        userID,
			ChatID:        chatID,
			RunID:         marker.RunID,
			OverallGoalUI: marker.OverallGoalUI,
			InternalPlan:  marker.InternalPlan,
		}, true
	}
	return PlanReviewPending{}, false
}

func extractPlanReviewMarker(messages []ports.Message) (planReviewMarker, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if strings.ToLower(strings.TrimSpace(msg.Role)) != "system" {
			continue
		}
		if marker, ok := parsePlanReviewMarker(msg.Content); ok {
			return marker, true
		}
	}
	return planReviewMarker{}, false
}

func parsePlanReviewMarker(content string) (planReviewMarker, bool) {
	start := strings.Index(content, planReviewMarkerStart)
	end := strings.Index(content, planReviewMarkerEnd)
	if start == -1 || end == -1 || end <= start {
		return planReviewMarker{}, false
	}
	body := strings.TrimSpace(content[start+len(planReviewMarkerStart) : end])
	if body == "" {
		return planReviewMarker{}, false
	}
	marker := planReviewMarker{}
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "run_id:"):
			marker.RunID = strings.TrimSpace(strings.TrimPrefix(line, "run_id:"))
		case strings.HasPrefix(line, "overall_goal_ui:"):
			marker.OverallGoalUI = strings.TrimSpace(strings.TrimPrefix(line, "overall_goal_ui:"))
		case strings.HasPrefix(line, "internal_plan:"):
			raw := strings.TrimSpace(strings.TrimPrefix(line, "internal_plan:"))
			if raw != "" {
				var plan any
				if err := jsonx.Unmarshal([]byte(raw), &plan); err == nil {
					marker.InternalPlan = plan
				} else {
					marker.InternalPlan = raw
				}
			}
		}
	}
	if strings.TrimSpace(marker.OverallGoalUI) == "" {
		return planReviewMarker{}, false
	}
	return marker, true
}

func buildPlanReviewReply(marker planReviewMarker, requireConfirmation bool) string {
	var sb strings.Builder
	sb.WriteString("计划确认\n")
	if marker.OverallGoalUI != "" {
		sb.WriteString("目标: ")
		sb.WriteString(marker.OverallGoalUI)
		sb.WriteString("\n")
	}
	if marker.InternalPlan != nil {
		sb.WriteString("\n计划:\n")
		if data, err := jsonx.MarshalIndent(marker.InternalPlan, "", "  "); err == nil {
			sb.WriteString(string(data))
		} else {
			sb.WriteString(fmt.Sprintf("%v", marker.InternalPlan))
		}
		sb.WriteString("\n")
	}
	if requireConfirmation {
		sb.WriteString("\n请回复 OK 继续，或直接回复修改意见。")
	}
	return strings.TrimSpace(sb.String())
}

func buildPlanFeedbackBlock(pending PlanReviewPending, userFeedback string) string {
	var sb strings.Builder
	sb.WriteString("<plan_feedback>\n")
	sb.WriteString("plan:\n")
	if pending.OverallGoalUI != "" {
		sb.WriteString("goal: ")
		sb.WriteString(strings.TrimSpace(pending.OverallGoalUI))
		sb.WriteString("\n")
	}
	if pending.InternalPlan != nil {
		sb.WriteString("internal_plan: ")
		if data, err := jsonx.MarshalIndent(pending.InternalPlan, "", "  "); err == nil {
			sb.WriteString(string(data))
		} else {
			sb.WriteString(fmt.Sprintf("%v", pending.InternalPlan))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\nuser_feedback:\n")
	sb.WriteString(strings.TrimSpace(userFeedback))
	sb.WriteString("\n\ninstruction: If the feedback changes the plan, call plan() again; otherwise continue with the next step.\n")
	sb.WriteString("</plan_feedback>")
	return strings.TrimSpace(sb.String())
}

func (g *Gateway) sendAttachments(ctx context.Context, chatID, messageID string, result *agent.TaskResult) {
	if result == nil || g.messenger == nil {
		return
	}

	attachments := filterNonA2UIAttachments(result.Attachments)
	if len(attachments) == 0 {
		return
	}

	ctx = shared.WithAllowLocalFetch(ctx)
	ctx = toolports.WithAttachmentContext(ctx, attachments, nil)
	client := artifacts.NewAttachmentHTTPClient(artifacts.AttachmentFetchTimeout, "LarkAttachment")
	maxBytes, allowExts := autoUploadLimits(ctx)

	names := sortedAttachmentNames(attachments)
	for _, name := range names {
		att := attachments[name]
		payload, mediaType, err := artifacts.ResolveAttachmentBytes(ctx, "["+name+"]", client)
		if err != nil {
			g.logger.Warn("Lark attachment %s resolve failed: %v", name, err)
			continue
		}

		fileName := fileNameForAttachment(att, name)
		if !allowExtension(filepath.Ext(fileName), allowExts) {
			g.logger.Warn("Lark attachment %s blocked by allowlist", fileName)
			continue
		}
		if maxBytes > 0 && len(payload) > maxBytes {
			g.logger.Warn("Lark attachment %s exceeds max size %d bytes", fileName, maxBytes)
			continue
		}

		target := replyTarget(messageID, true)

		if isImageAttachment(att, mediaType, name) {
			imageKey, err := g.uploadImage(ctx, payload)
			if err != nil {
				g.logger.Warn("Lark image upload failed (%s): %v", name, err)
				continue
			}
			g.dispatch(ctx, chatID, target, "image", imageContent(imageKey))
			continue
		}

		fileType := larkFileType(fileTypeForAttachment(fileName, mediaType))
		fileKey, err := g.uploadFile(ctx, payload, fileName, fileType)
		if err != nil {
			g.logger.Warn("Lark file upload failed (%s): %v", name, err)
			continue
		}
		g.dispatch(ctx, chatID, target, "file", fileContent(fileKey))
	}
}

func autoUploadLimits(ctx context.Context) (int, []string) {
	cfg := shared.GetAutoUploadConfig(ctx)
	maxBytes := cfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024
	}
	return maxBytes, normalizeExtensions(cfg.AllowExts)
}

func allowExtension(ext string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return false
	}
	for _, item := range allowlist {
		if strings.ToLower(strings.TrimSpace(item)) == ext {
			return true
		}
	}
	return false
}

func filterNonA2UIAttachments(attachments map[string]ports.Attachment) map[string]ports.Attachment {
	if len(attachments) == 0 {
		return nil
	}
	filtered := make(map[string]ports.Attachment, len(attachments))
	for name, att := range attachments {
		if isA2UIAttachment(att) {
			continue
		}
		filtered[name] = att
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func collectAttachmentsFromResult(result *agent.TaskResult) map[string]ports.Attachment {
	if result == nil || len(result.Messages) == 0 {
		return nil
	}

	attachments := make(map[string]ports.Attachment)
	for _, msg := range result.Messages {
		mergeAttachments(attachments, msg.Attachments)
		if len(msg.ToolResults) > 0 {
			for _, res := range msg.ToolResults {
				mergeAttachments(attachments, res.Attachments)
			}
		}
	}
	if len(attachments) == 0 {
		return nil
	}
	return attachments
}

func mergeAttachments(out map[string]ports.Attachment, incoming map[string]ports.Attachment) {
	if len(incoming) == 0 {
		return
	}
	for key, att := range incoming {
		name := strings.TrimSpace(key)
		if name == "" {
			name = strings.TrimSpace(att.Name)
		}
		if name == "" {
			continue
		}
		if _, exists := out[name]; exists {
			continue
		}
		if att.Name == "" {
			att.Name = name
		}
		out[name] = att
	}
}

// buildAttachmentSummary creates a text summary of non-A2UI attachments
// with CDN URLs appended to the reply. This consolidates attachment
// references into the summary message so users see everything in one place.
func buildAttachmentSummary(result *agent.TaskResult) string {
	if result == nil {
		return ""
	}
	attachments := result.Attachments
	if len(attachments) == 0 {
		return ""
	}
	names := sortedAttachmentNames(attachments)
	var lines []string
	for _, name := range names {
		att := attachments[name]
		if isA2UIAttachment(att) {
			continue
		}
		uri := strings.TrimSpace(att.URI)
		if uri == "" || strings.HasPrefix(strings.ToLower(uri), "data:") {
			lines = append(lines, fmt.Sprintf("- %s", name))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", name, uri))
	}
	if len(lines) == 0 {
		return ""
	}
	return "---\n[Attachments]\n" + strings.Join(lines, "\n")
}

func sortedAttachmentNames(attachments map[string]ports.Attachment) []string {
	names := make([]string, 0, len(attachments))
	for name := range attachments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func isA2UIAttachment(att ports.Attachment) bool {
	media := strings.ToLower(strings.TrimSpace(att.MediaType))
	format := strings.ToLower(strings.TrimSpace(att.Format))
	profile := strings.ToLower(strings.TrimSpace(att.PreviewProfile))
	return strings.Contains(media, "a2ui") || format == "a2ui" || strings.Contains(profile, "a2ui")
}

func isImageAttachment(att ports.Attachment, mediaType, name string) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mediaType)), "image/") {
		return true
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(att.MediaType)), "image/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp":
		return true
	default:
		return false
	}
}

func fileNameForAttachment(att ports.Attachment, fallback string) string {
	name := strings.TrimSpace(att.Name)
	if name == "" {
		name = strings.TrimSpace(fallback)
	}
	if name == "" {
		name = "attachment"
	}
	if filepath.Ext(name) == "" {
		if ext := extensionForMediaType(att.MediaType); ext != "" {
			name += ext
		}
	}
	return name
}

// larkSupportedFileTypes lists the file_type values accepted by the Lark
// im/v1/files upload API. Any extension not in this set must be sent as "stream".
var larkSupportedFileTypes = map[string]bool{
	"opus": true, "mp4": true, "pdf": true,
	"doc": true, "xls": true, "ppt": true,
	"stream": true,
}

// larkFileType maps a raw file extension to a Lark-compatible file_type value.
func larkFileType(ext string) string {
	lower := strings.ToLower(strings.TrimSpace(ext))
	if larkSupportedFileTypes[lower] {
		return lower
	}
	return "stream"
}

func fileTypeForAttachment(name, mediaType string) string {
	if ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."); ext != "" {
		return ext
	}
	if ext := strings.TrimPrefix(extensionForMediaType(mediaType), "."); ext != "" {
		return ext
	}
	return "bin"
}

func extensionForMediaType(mediaType string) string {
	trimmed := strings.TrimSpace(mediaType)
	if trimmed == "" {
		return ""
	}
	exts, err := mime.ExtensionsByType(trimmed)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return exts[0]
}

func (g *Gateway) uploadImage(ctx context.Context, payload []byte) (string, error) {
	if g.messenger == nil {
		return "", fmt.Errorf("lark messenger not initialized")
	}
	return g.messenger.UploadImage(ctx, payload)
}

func (g *Gateway) uploadFile(ctx context.Context, payload []byte, fileName, fileType string) (string, error) {
	if g.messenger == nil {
		return "", fmt.Errorf("lark messenger not initialized")
	}
	return g.messenger.UploadFile(ctx, payload, fileName, fileType)
}

// extractSenderID extracts sender identity from a Lark message event, preferring
// open_id and falling back to user_id/union_id.
func extractSenderID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Sender == nil || event.Event.Sender.SenderId == nil {
		return ""
	}
	id := strings.TrimSpace(deref(event.Event.Sender.SenderId.OpenId))
	if id != "" {
		return id
	}
	id = strings.TrimSpace(deref(event.Event.Sender.SenderId.UserId))
	if id != "" {
		return id
	}
	return strings.TrimSpace(deref(event.Event.Sender.SenderId.UnionId))
}

// extractMentions extracts mentioned user IDs from a Lark message event.
func extractMentions(event *larkim.P2MessageReceiveV1) []string {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}
	mentions := event.Event.Message.Mentions
	if len(mentions) == 0 {
		return nil
	}
	var ids []string
	for _, m := range mentions {
		if m == nil || m.Id == nil {
			continue
		}
		id := deref(m.Id.OpenId)
		if id == "" {
			id = deref(m.Id.UserId)
		}
		if id == "" {
			id = deref(m.Id.UnionId)
		}
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// isBotSender checks if the message sender is a bot (app).
func isBotSender(event *larkim.P2MessageReceiveV1) bool {
	if event == nil || event.Event == nil || event.Event.Sender == nil {
		return false
	}
	return deref(event.Event.Sender.SenderType) == "app"
}

// deref safely dereferences a string pointer.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func normalizeExtensions(exts []string) []string {
	if len(exts) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(exts))
	normalized := make([]string, 0, len(exts))
	for _, raw := range exts {
		trimmed := strings.TrimSpace(strings.ToLower(raw))
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + trimmed
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}
