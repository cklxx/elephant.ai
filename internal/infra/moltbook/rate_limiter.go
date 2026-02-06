package moltbook

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Rate limit defaults for Moltbook API.
const (
	postCooldown    = 30 * time.Minute
	commentCooldown = 20 * time.Second
)

// RateLimitedClient wraps a Client with rate limiting.
type RateLimitedClient struct {
	*Client

	mu          sync.Mutex
	lastPost    time.Time
	lastComment time.Time
}

// NewRateLimitedClient creates a rate-limited Moltbook client.
func NewRateLimitedClient(cfg Config) *RateLimitedClient {
	return newRateLimitedClient(cfg, nil)
}

func newRateLimitedClient(cfg Config, httpClient *Client) *RateLimitedClient {
	if httpClient == nil {
		httpClient = NewClient(cfg)
	}
	return &RateLimitedClient{Client: httpClient}
}

// CreatePost publishes a post, enforcing the post rate limit.
func (r *RateLimitedClient) CreatePost(ctx context.Context, req CreatePostRequest) (*Post, error) {
	r.mu.Lock()
	since := time.Since(r.lastPost)
	if since < postCooldown {
		remaining := postCooldown - since
		r.mu.Unlock()
		return nil, fmt.Errorf("moltbook: rate limited — next post allowed in %s", remaining.Truncate(time.Second))
	}
	r.mu.Unlock()

	post, err := r.Client.CreatePost(ctx, req)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.lastPost = time.Now()
	r.mu.Unlock()
	return post, nil
}

// CreateComment adds a comment, enforcing the comment rate limit.
func (r *RateLimitedClient) CreateComment(ctx context.Context, postID string, req CreateCommentRequest) (*Comment, error) {
	r.mu.Lock()
	since := time.Since(r.lastComment)
	if since < commentCooldown {
		remaining := commentCooldown - since
		r.mu.Unlock()
		return nil, fmt.Errorf("moltbook: rate limited — next comment allowed in %s", remaining.Truncate(time.Second))
	}
	r.mu.Unlock()

	comment, err := r.Client.CreateComment(ctx, postID, req)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.lastComment = time.Now()
	r.mu.Unlock()
	return comment, nil
}
