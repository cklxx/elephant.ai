package ui

import (
	"context"
	"fmt"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"
	id "alex/internal/utils/id"
)

type attention struct {
	shared.BaseTool
}

// NewAttention constructs a tool to pin high-signal notes for later recall.
func NewAttention() tools.ToolExecutor {
	return &attention{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "attention",
				Description: "Mark high-signal, user-personalized information so it persists across context compression. Use sparingly for identity, preferences, and commitments.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"content": {
							Type:        "string",
							Description: "The important note to keep across turns. Keep concise and specific to the user.",
						},
						"tags": {
							Type:        "array",
							Description: "Optional tags describing the note (e.g., preference, identity, constraint).",
							Items:       &ports.Property{Type: "string"},
						},
					},
					Required: []string{"content"},
				},
			},
			ports.ToolMetadata{
				Name:     "attention",
				Version:  "0.1.0",
				Category: "memory",
				Tags:     []string{"important", "note", "session"},
			},
		),
	}
}

func (t *attention) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	content, errResult := shared.RequireStringArg(call.Arguments, call.ID, "content")
	if errResult != nil {
		return errResult, nil
	}

	tags := shared.StringSliceArg(call.Arguments, "tags")

	note := ports.ImportantNote{
		ID:        id.NewKSUID(),
		Content:   content,
		Source:    "attention",
		Tags:      tags,
		CreatedAt: time.Now(),
	}

	metadata := map[string]any{
		"important_notes": []ports.ImportantNote{note},
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  fmt.Sprintf("Pinned important note (%s)", note.ID),
		Metadata: metadata,
	}, nil
}
