package react

import (
	"strings"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
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

	targetIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if containsSubagentToolCall(messages[i].ToolCalls, toolCallID) {
			targetIdx = i
			break
		}
	}

	if targetIdx == -1 {
		return messages
	}

	pruned := make([]ports.Message, 0, len(messages)-1)
	pruned = append(pruned, messages[:targetIdx]...)
	pruned = append(pruned, messages[targetIdx+1:]...)
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
