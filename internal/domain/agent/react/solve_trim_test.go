package react

import (
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	tokenutil "alex/internal/shared/token"
)

func TestAggressiveTrimMessagesKeepsOnlyCanonicalSystemPrompt(t *testing.T) {
	messages := []ports.Message{
		{Role: "system", Content: "canonical prompt", Source: ports.MessageSourceSystemPrompt},
		{Role: "system", Content: "legacy prompt", Source: ports.MessageSourceSystemPrompt},
		{Role: "user", Content: "first user", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "first reply", Source: ports.MessageSourceAssistantReply},
		{Role: "system", Content: "runtime correction", Source: ports.MessageSourceSystemPrompt},
		{Role: "user", Content: "latest user", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "latest reply", Source: ports.MessageSourceAssistantReply},
	}

	trimmed := aggressiveTrimMessages(messages, 1)

	systemPromptCount := 0
	containsLatestUser := false
	containsLatestReply := false
	for _, msg := range trimmed {
		if msg.Source == ports.MessageSourceSystemPrompt {
			systemPromptCount++
			if msg.Content != "canonical prompt" {
				t.Fatalf("expected only canonical system prompt to survive, got %q", msg.Content)
			}
		}
		if msg.Content == "latest user" {
			containsLatestUser = true
		}
		if msg.Content == "latest reply" {
			containsLatestReply = true
		}
		if msg.Content == "legacy prompt" || msg.Content == "runtime correction" {
			t.Fatalf("stale system prompt content should be removed, got %q", msg.Content)
		}
	}

	if systemPromptCount != 1 {
		t.Fatalf("expected exactly one system prompt after trim, got %d", systemPromptCount)
	}
	if !containsLatestUser || !containsLatestReply {
		t.Fatalf("expected latest turn to remain, got %+v", trimmed)
	}
}

func TestForceFitMessagesToLimitAlwaysFitsBudget(t *testing.T) {
	messages := []ports.Message{
		{
			Role:    "system",
			Source:  ports.MessageSourceSystemPrompt,
			Content: strings.Repeat("primary system prompt ", 6000),
		},
		{
			Role:    "assistant",
			Source:  ports.MessageSourceUserHistory,
			Content: strings.Repeat("historic context ", 4000),
		},
		{
			Role:    "user",
			Source:  ports.MessageSourceUserInput,
			Content: strings.Repeat("latest user request ", 1200),
		},
	}
	estimate := func(values []ports.Message) int {
		total := 0
		for _, msg := range values {
			total += tokenutil.CountTokens(msg.Content) + 4
		}
		return total
	}
	limit := 1500

	fitted := forceFitMessagesToLimit(messages, limit, estimate)
	if estimate(fitted) > limit {
		t.Fatalf("expected fitted messages to respect limit=%d, got=%d", limit, estimate(fitted))
	}
	if len(fitted) == 0 {
		t.Fatal("expected fitted messages to remain non-empty")
	}
}
