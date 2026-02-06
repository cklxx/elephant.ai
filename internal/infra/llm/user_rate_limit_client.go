package llm

import (
	"context"
	"fmt"
	"sync"

	"alex/internal/agent/ports"
	portsllm "alex/internal/agent/ports/llm"
	id "alex/internal/utils/id"

	"golang.org/x/time/rate"
)

// userRateLimitedClient applies per-user rate limiting around LLM calls.
type userRateLimitedClient struct {
	base   portsllm.LLMClient
	limit  rate.Limit
	burst  int
	mu     sync.Mutex
	bucket map[string]*rate.Limiter
}

// streamingUserRateLimitedClient preserves streaming support while enforcing the
// same per-user limiter as the base wrapper.
type streamingUserRateLimitedClient struct {
	*userRateLimitedClient
	streaming portsllm.StreamingLLMClient
}

// WrapWithUserRateLimit wraps the provided client with a per-user limiter when
// a positive limit is supplied. A burst less than 1 is coerced to 1.
func WrapWithUserRateLimit(client portsllm.LLMClient, limit rate.Limit, burst int) portsllm.LLMClient {
	if client == nil {
		return nil
	}

	// Always preserve streaming support so callers can opt into deltas without
	// re-wrapping the client themselves.
	client = EnsureStreamingClient(client)

	if limit <= 0 {
		return client
	}
	if burst < 1 {
		burst = 1
	}

	wrapper := &userRateLimitedClient{
		base:   client,
		limit:  limit,
		burst:  burst,
		bucket: make(map[string]*rate.Limiter),
	}

	if streaming, ok := client.(portsllm.StreamingLLMClient); ok {
		return streamingUserRateLimitedClient{userRateLimitedClient: wrapper, streaming: streaming}
	}

	return streamingUserRateLimitedClient{
		userRateLimitedClient: wrapper,
		streaming:             EnsureStreamingClient(client).(portsllm.StreamingLLMClient),
	}
}

func (c *userRateLimitedClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if err := c.reserve(ctx); err != nil {
		return nil, err
	}
	return c.base.Complete(ctx, req)
}

func (c *userRateLimitedClient) Model() string {
	return c.base.Model()
}

func (c *userRateLimitedClient) reserve(ctx context.Context) error {
	limiter := c.limiterForUser(id.UserIDFromContext(ctx))
	if limiter.Allow() {
		return nil
	}
	return fmt.Errorf("llm rate limit exceeded for user")
}

func (c *userRateLimitedClient) limiterForUser(userID string) *rate.Limiter {
	key := userID
	if key == "" {
		key = "anonymous"
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	limiter, ok := c.bucket[key]
	if !ok {
		limiter = rate.NewLimiter(c.limit, c.burst)
		c.bucket[key] = limiter
	}

	return limiter
}

func (c streamingUserRateLimitedClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	if err := c.reserve(ctx); err != nil {
		return nil, err
	}
	return c.streaming.StreamComplete(ctx, req, callbacks)
}
