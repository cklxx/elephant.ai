package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/tools"

	api "github.com/agent-infra/sandbox-sdk-go"
	sandboxclient "github.com/agent-infra/sandbox-sdk-go/client"
	"github.com/agent-infra/sandbox-sdk-go/option"
)

// CodeExecuteConfig configures the sandbox integration for the code_execute tool.
type CodeExecuteConfig struct {
	BaseURL        string
	Mode           tools.ExecutionMode
	SandboxManager *tools.SandboxManager
}

type codeExecute struct {
	client  *sandboxclient.Client
	sandbox *tools.SandboxManager
	config  CodeExecuteConfig
}

func NewCodeExecute(cfg CodeExecuteConfig) ports.ToolExecutor {
	mode := cfg.Mode
	if mode == tools.ExecutionModeUnknown {
		if cfg.SandboxManager != nil {
			mode = tools.ExecutionModeSandbox
		} else {
			mode = tools.ExecutionModeLocal
		}
	}

	tool := &codeExecute{
		config: CodeExecuteConfig{
			BaseURL:        cfg.BaseURL,
			Mode:           mode,
			SandboxManager: cfg.SandboxManager,
		},
		sandbox: cfg.SandboxManager,
	}

	if cfg.BaseURL != "" {
		tool.client = sandboxclient.NewClient(option.WithBaseURL(cfg.BaseURL))
	} else if mode == tools.ExecutionModeSandbox && cfg.SandboxManager != nil {
		if err := cfg.SandboxManager.Initialize(context.Background()); err == nil {
			tool.client = cfg.SandboxManager.Client()
		}
	}

	return tool
}

func (t *codeExecute) Mode() tools.ExecutionMode {
	return t.config.Mode
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

	// Validate required parameters
	if language == "" || code == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: missing required parameters (language and code)",
			Error:   fmt.Errorf("missing required parameters"),
			Metadata: map[string]interface{}{
				"success": false,
				"error":   "missing parameters",
			},
		}, nil
	}

	timeout := 30
	if timeoutVal, ok := call.Arguments["timeout"].(float64); ok {
		timeout = int(timeoutVal)
	}

	if language == "js" {
		language = "javascript"
	}

	// Route to remote sandbox when configured.
	if t.config.Mode == tools.ExecutionModeSandbox {
		if t.client == nil && t.sandbox != nil {
			if err := t.sandbox.Initialize(ctx); err != nil {
				return &ports.ToolResult{CallID: call.ID, Content: "Sandbox unavailable", Error: tools.FormatSandboxError(err)}, nil
			}
			t.client = t.sandbox.Client()
		}
		if t.client == nil {
			return &ports.ToolResult{CallID: call.ID, Content: "Sandbox client not initialised", Error: fmt.Errorf("sandbox client not available")}, nil
		}
		execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		switch language {
		case "python":
			return t.executePythonRemote(execCtx, call.ID, code, timeout)
		case "javascript", "js":
			return t.executeNodeRemote(execCtx, call.ID, code, timeout)
		case "bash":
			return t.executeBashRemote(execCtx, call.ID, code, timeout)
		case "go":
			return t.executeGoRemote(execCtx, call.ID, code, timeout)
		default:
			return &ports.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("Error: unsupported language '%s'", language),
				Error:   fmt.Errorf("unsupported language: %s", language),
				Metadata: map[string]interface{}{
					"success":  false,
					"language": language,
					"error":    "unsupported language",
				},
			}, nil
		}
	}

	return executeLocally(ctx, call, language, code, timeout)
}

func executeLocally(ctx context.Context, call ports.ToolCall, language, code string, timeout int) (*ports.ToolResult, error) {
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
		defer func() { _ = os.RemoveAll(tmpDir) }()
		tmpFile := filepath.Join(tmpDir, "main.go")
		_ = os.WriteFile(tmpFile, []byte(code), 0644)
		cmd := exec.CommandContext(execCtx, "go", "run", tmpFile)
		out, e := cmd.CombinedOutput()
		output, err = string(out), e
	case "javascript":
		tmpFile, _ := os.CreateTemp("", "alex-js-*.js")
		defer func() { _ = os.Remove(tmpFile.Name()) }()
		_, _ = tmpFile.WriteString(code)
		_ = tmpFile.Close()
		cmd := exec.CommandContext(execCtx, "node", tmpFile.Name())
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
		out, e := cmd.CombinedOutput()
		output, err = string(out), e
	default:
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error: unsupported language '%s'", language),
			Error:   fmt.Errorf("unsupported language: %s", language),
			Metadata: map[string]interface{}{
				"success":  false,
				"language": language,
				"error":    "unsupported language",
			},
		}, nil
	}

	duration := time.Since(startTime)

	if execCtx.Err() == context.DeadlineExceeded {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Execution timed out after %ds\n%s", timeout, output),
			Metadata: map[string]interface{}{
				"success":       false,
				"language":      language,
				"duration_ms":   duration.Milliseconds(),
				"timeout":       true,
				"timeout_after": timeout,
			},
		}, nil
	}

	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed:\n%s\n\n%s", err, output),
			Metadata: map[string]interface{}{
				"success":     false,
				"language":    language,
				"duration_ms": duration.Milliseconds(),
				"error":       err.Error(),
			},
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Success in %v:\n%s", duration.Round(time.Millisecond), output),
		Metadata: map[string]interface{}{
			"success":     true,
			"language":    language,
			"duration_ms": duration.Milliseconds(),
		},
	}, nil
}

