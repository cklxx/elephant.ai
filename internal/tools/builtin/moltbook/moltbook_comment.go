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

type moltbookComment struct {
	shared.BaseTool
	client *moltbookclient.RateLimitedClient
}

// NewMoltbookComment creates a tool for commenting on Moltbook posts.
func NewMoltbookComment(client *moltbookclient.RateLimitedClient) tools.ToolExecutor {
	return &moltbookComment{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "moltbook_comment",
				Description: `Leave a comment on a Moltbook post.

Add a thoughtful comment to engage with the AI agent community.
Rate limited to 1 comment per 20 seconds.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"post_id": {
							Type:        "string",
							Description: "The ID of the post to comment on",
						},
						"content": {
							Type:        "string",
							Description: "The comment text",
						},
					},
					Required: []string{"post_id", "content"},
				},
			},
			ports.ToolMetadata{
				Name:     "moltbook_comment",
				Version:  "1.0.0",
				Category: "social",
				Tags:     []string{"moltbook", "social", "comment"},
			},
		),
		client: client,
	}
}

func (t *moltbookComment) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	postID, _ := call.Arguments["post_id"].(string)
	content, _ := call.Arguments["content"].(string)

	if postID == "" || content == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: post_id and content are required",
			Error:   fmt.Errorf("missing post_id or content"),
		}, nil
	}

	comment, err := t.client.CreateComment(ctx, postID, moltbookclient.CreateCommentRequest{
		Content: content,
	})
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to create comment: %v", err),
			Error:   err,
		}, nil
	}

	data, _ := json.MarshalIndent(comment, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Comment posted successfully:\n%s", string(data)),
	}, nil
}
