package hooks

import (
	"context"
	"fmt"
	"strings"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/logging"
	"alex/internal/memory"
)

const (
	conversationCaptureHookName = "conversation_capture"
	maxConversationInputLen     = 250
	maxConversationAnswerLen    = 400
)

// ConversationCaptureHook stores user/assistant turn pairs as long-term memory
// for pure conversational interactions (no tool calls).
type ConversationCaptureHook struct {
	memoryService      memory.Service
	enabled            bool
	captureGroupMemory bool
	dedupeThreshold    float64
	logger             logging.Logger
}

// ConversationCaptureConfig controls capture behavior for chat turns.
type ConversationCaptureConfig struct {
	Enabled            bool
	CaptureMessages    bool
	CaptureGroupMemory bool
	DedupeThreshold    float64
}

// NewConversationCaptureHook creates a hook for conversation-only capture.
func NewConversationCaptureHook(svc memory.Service, logger logging.Logger, cfg ConversationCaptureConfig) *ConversationCaptureHook {
	enabled := cfg.Enabled && cfg.CaptureMessages
	dedupe := cfg.DedupeThreshold
	if dedupe <= 0 {
		dedupe = 0.85
	}
	return &ConversationCaptureHook{
		memoryService:      svc,
		enabled:            enabled,
		captureGroupMemory: cfg.CaptureGroupMemory,
		dedupeThreshold:    dedupe,
		logger:             logging.OrNop(logger),
	}
}

func (h *ConversationCaptureHook) Name() string { return conversationCaptureHookName }

// OnTaskStart is a no-op for conversation capture.
func (h *ConversationCaptureHook) OnTaskStart(_ context.Context, _ TaskInfo) []Injection {
	return nil
}

// OnTaskCompleted captures pure conversational turns as chat_turn memories.
func (h *ConversationCaptureHook) OnTaskCompleted(ctx context.Context, result TaskResultInfo) error {
	if h.memoryService == nil || !h.enabled {
		return nil
	}
	policy, hasPolicy := appcontext.MemoryPolicyFromContext(ctx)
	if hasPolicy {
		if !policy.Enabled || !policy.AutoCapture || !policy.CaptureMessages {
			return nil
		}
	}
	if len(result.ToolCalls) > 0 {
		return nil
	}

	userID := strings.TrimSpace(result.UserID)
	if userID == "" {
		return nil
	}

	input := smartTruncate(result.TaskInput, maxConversationInputLen)
	answer := smartTruncate(result.Answer, maxConversationAnswerLen)
	if input == "" && answer == "" {
		return nil
	}

	content := strings.TrimSpace(fmt.Sprintf("User: %s\nAssistant: %s", input, answer))
	entry := memory.Entry{
		UserID:   userID,
		Content:  content,
		Keywords: extractKeywords(result.TaskInput),
		Slots:    buildConversationSlots(ctx, result.SessionID, userID, "user"),
	}

	if h.isDuplicate(ctx, entry, map[string]string{"type": "chat_turn", "scope": "user"}) {
		h.logger.Debug("Skipped conversation capture due to similarity threshold (user=%s)", userID)
	} else if _, err := h.memoryService.Save(ctx, entry); err != nil {
		h.logger.Warn("Conversation capture failed: %v", err)
		return fmt.Errorf("conversation capture: %w", err)
	}

	if h.captureGroupMemory && appcontext.IsGroupFromContext(ctx) {
		if chatUserID := chatScopeUserID(ctx); chatUserID != "" {
			groupEntry := entry
			groupEntry.UserID = chatUserID
			groupEntry.Slots = buildConversationSlots(ctx, result.SessionID, userID, "chat")
			if h.isDuplicate(ctx, groupEntry, map[string]string{"type": "chat_turn", "scope": "chat"}) {
				h.logger.Debug("Skipped group conversation capture due to similarity threshold (chat=%s)", chatUserID)
				return nil
			}
			if _, err := h.memoryService.Save(ctx, groupEntry); err != nil {
				h.logger.Warn("Group conversation capture failed: %v", err)
				return fmt.Errorf("group conversation capture: %w", err)
			}
		}
	}

	return nil
}

func buildConversationSlots(ctx context.Context, sessionID, senderID, scope string) map[string]string {
	slots := map[string]string{
		"type":   "chat_turn",
		"scope":  scope,
		"source": "conversation_capture",
	}
	if sessionID != "" {
		slots["session_id"] = sessionID
	}
	if senderID != "" {
		slots["sender_id"] = senderID
	}
	if channel := appcontext.ChannelFromContext(ctx); channel != "" {
		slots["channel"] = channel
	}
	if chatID := appcontext.ChatIDFromContext(ctx); chatID != "" {
		slots["chat_id"] = chatID
	}
	return slots
}

func chatScopeUserID(ctx context.Context) string {
	channel := strings.TrimSpace(appcontext.ChannelFromContext(ctx))
	chatID := strings.TrimSpace(appcontext.ChatIDFromContext(ctx))
	if channel == "" || chatID == "" {
		return ""
	}
	return fmt.Sprintf("chat:%s:%s", channel, chatID)
}

func (h *ConversationCaptureHook) isDuplicate(ctx context.Context, entry memory.Entry, slots map[string]string) bool {
	if h.memoryService == nil || h.dedupeThreshold <= 0 {
		return false
	}
	query := memory.Query{
		UserID:   entry.UserID,
		Text:     entry.Content,
		Keywords: entry.Keywords,
		Slots:    slots,
		Limit:    5,
	}
	existing, err := h.memoryService.Recall(ctx, query)
	if err != nil || len(existing) == 0 {
		return false
	}
	for _, prev := range existing {
		if similarityScore(entry.Content, prev.Content) >= h.dedupeThreshold {
			return true
		}
	}
	return false
}

func smartTruncate(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || limit <= 0 {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	if limit <= 10 {
		return string(runes[:limit])
	}
	headLen := limit * 6 / 10
	tailLen := limit - headLen - 5
	if tailLen < 1 {
		tailLen = 1
	}
	return string(runes[:headLen]) + " ... " + string(runes[len(runes)-tailLen:])
}
