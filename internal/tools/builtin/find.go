package builtin

import (
	"alex/internal/agent/ports"
	"alex/internal/tools"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type find struct {
        mode    tools.ExecutionMode
        sandbox *tools.SandboxManager
}

func NewFind(cfg ShellToolConfig) ports.ToolExecutor {
        mode := cfg.Mode
        if mode == tools.ExecutionModeUnknown {
                mode = tools.ExecutionModeLocal
        }
        return &find{mode: mode, sandbox: cfg.SandboxManager}
}

func (t *find) Mode() tools.ExecutionMode {
        return t.mode
}

func (t *find) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	name, ok := call.Arguments["name"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'name'")}, nil
	}

	path := "."
	if p, ok := call.Arguments["path"].(string); ok && p != "" {
		path = p
	}

	maxDepth := 10
	if md, ok := call.Arguments["max_depth"].(float64); ok {
		maxDepth = int(md)
	}

	cmdArgs := t.buildArgs(call, path, maxDepth, name)

	if t.mode == tools.ExecutionModeSandbox {
		return t.executeSandbox(ctx, call, cmdArgs, name, path, maxDepth)
	}

	cmd := exec.CommandContext(ctx, "find", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return t.noMatchesResult(call, name, path, maxDepth)
		}
		return &ports.ToolResult{CallID: call.ID, Content: string(output), Error: fmt.Errorf("find command failed: %w", err)}, nil
	}

	return t.processOutput(call, string(output), name, path, maxDepth)
}

func (t *find) executeSandbox(ctx context.Context, call ports.ToolCall, cmdArgs []string, name, path string, maxDepth int) (*ports.ToolResult, error) {
	command := "find " + strings.Join(quoteArgs(cmdArgs), " ")
	output, exitCode, err := runSandboxCommandRaw(ctx, t.sandbox, command)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	if exitCode == 1 {
		return t.noMatchesResult(call, name, path, maxDepth)
	}
	if exitCode != 0 {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("find command failed with exit code %d", exitCode)}, nil
	}
	return t.processOutput(call, output, name, path, maxDepth)
}

func (t *find) processOutput(call ports.ToolCall, output, name, path string, maxDepth int) (*ports.ToolResult, error) {
	lines := strings.Split(output, "\n")
	var results []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		if relPath, err := filepath.Rel(path, line); err == nil && !strings.HasPrefix(relPath, "..") {
			results = append(results, relPath)
		} else {
			results = append(results, line)
		}
	}

	if len(results) == 0 {
		return t.noMatchesResult(call, name, path, maxDepth)
	}

	truncated := false
	if len(results) > 100 {
		results = results[:100]
		truncated = true
	}

	content := fmt.Sprintf("Found %d matches", len(results))
	if truncated {
		content += " (showing first 100)"
	}
	content += ":\n" + strings.Join(results, "\n")

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"pattern":   name,
			"path":      path,
			"matches":   len(results),
			"max_depth": maxDepth,
			"truncated": truncated,
			"results":   results,
		},
	}, nil
}

func (t *find) noMatchesResult(call ports.ToolCall, name, path string, maxDepth int) (*ports.ToolResult, error) {
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "No matches found",
		Metadata: map[string]any{
			"pattern":   name,
			"path":      path,
			"matches":   0,
			"max_depth": maxDepth,
		},
	}, nil
}

func (t *find) buildArgs(call ports.ToolCall, path string, maxDepth int, name string) []string {
	args := []string{path, "-maxdepth", fmt.Sprintf("%d", maxDepth)}
	if fileType, ok := call.Arguments["type"].(string); ok && fileType != "" {
		args = append(args, "-type", fileType)
	}
	args = append(args, "-name", name)
	return args
}

func (t *find) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "find",
		Description: "Find files and directories by name or pattern using the find command. Supports wildcards and limits results to maximum 100 matches.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"name": {
					Type:        "string",
					Description: "The name pattern to search for (supports wildcards like *.go)",
				},
				"path": {
					Type:        "string",
					Description: "Path to search in (default: current directory)",
				},
				"type": {
					Type:        "string",
					Description: "Type of files to find: 'f' for files, 'd' for directories",
					Enum:        []any{"f", "d"},
				},
				"max_depth": {
					Type:        "number",
					Description: "Maximum depth to search (default: 10)",
				},
			},
			Required: []string{"name"},
		},
	}
}

func (t *find) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "find",
		Version:  "1.0.0",
		Category: "search",
		Tags:     []string{"filesystem", "search", "files"},
	}
}
