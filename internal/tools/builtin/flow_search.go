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
	"alex/internal/httpclient"
)

type flowSearch struct {
	client *http.Client
	apiKey string
}

func NewFlowSearch(apiKey string) ports.ToolExecutor {
	return newFlowSearch(apiKey, nil)
}

func newFlowSearch(apiKey string, client *http.Client) *flowSearch {
	if client == nil {
		client = httpclient.New(20*time.Second, nil)
	}
	return &flowSearch{client: client, apiKey: apiKey}
}

func (t *flowSearch) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "flow_search",
		Version:  "1.0.0",
		Category: "web",
		Tags:     []string{"flow", "search", "writing"},
	}
}

func (t *flowSearch) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "flow_search",
		Description: `Search the web for writing flow mode and return concise bullets.

This tool is tuned for the flow-writing experience and limits output to 3-5 brief highlights per query.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"query": {
					Type:        "string",
					Description: "Search keywords to explore",
				},
				"reason": {
					Type:        "string",
					Description: "Context on why this search matters",
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum number of results (1-5, default 3)",
				},
			},
			Required: []string{"query"},
		},
	}
}

func (t *flowSearch) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.apiKey == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "flow_search is not configured. Set TAVILY_API_KEY to enable web search.",
		}, nil
	}

	query, _ := call.Arguments["query"].(string)
	if strings.TrimSpace(query) == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "query is required for flow_search",
			Error:   fmt.Errorf("missing query"),
		}, nil
	}

	maxResults := 3
	if raw, ok := call.Arguments["max_results"]; ok {
		switch v := raw.(type) {
		case float64:
			maxResults = int(v)
		case float32:
			maxResults = int(v)
		case int:
			maxResults = v
		case int32:
			maxResults = int(v)
		case int64:
			maxResults = int(v)
		case json.Number:
			if n, err := v.Int64(); err == nil {
				maxResults = int(n)
			}
		}
	}
	if maxResults < 1 {
		maxResults = 1
	}
	if maxResults > 5 {
		maxResults = 5
	}

	reqBody := map[string]any{
		"api_key":        t.apiKey,
		"query":          query,
		"max_results":    maxResults,
		"search_depth":   "advanced",
		"include_answer": true,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("encode request: %v", err),
			Error:   err,
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(data))
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("build request: %v", err),
			Error:   err,
		}, nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("search request failed: %v", err),
			Error:   err,
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("read response: %v", err),
			Error:   err,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("api error (%d): %s", resp.StatusCode, string(body)),
			Error:   fmt.Errorf("tavily status %d", resp.StatusCode),
		}, nil
	}

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
			Content: fmt.Sprintf("parse response: %v", err),
			Error:   err,
		}, nil
	}

	var b strings.Builder
	reason, _ := call.Arguments["reason"].(string)
	b.WriteString("Flow search\n")
	b.WriteString(fmt.Sprintf("Query: %s\n", tavilyResp.Query))
	if strings.TrimSpace(reason) != "" {
		b.WriteString(fmt.Sprintf("Context: %s\n", strings.TrimSpace(reason)))
	}
	if strings.TrimSpace(tavilyResp.Answer) != "" {
		b.WriteString(fmt.Sprintf("\nQuick take: %s\n", tavilyResp.Answer))
	}
	if len(tavilyResp.Results) > 0 {
		b.WriteString("\nFindings:\n")
		for i, r := range tavilyResp.Results {
			if i >= maxResults {
				break
			}
			snippet := strings.TrimSpace(r.Content)
			if len(snippet) > 240 {
				snippet = snippet[:240]
			}
			b.WriteString(fmt.Sprintf("%d) %s\n   %s\n   %s\n", i+1, r.Title, snippet, r.URL))
		}
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: b.String(),
		Metadata: map[string]any{
			"query":         tavilyResp.Query,
			"results_count": len(tavilyResp.Results),
		},
	}, nil
}
