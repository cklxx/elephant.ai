package llm

import (
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/json"
	"alex/internal/shared/utils"
)

func parseResponsesOutput(resp responsesResponse) (string, []ports.ToolCall, ports.Thinking) {
	var contentBuilder strings.Builder
	var toolCalls []ports.ToolCall
	var thinking ports.Thinking

	for _, item := range resp.Output {
		switch strings.ToLower(strings.TrimSpace(item.Type)) {
		case "message":
			for _, part := range item.Content {
				kind := strings.ToLower(strings.TrimSpace(part.Type))
				switch kind {
				case "output_text", "text":
					if text := responseContentText(part); text != "" {
						contentBuilder.WriteString(text)
					}
				case "reasoning", "thinking":
					appendThinkingText(&thinking, kind, responseContentTextTrimmed(part))
				}
			}
			for _, tc := range item.ToolCalls {
				args := parseToolArguments([]byte(tc.Function.Arguments))
				toolCalls = append(toolCalls, ports.ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: args,
				})
			}
		case "reasoning", "thinking":
			kind := strings.ToLower(strings.TrimSpace(item.Type))
			appendThinkingFromResponseContents(&thinking, kind, item.Content)
			appendThinkingFromResponseContents(&thinking, kind, item.Summary)
		case "tool_call", "function_call":
			args := parseToolArguments(item.Arguments)
			toolCalls = append(toolCalls, ports.ToolCall{
				ID:        item.ID,
				Name:      item.Name,
				Arguments: args,
			})
		}
	}

	content := contentBuilder.String()
	if utils.IsBlank(content) {
		if text := flattenOutputText(resp.OutputText); text != "" {
			content = text
		}
	}

	return content, toolCalls, thinking
}

func flattenOutputText(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		var builder strings.Builder
		for _, item := range v {
			if s, ok := item.(string); ok {
				builder.WriteString(s)
			}
		}
		return builder.String()
	default:
		return ""
	}
}

func parseToolArguments(raw jsonx.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}

	var args map[string]any
	if err := jsonx.Unmarshal(raw, &args); err == nil {
		return args
	}

	var asString string
	if err := jsonx.Unmarshal(raw, &asString); err != nil {
		return nil
	}

	if err := jsonx.Unmarshal([]byte(asString), &args); err != nil {
		return nil
	}
	return args
}

func appendThinkingFromResponseContents(thinking *ports.Thinking, kind string, parts []responseContent) {
	for _, part := range parts {
		appendThinkingText(thinking, kind, responseContentTextTrimmed(part))
	}
}

func responseContentText(part responseContent) string {
	if strings.TrimSpace(part.Text) != "" {
		return part.Text
	}
	if strings.TrimSpace(part.Content) != "" {
		return part.Content
	}
	if len(part.Summary) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, summaryPart := range part.Summary {
		if text := responseContentText(summaryPart); text != "" {
			builder.WriteString(text)
		}
	}
	return builder.String()
}

func responseContentTextTrimmed(part responseContent) string {
	return strings.TrimSpace(responseContentText(part))
}
