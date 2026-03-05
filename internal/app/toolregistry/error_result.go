package toolregistry

import (
	ports "alex/internal/domain/agent/ports"
)

func toolErrorResult(call ports.ToolCall, err error) *ports.ToolResult {
	return &ports.ToolResult{CallID: call.ID, Error: err}
}
