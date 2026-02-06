package context

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
)

// FlushBeforeCompactionHook is called before context compression to allow
// extraction and persistence of key information from the about-to-be-compressed
// messages.
type FlushBeforeCompactionHook interface {
	OnBeforeCompaction(ctx context.Context, messages []ports.Message) error
}

// MemoryFlushHook extracts key information from compressible messages and
// persists it via the provided save function. The extraction is purely
// string-based (no LLM calls).
type MemoryFlushHook struct {
	saveFn func(ctx context.Context, content string, metadata map[string]string) error
}

// NewMemoryFlushHook constructs a hook that persists extracted conversation
// fragments before they are compressed away.
func NewMemoryFlushHook(saveFn func(ctx context.Context, content string, metadata map[string]string) error) *MemoryFlushHook {
	return &MemoryFlushHook{saveFn: saveFn}
}

const flushMaxChars = 2000

// OnBeforeCompaction iterates messages, collects user inputs, assistant replies,
// and tool result summaries, then calls the save function with a structured text
// block capped at 2000 characters.
func (h *MemoryFlushHook) OnBeforeCompaction(ctx context.Context, messages []ports.Message) error {
	if h.saveFn == nil {
		return nil
	}
	content := extractFlushContent(messages)
	if content == "" {
		return nil
	}
	metadata := map[string]string{
		"type":   "compaction_flush",
		"source": "context_compaction",
	}
	return h.saveFn(ctx, content, metadata)
}

// extractFlushContent builds a structured text block from the messages.
func extractFlushContent(messages []ports.Message) string {
	var sections []string

	var userMsgs []string
	var assistantMsgs []string
	var toolSummaries []string

	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		switch role {
		case "user":
			snippet := truncateSnippet(msg.Content, 200)
			if snippet != "" {
				userMsgs = append(userMsgs, snippet)
			}
		case "assistant":
			snippet := truncateSnippet(msg.Content, 200)
			if snippet != "" {
				assistantMsgs = append(assistantMsgs, snippet)
			}
			// Summarize tool calls embedded in assistant messages.
			for _, tc := range msg.ToolCalls {
				summary := fmt.Sprintf("tool_call:%s", tc.Name)
				toolSummaries = append(toolSummaries, summary)
			}
		case "tool":
			name := "tool"
			if msg.ToolCallID != "" {
				name = msg.ToolCallID
			}
			output := truncateSnippet(msg.Content, 100)
			toolSummaries = append(toolSummaries, fmt.Sprintf("%s -> %s", name, output))
		}

		// Summarize inline tool results regardless of role.
		for _, tr := range msg.ToolResults {
			name := tr.CallID
			if name == "" {
				name = "tool"
			}
			output := truncateSnippet(tr.Content, 100)
			toolSummaries = append(toolSummaries, fmt.Sprintf("%s -> %s", name, output))
		}
	}

	if len(userMsgs) > 0 {
		sections = append(sections, fmt.Sprintf("[User messages]\n%s", strings.Join(userMsgs, "\n")))
	}
	if len(assistantMsgs) > 0 {
		sections = append(sections, fmt.Sprintf("[Assistant replies]\n%s", strings.Join(assistantMsgs, "\n")))
	}
	if len(toolSummaries) > 0 {
		sections = append(sections, fmt.Sprintf("[Tool results]\n%s", strings.Join(toolSummaries, "\n")))
	}

	result := strings.Join(sections, "\n\n")
	return truncateSnippet(result, flushMaxChars)
}

// truncateSnippet trims and truncates content to the given rune limit.
func truncateSnippet(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || limit <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}

// NoopFlushHook is the default hook when no flush handler is configured.
// It performs no work.
type NoopFlushHook struct{}

// OnBeforeCompaction is a no-op.
func (NoopFlushHook) OnBeforeCompaction(_ context.Context, _ []ports.Message) error {
	return nil
}
