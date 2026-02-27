package context

import (
	"context"
	"testing"

	"alex/internal/app/subscription"
)

func TestPropagateLLMSelection(t *testing.T) {
	selection := subscription.ResolvedSelection{
		Provider: "openai",
		Model:    "gpt-4o",
		APIKey:   "sk-test",
	}

	from := WithLLMSelection(context.Background(), selection)
	to := context.Background()

	result := PropagateLLMSelection(from, to)

	got, ok := GetLLMSelection(result)
	if !ok {
		t.Fatal("expected LLM selection to be propagated")
	}
	if got.Provider != "openai" || got.Model != "gpt-4o" {
		t.Fatalf("unexpected selection: %+v", got)
	}
}

func TestPropagateLLMSelection_NoSelection(t *testing.T) {
	from := context.Background()
	to := context.Background()

	result := PropagateLLMSelection(from, to)

	if _, ok := GetLLMSelection(result); ok {
		t.Fatal("expected no LLM selection when source has none")
	}
}
