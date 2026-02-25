package ui

import (
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/shared"
)

func parseOptionsArg(call ports.ToolCall) ([]string, *ports.ToolResult) {
	raw, exists := call.Arguments["options"]
	if !exists {
		return nil, nil
	}

	out := make([]string, 0, 4)
	seen := make(map[string]struct{})
	appendOption := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}

	switch value := raw.(type) {
	case []string:
		for _, item := range value {
			appendOption(item)
		}
	case []any:
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				result, _ := shared.ToolError(call.ID, "options must be an array of strings")
				return nil, result
			}
			appendOption(text)
		}
	default:
		result, _ := shared.ToolError(call.ID, "options must be an array of strings")
		return nil, result
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
