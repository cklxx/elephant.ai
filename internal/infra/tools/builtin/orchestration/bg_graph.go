package orchestration

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/tools/builtin/shared"
)

type bgGraph struct {
	shared.BaseTool
}

// NewBGGraph creates the bg_graph tool for dependency graph inspection.
func NewBGGraph() *bgGraph {
	return &bgGraph{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "bg_graph",
				Description: `Render the dependency graph of background tasks with status, agent type, and execution controls.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"task_ids": {
							Type:        "array",
							Description: "Optional list of task IDs to include. Omit to include all tracked background tasks.",
							Items:       &ports.Property{Type: "string"},
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "bg_graph",
				Version:  "1.0.0",
				Category: "agent",
				Tags:     []string{"background", "orchestration", "graph", "dag"},
			},
		),
	}
}

func (t *bgGraph) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	for key := range call.Arguments {
		switch key {
		case "task_ids":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	ids, err := parseStringList(call.Arguments, "task_ids")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	dispatcher := agent.GetBackgroundDispatcher(ctx)
	if dispatcher == nil {
		return shared.ToolError(call.ID, "background task dispatch is not available in this context")
	}

	summaries := dispatcher.Status(ids)
	if len(summaries) == 0 {
		return &ports.ToolResult{CallID: call.ID, Content: "No background tasks found."}, nil
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].ID < summaries[j].ID
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Background Dependency Graph (%d tasks)\n\n", len(summaries)))
	for _, s := range summaries {
		mode := s.ExecutionMode
		if mode == "" {
			mode = "execute"
		}
		autonomy := s.AutonomyLevel
		if autonomy == "" {
			autonomy = "controlled"
		}
		sb.WriteString(fmt.Sprintf("- %s [%s] status=%s mode=%s autonomy=%s\n", s.ID, s.AgentType, s.Status, mode, autonomy))
		if len(s.DependsOn) == 0 {
			sb.WriteString("  depends_on: (none)\n")
		} else {
			sb.WriteString(fmt.Sprintf("  depends_on: %s\n", strings.Join(s.DependsOn, ", ")))
		}
		if s.Description != "" {
			sb.WriteString(fmt.Sprintf("  desc: %s\n", s.Description))
		}
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: sb.String(),
	}, nil
}
