package larktools

import (
	"fmt"

	"alex/internal/domain/agent/ports"
)

// sdkCallErr returns a ToolResult for a Lark SDK transport error (network, timeout, etc.).
func sdkCallErr(callID, label string, err error) *ports.ToolResult {
	return &ports.ToolResult{
		CallID:  callID,
		Content: fmt.Sprintf("%s: API call failed: %v", label, err),
		Error:   fmt.Errorf("lark API call failed: %w", err),
	}
}

// sdkRespErr returns a ToolResult for a Lark SDK business error (non-zero code).
func sdkRespErr(callID, label string, code int, msg string) *ports.ToolResult {
	return &ports.ToolResult{
		CallID:  callID,
		Content: fmt.Sprintf("%s: API error code=%d msg=%s", label, code, msg),
		Error:   fmt.Errorf("lark API error: code=%d msg=%s", code, msg),
	}
}

// apiErr returns a ToolResult for a failed wrapped-API call ("Failed to X" pattern).
func apiErr(callID, operation string, err error) *ports.ToolResult {
	return &ports.ToolResult{
		CallID:  callID,
		Content: fmt.Sprintf("Failed to %s: %v", operation, err),
		Error:   err,
	}
}
