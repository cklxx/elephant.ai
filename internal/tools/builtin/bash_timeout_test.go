package builtin

import (
	"alex/internal/agent/ports"
	"alex/internal/tools"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func getTestSandboxURL() string {
	url := os.Getenv("ALEX_SANDBOX_BASE_URL")
	if url == "" {
		return "http://localhost:8090"
	}
	return url
}

// TestBashLongRunningCommand tests that long-running commands don't timeout
func TestBashLongRunningCommand(t *testing.T) {
	sandboxURL := getTestSandboxURL()
	if sandboxURL == "" {
		t.Skip("ALEX_SANDBOX_BASE_URL not set, skipping sandbox test")
	}

	sandbox := tools.NewSandboxManager(sandboxURL)
	tool := NewBash(ShellToolConfig{
		Mode:           tools.ExecutionModeSandbox,
		SandboxManager: sandbox,
	})

	// Test command that takes several seconds to complete
	call := ports.ToolCall{
		ID:   "test-long-cmd",
		Name: "bash",
		Arguments: map[string]any{
			"command": "sleep 5 && echo 'completed'",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != nil {
		t.Fatalf("Command failed: %v", result.Error)
	}

	if !strings.Contains(result.Content, "completed") {
		t.Errorf("Expected 'completed' in output, got: %s", result.Content)
	}
}

// TestBashCommandWithStdout tests command output handling
func TestBashCommandWithStdout(t *testing.T) {
	sandboxURL := getTestSandboxURL()
	if sandboxURL == "" {
		t.Skip("ALEX_SANDBOX_BASE_URL not set, skipping sandbox test")
	}

	sandbox := tools.NewSandboxManager(sandboxURL)
	tool := NewBash(ShellToolConfig{
		Mode:           tools.ExecutionModeSandbox,
		SandboxManager: sandbox,
	})

	call := ports.ToolCall{
		ID:   "test-stdout",
		Name: "bash",
		Arguments: map[string]any{
			"command": "echo 'Hello from sandbox'",
		},
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != nil {
		t.Fatalf("Command failed: %v", result.Error)
	}

	if !strings.Contains(result.Content, "Hello from sandbox") {
		t.Errorf("Expected output in content, got: %s", result.Content)
	}
}

// TestBashTimeoutNotHit tests that commands complete within timeout
func TestBashTimeoutNotHit(t *testing.T) {
	sandboxURL := getTestSandboxURL()
	if sandboxURL == "" {
		t.Skip("ALEX_SANDBOX_BASE_URL not set, skipping sandbox test")
	}

	sandbox := tools.NewSandboxManager(sandboxURL)
	tool := NewBash(ShellToolConfig{
		Mode:           tools.ExecutionModeSandbox,
		SandboxManager: sandbox,
	})

	// Command that completes quickly
	call := ports.ToolCall{
		ID:   "test-quick",
		Name: "bash",
		Arguments: map[string]any{
			"command": "echo 'quick command'",
		},
	}

	// Very short timeout should still work for quick commands
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result, err := tool.Execute(ctx, call)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != nil {
		t.Fatalf("Command failed: %v", result.Error)
	}

	// Should complete in well under 5 seconds
	if duration > 3*time.Second {
		t.Errorf("Command took too long: %v", duration)
	}
}
