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
	// No user messages → keep everything.
	if len(result) != 2 {
		t.Errorf("expected all 2 messages when no user messages, got %d", len(result))
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
