package react

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (e *ReactEngine) updateGoalPlanPrompts(state *TaskState, calls []ToolCall, results []ToolResult) {
	if state == nil || len(calls) == 0 || len(results) == 0 {
		return
	}
	limit := len(calls)
	if len(results) < limit {
		limit = len(results)
	}
	for i := 0; i < limit; i++ {
		if !strings.EqualFold(strings.TrimSpace(calls[i].Name), "plan") {
			continue
		}
		if goal := extractGoalPrompt(calls[i], results[i]); goal != "" {
			state.LatestGoalPrompt = goal
		}
		if plan := extractPlanPrompt(calls[i], results[i]); plan != "" {
			state.LatestPlanPrompt = plan
		}
	}
}

func extractGoalPrompt(call ToolCall, result ToolResult) string {
	goal, hasGoal := call.Arguments["overall_goal_ui"].(string)
	if !hasGoal && result.Metadata != nil {
		goal, hasGoal = result.Metadata["overall_goal_ui"].(string)
	}
	if hasGoal {
		if trimmed := strings.TrimSpace(goal); trimmed != "" {
			return trimmed
		}
	}
	if result.Error != nil {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(call.Name), "plan") {
		if trimmed := strings.TrimSpace(result.Content); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func extractPlanPrompt(call ToolCall, result ToolResult) string {
	if plan := normalizePlanPrompt(call.Arguments["internal_plan"]); plan != "" {
		return plan
	}
	if result.Metadata != nil {
		if plan := normalizePlanPrompt(result.Metadata["internal_plan"]); plan != "" {
			return plan
		}
	}
	return ""
}

func normalizePlanPrompt(raw any) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		serialized, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(serialized))
	}
}

func (e *ReactEngine) appendGoalPlanReminder(state *TaskState, messages []Message) []Message {
	if state == nil || len(messages) == 0 {
		return messages
	}
	goal := strings.TrimSpace(state.LatestGoalPrompt)
	plan := strings.TrimSpace(state.LatestPlanPrompt)
	if goal == "" || plan == "" {
		return messages
	}
	if lengthDivergence(goal, plan) <= goalPlanLengthDivergenceThreshold {
		return messages
	}
	reminder := buildGoalPlanReminder(goal, plan)
	for i := range messages {
		if strings.TrimSpace(messages[i].Content) == "" {
			messages[i].Content = reminder
			continue
		}
		messages[i].Content = strings.TrimSpace(messages[i].Content) + "\n\n" + reminder
	}
	return messages
}

func buildGoalPlanReminder(goal, plan string) string {
	return fmt.Sprintf("<system-reminder>Goal: %s\nPlan: %s</system-reminder>", goal, plan)
}

// longRunningToolThresholdMs is the duration threshold (in milliseconds)
// above which a system reminder is injected asking the LLM to explain the
// delay to the user.
const longRunningToolThresholdMs int64 = 2 * 60 * 1000 // 2 minutes

// appendLongRunningToolReminder checks each tool message for execution
// duration metadata. When a tool took longer than longRunningToolThresholdMs,
// a system reminder is appended instructing the LLM to briefly explain the
// delay to the user.
func appendLongRunningToolReminder(calls []ToolCall, results []ToolResult, messages []Message) []Message {
	if len(messages) == 0 || len(results) == 0 {
		return messages
	}

	// Build callID → tool name index.
	callNames := make(map[string]string, len(calls))
	for _, c := range calls {
		callNames[c.ID] = c.Name
	}

	for i, result := range results {
		if i >= len(messages) {
			break
		}
		durationMs, ok := result.Metadata["_duration_ms"].(int64)
		if !ok || durationMs < longRunningToolThresholdMs {
			continue
		}
		toolName := callNames[result.CallID]
		if toolName == "" {
			toolName = result.CallID
		}
		durationSec := durationMs / 1000
		reminder := fmt.Sprintf(
			"<system-reminder>Tool \"%s\" took %ds (>2min). "+
				"Please briefly inform the user about this delay and what was being done, "+
				"so they understand why the response took longer than usual.</system-reminder>",
			toolName, durationSec,
		)
		if strings.TrimSpace(messages[i].Content) == "" {
			messages[i].Content = reminder
		} else {
			messages[i].Content = strings.TrimSpace(messages[i].Content) + "\n\n" + reminder
		}
	}
	return messages
}

// lengthDivergence returns the absolute rune-count difference between goal and
// plan. When the plan elaborates far beyond the original goal the reminder is
// injected; when they are similarly sized it is suppressed.
func lengthDivergence(goal, plan string) int {
	goalLen := len([]rune(goal))
	planLen := len([]rune(plan))
	diff := goalLen - planLen
	if diff < 0 {
		diff = -diff
	}
	return diff
}
