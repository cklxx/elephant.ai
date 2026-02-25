package react

import (
	"strings"

	"alex/internal/domain/agent/ports"
)

const (
	compressionSummaryPrefix      = "[Earlier context compressed]"
	legacyTrimNoticeSummaryPrefix = "[Context trimmed to fit model window."
)

// normalizeContextMessages rewrites legacy synthetic messages so they don't
// keep inflating system instructions in codex-style requests:
//  1. Compression summaries are normalized to assistant/user_history.
//  2. Repeated compression summaries are collapsed to the latest one.
//  3. user_history messages are never kept as system/developer role.
func normalizeContextMessages(state *TaskState) {
	if state == nil || len(state.Messages) == 0 {
		return
	}

	for idx := range state.Messages {
		msg := &state.Messages[idx]
		if isCompressionSummaryContent(msg.Content) {
			msg.Role = "assistant"
			msg.Source = ports.MessageSourceUserHistory
			continue
		}

		if msg.Source == ports.MessageSourceUserHistory {
			role := strings.ToLower(strings.TrimSpace(msg.Role))
			if role == "" || role == "system" || role == "developer" {
				msg.Role = "assistant"
			}
		}
	}

	keepLastCompressionSummaryOnly(state)
}

func keepLastCompressionSummaryOnly(state *TaskState) {
	lastSummaryIdx := -1
	for idx := range state.Messages {
		if isCompressionSummaryContent(state.Messages[idx].Content) {
			lastSummaryIdx = idx
		}
	}
	if lastSummaryIdx < 0 {
		return
	}

	filtered := make([]Message, 0, len(state.Messages))
	for idx := range state.Messages {
		msg := state.Messages[idx]
		if isCompressionSummaryContent(msg.Content) && idx != lastSummaryIdx {
			continue
		}
		filtered = append(filtered, msg)
	}
	state.Messages = filtered
}

func isCompressionSummaryContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, compressionSummaryPrefix) ||
		strings.HasPrefix(trimmed, legacyTrimNoticeSummaryPrefix)
}
