package builtin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"alex/internal/tools"
	"alex/internal/utils"
)

// BashTool implements shell command execution functionality
type BashTool struct{}

func CreateBashTool() *BashTool {
	return &BashTool{}
}

func (t *BashTool) Name() string {
	return "bash"
}

func (t *BashTool) Description() string {
	return `Execute shell commands with comprehensive security validation and output capture.

Usage:
- Executes commands via system shell (sh on Unix, cmd on Windows)
- Captures both stdout and stderr with proper formatting
- Built-in security validation prevents dangerous operations
- Supports custom working directory and timeout controls
- Automatically formats diff-like output for readability

Parameters:
- command: Shell command to execute (required)
- working_dir: Directory to run command in (optional)
- timeout: Maximum execution time in seconds (default: 30, max: 300)

Security Features:
- Blocks dangerous commands (rm -rf /, dd, mkfs, etc.)
- Prevents access to sensitive paths (/etc/, /root/, etc.)
- Restricts networking commands (ssh, scp, nmap, etc.)
- Validates command length (max 1000 characters)
- Detects suspicious patterns and command injection attempts

Output Handling:
- Returns combined stdout/stderr if both present
- Applies syntax highlighting to diff output
- Shows execution time and exit code
- Handles timeouts gracefully
- Distinguishes between command success/failure

Common Usage Patterns:
- Build commands: "make build", "npm install", "go build"
- File operations: "ls -la", "find . -name '*.go'"
- Git operations: "git status", "git diff", "git add ."
- Test commands: "go test ./...", "npm test"

Note: Commands are executed with current user permissions. Interactive commands requiring input are not supported in default mode.`
}

func (t *BashTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"working_dir": map[string]interface{}{
				"type":        "string",
				"description": "Working directory for the command",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds",
				"default":     30,
				"minimum":     1,
				"maximum":     300,
			},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddStringField("command", "The shell command to execute").
		AddOptionalStringField("working_dir", "Working directory for the command").
		AddOptionalIntField("timeout", "Timeout in seconds", 1, 300)

	// First run standard validation
	if err := validator.Validate(args); err != nil {
		return err
	}

	// Get validated command
	command := args["command"].(string)

	// Enhanced security validation
	if err := t.validateSecurity(command); err != nil {
		return err
	}

	// Validate working directory if provided
	if workingDir, ok := args["working_dir"]; ok && workingDir != nil {
		if workingDirStr, ok := workingDir.(string); ok && workingDirStr != "" {
			if _, err := os.Stat(workingDirStr); os.IsNotExist(err) {
				return fmt.Errorf("working directory does not exist: %s", workingDirStr)
			}
		}
	}

	return nil
}

