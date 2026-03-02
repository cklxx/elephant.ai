package ports

import "strings"

// KeepRecentTurns returns the last N user-initiated turns from a conversation
// slice. A turn starts with a user message and includes all following
// assistant/tool messages until the next user message.
func KeepRecentTurns(messages []Message, maxTurns int) []Message {
	if len(messages) == 0 || maxTurns <= 0 {
		return nil
	}

	var turnStarts []int
	for i, msg := range messages {
		if strings.EqualFold(strings.TrimSpace(msg.Role), "user") {
			turnStarts = append(turnStarts, i)
		}
	}

	if len(turnStarts) == 0 {
		if len(messages) <= maxTurns {
			return messages
		}
		return messages[len(messages)-maxTurns:]
	}

	start := 0
	if len(turnStarts) > maxTurns {
		start = turnStarts[len(turnStarts)-maxTurns]
	} else {
		start = turnStarts[0]
	}

	return messages[start:]
}

// IsPreservedSource returns true for message sources that must not be compressed
// or trimmed (system prompts, important instructions, checkpoints).
func IsPreservedSource(source MessageSource) bool {
	switch source {
	case MessageSourceSystemPrompt, MessageSourceImportant, MessageSourceCheckpoint:
		return true
	default:
		return false
	}
}

// Synthetic message prefixes used by compression and compaction. Shared across
// the context manager and react engine to detect synthetic messages consistently.
const (
	CompressionSummaryPrefix  = "[Earlier context compressed]"
	TrimNoticeSummaryPrefix   = "[Context trimmed to fit model window."
	ArtifactPlaceholderPrefix = "[CTX_PLACEHOLDER"
)

// IsSyntheticSummary returns true if the message content starts with any
// compression/compaction prefix.
func IsSyntheticSummary(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, CompressionSummaryPrefix) ||
		strings.HasPrefix(trimmed, TrimNoticeSummaryPrefix) ||
		strings.HasPrefix(trimmed, ArtifactPlaceholderPrefix)
}

// TruncateRuneSnippet returns the first `limit` runes of a trimmed string,
// appending "…" if truncated.
func TruncateRuneSnippet(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || limit <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}
