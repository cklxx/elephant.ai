package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"alex/internal/httpclient"
)

const defaultSandboxBaseURL = "http://localhost:18086"
const sandboxSessionTTL = 10 * time.Minute

type Config struct {
	BaseURL          string
	Timeout          time.Duration
	MaxResponseBytes int
}

type Client struct {
	baseURL          string
	httpClient       *http.Client
	cache            *sessionCache
	maxResponseBytes int
}

type sessionCache struct {
	mu       sync.Mutex
	sessions map[string]*sandboxSession
	counter  uint64
}

type sandboxSession struct {
	id       string
	lastUsed time.Time
}

var sandboxSessionCaches sync.Map

func NewClient(cfg Config) *Client {
	baseURL := normalizeBaseURL(cfg.BaseURL)
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	maxResponseBytes := cfg.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = 8 * 1024 * 1024
	}

	return &Client{
		baseURL:          baseURL,
		httpClient:       httpclient.NewWithCircuitBreaker(timeout, nil, "sandbox"),
		cache:            sessionCacheFor(baseURL),
		maxResponseBytes: maxResponseBytes,
	}
}

func (c *Client) DoJSON(ctx context.Context, method, path string, payload any, sessionID string, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal sandbox request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := c.newRequest(ctx, method, path, body, sessionID)
	if err != nil {
		return fmt.Errorf("build sandbox request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sandbox request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		data, err := httpclient.ReadAllWithLimit(resp.Body, int64(c.maxResponseBytes))
		if err != nil {
			if httpclient.IsResponseTooLarge(err) {
				return fmt.Errorf("sandbox response exceeds %d bytes", c.maxResponseBytes)
			}
			return fmt.Errorf("sandbox request failed: %w", err)
		}
		return fmt.Errorf("sandbox request failed: %s", strings.TrimSpace(string(data)))
	}

	if out == nil {
		return nil
	}
	data, err := httpclient.ReadAllWithLimit(resp.Body, int64(c.maxResponseBytes))
	if err != nil {
		if httpclient.IsResponseTooLarge(err) {
			return fmt.Errorf("sandbox response exceeds %d bytes", c.maxResponseBytes)
		}
		return fmt.Errorf("read sandbox response: %w", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode sandbox response: %w", err)
	}
	return nil
}

func (c *Client) GetBytes(ctx context.Context, path string, sessionID string) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil, sessionID)
	if err != nil {
		return nil, fmt.Errorf("build sandbox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sandbox request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		data, _ := httpclient.ReadAllWithLimit(resp.Body, int64(c.maxResponseBytes))
		return nil, fmt.Errorf("sandbox request failed: %s", strings.TrimSpace(string(data)))
	}

	payload, err := httpclient.ReadAllWithLimit(resp.Body, int64(c.maxResponseBytes))
	if err != nil {
		if httpclient.IsResponseTooLarge(err) {
			return nil, fmt.Errorf("sandbox response exceeds %d bytes", c.maxResponseBytes)
		}
		return nil, fmt.Errorf("read sandbox response: %w", err)
	}
	return payload, nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader, sessionID string) (*http.Request, error) {
	url := c.baseURL
	if strings.HasPrefix(path, "/") {
		url += path
	} else {
		url += "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if sessionID != "" {
		req.Header.Set("X-Session-ID", c.resolveSessionID(sessionID))
	}
	return req, nil
}

func (c *Client) resolveSessionID(sessionID string) string {
	if sessionID == "" {
		return ""
	}

	now := time.Now()

	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()

	if current, ok := c.cache.sessions[sessionID]; ok {
		if now.Sub(current.lastUsed) <= sandboxSessionTTL {
			current.lastUsed = now
			return current.id
		}
	}

	c.cache.counter++
	entry := &sandboxSession{
		id:       fmt.Sprintf("%s-sandbox-%d", sessionID, c.cache.counter),
		lastUsed: now,
	}
	c.cache.sessions[sessionID] = entry
	c.pruneExpiredSessions(now)
	return entry.id
}

func (c *Client) pruneExpiredSessions(now time.Time) {
	for key, entry := range c.cache.sessions {
		if now.Sub(entry.lastUsed) > sandboxSessionTTL {
			delete(c.cache.sessions, key)
		}
	}
}

func sessionCacheFor(baseURL string) *sessionCache {
	if cached, ok := sandboxSessionCaches.Load(baseURL); ok {
		return cached.(*sessionCache)
	}
	cache := &sessionCache{sessions: make(map[string]*sandboxSession)}
	actual, _ := sandboxSessionCaches.LoadOrStore(baseURL, cache)
	return actual.(*sessionCache)
}

func normalizeBaseURL(raw string) string {
	baseURL := strings.TrimSpace(raw)
	if baseURL == "" {
		baseURL = defaultSandboxBaseURL
	}
	if !strings.Contains(baseURL, "://") {
		baseURL = "http://" + baseURL
	}
	return strings.TrimRight(baseURL, "/")
}
