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
	memoryRecallHookName = "memory_recall"
	defaultMaxRecalls    = 5
)

// MemoryRecallHook automatically recalls relevant memories before task execution
// and injects them into the agent context.
type MemoryRecallHook struct {
	memoryService memory.Service
	maxRecalls    int
	captureGroup  bool
	enabled       bool
	logger        logging.Logger
}

// MemoryRecallConfig configures the memory recall hook.
type MemoryRecallConfig struct {
	Enabled    bool // Enable auto recall
	AutoRecall bool // Enable recall specifically
	MaxRecalls int  // Maximum memories to recall (default: 5)
	// CaptureGroupMemory enables recalling group-scoped memories when available.
	CaptureGroupMemory bool
}

// NewMemoryRecallHook creates a new memory recall hook.
func NewMemoryRecallHook(svc memory.Service, logger logging.Logger, cfg MemoryRecallConfig) *MemoryRecallHook {
	maxRecalls := cfg.MaxRecalls
	if maxRecalls <= 0 {
		maxRecalls = defaultMaxRecalls
	}
	enabled := true
	if !cfg.Enabled || !cfg.AutoRecall {
		enabled = false
	}
	return &MemoryRecallHook{
		memoryService: svc,
		maxRecalls:    maxRecalls,
		captureGroup:  cfg.CaptureGroupMemory,
		enabled:       enabled,
		logger:        logging.OrNop(logger),
	}
}

func (h *MemoryRecallHook) Name() string { return memoryRecallHookName }

// OnTaskStart recalls memories matching the task input and returns them as injections.
func (h *MemoryRecallHook) OnTaskStart(ctx context.Context, task TaskInfo) []Injection {
	if h.memoryService == nil || !h.enabled {
		return nil
	}
	policy := appcontext.ResolveMemoryPolicy(ctx)
	if !policy.Enabled || !policy.AutoRecall {
		return nil
	}
	if strings.TrimSpace(task.TaskInput) == "" {
		return nil
	}
	userID := task.UserID
	if userID == "" {
		userID = "default"
	}

	keywords := extractKeywords(task.TaskInput)
	if len(keywords) == 0 {
		return nil
	}

	entries, err := h.memoryService.Recall(ctx, memory.Query{
		UserID:   userID,
		Text:     task.TaskInput,
		Keywords: keywords,
		Limit:    h.maxRecalls,
	})
	if err != nil {
		h.logger.Warn("Memory recall failed: %v", err)
		return nil
	}
	if h.captureGroup && appcontext.IsGroupFromContext(ctx) {
		if chatUserID := chatScopeUserID(ctx); chatUserID != "" {
			groupLimit := h.maxRecalls
			if groupLimit > 2 {
				groupLimit = 2
			}
			if groupLimit > 0 {
				groupEntries, err := h.memoryService.Recall(ctx, memory.Query{
					UserID:   chatUserID,
					Text:     task.TaskInput,
					Keywords: keywords,
					Slots:    map[string]string{"type": "chat_turn", "scope": "chat"},
					Limit:    groupLimit,
				})
				if err != nil {
					h.logger.Warn("Group memory recall failed: %v", err)
				} else if len(groupEntries) > 0 {
					entries = append(entries, groupEntries...)
				}
			}
		}
	}

	if len(entries) == 0 {
		return nil
	}

	content := formatMemoryEntries(entries)
	h.logger.Info("Auto-recalled %d memories for task (keywords: %v)", len(entries), keywords)

	return []Injection{
		{
			Type:     InjectionMemoryRecall,
			Content:  content,
			Source:   memoryRecallHookName,
			Priority: 100, // High priority: memories should appear early in context
		},
	}
}

// OnTaskCompleted is a no-op for the recall hook.
func (h *MemoryRecallHook) OnTaskCompleted(_ context.Context, _ TaskResultInfo) error {
	return nil
}

// extractKeywords performs lightweight keyword extraction from task input.
// It tokenizes the input, filters common stop words, and returns
// the most distinctive terms for memory query.
func extractKeywords(input string) []string {
	fields := strings.FieldsFunc(input, func(r rune) bool {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			return false
		}
		if r >= '0' && r <= '9' {
			return false
		}
		// Keep CJK characters
		if r >= 0x4E00 && r <= 0x9FFF {
			return false
		}
		return true
	})

	seen := make(map[string]bool, len(fields))
	var keywords []string
	for _, field := range fields {
		lower := strings.ToLower(strings.TrimSpace(field))
		if lower == "" || len(lower) < 2 || seen[lower] {
			continue
		}
		if isStopWord(lower) {
			continue
		}
		seen[lower] = true
		keywords = append(keywords, lower)
	}

	// Cap at reasonable number to avoid overly broad queries
	const maxKeywords = 10
	if len(keywords) > maxKeywords {
		keywords = keywords[:maxKeywords]
	}

	return keywords
}

// formatMemoryEntries formats recalled memory entries for context injection.
func formatMemoryEntries(entries []memory.Entry) string {
	var sb strings.Builder
	sb.WriteString("The following memories were automatically recalled from prior interactions:\n\n")

	for i, entry := range entries {
		sb.WriteString(fmt.Sprintf("**Memory %d**", i+1))
		if len(entry.Keywords) > 0 {
			sb.WriteString(fmt.Sprintf(" (keywords: %s)", strings.Join(entry.Keywords, ", ")))
		}
		sb.WriteString(":\n")
		sb.WriteString(entry.Content)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// isStopWord returns true for common words that should be excluded from keyword queries.
func isStopWord(word string) bool {
	stops := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "shall": true, "can": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "with": true, "by": true, "from": true, "as": true,
		"into": true, "about": true, "between": true,
		"and": true, "or": true, "but": true, "not": true, "no": true,
		"if": true, "then": true, "else": true, "when": true, "while": true,
		"it": true, "its": true, "this": true, "that": true, "these": true,
		"those": true, "my": true, "your": true, "his": true, "her": true,
		"me": true, "you": true, "we": true, "they": true, "them": true,
		"what": true, "which": true, "who": true, "how": true, "where": true,
		"there": true, "here": true,
		"all": true, "each": true, "every": true, "some": true, "any": true,
		"help": true, "please": true, "just": true, "also": true,
		// Chinese stop words
		"的": true, "了": true, "是": true, "在": true, "和": true,
		"我": true, "你": true, "他": true, "她": true, "它": true,
		"这": true, "那": true, "有": true, "不": true, "也": true,
		"都": true, "会": true, "就": true, "还": true, "把": true,
		"吗": true, "呢": true, "吧": true, "啊": true, "帮": true,
		"请": true, "要": true, "能": true, "可以": true,
	}
	return stops[word]
}
