package timer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	tmr "alex/internal/timer"
	"alex/internal/tools/builtin/shared"
	id "alex/internal/utils/id"
)

type listTimers struct {
	shared.BaseTool
}

// NewListTimers creates the list_timers tool.
func NewListTimers() tools.ToolExecutor {
	return &listTimers{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "list_timers",
				Description: "List timers created in this or previous sessions. Returns timer IDs, names, status, and schedule details.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"status": {
							Type:        "string",
							Description: "Filter by status: 'active' (default), 'fired', 'cancelled', or 'all'.",
							Enum:        []any{"active", "fired", "cancelled", "all"},
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "list_timers",
				Version:  "1.0.0",
				Category: "timer",
			},
		),
	}
}

func (t *listTimers) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	mgr := getTimerManager(ctx)
	if mgr == nil {
		return shared.ToolError(call.ID, "timer manager not available")
	}

	statusFilter := strings.TrimSpace(shared.StringArg(call.Arguments, "status"))
	if statusFilter == "" {
		statusFilter = "active"
	}

	userID := id.UserIDFromContext(ctx)
	allTimers := mgr.List(userID)

	var filtered []tmr.Timer
	for _, timer := range allTimers {
		if statusFilter == "all" || string(timer.Status) == statusFilter {
			filtered = append(filtered, timer)
		}
	}

	if len(filtered) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("No %s timers found.", statusFilter),
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d %s timer(s):\n\n", len(filtered), statusFilter))

	for _, timer := range filtered {
		sb.WriteString(fmt.Sprintf("- **%s** (`%s`)\n", timer.Name, timer.ID))
		sb.WriteString(fmt.Sprintf("  Status: %s | Type: %s\n", timer.Status, timer.Type))
		if timer.Type == tmr.TimerTypeOnce {
			sb.WriteString(fmt.Sprintf("  Fire at: %s\n", timer.FireAt.Format(time.RFC3339)))
		} else {
			sb.WriteString(fmt.Sprintf("  Schedule: %s\n", timer.Schedule))
		}
		sb.WriteString(fmt.Sprintf("  Task: %s\n", timer.Task))
		sb.WriteString(fmt.Sprintf("  Session: %s\n", timer.SessionID))
		sb.WriteString("\n")
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: sb.String(),
		Metadata: map[string]any{
			"count":  len(filtered),
			"filter": statusFilter,
		},
	}, nil
}
