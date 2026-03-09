package lark

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/app/subscription"
	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	portsllm "alex/internal/domain/agent/ports/llm"
	larkoauth "alex/internal/infra/lark/oauth"
	builtinshared "alex/internal/infra/tools/builtin/shared"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
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
	defaultToolFailureAbortN   = 6
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
	mu         sync.Mutex
	phase      slotPhase
	inputCh    chan agent.UserInput // non-nil only when phase == slotRunning
	taskCancel context.CancelFunc   // cancels the currently running task (phase == slotRunning)
	taskToken  uint64
	// intentionalCancelToken marks the task token cancelled by an explicit
	// control action (/stop, /new, inject timeout cleanup). These cancellations
	// should not emit a failure reply to Lark.
	intentionalCancelToken uint64
	sessionID              string
	lastSessionID          string
	pendingOptions         []string // options awaiting numeric reply
	lastTouched            time.Time
}

// Gateway bridges Lark bot messages into the agent runtime.
type Gateway struct {
	channels.BaseGateway
	cfg                 Config
	agent               AgentExecutor
	logger              logging.Logger
	client              *lark.Client
	wsClient            *larkws.Client
	messenger           LarkMessenger
	eventListener       agent.EventListener
	emojiPicker         *emojiPicker
	dedupMu             sync.Mutex
	dedupCache          *lru.Cache[string, time.Time]
	now                 func() time.Time
	planReviewStore     PlanReviewStore
	oauth               builtinshared.LarkOAuthService
	llmSelections       *subscription.SelectionStore
	llmResolver         *subscription.SelectionResolver
	cliCredsLoader      func() runtimeconfig.CLICredentials
	llamaResolver       func(context.Context) (subscription.LlamaServerTarget, bool)
	llmFactory          portsllm.LLMClientFactory // optional; for lightweight LLM calls (auto-reply)
	llmProfile          runtimeconfig.LLMProfile  // shared runtime LLM profile for auto-reply
	taskStore           TaskStore
	chatSessionStore    ChatSessionBindingStore
	deliveryOutboxStore DeliveryOutboxStore
	noticeState         *noticeStateStore
	activeSlots         sync.Map           // chatID → *sessionSlot
	pendingInputRelays  sync.Map           // chatID → *pendingRelayQueue
	aiCoordinator       *AIChatCoordinator // coordinates multi-bot chat sessions
	autoAuth            *AutoAuth          // in-message OAuth device flow
	taskWG              sync.WaitGroup     // tracks running task goroutines (for tests)
	cleanupMu           sync.Mutex
	cleanupCancel       context.CancelFunc
	cleanupWG           sync.WaitGroup
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
	if utils.IsBlank(cfg.AppID) || utils.IsBlank(cfg.AppSecret) {
		return nil, fmt.Errorf("lark gateway requires app_id and app_secret")
	}
	cfg.SessionPrefix = strings.TrimSpace(cfg.SessionPrefix)
	if cfg.SessionPrefix == "" {
		cfg.SessionPrefix = "lark"
	}
	cfg.ToolPreset = utils.TrimLower(cfg.ToolPreset)
	if cfg.ToolPreset == "" {
		cfg.ToolPreset = "full"
	}
	if cfg.BackgroundProgressEnabled == nil {
		enabled := true
		cfg.BackgroundProgressEnabled = &enabled
	}
	if cfg.RephraseEnabled == nil {
		enabled := true
		cfg.RephraseEnabled = &enabled
	}
	if cfg.SlowProgressSummaryEnabled == nil {
		enabled := true
		cfg.SlowProgressSummaryEnabled = &enabled
	}
	if cfg.SlowProgressSummaryDelay <= 0 {
		cfg.SlowProgressSummaryDelay = defaultSlowProgressSummaryDelay
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
	if cfg.ToolFailureAbortThreshold <= 0 {
		cfg.ToolFailureAbortThreshold = defaultToolFailureAbortN
	}
	cfg.DeliveryMode = string(normalizeDeliveryMode(cfg.DeliveryMode))
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
func (g *Gateway) SetEventListener(listener agent.EventListener) { g.eventListener = listener }

// SetPlanReviewStore configures the pending plan review store.
func (g *Gateway) SetPlanReviewStore(store PlanReviewStore) { g.planReviewStore = store }

// SetOAuthService configures the Lark user OAuth service used for user-scoped API calls.
func (g *Gateway) SetOAuthService(svc builtinshared.LarkOAuthService) { g.oauth = svc }

// SetAutoAuth configures automatic in-message OAuth authorization.
func (g *Gateway) SetAutoAuth(aa *AutoAuth) { g.autoAuth = aa }

// EnableAutoAuth creates and configures an AutoAuth instance that uses the
// gateway's messenger. The messenger is resolved lazily at first use since
// it is initialized during Start().
func (g *Gateway) EnableAutoAuth(oauthSvc *larkoauth.Service, logger logging.Logger) {
	g.autoAuth = NewAutoAuth(oauthSvc, &lazyMessenger{g: g}, logger)
}

// SetMessenger replaces the default SDK messenger with a custom implementation.
func (g *Gateway) SetMessenger(m LarkMessenger) { g.messenger = wrapInjectCaptureHub(m) }

// SetTaskStore configures the task persistence store.
func (g *Gateway) SetTaskStore(store TaskStore) { g.taskStore = store }

// SetLLMFactory configures an optional LLM client factory and shared profile
// for lightweight calls such as auto-reply generation during InjectMessageSync.
func (g *Gateway) SetLLMFactory(factory portsllm.LLMClientFactory, profile runtimeconfig.LLMProfile) {
	g.llmFactory = factory
	g.llmProfile = profile
}

// SetChatSessionBindingStore configures persistent chat->session bindings.
func (g *Gateway) SetChatSessionBindingStore(store ChatSessionBindingStore) {
	g.chatSessionStore = store
}

// SetDeliveryOutboxStore configures persistent terminal delivery intents.
func (g *Gateway) SetDeliveryOutboxStore(store DeliveryOutboxStore) {
	g.deliveryOutboxStore = store
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
func (g *Gateway) SetAIChatCoordinator(coordinator *AIChatCoordinator) { g.aiCoordinator = coordinator }

// NotifyCompletion implements agent.BackgroundCompletionNotifier. It writes
// the final task status directly to TaskStore, ensuring persistence even when
// the event listener chain is broken (e.g. SerializingEventListener idle timeout).
func (g *Gateway) NotifyCompletion(ctx context.Context, taskID, status, answer, errText, mergeStatus string, tokensUsed int) {
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
	if mergeStatus != "" {
		opts = append(opts, WithMergeStatus(mergeStatus))
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
	g.messenger = wrapInjectCaptureHub(g.messenger)
	g.startDeliveryWorker(runCtx)

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

// NotifyRunningTaskInterruptions cancels in-flight foreground tasks and sends
// a visible interruption notice to each affected chat.
func (g *Gateway) NotifyRunningTaskInterruptions(notice string) int {
	if g == nil {
		return 0
	}
	notice = strings.TrimSpace(notice)
	if notice == "" {
		notice = "服务正在重启，当前执行已中断。请稍后重试。"
	}

	type runningTarget struct {
		chatID string
		cancel context.CancelFunc
	}
	targets := make([]runningTarget, 0, 4)

	g.activeSlots.Range(func(key, value any) bool {
		chatID, ok := key.(string)
		if !ok {
			return true
		}
		slot, ok := value.(*sessionSlot)
		if !ok || slot == nil {
			return true
		}

		slot.mu.Lock()
		running := slot.phase == slotRunning && slot.taskCancel != nil
		if running {
			slot.intentionalCancelToken = slot.taskToken
			targets = append(targets, runningTarget{
				chatID: chatID,
				cancel: slot.taskCancel,
			})
		}
		slot.mu.Unlock()
		return true
	})

	if len(targets) == 0 {
		return 0
	}

	notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, target := range targets {
		target.cancel()
		g.dispatch(notifyCtx, target.chatID, "", "text", textContent(notice))
	}
	return len(targets)
}

// WaitForTasks blocks until all in-flight task goroutines complete.
// Intended for test synchronization only.
func (g *Gateway) WaitForTasks() {
	g.taskWG.Wait()
}

func wrapInjectCaptureHub(m LarkMessenger) LarkMessenger {
	if m == nil {
		return nil
	}
	if _, ok := m.(*injectCaptureHub); ok {
		return m
	}
	return newInjectCaptureHub(m)
}

func (g *Gateway) setCleanupCancel(cancel context.CancelFunc) {
	g.cleanupMu.Lock()
	if g.cleanupCancel != nil {
		g.cleanupCancel()
	}
	g.cleanupCancel = cancel
	g.cleanupMu.Unlock()
}

func (g *Gateway) stopStateCleanupLoop() {
	g.cleanupMu.Lock()
	cancel := g.cleanupCancel
	g.cleanupCancel = nil
	g.cleanupMu.Unlock()
	if cancel != nil {
		cancel()
	}
	g.cleanupWG.Wait()
}

func (g *Gateway) startStateCleanupLoop(ctx context.Context) {
	interval := g.cfg.StateCleanupInterval
	if interval <= 0 {
		return
	}
	g.cleanupWG.Add(1)
	go func() {
		defer g.cleanupWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				g.cleanupRuntimeState()
			}
		}
	}()
}

func (g *Gateway) cleanupRuntimeState() {
	now := g.currentTime()
	trimmedSlots := g.pruneActiveSlots(now)
	trimmedRelays := g.prunePendingInputRelays(now)

	trimmedAISessions := 0
	if g.aiCoordinator != nil && g.cfg.AIChatSessionTTL > 0 {
		trimmedAISessions = g.aiCoordinator.CleanupExpiredSessions(g.cfg.AIChatSessionTTL)
	}
	if trimmedSlots > 0 || trimmedRelays > 0 || trimmedAISessions > 0 {
		g.logger.Info(
			"Lark state cleanup: removed_slots=%d removed_relays=%d removed_ai_chat_sessions=%d",
			trimmedSlots, trimmedRelays, trimmedAISessions,
		)
	}
}

func (g *Gateway) pruneActiveSlots(now time.Time) int {
	ttl := g.cfg.ActiveSlotTTL
	maxEntries := g.cfg.ActiveSlotMaxEntries

	type slotMeta struct {
		chatID      string
		lastTouched time.Time
	}
	var idle []slotMeta
	total := 0
	removed := 0

	g.activeSlots.Range(func(key, value any) bool {
		chatID, ok := key.(string)
		if !ok {
			g.activeSlots.Delete(key)
			removed++
			return true
		}
		slot, ok := value.(*sessionSlot)
		if !ok || slot == nil {
			g.activeSlots.Delete(chatID)
			removed++
			return true
		}
		total++

		slot.mu.Lock()
		phase := slot.phase
		lastTouched := slot.lastTouched
		slot.mu.Unlock()

		if phase == slotRunning {
			return true
		}
		if ttl > 0 && !lastTouched.IsZero() && now.Sub(lastTouched) > ttl {
			g.activeSlots.Delete(chatID)
			removed++
			return true
		}
		idle = append(idle, slotMeta{chatID: chatID, lastTouched: lastTouched})
		return true
	})

	current := total - removed
	if maxEntries <= 0 || current <= maxEntries {
		return removed
	}

	sort.Slice(idle, func(i, j int) bool {
		if idle[i].lastTouched.Equal(idle[j].lastTouched) {
			return idle[i].chatID < idle[j].chatID
		}
		return idle[i].lastTouched.Before(idle[j].lastTouched)
	})

	need := current - maxEntries
	for i := 0; i < len(idle) && need > 0; i++ {
		g.activeSlots.Delete(idle[i].chatID)
		removed++
		need--
	}
	return removed
}

func (g *Gateway) prunePendingInputRelays(now time.Time) int {
	maxChats := g.cfg.PendingInputRelayMaxChats
	maxPerChat := g.cfg.PendingInputRelayMaxPerChat

	type queueMeta struct {
		chatID   string
		oldestAt int64
	}
	totalChats := 0
	removed := 0
	var metas []queueMeta

	g.pendingInputRelays.Range(func(key, value any) bool {
		chatID, ok := key.(string)
		if !ok {
			g.pendingInputRelays.Delete(key)
			removed++
			return true
		}
		queue, ok := value.(*pendingRelayQueue)
		if !ok || queue == nil {
			g.pendingInputRelays.Delete(chatID)
			removed++
			return true
		}

		removed += queue.PruneExpired(now)
		if maxPerChat > 0 {
			removed += queue.TrimToMax(maxPerChat)
		}
		if queue.Len() == 0 {
			g.pendingInputRelays.Delete(chatID)
			removed++
			return true
		}

		totalChats++
		metas = append(metas, queueMeta{
			chatID:   chatID,
			oldestAt: queue.OldestCreatedAtUnixNano(),
		})
		return true
	})

	if maxChats <= 0 || totalChats <= maxChats {
		return removed
	}
	sort.Slice(metas, func(i, j int) bool {
		if metas[i].oldestAt == metas[j].oldestAt {
			return metas[i].chatID < metas[j].chatID
		}
		return metas[i].oldestAt < metas[j].oldestAt
	})
	need := totalChats - maxChats
	for i := 0; i < len(metas) && need > 0; i++ {
		g.pendingInputRelays.Delete(metas[i].chatID)
		removed++
		need--
	}
	return removed
}

func (g *Gateway) currentTime() time.Time {
	nowFn := g.now
	if nowFn == nil {
		nowFn = time.Now
	}
	return nowFn()
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
	if g.isStopCommand(trimmedContent) {
		g.handleStopCommand(slot, msg) // releases slot.mu
		return nil
	}

	// Handle /new and /reset before in-flight input injection so command
	// intent is not swallowed by the running task input channel.
	if trimmedContent == "/new" {
		g.handleNewSessionCommand(slot, msg) // releases slot.mu
		return nil
	}

	if trimmedContent == "/reset" {
		g.handleResetCommand(slot, msg) // releases slot.mu
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
	taskCtx, taskCancel := context.WithCancel(context.Background())
	slot.phase = slotRunning
	slot.inputCh = inputCh
	slot.taskCancel = taskCancel
	slot.taskToken++
	taskToken := slot.taskToken
	slot.sessionID = sessionID
	slot.lastSessionID = sessionID
	slot.lastTouched = g.currentTime()
	slot.mu.Unlock()

	// Run the task asynchronously so the Lark SDK event handler returns
	// immediately and can ACK the WS frame. Without this, long-running
	// tasks delay the ACK, causing the Lark server to re-deliver the
	// event and produce duplicate responses.
	g.taskWG.Add(1)
	go func(taskCtx context.Context, taskCancel context.CancelFunc, taskToken uint64) {
		defer g.taskWG.Done()
		defer taskCancel()

		awaitingInput := g.runTask(taskCtx, msg, sessionID, inputCh, isResume, taskToken)

		slot.mu.Lock()
		if slot.intentionalCancelToken == taskToken {
			slot.intentionalCancelToken = 0
		}
		if slot.taskToken == taskToken {
			slot.inputCh = nil
			slot.taskCancel = nil
			if awaitingInput {
				slot.phase = slotAwaitingInput
				slot.lastSessionID = slot.sessionID
			} else {
				slot.phase = slotIdle
				slot.sessionID = ""
			}
			slot.lastTouched = g.currentTime()
		}
		slot.mu.Unlock()
		if awaitingInput {
			g.drainAndReprocess(inputCh, msg.chatID, msg.chatType)
		} else {
			g.discardPendingInputs(inputCh, msg.chatID)
		}
	}(taskCtx, taskCancel, taskToken)

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
	if hub, ok := g.messenger.(*injectCaptureHub); ok && isInjectSyntheticMessageID(messageID) {
		hub.recordInjectedIncoming(chatID, messageID, senderID, msgType, contentJSON, g.currentTime())
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

	send := func(currentType, currentContent string) (string, error) {
		if replyToID != "" {
			return g.messenger.ReplyMessage(ctx, replyToID, currentType, currentContent)
		}
		return g.messenger.SendMessage(ctx, chatID, currentType, currentContent)
	}

	messageID, err := send(msgType, content)
	if err == nil || !strings.EqualFold(strings.TrimSpace(msgType), "post") || !isPostPayloadInvalidError(err) {
		return messageID, err
	}

	fallbackText := flattenPostContentToText(content)
	if strings.TrimSpace(fallbackText) == "" {
		fallbackText = "本次富文本结果渲染失败，已回退为纯文本发送。"
	}
	g.logger.Warn("Lark post dispatch fallback to text: %v", err)
	return send("text", textContent(fallbackText))
}

func isPostPayloadInvalidError(err error) bool {
	if err == nil {
		return false
	}
	lower := utils.TrimLower(err.Error())
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "message_content_text_tag") ||
		strings.Contains(lower, "invalid message content") ||
		strings.Contains(lower, "text field can't be nil")
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
	if isInjectSyntheticMessageID(messageID) {
		// Synthetic inject message IDs are not valid Lark open_message_id values.
		// Falling back to SendMessage keeps inject runs observable in real chats.
		return ""
	}
	return messageID
}

// updateMessage updates an existing message in-place using the given format.
func (g *Gateway) updateMessage(ctx context.Context, messageID, msgType, content string) error {
	if g.messenger == nil {
		return fmt.Errorf("lark messenger not initialized")
	}
	return g.messenger.UpdateMessage(ctx, messageID, msgType, content)
}

// addReaction adds an emoji reaction to the specified message and returns the reaction ID.
// Synthetic (inject_*) message IDs are handled transparently by the injectCaptureHub
// messenger wrapper, which records the call without hitting the real Lark API.
func (g *Gateway) addReaction(ctx context.Context, messageID, emojiType string) string {
	if g.messenger == nil || messageID == "" || emojiType == "" {
		g.logger.Warn("Lark add reaction skipped: messenger=%v messageID=%q emojiType=%q", g.messenger != nil, messageID, emojiType)
		return ""
	}
	reactionID, err := g.messenger.AddReaction(ctx, messageID, emojiType)
	if err != nil {
		g.logger.Warn("Lark add reaction failed: %v", err)
		return ""
	}
	return reactionID
}

// deleteReaction removes an emoji reaction from the specified message.
func (g *Gateway) deleteReaction(ctx context.Context, messageID, reactionID string) {
	if g.messenger == nil || messageID == "" || reactionID == "" {
		return
	}
	if err := g.messenger.DeleteReaction(ctx, messageID, reactionID); err != nil {
		g.logger.Warn("Lark delete reaction failed: %v", err)
	}
}

func isInjectSyntheticMessageID(messageID string) bool {
	id := strings.TrimSpace(messageID)
	if id == "" {
		return false
	}
	return strings.HasPrefix(id, "inject_")
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

// withLarkContext injects the common Lark tool context values (client, chat ID,
// message ID, and base domain) into the given context. All call sites that
// previously called WithLarkClient + WithLarkChatID + WithLarkMessageID
// individually should use this helper instead to ensure BaseDomain is always set.
func (g *Gateway) withLarkContext(ctx context.Context, chatID, messageID string) context.Context {
	ctx = builtinshared.WithLarkClient(ctx, g.client)
	ctx = builtinshared.WithLarkMessenger(ctx, g.messenger)
	ctx = builtinshared.WithLarkChatID(ctx, chatID)
	ctx = builtinshared.WithLarkMessageID(ctx, messageID)
	if domain := strings.TrimSpace(g.cfg.BaseDomain); domain != "" {
		ctx = builtinshared.WithLarkBaseDomain(ctx, domain)
	}
	return ctx
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
		trimmed := utils.TrimLower(raw)
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
