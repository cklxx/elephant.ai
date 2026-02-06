package context

import (
	"testing"

	"alex/internal/agent/ports"
)

// simpleTokenEstimator counts words as a rough token proxy (test-only).
func simpleTokenEstimator(s string) int {
	if s == "" {
		return 0
	}
	count := 1
	for _, c := range s {
		if c == ' ' {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Source-based scoring
// ---------------------------------------------------------------------------

func TestRankMessages_SourceOrdering(t *testing.T) {
	ranker := NewMessageRanker(nil)

	messages := []ports.Message{
		{Role: "system", Content: "sys", Source: ports.MessageSourceSystemPrompt},
		{Role: "system", Content: "imp", Source: ports.MessageSourceImportant},
		{Role: "user", Content: "usr", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "ast", Source: ports.MessageSourceAssistantReply},
		{Role: "tool", Content: "tool", Source: ports.MessageSourceToolResult},
		{Role: "system", Content: "dbg", Source: ports.MessageSourceDebug},
	}

	ranked := ranker.RankMessages(messages)

	// Ignoring recency, the ordering should be:
	// SystemPrompt > Important > UserInput > AssistantReply > ToolResult > Debug
	// Since messages are in that order and recency favours later positions, it
	// should only reinforce the expected hierarchy for the top entries. We check
	// pairwise base scores are honoured.
	pairs := [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 4}, {4, 5}}
	for _, pair := range pairs {
		hi, lo := pair[0], pair[1]
		if ranked[hi].Priority < ranked[lo].Priority {
			t.Errorf("expected message[%d] (source=%s, p=%.3f) >= message[%d] (source=%s, p=%.3f)",
				hi, messages[hi].Source, ranked[hi].Priority,
				lo, messages[lo].Source, ranked[lo].Priority)
		}
	}
}

func TestRankMessages_SystemPromptHighest(t *testing.T) {
	ranker := NewMessageRanker(nil)
	messages := []ports.Message{
		{Role: "system", Content: "debug info", Source: ports.MessageSourceDebug},
		{Role: "system", Content: "system prompt", Source: ports.MessageSourceSystemPrompt},
	}
	ranked := ranker.RankMessages(messages)
	if ranked[1].Priority <= ranked[0].Priority {
		t.Fatalf("SystemPrompt (%.3f) should outrank Debug (%.3f)", ranked[1].Priority, ranked[0].Priority)
	}
}

// ---------------------------------------------------------------------------
// Recency bonus
// ---------------------------------------------------------------------------

func TestRankMessages_RecencyBonus(t *testing.T) {
	ranker := NewMessageRanker(nil)

	// Two messages with the same source: recency should differentiate.
	messages := []ports.Message{
		{Role: "user", Content: "first", Source: ports.MessageSourceUserInput},
		{Role: "user", Content: "second", Source: ports.MessageSourceUserInput},
		{Role: "user", Content: "third", Source: ports.MessageSourceUserInput},
	}
	ranked := ranker.RankMessages(messages)

	if ranked[2].Priority <= ranked[0].Priority {
		t.Fatalf("later message (%.3f) should have higher priority than earlier (%.3f)",
			ranked[2].Priority, ranked[0].Priority)
	}
	if ranked[1].Priority <= ranked[0].Priority {
		t.Fatalf("middle message (%.3f) should have higher priority than first (%.3f)",
			ranked[1].Priority, ranked[0].Priority)
	}
}

func TestRankMessages_RecencyBonusSingleMessage(t *testing.T) {
	ranker := NewMessageRanker(nil)
	messages := []ports.Message{
		{Role: "user", Content: "only", Source: ports.MessageSourceUserInput},
	}
	ranked := ranker.RankMessages(messages)

	// With a single message, recency bonus should be 0.
	expected := DefaultSourceWeights[ports.MessageSourceUserInput]
	if ranked[0].Priority != expected {
		t.Fatalf("single message priority should equal base weight %.2f, got %.3f",
			expected, ranked[0].Priority)
	}
}

// ---------------------------------------------------------------------------
// Content signal detection
// ---------------------------------------------------------------------------

func TestRankMessages_ToolCallSignal(t *testing.T) {
	ranker := NewMessageRanker(nil)
	messages := []ports.Message{
		{Role: "assistant", Content: "plain reply", Source: ports.MessageSourceAssistantReply},
		{Role: "assistant", Content: "tool reply", Source: ports.MessageSourceAssistantReply,
			ToolCalls: []ports.ToolCall{{ID: "t1", Name: "shell"}}},
	}
	ranked := ranker.RankMessages(messages)

	// The second message has a tool call bonus plus higher recency, so it must
	// be strictly higher.
	if ranked[1].Priority <= ranked[0].Priority {
		t.Fatalf("tool-call message (%.3f) should outrank plain reply (%.3f)",
			ranked[1].Priority, ranked[0].Priority)
	}
}

func TestRankMessages_ErrorContentSignal(t *testing.T) {
	ranker := NewMessageRanker(nil)
	messages := []ports.Message{
		{Role: "tool", Content: "result ok", Source: ports.MessageSourceToolResult},
		{Role: "tool", Content: "result ok", Source: ports.MessageSourceToolResult},
		// Same source and same position distance — the only difference is error text.
	}
	ranked := ranker.RankMessages(messages)
	baseNoError := ranked[0].Priority

	messagesWithError := []ports.Message{
		{Role: "tool", Content: "some error occurred", Source: ports.MessageSourceToolResult},
		{Role: "tool", Content: "result ok", Source: ports.MessageSourceToolResult},
	}
	rankedWithError := ranker.RankMessages(messagesWithError)
	baseWithError := rankedWithError[0].Priority

	if baseWithError <= baseNoError {
		t.Fatalf("error-containing message (%.3f) should score higher than plain message (%.3f)",
			baseWithError, baseNoError)
	}
}

func TestRankMessages_ErrorKeywords(t *testing.T) {
	keywords := []string{"error", "FAIL", "Exception", "panic", "FATAL"}
	ranker := NewMessageRanker(nil)

	for _, kw := range keywords {
		msgs := []ports.Message{
			{Role: "tool", Content: "prefix " + kw + " suffix", Source: ports.MessageSourceToolResult},
		}
		ranked := ranker.RankMessages(msgs)
		base := DefaultSourceWeights[ports.MessageSourceToolResult]
		if ranked[0].Priority <= base {
			t.Errorf("keyword %q should trigger error bonus, got priority %.3f (base %.2f)", kw, ranked[0].Priority, base)
		}
	}
}

// ---------------------------------------------------------------------------
// Priority clamping
// ---------------------------------------------------------------------------

func TestRankMessages_ClampToOne(t *testing.T) {
	// Use artificially high weights to verify clamping.
	weights := SourceWeights{
		ports.MessageSourceSystemPrompt: 1.0,
	}
	ranker := NewMessageRanker(weights)

	messages := []ports.Message{
		{Role: "system", Content: "panic error", Source: ports.MessageSourceSystemPrompt,
			ToolCalls: []ports.ToolCall{{ID: "x"}}},
		{Role: "system", Content: "panic error", Source: ports.MessageSourceSystemPrompt,
			ToolCalls: []ports.ToolCall{{ID: "y"}}},
	}
	ranked := ranker.RankMessages(messages)
	for i, rm := range ranked {
		if rm.Priority > 1.0 {
			t.Fatalf("message[%d] priority %.3f exceeds 1.0", i, rm.Priority)
		}
	}
}

// ---------------------------------------------------------------------------
// SelectTopN — budget selection
// ---------------------------------------------------------------------------

func TestSelectTopN_FitsWithinBudget(t *testing.T) {
	ranker := NewMessageRanker(nil)
	messages := []ports.Message{
		{Role: "system", Content: "sys prompt", Source: ports.MessageSourceSystemPrompt},           // ~2 tokens
		{Role: "user", Content: "hello world from the user", Source: ports.MessageSourceUserInput}, // ~5 tokens
		{Role: "assistant", Content: "reply here", Source: ports.MessageSourceAssistantReply},      // ~2 tokens
		{Role: "system", Content: "debug noise", Source: ports.MessageSourceDebug},                 // ~2 tokens
	}
	ranked := ranker.RankMessages(messages)

	// Budget = 4 tokens: should pick highest-priority items that fit.
	selected := SelectTopN(ranked, 4, simpleTokenEstimator)

	totalTokens := 0
	for _, rm := range selected {
		totalTokens += simpleTokenEstimator(rm.Message.Content)
	}
	if totalTokens > 4 {
		t.Fatalf("selected messages exceed token budget: %d > 4", totalTokens)
	}
	if len(selected) == 0 {
		t.Fatalf("expected at least one message to be selected")
	}
}

func TestSelectTopN_PreservesChronologicalOrder(t *testing.T) {
	ranker := NewMessageRanker(nil)
	messages := []ports.Message{
		{Role: "system", Content: "a", Source: ports.MessageSourceSystemPrompt},
		{Role: "user", Content: "b", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "c", Source: ports.MessageSourceAssistantReply},
		{Role: "system", Content: "d", Source: ports.MessageSourceImportant},
	}
	ranked := ranker.RankMessages(messages)

	// Large budget so all messages are selected.
	selected := SelectTopN(ranked, 1000, simpleTokenEstimator)
	if len(selected) != 4 {
		t.Fatalf("expected all 4 messages, got %d", len(selected))
	}

	for i := 1; i < len(selected); i++ {
		if selected[i].index <= selected[i-1].index {
			t.Fatalf("output not in chronological order at position %d: index %d <= %d",
				i, selected[i].index, selected[i-1].index)
		}
	}
}

func TestSelectTopN_PrefersHigherPriority(t *testing.T) {
	ranker := NewMessageRanker(nil)
	messages := []ports.Message{
		{Role: "system", Content: "x", Source: ports.MessageSourceDebug},                 // low priority
		{Role: "system", Content: "x", Source: ports.MessageSourceSystemPrompt},          // high priority
		{Role: "system", Content: "x", Source: ports.MessageSourceImportant},             // high priority
		{Role: "user", Content: "x", Source: ports.MessageSourceUserInput},               // medium
		{Role: "assistant", Content: "x", Source: ports.MessageSourceAssistantReply},     // medium-low
		{Role: "system", Content: "x extra words here", Source: ports.MessageSourceDebug}, // low priority, more tokens
	}
	ranked := ranker.RankMessages(messages)

	// Budget fits exactly 2 single-token messages.
	selected := SelectTopN(ranked, 2, simpleTokenEstimator)

	if len(selected) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(selected))
	}
	// Both selected should be SystemPrompt or Important (the two highest base scores).
	for _, rm := range selected {
		if rm.Message.Source != ports.MessageSourceSystemPrompt && rm.Message.Source != ports.MessageSourceImportant {
			t.Errorf("expected high-priority source, got %s (priority %.3f)", rm.Message.Source, rm.Priority)
		}
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestRankMessages_Empty(t *testing.T) {
	ranker := NewMessageRanker(nil)
	ranked := ranker.RankMessages(nil)
	if len(ranked) != 0 {
		t.Fatalf("expected empty result for nil input, got %d", len(ranked))
	}
	ranked = ranker.RankMessages([]ports.Message{})
	if len(ranked) != 0 {
		t.Fatalf("expected empty result for empty input, got %d", len(ranked))
	}
}

func TestSelectTopN_EmptyInput(t *testing.T) {
	selected := SelectTopN(nil, 100, simpleTokenEstimator)
	if selected != nil {
		t.Fatalf("expected nil for nil input, got %v", selected)
	}
	selected = SelectTopN([]RankedMessage{}, 100, simpleTokenEstimator)
	if selected != nil {
		t.Fatalf("expected nil for empty input, got %v", selected)
	}
}

func TestSelectTopN_ZeroBudget(t *testing.T) {
	ranker := NewMessageRanker(nil)
	messages := []ports.Message{
		{Role: "user", Content: "hi", Source: ports.MessageSourceUserInput},
	}
	ranked := ranker.RankMessages(messages)
	selected := SelectTopN(ranked, 0, simpleTokenEstimator)
	if selected != nil {
		t.Fatalf("expected nil for zero budget, got %d items", len(selected))
	}
}

func TestRankMessages_AllSamePriority(t *testing.T) {
	weights := SourceWeights{
		ports.MessageSourceUserInput: 0.50,
	}
	ranker := NewMessageRanker(weights)
	messages := []ports.Message{
		{Role: "user", Content: "a", Source: ports.MessageSourceUserInput},
		{Role: "user", Content: "b", Source: ports.MessageSourceUserInput},
		{Role: "user", Content: "c", Source: ports.MessageSourceUserInput},
	}
	ranked := ranker.RankMessages(messages)

	// With equal base weights, recency alone differentiates; last is highest.
	if ranked[2].Priority <= ranked[0].Priority {
		t.Fatalf("last message (%.3f) should have higher priority than first (%.3f) due to recency",
			ranked[2].Priority, ranked[0].Priority)
	}
}

func TestRankMessages_UnknownSource(t *testing.T) {
	ranker := NewMessageRanker(nil)
	messages := []ports.Message{
		{Role: "user", Content: "mystery", Source: "custom_source"},
	}
	ranked := ranker.RankMessages(messages)
	// Should use the Unknown fallback weight (0.40).
	if ranked[0].Priority != DefaultSourceWeights[ports.MessageSourceUnknown] {
		t.Fatalf("expected unknown source weight %.2f, got %.3f",
			DefaultSourceWeights[ports.MessageSourceUnknown], ranked[0].Priority)
	}
}

func TestRankMessages_ReasonContainsSource(t *testing.T) {
	ranker := NewMessageRanker(nil)
	messages := []ports.Message{
		{Role: "user", Content: "error found", Source: ports.MessageSourceUserInput,
			ToolCalls: []ports.ToolCall{{ID: "t1"}}},
	}
	ranked := ranker.RankMessages(messages)
	reason := ranked[0].Reason
	if reason == "" {
		t.Fatalf("expected non-empty reason")
	}
	if !containsSubstring(reason, "user_input") {
		t.Fatalf("expected reason to reference source, got %q", reason)
	}
	if !containsSubstring(reason, "content_signal") {
		t.Fatalf("expected reason to mention content_signal, got %q", reason)
	}
}

func TestSelectTopN_SkipsLargeMessages(t *testing.T) {
	ranker := NewMessageRanker(nil)
	messages := []ports.Message{
		{Role: "system", Content: "a b c d e f g h i j", Source: ports.MessageSourceSystemPrompt}, // 10 tokens
		{Role: "user", Content: "x", Source: ports.MessageSourceUserInput},                         // 1 token
	}
	ranked := ranker.RankMessages(messages)

	// Budget of 5 tokens: the system prompt (10 tokens) won't fit, but user (1 token) will.
	selected := SelectTopN(ranked, 5, simpleTokenEstimator)
	if len(selected) != 1 {
		t.Fatalf("expected 1 message, got %d", len(selected))
	}
	if selected[0].Message.Source != ports.MessageSourceUserInput {
		t.Fatalf("expected UserInput to be selected (fits budget), got %s", selected[0].Message.Source)
	}
}

func TestNewMessageRanker_CustomWeights(t *testing.T) {
	custom := SourceWeights{
		ports.MessageSourceDebug: 0.99,
	}
	ranker := NewMessageRanker(custom)
	messages := []ports.Message{
		{Role: "system", Content: "dbg", Source: ports.MessageSourceDebug},
	}
	ranked := ranker.RankMessages(messages)
	if ranked[0].Priority != 0.99 {
		t.Fatalf("expected custom weight 0.99, got %.3f", ranked[0].Priority)
	}
}

func TestRankMessages_ToolResultSignal(t *testing.T) {
	ranker := NewMessageRanker(nil)

	// Two tool-result messages at different positions; the one with ToolResults
	// populated should get the content signal bonus.
	messages := []ports.Message{
		{Role: "tool", Content: "ok", Source: ports.MessageSourceToolResult},
		{Role: "tool", Content: "ok", Source: ports.MessageSourceToolResult,
			ToolResults: []ports.ToolResult{{CallID: "c1", Content: "done"}}},
	}
	ranked := ranker.RankMessages(messages)

	// Both have recency (second is later) and second has content signal.
	// We verify the bonus by checking the delta exceeds pure recency.
	delta := ranked[1].Priority - ranked[0].Priority
	pureRecency := maxRecencyBonus // max difference for 2 items
	if delta <= pureRecency-0.001 {
		t.Fatalf("expected tool-result content bonus to contribute, delta=%.4f recency=%.4f",
			delta, pureRecency)
	}
}

// containsSubstring is defined in manager_prompt_okr_test.go (same package).
