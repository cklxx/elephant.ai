package toolregistry

import (
	"fmt"

	"alex/internal/domain/agent/ports"
)

func missingExecutorResult(callID string) (*ports.ToolResult, error) {
	return &ports.ToolResult{CallID: callID, Error: fmt.Errorf("tool executor missing")}, nil
}
