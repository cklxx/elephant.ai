package okr

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type okrRead struct {
	shared.BaseTool
	store *GoalStore
}

// NewOKRRead creates the okr_read tool.
func NewOKRRead(cfg OKRConfig) tools.ToolExecutor {
	return &okrRead{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "okr_read",
				Description: "Read structured OKR records (Objective/Key-Result objects) from OKR storage. Without goal_id: list OKR entries. With goal_id: return that OKR record. Not for repository topology/code search or general file reading.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"goal_id": {Type: "string", Description: "Optional goal ID to read. Omit to list all goals."},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "okr_read",
				Version:  "1.0.0",
				Category: "okr",
				Tags:     []string{"okr", "objective", "key_result", "goal"},
			},
		),
		store: NewGoalStore(cfg),
	}
}

func (t *okrRead) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	goalID := strings.TrimSpace(shared.StringArg(call.Arguments, "goal_id"))

	if goalID != "" {
		return t.readSingle(call.ID, goalID)
	}
	return t.listAll(call.ID)
}

func (t *okrRead) readSingle(callID, goalID string) (*ports.ToolResult, error) {
	raw, err := t.store.ReadGoalRaw(goalID)
	if err != nil {
		return &ports.ToolResult{CallID: callID, Error: fmt.Errorf("goal not found: %s", goalID)}, nil
	}
	return &ports.ToolResult{
		CallID:  callID,
		Content: string(raw),
		Metadata: map[string]any{
			"goal_id": goalID,
		},
	}, nil
}

func (t *okrRead) listAll(callID string) (*ports.ToolResult, error) {
	ids, err := t.store.ListGoals()
	if err != nil {
		return &ports.ToolResult{CallID: callID, Error: err}, nil
	}

	if len(ids) == 0 {
		return &ports.ToolResult{
			CallID:  callID,
			Content: "No goals found.",
			Metadata: map[string]any{
				"count": 0,
			},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d goal(s):\n\n", len(ids)))

	for _, id := range ids {
		goal, err := t.store.ReadGoal(id)
		if err != nil {
			sb.WriteString(fmt.Sprintf("- **%s** (error reading)\n", id))
			continue
		}

		sb.WriteString(fmt.Sprintf("- **%s** [%s]", id, goal.Meta.Status))

		// Add KR progress summary
		if len(goal.Meta.KeyResults) > 0 {
			var krSummaries []string
			for krID, kr := range goal.Meta.KeyResults {
				krSummaries = append(krSummaries, fmt.Sprintf("%s: %.1f%%", krID, kr.ProgressPct))
			}
			sb.WriteString(fmt.Sprintf(" — KRs: %s", strings.Join(krSummaries, ", ")))
		}

		if goal.Meta.ReviewCadence != "" {
			sb.WriteString(fmt.Sprintf(" (cadence: %s)", goal.Meta.ReviewCadence))
		}
		sb.WriteString("\n")
	}

	return &ports.ToolResult{
		CallID:  callID,
		Content: sb.String(),
		Metadata: map[string]any{
			"count":    len(ids),
			"goal_ids": ids,
		},
	}, nil
}

// Store returns the underlying GoalStore for reuse by other components.
func (t *okrRead) Store() *GoalStore {
	return t.store
}
