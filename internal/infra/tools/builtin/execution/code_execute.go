//go:build no_local_exec

package execution

import (
	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
	"context"
	"fmt"
)

// CodeExecuteConfig is reserved for future configuration options.
type CodeExecuteConfig struct{}

type codeExecute struct {
	shared.BaseTool
}

func NewCodeExecute(cfg CodeExecuteConfig) tools.ToolExecutor {
	_ = cfg
	return &codeExecute{
		BaseTool: shared.NewBaseTool(
			codeExecuteDefinition(),
			codeExecuteMetadata(),
		),
	}
}

func (t *codeExecute) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	err := fmt.Errorf("local code execution is disabled in this build; rebuild without -tags=no_local_exec to enable")
	return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
}

func codeExecuteDefinition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "code_execute",
		Description: `Execute code in multiple programming languages with local execution and timeout controls.

Supported Languages:
- Python: Executes Python code with standard library access
- Go: Compiles and runs Go code with basic packages
- JavaScript: Runs JavaScript via Node.js runtime
- Bash: Executes shell scripts in controlled environment

Usage:
- Runs code directly on the host machine (treat as dangerous)
- Captures both stdout and any runtime errors
- Shows execution time and exit codes
- Automatically handles compilation for compiled languages

Parameters:
- language: Programming language ("python", "go", "javascript"/"js", "bash")
- code: Inline source code to execute (mutually optional with code_path)
- code_path: Workspace-relative path to a source file to execute
- timeout: Maximum execution time in seconds (default: 30, max: 300)

Safety:
- Configurable timeout to prevent infinite loops
- Prefer running in trusted/local development environments`,
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
					Description: "Source code to execute (ignored when code_path is provided)",
				},
				"code_path": {
					Type:        "string",
					Description: "Workspace-relative path to a source file to execute",
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

func codeExecuteMetadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:      "code_execute",
		Version:   "1.0.0",
		Category:  "execution",
		Tags:      []string{"code", "execute"},
		Dangerous: true,
	}
}
