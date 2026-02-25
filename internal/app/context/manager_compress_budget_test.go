package context

import (
	"testing"

	"alex/internal/domain/agent/ports"
)

// ---------------------------------------------------------------------------
// BudgetCheck tests
// ---------------------------------------------------------------------------

func TestBudgetCheckOK(t *testing.T) {
	signal := BudgetCheck(500, 1000, 0.70, 0.85)
	if signal != BudgetOK {
		t.Errorf("expected OK, got %d", signal)
	}
}

func TestBudgetCheckCompress(t *testing.T) {
	// 750/1000 = 0.75 → above 0.70 but below 0.85
	signal := BudgetCheck(750, 1000, 0.70, 0.85)
	if signal != BudgetCompress {
		t.Errorf("expected Compress, got %d", signal)
	}
}

func TestBudgetCheckAggressiveTrim(t *testing.T) {
	// 900/1000 = 0.90 → above 0.85
	signal := BudgetCheck(900, 1000, 0.70, 0.85)
	if signal != BudgetAggressiveTrim {
		t.Errorf("expected AggressiveTrim, got %d", signal)
	}
}

func TestBudgetCheckExactBoundary(t *testing.T) {
	// Exactly at compression threshold
	signal := BudgetCheck(700, 1000, 0.70, 0.85)
	if signal != BudgetCompress {
		t.Errorf("expected Compress at exact boundary, got %d", signal)
	}

	// Exactly at aggressive threshold
	signal = BudgetCheck(850, 1000, 0.70, 0.85)
	if signal != BudgetAggressiveTrim {
		t.Errorf("expected AggressiveTrim at exact boundary, got %d", signal)
	}
}

func TestBudgetCheckZeroLimit(t *testing.T) {
	signal := BudgetCheck(500, 0, 0.70, 0.85)
	if signal != BudgetOK {
		t.Errorf("expected OK for zero limit, got %d", signal)
	}
}

// ---------------------------------------------------------------------------
// AggressiveTrim tests
// ---------------------------------------------------------------------------

func TestAggressiveTrimPreservesSystemMessages(t *testing.T) {
	messages := []ports.Message{
		msg(ports.MessageSourceSystemPrompt, "system", "You are helpful"),
		msg(ports.MessageSourceImportant, "system", "Important note"),
		msg(ports.MessageSourceUserInput, "user", "Old question 1"),
		msg(ports.MessageSourceAssistantReply, "assistant", "Old answer 1"),
		msg(ports.MessageSourceUserInput, "user", "Old question 2"),
		msg(ports.MessageSourceAssistantReply, "assistant", "Old answer 2"),
		msg(ports.MessageSourceUserInput, "user", "Recent question"),
		msg(ports.MessageSourceAssistantReply, "assistant", "Recent answer"),
	}

	result := AggressiveTrim(messages, 1)

	// System and important messages should always be preserved.
	preservedCount := 0
	for _, m := range result {
		switch m.Source {
		case ports.MessageSourceSystemPrompt, ports.MessageSourceImportant:
			preservedCount++
		}
	}
	if preservedCount < 2 {
		t.Errorf("expected at least 2 preserved messages, got %d", preservedCount)
	}
}

func TestAggressiveTrimKeepsRecentTurns(t *testing.T) {
	messages := []ports.Message{
		msg(ports.MessageSourceSystemPrompt, "system", "System prompt"),
		msg(ports.MessageSourceUserInput, "user", "Turn 1"),
		msg(ports.MessageSourceAssistantReply, "assistant", "Reply 1"),
		msg(ports.MessageSourceUserInput, "user", "Turn 2"),
		msg(ports.MessageSourceAssistantReply, "assistant", "Reply 2"),
		msg(ports.MessageSourceUserInput, "user", "Turn 3"),
		msg(ports.MessageSourceAssistantReply, "assistant", "Reply 3"),
	}

	result := AggressiveTrim(messages, 2)

	// Should keep system prompt + last 2 turns + compression summary.
	var userMsgs []string
	for _, m := range result {
		if m.Role == "user" && m.Source == ports.MessageSourceUserInput {
			userMsgs = append(userMsgs, m.Content)
		}
	}
	if len(userMsgs) != 2 {
		t.Errorf("expected 2 user messages in result, got %d: %v", len(userMsgs), userMsgs)
	}
	if userMsgs[0] != "Turn 2" || userMsgs[1] != "Turn 3" {
		t.Errorf("expected Turn 2 and Turn 3, got %v", userMsgs)
	}
}

