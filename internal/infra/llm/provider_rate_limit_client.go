package llm

import (
	"context"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"

	"golang.org/x/time/rate"
)

type sharedRateLimitedClient struct {
	base    portsllm.LLMClient
	limiter *rate.Limiter
}

type streamingSharedRateLimitedClient struct {
	*sharedRateLimitedClient
	streaming portsllm.StreamingLLMClient
}

// WrapWithSharedRateLimit applies a shared limiter to all calls made through
// this client wrapper. The same limiter can be reused across multiple client
// instances to enforce a global provider/model quota.
func WrapWithSharedRateLimit(client portsllm.LLMClient, limiter *rate.Limiter) portsllm.LLMClient {
	if client == nil {
		return nil
	}
	client = EnsureStreamingClient(client)
	if limiter == nil {
		return client
	}

	wrapper := &sharedRateLimitedClient{
		base:    client,
		limiter: limiter,
	}
	streaming := EnsureStreamingClient(client).(portsllm.StreamingLLMClient)
	return streamingSharedRateLimitedClient{
		sharedRateLimitedClient: wrapper,
		streaming:               streaming,
	}
}

func (c *sharedRateLimitedClient) wait(ctx context.Context) error {
	return c.limiter.Wait(ctx)
}

func (c *sharedRateLimitedClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	return c.base.Complete(ctx, req)
}

func (c *sharedRateLimitedClient) Model() string {
	return c.base.Model()
}

func (c streamingSharedRateLimitedClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	return c.streaming.StreamComplete(ctx, req, callbacks)
}
