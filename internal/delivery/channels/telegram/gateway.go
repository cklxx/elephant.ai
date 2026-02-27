package telegram

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	portsllm "alex/internal/domain/agent/ports/llm"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/mymmrac/telego"
)

const (
	messageDedupCacheSize    = 2048
	messageDedupTTL          = 10 * time.Minute
	chatSessionBindingPrefix = "telegram"
	defaultActiveSlotTTL     = 6 * time.Hour
	defaultActiveSlotMax     = 2048
	defaultStateCleanupEvery = 5 * time.Minute
)

// AgentExecutor is an alias for the shared channel executor interface.
type AgentExecutor = channels.AgentExecutor

// slotPhase describes the lifecycle phase of a sessionSlot.
type slotPhase int

const (
	slotIdle          slotPhase = iota // no active task
	slotRunning                        // task goroutine is active
	slotAwaitingInput                  // task ended with await_user_input
)

// sessionSlot tracks whether a task is active for a given chat.
type sessionSlot struct {
	mu          sync.Mutex
	phase       slotPhase
	inputCh     chan agent.UserInput // non-nil only when phase == slotRunning
	taskCancel  context.CancelFunc   // cancels the currently running task
	taskToken   uint64
	sessionID   string
	lastTouched time.Time
}

// Gateway bridges Telegram bot messages into the agent runtime.
type Gateway struct {
	channels.BaseGateway
	cfg           Config
	agent         AgentExecutor
	logger        logging.Logger
	bot           *telego.Bot
	messenger     Messenger
	eventListener agent.EventListener
	llmFactory    portsllm.LLMClientFactory
	llmProfile    runtimeconfig.LLMProfile
	taskStore     TaskStore
	planReview    PlanReviewStore
	dedupCache    *lru.Cache[int, time.Time]
	now           func() time.Time
	activeSlots   sync.Map   // chatID (int64) → *sessionSlot
	taskWG        sync.WaitGroup
	tokenCounter  atomic.Uint64
	cleanupCancel context.CancelFunc
	cleanupWG     sync.WaitGroup
}

// NewGateway creates a Telegram gateway. Call Start to begin polling.
func NewGateway(cfg Config, agent AgentExecutor, logger logging.Logger) (*Gateway, error) {
	if agent == nil {
		return nil, fmt.Errorf("telegram gateway: agent executor is required")
	}
	dedupCache, _ := lru.New[int, time.Time](messageDedupCacheSize)
	return &Gateway{
		cfg:        cfg,
		agent:      agent,
		logger:     logging.OrNop(logger),
		dedupCache: dedupCache,
		now:        time.Now,
	}, nil
}

// SetEventListener injects an event listener for broadcasting.
func (g *Gateway) SetEventListener(listener agent.EventListener) { g.eventListener = listener }

// SetTaskStore injects a task persistence store.
func (g *Gateway) SetTaskStore(store TaskStore) { g.taskStore = store }

// SetMessenger injects a custom Messenger (primary test injection point).
func (g *Gateway) SetMessenger(m Messenger) { g.messenger = m }

// SetPlanReviewStore injects a plan review store.
func (g *Gateway) SetPlanReviewStore(store PlanReviewStore) { g.planReview = store }

// SetLLMFactory injects an LLM factory for lightweight calls (slow progress summary).
func (g *Gateway) SetLLMFactory(factory portsllm.LLMClientFactory, profile runtimeconfig.LLMProfile) {
	g.llmFactory = factory
	g.llmProfile = profile
}

// Start creates the Telegram bot, starts long polling, and blocks until ctx is cancelled.
func (g *Gateway) Start(ctx context.Context) error {
	bot, err := telego.NewBot(g.cfg.BotToken)
	if err != nil {
		return fmt.Errorf("telegram bot init: %w", err)
	}
	g.bot = bot

	if g.messenger == nil {
		g.messenger = newSDKMessenger(bot)
	}

	g.logger.Info("Telegram gateway starting long polling...")

	pollCtx, pollCancel := context.WithCancel(ctx)
	g.cleanupCancel = pollCancel

	updates, err := bot.UpdatesViaLongPolling(pollCtx, &telego.GetUpdatesParams{
		Timeout: 30,
		AllowedUpdates: []string{
			"message",
			"edited_message",
			"callback_query",
		},
	})
	if err != nil {
		pollCancel()
		return fmt.Errorf("telegram long polling: %w", err)
	}

	// Start state cleanup loop.
	g.startCleanupLoop(pollCtx)

	g.logger.Info("Telegram gateway polling for updates")

	for update := range updates {
		g.handleUpdate(pollCtx, update)
	}

	g.logger.Info("Telegram gateway polling stopped")
	return nil
}