func TestAggressiveTrimDefaultMaxTurns(t *testing.T) {
	// When maxTurns=0, should default to 6.
	messages := []ports.Message{
		msg(ports.MessageSourceSystemPrompt, "system", "System"),
	}
	for i := 0; i < 10; i++ {
		messages = append(messages,
			msg(ports.MessageSourceUserInput, "user", "Q"),
			msg(ports.MessageSourceAssistantReply, "assistant", "A"),
		)
	}

	result := AggressiveTrim(messages, 0)
	userCount := 0
	for _, m := range result {
		if m.Role == "user" && m.Source == ports.MessageSourceUserInput {
			userCount++
		}
	}
	if userCount != 6 {
		t.Errorf("expected 6 user messages with default maxTurns, got %d", userCount)
	}
}

// ---------------------------------------------------------------------------
// keepRecentTurns tests
// ---------------------------------------------------------------------------

func TestKeepRecentTurns_Empty(t *testing.T) {
	result := keepRecentTurns(nil, 3)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestKeepRecentTurns_AllKept(t *testing.T) {
	messages := []ports.Message{
		{Role: "user", Content: "Q1"},
		{Role: "assistant", Content: "A1"},
	}
	result := keepRecentTurns(messages, 5)
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

func TestKeepRecentTurns_NoUserMessages(t *testing.T) {
	messages := []ports.Message{
		{Role: "assistant", Content: "A1"},
		{Role: "tool", Content: "R1"},
	}
	result := keepRecentTurns(messages, 1)
	// No user messages → keep only the most recent entry.
	if len(result) != 1 {
		t.Errorf("expected 1 message when no user messages, got %d", len(result))
	}
	if len(result) == 1 && result[0].Content != "R1" {
		t.Errorf("expected to keep latest message R1, got %q", result[0].Content)
	}
}

func TestKeepRecentTurns_TrimOldTurns(t *testing.T) {
	messages := []ports.Message{
		{Role: "user", Content: "Turn 1"},
		{Role: "assistant", Content: "Reply 1"},
		{Role: "user", Content: "Turn 2"},
		{Role: "assistant", Content: "Reply 2"},
		{Role: "tool", Content: "Tool result 2"},
		{Role: "user", Content: "Turn 3"},
		{Role: "assistant", Content: "Reply 3"},
	}
	result := keepRecentTurns(messages, 2)
	if len(result) != 5 {
		t.Errorf("expected 5 messages (last 2 turns), got %d", len(result))
	}
	if result[0].Content != "Turn 2" {
		t.Errorf("first kept message = %q, want 'Turn 2'", result[0].Content)
	}
}

// ---------------------------------------------------------------------------
// EstimateMessageTokens tests
// ---------------------------------------------------------------------------

func TestEstimateMessageTokensContentOnly(t *testing.T) {
	msg := ports.Message{Role: "user", Content: "Hello world"}
	tokens := EstimateMessageTokens(msg)
	// Should count content + 4 overhead.
	if tokens <= 4 {
		t.Errorf("expected tokens > 4 for content message, got %d", tokens)
	}
}

func TestEstimateMessageTokensWithToolCalls(t *testing.T) {
	msgPlain := ports.Message{Role: "assistant", Content: "I will search."}
	msgWithTools := ports.Message{
		Role:    "assistant",
		Content: "I will search.",
		ToolCalls: []ports.ToolCall{
			{
				ID:   "call_123",
				Name: "web_search",
				Arguments: map[string]any{
					"query": "how to fix context length exceeded errors in LLM applications",
				},
			},
		},
	}

	plainTokens := EstimateMessageTokens(msgPlain)
	withToolTokens := EstimateMessageTokens(msgWithTools)

	if withToolTokens <= plainTokens {
		t.Errorf("message with tool calls (%d) should have more tokens than plain (%d)",
			withToolTokens, plainTokens)
	}
}

func TestEstimateMessageTokensWithThinking(t *testing.T) {
	msgNoThink := ports.Message{Role: "assistant", Content: "Answer."}
	msgWithThink := ports.Message{
		Role:    "assistant",
		Content: "Answer.",
		Thinking: ports.Thinking{
			Parts: []ports.ThinkingPart{
				{Text: "Let me think about this problem step by step and consider all the options."},
			},
		},
	}

	plainTokens := EstimateMessageTokens(msgNoThink)
	thinkTokens := EstimateMessageTokens(msgWithThink)

	if thinkTokens <= plainTokens {
		t.Errorf("message with thinking (%d) should have more tokens than plain (%d)",
			thinkTokens, plainTokens)
	}
}

func TestEstimateMessageTokensWithAttachments(t *testing.T) {
	msgNoAtt := ports.Message{Role: "user", Content: "Analyze this."}
	msgWithAtt := ports.Message{
		Role:    "user",
		Content: "Analyze this.",
		Attachments: map[string]ports.Attachment{
			"image.png": {
				Name:      "image.png",
				MediaType: "image/png",
				Data:      "iVBORw0KGgoAAAANSUhEUg==", // fake base64
			},
		},
	}

	plainTokens := EstimateMessageTokens(msgNoAtt)
	attTokens := EstimateMessageTokens(msgWithAtt)

	if attTokens <= plainTokens {
		t.Errorf("message with attachment (%d) should have more tokens than plain (%d)",
			attTokens, plainTokens)
	}
	// Attachment with data should add at least 500 tokens.
	if attTokens < plainTokens+500 {
		t.Errorf("attachment estimate should add ~500 tokens, got delta=%d", attTokens-plainTokens)
	}
}

func TestEstimateMessageTokensWithToolCallID(t *testing.T) {
	msgNoID := ports.Message{Role: "tool", Content: "Result content."}
	msgWithID := ports.Message{
		Role:       "tool",
		Content:    "Result content.",
		ToolCallID: "call_abc123def456",
	}

	plainTokens := EstimateMessageTokens(msgNoID)
	idTokens := EstimateMessageTokens(msgWithID)

	if idTokens <= plainTokens {
		t.Errorf("message with tool_call_id (%d) should have more tokens than plain (%d)",
			idTokens, plainTokens)
	}
}

func TestEstimateTokensCountsAllComponents(t *testing.T) {
	// Regression test: previous implementation only counted msg.Content,
	// causing severe underestimates and context_length_exceeded errors.
	mgr := &manager{}
	messages := []ports.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Search for information about Go."},
		{
			Role:    "assistant",
			Content: "I'll search for that.",
			ToolCalls: []ports.ToolCall{
				{
					ID:   "call_1",
					Name: "web_search",
					Arguments: map[string]any{
						"query": "Go programming language information",
					},
				},
			},
			Thinking: ports.Thinking{
				Parts: []ports.ThinkingPart{
					{Text: "The user wants me to search for Go programming language."},
				},
			},
		},
		{
			Role:       "tool",
			Content:    "Go is a statically typed, compiled programming language designed at Google.",
			ToolCallID: "call_1",
		},
	}

	fullEstimate := mgr.EstimateTokens(messages)

	// Old (content-only) estimate for comparison.
	contentOnly := 0
	for _, msg := range messages {
		if msg.Content != "" {
			contentOnly += len([]rune(msg.Content)) / 4 // rough estimate
		}
	}

	if fullEstimate <= contentOnly {
		t.Errorf("full estimate (%d) should exceed content-only estimate (%d)",
			fullEstimate, contentOnly)
	}
}
