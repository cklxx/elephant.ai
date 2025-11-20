package llm

import (
	"context"

	"alex/internal/agent/ports"
)

// streamingAdapter wraps an LLMClient that lacks native streaming support and
// synthesizes CompletionStreamCallbacks by invoking Complete.
type streamingAdapter struct {
	base ports.LLMClient
}

var _ ports.StreamingLLMClient = (*streamingAdapter)(nil)

// ensureStreamingClient guarantees the returned client implements
// StreamingLLMClient by wrapping non-streaming implementations with a fallback
// adapter.
func ensureStreamingClient(client ports.LLMClient) ports.LLMClient {
	if client == nil {
		return nil
	}
	if _, ok := client.(ports.StreamingLLMClient); ok {
		return client
	}
	return &streamingAdapter{base: client}
}

func (a *streamingAdapter) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return a.base.Complete(ctx, req)
}

func (a *streamingAdapter) Model() string {
	return a.base.Model()
}

func (a *streamingAdapter) StreamComplete(
	ctx context.Context,
	req ports.CompletionRequest,
	callbacks ports.CompletionStreamCallbacks,
) (*ports.CompletionResponse, error) {
	if streaming, ok := a.base.(ports.StreamingLLMClient); ok {
		return streaming.StreamComplete(ctx, req, callbacks)
	}

	resp, err := a.base.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	if callbacks.OnContentDelta != nil {
		if resp != nil && resp.Content != "" {
			callbacks.OnContentDelta(ports.ContentDelta{Delta: resp.Content})
		}
		callbacks.OnContentDelta(ports.ContentDelta{Final: true})
	}

	return resp, nil
}
