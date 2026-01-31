package moltbook

import (
	"context"
	"encoding/json"
	"fmt"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	moltbookclient "alex/internal/moltbook"
	"alex/internal/tools/builtin/shared"
)

type moltbookFeed struct {
	shared.BaseTool
	client *moltbookclient.RateLimitedClient
}

// NewMoltbookFeed creates a tool for reading the Moltbook feed.
func NewMoltbookFeed(client *moltbookclient.RateLimitedClient) tools.ToolExecutor {
	return &moltbookFeed{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "moltbook_feed",
				Description: `Read the Moltbook feed to see recent posts from AI agents.

Returns a list of recent posts with titles, authors, and engagement metrics.
Use this to understand what the community is discussing before posting or commenting.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"page": {
							Type:        "integer",
							Description: "Page number (default 1)",
						},
						"max_results": {
							Type:        "integer",
							Description: "Maximum number of results to return (1-20, default 10)",
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "moltbook_feed",
				Version:  "1.0.0",
				Category: "social",
				Tags:     []string{"moltbook", "social", "feed", "read"},
			},
		),
		client: client,
	}
}

func (t *moltbookFeed) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	page := 1
	if p, ok := call.Arguments["page"].(float64); ok && p >= 1 {
		page = int(p)
	}

	maxResults := 10
	if mr, ok := call.Arguments["max_results"].(float64); ok {
		maxResults = int(mr)
		if maxResults < 1 {
			maxResults = 1
		}
		if maxResults > 20 {
			maxResults = 20
		}
	}

	posts, err := t.client.GetFeed(ctx, page)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to fetch feed: %v", err),
			Error:   err,
		}, nil
	}

	if len(posts) > maxResults {
		posts = posts[:maxResults]
	}

	data, _ := json.MarshalIndent(posts, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Moltbook feed (page %d, %d posts):\n%s", page, len(posts), string(data)),
	}, nil
}
