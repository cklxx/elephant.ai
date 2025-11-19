package llm

import (
	"context"

	"alex/internal/agent/ports"
)

// streamingAdapter wraps an LLMClient that lacks native streaming support and
// synthesizes CompletionStreamCallbacks by invoking Complete.
type streamingAdapter struct {
	ports.LLMClient
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
	return &streamingAdapter{LLMClient: client}
}

func (a *streamingAdapter) StreamComplete(
	ctx context.Context,
	req ports.CompletionRequest,
	callbacks ports.CompletionStreamCallbacks,
) (*ports.CompletionResponse, error) {
	if streaming, ok := a.LLMClient.(ports.StreamingLLMClient); ok {
		return streaming.StreamComplete(ctx, req, callbacks)
	}

	resp, err := a.LLMClient.Complete(ctx, req)
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
