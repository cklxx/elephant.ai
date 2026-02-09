package aliases

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
)

type readFile struct {
	shared.BaseTool
}

func NewReadFile(cfg shared.FileToolConfig) tools.ToolExecutor {
	_ = cfg
	return &readFile{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "read_file",
				Description: "Read repository/workspace file content from a specific absolute path, including proof/context windows around suspect logic. Use after file/path selection; for directory inventory use list_dir/find. Do not use for memory notes (use memory_search/memory_get).",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path":       {Type: "string", Description: "Absolute file path"},
						"start_line": {Type: "integer", Description: "Optional start line (0-based)"},
						"end_line":   {Type: "integer", Description: "Optional end line (exclusive)"},
						"sudo":       {Type: "boolean", Description: "Use sudo privileges"},
					},
					Required: []string{"path"},
				},
			},
			ports.ToolMetadata{
				Name:     "read_file",
				Version:  "0.1.0",
				Category: "files",
				Tags:     []string{"file", "read", "content", "inspect"},
			},
		),
	}
}

func (t *readFile) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	resolved, err := pathutil.ResolveLocalPath(ctx, path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	content, err := os.ReadFile(resolved)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	output := string(content)
	if start, ok := intArgOptional(call.Arguments, "start_line"); ok || hasEndLine(call.Arguments) {
		end, _ := intArgOptional(call.Arguments, "end_line")
		if start < 0 {
			start = 0
		}
		lines := strings.Split(output, "\n")
		if start >= len(lines) {
			output = ""
		} else {
			if end <= 0 || end > len(lines) {
				end = len(lines)
			}
			if end < start {
				err := fmt.Errorf("end_line must be >= start_line")
				return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
			}
			output = strings.Join(lines[start:end], "\n")
		}
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  output,
		Metadata: map[string]any{"path": resolved},
	}, nil
}

func hasEndLine(args map[string]any) bool {
	if args == nil {
		return false
	}
	_, ok := args["end_line"]
	return ok
}
