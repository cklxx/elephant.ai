package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/agent/ports"
)

type todoRead struct {
	sessionsDir string // For testing override
}

func NewTodoRead() ports.ToolExecutor {
	homeDir, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(homeDir, ".alex-sessions")
	return &todoRead{sessionsDir: sessionsDir}
}

// NewTodoReadWithSessionsDir creates todo_read with custom sessions directory (for testing)
func NewTodoReadWithSessionsDir(sessionsDir string) ports.ToolExecutor {
	// Expand tilde if present
	if strings.HasPrefix(sessionsDir, "~/") {
		homeDir, _ := os.UserHomeDir()
		sessionsDir = filepath.Join(homeDir, sessionsDir[2:])
	}
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
	// Get session ID from context
	sessionID, ok := GetSessionID(ctx)
	if !ok || sessionID == "" {
		sessionID = "default"
	}

	// Construct file path
	todoFile := filepath.Join(t.sessionsDir, sessionID+"_todo.md")

	content, err := os.ReadFile(todoFile)
	if os.IsNotExist(err) {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No todo list found. Use todo_update to create one.",
			Metadata: map[string]interface{}{
				"has_todos":  false,
				"task_count": 0,
			},
		}, nil
	}

	// Count tasks by status
	lines := strings.Split(string(content), "\n")
	totalCount := 0
	inProgressCount := 0
	pendingCount := 0
	completedCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "→") {
			inProgressCount++
			totalCount++
		} else if strings.HasPrefix(trimmed, "☐") {
			pendingCount++
			totalCount++
		} else if strings.HasPrefix(trimmed, "✓") {
			completedCount++
			totalCount++
		}
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: string(content),
		Metadata: map[string]interface{}{
			"has_todos":         totalCount > 0,
			"total_count":       totalCount,
			"in_progress_count": inProgressCount,
			"pending_count":     pendingCount,
			"completed_count":   completedCount,
		},
	}, nil
}
