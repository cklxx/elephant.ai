package orchestration

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/tools/builtin/shared"
)

type extMerge struct {
	shared.BaseTool
}

// NewExtMerge creates the ext_merge tool for merging external agent workspaces.
func NewExtMerge() *extMerge {
	return &extMerge{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "ext_merge",
				Description: `Merge an external agent's work branch back into the base branch.
Use after an external task completes to integrate its changes.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"task_id": {
							Type:        "string",
							Description: "The completed background task ID.",
						},
						"strategy": {
							Type:        "string",
							Description: `Merge strategy: "auto" (default), "squash", "rebase", or "review".`,
						},
					},
					Required: []string{"task_id"},
				},
			},
			ports.ToolMetadata{
				Name:     "ext_merge",
				Version:  "1.0.0",
				Category: "agent",
				Tags:     []string{"background", "orchestration", "external"},
			},
		),
	}
}

func (t *extMerge) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	for key := range call.Arguments {
		switch key {
		case "task_id", "strategy":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	taskID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "task_id")
	if errResult != nil {
		return errResult, nil
	}
	strategy := agent.MergeStrategyAuto
	if raw, ok := call.Arguments["strategy"]; ok {
		if str, ok := raw.(string); ok {
			strategy = agent.MergeStrategy(strings.TrimSpace(str))
		} else {
			return shared.ToolError(call.ID, "strategy must be a string")
		}
	}
	if strategy == "" {
		strategy = agent.MergeStrategyAuto
	}

	dispatcher := agent.GetBackgroundDispatcher(ctx)
	if dispatcher == nil {
		return shared.ToolError(call.ID, "background task dispatch is not available in this context")
	}
	merger, ok := dispatcher.(agent.ExternalWorkspaceMerger)
	if !ok {
		return shared.ToolError(call.ID, "external workspace merger is not available in this context")
	}

	result, err := merger.MergeExternalWorkspace(ctx, taskID, strategy)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	content := formatMergeResult(result)
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
	}, nil
}

func formatMergeResult(result *agent.MergeResult) string {
	if result == nil {
		return "No merge result available."
	}
	if !result.Success {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Merge failed for task %q\n", result.TaskID))
		if len(result.Conflicts) > 0 {
			sb.WriteString("Conflicts:\n")
			for _, path := range result.Conflicts {
				sb.WriteString(fmt.Sprintf("  - %s\n", path))
			}
		}
		if result.DiffSummary != "" {
			sb.WriteString(fmt.Sprintf("Diff stats: %s\n", result.DiffSummary))
		}
		return sb.String()
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Merge successful for task %q\n", result.TaskID))
	sb.WriteString(fmt.Sprintf("Strategy: %s\n", result.Strategy))
	if result.CommitHash != "" {
		sb.WriteString(fmt.Sprintf("Commit: %s\n", result.CommitHash))
	}
	if len(result.FilesChanged) > 0 {
		sb.WriteString("Files changed:\n")
		for _, path := range result.FilesChanged {
			sb.WriteString(fmt.Sprintf("  - %s\n", path))
		}
	}
	if result.DiffSummary != "" {
		sb.WriteString(fmt.Sprintf("Diff stats: %s\n", result.DiffSummary))
	}
	return sb.String()
}
