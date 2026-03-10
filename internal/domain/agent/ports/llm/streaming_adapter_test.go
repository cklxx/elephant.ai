package llm

import (
	"context"
	"errors"
	"testing"

	core "alex/internal/domain/agent/ports"
)

type mockLLMClient struct {
	model  string
	resp   *core.CompletionResponse
	err    error
	called bool
}

func (m *mockLLMClient) Complete(_ context.Context, _ core.CompletionRequest) (*core.CompletionResponse, error) {
	m.called = true
	return m.resp, m.err
}
func (m *mockLLMClient) Model() string { return m.model }

type mockStreamingClient struct {
	mockLLMClient
	streamCalled bool
}

func (m *mockStreamingClient) StreamComplete(_ context.Context, _ core.CompletionRequest, _ core.CompletionStreamCallbacks) (*core.CompletionResponse, error) {
	m.streamCalled = true
	return m.resp, m.err
}

func TestEnsureStreamingClient_Nil(t *testing.T) {
	if EnsureStreamingClient(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

func TestEnsureStreamingClient_AlreadyStreaming(t *testing.T) {
	sc := &mockStreamingClient{mockLLMClient: mockLLMClient{model: "gpt-4"}}
	result := EnsureStreamingClient(sc)
	if result != sc {
		t.Error("expected same instance for already-streaming client")
	}
}

func TestEnsureStreamingClient_Wraps(t *testing.T) {
	mc := &mockLLMClient{model: "gpt-3.5"}
	result := EnsureStreamingClient(mc)
	if _, ok := result.(StreamingLLMClient); !ok {
		t.Error("expected StreamingLLMClient")
	}
}

func TestStreamingAdapter_Complete(t *testing.T) {
	resp := &core.CompletionResponse{Content: "hello"}
	mc := &mockLLMClient{model: "test", resp: resp}
	adapter := EnsureStreamingClient(mc).(StreamingLLMClient)

	got, err := adapter.Complete(context.Background(), core.CompletionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "hello" {
		t.Errorf("expected hello, got %s", got.Content)
	}
}

func TestStreamingAdapter_Model(t *testing.T) {
	mc := &mockLLMClient{model: "claude-3"}
	adapter := EnsureStreamingClient(mc)
	if adapter.(StreamingLLMClient).Model() != "claude-3" {
		t.Error("model mismatch")
	}
}

func TestStreamingAdapter_StreamComplete_Fallback(t *testing.T) {
	resp := &core.CompletionResponse{Content: "streamed content"}
	mc := &mockLLMClient{model: "test", resp: resp}
	adapter := EnsureStreamingClient(mc).(StreamingLLMClient)

	var deltas []core.ContentDelta
	callbacks := core.CompletionStreamCallbacks{
		OnContentDelta: func(d core.ContentDelta) {
			deltas = append(deltas, d)
		},
	}

	got, err := adapter.StreamComplete(context.Background(), core.CompletionRequest{}, callbacks)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "streamed content" {
		t.Errorf("expected streamed content, got %s", got.Content)
	}
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas (content + final), got %d", len(deltas))
	}
	if deltas[0].Delta != "streamed content" {
		t.Errorf("first delta should be content")
	}
	if !deltas[1].Final {
		t.Error("second delta should be final")
	}
}

func TestStreamingAdapter_StreamComplete_Error(t *testing.T) {
	mc := &mockLLMClient{model: "test", err: errors.New("api error")}
	adapter := EnsureStreamingClient(mc).(StreamingLLMClient)

	_, err := adapter.StreamComplete(context.Background(), core.CompletionRequest{}, core.CompletionStreamCallbacks{})
	if err == nil || err.Error() != "api error" {
		t.Errorf("expected api error, got %v", err)
	}
}

func TestStreamingAdapter_StreamComplete_NilCallback(t *testing.T) {
	resp := &core.CompletionResponse{Content: "ok"}
	mc := &mockLLMClient{model: "test", resp: resp}
	adapter := EnsureStreamingClient(mc).(StreamingLLMClient)

	got, err := adapter.StreamComplete(context.Background(), core.CompletionRequest{}, core.CompletionStreamCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "ok" {
		t.Errorf("expected ok, got %s", got.Content)
	}
}

func TestStreamingAdapter_StreamComplete_EmptyContent(t *testing.T) {
	resp := &core.CompletionResponse{Content: ""}
	mc := &mockLLMClient{model: "test", resp: resp}
	adapter := EnsureStreamingClient(mc).(StreamingLLMClient)

	var deltas []core.ContentDelta
	callbacks := core.CompletionStreamCallbacks{
		OnContentDelta: func(d core.ContentDelta) {
			deltas = append(deltas, d)
		},
	}

	_, err := adapter.StreamComplete(context.Background(), core.CompletionRequest{}, callbacks)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta (final only), got %d", len(deltas))
	}
	if !deltas[0].Final {
		t.Error("expected final delta")
	}
}
