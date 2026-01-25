package react

import "alex/internal/agent/ports"

// splitMessagesForLLM separates messages that are safe for the model from
// system-only entries (e.g., attachment catalogs) while deep-cloning them.
func splitMessagesForLLM(messages []Message) ([]Message, []Message) {
	if len(messages) == 0 {
		return nil, nil
	}
	filtered := make([]Message, 0, len(messages))
	excluded := make([]Message, 0)
	for _, msg := range messages {
		cloned := cloneMessageForLLM(msg)
		switch msg.Source {
		case ports.MessageSourceDebug, ports.MessageSourceEvaluation:
			excluded = append(excluded, cloned)
		default:
			filtered = append(filtered, cloned)
		}
	}
	return filtered, excluded
}

func cloneMessageForLLM(msg Message) Message {
	cloned := msg
	if len(msg.ToolCalls) > 0 {
		cloned.ToolCalls = append([]ToolCall(nil), msg.ToolCalls...)
	}
	if len(msg.ToolResults) > 0 {
		cloned.ToolResults = make([]ToolResult, len(msg.ToolResults))
		for i, result := range msg.ToolResults {
			cloned.ToolResults[i] = cloneToolResultForLLM(result)
		}
	}
	if len(msg.Metadata) > 0 {
		metadata := make(map[string]any, len(msg.Metadata))
		for key, value := range msg.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(msg.Attachments) > 0 {
		cloned.Attachments = ports.CloneAttachmentMap(msg.Attachments)
	}
	return cloned
}

func cloneToolResultForLLM(result ToolResult) ToolResult {
	cloned := result
	if len(result.Metadata) > 0 {
		metadata := make(map[string]any, len(result.Metadata))
		for key, value := range result.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(result.Attachments) > 0 {
		cloned.Attachments = ports.CloneAttachmentMap(result.Attachments)
	}
	return cloned
}
