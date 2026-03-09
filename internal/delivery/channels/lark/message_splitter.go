package lark

import (
	"strings"
	"time"
)

// MessageSplitConfig controls automatic splitting of long replies into
// multiple short messages for a more conversational feel.
type MessageSplitConfig struct {
	Enabled       bool          `yaml:"enabled"`
	MaxChunkChars int           `yaml:"max_chunk_chars"` // default 400
	MaxChunks     int           `yaml:"max_chunks"`      // default 5
	DelayBetween  time.Duration `yaml:"delay_between"`   // default 500ms
}

func (c MessageSplitConfig) maxChunkChars() int {
	if c.MaxChunkChars > 0 {
		return c.MaxChunkChars
	}
	return 400
}

func (c MessageSplitConfig) maxChunks() int {
	if c.MaxChunks > 0 {
		return c.MaxChunks
	}
	return 5
}

func (c MessageSplitConfig) delayBetween() time.Duration {
	if c.DelayBetween > 0 {
		return c.DelayBetween
	}
	return 500 * time.Millisecond
}

// splitMessage splits a long text into multiple chunks suitable for
// sequential delivery as chat messages. The algorithm:
//  1. Split by double-newline into paragraphs.
//  2. Merge short paragraphs up to ~maxChunkChars.
//  3. Keep code fences (``` blocks) intact within a single chunk.
//  4. Keep consecutive numbered list lines together.
//  5. Merge trailing chunks when exceeding maxChunks.
//  6. Return a single chunk when total length < maxChunkChars.
func splitMessage(text string, cfg MessageSplitConfig) []string {
	maxChars := cfg.maxChunkChars()
	maxC := cfg.maxChunks()

	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}

	// Short text — no split needed.
	if len([]rune(text)) <= maxChars {
		return []string{text}
	}

	// Split into semantic segments (paragraphs, code fences, numbered lists).
	segments := splitIntoSegments(text)
	if len(segments) == 0 {
		return []string{text}
	}

	// Merge short segments into chunks up to maxChars.
	chunks := mergeSegments(segments, maxChars)

	// Enforce maxChunks by merging trailing chunks.
	if len(chunks) > maxC {
		merged := make([]string, maxC)
		copy(merged, chunks[:maxC-1])
		merged[maxC-1] = strings.Join(chunks[maxC-1:], "\n\n")
		chunks = merged
	}

	if len(chunks) == 0 {
		return []string{text}
	}
	return chunks
}

// splitIntoSegments splits text into semantic segments that should not be
// broken apart: paragraphs, code fence blocks, and consecutive numbered lists.
func splitIntoSegments(text string) []string {
	lines := strings.Split(text, "\n")
	var segments []string
	var current []string
	inCodeFence := false

	flushCurrent := func() {
		if len(current) == 0 {
			return
		}
		seg := strings.TrimSpace(strings.Join(current, "\n"))
		if seg != "" {
			segments = append(segments, seg)
		}
		current = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Toggle code fence state.
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeFence {
				// Starting a code fence — flush anything before it.
				flushCurrent()
				inCodeFence = true
				current = append(current, line)
				continue
			}
			// Closing a code fence.
			current = append(current, line)
			inCodeFence = false
			flushCurrent()
			continue
		}

		if inCodeFence {
			current = append(current, line)
			continue
		}

		// Empty line = paragraph break (outside code fence).
		if trimmed == "" {
			flushCurrent()
			continue
		}

		// Check if this is a numbered list continuation.
		if isNumberedListLine(trimmed) && len(current) > 0 && isNumberedListLine(strings.TrimSpace(current[len(current)-1])) {
			current = append(current, line)
			continue
		}

		// Non-numbered line after a numbered list — break.
		if len(current) > 0 && isNumberedListLine(strings.TrimSpace(current[0])) && !isNumberedListLine(trimmed) {
			flushCurrent()
		}

		current = append(current, line)
	}

	// If we ended inside a code fence, flush whatever we have.
	flushCurrent()
	return segments
}

// mergeSegments combines small segments into chunks that don't exceed maxChars.
func mergeSegments(segments []string, maxChars int) []string {
	var chunks []string
	var currentParts []string
	currentLen := 0

	for _, seg := range segments {
		segLen := len([]rune(seg))

		// If adding this segment would exceed the limit, flush.
		if currentLen > 0 && currentLen+segLen+2 > maxChars { // +2 for "\n\n" separator
			chunks = append(chunks, strings.Join(currentParts, "\n\n"))
			currentParts = nil
			currentLen = 0
		}

		currentParts = append(currentParts, seg)
		if currentLen > 0 {
			currentLen += 2 // "\n\n"
		}
		currentLen += segLen
	}

	if len(currentParts) > 0 {
		chunks = append(chunks, strings.Join(currentParts, "\n\n"))
	}
	return chunks
}

// isNumberedListLine checks if a line starts with a numbered list pattern (e.g. "1.", "2.").
func isNumberedListLine(line string) bool {
	for i, ch := range line {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '.' && i > 0 {
			return true
		}
		return false
	}
	return false
}
