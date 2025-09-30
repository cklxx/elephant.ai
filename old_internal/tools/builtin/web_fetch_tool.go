package builtin

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"alex/internal/llm"

	"github.com/PuerkitoBio/goquery"
)

// WebFetchTool implements web content fetching and AI processing
type WebFetchTool struct {
	client    *http.Client
	llmClient llm.Client
	cache     map[string]*CacheEntry
	cacheMu   sync.RWMutex
}

// CacheEntry represents a cached web fetch result
type CacheEntry struct {
	Content   string
	Timestamp time.Time
	URL       string
}

// CreateWebFetchTool creates a new web fetch tool
func CreateWebFetchTool() *WebFetchTool {
	return CreateWebFetchToolWithLLM(nil)
}

// CreateWebFetchToolWithLLM creates a new web fetch tool with LLM client
func CreateWebFetchToolWithLLM(llmClient llm.Client) *WebFetchTool {
	tool := &WebFetchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Allow up to 10 redirects
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				return nil
			},
		},
		llmClient: llmClient,
		cache:     make(map[string]*CacheEntry),
	}

	// Start cache cleanup goroutine
	go tool.cacheCleanup()

	return tool
}

func (t *WebFetchTool) Name() string {
	return "web_fetch"
}

func (t *WebFetchTool) Description() string {
	return "Fetches content from a specified URL and processes it using an AI model. Takes a URL and a prompt as input, fetches the URL content, converts HTML to markdown, and processes the content with the prompt using a small, fast model."
}

func (t *WebFetchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to fetch content from. Must be a fully-formed valid URL. HTTP URLs will be automatically upgraded to HTTPS.",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "The prompt to run on the fetched content. Should describe what information you want to extract from the page.",
			},
		},
		"required": []string{"url", "prompt"},
	}
}

func (t *WebFetchTool) Validate(args map[string]any) error {
	validator := NewValidationFramework().
		AddStringField("url", "The URL to fetch content from").
		AddStringField("prompt", "The prompt to run on the fetched content")

	if err := validator.Validate(args); err != nil {
		return err
	}

	// Validate URL format
	urlStr := args["url"].(string)
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check for valid scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}

	// Check for valid host
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid host")
	}

	return nil
}

func (t *WebFetchTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	urlStr := args["url"].(string)
	prompt := args["prompt"].(string)

	// Upgrade HTTP to HTTPS
	if strings.HasPrefix(urlStr, "http://") {
		urlStr = strings.Replace(urlStr, "http://", "https://", 1)
	}

	// Check cache first
	cacheKey := t.getCacheKey(urlStr)
	if cached := t.getFromCache(cacheKey); cached != nil {
		// Return cached raw content (skip AI processing for now)
		formattedContent := fmt.Sprintf("**Fetched from:** %s (cached)\n\n**User Query:** %s\n\n**Page Content:**\n\n%s\n\n---\n*Note: Content served from cache. AI analysis temporarily disabled due to configuration issues.*",
			urlStr, prompt, cached.Content)

		return &ToolResult{
			Content: formattedContent,
			Data: map[string]any{
				"url":             urlStr,
				"content_size":    len(cached.Content),
				"prompt":          prompt,
				"cached":          true,
				"cache_timestamp": cached.Timestamp,
				"ai_processed":    false,
				"ai_available":    false,
				"status":          "cached_raw_content_only",
				"content":         formattedContent,
			},
		}, nil
	}

	// Fetch content
	content, finalURL, err := t.fetchContent(ctx, urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content: %w", err)
	}

	// Check for redirect to different host
	if finalURL != urlStr {
		originalHost := t.getHost(urlStr)
		finalHost := t.getHost(finalURL)
		if originalHost != finalHost {
			return &ToolResult{
				Content: fmt.Sprintf("The URL redirected to a different host:\nOriginal: %s\nRedirect: %s\n\nPlease make a new WebFetch request with the redirect URL to fetch the content.", urlStr, finalURL),
				Data: map[string]any{
					"redirected":    true,
					"original_url":  urlStr,
					"redirect_url":  finalURL,
					"original_host": originalHost,
					"final_host":    finalHost,
					"content":       content,
				},
			}, nil
		}
	}

	// Cache the content
	t.putInCache(cacheKey, &CacheEntry{
		Content:   content,
		Timestamp: time.Now(),
		URL:       finalURL,
	})

	// For now, skip AI processing entirely and return formatted raw content
	// TODO: Re-enable AI processing once LLM configuration is fixed
	formattedContent := fmt.Sprintf("**Fetched from:** %s\n\n**User Query:** %s\n\n**Page Content:**\n\n%s\n\n---\n*Note: Showing raw extracted content. AI analysis temporarily disabled due to configuration issues.*",
		finalURL, prompt, content)

	return &ToolResult{
		Content: formattedContent,
		Data: map[string]any{
			"url":          finalURL,
			"content_size": len(content),
			"prompt":       prompt,
			"cached":       false,
			"ai_processed": false,
			"ai_available": false,
			"status":       "raw_content_only",
			"content":      formattedContent,
		},
	}, nil
}

func (t *WebFetchTool) fetchContent(ctx context.Context, urlStr string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", "ALEX-Agent/1.0 (Web Content Fetcher)")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	// Convert HTML to markdown-like text
	content, err := t.htmlToText(string(body))
	if err != nil {
		return "", "", fmt.Errorf("failed to process HTML: %w", err)
	}

	return content, resp.Request.URL.String(), nil
}

func (t *WebFetchTool) htmlToText(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}

	// Remove script and style elements
	doc.Find("script, style, nav, footer, header, aside").Remove()

	var content strings.Builder

	// Extract title
	title := doc.Find("title").Text()
	if title != "" {
		content.WriteString("# " + strings.TrimSpace(title) + "\n\n")
	}

	// Extract main content
	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			level := strings.Repeat("#", len(s.Get(0).Data)-1) // h1=1, h2=2, etc.
			content.WriteString(level + " " + text + "\n\n")
		}
	})

	doc.Find("p, div, article, section").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" && len(text) > 20 { // Filter out very short text
			content.WriteString(text + "\n\n")
		}
	})

	// Extract lists
	doc.Find("ul, ol").Each(func(i int, s *goquery.Selection) {
		s.Find("li").Each(func(j int, li *goquery.Selection) {
			text := strings.TrimSpace(li.Text())
			if text != "" {
				content.WriteString("- " + text + "\n")
			}
		})
		content.WriteString("\n")
	})

	result := content.String()

	// Limit content size
	if len(result) > 15000 {
		result = result[:15000] + "\n\n[Content truncated due to length...]"
	}

	return result, nil
}

func (t *WebFetchTool) getCacheKey(url string) string {
	hash := md5.Sum([]byte(url))
	return fmt.Sprintf("%x", hash)
}

func (t *WebFetchTool) getFromCache(key string) *CacheEntry {
	t.cacheMu.RLock()
	defer t.cacheMu.RUnlock()

	if entry, ok := t.cache[key]; ok {
		// Check if cache entry is still valid (15 minutes)
		if time.Since(entry.Timestamp) < 15*time.Minute {
			return entry
		}
	}
	return nil
}

func (t *WebFetchTool) putInCache(key string, entry *CacheEntry) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	t.cache[key] = entry
}

func (t *WebFetchTool) cacheCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		t.cacheMu.Lock()
		for key, entry := range t.cache {
			if time.Since(entry.Timestamp) > 15*time.Minute {
				delete(t.cache, key)
			}
		}
		t.cacheMu.Unlock()
	}
}

func (t *WebFetchTool) getHost(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return u.Host
}
