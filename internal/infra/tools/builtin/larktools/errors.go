package larktools

import (
	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/shared"
)

func larkToolErrorResult(callID, format string, args ...any) *ports.ToolResult {
	result, _ := shared.ToolError(callID, format, args...)
	return result
}

func missingChatIDResult(callID, toolName string) *ports.ToolResult {
	return larkToolErrorResult(callID, "%s: no chat_id available in context.", toolName)
}
