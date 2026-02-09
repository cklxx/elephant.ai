package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/delivery/schedulerapi"
	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type schedulerList struct {
	shared.BaseTool
}

// NewSchedulerList creates the scheduler_list_jobs tool.
func NewSchedulerList() tools.ToolExecutor {
	return &schedulerList{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "scheduler_list_jobs",
				Description: "List registered recurring scheduler jobs/automations with status, cadence, and next run time. Use this for pre-mutation inventory/audit of scheduler jobs. Do not use for artifacts or calendar event queries.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"status": {
							Type:        "string",
							Description: "Filter by job status: pending, active, paused, or completed. Omit to list all jobs.",
							Enum:        []any{"pending", "active", "paused", "completed"},
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:        "scheduler_list_jobs",
				Version:     "1.0.0",
				Category:    "scheduler",
				Tags:        []string{"scheduler", "cron", "automation"},
				SafetyLevel: ports.SafetyLevelReadOnly,
			},
		),
	}
}

var validStatuses = map[string]bool{
	"pending":   true,
	"active":    true,
	"paused":    true,
	"completed": true,
}

func (t *schedulerList) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	svc := getService(ctx)
	if svc == nil {
		return shared.ToolError(call.ID, "scheduler not available")
	}

	statusFilter := strings.TrimSpace(shared.StringArg(call.Arguments, "status"))

	jobs, err := svc.ListJobs(ctx)
	if err != nil {
		return shared.ToolError(call.ID, "failed to list jobs: %v", err)
	}

	// Apply status filter if provided.
	if statusFilter != "" {
		if !validStatuses[statusFilter] {
			return shared.ToolError(call.ID, "invalid status filter %q: must be pending, active, paused, or completed", statusFilter)
		}
		filtered := make([]schedulerapi.Job, 0, len(jobs))
		for _, job := range jobs {
			if job.Status == statusFilter {
				filtered = append(filtered, job)
			}
		}
		jobs = filtered
	}

	if len(jobs) == 0 {
		msg := "No scheduled jobs found."
		if statusFilter != "" {
			msg = fmt.Sprintf("No scheduled jobs with status %q found.", statusFilter)
		}
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: msg,
			Metadata: map[string]any{
				"count": 0,
			},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Scheduled jobs (%d):\n\n", len(jobs)))
	sb.WriteString(fmt.Sprintf("%-30s %-20s %-10s %-25s %s\n", "ID", "Schedule", "Status", "Next Run", "Task"))
	sb.WriteString(strings.Repeat("-", 120) + "\n")

	for _, job := range jobs {
		nextRun := "N/A"
		if !job.NextRun.IsZero() {
			nextRun = job.NextRun.Format(time.RFC3339)
		}
		taskPreview := job.Trigger
		if len(taskPreview) > 50 {
			taskPreview = taskPreview[:47] + "..."
		}
		sb.WriteString(fmt.Sprintf("%-30s %-20s %-10s %-25s %s\n",
			job.ID, job.CronExpr, job.Status, nextRun, taskPreview))
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: sb.String(),
		Metadata: map[string]any{
			"count": len(jobs),
		},
	}, nil
}
