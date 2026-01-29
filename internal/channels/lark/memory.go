package lark

import (
	"context"
	"fmt"
	"strings"
	"time"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/logging"
	"alex/internal/memory"
)

// larkMemoryManager provides automatic memory save/recall for the Lark channel.
type larkMemoryManager struct {
	service memory.Service
	logger  logging.Logger
}

// newLarkMemoryManager creates a memory manager backed by the given service.
func newLarkMemoryManager(svc memory.Service, logger logging.Logger) *larkMemoryManager {
	return &larkMemoryManager{
		service: svc,
		logger:  logging.OrNop(logger),
	}
}

// SaveFromResult extracts important notes from the task result and persists
// them via the memory service, keyed by session/user context.
func (m *larkMemoryManager) SaveFromResult(ctx context.Context, sessionID string, result *agent.TaskResult) {
	if m == nil || m.service == nil || result == nil {
		return
	}
	if len(result.Important) == 0 {
		return
	}

	for _, note := range result.Important {
		content := strings.TrimSpace(note.Content)
		if content == "" {
			continue
		}
		entry := memory.Entry{
			UserID:  sessionID,
			Content: content,
			Keywords: note.Tags,
			Slots: map[string]string{
				"source":     note.Source,
				"session_id": sessionID,
			},
			CreatedAt: note.CreatedAt,
		}
		if entry.CreatedAt.IsZero() {
			entry.CreatedAt = time.Now()
		}
		if _, err := m.service.Save(ctx, entry); err != nil {
			m.logger.Warn("Lark memory save failed: %v", err)
		}
	}
}

// RecallForTask queries the memory service with the task text and returns
// relevant memories formatted as a context string.
func (m *larkMemoryManager) RecallForTask(ctx context.Context, sessionID, task string) string {
	if m == nil || m.service == nil {
		return ""
	}

	keywords := extractKeywords(task)
	if len(keywords) == 0 {
		return ""
	}

	entries, err := m.service.Recall(ctx, memory.Query{
		UserID:   sessionID,
		Keywords: keywords,
		Limit:    5,
	})
	if err != nil {
		m.logger.Warn("Lark memory recall failed: %v", err)
		return ""
	}
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("[相关记忆]\n")
	for i, entry := range entries {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, entry.Content))
	}
	return strings.TrimRight(b.String(), "\n")
}

// extractKeywords performs a simple tokenization of the task text for use
// as recall keywords. Filters out short tokens and common stop words.
func extractKeywords(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	words := strings.Fields(text)
	seen := make(map[string]bool, len(words))
	var keywords []string
	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, ".,!?;:\"'()[]{}"))
		if len(w) < 2 || stopWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		keywords = append(keywords, w)
		if len(keywords) >= 10 {
			break
		}
	}
	return keywords
}

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true, "need": true,
	"to": true, "of": true, "in": true, "for": true, "on": true,
	"with": true, "at": true, "by": true, "from": true, "as": true,
	"into": true, "about": true, "it": true, "its": true, "this": true,
	"that": true, "these": true, "those": true, "he": true, "she": true,
	"we": true, "they": true, "me": true, "him": true, "her": true,
	"us": true, "them": true, "my": true, "your": true, "his": true,
	"our": true, "their": true, "and": true, "but": true, "or": true,
	"not": true, "so": true, "if": true, "then": true,
	"的": true, "了": true, "是": true, "在": true, "我": true,
	"有": true, "和": true, "就": true, "不": true, "人": true,
	"都": true, "一": true, "个": true, "上": true, "也": true,
	"很": true, "到": true, "说": true, "要": true, "去": true,
	"你": true, "会": true, "着": true, "没有": true, "看": true,
	"好": true, "自己": true, "这": true,
}
