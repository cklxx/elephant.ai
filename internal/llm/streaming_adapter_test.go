package llm

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	portsllm "alex/internal/agent/ports/llm"
)

type nonStreamingClient struct {
	content string
}

func (c *nonStreamingClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return &ports.CompletionResponse{Content: c.content}, nil
}

func (c *nonStreamingClient) Model() string { return "stub" }

func TestEnsureStreamingClientWrapsNonStreaming(t *testing.T) {
	base := &nonStreamingClient{content: "hello"}

	wrapped := EnsureStreamingClient(base)
	streaming, ok := wrapped.(portsllm.StreamingLLMClient)
	if !ok {
		t.Fatalf("expected wrapped client to implement StreamingLLMClient")
	}

	var deltas []ports.ContentDelta
	resp, err := streaming.StreamComplete(context.Background(), ports.CompletionRequest{}, ports.CompletionStreamCallbacks{
		OnContentDelta: func(delta ports.ContentDelta) {
			deltas = append(deltas, delta)
		},
	})
	if err != nil {
		t.Fatalf("StreamComplete returned error: %v", err)
	}
	if resp == nil || resp.Content != "hello" {
		t.Fatalf("expected response content 'hello', got %#v", resp)
	}
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas (content + final), got %d", len(deltas))
	}
	if deltas[0].Delta != "hello" || deltas[0].Final {
		t.Fatalf("unexpected first delta: %#v", deltas[0])
	}
	if !deltas[1].Final {
		t.Fatalf("expected second delta to be final: %#v", deltas[1])
	}
}

func TestEnsureStreamingClientPreservesStreamingImplementation(t *testing.T) {
	mock := NewMockClient()
	streaming := EnsureStreamingClient(mock)
	if streaming != mock {
		t.Fatalf("expected existing streaming client to be returned unchanged")
	}
}
