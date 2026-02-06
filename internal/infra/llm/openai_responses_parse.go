package llm

import (
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/jsonx"
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
					contentBuilder.WriteString(part.Text)
				case "reasoning", "thinking":
					appendThinkingText(&thinking, kind, part.Text)
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
			for _, part := range item.Content {
				if text := strings.TrimSpace(part.Text); text != "" {
					appendThinkingText(&thinking, strings.ToLower(strings.TrimSpace(item.Type)), text)
				}
			}
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
	if strings.TrimSpace(content) == "" {
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
