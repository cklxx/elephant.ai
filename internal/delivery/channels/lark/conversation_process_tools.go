package lark

import (
	"context"
	"fmt"
	"strings"

	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

const (
	queryTasksToolName   = "query_tasks"
	queryUsageToolName   = "query_usage"
	manageNoticeToolName = "manage_notice"
)

// buildQueryTasksTool constructs the query_tasks tool definition.
func buildQueryTasksTool() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        queryTasksToolName,
		Description: "When the user asks about tasks, progress, or what's running → query and report task status.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"scope": {
					Type:        "string",
					Description: "Query scope: 'active' for running tasks, 'status' for a specific task, 'history' for completed tasks.",
					Enum:        []any{"active", "status", "history"},
				},
				"task_id": {
					Type:        "string",
					Description: "Task ID to query (required when scope='status').",
				},
				"ack": {
					Type:        "string",
					Description: "Reply to show the user summarising the task status.",
				},
			},
			Required: []string{"scope", "ack"},
		},
	}
}

// buildQueryUsageTool constructs the query_usage tool definition.
func buildQueryUsageTool() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        queryUsageToolName,
		Description: "When the user asks about usage, cost, stats, or model info → report AI usage statistics.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"period": {
					Type:        "string",
					Description: "Time period: 'today', 'week', or 'all'. Default 'today'.",
					Enum:        []any{"today", "week", "all"},
				},
				"ack": {
					Type:        "string",
					Description: "Reply to show the user summarising usage data.",
				},
			},
			Required: []string{"ack"},
		},
	}
}

// buildManageNoticeTool constructs the manage_notice tool definition.
func buildManageNoticeTool() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        manageNoticeToolName,
		Description: "When the user wants to manage notification group binding → bind, check, or clear the notice chat.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"action": {
					Type:        "string",
					Description: "Action to perform: 'bind', 'status', or 'clear'.",
					Enum:        []any{"bind", "status", "clear"},
				},
				"ack": {
					Type:        "string",
					Description: "Reply to show the user confirming the notice action.",
				},
			},
			Required: []string{"action", "ack"},
		},
	}
}

// executeQueryTasks handles the query_tasks tool call, returning structured
// data for the conversation LLM to narrate.
func (g *Gateway) executeQueryTasks(ctx context.Context, msg *incomingMessage, args map[string]any) string {
	scope, _ := args["scope"].(string)
	if scope == "" {
		scope = "active"
	}

	switch scope {
	case "active":
		return g.queryTasksActive(ctx, msg)
	case "status":
		taskID, _ := args["task_id"].(string)
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			return "error: task_id required for scope=status"
		}
		return g.queryTasksStatus(ctx, taskID)
	case "history":
		return g.queryTasksHistory(ctx, msg)
	default:
		return g.queryTasksActive(ctx, msg)
	}
}

func (g *Gateway) queryTasksActive(ctx context.Context, msg *incomingMessage) string {
	if g.taskStore == nil {
		return "No task store configured."
	}
	tasks, err := g.taskStore.ListByChat(ctx, msg.chatID, true, 10)
	if err != nil {
		return fmt.Sprintf("Failed to query tasks: %v", err)
	}
	if len(tasks) == 0 {
		return "No active tasks."
	}
	return g.formatActiveTaskList(tasks)
}

func (g *Gateway) queryTasksStatus(ctx context.Context, taskID string) string {
	if g.taskStore == nil {
		return "No task store configured."
	}
	task, ok, err := g.taskStore.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Sprintf("Failed to query task: %v", err)
	}
	if !ok {
		return fmt.Sprintf("Task not found: %s", taskID)
	}
	return formatTaskDetail(task)
}

func (g *Gateway) queryTasksHistory(ctx context.Context, msg *incomingMessage) string {
	if g.taskStore == nil {
		return "No task store configured."
	}
	tasks, err := g.taskStore.ListByChat(ctx, msg.chatID, false, 10)
	if err != nil {
		return fmt.Sprintf("Failed to query task history: %v", err)
	}
	if len(tasks) == 0 {
		return "No task history."
	}
	return formatTaskHistory(tasks)
}

// executeQueryUsage handles the query_usage tool call.
func (g *Gateway) executeQueryUsage(ctx context.Context, msg *incomingMessage, args map[string]any) string {
	period, _ := args["period"].(string)
	if period == "" {
		period = "today"
	}

	now := g.currentTime()
	var sb strings.Builder

	// Model info
	sb.WriteString(g.formatCurrentModel(ctx, msg))

	switch period {
	case "week":
		sb.WriteString(g.formatCostSummary(ctx, now))
	case "all":
		sb.WriteString(g.formatCostSummary(ctx, now))
		sb.WriteString(g.formatTopTasks(ctx, msg.chatID))
	default: // "today"
		if g.costTracker != nil {
			today, err := g.costTracker.GetDailyCost(ctx, now)
			if err == nil && today != nil && today.RequestCount > 0 {
				sb.WriteString("\nToday:\n")
				sb.WriteString(formatCostSummaryBlock(today))
			} else {
				sb.WriteString("\nNo usage data for today.\n")
			}
		}
	}

	sb.WriteString(g.formatActiveTaskSummary(ctx, msg.chatID))
	return sb.String()
}

// executeManageNotice handles the manage_notice tool call.
func (g *Gateway) executeManageNotice(msg *incomingMessage, args map[string]any) string {
	action, _ := args["action"].(string)
	switch action {
	case "bind":
		return g.bindNoticeChat(msg)
	case "status":
		return g.noticeStatus()
	case "clear":
		return g.clearNoticeChat(msg)
	default:
		return g.noticeStatus()
	}
}

// executeStopWorkerExtended extends stop_worker to also cancel TaskStore records.
func (g *Gateway) executeStopWorkerExtended(ctx context.Context, slotMap *chatSlotMap, taskIDArg string) {
	taskIDArg = strings.TrimSpace(taskIDArg)
	if taskIDArg == "" {
		slotMap.stopAll(true)
		return
	}

	// Stop in-memory slot.
	slotMap.stopByTaskID(taskIDArg)

	// Also cancel in TaskStore if it's a background task ID.
	if g.taskStore != nil {
		task, ok, err := g.taskStore.GetTask(ctx, taskIDArg)
		if err == nil && ok && !isTerminalTaskStatus(task.Status) {
			if updateErr := g.taskStore.UpdateStatus(ctx, taskIDArg, taskStatusCancelled, WithErrorText("user cancelled")); updateErr != nil {
				g.logger.Warn("stop_worker: TaskStore cancel failed for %s: %v", taskIDArg, updateErr)
			}
			// Best-effort: cancel via BackgroundTaskCanceller.
			if canceller, ok := g.agent.(agent.BackgroundTaskCanceller); ok {
				if cancelErr := canceller.CancelBackgroundTask(ctx, taskIDArg); cancelErr != nil {
					g.logger.Warn("stop_worker: background cancel %s: %v", taskIDArg, cancelErr)
				}
			}
		}
	}
}
