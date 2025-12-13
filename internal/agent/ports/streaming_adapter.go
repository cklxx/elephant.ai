package ports

import "context"

// streamingAdapter wraps an LLMClient that lacks native streaming support and
// synthesizes CompletionStreamCallbacks by invoking Complete.
type streamingAdapter struct {
	base LLMClient
}

var _ StreamingLLMClient = (*streamingAdapter)(nil)

// EnsureStreamingClient guarantees the returned client implements
// StreamingLLMClient by wrapping non-streaming implementations with a fallback
// adapter.
func EnsureStreamingClient(client LLMClient) LLMClient {
	if client == nil {
		return nil
	}
	if _, ok := client.(StreamingLLMClient); ok {
		return client
	}
	return &streamingAdapter{base: client}
}

func (a *streamingAdapter) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	return a.base.Complete(ctx, req)
}

func (a *streamingAdapter) Model() string {
	return a.base.Model()
}

func (a *streamingAdapter) StreamComplete(
	ctx context.Context,
	req CompletionRequest,
	callbacks CompletionStreamCallbacks,
) (*CompletionResponse, error) {
	if streaming, ok := a.base.(StreamingLLMClient); ok {
		return streaming.StreamComplete(ctx, req, callbacks)
	}

	resp, err := a.base.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	if callbacks.OnContentDelta != nil {
		if resp != nil && resp.Content != "" {
			callbacks.OnContentDelta(ContentDelta{Delta: resp.Content})
		}
		callbacks.OnContentDelta(ContentDelta{Final: true})
	}

	return resp, nil
}
