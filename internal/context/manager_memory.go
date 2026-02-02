package context

import (
	"context"
	"fmt"
	"strings"
	"time"

	storage "alex/internal/agent/ports/storage"
	id "alex/internal/utils/id"
)

const (
	maxMemorySnapshotChars = 10000
	maxMemorySectionChars  = 4000
)

func (m *manager) memoryEnabled(ctx context.Context) bool {
	if m == nil || m.memoryEngine == nil {
		return false
	}
	if m.memoryGate == nil {
		return true
	}
	return m.memoryGate(ctx)
}

func (m *manager) loadMemorySnapshot(ctx context.Context, session *storage.Session) string {
	if !m.memoryEnabled(ctx) {
		return ""
	}

	userID := resolveMemoryUserID(ctx, session)
	now := time.Now()

	longTerm, _ := m.memoryEngine.LoadLongTerm(ctx, userID)
	today, _ := m.memoryEngine.LoadDaily(ctx, userID, now)
	yesterday, _ := m.memoryEngine.LoadDaily(ctx, userID, now.AddDate(0, 0, -1))

	longTerm = truncateMemorySection(longTerm, maxMemorySectionChars)
	today = truncateMemorySection(today, maxMemorySectionChars)
	yesterday = truncateMemorySection(yesterday, maxMemorySectionChars)

	var sections []string
	if longTerm != "" {
		sections = append(sections, fmt.Sprintf("## Long-term Memory (MEMORY.md)\n%s", longTerm))
	}
	if today != "" {
		sections = append(sections, fmt.Sprintf("## Daily Log (%s)\n%s", now.Format("2006-01-02"), today))
	}
	if yesterday != "" {
		sections = append(sections, fmt.Sprintf("## Daily Log (%s)\n%s", now.AddDate(0, 0, -1).Format("2006-01-02"), yesterday))
	}
	if len(sections) == 0 {
		return ""
	}

	return truncateMemorySection(strings.Join(sections, "\n\n"), maxMemorySnapshotChars)
}

func resolveMemoryUserID(ctx context.Context, session *storage.Session) string {
	if ctx != nil {
		if uid := strings.TrimSpace(id.UserIDFromContext(ctx)); uid != "" {
			return uid
		}
	}
	if session != nil && session.Metadata != nil {
		if uid := strings.TrimSpace(session.Metadata["user_id"]); uid != "" {
			return uid
		}
	}
	if session != nil && strings.HasPrefix(session.ID, "lark-") {
		return session.ID
	}
	return ""
}

func truncateMemorySection(content string, limit int) string {
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
