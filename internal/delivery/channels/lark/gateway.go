package lark

import (
	"context"
	"crypto/sha1"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/app/subscription"
	toolcontext "alex/internal/app/toolcontext"
	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	runtimeconfig "alex/internal/shared/config"
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
	messageDedupCacheSize      = 2048
	messageDedupTTL            = 10 * time.Minute
	chatSessionBindingChannel  = "lark"
	defaultRecentChatMaxRounds = 5
	defaultActiveSlotTTL       = 6 * time.Hour
	defaultActiveSlotMax       = 2048
	defaultRelayTTL            = 30 * time.Minute
	defaultRelayMaxChats       = 2048
	defaultRelayMaxPerChat     = 64
	defaultAIChatSessionTTL    = 45 * time.Minute
	defaultStateCleanupEvery   = 5 * time.Minute
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
	lastTouched    time.Time
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
	oauth              toolcontext.LarkOAuthService
	llmSelections      *subscription.SelectionStore
	llmResolver        *subscription.SelectionResolver
	cliCredsLoader     func() runtimeconfig.CLICredentials
	llamaResolver      func(context.Context) (subscription.LlamaServerTarget, bool)
	taskStore          TaskStore
	chatSessionStore   ChatSessionBindingStore
	noticeState        *noticeStateStore
	activeSlots        sync.Map           // chatID → *sessionSlot
	pendingInputRelays sync.Map           // chatID → *pendingRelayQueue
	aiCoordinator      *AIChatCoordinator // coordinates multi-bot chat sessions
	taskWG             sync.WaitGroup     // tracks running task goroutines (for tests)
	cleanupMu          sync.Mutex
	cleanupCancel      context.CancelFunc
	cleanupWG          sync.WaitGroup
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
	if cfg.ActiveSlotTTL <= 0 {
		cfg.ActiveSlotTTL = defaultActiveSlotTTL
	}
	if cfg.ActiveSlotMaxEntries <= 0 {
		cfg.ActiveSlotMaxEntries = defaultActiveSlotMax
	}
	if cfg.PendingInputRelayTTL <= 0 {
		cfg.PendingInputRelayTTL = defaultRelayTTL
	}
	if cfg.PendingInputRelayMaxChats <= 0 {
		cfg.PendingInputRelayMaxChats = defaultRelayMaxChats
	}
	if cfg.PendingInputRelayMaxPerChat <= 0 {
		cfg.PendingInputRelayMaxPerChat = defaultRelayMaxPerChat
	}
	if cfg.AIChatSessionTTL <= 0 {
		cfg.AIChatSessionTTL = defaultAIChatSessionTTL
	}
	if cfg.StateCleanupInterval <= 0 {
		cfg.StateCleanupInterval = defaultStateCleanupEvery
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
func (g *Gateway) SetOAuthService(svc toolcontext.LarkOAuthService) {
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

// SetChatSessionBindingStore configures persistent chat->session bindings.
func (g *Gateway) SetChatSessionBindingStore(store ChatSessionBindingStore) {
	if g == nil {
		return
	}
	g.chatSessionStore = store
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

// NoticeLoader returns a loader that reads the notice binding's chat ID.
// This is used by the hooks bridge to resolve where to send hook events.
// Returns nil if the notice state store is not available.
func (g *Gateway) NoticeLoader() func() (string, bool, error) {
	if g == nil || g.noticeState == nil {
		return nil
	}
	return func() (string, bool, error) {
		binding, ok, err := g.noticeState.Load()
		if err != nil || !ok {
			return "", ok, err
		}
		return binding.ChatID, true, nil
	}
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

// PersistBridgeMeta implements agent.BridgeMetaPersister. It saves bridge
// subprocess metadata to the task store for resilience (orphan adoption on restart).
func (g *Gateway) PersistBridgeMeta(ctx context.Context, taskID string, info any) {
	if g == nil || g.taskStore == nil {
		return
	}
	type bridgeMetaSetter interface {
		SetBridgeMeta(ctx context.Context, taskID string, info any) error
	}
	if setter, ok := g.taskStore.(bridgeMetaSetter); ok {
		storeCtx := context.WithoutCancel(ctx)
		if err := setter.SetBridgeMeta(storeCtx, taskID, info); err != nil {
			g.logger.Warn("BridgeMetaPersister: SetBridgeMeta failed for %s: %v", taskID, err)
		}
	}
}

// Start creates the Lark SDK client, event dispatcher, and WebSocket client, then blocks.
func (g *Gateway) Start(ctx context.Context) error {
	if !g.cfg.Enabled {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	g.setCleanupCancel(cancel)
	g.startStateCleanupLoop(runCtx)

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
	err := g.wsClient.Start(runCtx)
	g.stopStateCleanupLoop()
	return err
}

// Stop releases resources. The WebSocket client does not expose a Stop method;
// cancelling the context passed to Start is the primary shutdown mechanism.
func (g *Gateway) Stop() {
	g.stopStateCleanupLoop()
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
	slot.lastTouched = g.currentTime()
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

	// Handle /new outside of a running task.
	if trimmedContent == "/new" {
		g.handleNewSessionCommand(slot, msg) // releases slot.mu
		return nil
	}

	// Handle /reset outside of a running task.
	if trimmedContent == "/reset" {
		g.handleResetCommand(slot, msg) // releases slot.mu
		return nil
	}
	if strings.HasPrefix(trimmedContent, "/model") || strings.HasPrefix(trimmedContent, "/models") {
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
	sessionID, isResume := g.resolveSessionForNewTask(ctx, msg.chatID, slot)
	inputCh := make(chan agent.UserInput, 16)
	slot.phase = slotRunning
	slot.inputCh = inputCh
	slot.sessionID = sessionID
	slot.lastSessionID = sessionID
	slot.lastTouched = g.currentTime()
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
		slot.lastTouched = g.currentTime()
		slot.mu.Unlock()
		if awaitingInput {
			g.drainAndReprocess(inputCh, msg.chatID, msg.chatType)
		} else {
			g.discardPendingInputs(inputCh, msg.chatID)
		}
	}()

	return nil
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
