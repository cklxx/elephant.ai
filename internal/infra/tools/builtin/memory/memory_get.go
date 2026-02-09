package memory

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/memory"
	"alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"
)

type memoryGet struct {
	shared.BaseTool
	engine memory.Engine
}

// NewMemoryGet constructs a tool for reading a memory file slice by line number.
func NewMemoryGet(engine memory.Engine) tools.ToolExecutor {
	return &memoryGet{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "memory_get",
				Description: "Retrieve memory note excerpts by line range from a memory path returned by memory_search (habit/persona/history notes). Use only after memory_search returns a memory path and only for memory markdown notes. Not for repository/workspace source files or proof windows around code/contracts (use read_file).",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path": {
							Type:        "string",
							Description: "Path returned from memory_search.",
						},
						"from": {
							Type:        "integer",
							Description: "Start line number (1-based).",
						},
						"lines": {
							Type:        "integer",
							Description: "Number of lines to read.",
						},
					},
					Required: []string{"path"},
				},
			},
			ports.ToolMetadata{
				Name:        "memory_get",
				Version:     "0.1.0",
				Category:    "memory",
				Tags:        []string{"memory", "notes", "history", "persona"},
				SafetyLevel: ports.SafetyLevelReadOnly,
			},
		),
		engine: engine,
	}
}

func (t *memoryGet) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.engine == nil {
		err := fmt.Errorf("memory engine not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	userID := id.UserIDFromContext(ctx)
	if strings.TrimSpace(userID) == "" {
		err := fmt.Errorf("user_id required for memory_get")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	path, errResult := shared.RequireStringArg(call.Arguments, call.ID, "path")
	if errResult != nil {
		return errResult, nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return shared.ToolError(call.ID, "path cannot be empty")
	}

	fromLine := 1
	if raw, ok := call.Arguments["from"]; ok {
		switch v := raw.(type) {
		case float64:
			if v > 0 {
				fromLine = int(v)
			}
		case int:
			if v > 0 {
				fromLine = v
			}
		}
	}
	lineCount := 20
	if raw, ok := call.Arguments["lines"]; ok {
		switch v := raw.(type) {
		case float64:
			if v > 0 {
				lineCount = int(v)
			}
		case int:
			if v > 0 {
				lineCount = v
			}
		}
	}

	text, err := t.engine.GetLines(ctx, userID, path, fromLine, lineCount)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if strings.TrimSpace(text) == "" {
		text = "(no content)"
	}

	metadata := map[string]any{
		"path":  path,
		"from":  fromLine,
		"lines": lineCount,
	}

	return &ports.ToolResult{CallID: call.ID, Content: text, Metadata: metadata}, nil
}
