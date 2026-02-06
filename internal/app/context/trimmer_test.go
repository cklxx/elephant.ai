package context

import (
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/token"
)

// helper: build a message with the given source and content.
func msg(source ports.MessageSource, role, content string) ports.Message {
	return ports.Message{Role: role, Content: content, Source: source}
}

// helper: sum token counts for a slice of messages.
func sumTokens(msgs []ports.Message) int {
	n := 0
	for _, m := range msgs {
		n += tokenutil.CountTokens(m.Content)
	}
	return n
}

// ---------------------------------------------------------------------------
// 1. Basic trimming: messages trimmed to fit token budget
// ---------------------------------------------------------------------------

func TestTrimMessages_BasicTrimming(t *testing.T) {
	messages := []ports.Message{
		msg(ports.MessageSourceUserInput, "user", "Hello, can you help me?"),
		msg(ports.MessageSourceAssistantReply, "assistant", "Sure, what do you need?"),
		msg(ports.MessageSourceDebug, "system", strings.Repeat("debug info ", 50)),
	}

	totalBefore := sumTokens(messages)
	// Set budget to less than total so trimming must occur.
	budget := totalBefore - tokenutil.CountTokens(messages[2].Content) + 1
	result := TrimMessages(messages, TrimConfig{MaxTokens: budget})

	if result.TotalTokens > budget {
		t.Fatalf("expected total tokens <= %d, got %d", budget, result.TotalTokens)
	}
	if len(result.Trimmed) == 0 {
		t.Fatalf("expected at least one message to be trimmed")
	}
	// Debug should be trimmed first (lowest priority).
	for _, trimmed := range result.Trimmed {
		if trimmed.Source != ports.MessageSourceDebug {
			t.Fatalf("expected debug message to be trimmed first, got source %q", trimmed.Source)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Preserved sources: SystemPrompt and Important always kept
// ---------------------------------------------------------------------------

func TestTrimMessages_PreservedSources(t *testing.T) {
	messages := []ports.Message{
		msg(ports.MessageSourceSystemPrompt, "system", strings.Repeat("system prompt ", 40)),
		msg(ports.MessageSourceImportant, "system", strings.Repeat("important note ", 40)),
		msg(ports.MessageSourceDebug, "system", strings.Repeat("debug data ", 40)),
		msg(ports.MessageSourceUserHistory, "user", strings.Repeat("old history ", 40)),
	}

	// Set a very small budget — only preserved messages should survive.
	preservedTokens := tokenutil.CountTokens(messages[0].Content) + tokenutil.CountTokens(messages[1].Content)
	result := TrimMessages(messages, TrimConfig{
		MaxTokens:        preservedTokens + 1,
		PreservedSources: []ports.MessageSource{ports.MessageSourceSystemPrompt, ports.MessageSourceImportant},
	})

	if len(result.Kept) < 2 {
		t.Fatalf("expected at least 2 preserved messages, got %d", len(result.Kept))
	}
	for _, k := range result.Kept {
		if k.Source != ports.MessageSourceSystemPrompt && k.Source != ports.MessageSourceImportant {
			t.Fatalf("expected only preserved sources in kept, got %q", k.Source)
		}
	}
	if len(result.Trimmed) != 2 {
		t.Fatalf("expected 2 trimmed messages, got %d", len(result.Trimmed))
	}
}

// ---------------------------------------------------------------------------
// 3. Priority ordering: Debug/Evaluation trimmed before UserInput
// ---------------------------------------------------------------------------

func TestTrimMessages_PriorityOrdering(t *testing.T) {
	padding := strings.Repeat("word ", 20)
	messages := []ports.Message{
		msg(ports.MessageSourceUserInput, "user", "important request "+padding),
		msg(ports.MessageSourceEvaluation, "system", "eval data "+padding),
		msg(ports.MessageSourceDebug, "system", "debug trace "+padding),
		msg(ports.MessageSourceAssistantReply, "assistant", "reply "+padding),
		msg(ports.MessageSourceToolResult, "tool", "result "+padding),
	}

	total := sumTokens(messages)
	// Budget allows keeping roughly 3 out of 5 messages.
	budget := total - sumTokens(messages[1:3])
	result := TrimMessages(messages, TrimConfig{MaxTokens: budget})

	// Debug and Evaluation have the lowest priority (1), so they should be trimmed first.
	trimmedSources := make(map[ports.MessageSource]bool)
	for _, m := range result.Trimmed {
		trimmedSources[m.Source] = true
	}

	if !trimmedSources[ports.MessageSourceDebug] {
		t.Fatalf("expected debug to be trimmed, trimmed sources: %v", trimmedSources)
	}
	if !trimmedSources[ports.MessageSourceEvaluation] {
		t.Fatalf("expected evaluation to be trimmed, trimmed sources: %v", trimmedSources)
	}

	// UserInput should still be kept.
	keptSources := make(map[ports.MessageSource]bool)
	for _, m := range result.Kept {
		keptSources[m.Source] = true
	}
	if !keptSources[ports.MessageSourceUserInput] {
		t.Fatalf("expected user input to be kept, kept sources: %v", keptSources)
	}
}

// ---------------------------------------------------------------------------
// 4. Cost-based trimming: stays within cost budget
// ---------------------------------------------------------------------------

func TestTrimMessages_CostBasedTrimming(t *testing.T) {
	padding := strings.Repeat("token ", 100)
	messages := []ports.Message{
		msg(ports.MessageSourceUserInput, "user", "question "+padding),
		msg(ports.MessageSourceAssistantReply, "assistant", "answer "+padding),
		msg(ports.MessageSourceDebug, "system", "debug "+padding),
		msg(ports.MessageSourceToolResult, "tool", "result "+padding),
	}

	model := &ModelCostProfile{
		Name:           "gpt-4",
		InputCostPer1K: 0.03,
		ContextWindow:  8192,
	}

	total := sumTokens(messages)
	// Set a token budget that fits everything, but a cost budget that does not.
	maxCost := EstimateInputCost(total/2, model)
	result := TrimMessages(messages, TrimConfig{
		MaxTokens:  total + 1000, // generous token budget
		MaxCostUSD: maxCost,
		Model:      model,
	})

	if result.EstimatedCostUSD > maxCost {
		t.Fatalf("expected cost <= %.6f, got %.6f", maxCost, result.EstimatedCostUSD)
	}
	if len(result.Trimmed) == 0 {
		t.Fatalf("expected some messages to be trimmed for cost, but none were")
	}
}

// ---------------------------------------------------------------------------
// 5. No trimming needed: all messages fit
// ---------------------------------------------------------------------------

func TestTrimMessages_NoTrimmingNeeded(t *testing.T) {
	messages := []ports.Message{
		msg(ports.MessageSourceSystemPrompt, "system", "You are a helpful assistant."),
		msg(ports.MessageSourceUserInput, "user", "Hello"),
		msg(ports.MessageSourceAssistantReply, "assistant", "Hi there!"),
	}

	total := sumTokens(messages)
	result := TrimMessages(messages, TrimConfig{MaxTokens: total + 100})

	if len(result.Kept) != len(messages) {
		t.Fatalf("expected all %d messages kept, got %d", len(messages), len(result.Kept))
	}
	if len(result.Trimmed) != 0 {
		t.Fatalf("expected no trimmed messages, got %d", len(result.Trimmed))
	}
	if result.TotalTokens != total {
		t.Fatalf("expected total tokens %d, got %d", total, result.TotalTokens)
	}
}

// ---------------------------------------------------------------------------
// 6. Empty messages
// ---------------------------------------------------------------------------

func TestTrimMessages_Empty(t *testing.T) {
	result := TrimMessages(nil, TrimConfig{MaxTokens: 100})
	if len(result.Kept) != 0 {
		t.Fatalf("expected 0 kept, got %d", len(result.Kept))
	}
	if len(result.Trimmed) != 0 {
		t.Fatalf("expected 0 trimmed, got %d", len(result.Trimmed))
	}
	if result.TotalTokens != 0 {
		t.Fatalf("expected 0 tokens, got %d", result.TotalTokens)
	}

	result = TrimMessages([]ports.Message{}, TrimConfig{MaxTokens: 100})
	if len(result.Kept) != 0 || len(result.Trimmed) != 0 {
		t.Fatalf("expected empty result for empty slice")
	}
}

// ---------------------------------------------------------------------------
// 7. Single preserved message exceeding budget is still kept
// ---------------------------------------------------------------------------

func TestTrimMessages_SinglePreservedExceedsBudget(t *testing.T) {
	huge := msg(ports.MessageSourceSystemPrompt, "system", strings.Repeat("This is a very long system prompt. ", 200))
	messages := []ports.Message{huge}

	result := TrimMessages(messages, TrimConfig{
		MaxTokens:        10, // tiny budget
		PreservedSources: []ports.MessageSource{ports.MessageSourceSystemPrompt},
	})

	if len(result.Kept) != 1 {
		t.Fatalf("expected 1 kept (preserved even over budget), got %d", len(result.Kept))
	}
	if result.Kept[0].Source != ports.MessageSourceSystemPrompt {
		t.Fatalf("expected system prompt to be kept, got %q", result.Kept[0].Source)
	}
	if len(result.Trimmed) != 0 {
		t.Fatalf("expected 0 trimmed, got %d", len(result.Trimmed))
	}
}

// ---------------------------------------------------------------------------
// 8. Original order preservation
// ---------------------------------------------------------------------------

func TestTrimMessages_OrderPreservation(t *testing.T) {
	messages := []ports.Message{
		msg(ports.MessageSourceSystemPrompt, "system", "System prompt"),
		msg(ports.MessageSourceUserInput, "user", "First user message"),
		msg(ports.MessageSourceDebug, "system", strings.Repeat("debug data ", 50)),
		msg(ports.MessageSourceAssistantReply, "assistant", "First reply"),
		msg(ports.MessageSourceUserInput, "user", "Second user message"),
		msg(ports.MessageSourceEvaluation, "system", strings.Repeat("eval data ", 50)),
		msg(ports.MessageSourceAssistantReply, "assistant", "Second reply"),
	}

	total := sumTokens(messages)
	// Trim enough to remove debug and evaluation messages.
	debugTokens := tokenutil.CountTokens(messages[2].Content)
	evalTokens := tokenutil.CountTokens(messages[5].Content)
	budget := total - debugTokens - evalTokens + 1
	result := TrimMessages(messages, TrimConfig{
		MaxTokens:        budget,
		PreservedSources: []ports.MessageSource{ports.MessageSourceSystemPrompt},
	})

	// Verify kept messages are in original order.
	expectedOrder := []ports.MessageSource{
		ports.MessageSourceSystemPrompt,
		ports.MessageSourceUserInput,
		ports.MessageSourceAssistantReply,
		ports.MessageSourceUserInput,
		ports.MessageSourceAssistantReply,
	}
	if len(result.Kept) != len(expectedOrder) {
		t.Fatalf("expected %d kept messages, got %d", len(expectedOrder), len(result.Kept))
	}
	for i, expected := range expectedOrder {
		if result.Kept[i].Source != expected {
			t.Fatalf("kept[%d]: expected source %q, got %q", i, expected, result.Kept[i].Source)
		}
	}
}

// ---------------------------------------------------------------------------
// 9. EstimateInputCost
// ---------------------------------------------------------------------------

func TestEstimateInputCost(t *testing.T) {
	model := &ModelCostProfile{
		Name:           "gpt-4",
		InputCostPer1K: 0.03,
	}
	cost := EstimateInputCost(1000, model)
	if cost != 0.03 {
		t.Fatalf("expected 0.03, got %f", cost)
	}

	cost = EstimateInputCost(500, model)
	expected := 0.015
	if cost != expected {
		t.Fatalf("expected %f, got %f", expected, cost)
	}

	cost = EstimateInputCost(1000, nil)
	if cost != 0 {
		t.Fatalf("expected 0 for nil model, got %f", cost)
	}
}

// ---------------------------------------------------------------------------
// 10. DefaultModelProfiles completeness
// ---------------------------------------------------------------------------

func TestDefaultModelProfiles(t *testing.T) {
	expectedModels := []string{"gpt-4", "gpt-3.5-turbo", "claude-3-opus", "claude-3-sonnet", "deepseek-chat"}
	for _, name := range expectedModels {
		profile, ok := DefaultModelProfiles[name]
		if !ok {
			t.Fatalf("expected model %q in DefaultModelProfiles", name)
		}
		if profile.Name != name {
			t.Fatalf("expected Name=%q, got %q", name, profile.Name)
		}
		if profile.InputCostPer1K <= 0 {
			t.Fatalf("expected positive InputCostPer1K for %q", name)
		}
		if profile.ContextWindow <= 0 {
			t.Fatalf("expected positive ContextWindow for %q", name)
		}
	}
}

// ---------------------------------------------------------------------------
// 11. sourcePriority ordering correctness
// ---------------------------------------------------------------------------

func TestSourcePriority(t *testing.T) {
	ordered := []ports.MessageSource{
		ports.MessageSourceDebug,          // 1
		ports.MessageSourceEvaluation,     // 1
		ports.MessageSourceUserHistory,    // 2
		ports.MessageSourceToolResult,     // 3
		ports.MessageSourceAssistantReply, // 4
		ports.MessageSourceProactive,      // 5
		ports.MessageSourceUserInput,      // 6
		ports.MessageSourceImportant,      // 7
		ports.MessageSourceSystemPrompt,   // 8
	}

	for i := 1; i < len(ordered); i++ {
		prev := sourcePriority(ordered[i-1])
		curr := sourcePriority(ordered[i])
		if curr < prev {
			t.Fatalf("expected priority(%q)=%d >= priority(%q)=%d",
				ordered[i], curr, ordered[i-1], prev)
		}
	}

	// Unknown source gets priority 0.
	if p := sourcePriority(ports.MessageSourceUnknown); p != 0 {
		t.Fatalf("expected priority 0 for unknown source, got %d", p)
	}
}

// ---------------------------------------------------------------------------
// 12. Mixed preserved and non-preserved with cost model
// ---------------------------------------------------------------------------

func TestTrimMessages_MixedPreservedWithCostModel(t *testing.T) {
	padding := strings.Repeat("word ", 50)
	messages := []ports.Message{
		msg(ports.MessageSourceSystemPrompt, "system", "Be helpful. "+padding),
		msg(ports.MessageSourceImportant, "system", "User is VIP. "+padding),
		msg(ports.MessageSourceUserInput, "user", "Question "+padding),
		msg(ports.MessageSourceAssistantReply, "assistant", "Answer "+padding),
		msg(ports.MessageSourceDebug, "system", "Trace "+padding),
		msg(ports.MessageSourceToolResult, "tool", "Output "+padding),
	}

	model := DefaultModelProfiles["gpt-4"]
	total := sumTokens(messages)
	// Cost budget that forces trimming of about half.
	maxCost := EstimateInputCost(total*2/3, &model)

	result := TrimMessages(messages, TrimConfig{
		MaxTokens:        total + 1000,
		MaxCostUSD:       maxCost,
		PreservedSources: []ports.MessageSource{ports.MessageSourceSystemPrompt, ports.MessageSourceImportant},
		Model:            &model,
	})

	// Preserved messages must always be present.
	keptSources := make(map[ports.MessageSource]int)
	for _, m := range result.Kept {
		keptSources[m.Source]++
	}
	if keptSources[ports.MessageSourceSystemPrompt] != 1 {
		t.Fatalf("expected system prompt to be kept")
	}
	if keptSources[ports.MessageSourceImportant] != 1 {
		t.Fatalf("expected important to be kept")
	}

	// Cost should be at or below budget.
	if result.EstimatedCostUSD > maxCost {
		t.Fatalf("expected cost <= %.6f, got %.6f", maxCost, result.EstimatedCostUSD)
	}
}
