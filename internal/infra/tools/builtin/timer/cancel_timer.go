package timer

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type cancelTimer struct {
	shared.BaseTool
}

// NewCancelTimer creates the cancel_timer tool.
func NewCancelTimer() tools.ToolExecutor {
	return &cancelTimer{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "cancel_timer",
				Description: "Cancel/remove/retire an existing timer by timer_id (withdraw stale reminder/nudge cadence). Use only when deletion/cancellation intent is explicit; use list_timers to inspect active timers.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"timer_id": {
							Type:        "string",
							Description: "The timer ID to cancel (e.g., 'tmr-2ABC...').",
						},
					},
					Required: []string{"timer_id"},
				},
			},
			ports.ToolMetadata{
				Name:     "cancel_timer",
				Version:  "1.0.0",
				Category: "timer",
				Tags:     []string{"timer", "cancel", "delete", "remove", "reminder"},
			},
		),
	}
}

func (t *cancelTimer) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	mgr := shared.TimerManagerFromContext(ctx)
	if mgr == nil {
		return shared.ToolError(call.ID, "timer manager not available")
	}

	timerID := strings.TrimSpace(shared.StringArg(call.Arguments, "timer_id"))
	if timerID == "" {
		return shared.ToolError(call.ID, "timer_id is required")
	}

	// Check timer exists before cancellation.
	timer, ok := mgr.Get(timerID)
	if !ok {
		return shared.ToolError(call.ID, "timer not found: %s", timerID)
	}

	if err := mgr.Cancel(timerID); err != nil {
		return shared.ToolError(call.ID, "failed to cancel timer: %v", err)
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Timer cancelled:\n- ID: %s\n- Name: %s\n- Task: %s", timerID, timer.Name, timer.Task),
		Metadata: map[string]any{
			"timer_id":   timerID,
			"timer_name": timer.Name,
		},
	}, nil
}
