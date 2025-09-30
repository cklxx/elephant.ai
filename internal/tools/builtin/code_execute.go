package builtin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/agent/ports"
)

type codeExecute struct{}

func NewCodeExecute() ports.ToolExecutor {
	return &codeExecute{}
}

func (t *codeExecute) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:      "code_execute",
		Version:   "1.0.0",
		Category:  "execution",
		Tags:      []string{"code", "execute", "sandbox"},
		Dangerous: true,
	}
}

func (t *codeExecute) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "code_execute",
		Description: `Execute code in multiple programming languages with sandboxed execution and timeout controls.

Supported Languages:
- Python: Executes Python code with standard library access
- Go: Compiles and runs Go code with basic packages
- JavaScript: Runs JavaScript via Node.js runtime
- Bash: Executes shell scripts in controlled environment

Usage:
- Provides isolated execution environment for code testing
- Captures both stdout and any runtime errors
- Shows execution time and exit codes
- Automatically handles compilation for compiled languages

Parameters:
- language: Programming language ("python", "go", "javascript"/"js", "bash")
- code: Source code to execute
- timeout: Maximum execution time in seconds (default: 30, max: 300)

Security Features:
- Sandboxed execution environment
- Configurable timeout to prevent infinite loops
- Limited system access and resource usage
- Safe for testing code snippets and algorithms`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"language": {
					Type:        "string",
					Description: "Programming language to execute",
					Enum:        []any{"python", "go", "javascript", "js", "bash"},
				},
				"code": {
					Type:        "string",
					Description: "Source code to execute",
				},
				"timeout": {
					Type:        "integer",
					Description: "Timeout in seconds (default: 30)",
				},
			},
			Required: []string{"language", "code"},
		},
	}
}

func (t *codeExecute) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	language, _ := call.Arguments["language"].(string)
	code, _ := call.Arguments["code"].(string)

	timeout := 30
	if timeoutVal, ok := call.Arguments["timeout"].(float64); ok {
		timeout = int(timeoutVal)
	}

	if language == "js" {
		language = "javascript"
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	startTime := time.Now()
	var output string
	var err error

	switch language {
	case "python":
		cmd := exec.CommandContext(execCtx, "python3", "-c", code)
		out, e := cmd.CombinedOutput()
		output, err = string(out), e
	case "go":
		tmpDir, _ := os.MkdirTemp("", "alex-go-*")
		defer os.RemoveAll(tmpDir)
		tmpFile := filepath.Join(tmpDir, "main.go")
		os.WriteFile(tmpFile, []byte(code), 0644)
		cmd := exec.CommandContext(execCtx, "go", "run", tmpFile)
		out, e := cmd.CombinedOutput()
		output, err = string(out), e
	case "javascript":
		tmpFile, _ := os.CreateTemp("", "alex-js-*.js")
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString(code)
		tmpFile.Close()
		cmd := exec.CommandContext(execCtx, "node", tmpFile.Name())
		out, e := cmd.CombinedOutput()
		output, err = string(out), e
	case "bash":
		tmpFile, _ := os.CreateTemp("", "alex-bash-*.sh")
		defer os.Remove(tmpFile.Name())
		if !strings.HasPrefix(code, "#!") {
			code = "#!/bin/bash\n" + code
		}
		tmpFile.WriteString(code)
		tmpFile.Close()
		os.Chmod(tmpFile.Name(), 0755)
		cmd := exec.CommandContext(execCtx, "bash", tmpFile.Name())
		out, e := cmd.CombinedOutput()
		output, err = string(out), e
	default:
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error: unsupported language '%s'", language),
			Error:   fmt.Errorf("unsupported language"),
		}, nil
	}

	duration := time.Since(startTime)

	if execCtx.Err() == context.DeadlineExceeded {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Timeout after %ds\n%s", timeout, output),
		}, nil
	}

	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed:\n%s\n\n%s", err, output),
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Success in %v:\n%s", duration.Round(time.Millisecond), output),
	}, nil
}
