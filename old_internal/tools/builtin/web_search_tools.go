package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// WebSearchTool implements web search functionality using Tavily API
type WebSearchTool struct {
	apiKey string
	client *http.Client
}

// TavilyRequest represents a request to Tavily API
type TavilyRequest struct {
	APIKey            string   `json:"api_key"`
	Query             string   `json:"query"`
	SearchDepth       string   `json:"search_depth,omitempty"`        // basic or advanced
	IncludeAnswer     bool     `json:"include_answer,omitempty"`      // Include a short answer to the query
	IncludeImages     bool     `json:"include_images,omitempty"`      // Include a list of query related images
	IncludeRawContent bool     `json:"include_raw_content,omitempty"` // Include raw content of search results
	MaxResults        int      `json:"max_results,omitempty"`         // Maximum number of search results
	IncludeDomains    []string `json:"include_domains,omitempty"`     // List of domains to include
	ExcludeDomains    []string `json:"exclude_domains,omitempty"`     // List of domains to exclude
	UseGPT4o          bool     `json:"use_gpt4o,omitempty"`           // Use GPT-4o for answer generation
}

// TavilyResponse represents the response from Tavily API
type TavilyResponse struct {
	Answer            string         `json:"answer,omitempty"`
	Query             string         `json:"query"`
	ResponseTime      float64        `json:"response_time"`
	Images            []TavilyImage  `json:"images,omitempty"`
	Results           []TavilyResult `json:"results"`
	FollowUpQuestions []string       `json:"follow_up_questions,omitempty"`
}

// TavilyResult represents a single search result
type TavilyResult struct {
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	Content    string  `json:"content"`
	RawContent string  `json:"raw_content,omitempty"`
	Score      float64 `json:"score"`
	Published  string  `json:"published_date,omitempty"`
}

// TavilyImage represents an image result
type TavilyImage struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// CreateWebSearchTool creates a new web search tool
func CreateWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Description() string {
	return "Search the web for current information using Tavily API. Returns relevant search results with summaries and URLs."
}

func (t *WebSearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "The search query to execute",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of search results to return",
				"default":     5,
				"minimum":     1,
				"maximum":     10,
			},
			"search_depth": map[string]interface{}{
				"type":        "string",
				"description": "Search depth: basic or advanced",
				"enum":        []string{"basic", "advanced"},
				"default":     "basic",
			},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddStringField("query", "The search query to execute").
		AddOptionalIntField("max_results", "Maximum number of search results to return", 1, 10).
		AddCustomValidator("search_depth", "Search depth (basic or advanced)", false, func(value interface{}) error {
			if value == nil {
				return nil // Optional field
			}
			depth, ok := value.(string)
			if !ok {
				return fmt.Errorf("search_depth must be a string")
			}
			if depth != "basic" && depth != "advanced" {
				return fmt.Errorf("search_depth must be 'basic' or 'advanced'")
			}
			return nil
		})

	return validator.Validate(args)
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// Check for API key in tool instance, environment, or configuration
	apiKey := t.apiKey
	if apiKey == "" {
		// Check environment variable as fallback
		if envKey := os.Getenv("TAVILY_API_KEY"); envKey != "" {
			apiKey = envKey
		}
	}

	if apiKey == "" {
		return &ToolResult{
			Content: "Web search is not configured. Please set the Tavily API key:\n\n" +
				"Option 1: Environment variable: export TAVILY_API_KEY=\"your-api-key\"\n" +
				"Option 2: Configuration file: Add \"tavilyApiKey\": \"your-api-key\" to ~/.alex-config.json\n\n" +
				"Get your API key from: https://app.tavily.com/",
			Data: map[string]interface{}{
				"configured": false,
				"message":    "API key required",
				"content": "Web search is not configured. Please set the Tavily API key:\n\n" +
					"Option 1: Environment variable: export TAVILY_API_KEY=\"your-api-key\"\n" +
					"Option 2: Configuration file: Add \"tavilyApiKey\": \"your-api-key\" to ~/.alex-config.json\n\n" +
					"Get your API key from: https://app.tavily.com/",
			},
		}, nil
	}

	query := args["query"].(string)

	// Build request
	request := TavilyRequest{
		APIKey: apiKey,
		Query:  query,
	}

	// Set optional parameters
	if maxResults, ok := args["max_results"]; ok {
		if mr, ok := maxResults.(float64); ok {
			request.MaxResults = int(mr)
		}
	} else {
		request.MaxResults = 5
	}

	if searchDepth, ok := args["search_depth"]; ok {
		if sd, ok := searchDepth.(string); ok {
			request.SearchDepth = sd
		}
	} else {
		request.SearchDepth = "basic"
	}

	if includeAnswer, ok := args["include_answer"]; ok {
		if ia, ok := includeAnswer.(bool); ok {
			request.IncludeAnswer = ia
		}
	} else {
		request.IncludeAnswer = true
	}

	if includeImages, ok := args["include_images"]; ok {
		if ii, ok := includeImages.(bool); ok {
			request.IncludeImages = ii
		}
	}

	if includeRawContent, ok := args["include_raw_content"]; ok {
		if irc, ok := includeRawContent.(bool); ok {
			request.IncludeRawContent = irc
		}
	}

	if includeDomains, ok := args["include_domains"]; ok {
		if domains, ok := includeDomains.([]interface{}); ok {
			for _, domain := range domains {
				if domainStr, ok := domain.(string); ok {
					request.IncludeDomains = append(request.IncludeDomains, domainStr)
				}
			}
		}
	}

	if excludeDomains, ok := args["exclude_domains"]; ok {
		if domains, ok := excludeDomains.([]interface{}); ok {
			for _, domain := range domains {
				if domainStr, ok := domain.(string); ok {
					request.ExcludeDomains = append(request.ExcludeDomains, domainStr)
				}
			}
		}
	}

	// Make API request
	response, err := t.makeRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to search web: %w", err)
	}

	// Format results
	content := t.formatResults(response)

	return &ToolResult{
		Content: content,
		Data: map[string]interface{}{
			"query":               response.Query,
			"answer":              response.Answer,
			"results_count":       len(response.Results),
			"results":             response.Results,
			"images":              response.Images,
			"response_time":       response.ResponseTime,
			"follow_up_questions": response.FollowUpQuestions,
			"content":             content,
		},
	}, nil
}

