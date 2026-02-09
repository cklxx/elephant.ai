package aliases

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/sandbox"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
)

type searchFile struct {
	shared.BaseTool
}

func NewSearchFile(cfg shared.FileToolConfig) tools.ToolExecutor {
	_ = cfg
	return &searchFile{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "search_file",
				Description: "Search semantic/content-first rule evidence inside one known absolute file path (line/content matches in file bodies). Use for inside-file matching only. Do not use for path/name discovery (use find/list_dir), and do not use for visual/browser capture.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path":  {Type: "string", Description: "Absolute file path (iterate over multiple files for project-wide search)"},
						"regex": {Type: "string", Description: "Regex/pattern/symbol to search"},
						"sudo":  {Type: "boolean", Description: "Use sudo privileges"},
					},
					Required: []string{"path", "regex"},
				},
			},
			ports.ToolMetadata{
				Name:     "search_file",
				Version:  "0.1.0",
				Category: "files",
				Tags:     []string{"file", "search", "regex", "symbol", "token", "pattern"},
			},
		),
	}
}

func (t *searchFile) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	regex := strings.TrimSpace(shared.StringArg(call.Arguments, "regex"))
	if regex == "" {
		err := errors.New("regex is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	resolved, err := pathutil.ResolveLocalPath(ctx, path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	re, err := regexp.Compile(regex)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	file, err := os.Open(resolved)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var matches []string
	var lineNumbers []int
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if re.MatchString(text) {
			matches = append(matches, text)
			lineNumbers = append(lineNumbers, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	result := sandbox.FileSearchResult{
		File:        resolved,
		Matches:     matches,
		LineNumbers: lineNumbers,
	}

	payloadJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  string(payloadJSON),
		Metadata: map[string]any{"path": resolved},
	}, nil
}
