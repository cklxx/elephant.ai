package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type schedulerCreate struct {
	shared.BaseTool
}

// NewSchedulerCreate creates the scheduler_create_job tool.
func NewSchedulerCreate() tools.ToolExecutor {
	return &schedulerCreate{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "scheduler_create_job",
				Description: "Create a new scheduled job that runs on a cron schedule. The job persists across restarts and executes the given task prompt autonomously when the schedule fires.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"name": {
							Type:        "string",
							Description: "Human-readable job name used as the unique identifier (e.g., 'daily_standup', 'weekly_report')",
						},
						"schedule": {
							Type:        "string",
							Description: "Cron expression (5-field: minute hour dom month dow). Examples: '0 9 * * 1-5' (weekday 9am), '*/30 * * * *' (every 30 min)",
						},
						"task": {
							Type:        "string",
							Description: "The prompt/instruction for the agent to execute when the schedule fires",
						},
						"channel": {
							Type:        "string",
							Description: "Notification channel for results: 'lark' or 'moltbook'. Defaults to the originating channel.",
						},
					},
					Required: []string{"name", "schedule", "task"},
				},
			},
			ports.ToolMetadata{
				Name:     "scheduler_create_job",
				Version:  "1.0.0",
				Category: "scheduler",
				Tags:     []string{"scheduler", "cron", "automation"},
			},
		),
	}
}

func (t *schedulerCreate) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	svc := getService(ctx)
	if svc == nil {
		return shared.ToolError(call.ID, "scheduler not available")
	}

	name := strings.TrimSpace(shared.StringArg(call.Arguments, "name"))
	schedule := strings.TrimSpace(shared.StringArg(call.Arguments, "schedule"))
	task := strings.TrimSpace(shared.StringArg(call.Arguments, "task"))
	channel := strings.TrimSpace(shared.StringArg(call.Arguments, "channel"))

	if name == "" {
		return shared.ToolError(call.ID, "name is required")
	}
	if schedule == "" {
		return shared.ToolError(call.ID, "schedule is required")
	}
	if task == "" {
		return shared.ToolError(call.ID, "task is required")
	}

	// Validate cron expression before attempting registration.
	if _, err := svc.CronParser().Parse(schedule); err != nil {
		return shared.ToolError(call.ID, "invalid cron expression %q: %v", schedule, err)
	}

	job, err := svc.RegisterDynamicTrigger(ctx, name, schedule, task, channel)
	if err != nil {
		return shared.ToolError(call.ID, "failed to create job: %v", err)
	}

	nextRun := "unknown"
	if !job.NextRun.IsZero() {
		nextRun = job.NextRun.Format(time.RFC3339)
	}

	content := fmt.Sprintf("Scheduled job created:\n- ID: %s\n- Name: %s\n- Schedule: %s\n- Next run: %s\n- Task: %s",
		job.ID, job.Name, job.CronExpr, nextRun, job.Trigger)

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"job_id":   job.ID,
			"name":     job.Name,
			"schedule": job.CronExpr,
			"next_run": nextRun,
			"status":   job.Status,
		},
	}, nil
}