// SetAPIKey sets the Tavily API key
func (t *WebSearchTool) SetAPIKey(apiKey string) {
	t.apiKey = apiKey
}

func (t *WebSearchTool) makeRequest(ctx context.Context, request TavilyRequest) (*TavilyResponse, error) {
	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Parse response
	var tavilyResp TavilyResponse
	if err := json.Unmarshal(responseBody, &tavilyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &tavilyResp, nil
}

func (t *WebSearchTool) formatResults(response *TavilyResponse) string {
	var content bytes.Buffer

	content.WriteString(fmt.Sprintf("# Web Search Results for: %s\n\n", response.Query))

	// Add answer if available
	if response.Answer != "" {
		content.WriteString(fmt.Sprintf("## Summary\n%s\n\n", response.Answer))
	}

	// Add search results
	content.WriteString("## Search Results\n\n")
	for i, result := range response.Results {
		content.WriteString(fmt.Sprintf("### %d. %s\n", i+1, result.Title))
		content.WriteString(fmt.Sprintf("**URL:** %s\n", result.URL))
		if result.Published != "" {
			content.WriteString(fmt.Sprintf("**Published:** %s\n", result.Published))
		}
		content.WriteString(fmt.Sprintf("**Score:** %.2f\n\n", result.Score))
		content.WriteString(fmt.Sprintf("%s\n\n", result.Content))

		if result.RawContent != "" && len(result.RawContent) > len(result.Content) {
			content.WriteString("**Full Content:**\n")
			content.WriteString(fmt.Sprintf("%s\n\n", result.RawContent))
		}

		content.WriteString("---\n\n")
	}

	// Add images if available
	if len(response.Images) > 0 {
		content.WriteString("## Related Images\n\n")
		for i, image := range response.Images {
			content.WriteString(fmt.Sprintf("%d. %s", i+1, image.URL))
			if image.Description != "" {
				content.WriteString(fmt.Sprintf(" - %s", image.Description))
			}
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	// Add follow-up questions if available
	if len(response.FollowUpQuestions) > 0 {
		content.WriteString("## Follow-up Questions\n\n")
		for i, question := range response.FollowUpQuestions {
			content.WriteString(fmt.Sprintf("%d. %s\n", i+1, question))
		}
		content.WriteString("\n")
	}

	content.WriteString(fmt.Sprintf("*Search completed in %.2f seconds*\n", response.ResponseTime))

	return content.String()
}

// NewsSearchTool implements news search functionality using Tavily API
type NewsSearchTool struct {
	*WebSearchTool
}

// CreateNewsSearchTool creates a new news search tool
func CreateNewsSearchTool() *NewsSearchTool {
	return &NewsSearchTool{
		WebSearchTool: CreateWebSearchTool(),
	}
}

func (t *NewsSearchTool) Name() string {
	return "news_search"
}

func (t *NewsSearchTool) Description() string {
	return "Search for recent news articles and current events using Tavily API. Focuses on news sources and recent content."
}

func (t *NewsSearchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// Add news-specific domains if not already specified
	if _, ok := args["include_domains"]; !ok {
		// Focus on major news sources
		args["include_domains"] = []interface{}{
			"reuters.com", "ap.org", "bbc.com", "cnn.com", "npr.org",
			"wsj.com", "nytimes.com", "guardian.co.uk", "bloomberg.com",
			"techcrunch.com", "arstechnica.com", "wired.com",
		}
	}

	// Set advanced search for news to get more comprehensive results
	if _, ok := args["search_depth"]; !ok {
		args["search_depth"] = "advanced"
	}

	// Call the parent web search functionality
	return t.WebSearchTool.Execute(ctx, args)
}

// AcademicSearchTool implements academic search functionality using Tavily API
type AcademicSearchTool struct {
	*WebSearchTool
}

// CreateAcademicSearchTool creates a new academic search tool
func CreateAcademicSearchTool() *AcademicSearchTool {
	return &AcademicSearchTool{
		WebSearchTool: CreateWebSearchTool(),
	}
}

func (t *AcademicSearchTool) Name() string {
	return "academic_search"
}

func (t *AcademicSearchTool) Description() string {
	return "Search for academic papers, research, and scholarly content using Tavily API. Focuses on academic and research sources."
}

func (t *AcademicSearchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// Add academic-specific domains if not already specified
	if _, ok := args["include_domains"]; !ok {
		// Focus on academic and research sources
		args["include_domains"] = []interface{}{
			"arxiv.org", "scholar.google.com", "pubmed.ncbi.nlm.nih.gov",
			"jstor.org", "springer.com", "ieee.org", "acm.org",
			"sciencedirect.com", "nature.com", "science.org",
			"researchgate.net", "semanticscholar.org",
		}
	}

	// Use advanced search for academic content
	args["search_depth"] = "advanced"

	// Include raw content for academic papers
	if _, ok := args["include_raw_content"]; !ok {
		args["include_raw_content"] = true
	}

	// Call the parent web search functionality
	return t.WebSearchTool.Execute(ctx, args)
}
