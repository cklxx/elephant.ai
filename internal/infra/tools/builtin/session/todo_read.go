package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"
)

type todoRead struct {
	shared.BaseTool
	sessionsDir string // For testing override
}

func newTodoReadBaseTool() shared.BaseTool {
	return shared.NewBaseTool(
		ports.ToolDefinition{
			Name:        "todo_read",
			Description: "Read the current session's todo list content",
			Parameters: ports.ParameterSchema{
				Type:       "object",
				Properties: map[string]ports.Property{},
			},
		},
		ports.ToolMetadata{
			Name:     "todo_read",
			Version:  "1.0.0",
			Category: "session",
			Tags:     []string{"todo"},
		},
	)
}

func NewTodoRead() tools.ToolExecutor {
	homeDir, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(homeDir, ".alex", "sessions")
	return &todoRead{BaseTool: newTodoReadBaseTool(), sessionsDir: sessionsDir}
}

// NewTodoReadWithSessionsDir creates todo_read with custom sessions directory (for testing)
func NewTodoReadWithSessionsDir(sessionsDir string) tools.ToolExecutor {
	// Expand tilde if present
	if strings.HasPrefix(sessionsDir, "~/") {
		homeDir, _ := os.UserHomeDir()
		sessionsDir = filepath.Join(homeDir, sessionsDir[2:])
	}
	return &todoRead{BaseTool: newTodoReadBaseTool(), sessionsDir: sessionsDir}
}

func (t *todoRead) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Get session ID from context
	sessionID, _ := shared.GetSessionID(ctx)
	sessionID = sanitizeSessionID(sessionID)

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

	// Parse tasks by status
	lines := strings.Split(string(content), "\n")
	totalCount := 0
	inProgressCount := 0
	pendingCount := 0
	completedCount := 0

	var todosData []map[string]interface{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		var taskContent string
		var status string

		if strings.HasPrefix(trimmed, "→ ") {
			taskContent = strings.TrimSpace(trimmed[len("→ "):])
			status = "in_progress"
			inProgressCount++
			totalCount++
		} else if strings.HasPrefix(trimmed, "☐ ") {
			taskContent = strings.TrimSpace(trimmed[len("☐ "):])
			status = "pending"
			pendingCount++
			totalCount++
		} else if strings.HasPrefix(trimmed, "✓ ") {
			taskContent = strings.TrimSpace(trimmed[len("✓ "):])
			status = "completed"
			completedCount++
			totalCount++
		}

		if taskContent != "" && status != "" {
			todosData = append(todosData, map[string]interface{}{
				"content": taskContent,
				"status":  status,
			})
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
			"todos":             todosData,
		},
	}, nil
}
