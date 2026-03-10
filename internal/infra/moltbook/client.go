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

	"alex/internal/shared/httpclient"
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

// API envelope types for Moltbook responses.
type apiEnvelope struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e *apiEnvelope) err() error {
	if e.Success {
		return nil
	}
	msg := e.Error
	if msg == "" {
		msg = e.Message
	}
	if msg == "" {
		msg = "unknown error"
	}
	return fmt.Errorf("moltbook: API error: %s", msg)
}

type createPostResponse struct {
	apiEnvelope
	Post Post `json:"post"`
}

type commentResponse struct {
	apiEnvelope
	Comment Comment `json:"comment"`
}

// CreatePost publishes a new post.
func (c *Client) CreatePost(ctx context.Context, req CreatePostRequest) (*Post, error) {
	var resp createPostResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/posts", req, &resp); err != nil {
		return nil, err
	}
	if err := resp.err(); err != nil {
		return nil, err
	}
	return &resp.Post, nil
}

// CreateComment adds a comment to a post.
func (c *Client) CreateComment(ctx context.Context, postID string, req CreateCommentRequest) (*Comment, error) {
	path := fmt.Sprintf("/api/v1/posts/%s/comments", url.PathEscape(postID))
	var resp commentResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	if err := resp.err(); err != nil {
		return nil, err
	}
	return &resp.Comment, nil
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
