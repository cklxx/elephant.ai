package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"alex/internal/agent/ports"
)

type webSearch struct {
	client *http.Client
	apiKey string
}

func NewWebSearch(apiKey string) ports.ToolExecutor {
	return newWebSearch(apiKey, nil)
}

func newWebSearch(apiKey string, client *http.Client) *webSearch {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
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
2. Set environment: export TAVILY_API_KEY="your-key"
   OR add to ~/.alex-config.json: "tavily_api_key": "your-key"`,
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
	if t.apiKey == "" {
		return &ports.ToolResult{
			CallID: call.ID,
			Content: "Web search not configured. Please set Tavily API key:\n\n" +
				"1. Get key from: https://app.tavily.com/\n" +
				"2. Set env: export TAVILY_API_KEY=\"your-key\"\n" +
				"   OR add to ~/.alex-config.json: \"tavily_api_key\": \"your-key\"",
		}, nil
	}

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
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error encoding request: %v", err),
			Error:   err,
		}, nil
	}

	// Make API request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewBuffer(jsonData))
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error creating request: %v", err),
			Error:   err,
		}, nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error making request: %v", err),
			Error:   err,
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error reading response: %v", err),
			Error:   err,
		}, nil
	}

	if resp.StatusCode != 200 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
			Error:   fmt.Errorf("API returned status %d", resp.StatusCode),
		}, nil
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
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error parsing response: %v", err),
			Error:   err,
		}, nil
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
		CallID:  call.ID,
		Content: output.String(),
		Metadata: map[string]any{
			"query":         tavilyResp.Query,
			"answer":        tavilyResp.Answer,
			"results_count": len(tavilyResp.Results),
		},
	}, nil
}
