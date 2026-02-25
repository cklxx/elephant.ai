// Package summary provides heuristic-based summarization for Lark group
// discussions. All extraction is pure string matching — no LLM calls — so the
// output is deterministic and fast. The structured result is designed for
// injection into an LLM system prompt or for direct display in a Lark card.
package summary

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ---------- Domain types ----------

// GroupMessage represents a single message in a group chat.
type GroupMessage struct {
	SenderID   string
	SenderName string
	Content    string
	Timestamp  time.Time
	MsgType    string // "text", "image", "file"
}

// Highlight captures a notable item extracted from the discussion.
type Highlight struct {
	Type      string // "decision", "action", "question", "info"
	Content   string
	Author    string
	Timestamp time.Time
}

// GroupSummary is the structured output of a group discussion summarization.
type GroupSummary struct {
	Topic          string
	Highlights     []Highlight
	Participants   []string
	Duration       time.Duration
	MessageCount   int
	ActiveSpeakers int
	TextSummary    string
}

// SummaryConfig controls summarization thresholds and output limits.
type SummaryConfig struct {
	MaxOutputChars int
	MinMessages    int
	MinParticipants int
	HighlightLimit int
	TimeWindow     time.Duration
}

// DefaultSummaryConfig returns sensible defaults for group summarization.
func DefaultSummaryConfig() SummaryConfig {
	return SummaryConfig{
		MaxOutputChars:  1000,
		MinMessages:     5,
		MinParticipants: 2,
		HighlightLimit:  5,
		TimeWindow:      time.Hour,
	}
}

// ---------- Keyword tables ----------

var decisionKeywords = []string{
	"decided", "agreed", "let's go with", "confirmed", "approved",
}

var actionKeywords = []string{
	"will do", "todo", "action item", "i'll", "assigned to", "deadline",
}

var questionPrefixes = []string{
	"how", "what", "why", "when", "where",
}

// ---------- Public API ----------

// Summarize extracts a structured summary from the provided messages. It
// filters messages within config.TimeWindow, returns nil when message volume
// or participant count is below the configured thresholds, and produces a
// markdown TextSummary suitable for display.
func Summarize(messages []GroupMessage, config SummaryConfig) *GroupSummary {
	if len(messages) == 0 {
		return nil
	}

	// Filter to the configured time window using the latest message as anchor.
	filtered := filterByTimeWindow(messages, config.TimeWindow)
	if len(filtered) == 0 {
		return nil
	}

	// Extract unique participants.
	participants := extractParticipants(filtered)

	// Threshold checks.
	if len(filtered) < config.MinMessages {
		return nil
	}
	if len(participants) < config.MinParticipants {
		return nil
	}

	// Compute duration.
	duration := computeDuration(filtered)

	// Detect highlights (only from text messages).
	highlights := detectHighlights(filtered, config.HighlightLimit)

	summary := &GroupSummary{
		Highlights:     highlights,
		Participants:   participants,
		Duration:       duration,
		MessageCount:   len(filtered),
		ActiveSpeakers: len(participants),
	}

	summary.TextSummary = buildTextSummary(summary, config.MaxOutputChars)
	return summary
}

// ShouldAutoSummarize returns true when the message volume or participant
// diversity warrants an automatic summary: more than 20 messages in the time
// window, or more than 5 unique participants.
func ShouldAutoSummarize(messages []GroupMessage, config SummaryConfig) bool {
	if len(messages) == 0 {
		return false
	}
	filtered := filterByTimeWindow(messages, config.TimeWindow)
	if len(filtered) == 0 {
		return false
	}
	if len(filtered) > 20 {
		return true
	}
	participants := extractParticipants(filtered)
	return len(participants) > 5
}

// ---------- Internal helpers ----------

// filterByTimeWindow keeps only messages whose timestamp falls within
// [latest - window, latest]. Messages are returned in chronological order.
func filterByTimeWindow(messages []GroupMessage, window time.Duration) []GroupMessage {
	if len(messages) == 0 {
		return nil
	}

	// Find the latest timestamp.
	latest := messages[0].Timestamp
	for _, m := range messages[1:] {
		if m.Timestamp.After(latest) {
			latest = m.Timestamp
		}
	}

	cutoff := latest.Add(-window)
	filtered := make([]GroupMessage, 0, len(messages))
	for _, m := range messages {
		if !m.Timestamp.Before(cutoff) {
			filtered = append(filtered, m)
		}
	}

	// Sort chronologically.
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})
	return filtered
}

