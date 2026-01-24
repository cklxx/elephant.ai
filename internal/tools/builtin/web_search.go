package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/httpclient"
	"golang.org/x/net/html"
)

type webSearch struct {
	client *http.Client
	apiKey string
}

type webSearchResult struct {
	Title   string
	URL     string
	Content string
}

func NewWebSearch(apiKey string) ports.ToolExecutor {
	return newWebSearch(apiKey, nil)
}

func newWebSearch(apiKey string, client *http.Client) *webSearch {
	if client == nil {
		client = httpclient.NewWithCircuitBreaker(30*time.Second, nil, "web_search")
	}
	return &webSearch{client: client, apiKey: apiKey}
}

func (t *webSearch) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "web_search",
		Version:  "1.0.0",
		Category: "web",
		Tags:     []string{"search", "web", "internet"},
	}
}

func (t *webSearch) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "web_search",
		Description: `Search the web for current information using Tavily API.

Returns relevant search results with summaries and URLs.

Setup:
1. Get API key from https://app.tavily.com/
2. Set runtime.tavily_api_key in ~/.alex/config.yaml (you can reference ${TAVILY_API_KEY})`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"query": {
					Type:        "string",
					Description: "The search query to execute",
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum number of results (1-10, default 5)",
				},
				"search_depth": {
					Type:        "string",
					Description: "Search depth: basic or advanced",
					Enum:        []any{"basic", "advanced"},
				},
			},
			Required: []string{"query"},
		},
	}
}

func (t *webSearch) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Parse parameters
	query, _ := call.Arguments["query"].(string)
	if query == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: query parameter required",
			Error:   fmt.Errorf("missing query"),
		}, nil
	}

	maxResults := 5
	if mr, ok := call.Arguments["max_results"].(float64); ok {
		maxResults = int(mr)
		if maxResults < 1 {
			maxResults = 1
		}
		if maxResults > 10 {
			maxResults = 10
		}
	}

	searchDepth := "basic"
	if sd, ok := call.Arguments["search_depth"].(string); ok {
		searchDepth = sd
	}

	if t.apiKey != "" {
		result, err := t.searchTavily(ctx, query, maxResults, searchDepth)
		if err == nil {
			result.CallID = call.ID
			return result, nil
		}
	}

	return t.searchFallback(ctx, call.ID, query, maxResults)
}

func (t *webSearch) searchTavily(ctx context.Context, query string, maxResults int, searchDepth string) (*ports.ToolResult, error) {
	// Build request
	reqBody := map[string]any{
		"api_key":        t.apiKey,
		"query":          query,
		"max_results":    maxResults,
		"search_depth":   searchDepth,
		"include_answer": true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// Make API request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Parse response
	var tavilyResp struct {
		Query   string `json:"query"`
		Answer  string `json:"answer"`
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &tavilyResp); err != nil {
		return nil, err
	}

	// Format results (no emojis for TUI compatibility)
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search: %s\n\n", tavilyResp.Query))

	if tavilyResp.Answer != "" {
		output.WriteString(fmt.Sprintf("Summary: %s\n\n", tavilyResp.Answer))
	}

	output.WriteString(fmt.Sprintf("%d Results:\n\n", len(tavilyResp.Results)))
	for i, result := range tavilyResp.Results {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		output.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
		output.WriteString(fmt.Sprintf("   %s\n\n", result.Content))
	}

	return &ports.ToolResult{
		Content: output.String(),
		Metadata: map[string]any{
			"query":         tavilyResp.Query,
			"answer":        tavilyResp.Answer,
			"results_count": len(tavilyResp.Results),
			"source":        "tavily",
		},
	}, nil
}

func (t *webSearch) searchFallback(ctx context.Context, callID string, query string, maxResults int) (*ports.ToolResult, error) {
	fallbackURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fallbackURL, nil)
	if err != nil {
		return &ports.ToolResult{CallID: callID, Content: fmt.Sprintf("Error creating fallback request: %v", err), Error: err}, nil
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return &ports.ToolResult{CallID: callID, Content: fmt.Sprintf("Error making fallback request: %v", err), Error: err}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ports.ToolResult{CallID: callID, Content: fmt.Sprintf("Error reading fallback response: %v", err), Error: err}, nil
	}

	results, err := parseDuckDuckGoResults(body, maxResults)
	if err != nil {
		return &ports.ToolResult{CallID: callID, Content: fmt.Sprintf("Error parsing fallback response: %v", err), Error: err}, nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search (fallback): %s\n\n", query))
	output.WriteString(fmt.Sprintf("%d Results:\n\n", len(results)))
	for i, result := range results {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		output.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
		if result.Content != "" {
			output.WriteString(fmt.Sprintf("   %s\n\n", result.Content))
		} else {
			output.WriteString("\n")
		}
	}

	return &ports.ToolResult{
		CallID:  callID,
		Content: output.String(),
		Metadata: map[string]any{
			"query":         query,
			"results_count": len(results),
			"source":        "duckduckgo",
		},
	}, nil
}

func parseDuckDuckGoResults(body []byte, maxResults int) ([]webSearchResult, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	results := make([]webSearchResult, 0, maxResults)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if len(results) >= maxResults {
			return
		}
		if node.Type == html.ElementNode && hasClass(node, "result") {
			link := findNodeByClass(node, "a", "result__a")
			if link != nil {
				results = append(results, webSearchResult{
					Title:   strings.TrimSpace(textContent(link)),
					URL:     attrValue(link, "href"),
					Content: strings.TrimSpace(textContent(findNodeByClass(node, "", "result__snippet"))),
				})
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
			if len(results) >= maxResults {
				return
			}
		}
	}
	walk(doc)

	return results, nil
}

func hasClass(node *html.Node, className string) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" {
			for _, class := range strings.Fields(attr.Val) {
				if class == className {
					return true
				}
			}
		}
	}
	return false
}

func attrValue(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func findNodeByClass(node *html.Node, tag string, class string) *html.Node {
	var match func(*html.Node) *html.Node
	match = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && (tag == "" || n.Data == tag) && hasClass(n, class) {
			return n
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if found := match(child); found != nil {
				return found
			}
		}
		return nil
	}
	return match(node)
}

func textContent(node *html.Node) string {
	if node == nil {
		return ""
	}
	var output strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			output.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return output.String()
}