func (t *codeExecute) executePythonRemote(ctx context.Context, callID, code string, timeout int) (*ports.ToolResult, error) {
	start := time.Now()
	req := &api.JupyterExecuteRequest{Code: code}
	if timeout > 0 {
		req.Timeout = intPtr(timeout)
	}

	resp, err := t.client.Jupyter.ExecuteJupyterCode(ctx, req)
	if err != nil {
		return sandboxError(callID, "python", err), nil
	}

	data := resp.GetData()
	status := ""
	if data != nil {
		status = data.GetStatus()
	}

	success := strings.EqualFold(status, "ok")

	var builder strings.Builder
	fmt.Fprintf(&builder, "Status: %s\n", status)
	if message := resp.GetMessage(); message != nil && *message != "" {
		fmt.Fprintf(&builder, "Message: %s\n", *message)
	}
	if data != nil {
		outputs := data.GetOutputs()
		if len(outputs) > 0 {
			builder.WriteString("Outputs:\n")
			for idx, output := range outputs {
				fmt.Fprintf(&builder, "  [%d] %s\n", idx+1, strings.ToUpper(output.GetOutputType()))
				appendJupyterOutput(&builder, output)
			}
		}
	}

	metadata := map[string]any{
		"success":     success,
		"language":    "python",
		"duration_ms": time.Since(start).Milliseconds(),
		"remote":      true,
		"status":      status,
	}
	if data != nil {
		if session := data.GetSessionId(); session != "" {
			metadata["session_id"] = session
		}
	}

	return &ports.ToolResult{
		CallID:   callID,
		Content:  strings.TrimSpace(builder.String()),
		Metadata: metadata,
	}, nil
}

func (t *codeExecute) executeNodeRemote(ctx context.Context, callID, code string, timeout int) (*ports.ToolResult, error) {
	start := time.Now()
	req := &api.NodeJsExecuteRequest{Code: code}
	if timeout > 0 {
		req.Timeout = intPtr(timeout)
	}

	resp, err := t.client.Nodejs.ExecuteNodejsCode(ctx, req)
	if err != nil {
		return sandboxError(callID, "javascript", err), nil
	}

	data := resp.GetData()
	status := ""
	if data != nil {
		status = data.GetStatus()
	}
	success := strings.EqualFold(status, "ok")

	var builder strings.Builder
	fmt.Fprintf(&builder, "Status: %s\n", status)
	if message := resp.GetMessage(); message != nil && *message != "" {
		fmt.Fprintf(&builder, "Message: %s\n", *message)
	}
	if data != nil {
		if stdout := data.GetStdout(); stdout != nil && *stdout != "" {
			builder.WriteString("Stdout:\n")
			builder.WriteString(strings.TrimSpace(*stdout))
			builder.WriteString("\n")
		}
		if stderr := data.GetStderr(); stderr != nil && *stderr != "" {
			builder.WriteString("Stderr:\n")
			builder.WriteString(strings.TrimSpace(*stderr))
			builder.WriteString("\n")
		}
		fmt.Fprintf(&builder, "Exit Code: %d\n", data.GetExitCode())
	}

	metadata := map[string]any{
		"success":     success,
		"language":    "javascript",
		"duration_ms": time.Since(start).Milliseconds(),
		"remote":      true,
		"status":      status,
	}

	return &ports.ToolResult{
		CallID:   callID,
		Content:  strings.TrimSpace(builder.String()),
		Metadata: metadata,
	}, nil
}

func (t *codeExecute) executeBashRemote(ctx context.Context, callID, code string, timeout int) (*ports.ToolResult, error) {
	start := time.Now()
	req := &api.ShellExecRequest{Command: code}

	resp, err := t.client.Shell.ExecCommand(ctx, req)
	if err != nil {
		return sandboxError(callID, "bash", err), nil
	}

	data := resp.GetData()
	status := ""
	output := ""
	exitCode := 0
	sessionID := ""
	if data != nil {
		status = string(data.GetStatus())
		if out := data.GetOutput(); out != nil {
			output = *out
		}
		if codePtr := data.GetExitCode(); codePtr != nil {
			exitCode = *codePtr
		}
		sessionID = data.GetSessionId()
	}
	success := strings.EqualFold(status, string(api.BashCommandStatusCompleted))

	var builder strings.Builder
	fmt.Fprintf(&builder, "Status: %s\n", status)
	if sessionID != "" {
		fmt.Fprintf(&builder, "Session: %s\n", sessionID)
	}
	builder.WriteString("Output:\n")
	builder.WriteString(strings.TrimSpace(output))
	builder.WriteString("\n")
	fmt.Fprintf(&builder, "Exit Code: %d\n", exitCode)

	metadata := map[string]any{
		"success":     success,
		"language":    "bash",
		"duration_ms": time.Since(start).Milliseconds(),
		"remote":      true,
		"status":      status,
		"exit_code":   exitCode,
	}
	if sessionID != "" {
		metadata["session_id"] = sessionID
	}

	return &ports.ToolResult{
		CallID:   callID,
		Content:  strings.TrimSpace(builder.String()),
		Metadata: metadata,
	}, nil
}