func (t *BashTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// 防御性检查：确保参数存在且有效（通常已通过Validate验证）
	if args == nil {
		return nil, fmt.Errorf("arguments cannot be nil")
	}
	
	commandValue, exists := args["command"]
	if !exists {
		return nil, fmt.Errorf("command parameter is required")
	}
	
	command, ok := commandValue.(string)
	if !ok {
		return nil, fmt.Errorf("command must be a string")
	}
	
	if command == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// Get optional parameters
	workingDir := ""
	if wd, ok := args["working_dir"]; ok {
		workingDir, _ = wd.(string)
	}

	timeout := 30
	if timeoutArg, ok := args["timeout"]; ok {
		if timeoutFloat, ok := timeoutArg.(float64); ok {
			timeout = int(timeoutFloat)
		}
	}

	captureOutput := true
	if captureArg, ok := args["capture_output"]; ok {
		captureOutput, _ = captureArg.(bool)
	}

	allowInteractive := false
	if interactiveArg, ok := args["allow_interactive"]; ok {
		allowInteractive, _ = interactiveArg.(bool)
	}

	// Create command context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Determine shell command based on OS
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(cmdCtx, "sh", "-c", command)
	}

	// Set working directory if specified
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// Prepare for output capture
	var stdout, stderr strings.Builder
	var exitCode int

	startTime := time.Now()

	if captureOutput {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	} else if !allowInteractive {
		// If not capturing output and not interactive, discard output
		cmd.Stdout = nil
		cmd.Stderr = nil
	} else {
		// Interactive mode - connect to terminal
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	// Execute command
	err := cmd.Run()
	duration := time.Since(startTime)

	// Get exit code
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	// Prepare result
	var resultContent string
	success := err == nil

	if captureOutput {
		stdoutStr := stdout.String()
		stderrStr := stderr.String()

		// Apply diff formatting if this looks like diff output
		if stdoutStr != "" && utils.IsDiffOutput(stdoutStr) {
			stdoutStr = utils.FormatDiffOutput(stdoutStr)
		}
		if stderrStr != "" && utils.IsDiffOutput(stderrStr) {
			stderrStr = utils.FormatDiffOutput(stderrStr)
		}

		if stdoutStr != "" && stderrStr != "" {
			resultContent = fmt.Sprintf("STDOUT:\n%s\n\nSTDERR:\n%s", stdoutStr, stderrStr)
		} else if stdoutStr != "" {
			resultContent = stdoutStr
		} else if stderrStr != "" {
			resultContent = stderrStr
		} else {
			resultContent = "Command executed successfully (no output)"
		}
	} else {
		if success {
			resultContent = "Command executed successfully"
		} else {
			resultContent = fmt.Sprintf("Command failed: %v", err)
		}
	}

	// Handle context cancellation (timeout)
	if cmdCtx.Err() == context.DeadlineExceeded {
		resultContent += fmt.Sprintf("\n\nCommand timed out after %d seconds", timeout)
		success = false
	}

	return &ToolResult{
		Content: resultContent,
		Data: map[string]interface{}{
			"command":     command,
			"exit_code":   exitCode,
			"success":     success,
			"duration_ms": duration.Milliseconds(),
			"working_dir": workingDir,
			"stdout":      stdout.String(),
			"stderr":      stderr.String(),
		},
	}, nil
}

// validateSecurity performs comprehensive security validation on commands
func (t *BashTool) validateSecurity(command string) error {
	lowerCommand := strings.ToLower(strings.TrimSpace(command))

	// Dangerous commands that could harm the system
	dangerousCommands := []string{
		"rm -rf /", "rm -rf .", "rm -rf *", "rm -rf ~",
		"dd if=", "mkfs", "fdisk", "format", "diskpart",
		"del /s", "rmdir /s", "rd /s",
		"shutdown", "reboot", "halt", "poweroff", "init 0", "init 6",
		"killall", "pkill -9", "kill -9",
		"chmod 777 /", "chown root /",
		"mv / ", "cp -r / ",
		"cat /dev/urandom", "> /dev/sda", "> /dev/null",
		":(){ :|:& };:", // fork bomb
	}

	for _, dangerous := range dangerousCommands {
		if strings.Contains(lowerCommand, dangerous) {
			return fmt.Errorf("dangerous command detected: %s", dangerous)
		}
	}

	// Suspicious patterns that warrant extra scrutiny
	suspiciousPatterns := []string{
		"/etc/passwd", "/etc/shadow", "/etc/sudoers",
		"sudo su", "sudo -i", "sudo bash", "sudo sh",
		"nc -l", "netcat -l", "socat",
		"wget http", "curl http", "curl ftp",
		"python -c", "python3 -c", "perl -e",
		"base64 -d", "echo | sh", "eval ",
		"$(", "`", // command substitution
		"&& rm", "|| rm", "; rm",
		"chmod +x", "chmod 755", "chmod 777",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerCommand, pattern) {
			return fmt.Errorf("potentially dangerous pattern detected: %s", pattern)
		}
	}

	// Check for attempts to access sensitive directories
	restrictedPaths := []string{
		"/etc/", "/root/", "/boot/", "/sys/", "/proc/",
		"/var/log/", "/usr/bin/", "/usr/sbin/", "/sbin/",
		"c:\\windows\\", "c:\\program files\\", "c:\\system32\\",
	}

	for _, path := range restrictedPaths {
		if strings.Contains(lowerCommand, path) {
			return fmt.Errorf("access to restricted path detected: %s", path)
		}
	}

	// Check for networking commands that could be used maliciously
	networkCommands := []string{
		"ssh", "scp", "rsync", "ftp", "sftp",
		"telnet", "nmap", "ping -f", "ping -c 1000",
		"iptables", "ufw", "firewall-cmd",
	}

	for _, netCmd := range networkCommands {
		if strings.Contains(lowerCommand, netCmd) {
			return fmt.Errorf("network command requires explicit permission: %s", netCmd)
		}
	}

	// Check command length to prevent buffer overflow attempts
	if len(command) > 1000 {
		return fmt.Errorf("command too long (max 1000 characters)")
	}

	return nil
}

