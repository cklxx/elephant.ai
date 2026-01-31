package moltbook

import (
	"context"
	"fmt"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	moltbookclient "alex/internal/moltbook"
	"alex/internal/tools/builtin/shared"
)

type moltbookVote struct {
	shared.BaseTool
	client *moltbookclient.RateLimitedClient
}

// NewMoltbookVote creates a tool for voting on Moltbook posts.
func NewMoltbookVote(client *moltbookclient.RateLimitedClient) tools.ToolExecutor {
	return &moltbookVote{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "moltbook_vote",
				Description: `Upvote or downvote a Moltbook post.

Express appreciation for valuable posts by upvoting, or downvote low-quality content.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"post_id": {
							Type:        "string",
							Description: "The ID of the post to vote on",
						},
						"direction": {
							Type:        "string",
							Description: "Vote direction",
							Enum:        []any{"up", "down"},
						},
					},
					Required: []string{"post_id", "direction"},
				},
			},
			ports.ToolMetadata{
				Name:     "moltbook_vote",
				Version:  "1.0.0",
				Category: "social",
				Tags:     []string{"moltbook", "social", "vote"},
			},
		),
		client: client,
	}
}

func (t *moltbookVote) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	postID, _ := call.Arguments["post_id"].(string)
	direction, _ := call.Arguments["direction"].(string)

	if postID == "" || direction == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: post_id and direction are required",
			Error:   fmt.Errorf("missing post_id or direction"),
		}, nil
	}

	var err error
	switch direction {
	case "up":
		err = t.client.Upvote(ctx, postID)
	case "down":
		err = t.client.Downvote(ctx, postID)
	default:
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error: invalid direction %q, must be 'up' or 'down'", direction),
			Error:   fmt.Errorf("invalid direction: %s", direction),
		}, nil
	}

	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to vote: %v", err),
			Error:   err,
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Successfully %svoted post %s", direction, postID),
	}, nil
}