// extractParticipants returns a sorted, deduplicated list of sender names. If
// SenderName is empty SenderID is used as fallback.
func extractParticipants(messages []GroupMessage) []string {
	seen := make(map[string]struct{})
	for _, m := range messages {
		name := m.SenderName
		if name == "" {
			name = m.SenderID
		}
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// computeDuration returns the time span between the earliest and latest
// messages.
func computeDuration(messages []GroupMessage) time.Duration {
	if len(messages) < 2 {
		return 0
	}
	earliest := messages[0].Timestamp
	latest := messages[0].Timestamp
	for _, m := range messages[1:] {
		if m.Timestamp.Before(earliest) {
			earliest = m.Timestamp
		}
		if m.Timestamp.After(latest) {
			latest = m.Timestamp
		}
	}
	return latest.Sub(earliest)
}

// detectHighlights scans text messages for keyword-based highlights.
func detectHighlights(messages []GroupMessage, limit int) []Highlight {
	var highlights []Highlight
	for _, m := range messages {
		if m.MsgType != "" && m.MsgType != "text" {
			continue
		}
		if h, ok := classifyMessage(m); ok {
			highlights = append(highlights, h)
			if limit > 0 && len(highlights) >= limit {
				break
			}
		}
	}
	return highlights
}

// classifyMessage checks a single message against keyword heuristics. Decision
// keywords are checked first (highest priority), then action, then question.
func classifyMessage(m GroupMessage) (Highlight, bool) {
	lower := strings.ToLower(m.Content)
	author := m.SenderName
	if author == "" {
		author = m.SenderID
	}

	for _, kw := range decisionKeywords {
		if strings.Contains(lower, kw) {
			return Highlight{
				Type:      "decision",
				Content:   m.Content,
				Author:    author,
				Timestamp: m.Timestamp,
			}, true
		}
	}
	for _, kw := range actionKeywords {
		if strings.Contains(lower, kw) {
			return Highlight{
				Type:      "action",
				Content:   m.Content,
				Author:    author,
				Timestamp: m.Timestamp,
			}, true
		}
	}
	trimmed := strings.TrimSpace(m.Content)
	if strings.HasSuffix(trimmed, "?") {
		return Highlight{
			Type:      "question",
			Content:   m.Content,
			Author:    author,
			Timestamp: m.Timestamp,
		}, true
	}
	for _, prefix := range questionPrefixes {
		if strings.HasPrefix(lower, prefix+" ") || strings.HasPrefix(lower, prefix+",") {
			return Highlight{
				Type:      "question",
				Content:   m.Content,
				Author:    author,
				Timestamp: m.Timestamp,
			}, true
		}
	}
	return Highlight{}, false
}

// buildTextSummary renders the structured summary as markdown. The output is
// truncated to maxChars if it exceeds the budget.
func buildTextSummary(s *GroupSummary, maxChars int) string {
	var sb strings.Builder

	// Header.
	durationStr := formatDuration(s.Duration)
	sb.WriteString(fmt.Sprintf(
		"**Group Discussion Summary** (%d messages, %d participants, %s)\n\n",
		s.MessageCount, s.ActiveSpeakers, durationStr,
	))

	// Participants.
	sb.WriteString("**Participants**: ")
	sb.WriteString(strings.Join(s.Participants, ", "))
	sb.WriteString("\n")

	// Highlights.
	if len(s.Highlights) > 0 {
		sb.WriteString("\n**Key Highlights**:\n")
		for _, h := range s.Highlights {
			icon := highlightIcon(h.Type)
			label := highlightLabel(h.Type)
			sb.WriteString(fmt.Sprintf("- %s [%s] \"%s\" — %s\n", icon, label, h.Content, h.Author))
		}
	}

	// Activity.
	minutes := int(s.Duration.Minutes())
	sb.WriteString(fmt.Sprintf("\n**Activity**: %d messages over %d minutes\n", s.MessageCount, minutes))

	result := sb.String()
	if maxChars > 0 && len(result) > maxChars {
		result = result[:maxChars-3] + "..."
	}
	return result
}

// formatDuration renders a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	if minutes < 1 {
		return "< 1 min"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d min", minutes)
	}
	hours := minutes / 60
	rem := minutes % 60
	if rem == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, rem)
}

// highlightIcon returns the emoji prefix for a highlight type.
func highlightIcon(t string) string {
	switch t {
	case "decision":
		return "\U0001F3AF" // 🎯
	case "action":
		return "\u2705" // ✅
	case "question":
		return "\u2753" // ❓
	default:
		return "\u2139\uFE0F" // ℹ️
	}
}

// highlightLabel returns the display label for a highlight type.
func highlightLabel(t string) string {
	switch t {
	case "decision":
		return "Decision"
	case "action":
		return "Action"
	case "question":
		return "Question"
	default:
		return "Info"
	}
}