// CodeExecutorTool implements the CodeActExecutor as a tool
type CodeExecutorTool struct {
	executor *tools.CodeActExecutor
}

func CreateCodeExecutorTool() *CodeExecutorTool {
	return &CodeExecutorTool{
		executor: tools.NewCodeActExecutor(),
	}
}

func (t *CodeExecutorTool) Name() string {
	return "code_execute"
}

func (t *CodeExecutorTool) Description() string {
	return `Execute code in multiple programming languages with sandboxed execution and timeout controls.

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
- Safe for testing code snippets and algorithms

Example Usage:
- Python: Test data processing scripts, algorithms
- Go: Compile and test Go functions, packages
- JavaScript: Run Node.js scripts, test JavaScript logic
- Bash: Execute shell scripts safely

Output Format:
- Success: Shows execution time and program output
- Failure: Displays compilation/runtime errors
- Timeout: Indicates if execution exceeded time limit

Note: This tool is designed for code testing and prototyping. Production code should be executed through proper deployment processes.`
}

func (t *CodeExecutorTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"language": map[string]interface{}{
				"type":        "string",
				"description": "Programming language to execute",
				"enum":        []string{"python", "go", "javascript", "js", "bash"},
			},
			"code": map[string]interface{}{
				"type":        "string",
				"description": "Source code to execute",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds (default: 30)",
				"default":     30,
				"minimum":     1,
				"maximum":     300,
			},
		},
		"required": []string{"language", "code"},
	}
}

func (t *CodeExecutorTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddCustomValidator("language", "Programming language to execute", true, func(value interface{}) error {
			language, ok := value.(string)
			if !ok {
				return fmt.Errorf("language must be a string")
			}
			supportedLangs := []string{"python", "go", "javascript", "js", "bash"}
			for _, lang := range supportedLangs {
				if language == lang {
					return nil
				}
			}
			return fmt.Errorf("unsupported language: %s", language)
		}).
		AddStringField("code", "Source code to execute").
		AddOptionalIntField("timeout", "Timeout in seconds", 1, 300)

	return validator.Validate(args)
}

func (t *CodeExecutorTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// 防御性检查：确保参数存在且有效（通常已通过Validate验证）
	if args == nil {
		return nil, fmt.Errorf("arguments cannot be nil")
	}
	
	languageValue, exists := args["language"]
	if !exists {
		return nil, fmt.Errorf("language parameter is required")
	}
	
	language, ok := languageValue.(string)
	if !ok {
		return nil, fmt.Errorf("language must be a string")
	}
	
	codeValue, exists := args["code"]
	if !exists {
		return nil, fmt.Errorf("code parameter is required")
	}
	
	code, ok := codeValue.(string)
	if !ok {
		return nil, fmt.Errorf("code must be a string")
	}

	// Set timeout if provided
	if timeoutArg, ok := args["timeout"]; ok {
		if timeoutFloat, ok := timeoutArg.(float64); ok {
			timeout := time.Duration(timeoutFloat) * time.Second
			t.executor.SetTimeout(timeout)
		}
	}

	// Execute the code
	result, err := t.executor.ExecuteCode(ctx, language, code)
	if err != nil {
		return nil, fmt.Errorf("failed to execute code: %w", err)
	}

	// Prepare content
	var content string
	if result.Success {
		if result.Output != "" {
			content = fmt.Sprintf("Code executed successfully in %v:\n\n  %s", result.ExecutionTime, result.Output)
		} else {
			content = fmt.Sprintf("Code executed successfully in %v (no output)", result.ExecutionTime)
		}
	} else {
		content = fmt.Sprintf("Code execution failed:\n\n%s", result.Error)
	}

	return &ToolResult{
		Content: content,
		Data: map[string]interface{}{
			"success":        result.Success,
			"output":         result.Output,
			"error":          result.Error,
			"exit_code":      result.ExitCode,
			"execution_time": result.ExecutionTime.Milliseconds(),
			"language":       result.Language,
			"code":           result.Code,
		},
	}, nil
}
