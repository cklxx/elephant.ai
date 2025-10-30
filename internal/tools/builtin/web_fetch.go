package builtin

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"

	"github.com/PuerkitoBio/goquery"
)

// webFetch implements web content fetching with caching and optional LLM processing
type webFetch struct {
	httpClient *http.Client
	llmClient  ports.LLMClient // Optional LLM for content analysis
	cache      *fetchCache
}

// fetchCache manages URL content cache with TTL
type fetchCache struct {
	entries map[string]*cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

type cacheEntry struct {
	content   string
	timestamp time.Time
	url       string
}

func NewWebFetch(cfg WebFetchConfig) ports.ToolExecutor {
	return NewWebFetchWithLLM(nil, cfg)
}

// NewWebFetchWithLLM creates web_fetch with optional LLM client for analysis
func NewWebFetchWithLLM(llmClient ports.LLMClient, cfg WebFetchConfig) ports.ToolExecutor {
	_ = cfg
	cache := &fetchCache{
		entries: make(map[string]*cacheEntry),
		ttl:     15 * time.Minute,
	}

	tool := &webFetch{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				return nil
			},
		},
		llmClient: llmClient,
		cache:     cache,
	}

	// Start background cache cleanup
	go cache.startCleanup()

	return tool
}

func (t *webFetch) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "web_fetch",
		Version:  "1.0.0",
		Category: "web",
		Tags:     []string{"web", "fetch", "http", "content"},
	}
}

func (t *webFetch) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "web_fetch",
		Description: `Fetch and analyze web page content with intelligent processing.

Features:
- Fetches HTML content and converts to clean text
- 15-minute cache for repeated requests
- Handles redirects automatically
- Optional LLM analysis of content
- Extracts headings, paragraphs, lists

Usage:
- url: Full URL to fetch (http/https)
- prompt: Analysis question (optional, requires LLM)`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"url": {
					Type:        "string",
					Description: "Full URL to fetch (http/https)",
				},
				"prompt": {
					Type:        "string",
					Description: "Optional: Question to analyze content with LLM",
				},
			},
			Required: []string{"url"},
		},
	}
}

func (t *webFetch) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Parse URL
	urlStr, _ := call.Arguments["url"].(string)
	if urlStr == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: url parameter required",
			Error:   fmt.Errorf("missing url"),
		}, nil
	}

	// Validate URL
	parsedURL, err := neturl.Parse(urlStr)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Invalid URL format: %v", err),
			Error:   err,
		}, nil
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "URL must use http or https scheme",
			Error:   fmt.Errorf("invalid scheme: %s", parsedURL.Scheme),
		}, nil
	}

	// Upgrade HTTP to HTTPS
	if parsedURL.Scheme == "http" {
		parsedURL.Scheme = "https"
		urlStr = parsedURL.String()
	}

	// Check cache
	cacheKey := t.cache.key(urlStr)
	if cached := t.cache.get(cacheKey); cached != nil {
		return t.buildResult(call.ID, cached.url, cached.content, true, call.Arguments["prompt"])
	}

	// Fetch content
	content, finalURL, err := t.fetchContent(ctx, urlStr)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to fetch URL: %v", err),
			Error:   err,
		}, nil
	}

	// Check for cross-domain redirect
	if t.isDifferentHost(urlStr, finalURL) {
		return &ports.ToolResult{
			CallID: call.ID,
			Content: fmt.Sprintf("URL redirected to different domain:\n\n"+
				"Original: %s\n"+
				"Redirect: %s\n\n"+
				"Please make a new request with the redirect URL.", urlStr, finalURL),
			Metadata: map[string]any{
				"redirected":    true,
				"original_url":  urlStr,
				"redirect_url":  finalURL,
				"original_host": t.getHost(urlStr),
				"final_host":    t.getHost(finalURL),
			},
		}, nil
	}

	// Cache the result
	t.cache.put(cacheKey, &cacheEntry{
		content:   content,
		timestamp: time.Now(),
		url:       finalURL,
	})

	return t.buildResult(call.ID, finalURL, content, false, call.Arguments["prompt"])
}

// fetchContent fetches and processes HTML content
func (t *webFetch) fetchContent(ctx context.Context, urlStr string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "ALEX-Agent/1.0 (Web Content Fetcher)")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read response: %w", err)
	}

	// Convert HTML to clean text
	content, err := t.htmlToText(string(body))
	if err != nil {
		return "", "", fmt.Errorf("parse HTML: %w", err)
	}

	return content, resp.Request.URL.String(), nil
}

