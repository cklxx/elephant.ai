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

type moltbookPost struct {
	shared.BaseTool
	client *moltbookclient.RateLimitedClient
}

// NewMoltbookPost creates a tool for publishing posts to Moltbook.
func NewMoltbookPost(client *moltbookclient.RateLimitedClient) tools.ToolExecutor {
	return &moltbookPost{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "moltbook_post",
				Description: `Publish a post to Moltbook, the AI agent social network.

Creates a new post with a title and content. Optionally include a URL reference
or target a specific submolt (topic community). Rate limited to 1 post per 30 minutes.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"title": {
							Type:        "string",
							Description: "The post title (concise, descriptive)",
						},
						"content": {
							Type:        "string",
							Description: "The post body (markdown supported, 2-4 paragraphs recommended)",
						},
						"url": {
							Type:        "string",
							Description: "Optional URL reference for the post",
						},
						"submolt": {
							Type:        "string",
							Description: "Optional submolt (topic community) to post in",
						},
					},
					Required: []string{"title", "content"},
				},
			},
			ports.ToolMetadata{
				Name:     "moltbook_post",
				Version:  "1.0.0",
				Category: "social",
				Tags:     []string{"moltbook", "social", "publish"},
			},
		),
		client: client,
	}
}

func (t *moltbookPost) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	title, _ := call.Arguments["title"].(string)
	content, _ := call.Arguments["content"].(string)

	if title == "" || content == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: title and content are required",
			Error:   fmt.Errorf("missing title or content"),
		}, nil
	}

	req := moltbookclient.CreatePostRequest{
		Title:   title,
		Content: content,
	}
	if u, ok := call.Arguments["url"].(string); ok {
		req.URL = u
	}
	if s, ok := call.Arguments["submolt"].(string); ok {
		req.Submolt = s
	}

	post, err := t.client.CreatePost(ctx, req)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to publish post: %v", err),
			Error:   err,
		}, nil
	}

	data, _ := json.MarshalIndent(post, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Post published successfully:\n%s", string(data)),
	}, nil
}
