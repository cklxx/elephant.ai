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
	larkcards "alex/internal/infra/lark/cards"
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
	maxAttachmentCardImgs = 3
)

// AgentExecutor is an alias for the shared channel executor interface.
type AgentExecutor = channels.AgentExecutor

// sessionSlot tracks whether a task is active for a given chat and holds
// the user input channel used to inject follow-up messages into a running
// ReAct loop, plus the pending session state for await-user-input handoffs.
type sessionSlot struct {
	mu            sync.Mutex
	inputCh       chan agent.UserInput // non-nil while a task is active
	sessionID     string
	lastSessionID string
	awaitingInput bool
}

// Gateway bridges Lark bot messages into the agent runtime.
type Gateway struct {
	channels.BaseGateway
	cfg             Config
	agent           AgentExecutor
	logger          logging.Logger
	client          *lark.Client
	wsClient        *larkws.Client
	messenger       LarkMessenger
	eventListener   agent.EventListener
	emojiPicker     *emojiPicker
	dedupMu         sync.Mutex
	dedupCache      *lru.Cache[string, time.Time]
	now             func() time.Time
	planReviewStore PlanReviewStore
	oauth           *larkoauth.Service
	llmSelections   *subscription.SelectionStore
	llmResolver     *subscription.SelectionResolver
	activeSlots     sync.Map           // chatID → *sessionSlot
	aiCoordinator   *AIChatCoordinator // coordinates multi-bot chat sessions
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
	if cfg.CardsEnabled && !cfg.CardsPlanReview && !cfg.CardsResults && !cfg.CardsErrors {
		cfg.CardsPlanReview = true
		cfg.CardsResults = true
		cfg.CardsErrors = true
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
		llmResolver: subscription.NewSelectionResolver(func() runtimeconfig.CLICredentials {
			return runtimeconfig.LoadCLICredentials()
		}),
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

// SetAIChatCoordinator configures the AI chat coordinator for multi-bot conversations.
func (g *Gateway) SetAIChatCoordinator(coordinator *AIChatCoordinator) {
	if g == nil {
		return
	}
	g.aiCoordinator = coordinator
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

	// Build the event dispatcher and register the message handler.
	eventDispatcher := dispatcher.NewEventDispatcher("", "")
	eventDispatcher.OnP2MessageReceiveV1(g.handleMessage)

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

	// If a task is already running for this chat, inject the new message
	// into the running ReAct loop instead of starting a new task.
	if slot.inputCh != nil {
		ch := slot.inputCh
		activeSessionID := slot.sessionID
		slot.mu.Unlock()
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

	// Resolve session ID and acquire the slot for a new task.
	sessionID := slot.sessionID
	if !slot.awaitingInput || sessionID == "" {
		sessionID = g.newSessionID()
	}
	inputCh := make(chan agent.UserInput, 16)
	slot.inputCh = inputCh
	slot.sessionID = sessionID
	slot.lastSessionID = sessionID
	slot.awaitingInput = false
	slot.mu.Unlock()

	awaitingInput := false
	defer func() {
		slot.mu.Lock()
		slot.inputCh = nil
		if awaitingInput {
			slot.awaitingInput = true
			slot.lastSessionID = slot.sessionID
		} else {
			slot.awaitingInput = false
			slot.sessionID = ""
		}
		slot.mu.Unlock()
		g.drainAndReprocess(inputCh, msg.chatID, msg.chatType)
	}()

	awaitingInput = g.runNewTask(msg, sessionID, inputCh)

	// After responding, advance the turn in AI chat session if applicable
	if g.aiCoordinator != nil && !awaitingInput && msg.aiChatSessionActive {
		nextBot, shouldContinue := g.aiCoordinator.AdvanceTurn(msg.chatID, g.cfg.AppID)
		if shouldContinue && nextBot != "" {
			g.logger.Info("AI chat: advanced turn, next bot=%s", nextBot)
			// Trigger the next bot to respond by sending a mention
			go g.triggerNextBotResponse(context.Background(), msg.chatID, nextBot)
		}
	}

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
		g.logger.Debug("Lark duplicate message skipped: %s", messageID)
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
	slot.awaitingInput = false
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

// runNewTask executes a full task lifecycle: context setup, session ensure,
// listener wiring, content preparation, execution, and reply dispatch.
// Returns true if the result indicates await_user_input.
func (g *Gateway) runNewTask(msg *incomingMessage, sessionID string, inputCh chan agent.UserInput) bool {
	execCtx := g.buildExecContext(msg, sessionID, inputCh)

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

	awaitUserInput := sessionHasAwaitFlag(session)

	execCtx = channels.ApplyPresets(execCtx, g.cfg.BaseConfig)
	execCtx, cancelTimeout := channels.ApplyTimeout(execCtx, g.cfg.BaseConfig)
	defer cancelTimeout()

	awaitTracker := &awaitQuestionTracker{}
	listener, cleanupListeners := g.setupListeners(execCtx, msg, awaitTracker)
	defer cleanupListeners()
	execCtx = shared.WithParentListener(execCtx, listener)

	taskContent := g.prepareTaskContent(execCtx, session, msg, inputCh, sessionID, awaitUserInput)

	startEmoji, endEmoji := g.pickReactionEmojis()
	if msg.messageID != "" && startEmoji != "" {
		go g.addReaction(execCtx, msg.messageID, "Get")
	}

	result, execErr := g.agent.ExecuteTask(execCtx, taskContent, sessionID, listener)

	if msg.messageID != "" && endEmoji != "" {
		go g.addReaction(execCtx, msg.messageID, endEmoji)
	}

	g.dispatchResult(execCtx, msg, result, execErr, awaitTracker)

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
// await_user_input state.
func sessionHasAwaitFlag(session *storage.Session) bool {
	if session == nil || session.Metadata == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(session.Metadata["await_user_input"]), "true")
}

// setupListeners configures the event listener chain (progress, plan clarify)
// and returns the composed listener plus a cleanup function.
func (g *Gateway) setupListeners(execCtx context.Context, msg *incomingMessage, awaitTracker *awaitQuestionTracker) (agent.EventListener, func()) {
	listener := g.eventListener
	if listener == nil {
		listener = agent.NoopEventListener{}
	}

	var cleanups []func()

	if g.cfg.ShowToolProgress {
		sender := &larkProgressSender{gateway: g, chatID: msg.chatID, messageID: msg.messageID, isGroup: msg.isGroup}
		progressLn := newProgressListener(execCtx, listener, sender, g.logger)
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
	return listener, cleanup
}

// prepareTaskContent resolves plan review state, seeds pending user input, and
// fetches auto chat context to build the final task content string.
func (g *Gateway) prepareTaskContent(execCtx context.Context, session *storage.Session, msg *incomingMessage, inputCh chan agent.UserInput, sessionID string, awaitUserInput bool) string {
	taskContent := msg.content
	hasPending := false

	if g.cfg.PlanReviewEnabled {
		pending, ok := g.loadPlanReviewPending(execCtx, session, msg.senderID, msg.chatID)
		hasPending = ok
		if hasPending {
			taskContent = buildPlanFeedbackBlock(pending, msg.content)
			if g.planReviewStore != nil {
				if err := g.planReviewStore.ClearPending(execCtx, msg.senderID, msg.chatID); err != nil {
					g.logger.Warn("Lark plan review pending clear failed: %v", err)
				}
			}
		}
	}

	if awaitUserInput && !hasPending {
		select {
		case inputCh <- agent.UserInput{Content: msg.content, SenderID: msg.senderID, MessageID: msg.messageID}:
			g.logger.Info("Seeded pending user input for session %s", sessionID)
		default:
			g.logger.Warn("Pending user input channel full for session %s; message dropped", sessionID)
		}
		taskContent = ""
	}

	if taskContent != "" && g.messenger != nil && msg.isGroup {
		pageSize := g.cfg.AutoChatContextSize
		if pageSize <= 0 {
			pageSize = 20
		}
		if chatHistory, err := g.fetchRecentChatMessages(execCtx, msg.chatID, pageSize); err != nil {
			g.logger.Warn("Lark auto chat context fetch failed: %v", err)
		} else if chatHistory != "" {
			if hasPending {
				taskContent = taskContent + "\n\n[近期对话]\n" + chatHistory
			} else {
				taskContent = "[近期对话]\n" + chatHistory + "\n\n" + taskContent
			}
		}
	}

	return taskContent
}

// dispatchResult builds the reply from the execution result and sends it to
// the Lark chat, including any attachments.
func (g *Gateway) dispatchResult(execCtx context.Context, msg *incomingMessage, result *agent.TaskResult, execErr error, awaitTracker *awaitQuestionTracker) {
	isAwait := execErr == nil && isResultAwaitingInput(result)

	reply := ""
	replyMsgType := "text"
	replyContent := ""
	attachmentCardSent := false

	attachments := map[string]ports.Attachment(nil)
	if result != nil && len(result.Attachments) > 0 {
		attachments = filterNonA2UIAttachments(result.Attachments)
	}
	hasAttachments := len(attachments) > 0

	if isAwait && g.cfg.PlanReviewEnabled {
		reply, replyMsgType, replyContent = g.buildPlanReviewReplyContent(execCtx, msg, result)
	}

	skipReply := isAwait && awaitTracker.Sent()

	if replyContent == "" && !skipReply {
		if reply == "" && isAwait {
			if question, ok := agent.ExtractAwaitUserInputQuestion(result.Messages); ok {
				reply = question
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
		// Skip cards when the message is from another bot to avoid card loops in bot-to-bot chats.
		if !msg.isFromBot {
			if g.cfg.CardsEnabled && execErr == nil && g.cfg.CardsResults && hasAttachments && !isAwait {
				if card, err := g.buildAttachmentCard(execCtx, reply, result); err == nil {
					replyMsgType = "interactive"
					replyContent = card
					attachmentCardSent = true
				} else {
					g.logger.Warn("Lark attachment card build failed: %v", err)
				}
			}
			if !attachmentCardSent && g.cfg.CardsEnabled && ((execErr != nil && g.cfg.CardsErrors) || (execErr == nil && g.cfg.CardsResults)) {
				if card, err := g.buildCardReply(reply, result, execErr); err == nil {
					replyMsgType = "interactive"
					replyContent = card
				} else {
					g.logger.Warn("Lark card reply build failed: %v", err)
				}
			}
		}
		if replyContent == "" {
			if summary := buildAttachmentSummary(result); summary != "" {
				reply += "\n\n" + summary
			}
			replyContent = textContent(reply)
		}
	}

	if !skipReply {
		g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), replyMsgType, replyContent)
		if !attachmentCardSent {
			g.sendAttachments(execCtx, msg.chatID, msg.messageID, result)
		}
	}
}

// buildPlanReviewReplyContent handles plan review marker extraction, card
// building, pending store save, and returns the reply text, message type,
// and content payload.
func (g *Gateway) buildPlanReviewReplyContent(execCtx context.Context, msg *incomingMessage, result *agent.TaskResult) (reply, msgType, content string) {
	marker, ok := extractPlanReviewMarker(result.Messages)
	if !ok {
		return "", "", ""
	}

	msgType = "text"
	if g.cfg.CardsEnabled && g.cfg.CardsPlanReview {
		if card, err := g.buildPlanReviewCard(marker); err == nil {
			msgType = "interactive"
			content = card
		} else {
			g.logger.Warn("Lark plan review card build failed: %v", err)
		}
	}
	if content == "" {
		reply = buildPlanReviewReply(marker, g.cfg.PlanReviewRequireConfirmation)
	}

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

	return reply, msgType, content
}

// drainAndReprocess drains any remaining messages from the input channel after
// a task finishes and reprocesses each as a new task. This handles messages that
// arrived between the last ReAct iteration drain and the task completion.
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
	for _, msg := range remaining {
		go g.reprocessMessage(chatID, chatType, msg)
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

func (g *Gateway) buildAttachmentCard(ctx context.Context, reply string, result *agent.TaskResult) (string, error) {
	summary := strings.TrimSpace(reply)
	if summary == "" {
		return "", fmt.Errorf("empty reply")
	}
	if len(summary) > maxCardReplyChars {
		return "", fmt.Errorf("reply too long for card")
	}
	summary = truncateCardText(summary, maxCardReplyChars)

	assets, err := g.uploadCardAttachments(ctx, result)
	if err != nil {
		return "", err
	}
	if len(assets) == 0 {
		return "", fmt.Errorf("no attachments to display")
	}

	return larkcards.AttachmentCard(larkcards.AttachmentCardParams{
		Title:      "任务完成",
		Summary:    summary,
		Footer:     "点击按钮发送附件。",
		TitleColor: "green",
		Assets:     assets,
	})
}

func (g *Gateway) uploadCardAttachments(ctx context.Context, result *agent.TaskResult) ([]larkcards.AttachmentAsset, error) {
	if result == nil || g.messenger == nil {
		return nil, fmt.Errorf("attachments unavailable")
	}
	attachments := filterNonA2UIAttachments(result.Attachments)
	if len(attachments) == 0 {
		return nil, fmt.Errorf("attachments unavailable")
	}

	ctx = shared.WithAllowLocalFetch(ctx)
	ctx = toolports.WithAttachmentContext(ctx, attachments, nil)
	client := artifacts.NewAttachmentHTTPClient(artifacts.AttachmentFetchTimeout, "LarkAttachmentCard")
	maxBytes, allowExts := autoUploadLimits(ctx)

	names := sortedAttachmentNames(attachments)
	assets := make([]larkcards.AttachmentAsset, 0, len(names))
	previewed := 0
	for _, name := range names {
		att := attachments[name]
		payload, mediaType, err := artifacts.ResolveAttachmentBytes(ctx, "["+name+"]", client)
		if err != nil {
			return nil, err
		}
		if maxBytes > 0 && len(payload) > maxBytes {
			return nil, fmt.Errorf("attachment %s exceeds max size %d bytes", name, maxBytes)
		}

		fileName := fileNameForAttachment(att, name)
		if !allowExtension(filepath.Ext(fileName), allowExts) {
			return nil, fmt.Errorf("attachment %s blocked by allowlist", fileName)
		}

		asset := larkcards.AttachmentAsset{
			Name:      fileName,
			FileName:  fileName,
			ButtonTag: "attachment_send",
		}

		if isImageAttachment(att, mediaType, name) {
			imageKey, err := g.uploadImage(ctx, payload)
			if err != nil {
				return nil, err
			}
			asset.Kind = "image"
			asset.ImageKey = imageKey
			if previewed < maxAttachmentCardImgs {
				asset.ShowPreview = true
				previewed++
			}
		} else {
			fileType := larkFileType(fileTypeForAttachment(fileName, mediaType))
			fileKey, err := g.uploadFile(ctx, payload, fileName, fileType)
			if err != nil {
				return nil, err
			}
			asset.Kind = "file"
			asset.FileKey = fileKey
		}

		assets = append(assets, asset)
	}

	return assets, nil
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

// extractSenderID extracts the sender open_id from a Lark message event.
func extractSenderID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Sender == nil || event.Event.Sender.SenderId == nil {
		return ""
	}
	return deref(event.Event.Sender.SenderId.OpenId)
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

// triggerNextBotResponse triggers the next bot in an AI chat session to respond.
func (g *Gateway) triggerNextBotResponse(ctx context.Context, chatID, chatType string) {
	if g.aiCoordinator == nil {
		return
	}

	nextBotID, shouldContinue := g.aiCoordinator.AdvanceTurn(chatID, g.cfg.AppID)
	if !shouldContinue {
		return
	}

	g.logger.Info("AI chat: triggering next bot %s in chat %s", nextBotID, chatID)

	// Send a message mentioning the next bot to trigger its response
	triggerMsg := fmt.Sprintf("@%s(%s) 轮到你了，继续我们的讨论吧", nextBotID, nextBotID)
	content := textContent(triggerMsg)

	replyTo := ""
	if chatType != "p2p" {
		// In group chats, don't reply to a specific message to keep the flow natural
		replyTo = ""
	}

	g.dispatch(ctx, chatID, replyTo, "text", content)
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
