package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/agent/ports"
)

type todoUpdate struct {
	sessionsDir string // For testing override
}

func NewTodoUpdate() ports.ToolExecutor {
	return &todoUpdate{}
}

// NewTodoUpdateWithSessionsDir creates todo_update with custom sessions directory (for testing)
func NewTodoUpdateWithSessionsDir(sessionsDir string) ports.ToolExecutor {
	return &todoUpdate{sessionsDir: sessionsDir}
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

	var inProgress, pending, completed []string

	for _, item := range todosArray {
		todo, _ := item.(map[string]any)
		content, _ := todo["content"].(string)
		status, _ := todo["status"].(string)

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
			md.WriteString(fmt.Sprintf("- [▶] %s\n", t))
		}
		md.WriteString("\n")
	}

	if len(pending) > 0 {
		md.WriteString("## Pending\n\n")
		for _, t := range pending {
			md.WriteString(fmt.Sprintf("- [ ] %s\n", t))
		}
		md.WriteString("\n")
	}

	if len(completed) > 0 {
		md.WriteString("## Completed\n\n")
		for _, t := range completed {
			md.WriteString(fmt.Sprintf("- [✓] %s\n", t))
		}
		md.WriteString("\n")
	}

	var sessionDir, todoFile string
	if t.sessionsDir != "" {
		// Test mode with custom sessions directory
		sessionDir = filepath.Join(t.sessionsDir, "default")
	} else {
		// Production mode
		homeDir, _ := os.UserHomeDir()
		sessionDir = filepath.Join(homeDir, ".alex-sessions", "default")
	}
	os.MkdirAll(sessionDir, 0755)
	todoFile = filepath.Join(sessionDir, "todo.md")
	os.WriteFile(todoFile, []byte(md.String()), 0644)

	// Build detailed result content
	var result strings.Builder
	total := len(inProgress) + len(pending) + len(completed)
	result.WriteString(fmt.Sprintf("Updated: %d in progress, %d pending, %d completed (%d total)\n\n",
		len(inProgress), len(pending), len(completed), total))

	// Add task details
	if len(inProgress) > 0 {
		result.WriteString("In Progress:\n")
		for _, task := range inProgress {
			result.WriteString(fmt.Sprintf("  - %s\n", task))
		}
		result.WriteString("\n")
	}

	if len(pending) > 0 {
		result.WriteString("Pending:\n")
		for _, task := range pending {
			result.WriteString(fmt.Sprintf("  - %s\n", task))
		}
		result.WriteString("\n")
	}

	if len(completed) > 0 && len(completed) <= 3 {
		result.WriteString("Recently Completed:\n")
		for _, task := range completed {
			result.WriteString(fmt.Sprintf("  - %s\n", task))
		}
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: result.String(),
	}, nil
}
