package react

import (
	"time"

	agent "alex/internal/agent/ports/agent"
)

func (e *ReactEngine) observeToolResults(state *TaskState, iteration int, results []ToolResult) {
	if state == nil || len(results) == 0 {
		return
	}
	ensureWorldStateMap(state)
	updates := make([]map[string]any, 0, len(results))
	for _, result := range results {
		updates = append(updates, summarizeToolResultForWorld(result))
	}
	state.WorldState["last_tool_results"] = updates
	state.WorldState["last_iteration"] = iteration
	state.WorldState["last_updated_at"] = e.clock.Now().Format(time.RFC3339)
	state.WorldDiff = map[string]any{
		"iteration":    iteration,
		"tool_results": updates,
	}
	e.compactToolCallHistory(state, results)
	e.appendFeedbackSignals(state, results)
}

func (e *ReactEngine) appendFeedbackSignals(state *TaskState, results []ToolResult) {
	if state == nil || len(results) == 0 {
		return
	}
	now := e.clock.Now()
	for _, result := range results {
		signal := agent.FeedbackSignal{
			Kind:      "tool_result",
			Message:   buildFeedbackMessage(result),
			Value:     deriveFeedbackValue(result),
			CreatedAt: now,
		}
		state.FeedbackSignals = append(state.FeedbackSignals, signal)
	}
	if len(state.FeedbackSignals) > maxFeedbackSignals {
		state.FeedbackSignals = state.FeedbackSignals[len(state.FeedbackSignals)-maxFeedbackSignals:]
	}
}

func (e *ReactEngine) compactToolCallHistory(state *TaskState, results []ToolResult) {
	if state == nil || len(results) == 0 {
		return
	}
	resultMap := make(map[string]ToolResult, len(results))
	for _, result := range results {
		if result.CallID == "" {
			continue
		}
		resultMap[result.CallID] = result
	}
	if len(resultMap) == 0 {
		return
	}

	for idx := range state.Messages {
		msg := state.Messages[idx]
		if len(msg.ToolCalls) == 0 {
			continue
		}
		changed := false
		for i, call := range msg.ToolCalls {
			result, ok := resultMap[call.ID]
			if !ok {
				continue
			}
			if compacted, updated := compactToolCallArguments(call, result); updated {
				call.Arguments = compacted
				msg.ToolCalls[i] = call
				changed = true
			}
		}
		if changed {
			state.Messages[idx] = msg
		}
	}
}
