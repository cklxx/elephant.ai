package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/tools/builtin/shared"
)

type bgStatus struct {
	shared.BaseTool
}

// NewBGStatus creates the bg_status tool for querying background task status.
func NewBGStatus() *bgStatus {
	return &bgStatus{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "bg_status",
				Description: `Query the status of background tasks. Returns a grouped dashboard including pending, running, waiting-for-input, blocked, completed, failed, and cancelled tasks.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"task_ids": {
							Type:        "array",
							Description: "Optional list of task IDs to query. Omit to query all background tasks.",
							Items:       &ports.Property{Type: "string"},
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "bg_status",
				Version:  "1.0.0",
				Category: "agent",
				Tags:     []string{"background", "orchestration", "async"},
			},
		),
	}
}

func (t *bgStatus) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
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
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No background tasks found.",
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Task Dashboard (%d tasks) ===\n\n", len(summaries)))
	groups := groupSummaries(summaries)
	for _, group := range groups {
		if len(group.tasks) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s (%d)\n", group.name, len(group.tasks)))
		for _, s := range group.tasks {
			sb.WriteString(formatTaskSummary(s, group.kind))
		}
		sb.WriteString("\n")
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: sb.String(),
	}, nil
}

type statusGroupKind string

const (
	groupRunning   statusGroupKind = "running"
	groupWaiting   statusGroupKind = "waiting"
	groupBlocked   statusGroupKind = "blocked"
	groupCompleted statusGroupKind = "completed"
	groupFailed    statusGroupKind = "failed"
	groupCancelled statusGroupKind = "cancelled"
	groupPending   statusGroupKind = "pending"
)

type statusGroup struct {
	name  string
	kind  statusGroupKind
	tasks []agent.BackgroundTaskSummary
}

func groupSummaries(summaries []agent.BackgroundTaskSummary) []statusGroup {
	groups := []statusGroup{
		{name: "RUNNING", kind: groupRunning},
		{name: "WAITING FOR INPUT", kind: groupWaiting},
		{name: "BLOCKED", kind: groupBlocked},
		{name: "COMPLETED", kind: groupCompleted},
		{name: "FAILED", kind: groupFailed},
		{name: "CANCELLED", kind: groupCancelled},
		{name: "PENDING", kind: groupPending},
	}
	for _, summary := range summaries {
		kind := classifySummary(summary)
		for idx := range groups {
			if groups[idx].kind == kind {
				groups[idx].tasks = append(groups[idx].tasks, summary)
				break
			}
		}
	}
	return groups
}

func classifySummary(s agent.BackgroundTaskSummary) statusGroupKind {
	if s.Status == agent.BackgroundTaskStatusRunning && s.PendingInput != nil {
		return groupWaiting
	}
	switch s.Status {
	case agent.BackgroundTaskStatusRunning:
		return groupRunning
	case agent.BackgroundTaskStatusBlocked:
		return groupBlocked
	case agent.BackgroundTaskStatusCompleted:
		return groupCompleted
	case agent.BackgroundTaskStatusFailed:
		return groupFailed
	case agent.BackgroundTaskStatusCancelled:
		return groupCancelled
	default:
		return groupPending
	}
}

func formatTaskSummary(s agent.BackgroundTaskSummary, kind statusGroupKind) string {
	var sb strings.Builder
	elapsed := formatDuration(s.Elapsed)
	sb.WriteString(fmt.Sprintf("  - %s [%s] %s\n", s.ID, s.AgentType, elapsed))
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("    desc: %s\n", s.Description))
	}
	if s.Workspace != nil {
		sb.WriteString(fmt.Sprintf("    workspace: %s", s.Workspace.Mode))
		if s.Workspace.Branch != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", s.Workspace.Branch))
		}
		sb.WriteString("\n")
	}
	if len(s.FileScope) > 0 {
		sb.WriteString(fmt.Sprintf("    scope: %s\n", strings.Join(truncateList(s.FileScope, 3), ", ")))
	}
	if len(s.DependsOn) > 0 {
		sb.WriteString(fmt.Sprintf("    depends_on: %s\n", strings.Join(s.DependsOn, ", ")))
	}

	switch kind {
	case groupRunning:
		sb.WriteString(formatProgressLines(s))
	case groupWaiting:
		sb.WriteString(formatProgressLines(s))
		if s.PendingInput != nil {
			sb.WriteString(fmt.Sprintf("    input: %s (request_id=%s)\n", s.PendingInput.Summary, s.PendingInput.RequestID))
		}
	case groupFailed:
		if s.Error != "" {
			sb.WriteString(fmt.Sprintf("    error: %s\n", s.Error))
		}
	case groupCancelled:
		if s.Error != "" {
			sb.WriteString(fmt.Sprintf("    error: %s\n", s.Error))
		}
	}

	return sb.String()
}

func formatProgressLines(s agent.BackgroundTaskSummary) string {
	if s.Progress == nil {
		return ""
	}
	p := s.Progress
	var sb strings.Builder
	iter := fmt.Sprintf("%d", p.Iteration)
	if p.MaxIter > 0 {
		iter = fmt.Sprintf("%d/%d", p.Iteration, p.MaxIter)
	}
	line := fmt.Sprintf("    progress: iter %s · %d tokens", iter, p.TokensUsed)
	if p.CostUSD > 0 {
		line = fmt.Sprintf("%s · $%.2f", line, p.CostUSD)
	}
	sb.WriteString(line)
	sb.WriteString("\n")
	if p.CurrentTool != "" {
		if p.CurrentArgs != "" {
			sb.WriteString(fmt.Sprintf("    current: %s(%s)\n", p.CurrentTool, p.CurrentArgs))
		} else {
			sb.WriteString(fmt.Sprintf("    current: %s\n", p.CurrentTool))
		}
	}
	if len(p.FilesTouched) > 0 {
		sb.WriteString(fmt.Sprintf("    files: %s\n", strings.Join(truncateList(p.FilesTouched, 3), ", ")))
	}
	return sb.String()
}

func truncateList(items []string, limit int) []string {
	if len(items) <= limit || limit <= 0 {
		return items
	}
	out := append([]string(nil), items[:limit]...)
	out = append(out, fmt.Sprintf("(+%d more)", len(items)-limit))
	return out
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	seconds := int(d.Seconds())
	min := seconds / 60
	sec := seconds % 60
	if min > 0 {
		return fmt.Sprintf("%dm%ds", min, sec)
	}
	return fmt.Sprintf("%ds", sec)
}
