// Package tokenutil provides a centralized token counting utility backed by
// tiktoken-go. It lazily initializes the cl100k_base encoding (GPT-3.5/4 and
// Claude compatible) on first use and falls back to a character-based heuristic
// if initialization fails.
package tokenutil

import (
	"strings"
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

var (
	once     sync.Once
	encoding *tiktoken.Tiktoken
)

func init() {
	// Eager initialization so the first call doesn't pay the latency.
	initEncoding()
}

func initEncoding() {
	once.Do(func() {
		enc, err := tiktoken.GetEncoding("cl100k_base")
		if err == nil {
			encoding = enc
		}
	})
}

// CountTokens returns an accurate token count using cl100k_base encoding.
// If tiktoken is unavailable, it falls back to EstimateFast.
func CountTokens(text string) int {
	if encoding != nil {
		return len(encoding.Encode(text, nil, nil))
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
	if encoding != nil {
		tokens := encoding.Encode(text, nil, nil)
		if len(tokens) <= maxTokens {
			return text
		}
		return encoding.Decode(tokens[:maxTokens]) + "..."
	}
	// Fallback: character-based heuristic
	runes := []rune(text)
	limit := maxTokens * 4
	if limit >= len(runes) {
		return text
	}
	return string(runes[:limit]) + "..."
}
