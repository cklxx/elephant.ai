package builtin

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/agent/ports"
)

// CodeExecuteConfig is reserved for future configuration options.
type CodeExecuteConfig struct{}

type codeExecute struct {
}

func NewCodeExecute(cfg CodeExecuteConfig) ports.ToolExecutor {
	_ = cfg
	return &codeExecute{}
}

func (t *codeExecute) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:      "code_execute",
		Version:   "1.0.0",
		Category:  "execution",
		Tags:      []string{"code", "execute"},
		Dangerous: true,
	}
}

func (t *codeExecute) Definition() ports.ToolDefinition {
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

func (t *codeExecute) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	language, _ := call.Arguments["language"].(string)
	code, _ := call.Arguments["code"].(string)
	codePath, _ := call.Arguments["code_path"].(string)

	resolver := GetPathResolverFromContext(ctx)
	workingDir := resolver.ResolvePath(".")
	extraMeta := map[string]any{}

	if codePath != "" {
		resolved := resolver.ResolvePath(codePath)
		content, err := os.ReadFile(resolved)
		if err != nil {
			return &ports.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("Error: failed to read code_path %s: %v", codePath, err),
				Error:   fmt.Errorf("failed to read code_path: %w", err),
				Metadata: map[string]any{
					"success":         false,
					"language":        language,
					"code_path":       codePath,
					"resolved_path":   resolved,
					"working_dir":     workingDir,
					"error":           "code_path_unreadable",
					"code_provenance": "file",
				},
			}, nil
		}
		code = string(content)
		extraMeta["code_path"] = codePath
		extraMeta["resolved_path"] = resolved
		extraMeta["code_provenance"] = "file"
	}

	if strings.HasPrefix(strings.TrimSpace(code), "data:") {
		if decoded, err := decodeDataURIString(code); err == nil {
			code = decoded
			extraMeta["code_provenance"] = "attachment"
		}
	}

	code = strings.TrimRight(code, "\n")

	// Validate required parameters
	if language == "" || strings.TrimSpace(code) == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: missing required parameters (language with code or code_path)",
			Error:   fmt.Errorf("missing required parameters"),
			Metadata: mergeMetadata(map[string]any{
				"success": false,
				"error":   "missing parameters",
			}, extraMeta),
		}, nil
	}

	timeout := 30
	if timeoutVal, ok := call.Arguments["timeout"].(float64); ok {
		timeout = int(timeoutVal)
	}
	if timeout < 1 {
		timeout = 1
	}
	if timeout > 300 {
		timeout = 300
	}

	if language == "js" {
		language = "javascript"
	}

	if workingDir != "" {
		extraMeta["working_dir"] = workingDir
	}

	result, err := executeLocally(ctx, call, language, code, timeout, workingDir, extraMeta)
	if err != nil || result == nil {
		return result, err
	}
	result.Metadata = mergeMetadata(result.Metadata, extraMeta)
	return result, nil
}

func executeLocally(ctx context.Context, call ports.ToolCall, language, code string, timeout int, workingDir string, extraMeta map[string]any) (*ports.ToolResult, error) {
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	startTime := time.Now()
	var output string
	var err error

	switch language {
	case "python":
		cmd := exec.CommandContext(execCtx, "python3", "-c", code)
		if workingDir != "" {
			cmd.Dir = workingDir
		}
		out, e := cmd.CombinedOutput()
		output, err = string(out), e
	case "go":
		tmpDir, _ := os.MkdirTemp("", "alex-go-*")
		defer func() { _ = os.RemoveAll(tmpDir) }()
		tmpFile := filepath.Join(tmpDir, "main.go")
		_ = os.WriteFile(tmpFile, []byte(code), 0644)
		cmd := exec.CommandContext(execCtx, "go", "run", tmpFile)
		if workingDir != "" {
			cmd.Dir = workingDir
		}
		out, e := cmd.CombinedOutput()
		output, err = string(out), e
	case "javascript":
		tmpFile, _ := os.CreateTemp("", "alex-js-*.js")
		defer func() { _ = os.Remove(tmpFile.Name()) }()
		_, _ = tmpFile.WriteString(code)
		_ = tmpFile.Close()
		cmd := exec.CommandContext(execCtx, "node", tmpFile.Name())
		if workingDir != "" {
			cmd.Dir = workingDir
		}
		out, e := cmd.CombinedOutput()
		output, err = string(out), e
	case "bash":
		tmpFile, _ := os.CreateTemp("", "alex-bash-*.sh")
		defer func() { _ = os.Remove(tmpFile.Name()) }()
		if !strings.HasPrefix(code, "#!") {
			code = "#!/bin/bash\n" + code
		}
		_, _ = tmpFile.WriteString(code)
		_ = tmpFile.Close()
		_ = os.Chmod(tmpFile.Name(), 0755)
		cmd := exec.CommandContext(execCtx, "bash", tmpFile.Name())
		if workingDir != "" {
			cmd.Dir = workingDir
		}
		out, e := cmd.CombinedOutput()
		output, err = string(out), e
	default:
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error: unsupported language '%s'", language),
			Error:   fmt.Errorf("unsupported language: %s", language),
			Metadata: mergeMetadata(map[string]interface{}{
				"success":  false,
				"language": language,
				"error":    "unsupported language",
			}, extraMeta),
		}, nil
	}

	duration := time.Since(startTime)

	if execCtx.Err() == context.DeadlineExceeded {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Execution timed out after %ds\n%s", timeout, output),
			Metadata: mergeMetadata(map[string]interface{}{
				"success":       false,
				"language":      language,
				"duration_ms":   duration.Milliseconds(),
				"timeout":       true,
				"timeout_after": timeout,
			}, extraMeta),
		}, nil
	}

	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed:\n%s\n\n%s", err, output),
			Metadata: mergeMetadata(map[string]interface{}{
				"success":     false,
				"language":    language,
				"duration_ms": duration.Milliseconds(),
				"error":       err.Error(),
			}, extraMeta),
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Success in %v:\n%s", duration.Round(time.Millisecond), output),
		Metadata: mergeMetadata(map[string]interface{}{
			"success":     true,
			"language":    language,
			"duration_ms": duration.Milliseconds(),
		}, extraMeta),
	}, nil
}

func mergeMetadata(base, extra map[string]any) map[string]any {
	if base == nil && extra == nil {
		return nil
	}
	if base == nil {
		base = map[string]any{}
	}
	for k, v := range extra {
		base[k] = v
	}
	return base
}

func decodeDataURIString(dataURI string) (string, error) {
	comma := strings.Index(dataURI, ",")
	if !strings.HasPrefix(dataURI, "data:") || comma == -1 {
		return "", fmt.Errorf("invalid data URI")
	}
	meta := dataURI[5:comma]
	payload := dataURI[comma+1:]

	if strings.HasSuffix(meta, ";base64") {
		encoded := payload
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}

	unescaped, err := url.QueryUnescape(payload)
	if err != nil {
		return "", err
	}
	return unescaped, nil
}
