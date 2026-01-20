package builtin

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/agent/ports"
)

type todoUpdate struct {
	sessionsDir string
	writeFile   func(string, []byte, fs.FileMode) error
}

func NewTodoUpdate() ports.ToolExecutor {
	homeDir, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(homeDir, ".alex-sessions")
	return newTodoUpdate(sessionsDir, nil)
}

// NewTodoUpdateWithSessionsDir creates todo_update with custom sessions directory (for testing)
func NewTodoUpdateWithSessionsDir(sessionsDir string) ports.ToolExecutor {
	// Expand tilde if present
	if strings.HasPrefix(sessionsDir, "~/") {
		homeDir, _ := os.UserHomeDir()
		sessionsDir = filepath.Join(homeDir, sessionsDir[2:])
	}
	return newTodoUpdate(sessionsDir, nil)
}

func newTodoUpdate(sessionsDir string, writer func(string, []byte, fs.FileMode) error) *todoUpdate {
	if writer == nil {
		writer = os.WriteFile
	}

	return &todoUpdate{
		sessionsDir: sessionsDir,
		writeFile:   writer,
	}
}

func (t *todoUpdate) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "todo_update",
		Version:  "1.0.0",
		Category: "session",
		Tags:     []string{"todo"},
	}
}

func (t *todoUpdate) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "todo_update",
		Description: `Update session todo list with structured format.

Parameters:
- todos: Array of {content, status, activeForm}
  - content (required): Task description
  - status (required): "pending", "in_progress", "completed"
  - activeForm (optional): Present continuous form`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"todos": {
					Type:        "array",
					Description: "Array of todo items",
					Items:       &ports.Property{Type: "object"},
				},
			},
			Required: []string{"todos"},
		},
	}
}

func (t *todoUpdate) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	todosArg, ok := call.Arguments["todos"]
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Content: "Error: todos required", Error: fmt.Errorf("missing todos")}, nil
	}

	todosArray, ok := todosArg.([]any)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Content: "Error: todos must be an array", Error: fmt.Errorf("todos must be an array")}, nil
	}

	// Get session ID from context
	sessionID, ok := GetSessionID(ctx)
	if !ok || sessionID == "" {
		sessionID = "default"
	}

	var inProgress, pending, completed []string

	for _, item := range todosArray {
		todo, _ := item.(map[string]any)
		content, _ := todo["content"].(string)
		status, _ := todo["status"].(string)

		// Default to pending if no status
		if status == "" && content != "" {
			status = "pending"
		}

		switch status {
		case "in_progress":
			inProgress = append(inProgress, content)
		case "pending":
			pending = append(pending, content)
		case "completed":
			completed = append(completed, content)
		}
	}

	var md strings.Builder
	md.WriteString("# Task List\n\n")

	if len(inProgress) > 0 {
		md.WriteString("## In Progress\n\n")
		for _, t := range inProgress {
			md.WriteString(fmt.Sprintf("→ %s\n", t))
		}
		md.WriteString("\n")
	}

	if len(pending) > 0 {
		md.WriteString("## Pending\n\n")
		for _, t := range pending {
			md.WriteString(fmt.Sprintf("☐ %s\n", t))
		}
		md.WriteString("\n")
	}

	if len(completed) > 0 {
		md.WriteString("## Completed\n\n")
		for _, t := range completed {
			md.WriteString(fmt.Sprintf("✓ %s\n", t))
		}
		md.WriteString("\n")
	}

	// Determine file path and write
	todoFile := filepath.Join(t.sessionsDir, sessionID+"_todo.md")

	// Write file with error handling
	if err := t.writeFile(todoFile, []byte(md.String()), 0644); err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error: failed to write todo file: %v", err),
			Error:   fmt.Errorf("failed to write todo file: %w", err),
		}, nil
	}

	// Build detailed result content with flat layout
	var result strings.Builder
	total := len(inProgress) + len(pending) + len(completed)

	// Add system reminder for model (will be filtered out in display)
	result.WriteString("<system-reminder>Todos have been modified successfully.</system-reminder>\n\n")

	// Flat layout with completion status
	result.WriteString(fmt.Sprintf("Tasks: %d total", total))
	if len(inProgress) > 0 {
		result.WriteString(fmt.Sprintf(" | %d in progress", len(inProgress)))
	}
	if len(pending) > 0 {
		result.WriteString(fmt.Sprintf(" | %d pending", len(pending)))
	}
	if len(completed) > 0 {
		result.WriteString(fmt.Sprintf(" | %d completed", len(completed)))
	}
	result.WriteString("\n\n")

	// Show all tasks in flat list
	if len(inProgress) > 0 {
		for _, task := range inProgress {
			result.WriteString(fmt.Sprintf("→ %s\n", task))
		}
	}
	if len(pending) > 0 {
		for _, task := range pending {
			result.WriteString(fmt.Sprintf("☐ %s\n", task))
		}
	}
	if len(completed) > 0 && len(completed) <= 3 {
		for _, task := range completed {
			result.WriteString(fmt.Sprintf("✓ %s\n", task))
		}
	}

	// Build structured todos array for frontend
	var todosData []map[string]any
	for _, task := range inProgress {
		todosData = append(todosData, map[string]any{
			"content": task,
			"status":  "in_progress",
		})
	}
	for _, task := range pending {
		todosData = append(todosData, map[string]any{
			"content": task,
			"status":  "pending",
		})
	}
	for _, task := range completed {
		todosData = append(todosData, map[string]any{
			"content": task,
			"status":  "completed",
		})
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: result.String(),
		Metadata: map[string]any{
			"total_count":       total,
			"in_progress_count": len(inProgress),
			"pending_count":     len(pending),
			"completed_count":   len(completed),
			"todos":             todosData,
		},
	}, nil
}