// Stop cancels polling and waits for running tasks.
func (g *Gateway) Stop() {
	if g.cleanupCancel != nil {
		g.cleanupCancel()
	}
	g.cleanupWG.Wait()
	g.taskWG.Wait()
}

// WaitForTasks blocks until all running task goroutines finish (for tests).
func (g *Gateway) WaitForTasks() {
	g.taskWG.Wait()
}

func (g *Gateway) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

func (g *Gateway) nextToken() uint64 {
	return g.tokenCounter.Add(1)
}

// newSessionID generates a fresh session ID with the configured prefix.
func (g *Gateway) newSessionID() string {
	prefix := g.cfg.SessionPrefix
	if prefix == "" {
		prefix = "tg"
	}
	return prefix + "-" + id.NewLogID()
}

// getOrCreateSlot returns the session slot for the given chat, creating if needed.
func (g *Gateway) getOrCreateSlot(chatID int64) *sessionSlot {
	raw, loaded := g.activeSlots.LoadOrStore(chatID, &sessionSlot{
		lastTouched: g.clock(),
	})
	slot := raw.(*sessionSlot)
	if loaded {
		slot.mu.Lock()
		slot.lastTouched = g.clock()
		slot.mu.Unlock()
	}
	return slot
}

// getSlot returns the session slot for a chat if it exists.
func (g *Gateway) getSlot(chatID int64) *sessionSlot {
	raw, ok := g.activeSlots.Load(chatID)
	if !ok {
		return nil
	}
	return raw.(*sessionSlot)
}

// isDuplicate checks and records a message ID for dedup.
func (g *Gateway) isDuplicate(msgID int) bool {
	if g.dedupCache == nil {
		return false
	}
	now := g.clock()
	if ts, ok := g.dedupCache.Get(msgID); ok {
		if now.Sub(ts) < messageDedupTTL {
			return true
		}
	}
	g.dedupCache.Add(msgID, now)
	return false
}

// isGroupAllowed checks whether a group chat ID is permitted.
func (g *Gateway) isGroupAllowed(chatID int64) bool {
	if len(g.cfg.AllowedGroups) == 0 {
		return true // empty = all groups allowed
	}
	for _, allowed := range g.cfg.AllowedGroups {
		if allowed == chatID {
			return true
		}
	}
	return false
}

// chatIDStr returns the string representation of a chat ID for session keying.
func chatIDStr(chatID int64) string {
	return strconv.FormatInt(chatID, 10)
}

// startCleanupLoop runs periodic state cleanup (slots, dedup cache).
func (g *Gateway) startCleanupLoop(ctx context.Context) {
	interval := g.cfg.StateCleanupInterval
	if interval <= 0 {
		interval = defaultStateCleanupEvery
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
				g.cleanupSlots()
			}
		}
	}()
}

// cleanupSlots evicts idle slots that exceed TTL or max entries.
func (g *Gateway) cleanupSlots() {
	ttl := g.cfg.ActiveSlotTTL
	if ttl <= 0 {
		ttl = defaultActiveSlotTTL
	}
	maxEntries := g.cfg.ActiveSlotMaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultActiveSlotMax
	}

	now := g.clock()
	cutoff := now.Add(-ttl)

	// Evict by TTL.
	g.activeSlots.Range(func(key, value any) bool {
		slot := value.(*sessionSlot)
		slot.mu.Lock()
		idle := slot.phase == slotIdle && slot.lastTouched.Before(cutoff)
		slot.mu.Unlock()
		if idle {
			g.activeSlots.Delete(key)
		}
		return true
	})

	// Enforce max entries by evicting oldest idle slots.
	type entry struct {
		key         int64
		lastTouched time.Time
	}
	var entries []entry
	g.activeSlots.Range(func(key, value any) bool {
		slot := value.(*sessionSlot)
		slot.mu.Lock()
		if slot.phase == slotIdle {
			entries = append(entries, entry{key: key.(int64), lastTouched: slot.lastTouched})
		}
		slot.mu.Unlock()
		return true
	})
	if len(entries) > maxEntries {
		// Sort by lastTouched ascending and evict oldest.
		for i := 0; i < len(entries)-1; i++ {
			for j := i + 1; j < len(entries); j++ {
				if entries[j].lastTouched.Before(entries[i].lastTouched) {
					entries[i], entries[j] = entries[j], entries[i]
				}
			}
		}
		toEvict := len(entries) - maxEntries
		for i := 0; i < toEvict; i++ {
			g.activeSlots.Delete(entries[i].key)
		}
	}
}
