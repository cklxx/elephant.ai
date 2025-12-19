package builtin

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
)

type uiPlan struct{}

func NewPlan() ports.ToolExecutor {
	return &uiPlan{}
}

func (t *uiPlan) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "plan",
		Version:  "1.0.0",
		Category: "ui",
		Tags:     []string{"ui", "orchestration"},
	}
}

func (t *uiPlan) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "plan",
		Description: `UI tool: emit Level 1 goal and (optionally) attach a hidden internal plan for the orchestrator.

Rules:
- Must be called before any non-plan/clearify tool call.
- When complexity="simple", overall_goal_ui must be a single line.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"run_id": {
					Type:        "string",
					Description: "Run identifier (must match the current run_id provided by the system).",
				},
				"overall_goal_ui": {
					Type:        "string",
					Description: "User-facing goal UI text. For complexity=simple it must be single-line.",
				},
				"complexity": {
					Type:        "string",
					Description: "simple or complex",
					Enum:        []any{"simple", "complex"},
				},
				"internal_plan": {
					Type:        "object",
					Description: "Hidden internal plan object for orchestrator storage (must not be rendered).",
				},
			},
			Required: []string{"run_id", "overall_goal_ui", "complexity"},
		},
	}
}

func (t *uiPlan) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	runID, ok := call.Arguments["run_id"].(string)
	if !ok {
		err := fmt.Errorf("missing 'run_id'")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		err := fmt.Errorf("run_id cannot be empty")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	expected := strings.TrimSpace(id.TaskIDFromContext(ctx))
	if expected != "" && runID != expected {
		err := fmt.Errorf("run_id must equal current run_id %q", expected)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	complexityRaw, ok := call.Arguments["complexity"].(string)
	if !ok {
		err := fmt.Errorf("missing 'complexity'")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	complexity := strings.ToLower(strings.TrimSpace(complexityRaw))
	if complexity != "simple" && complexity != "complex" {
		err := fmt.Errorf("complexity must be \"simple\" or \"complex\"")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	goal, ok := call.Arguments["overall_goal_ui"].(string)
	if !ok {
		err := fmt.Errorf("missing 'overall_goal_ui'")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	goal = strings.TrimSpace(goal)
	if goal == "" {
		err := fmt.Errorf("overall_goal_ui cannot be empty")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	if complexity == "simple" && (strings.Contains(goal, "\n") || strings.Contains(goal, "\r")) {
		err := fmt.Errorf("overall_goal_ui must be single-line when complexity=\"simple\"")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	// Reject unexpected parameters to keep the protocol strict.
	for key := range call.Arguments {
		switch key {
		case "run_id", "overall_goal_ui", "complexity", "internal_plan":
		default:
			err := fmt.Errorf("unsupported parameter: %s", key)
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	metadata := map[string]any{
		"run_id":          runID,
		"overall_goal_ui": goal,
		"complexity":      complexity,
	}
	if internalPlan, exists := call.Arguments["internal_plan"]; exists {
		metadata["internal_plan"] = internalPlan
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  goal,
		Metadata: metadata,
	}, nil
}
