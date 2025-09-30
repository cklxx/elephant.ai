package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/agent/ports"
)

type todoRead struct {
	sessionsDir string // For testing override
}

func NewTodoRead() ports.ToolExecutor {
	return &todoRead{}
}

// NewTodoReadWithSessionsDir creates todo_read with custom sessions directory (for testing)
func NewTodoReadWithSessionsDir(sessionsDir string) ports.ToolExecutor {
	return &todoRead{sessionsDir: sessionsDir}
}

func (t *todoRead) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "todo_read",
		Version:  "1.0.0",
		Category: "session",
		Tags:     []string{"todo"},
	}
}

func (t *todoRead) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "todo_read",
		Description: "Read the current session's todo list content",
		Parameters: ports.ParameterSchema{
			Type:       "object",
			Properties: map[string]ports.Property{},
		},
	}
}

func (t *todoRead) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	var todoFile string
	if t.sessionsDir != "" {
		// Test mode with custom sessions directory
		todoFile = filepath.Join(t.sessionsDir, "default", "todo.md")
	} else {
		// Production mode
		homeDir, _ := os.UserHomeDir()
		todoFile = filepath.Join(homeDir, ".alex-sessions", "default", "todo.md")
	}

	content, err := os.ReadFile(todoFile)
	if os.IsNotExist(err) {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No todo list found. Use todo_update to create one.",
		}, nil
	}

	lines := strings.Split(string(content), "\n")
	taskCount := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "-") {
			taskCount++
		}
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Todo List:\n\n%s\n\n%d tasks", string(content), taskCount),
	}, nil
}
