package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"alex/internal/shared/logging"
)

const (
	defaultHooksAggregateWindow = 30 * time.Second
	defaultHooksDedupeWindow    = 2 * time.Second
	maxRecentHookFingerprints   = 4096
	maxFingerprintInputBytes    = 1024
)

// NoticeLoader loads the notice binding to determine which chat receives
// hook notifications. It mirrors the read-only subset of noticeStateStore.
type NoticeLoader interface {
	Load() (chatID string, ok bool, err error)
}

// NoticeLoaderFunc adapts a plain function to the NoticeLoader interface.
type NoticeLoaderFunc func() (string, bool, error)

func (f NoticeLoaderFunc) Load() (string, bool, error) { return f() }

// LarkNotifier is the subset of lark.Gateway used by the hooks bridge.
type LarkNotifier interface {
	SendNotification(ctx context.Context, chatID, text string) error
}

// toolEvent captures one PostToolUse for aggregation.
type toolEvent struct {
	chatID  string
	message string
}

// HooksBridge receives Claude Code hook events and forwards them to Lark.
// PostToolUse events are aggregated into periodic summaries; Stop events
// are forwarded immediately.
type HooksBridge struct {
	gateway       LarkNotifier
	token         string
	logger        logging.Logger
	defaultChatID string
	noticeLoader  NoticeLoader

	// Aggregation state for PostToolUse events.
	mu         sync.Mutex
	toolBuffer []toolEvent
	flushTimer *time.Timer
	aggWindow  time.Duration
	now        func() time.Time // injectable clock for testing

	dedupeMu           sync.Mutex
	recentFingerprints map[uint64]time.Time
	dedupeWindow       time.Duration
}

// NewHooksBridge constructs a hooks bridge.
func NewHooksBridge(gateway LarkNotifier, noticeLoader NoticeLoader, token, defaultChatID string, logger logging.Logger) *HooksBridge {
	return &HooksBridge{
		gateway:       gateway,
		token:         token,
		logger:        logging.OrNop(logger),
		defaultChatID: defaultChatID,
		noticeLoader:  noticeLoader,
		aggWindow:     defaultHooksAggregateWindow,
		now:           time.Now,
		dedupeWindow:  defaultHooksDedupeWindow,
	}
}

// ServeHTTP handles POST /api/hooks/claude-code.
func (h *HooksBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.verifyToken(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	payload, err := h.readAndDecode(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chatID := h.resolveChatID(r)
	if chatID == "" {
		http.Error(w, "no target chat_id", http.StatusBadRequest)
		return
	}
	if h.isDuplicateEvent(chatID, payload) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	message := h.formatHookEvent(payload)
	if message == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.dispatchEvent(r.Context(), chatID, message, payload.Event)
	w.WriteHeader(http.StatusOK)
}

// verifyToken checks the Bearer token if one is configured.
func (h *HooksBridge) verifyToken(r *http.Request) bool {
	if h.token == "" {
		return true
	}
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(auth)), []byte(h.token)) == 1
}

// readAndDecode reads the request body and decodes the hook payload.
func (h *HooksBridge) readAndDecode(r *http.Request) (hookPayload, error) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		return hookPayload{}, fmt.Errorf("read body failed")
	}
	payload, err := decodeHookPayload(body)
	if err != nil {
		h.logger.Warn("Hooks bridge: invalid json payload (%v): %s", err, truncateHookText(string(body), 180))
		return hookPayload{}, fmt.Errorf("invalid json")
	}
	return payload, nil
}

// dispatchEvent routes a formatted message based on event type.
func (h *HooksBridge) dispatchEvent(ctx context.Context, chatID, message, event string) {
	switch event {
	case "PostToolUse", "PreToolUse":
		h.bufferToolEvent(ctx, chatID, message)
	case "Stop":
		h.flushToolBuffer(ctx)
		h.sendToLark(ctx, chatID, message)
	default:
		h.sendToLark(ctx, chatID, message)
	}
}

// resolveChatID determines the target Lark chat for a hook event.
// Priority: query param > notice binding > defaultChatID.
func (h *HooksBridge) resolveChatID(r *http.Request) string {
	if chatID := strings.TrimSpace(r.URL.Query().Get("chat_id")); chatID != "" {
		return chatID
	}
	if h.noticeLoader != nil {
		if chatID, ok, err := h.noticeLoader.Load(); err == nil && ok && chatID != "" {
			return chatID
		}
	}
	return h.defaultChatID
}

// sendToLark dispatches a text message to the specified Lark chat.
func (h *HooksBridge) sendToLark(ctx context.Context, chatID, message string) {
	if h.gateway == nil {
		h.logger.Warn("Hooks bridge: gateway not set")
		return
	}
	if err := h.gateway.SendNotification(ctx, chatID, message); err != nil {
		h.logger.Warn("Hooks bridge send failed: %v", err)
	}
}

// bufferToolEvent adds a tool event to the buffer and schedules a flush.
func (h *HooksBridge) bufferToolEvent(ctx context.Context, chatID, message string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.toolBuffer = append(h.toolBuffer, toolEvent{chatID: chatID, message: message})

	if h.flushTimer == nil {
		h.flushTimer = time.AfterFunc(h.aggWindow, func() {
			h.flushToolBuffer(ctx)
		})
	}
}

// flushToolBuffer sends all buffered tool events as a single aggregated message.
func (h *HooksBridge) flushToolBuffer(ctx context.Context) {
	h.mu.Lock()
	if len(h.toolBuffer) == 0 {
		h.mu.Unlock()
		return
	}
	events := h.toolBuffer
	h.toolBuffer = nil
	if h.flushTimer != nil {
		h.flushTimer.Stop()
		h.flushTimer = nil
	}
	h.mu.Unlock()

	grouped := groupEventsByChatID(events)
	for chatID, messages := range grouped {
		h.sendToLark(ctx, chatID, formatToolSummary(messages))
	}
}

// groupEventsByChatID groups tool events by their target chat.
func groupEventsByChatID(events []toolEvent) map[string][]string {
	grouped := make(map[string][]string)
	for _, e := range events {
		grouped[e.chatID] = append(grouped[e.chatID], e.message)
	}
	return grouped
}

// Close flushes remaining buffered events. Call on shutdown.
func (h *HooksBridge) Close(ctx context.Context) {
	h.flushToolBuffer(ctx)
}
