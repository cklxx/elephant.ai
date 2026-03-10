// Package tokenutil provides a centralized token counting utility backed by
// tiktoken-go. It initializes the cl100k_base encoding once at package load
// time and falls back to a character-based heuristic if initialization fails.
package tokenutil

import (
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

var encoding = loadEncoding()

func loadEncoding() *tiktoken.Tiktoken {
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil
	}
	return enc
}

// CountTokens returns an accurate token count using cl100k_base encoding.
// If tiktoken is unavailable, it falls back to EstimateFast.
func CountTokens(text string) int {
	if tokens, ok := encodedTokens(text); ok {
		return len(tokens)
	}
	return EstimateFast(text)
}

// EstimateFast returns a heuristic token estimate: max(runes/4, word_count).
// Use this only when the tiktoken overhead is unacceptable (e.g. tight loops
// over very large text).
func EstimateFast(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	runes := len([]rune(trimmed))
	words := len(strings.Fields(trimmed))
	estimate := runes / 4
	if estimate < words {
		estimate = words
	}
	if estimate == 0 {
		estimate = 1
	}
	return estimate
}

// TruncateToTokens truncates text to approximately maxTokens.
// Uses tiktoken for accurate truncation when available.
func TruncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return text
	}
	if tokens, ok := encodedTokens(text); ok {
		if len(tokens) <= maxTokens {
			return text
		}
		return encoding.Decode(tokens[:maxTokens]) + "..."
	}
	return truncateRunesApprox(text, maxTokens)
}

func encodedTokens(text string) ([]int, bool) {
	if encoding == nil {
		return nil, false
	}
	return encoding.Encode(text, nil, nil), true
}

func truncateRunesApprox(text string, maxTokens int) string {
	runes := []rune(text)
	limit := maxTokens * 4
	if limit >= len(runes) {
		return text
	}
	return string(runes[:limit]) + "..."
}
