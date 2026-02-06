package okr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type okrWrite struct {
	shared.BaseTool
	store *GoalStore
}

// NewOKRWrite creates the okr_write tool.
func NewOKRWrite(cfg OKRConfig) tools.ToolExecutor {
	return &okrWrite{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "okr_write",
				Description: "Create or update an OKR goal file. Content must be a valid markdown file with YAML frontmatter containing goal metadata and key results. The 'updated' field is automatically set to today's date.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"goal_id": {Type: "string", Description: "Goal identifier (used as filename, e.g. 'q1-2026-revenue')"},
						"content": {Type: "string", Description: "Full goal file content with YAML frontmatter and markdown body"},
					},
					Required: []string{"goal_id", "content"},
				},
			},
			ports.ToolMetadata{
				Name:     "okr_write",
				Version:  "1.0.0",
				Category: "okr",
			},
		),
		store: NewGoalStore(cfg),
	}
}

func (t *okrWrite) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	goalID := strings.TrimSpace(shared.StringArg(call.Arguments, "goal_id"))
	if goalID == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing required parameter 'goal_id'")}, nil
	}

	content := shared.StringArg(call.Arguments, "content")
	if content == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing required parameter 'content'")}, nil
	}

	// Validate that content parses as a valid GoalFile
	goal, err := ParseGoalFile([]byte(content))
	if err != nil {
		return &ports.ToolResult{
			CallID: call.ID,
			Error:  fmt.Errorf("invalid goal content: %w", err),
		}, nil
	}

	// Auto-set updated date
	goal.Meta.Updated = time.Now().Format("2006-01-02")

	// Write back with auto-updated date
	if err := t.store.WriteGoal(goalID, goal); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	action := "updated"
	if !t.store.GoalExists(goalID) {
		action = "created"
	}
	// Since we already wrote, the file exists — check if it was new by looking
	// at whether it existed before (the WriteGoal above already created it).
	// Simplify: always say "saved".
	action = "saved"

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Goal '%s' %s successfully (updated: %s).", goalID, action, goal.Meta.Updated),
		Metadata: map[string]any{
			"goal_id":  goalID,
			"status":   goal.Meta.Status,
			"updated":  goal.Meta.Updated,
			"kr_count": len(goal.Meta.KeyResults),
		},
	}, nil
}
