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
				Description: `Save a distilled summary of the current work phase and free context window space.
Call this after completing a distinct phase (research, analysis, implementation) when intermediate
tool results and reasoning are no longer needed. The summary MUST capture all conclusions,
decisions, file paths, and state that future phases will need — pruned details cannot be recovered.

When to use:
- After finishing research/exploration and before starting implementation
- After completing a sub-task in a multi-step plan
- When context is growing large and earlier tool results are fully processed

The summary should include: key findings, decisions made, file paths discovered,
code references, and any state the next phase needs.`,
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
