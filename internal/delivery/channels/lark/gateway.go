package lark

import (
	"context"
	"fmt"
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

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

const (
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
	dedup               *eventDedup
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
	costTracker         CostTrackerReader // optional; for /usage dashboard
	chatSessionStore    ChatSessionBindingStore
	deliveryOutboxStore DeliveryOutboxStore
	noticeState         *noticeStateStore
	activeSlots         sync.Map           // chatID → *sessionSlot
	pendingInputRelays  sync.Map           // chatID → *pendingRelayQueue
	aiCoordinator       *AIChatCoordinator // coordinates multi-bot chat sessions
	autoAuth            *AutoAuth          // in-message OAuth device flow
	attentionGate       *AttentionGate     // optional urgency filter for incoming messages
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
		dedup:         newEventDedup(logger),
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
		attentionGate: NewAttentionGate(cfg.AttentionGate),
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

// SetCostTracker configures the cost tracker for the /usage dashboard.
func (g *Gateway) SetCostTracker(ct CostTrackerReader) { g.costTracker = ct }

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
