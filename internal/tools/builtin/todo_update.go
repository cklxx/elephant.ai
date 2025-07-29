package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	
	"alex/internal/session"
)

// NewTodoUpdateTool implements todo update functionality (full replacement)
type NewTodoUpdateTool struct{
	sessionManager *session.Manager  // 直接引用 session manager
}

func CreateNewTodoUpdateTool() *NewTodoUpdateTool {
	return &NewTodoUpdateTool{}
}

// CreateTodoUpdateToolWithSessionManager creates todo update tool with session manager
func CreateTodoUpdateToolWithSessionManager(sessionManager *session.Manager) *NewTodoUpdateTool {
	return &NewTodoUpdateTool{
		sessionManager: sessionManager,
	}
}

func (t *NewTodoUpdateTool) Name() string {
	return "todo_update"
}

func (t *NewTodoUpdateTool) Description() string {
	return `Update session todo list with markdown content supporting goals, tasks, and notes.

Format:
- Use ☐ for pending tasks and ☒ for completed tasks
- Supports any markdown formatting (headers, lists, notes)
- Replaces entire todo content with provided text
- Content persists across session interactions

Example structure:
# Current Goals
☐ Task 1 description
☐ Task 2 description  
☒ Completed task

## Notes
- Additional context or reminders
- Project notes and observations

Usage:
- Overwrites existing todo content completely
- Automatically counts pending/completed tasks
- Session-specific storage (survives session resume)`
}

func (t *NewTodoUpdateTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The complete todo content in markdown format. Can include goals, tasks, notes, etc.",
			},
		},
		"required": []string{"content"},
	}
}

func (t *NewTodoUpdateTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddStringField("content", "Todo content")

	return validator.Validate(args)
}

func (t *NewTodoUpdateTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// Extract content parameter
	content := args["content"].(string)

	// For tools created without session manager, fall back to error
	if t.sessionManager == nil {
		return nil, fmt.Errorf("todo operations require session manager - tool not properly initialized")
	}

	// Get sessions directory and ensure it exists  
	sessionsDir := t.sessionManager.GetSessionsDir()

	// Get session ID directly from manager
	sessionID, hasSession := t.sessionManager.GetSessionID()
	if !hasSession {
		return nil, fmt.Errorf("session ID not available - todo operations require a valid session")
	}
	
	// Use session-specific todo file
	todoFile := filepath.Join(sessionsDir, sessionID+"_todo.md")

	// Write content directly to todo.md file
	err := os.WriteFile(todoFile, []byte(content), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write todo file: %w", err)
	}

	// Count lines for basic statistics
	lines := strings.Split(content, "\n")
	lineCount := len(lines)

	// Count checkboxes if present
	completedCount := 0
	pendingCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "☒") {
			completedCount++
		} else if strings.HasPrefix(line, "☐") {
			pendingCount++
		}
	}

	return &ToolResult{
		Content: content,
		Data: map[string]interface{}{
			"content":         content,
			"line_count":      lineCount,
			"pending_count":   pendingCount,
			"completed_count": completedCount,
			"file_path":       todoFile,
		},
	}, nil
}
