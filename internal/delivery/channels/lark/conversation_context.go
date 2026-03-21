package lark

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	conversationContextMaxEntries = 20
	conversationContextTTL        = 30 * time.Minute
)

// contextEntry records a single tool invocation for sliding context.
type contextEntry struct {
	Kind      string    // tool name that was called
	Summary   string    // deterministic: extracted from tool args
	Timestamp time.Time
}

// chatConversationContext holds a per-chat sliding window of recent tool calls
// so the conversation LLM has memory of what was discussed.
type chatConversationContext struct {
	mu      sync.Mutex
	entries []contextEntry
	updated time.Time
}

// record appends a new entry, evicting the oldest if at capacity.
func (c *chatConversationContext) record(kind, summary string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= conversationContextMaxEntries {
		c.entries = c.entries[1:]
	}
	c.entries = append(c.entries, contextEntry{
		Kind:      kind,
		Summary:   summary,
		Timestamp: now,
	})
	c.updated = now
}

// isExpired reports whether the context has not been updated within the TTL.
func (c *chatConversationContext) isExpired(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.updated.IsZero() && now.Sub(c.updated) > conversationContextTTL
}

// render formats the sliding context for injection into the LLM user message.
// Returns empty string if no entries exist.
func (c *chatConversationContext) render() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Recent actions in this chat:\n")
	for _, e := range c.entries {
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", e.Timestamp.Format("15:04"), e.Kind, e.Summary))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// getOrCreateChatContext returns (or lazily creates) the conversation context for a chat.
func (g *Gateway) getOrCreateChatContext(chatID string) *chatConversationContext {
	v, _ := g.chatContexts.LoadOrStore(chatID, &chatConversationContext{})
	return v.(*chatConversationContext)
}

// recordToolContext records a tool invocation in the per-chat sliding context.
// The summary is deterministic (extracted from tool args, no LLM call).
func (g *Gateway) recordToolContext(chatID, toolName string, args map[string]any) {
	summary := buildToolContextSummary(toolName, args)
	if summary == "" {
		return
	}
	ctx := g.getOrCreateChatContext(chatID)
	ctx.record(toolName, summary, g.currentTime())
}

// buildToolContextSummary produces a deterministic one-line summary from tool args.
func buildToolContextSummary(toolName string, args map[string]any) string {
	switch toolName {
	case dispatchWorkerToolName:
		task, _ := args["task"].(string)
		if len([]rune(task)) > 60 {
			task = string([]rune(task)[:60]) + "..."
		}
		return fmt.Sprintf("dispatched worker: %s", task)
	case stopWorkerToolName:
		taskID, _ := args["task_id"].(string)
		if taskID == "" {
			return "stopped all workers"
		}
		return fmt.Sprintf("stopped worker %s", taskID)
	case queryTasksToolName:
		scope, _ := args["scope"].(string)
		return fmt.Sprintf("queried tasks (scope=%s)", scope)
	case queryUsageToolName:
		period, _ := args["period"].(string)
		if period == "" {
			period = "today"
		}
		return fmt.Sprintf("queried usage (period=%s)", period)
	case manageNoticeToolName:
		action, _ := args["action"].(string)
		return fmt.Sprintf("manage notice (action=%s)", action)
	default:
		return toolName
	}
}

// evictExpiredChatContexts removes stale chat context entries.
// Called by the state cleanup goroutine.
func (g *Gateway) evictExpiredChatContexts() {
	now := g.currentTime()
	g.chatContexts.Range(func(k, v any) bool {
		ctx, ok := v.(*chatConversationContext)
		if !ok {
			g.chatContexts.Delete(k)
			return true
		}
		if ctx.isExpired(now) {
			g.chatContexts.Delete(k)
		}
		return true
	})
}
