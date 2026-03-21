package ui

import (
	"context"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type uiContextCheckpoint struct {
	shared.BaseTool
}

// NewContextCheckpoint creates the context_checkpoint tool that lets the LLM
// proactively shed intermediate turn history between work phases.
func NewContextCheckpoint() tools.ToolExecutor {
	return &uiContextCheckpoint{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "context_checkpoint",
				Description: `When context grows large after completing a distinct work phase → use context_checkpoint to distill conclusions and free context window space.

Use after:
- Finishing research/exploration, before starting implementation
- Completing a sub-task in a multi-step plan
- Processing all earlier tool results that are no longer needed

The summary MUST capture all conclusions, decisions, file paths, code references, and state that future phases need — pruned details cannot be recovered. Do not use mid-phase when intermediate results are still needed.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"summary": {
							Type:        "string",
							Description: "Distilled conclusions from the completed phase. Must be comprehensive — pruned details cannot be recovered.",
						},
						"phase_label": {
							Type:        "string",
							Description: "Short label for the completed phase (e.g. 'research', 'implementation', 'analysis').",
						},
					},
					Required: []string{"summary"},
				},
			},
			ports.ToolMetadata{
				Name:     "context_checkpoint",
				Version:  "1.0.0",
				Category: "ui",
				Tags:     []string{"ui", "context", "checkpoint", "pruning", "phase"},
			},
		),
	}
}

const minSummaryLength = 50

func (t *uiContextCheckpoint) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	summary, errResult := shared.RequireStringArg(call.Arguments, call.ID, "summary")
	if errResult != nil {
		return errResult, nil
	}

	if len([]rune(strings.TrimSpace(summary))) < minSummaryLength {
		return shared.ToolError(call.ID, "summary must be at least %d characters to ensure meaningful context is preserved", minSummaryLength)
	}

	phaseLabel := shared.StringArg(call.Arguments, "phase_label")
	if phaseLabel == "" {
		phaseLabel = "phase"
	}

	// Reject unexpected parameters.
	for key := range call.Arguments {
		switch key {
		case "summary", "phase_label":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	metadata := map[string]any{
		"phase":  phaseLabel,
		"action": "checkpoint",
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  "Context checkpoint accepted. Phase: " + phaseLabel,
		Metadata: metadata,
	}, nil
}
