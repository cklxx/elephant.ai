package context

import (
	"sort"
	"strings"

	"alex/internal/domain/agent/ports"
)

// MessagePriority represents the importance score of a message.
// Range: 0.0 (lowest) to 1.0 (highest).
type MessagePriority = float64

// RankedMessage pairs a conversation message with its computed priority and
// a human-readable reason explaining how the score was derived.
type RankedMessage struct {
	Message  ports.Message
	Priority MessagePriority
	Reason   string
	// index preserves the original chronological position so output ordering
	// can be restored after priority-based selection.
	index int
}

// SourceWeights maps each MessageSource to its base priority score.
type SourceWeights map[ports.MessageSource]float64

// DefaultSourceWeights defines the default base score for every known
// MessageSource. Callers can override individual entries to tune ranking
// without forking the ranker.
var DefaultSourceWeights = SourceWeights{
	ports.MessageSourceSystemPrompt:   1.0,
	ports.MessageSourceImportant:      0.95,
	ports.MessageSourceUserInput:      0.85,
	ports.MessageSourceProactive:      0.80,
	ports.MessageSourceAssistantReply: 0.70,
	ports.MessageSourceToolResult:     0.60,
	ports.MessageSourceUserHistory:    0.50,
	ports.MessageSourceEvaluation:     0.30,
	ports.MessageSourceDebug:          0.20,
	ports.MessageSourceUnknown:        0.40,
}

const (
	// maxRecencyBonus is the upper bound of the positional recency bonus.
	maxRecencyBonus float64 = 0.15
	// toolCallBonus is added when a message carries tool call data.
	toolCallBonus float64 = 0.05
	// errorContentBonus is added when a message content hints at an error.
	errorContentBonus float64 = 0.10
)

// MessageRanker scores conversation messages by importance. The zero value
// uses DefaultSourceWeights; supply custom weights via NewMessageRanker.
type MessageRanker struct {
	Weights SourceWeights
}

// NewMessageRanker creates a ranker with the provided source weights.
// A nil weights argument falls back to DefaultSourceWeights.
func NewMessageRanker(weights SourceWeights) *MessageRanker {
	if weights == nil {
		weights = DefaultSourceWeights
	}
	return &MessageRanker{Weights: weights}
}

// RankMessages scores each message and returns a slice of RankedMessage in the
// same chronological order as the input. Priorities are clamped to [0.0, 1.0].
func (r *MessageRanker) RankMessages(messages []ports.Message) []RankedMessage {
	n := len(messages)
	ranked := make([]RankedMessage, n)

	weights := r.Weights
	if weights == nil {
		weights = DefaultSourceWeights
	}

	for i, msg := range messages {
		base := lookupSourceWeight(weights, msg.Source)
		recency := recencyBonus(i, n)
		content := contentSignalBonus(msg)
		priority := base + recency + content

		// Clamp to [0.0, 1.0].
		if priority > 1.0 {
			priority = 1.0
		}
		if priority < 0.0 {
			priority = 0.0
		}

		reason := buildReason(msg.Source, recency, content)
		ranked[i] = RankedMessage{
			Message:  msg,
			Priority: priority,
			Reason:   reason,
			index:    i,
		}
	}
	return ranked
}

// SelectTopN picks the highest-priority messages whose cumulative token count
// fits within tokenBudget. The returned slice preserves the original
// chronological order of the selected messages.
//
// estimateTokensFn estimates the token count for a single content string. It
// must not be nil.
func SelectTopN(ranked []RankedMessage, tokenBudget int, estimateTokensFn func(string) int) []RankedMessage {
	if len(ranked) == 0 || tokenBudget <= 0 {
		return nil
	}

	// Sort a copy by descending priority so we greedily pick the best first.
	byPriority := make([]RankedMessage, len(ranked))
	copy(byPriority, ranked)
	sort.SliceStable(byPriority, func(i, j int) bool {
		return byPriority[i].Priority > byPriority[j].Priority
	})

	var selected []RankedMessage
	tokensUsed := 0
	for _, rm := range byPriority {
		cost := estimateTokensFn(rm.Message.Content)
		if tokensUsed+cost > tokenBudget {
			continue
		}
		tokensUsed += cost
		selected = append(selected, rm)
	}

	// Restore chronological order by original index.
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].index < selected[j].index
	})
	return selected
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func lookupSourceWeight(weights SourceWeights, source ports.MessageSource) float64 {
	if w, ok := weights[source]; ok {
		return w
	}
	// Fall back to Unknown weight when the source is not mapped.
	if w, ok := weights[ports.MessageSourceUnknown]; ok {
		return w
	}
	return 0.40
}

// recencyBonus returns a positional bonus in [0.0, maxRecencyBonus].
// The last message receives the full bonus; the first message receives 0.
func recencyBonus(position, total int) float64 {
	if total <= 1 {
		return 0
	}
	return maxRecencyBonus * float64(position) / float64(total-1)
}

// contentSignalBonus detects high-value content signals and returns an additive
// bonus score.
func contentSignalBonus(msg ports.Message) float64 {
	bonus := 0.0
	if len(msg.ToolCalls) > 0 || len(msg.ToolResults) > 0 {
		bonus += toolCallBonus
	}
	if containsErrorSignal(msg.Content) {
		bonus += errorContentBonus
	}
	return bonus
}

// containsErrorSignal performs a case-insensitive check for common error
// indicators in message content.
func containsErrorSignal(content string) bool {
	lower := strings.ToLower(content)
	errorKeywords := []string{"error", "fail", "exception", "panic", "fatal"}
	for _, kw := range errorKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func buildReason(source ports.MessageSource, recency, content float64) string {
	var parts []string
	parts = append(parts, "base="+sourceLabel(source))
	if recency > 0 {
		parts = append(parts, "recency")
	}
	if content > 0 {
		parts = append(parts, "content_signal")
	}
	return strings.Join(parts, "+")
}

func sourceLabel(source ports.MessageSource) string {
	if source == "" {
		return "unknown"
	}
	return string(source)
}
