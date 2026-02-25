package preparation

import (
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestBuildHistorySummaryPromptIncludesCurrentTask(t *testing.T) {
	messages := []ports.Message{
		{Role: "user", Content: "previously we discussed OAuth refresh retries"},
		{Role: "assistant", Content: "I proposed retry with backoff and metrics"},
	}

	prompt := buildHistorySummaryPrompt("fix the Lark auth refresh regression", messages)
	if !strings.Contains(prompt, "Current task:") {
		t.Fatalf("expected prompt to include current task header, got %q", prompt)
	}
	if !strings.Contains(prompt, "fix the Lark auth refresh regression") {
		t.Fatalf("expected prompt to include current task text, got %q", prompt)
	}
}

func TestBuildHistorySummaryPromptOmitsCurrentTaskWhenEmpty(t *testing.T) {
	messages := []ports.Message{
		{Role: "user", Content: "context"},
	}
	prompt := buildHistorySummaryPrompt("", messages)
	if strings.Contains(prompt, "Current task:") {
		t.Fatalf("did not expect current task header when task is empty, got %q", prompt)
	}
}
