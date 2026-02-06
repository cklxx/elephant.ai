package react

import (
	"strings"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

func buildSubagentStateSnapshot(state *TaskState, call ToolCall) *agent.TaskState {
	if state == nil {
		return nil
	}

	snapshot := agent.CloneTaskState((*agent.TaskState)(state))
	snapshot.Messages = removeSubagentToolCallMessage(snapshot.Messages, call.ID)

	if prompt := extractSubagentPrompt(call.Arguments); prompt != "" {
		snapshot.Messages = append(snapshot.Messages, ports.Message{
			Role:    "user",
			Content: prompt,
			Source:  ports.MessageSourceUserInput,
		})
	}

	return snapshot
}

func removeSubagentToolCallMessage(messages []ports.Message, toolCallID string) []ports.Message {
	if len(messages) == 0 {
		return nil
	}

	pruned := make([]ports.Message, 0, len(messages))
	for _, msg := range messages {
		if containsSubagentToolCall(msg.ToolCalls, toolCallID) {
			continue
		}
		if toolCallID != "" && isToolResultForCall(msg, toolCallID) {
			continue
		}
		pruned = append(pruned, msg)
	}
	return pruned
}

func containsSubagentToolCall(calls []ports.ToolCall, toolCallID string) bool {
	if len(calls) == 0 {
		return false
	}
	for _, call := range calls {
		if call.Name != "subagent" {
			continue
		}
		if toolCallID == "" || call.ID == toolCallID {
			return true
		}
	}
	return false
}

func isToolResultForCall(msg ports.Message, toolCallID string) bool {
	if strings.EqualFold(strings.TrimSpace(msg.Role), "tool") &&
		strings.TrimSpace(msg.ToolCallID) == toolCallID {
		return true
	}
	for _, result := range msg.ToolResults {
		if strings.TrimSpace(result.CallID) == toolCallID {
			return true
		}
	}
	return false
}

func extractSubagentPrompt(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}

	if raw, ok := args["prompt"]; ok {
		if str, ok := raw.(string); ok {
			return strings.TrimSpace(str)
		}
	}

	return ""
}
