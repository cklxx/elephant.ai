package moltbook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"alex/internal/httpclient"
)

const defaultBaseURL = "https://www.moltbook.com"

// Config holds Moltbook client configuration.
type Config struct {
	BaseURL string
	APIKey  string
}

// Client is a Moltbook API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient creates a new Moltbook API client.
func NewClient(cfg Config) *Client {
	return newClient(cfg, nil)
}

func newClient(cfg Config, httpClient *http.Client) *Client {
	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = httpclient.NewWithCircuitBreaker(30*time.Second, nil, "moltbook")
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    base,
		apiKey:     cfg.APIKey,
	}
}

// CreatePost publishes a new post.
func (c *Client) CreatePost(ctx context.Context, req CreatePostRequest) (*Post, error) {
	var post Post
	if err := c.do(ctx, http.MethodPost, "/api/v1/posts", req, &post); err != nil {
		return nil, err
	}
	return &post, nil
}

// GetFeed retrieves the feed for a given page.
func (c *Client) GetFeed(ctx context.Context, page int) ([]Post, error) {
	if page < 1 {
		page = 1
	}
	path := fmt.Sprintf("/api/v1/feed?page=%d", page)
	var posts []Post
	if err := c.do(ctx, http.MethodGet, path, nil, &posts); err != nil {
		return nil, err
	}
	return posts, nil
}

// CreateComment adds a comment to a post.
func (c *Client) CreateComment(ctx context.Context, postID string, req CreateCommentRequest) (*Comment, error) {
	path := fmt.Sprintf("/api/v1/posts/%s/comments", url.PathEscape(postID))
	var comment Comment
	if err := c.do(ctx, http.MethodPost, path, req, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// Upvote upvotes a post.
func (c *Client) Upvote(ctx context.Context, postID string) error {
	path := fmt.Sprintf("/api/v1/posts/%s/upvote", url.PathEscape(postID))
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// Downvote downvotes a post.
func (c *Client) Downvote(ctx context.Context, postID string) error {
	path := fmt.Sprintf("/api/v1/posts/%s/downvote", url.PathEscape(postID))
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// Search searches Moltbook for the given query.
func (c *Client) Search(ctx context.Context, query string) (*SearchResult, error) {
	path := fmt.Sprintf("/api/v1/search?q=%s", url.QueryEscape(query))
	var result SearchResult
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetProfile retrieves the authenticated agent's profile.
func (c *Client) GetProfile(ctx context.Context) (*AgentProfile, error) {
	var profile AgentProfile
	if err := c.do(ctx, http.MethodGet, "/api/v1/agents/me", nil, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// UpdateProfile updates the authenticated agent's description.
func (c *Client) UpdateProfile(ctx context.Context, description string) error {
	body := map[string]string{"description": description}
	return c.do(ctx, http.MethodPut, "/api/v1/agents/me", body, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("moltbook: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("moltbook: create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("moltbook: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("moltbook: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		msg := string(respBody)
		var apiErr struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil {
			if apiErr.Message != "" {
				msg = apiErr.Message
			} else if apiErr.Error != "" {
				msg = apiErr.Error
			}
		}
		return &MoltbookError{StatusCode: resp.StatusCode, Message: msg}
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("moltbook: decode response: %w", err)
		}
	}

	return nil
}
