package builtin

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

const sandboxSessionTTL = 10 * time.Minute

type sandboxSession struct {
	id       string
	lastUsed time.Time
}

type SandboxConfig struct {
	BaseURL string
}

type sandboxClient struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.Mutex
	sessions   map[string]*sandboxSession
	counter    uint64
}

func newSandboxClient(cfg SandboxConfig) *sandboxClient {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")

	return &sandboxClient{
		baseURL:    baseURL,
		httpClient: httpclient.New(30*time.Second, nil),
		sessions:   make(map[string]*sandboxSession),
	}
}

func (c *sandboxClient) doJSON(ctx context.Context, method, path string, payload any, sessionID string, out any) error {
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
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sandbox request failed: %s", strings.TrimSpace(string(data)))
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode sandbox response: %w", err)
	}
	return nil
}

func (c *sandboxClient) getBytes(ctx context.Context, path string, sessionID string) ([]byte, error) {
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
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox request failed: %s", strings.TrimSpace(string(data)))
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read sandbox response: %w", err)
	}
	return payload, nil
}

func (c *sandboxClient) newRequest(ctx context.Context, method, path string, body io.Reader, sessionID string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if sessionID != "" {
		req.Header.Set("X-Session-ID", c.resolveSessionID(sessionID))
	}
	return req, nil
}

func (c *sandboxClient) resolveSessionID(sessionID string) string {
	if sessionID == "" {
		return ""
	}

	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	if current, ok := c.sessions[sessionID]; ok {
		if now.Sub(current.lastUsed) <= sandboxSessionTTL {
			current.lastUsed = now
			return current.id
		}
	}

	c.counter++
	entry := &sandboxSession{
		id:       fmt.Sprintf("%s-sandbox-%d", sessionID, c.counter),
		lastUsed: now,
	}
	c.sessions[sessionID] = entry
	c.pruneExpiredSessions(now)
	return entry.id
}

func (c *sandboxClient) pruneExpiredSessions(now time.Time) {
	for key, entry := range c.sessions {
		if now.Sub(entry.lastUsed) > sandboxSessionTTL {
			delete(c.sessions, key)
		}
	}
}
