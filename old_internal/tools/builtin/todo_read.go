package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"alex/internal/session"
)

// TodoReadTool implements todo reading functionality
type TodoReadTool struct {
	sessionManager *session.Manager // 直接引用 session manager
}

func CreateTodoReadTool() *TodoReadTool {
	return &TodoReadTool{}
}

// CreateTodoReadToolWithSessionManager creates todo read tool with session manager
func CreateTodoReadToolWithSessionManager(sessionManager *session.Manager) *TodoReadTool {
	return &TodoReadTool{
		sessionManager: sessionManager,
	}
}

func (t *TodoReadTool) Name() string {
	return "todo_read"
}

func (t *TodoReadTool) Description() string {
	return `Read the current session's todo list content.

Usage:
- Displays complete todo content from session-specific file
- Shows free-form markdown content including goals, tasks, and notes
- Returns empty message if no todo file exists
- Content preserved exactly as written with todo_update

Output:
- Raw markdown content with checkboxes and formatting
- Basic statistics (pending ☐ and completed ☒ counts)
- Session-specific todo content

Note: Use todo_update to create or modify todo content`
}

func (t *TodoReadTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *TodoReadTool) Validate(args map[string]interface{}) error {
	// No validation needed as there are no parameters
	return nil
}

func (t *TodoReadTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// For tools created without session manager, fall back to trying context or return error
	if t.sessionManager == nil {
		return nil, fmt.Errorf("todo operations require session manager - tool not properly initialized")
	}

	// Get sessions directory and ensure it exists
	sessionsDir := t.sessionManager.GetSessionsDir()

	// For direct session manager approach, we need to find a way to get current session
	// This is a limitation - we need the session ID somehow
	// For now, let's check if there's a session ID in args or context
	var sessionID string
	if id, exists := t.sessionManager.GetSessionID(); exists {
		sessionID = id
	}

	if sessionID == "" {
		return nil, fmt.Errorf("session ID not provided - todo operations require a valid session")
	}

	// Use session-specific todo file
	todoFile := filepath.Join(sessionsDir, sessionID+"_todo.md")

	return t.executeWithSessionID(todoFile)
}

// executeWithSessionID - 直接使用session ID执行，避免context依赖
func (t *TodoReadTool) executeWithSessionID(todoFile string) (*ToolResult, error) {

	// Check if todo file exists
	if _, err := os.Stat(todoFile); os.IsNotExist(err) {
		return &ToolResult{
			Content: "No todo file found. Use todo_update to create one.",
			Data: map[string]interface{}{
				"has_todos":     false,
				"final_goal":    "",
				"todo_items":    []string{},
				"completed":     []string{},
				"pending":       []string{},
				"total_count":   0,
				"pending_count": 0,
				"content":       "No todo file found. Use todo_update to create one.",
			},
		}, nil
	}

	// Read todo file
	content, err := os.ReadFile(todoFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read todo file: %w", err)
	}

	return &ToolResult{
		Content: string(content),
		Data: map[string]interface{}{
			"content": string(content),
		},
	}, nil
}
