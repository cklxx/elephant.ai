package llm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	id "alex/internal/shared/utils/id"

	"golang.org/x/time/rate"
)

const (
	maxBucketEntries = 1000
	bucketEvictAfter = 1 * time.Hour
)

// bucketEntry wraps a rate limiter with a last-used timestamp for eviction.
type bucketEntry struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

// userRateLimitedClient applies per-user rate limiting around LLM calls.
type userRateLimitedClient struct {
	base   portsllm.LLMClient
	limit  rate.Limit
	burst  int
	mu     sync.Mutex
	bucket map[string]*bucketEntry
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
		bucket: make(map[string]*bucketEntry),
	}

	return streamingUserRateLimitedClient{
		userRateLimitedClient: wrapper,
		streaming:             EnsureStreamingClient(client),
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

	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.bucket[key]
	if ok {
		entry.lastUsed = now
		return entry.limiter
	}

	// Evict stale entries when the map exceeds the maximum size.
	if len(c.bucket) >= maxBucketEntries {
		cutoff := now.Add(-bucketEvictAfter)
		for k, e := range c.bucket {
			if e.lastUsed.Before(cutoff) {
				delete(c.bucket, k)
			}
		}
	}

	limiter := rate.NewLimiter(c.limit, c.burst)
	c.bucket[key] = &bucketEntry{limiter: limiter, lastUsed: now}
	return limiter
}

func (c streamingUserRateLimitedClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	if err := c.reserve(ctx); err != nil {
		return nil, err
	}
	return c.streaming.StreamComplete(ctx, req, callbacks)
}
