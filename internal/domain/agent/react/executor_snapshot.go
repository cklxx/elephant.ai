package react

import (
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
)

func buildExecutorStateSnapshot(state *TaskState, call ToolCall) *agent.TaskState {
	if state == nil {
		return nil
	}

	snapshot := agent.CloneTaskState((*agent.TaskState)(state))
	snapshot.Messages = removeExecutorToolCallMessage(snapshot.Messages, call.ID)
	return snapshot
}

func removeExecutorToolCallMessage(messages []ports.Message, toolCallID string) []ports.Message {
	if len(messages) == 0 {
		return nil
	}

	targetIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if containsExecutorToolCall(messages[i].ToolCalls, toolCallID) {
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

func containsExecutorToolCall(calls []ports.ToolCall, toolCallID string) bool {
	if len(calls) == 0 {
		return false
	}
	for _, call := range calls {
		if call.Name != "acp_executor" {
			continue
		}
		if toolCallID == "" || call.ID == toolCallID {
			return true
		}
	}
	return false
}
