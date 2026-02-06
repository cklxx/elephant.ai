package llm

import (
	"context"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
)

func TestMockClientCompleteDefaultResponse(t *testing.T) {
	t.Parallel()

	client := NewMockClient()

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "Return a concise answer"}},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("Complete returned nil response")
	}

	if resp.Content != "Mock LLM response" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}

	if resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 10 || resp.Usage.TotalTokens != 20 {
		t.Fatalf("unexpected usage: %#v", resp.Usage)
	}
}

func TestMockClientStreamCompleteEmitsDeltas(t *testing.T) {
	t.Parallel()

	client := NewMockClient()
	streaming, ok := client.(portsllm.StreamingLLMClient)
	if !ok {
		t.Fatal("mock client does not implement StreamingLLMClient")
	}

	var deltas []ports.ContentDelta
	resp, err := streaming.StreamComplete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "What is 2 + 2?"}},
	}, ports.CompletionStreamCallbacks{
		OnContentDelta: func(delta ports.ContentDelta) {
			deltas = append(deltas, delta)
		},
	})
	if err != nil {
		t.Fatalf("StreamComplete returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("StreamComplete returned nil response")
	}

	if len(deltas) == 0 {
		t.Fatal("expected streaming deltas, got none")
	}

	if !deltas[len(deltas)-1].Final {
		t.Fatal("final delta not marked as Final")
	}

	var builder strings.Builder
	for _, delta := range deltas {
		builder.WriteString(delta.Delta)
	}

	if builder.String() != resp.Content {
		t.Fatalf("aggregated deltas %q do not match response content %q", builder.String(), resp.Content)
	}
}
