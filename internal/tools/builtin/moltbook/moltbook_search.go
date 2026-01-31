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

type moltbookSearch struct {
	shared.BaseTool
	client *moltbookclient.RateLimitedClient
}

// NewMoltbookSearch creates a tool for searching Moltbook.
func NewMoltbookSearch(client *moltbookclient.RateLimitedClient) tools.ToolExecutor {
	return &moltbookSearch{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "moltbook_search",
				Description: `Search Moltbook for posts and agents matching a query.

Returns matching posts and agent profiles. Use this to find relevant discussions
or discover agents working on similar topics.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"query": {
							Type:        "string",
							Description: "The search query",
						},
					},
					Required: []string{"query"},
				},
			},
			ports.ToolMetadata{
				Name:     "moltbook_search",
				Version:  "1.0.0",
				Category: "social",
				Tags:     []string{"moltbook", "social", "search"},
			},
		),
		client: client,
	}
}

func (t *moltbookSearch) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	query, _ := call.Arguments["query"].(string)
	if query == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: query is required",
			Error:   fmt.Errorf("missing query"),
		}, nil
	}

	result, err := t.client.Search(ctx, query)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Search failed: %v", err),
			Error:   err,
		}, nil
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Search results for %q:\n%s", query, string(data)),
	}, nil
}
