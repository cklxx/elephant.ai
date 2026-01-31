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
	minRecallTextLen     = 12
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
	trimmedInput := strings.TrimSpace(task.TaskInput)
	userID := task.UserID
	if userID == "" {
		userID = "default"
	}

	keywords := extractKeywords(trimmedInput)
	if len(keywords) == 0 && len([]rune(trimmedInput)) < minRecallTextLen {
		return nil
	}

	entries, err := h.memoryService.Recall(ctx, memory.Query{
		UserID:   userID,
		Text:     trimmedInput,
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
					Text:     trimmedInput,
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
