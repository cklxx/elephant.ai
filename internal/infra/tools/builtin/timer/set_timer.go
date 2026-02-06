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

type setTimer struct {
	shared.BaseTool
}

// NewSetTimer creates the set_timer tool.
func NewSetTimer() tools.ToolExecutor {
	return &setTimer{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "set_timer",
				Description: `Create a timer that fires at a specified time or after a delay, executing a task within the current session context. When the timer fires, the agent resumes with full conversation history and runs the task prompt autonomously.

Use this to schedule follow-ups, reminders, periodic checks, or deferred work. The timer persists across server restarts.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"name": {
							Type:        "string",
							Description: "Human-readable timer name (e.g., 'daily standup reminder', 'follow up on PR review')",
						},
						"task": {
							Type:        "string",
							Description: "Task prompt for the agent to execute when the timer fires. Should be a complete instruction.",
						},
						"type": {
							Type:        "string",
							Description: "Timer type: 'once' (default) fires once; 'recurring' fires on a cron schedule.",
							Enum:        []any{"once", "recurring"},
						},
						"delay": {
							Type:        "string",
							Description: "Relative delay for one-shot timers: '5m', '1h', '2h30m'. Parsed as Go duration.",
						},
						"fire_at": {
							Type:        "string",
							Description: "Absolute fire time in RFC3339 format (e.g., '2026-02-01T15:00:00+08:00'). Alternative to delay.",
						},
						"schedule": {
							Type:        "string",
							Description: "Cron expression for recurring timers (5-field: minute hour dom month dow). Required when type is 'recurring'.",
						},
						"channel": {
							Type:        "string",
							Description: "Override notification channel: 'lark' or 'moltbook'. Defaults to originating channel.",
						},
						"chat_id": {
							Type:        "string",
							Description: "Override chat ID for notifications.",
						},
					},
					Required: []string{"name", "task"},
				},
			},
			ports.ToolMetadata{
				Name:     "set_timer",
				Version:  "1.0.0",
				Category: "timer",
			},
		),
	}
}

func (t *setTimer) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	mgr := getTimerManager(ctx)
	if mgr == nil {
		return shared.ToolError(call.ID, "timer manager not available")
	}

	name := strings.TrimSpace(shared.StringArg(call.Arguments, "name"))
	task := strings.TrimSpace(shared.StringArg(call.Arguments, "task"))
	if name == "" {
		return shared.ToolError(call.ID, "name is required")
	}
	if task == "" {
		return shared.ToolError(call.ID, "task is required")
	}

	timerType := tmr.TimerTypeOnce
	if typ := strings.TrimSpace(shared.StringArg(call.Arguments, "type")); typ == "recurring" {
		timerType = tmr.TimerTypeRecurring
	}

	delay := strings.TrimSpace(shared.StringArg(call.Arguments, "delay"))
	fireAtStr := strings.TrimSpace(shared.StringArg(call.Arguments, "fire_at"))
	schedule := strings.TrimSpace(shared.StringArg(call.Arguments, "schedule"))
	channel := strings.TrimSpace(shared.StringArg(call.Arguments, "channel"))
	chatID := strings.TrimSpace(shared.StringArg(call.Arguments, "chat_id"))

	// Resolve fire time.
	var fireAt time.Time
	switch timerType {
	case tmr.TimerTypeOnce:
		if delay != "" {
			d, err := time.ParseDuration(delay)
			if err != nil {
				return shared.ToolError(call.ID, "invalid delay %q: %v", delay, err)
			}
			if d <= 0 {
				return shared.ToolError(call.ID, "delay must be positive")
			}
			fireAt = time.Now().Add(d)
		} else if fireAtStr != "" {
			parsed, err := time.Parse(time.RFC3339, fireAtStr)
			if err != nil {
				return shared.ToolError(call.ID, "invalid fire_at %q: %v", fireAtStr, err)
			}
			if parsed.Before(time.Now()) {
				return shared.ToolError(call.ID, "fire_at must be in the future")
			}
			fireAt = parsed
		} else {
			return shared.ToolError(call.ID, "one-shot timer requires 'delay' or 'fire_at'")
		}
	case tmr.TimerTypeRecurring:
		if schedule == "" {
			return shared.ToolError(call.ID, "recurring timer requires 'schedule' (cron expression)")
		}
	}

	// Capture session and user from context.
	sessionID := id.SessionIDFromContext(ctx)
	userID := id.UserIDFromContext(ctx)

	// Inherit channel from context if not overridden.
	if channel == "" {
		channel = shared.LarkChatIDFromContext(ctx)
		if channel != "" {
			channel = "lark"
			if chatID == "" {
				chatID = shared.LarkChatIDFromContext(ctx)
			}
		}
	}

	timer := &tmr.Timer{
		ID:        tmr.NewTimerID(),
		Name:      name,
		Type:      timerType,
		Schedule:  schedule,
		Delay:     delay,
		FireAt:    fireAt,
		Task:      task,
		SessionID: sessionID,
		Channel:   channel,
		UserID:    userID,
		ChatID:    chatID,
		CreatedAt: time.Now().UTC(),
		Status:    tmr.StatusActive,
	}

	if err := mgr.Add(timer); err != nil {
		return shared.ToolError(call.ID, "failed to create timer: %v", err)
	}

	// Build response summary.
	var when string
	switch timerType {
	case tmr.TimerTypeOnce:
		when = fmt.Sprintf("fires at %s", fireAt.Format(time.RFC3339))
		if delay != "" {
			when = fmt.Sprintf("fires in %s (%s)", delay, fireAt.Format(time.RFC3339))
		}
	case tmr.TimerTypeRecurring:
		when = fmt.Sprintf("schedule: %s", schedule)
	}

	content := fmt.Sprintf("Timer created:\n- ID: %s\n- Name: %s\n- Type: %s\n- %s\n- Task: %s\n- Session: %s",
		timer.ID, timer.Name, timer.Type, when, timer.Task, timer.SessionID)

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"timer_id":   timer.ID,
			"timer_name": timer.Name,
			"fire_at":    timer.FireAt.Format(time.RFC3339),
			"session_id": timer.SessionID,
		},
	}, nil
}

func getTimerManager(ctx context.Context) *tmr.TimerManager {
	v := shared.TimerManagerFromContext(ctx)
	if v == nil {
		return nil
	}
	mgr, ok := v.(*tmr.TimerManager)
	if !ok {
		return nil
	}
	return mgr
}
