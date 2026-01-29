package orchestration

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
)

type bgStatus struct{}

// NewBGStatus creates the bg_status tool for querying background task status.
func NewBGStatus() *bgStatus {
	return &bgStatus{}
}

func (t *bgStatus) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "bg_status",
		Description: `Query the status of background tasks. Returns a summary of each task including its current state (pending, running, completed, failed, cancelled).`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"task_ids": {
					Type:        "array",
					Description: "Optional list of task IDs to query. Omit to query all background tasks.",
					Items:       &ports.Property{Type: "string"},
				},
			},
		},
	}
}

func (t *bgStatus) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "bg_status",
		Version:  "1.0.0",
		Category: "agent",
		Tags:     []string{"background", "orchestration", "async"},
	}
}

func (t *bgStatus) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	for key := range call.Arguments {
		switch key {
		case "task_ids":
		default:
			err := fmt.Errorf("unsupported parameter: %s", key)
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	ids, err := parseStringList(call.Arguments, "task_ids")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	dispatcher := agent.GetBackgroundDispatcher(ctx)
	if dispatcher == nil {
		err := fmt.Errorf("background task dispatch is not available in this context")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	summaries := dispatcher.Status(ids)
	if len(summaries) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No background tasks found.",
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Background Tasks (%d):\n\n", len(summaries)))
	for _, s := range summaries {
		sb.WriteString(fmt.Sprintf("  [%s] id=%q description=%q status=%s",
			s.AgentType, s.ID, s.Description, string(s.Status)))
		if s.Error != "" {
			sb.WriteString(fmt.Sprintf(" error=%q", s.Error))
		}
		sb.WriteString("\n")
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: sb.String(),
	}, nil
}
