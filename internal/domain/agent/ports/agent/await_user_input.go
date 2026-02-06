package agent

import (
	"strings"

	core "alex/internal/domain/agent/ports"
)

const (
	awaitUserInputKey       = "needs_user_input"
	awaitUserQuestionKey    = "question_to_user"
	awaitUserMessageKey     = "message"
	awaitUserInputTrueValue = "true"
)

// ExtractAwaitUserInputQuestion scans messages for the most recent tool result
// that signals awaiting user input and returns the question/message to ask.
func ExtractAwaitUserInputQuestion(messages []core.Message) (string, bool) {
	if len(messages) == 0 {
		return "", false
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if len(msg.ToolResults) == 0 {
			continue
		}
		for j := len(msg.ToolResults) - 1; j >= 0; j-- {
			result := msg.ToolResults[j]
			if !toolResultNeedsUserInput(result) {
				continue
			}
			if question := toolResultStringMeta(result, awaitUserQuestionKey); question != "" {
				return question, true
			}
			if message := toolResultStringMeta(result, awaitUserMessageKey); message != "" {
				return message, true
			}
			if content := strings.TrimSpace(result.Content); content != "" {
				return content, true
			}
			return "", false
		}
	}
	return "", false
}

func toolResultNeedsUserInput(result core.ToolResult) bool {
	if result.Metadata == nil {
		return false
	}
	raw, ok := result.Metadata[awaitUserInputKey]
	if !ok {
		return false
	}
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), awaitUserInputTrueValue)
	default:
		return false
	}
}

func toolResultStringMeta(result core.ToolResult, key string) string {
	if result.Metadata == nil {
		return ""
	}
	raw, ok := result.Metadata[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
