package react

import (
	"strings"

	"alex/internal/domain/agent/ports"
	tokenutil "alex/internal/shared/token"
)

// aggressiveTrimMessages keeps system/important/checkpoint messages and the
// last N user-initiated turns, inserting a compression summary for removed
// messages. This is a package-internal wrapper around the same logic used by
// the context package's AggressiveTrim.
func aggressiveTrimMessages(messages []ports.Message, maxTurns int) []ports.Message {
	if maxTurns <= 0 {
		maxTurns = 1
	}

	var preserved, conversation []ports.Message
	systemPromptKept := false
	for _, msg := range messages {
		switch {
		case msg.Source == ports.MessageSourceImportant, msg.Source == ports.MessageSourceCheckpoint:
			preserved = append(preserved, msg)
		case msg.Source == ports.MessageSourceSystemPrompt:
			if !systemPromptKept && isPrimarySystemPromptForTrim(msg) {
				preserved = append(preserved, msg)
				systemPromptKept = true
				continue
			}
			conversation = append(conversation, msg)
		default:
			conversation = append(conversation, msg)
		}
	}

	kept := ports.KeepRecentTurns(conversation, maxTurns)

	result := make([]ports.Message, 0, len(preserved)+len(kept)+1)
	result = append(result, preserved...)
	if len(kept) > 0 && len(conversation) > len(kept) {
		result = append(result, ports.Message{
			Role:    "assistant",
			Content: "[Context trimmed to fit model window. Earlier conversation was removed.]",
			Source:  ports.MessageSourceUserHistory,
		})
	}
	result = append(result, kept...)
	return result
}

func isPrimarySystemPromptForTrim(msg ports.Message) bool {
	role := strings.ToLower(strings.TrimSpace(msg.Role))
	return role == "system" && strings.TrimSpace(msg.Content) != ""
}

const (
	// maxHalveAttempts: 24 halvings can reduce a 128K-token message to <1 token.
	maxHalveAttempts = 24
	// halveFloorTokens: stop halving when a message is below this threshold.
	halveFloorTokens = 64
	// systemPromptCapTokens: hard cap for system prompt in minimal payload.
	systemPromptCapTokens = 512
	// lastMessageCapTokens: hard cap for last message in minimal payload.
	lastMessageCapTokens = 256
	// maxMinimalHalveAttempts: attempts for the final minimal-payload halving pass.
	maxMinimalHalveAttempts = 12
	// minimalHalveFloorTokens: floor for the minimal-payload halving pass.
	minimalHalveFloorTokens = 32
)

func forceFitMessagesToLimit(messages []ports.Message, limit int, estimate func([]ports.Message) int) []ports.Message {
	if len(messages) == 0 || limit <= 0 || estimate == nil {
		return messages
	}

	fitted := append([]ports.Message(nil), messages...)
	if estimate(fitted) <= limit {
		return fitted
	}

	// Phase 1: iteratively halve the largest message until we fit or run out.
	for attempt := 0; attempt < maxHalveAttempts && estimate(fitted) > limit; attempt++ {
		idx := indexOfLargestMessageContent(fitted)
		if idx < 0 {
			break
		}
		content := strings.TrimSpace(fitted[idx].Content)
		if content == "" {
			fitted[idx].Content = "[context truncated]"
			continue
		}
		currentTokens := tokenutil.CountTokens(content)
		if currentTokens <= halveFloorTokens {
			fitted[idx].Content = "[context truncated]"
			continue
		}
		fitted[idx].Content = tokenutil.TruncateToTokens(content, currentTokens/2)
	}
	if estimate(fitted) <= limit {
		return fitted
	}

	// Phase 2: deterministic minimal payload (canonical system + latest message).
	systemIdx := -1
	for i, msg := range messages {
		if msg.Source == ports.MessageSourceSystemPrompt {
			systemIdx = i
			break
		}
	}
	if systemIdx < 0 {
		systemIdx = 0
	}
	lastIdx := len(messages) - 1

	minimal := make([]ports.Message, 0, 3)
	sys := messages[systemIdx]
	sysContent := strings.TrimSpace(sys.Content)
	if sysContent != "" {
		sys.Content = tokenutil.TruncateToTokens(sysContent, systemPromptCapTokens)
	}
	minimal = append(minimal, sys)

	if lastIdx != systemIdx {
		last := messages[lastIdx]
		lastContent := strings.TrimSpace(last.Content)
		if lastContent != "" {
			last.Content = tokenutil.TruncateToTokens(lastContent, lastMessageCapTokens)
		}
		minimal = append(minimal, ports.Message{
			Role:    "assistant",
			Content: "[Additional context truncated to satisfy model window.]",
			Source:  ports.MessageSourceUserHistory,
		})
		minimal = append(minimal, last)
	}

	// Final nudge if still above limit: repeatedly halve largest content.
	for attempt := 0; attempt < maxMinimalHalveAttempts && estimate(minimal) > limit; attempt++ {
		idx := indexOfLargestMessageContent(minimal)
		if idx < 0 {
			break
		}
		content := strings.TrimSpace(minimal[idx].Content)
		if content == "" {
			minimal[idx].Content = "[context truncated]"
			continue
		}
		tokens := tokenutil.CountTokens(content)
		if tokens <= minimalHalveFloorTokens {
			minimal[idx].Content = "[context truncated]"
			continue
		}
		minimal[idx].Content = tokenutil.TruncateToTokens(content, tokens/2)
	}

	return minimal
}

func indexOfLargestMessageContent(messages []ports.Message) int {
	longestIdx := -1
	longest := 0
	for i, msg := range messages {
		length := len([]rune(strings.TrimSpace(msg.Content)))
		if length > longest {
			longest = length
			longestIdx = i
		}
	}
	return longestIdx
}
