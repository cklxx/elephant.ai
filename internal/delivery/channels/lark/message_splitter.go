package lark

import (
	"strings"
	"time"
)

const (
	// messageSplitMaxChunkChars is the target maximum characters per chunk.
	messageSplitMaxChunkChars = 400
	// messageSplitMaxChunks caps the number of chunks to avoid message spam.
	messageSplitMaxChunks = 5
	// messageSplitDelay is the pause between consecutive messages to
	// simulate a natural typing rhythm.
	messageSplitDelay = 500 * time.Millisecond
)

// splitMessage splits a long text into multiple chunks suitable for
// sequential delivery as chat messages. The algorithm:
//  1. Split by double-newline into paragraphs.
//  2. Merge short paragraphs up to ~400 chars.
//  3. Keep code fences (``` blocks) intact within a single chunk.
//  4. Keep consecutive numbered list lines together.
//  5. Merge trailing chunks when exceeding 5.
//  6. Return a single chunk when total length < 400 chars.
func splitMessage(text string) []string {
	maxChars := messageSplitMaxChunkChars
	maxC := messageSplitMaxChunks

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