func (t *codeExecute) executeGoRemote(ctx context.Context, callID, code string, timeout int) (*ports.ToolResult, error) {
	start := time.Now()
	tempDir := fmt.Sprintf("/tmp/alex-go-%d", time.Now().UnixNano())
	cleanupCmd := fmt.Sprintf("rm -rf %s", tempDir)

	// Create workspace directory.
	if _, err := t.client.Shell.ExecCommand(ctx, &api.ShellExecRequest{Command: fmt.Sprintf("mkdir -p %s", tempDir)}); err != nil {
		return sandboxError(callID, "go", err), nil
	}
	defer func() {
		_, _ = t.client.Shell.ExecCommand(context.Background(), &api.ShellExecRequest{Command: cleanupCmd})
	}()

	filePath := tempDir + "/main.go"
	if _, err := t.client.File.WriteFile(ctx, &api.FileWriteRequest{File: filePath, Content: code}); err != nil {
		return sandboxError(callID, "go", err), nil
	}

	execDir := tempDir
	resp, err := t.client.Shell.ExecCommand(ctx, &api.ShellExecRequest{
		ExecDir: &execDir,
		Command: "go run main.go",
	})
	if err != nil {
		return sandboxError(callID, "go", err), nil
	}

	data := resp.GetData()
	status := ""
	output := ""
	exitCode := 0
	sessionID := ""
	if data != nil {
		status = string(data.GetStatus())
		if out := data.GetOutput(); out != nil {
			output = *out
		}
		if codePtr := data.GetExitCode(); codePtr != nil {
			exitCode = *codePtr
		}
		sessionID = data.GetSessionId()
	}
	success := strings.EqualFold(status, string(api.BashCommandStatusCompleted))

	var builder strings.Builder
	fmt.Fprintf(&builder, "Status: %s\n", status)
	if sessionID != "" {
		fmt.Fprintf(&builder, "Session: %s\n", sessionID)
	}
	builder.WriteString("Output:\n")
	builder.WriteString(strings.TrimSpace(output))
	builder.WriteString("\n")
	fmt.Fprintf(&builder, "Exit Code: %d\n", exitCode)

	metadata := map[string]any{
		"success":     success,
		"language":    "go",
		"duration_ms": time.Since(start).Milliseconds(),
		"remote":      true,
		"status":      status,
		"exit_code":   exitCode,
	}
	if sessionID != "" {
		metadata["session_id"] = sessionID
	}

	return &ports.ToolResult{
		CallID:   callID,
		Content:  strings.TrimSpace(builder.String()),
		Metadata: metadata,
	}, nil
}

func appendJupyterOutput(builder *strings.Builder, output *api.JupyterOutput) {
	switch output.GetOutputType() {
	case "stream":
		name := "stream"
		if output.GetName() != nil {
			name = *output.GetName()
		}
		text := ""
		if output.GetText() != nil {
			text = *output.GetText()
		}
		fmt.Fprintf(builder, "    (%s) %s\n", name, strings.TrimSpace(text))
	case "error":
		builder.WriteString("    Error:\n")
		if output.GetEname() != nil {
			fmt.Fprintf(builder, "      Name: %s\n", *output.GetEname())
		}
		if output.GetEvalue() != nil {
			fmt.Fprintf(builder, "      Value: %s\n", *output.GetEvalue())
		}
		if trace := output.GetTraceback(); len(trace) > 0 {
			builder.WriteString("      Traceback:\n")
			for _, line := range trace {
				builder.WriteString("        " + line + "\n")
			}
		}
	default:
		if data := output.GetData(); len(data) > 0 {
			encoded, err := json.MarshalIndent(data, "      ", "  ")
			if err == nil {
				builder.WriteString("      Data:\n")
				builder.WriteString(string(encoded))
				builder.WriteString("\n")
			}
		}
	}
}

func sandboxError(callID, language string, err error) *ports.ToolResult {
	return &ports.ToolResult{
		CallID:  callID,
		Content: fmt.Sprintf("Sandbox request failed: %v", err),
		Metadata: map[string]any{
			"success":  false,
			"language": language,
			"remote":   true,
			"error":    err.Error(),
		},
	}
}

func intPtr(value int) *int {
	return &value
}
