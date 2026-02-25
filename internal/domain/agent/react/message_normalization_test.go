package react

import (
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestNormalizeContextMessagesDemotesLegacySystemSummaries(t *testing.T) {
	state := &TaskState{
		SystemPrompt: "base system",
		Messages: []Message{
			{Role: "system", Content: "base system", Source: ports.MessageSourceSystemPrompt},
			{Role: "system", Content: "[Earlier context compressed] one", Source: ports.MessageSourceSystemPrompt},
			{Role: "system", Content: "[Earlier context compressed] two", Source: ports.MessageSourceSystemPrompt},
			{Role: "assistant", Content: "latest assistant", Source: ports.MessageSourceAssistantReply},
		},
	}

	normalizeContextMessages(state)

	var summaryCount int
	for _, msg := range state.Messages {
		if isCompressionSummaryContent(msg.Content) {
			summaryCount++
			if msg.Source != ports.MessageSourceUserHistory {
				t.Fatalf("expected summary source=user_history, got %q", msg.Source)
			}
			if msg.Role != "assistant" {
				t.Fatalf("expected summary role=assistant, got %q", msg.Role)
			}
			if msg.Content != "[Earlier context compressed] two" {
				t.Fatalf("expected latest summary kept, got %q", msg.Content)
			}
		}
	}
	if summaryCount != 1 {
		t.Fatalf("expected exactly 1 compression summary, got %d", summaryCount)
	}
}

func TestNormalizeContextMessagesConvertsUserHistorySystemRole(t *testing.T) {
	state := &TaskState{
		Messages: []Message{
			{Role: "system", Content: "history note", Source: ports.MessageSourceUserHistory},
			{Role: "assistant", Content: "ok", Source: ports.MessageSourceAssistantReply},
		},
	}

	normalizeContextMessages(state)

	if got := state.Messages[0].Role; got != "assistant" {
		t.Fatalf("expected user_history role rewritten to assistant, got %q", got)
	}
}
