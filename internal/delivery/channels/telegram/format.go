package telegram

import (
	"strings"

	"alex/internal/shared/utils"
)

const telegramMaxMessageLen = 4096

// splitForTelegram splits text into chunks that fit within the Telegram message
// size limit, preferring to break at line boundaries.
func splitForTelegram(text string, limit int) []string {
	if limit <= 0 {
		limit = telegramMaxMessageLen
	}
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= limit {
			chunks = append(chunks, text)
			break
		}
		// Find a good break point: last newline within the limit.
		cut := limit
		if idx := strings.LastIndex(text[:limit], "\n"); idx > 0 {
			cut = idx + 1 // include the newline in the current chunk
		}
		chunks = append(chunks, text[:cut])
		text = text[cut:]
	}
	return chunks
}

// truncateWithEllipsis truncates text to limit, appending "..." if truncated.
func truncateWithEllipsis(text string, limit int) string {
	if len([]rune(text)) <= limit {
		return text
	}
	if limit <= 3 {
		return utils.TruncateWithSuffix(text, limit, "")
	}
	return utils.TruncateWithSuffix(text, limit-3, "...")
}
