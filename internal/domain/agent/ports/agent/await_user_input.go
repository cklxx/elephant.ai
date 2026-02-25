package agent

import (
	"strings"

	core "alex/internal/domain/agent/ports"
)

const (
	awaitUserInputKey       = "needs_user_input"
	awaitUserQuestionKey    = "question_to_user"
	awaitUserMessageKey     = "message"
	awaitUserOptionsKey     = "options"
	awaitUserInputTrueValue = "true"
)

// AwaitUserInputPrompt captures the extracted await-user-input payload.
type AwaitUserInputPrompt struct {
	Question string
	Options  []string
}

// ExtractAwaitUserInputPrompt scans messages for the most recent tool result
// that signals awaiting user input and returns a structured prompt.
func ExtractAwaitUserInputPrompt(messages []core.Message) (AwaitUserInputPrompt, bool) {
	if len(messages) == 0 {
		return AwaitUserInputPrompt{}, false
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

			question := toolResultStringMeta(result, awaitUserQuestionKey)
			if question == "" {
				question = toolResultStringMeta(result, awaitUserMessageKey)
			}
			if question == "" {
				question = strings.TrimSpace(result.Content)
			}
			if question == "" {
				return AwaitUserInputPrompt{}, false
			}
			return AwaitUserInputPrompt{
				Question: question,
				Options:  toolResultOptionsMeta(result, awaitUserOptionsKey),
			}, true
		}
	}
	return AwaitUserInputPrompt{}, false
}

// ExtractAwaitUserInputQuestion scans messages for the most recent tool result
// that signals awaiting user input and returns the question/message to ask.
func ExtractAwaitUserInputQuestion(messages []core.Message) (string, bool) {
	prompt, ok := ExtractAwaitUserInputPrompt(messages)
	if !ok {
		return "", false
	}
	return prompt.Question, true
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

func toolResultOptionsMeta(result core.ToolResult, key string) []string {
	if result.Metadata == nil {
		return nil
	}
	raw, ok := result.Metadata[key]
	if !ok {
		return nil
	}

	out := make([]string, 0, 4)
	seen := make(map[string]struct{})
	appendOption := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		if _, exists := seen[trimmed]; exists {
			return
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}

	appendFromObject := func(v map[string]any) {
		if label, ok := v["label"].(string); ok {
			appendOption(label)
			return
		}
		if text, ok := v["text"].(string); ok {
			appendOption(text)
			return
		}
		if value, ok := v["value"].(string); ok {
			appendOption(value)
			return
		}
	}

	switch v := raw.(type) {
	case []string:
		for _, item := range v {
			appendOption(item)
		}
	case []any:
		for _, item := range v {
			switch option := item.(type) {
			case string:
				appendOption(option)
			case map[string]any:
				appendFromObject(option)
			}
		}
	case []map[string]any:
		for _, item := range v {
			appendFromObject(item)
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}
