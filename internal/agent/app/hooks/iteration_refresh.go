package hooks

import (
	"context"
	"fmt"
	"strings"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/textutil"
	"alex/internal/logging"
	"alex/internal/memory"
	id "alex/internal/utils/id"
)

const (
	defaultRefreshKeywordLimit = 5
	defaultRefreshRecallLimit  = 3
)

// IterationRefreshConfig configures mid-loop memory refresh defaults.
type IterationRefreshConfig struct {
	DefaultInterval int
	MaxTokens       int
	KeywordLimit    int
	RecallLimit     int
}

// IterationRefreshHook injects recalled memories mid-loop.
type IterationRefreshHook struct {
	memoryService   memory.Service
	logger          logging.Logger
	defaultInterval int
	maxTokens       int
	keywordLimit    int
	recallLimit     int
}

// NewIterationRefreshHook creates a refresh hook with config defaults.
func NewIterationRefreshHook(svc memory.Service, logger logging.Logger, cfg IterationRefreshConfig) *IterationRefreshHook {
	keywordLimit := cfg.KeywordLimit
	if keywordLimit <= 0 {
		keywordLimit = defaultRefreshKeywordLimit
	}
	recallLimit := cfg.RecallLimit
	if recallLimit <= 0 {
		recallLimit = defaultRefreshRecallLimit
	}
	return &IterationRefreshHook{
		memoryService:   svc,
		logger:          logging.OrNop(logger),
		defaultInterval: cfg.DefaultInterval,
		maxTokens:       cfg.MaxTokens,
		keywordLimit:    keywordLimit,
		recallLimit:     recallLimit,
	}
}

// OnIteration refreshes context based on the MemoryPolicy.
func (h *IterationRefreshHook) OnIteration(ctx context.Context, state *agent.TaskState, iteration int) agent.IterationHookResult {
	if h == nil || h.memoryService == nil || state == nil {
		return agent.IterationHookResult{}
	}
	if appcontext.IsSubagentContext(ctx) {
		return agent.IterationHookResult{}
	}

	policy := appcontext.ResolveMemoryPolicy(ctx)
	if !policy.Enabled || !policy.RefreshEnabled {
		return agent.IterationHookResult{}
	}

	interval := policy.RefreshInterval
	if interval <= 0 {
		interval = h.defaultInterval
	}
	if interval <= 0 || iteration == 0 || iteration%interval != 0 {
		return agent.IterationHookResult{}
	}

	userID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if userID == "" {
		h.logger.Debug("Memory refresh skipped: missing user_id")
		return agent.IterationHookResult{}
	}

	keywords := extractRecentKeywords(state.ToolResults, h.keywordLimit)
	if len(keywords) == 0 {
		return agent.IterationHookResult{}
	}

	memories, err := h.memoryService.Recall(ctx, memory.Query{
		UserID:   userID,
		Text:     strings.Join(keywords, " "),
		Keywords: keywords,
		Limit:    h.recallLimit,
	})
	if err != nil || len(memories) == 0 {
		if err != nil {
			h.logger.Debug("Memory refresh recall failed: %v", err)
		}
		return agent.IterationHookResult{}
	}

	maxTokens := policy.RefreshMaxTokens
	if maxTokens <= 0 {
		maxTokens = h.maxTokens
	}
	content := formatRefreshMemories(memories, maxTokens)
	if strings.TrimSpace(content) == "" {
		return agent.IterationHookResult{}
	}

	insertProactiveSystemMessage(state, content)
	return agent.IterationHookResult{MemoriesInjected: len(memories)}
}

func extractRecentKeywords(results []ports.ToolResult, limit int) []string {
	if limit <= 0 || len(results) == 0 {
		return nil
	}
	start := len(results) - limit
	if start < 0 {
		start = 0
	}
	var tokens []string
	for i := start; i < len(results); i++ {
		res := results[i]
		if res.Content != "" {
			tokens = append(tokens, res.Content)
		}
	}
	return textutil.ExtractKeywords(strings.Join(tokens, " "), textutil.KeywordOptions{})
}

func formatRefreshMemories(entries []memory.Entry, maxTokens int) string {
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Proactive Memory Refresh\n\n")
	sb.WriteString("Additional context recalled from prior work:\n\n")
	for i, entry := range entries {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, strings.TrimSpace(entry.Content)))
	}
	text := sb.String()
	if maxTokens <= 0 {
		return text
	}
	if estimateTokenCount(text) <= maxTokens {
		return text
	}
	return truncateToTokens(text, maxTokens)
}

func insertProactiveSystemMessage(state *agent.TaskState, content string) {
	if state == nil || strings.TrimSpace(content) == "" {
		return
	}
	msg := ports.Message{
		Role:    "system",
		Content: content,
		Source:  ports.MessageSourceProactive,
	}
	insertAt := 0
	for i, existing := range state.Messages {
		if strings.EqualFold(existing.Role, "system") && existing.Source == ports.MessageSourceSystemPrompt {
			insertAt = i + 1
			break
		}
	}
	state.Messages = append(state.Messages, ports.Message{})
	copy(state.Messages[insertAt+1:], state.Messages[insertAt:])
	state.Messages[insertAt] = msg
}

func estimateTokenCount(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	return len([]rune(trimmed)) / 4
}

func truncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return text
	}
	runes := []rune(text)
	limit := maxTokens * 4
	if limit <= 0 || limit >= len(runes) {
		return text
	}
	return string(runes[:limit]) + "..."
}