// htmlToText converts HTML to clean markdown-like text
func (t *webFetch) htmlToText(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}

	// Remove noise elements
	doc.Find("script, style, nav, footer, header, aside, iframe").Remove()

	var content strings.Builder

	// Extract title
	if title := doc.Find("title").Text(); title != "" {
		content.WriteString("# " + strings.TrimSpace(title) + "\n\n")
	}

	// Extract headings
	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); text != "" {
			level := s.Get(0).Data[1] - '0' // Extract level from h1,h2,etc
			prefix := strings.Repeat("#", int(level))
			content.WriteString(prefix + " " + text + "\n\n")
		}
	})

	// Extract paragraphs and content blocks
	doc.Find("p, div.content, article, section").Each(func(i int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); text != "" && len(text) > 30 {
			content.WriteString(text + "\n\n")
		}
	})

	// Extract lists
	doc.Find("ul, ol").Each(func(i int, s *goquery.Selection) {
		s.Find("li").Each(func(j int, li *goquery.Selection) {
			if text := strings.TrimSpace(li.Text()); text != "" {
				content.WriteString("• " + text + "\n")
			}
		})
		content.WriteString("\n")
	})

	result := content.String()

	// Limit content size
	const maxSize = 15000
	if len(result) > maxSize {
		result = result[:maxSize] + "\n\n[Content truncated...]"
	}

	return result, nil
}

// buildResult constructs the final tool result with optional LLM analysis
func (t *webFetch) buildResult(callID, url, content string, cached bool, promptArg any) (*ports.ToolResult, error) {
	prompt, hasPrompt := promptArg.(string)

	// If LLM available and prompt provided, analyze content
	if hasPrompt && prompt != "" && t.llmClient != nil {
		analysisCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		analysis, err := t.analyzeLLM(analysisCtx, content, prompt)
		if err == nil && analysis != "" {
			// Return LLM analysis (no emojis for TUI compatibility)
			output := fmt.Sprintf("Source: %s%s\n\n"+
				"Question: %s\n\n"+
				"Analysis:\n%s",
				url,
				cacheStatus(cached),
				prompt,
				analysis)

			metadata := map[string]any{
				"url":          url,
				"cached":       cached,
				"analyzed":     true,
				"content_size": len(content),
			}
			metadata["web"] = map[string]any{
				"url":      url,
				"content":  content,
				"prompt":   prompt,
				"analysis": analysis,
			}

			return &ports.ToolResult{
				CallID:   callID,
				Content:  output,
				Metadata: metadata,
			}, nil
		}
	}

	// Return raw content (no emojis for TUI compatibility)
	output := fmt.Sprintf("Source: %s%s\n\n%s",
		url,
		cacheStatus(cached),
		content)

	metadata := map[string]any{
		"url":          url,
		"cached":       cached,
		"analyzed":     false,
		"content_size": len(content),
	}
	metadata["web"] = map[string]any{
		"url":     url,
		"content": content,
	}

	return &ports.ToolResult{
		CallID:   callID,
		Content:  output,
		Metadata: metadata,
	}, nil
}

// analyzeLLM uses LLM to analyze content based on prompt
func (t *webFetch) analyzeLLM(ctx context.Context, content, prompt string) (string, error) {
	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role: "user",
				Content: fmt.Sprintf("Based on this web page content:\n\n%s\n\n%s",
					content, prompt),
			},
		},
		Temperature: 0.3,
		MaxTokens:   1000,
	}

	resp, err := t.llmClient.Complete(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// Helper functions
func (t *webFetch) isDifferentHost(url1, url2 string) bool {
	return t.getHost(url1) != t.getHost(url2)
}

func (t *webFetch) getHost(urlStr string) string {
	u, _ := neturl.Parse(urlStr)
	return u.Host
}

func cacheStatus(cached bool) string {
	if cached {
		return " (cached)"
	}
	return ""
}

// Cache implementation
func (c *fetchCache) key(url string) string {
	hash := md5.Sum([]byte(url))
	return fmt.Sprintf("%x", hash)
}

func (c *fetchCache) get(key string) *cacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.entries[key]; ok {
		if time.Since(entry.timestamp) < c.ttl {
			return entry
		}
	}
	return nil
}

func (c *fetchCache) put(key string, entry *cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry
}

func (c *fetchCache) startCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		for key, entry := range c.entries {
			if time.Since(entry.timestamp) > c.ttl {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}
