package toolregistry

import (
	"fmt"

	ports "alex/internal/domain/agent/ports"
)

func toolErrorResult(call ports.ToolCall, err error) *ports.ToolResult {
	return &ports.ToolResult{CallID: call.ID, Error: err}
}

func toolErrorResultf(call ports.ToolCall, format string, args ...any) *ports.ToolResult {
	return toolErrorResult(call, fmt.Errorf(format, args...))
}
