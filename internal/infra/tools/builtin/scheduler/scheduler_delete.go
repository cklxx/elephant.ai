package scheduler

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type schedulerDelete struct {
	shared.BaseTool
}

// NewSchedulerDelete creates the scheduler_delete_job tool.
// This tool is marked as Dangerous and requires approval before execution.
func NewSchedulerDelete() tools.ToolExecutor {
	return &schedulerDelete{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "scheduler_delete_job",
				Description: "Delete a scheduled job by ID. This permanently removes the job and its cron schedule.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"job_id": {
							Type:        "string",
							Description: "The unique ID of the job to delete",
						},
					},
					Required: []string{"job_id"},
				},
			},
			ports.ToolMetadata{
				Name:      "scheduler_delete_job",
				Version:   "1.0.0",
				Category:  "scheduler",
				Tags:      []string{"scheduler", "cron", "automation"},
				Dangerous: true,
			},
		),
	}
}

func (t *schedulerDelete) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	svc := getService(ctx)
	if svc == nil {
		return shared.ToolError(call.ID, "scheduler not available")
	}

	jobID := strings.TrimSpace(shared.StringArg(call.Arguments, "job_id"))
	if jobID == "" {
		return shared.ToolError(call.ID, "job_id is required")
	}

	if err := svc.UnregisterTrigger(ctx, jobID); err != nil {
		return shared.ToolError(call.ID, "failed to delete job %q: %v", jobID, err)
	}

	content := fmt.Sprintf("Scheduled job deleted:\n- ID: %s\n- The job has been removed from the scheduler and job store.", jobID)

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"job_id":  jobID,
			"deleted": true,
		},
	}, nil
}
