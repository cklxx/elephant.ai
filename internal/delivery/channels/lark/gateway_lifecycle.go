package lark

import (
	"context"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

const (
	wsReconnectBaseDelay = 2 * time.Second
	wsReconnectMaxDelay  = 60 * time.Second
)

// Start creates the Lark SDK client, event dispatcher, and WebSocket client, then blocks.
// It automatically reconnects with exponential backoff when the WebSocket connection drops.
func (g *Gateway) Start(ctx context.Context) error {
	if !g.cfg.Enabled {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	g.setCleanupCancel(cancel)
	g.dedup.startCleanup(runCtx, &g.cleanupWG)
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
	g.startDrainQueueTimer(runCtx)

	// Build the event dispatcher (shared across reconnections).
	eventDispatcher := g.buildEventDispatcher()

	g.logger.Info("Lark gateway connecting (app_id=%s)...", g.cfg.AppID)
	err := g.wsConnectLoop(runCtx, eventDispatcher)
	g.stopStateCleanupLoop()
	return err
}

// buildEventDispatcher creates the Lark event dispatcher with all registered handlers.
func (g *Gateway) buildEventDispatcher() *dispatcher.EventDispatcher {
	eventDispatcher := dispatcher.NewEventDispatcher("", "")
	eventDispatcher.OnP2MessageReceiveV1(g.handleMessage)

	// Register no-op handlers for events we intentionally ignore.
	// Without these, the SDK logs "unhandled event" warnings on every
	// reaction, read receipt, and bot-entered notification.
	eventDispatcher.OnP2MessageReactionCreatedV1(func(_ context.Context, _ *larkim.P2MessageReactionCreatedV1) error {
		return nil
	})
	eventDispatcher.OnP2MessageReactionDeletedV1(func(_ context.Context, _ *larkim.P2MessageReactionDeletedV1) error {
		return nil
	})
	eventDispatcher.OnP2ChatAccessEventBotP2pChatEnteredV1(func(_ context.Context, _ *larkim.P2ChatAccessEventBotP2pChatEnteredV1) error {
		return nil
	})
	eventDispatcher.OnP2MessageReadV1(func(_ context.Context, _ *larkim.P2MessageReadV1) error {
		return nil
	})
	return eventDispatcher
}

// wsConnectLoop connects the WebSocket client and automatically reconnects
// with exponential backoff when the connection drops. It blocks until the
// context is cancelled.
func (g *Gateway) wsConnectLoop(ctx context.Context, eventDispatcher *dispatcher.EventDispatcher) error {
	delay := wsReconnectBaseDelay
	for {
		wsClient := g.newWSClient(eventDispatcher)
		g.wsClient = wsClient

		err := wsClient.Start(ctx)

		// Context cancelled — intentional shutdown, exit cleanly.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Unexpected disconnect — log and reconnect with backoff.
		g.logger.Warn("Lark WebSocket disconnected (err=%v), reconnecting in %v...", err, delay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		// Exponential backoff capped at wsReconnectMaxDelay.
		delay *= 2
		if delay > wsReconnectMaxDelay {
			delay = wsReconnectMaxDelay
		}

		g.logger.Info("Lark gateway reconnecting (app_id=%s)...", g.cfg.AppID)
	}
}

// newWSClient creates a fresh larkws.Client for a (re)connection attempt.
func (g *Gateway) newWSClient(eventDispatcher *dispatcher.EventDispatcher) *larkws.Client {
	var wsOpts []larkws.ClientOption
	wsOpts = append(wsOpts, larkws.WithEventHandler(eventDispatcher))
	wsOpts = append(wsOpts, larkws.WithLogLevel(larkcore.LogLevelInfo))
	if domain := strings.TrimSpace(g.cfg.BaseDomain); domain != "" {
		wsOpts = append(wsOpts, larkws.WithDomain(domain))
	}
	return larkws.NewClient(g.cfg.AppID, g.cfg.AppSecret, wsOpts...)
}

// Stop releases resources. The WebSocket client does not expose a Stop method;
// cancelling the context passed to Start is the primary shutdown mechanism.
func (g *Gateway) Stop() {
	if g.attentionGate != nil {
		g.attentionGate.StopDrainTimer()
	}
	g.stopStateCleanupLoop()
}

// NotifyRunningTaskInterruptions cancels in-flight foreground tasks and sends
// a visible interruption notice to each affected chat. When the TaskStore is
// available, the notice includes the task description and promises auto-resume.
func (g *Gateway) NotifyRunningTaskInterruptions(notice string) int {
	if g == nil {
		return 0
	}
	notice = strings.TrimSpace(notice)
	if notice == "" {
		notice = "系统正在维护中，您的任务将在服务恢复后自动重新执行。"
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

	// Build per-chat messages with task descriptions when TaskStore is available.
	chatIDs := make([]string, len(targets))
	for i, t := range targets {
		chatIDs[i] = t.chatID
	}
	chatNotices := g.buildShutdownNotices(chatIDs)

	notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, target := range targets {
		target.cancel()
		msg := chatNotices[target.chatID]
		if msg == "" {
			msg = notice
		}
		g.dispatch(notifyCtx, target.chatID, "", "text", textContent(msg))
	}
	return len(targets)
}

// buildShutdownNotices looks up active task descriptions for each affected chat
// and returns a per-chat notice that includes the task name.
func (g *Gateway) buildShutdownNotices(chatIDs []string) map[string]string {
	notices := make(map[string]string, len(chatIDs))
	if g.taskStore == nil {
		return notices
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for _, chatID := range chatIDs {
		tasks, err := g.taskStore.ListByChat(ctx, chatID, true, 1)
		if err != nil || len(tasks) == 0 {
			continue
		}
		desc := strings.TrimSpace(tasks[0].Description)
		if desc == "" {
			continue
		}
		// Truncate long descriptions for readability.
		if len(desc) > 80 {
			desc = desc[:80] + "..."
		}
		notices[chatID] = "系统正在维护中，您的任务「" + desc + "」将在服务恢复后自动重新执行。"
	}
	return notices
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

// startDrainQueueTimer wires the attention gate drain timer into the gateway.
// When quiet hours end, all queued messages are dispatched to their chats.
func (g *Gateway) startDrainQueueTimer(ctx context.Context) {
	if g.attentionGate == nil || !g.attentionGate.IsEnabled() {
		return
	}
	// Share the gateway's time function with the attention gate.
	g.attentionGate.nowFn = g.now

	g.attentionGate.StartDrainTimer(ctx, func(msgs []QueuedMessage) {
		g.logger.Info("Attention gate: draining %d queued messages after quiet hours", len(msgs))
		drainCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		for _, msg := range msgs {
			g.dispatch(drainCtx, msg.ChatID, "", "text", textContent(msg.Content))
		}
	})
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
