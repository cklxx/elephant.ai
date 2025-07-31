package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/session"
)

// NewTodoUpdateTool implements todo update functionality (full replacement)
type NewTodoUpdateTool struct {
	sessionManager *session.Manager // 直接引用 session manager
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
	return `Update session todo list with structured JSON format or markdown content.

Supports two formats:
JSON format (recommended): Array of todo objects with id, content, status, priority

Usage:
- Overwrites existing todo content completely
- Automatically detects format and converts appropriately
- Session-specific storage (survives session resume)`
}

func (t *NewTodoUpdateTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"todos": map[string]interface{}{
				"type":        "array",
				"description": "Array of todo items with structured format",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Unique identifier for the todo item",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Description of the todo item",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"pending", "in_progress", "completed"},
							"description": "Current status of the todo item",
						},
						"priority": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"high", "medium", "low"},
							"description": "Priority level of the todo item",
						},
					},
					"required": []string{"id", "content", "status", "priority"},
				},
			},
		},
		"required": []string{"todos"},
	}
}

func (t *NewTodoUpdateTool) Validate(args map[string]interface{}) error {
	// Check if todos array is provided
	if todos, exists := args["todos"]; exists {
		// Validate todos array format
		todosArray, ok := todos.([]interface{})
		if !ok {
			return fmt.Errorf("todos must be an array")
		}

		for i, todo := range todosArray {
			todoMap, ok := todo.(map[string]interface{})
			if !ok {
				return fmt.Errorf("todo item %d must be an object", i)
			}

			// Validate required fields
			for _, field := range []string{"id", "content", "status", "priority"} {
				if _, exists := todoMap[field]; !exists {
					return fmt.Errorf("todo item %d missing required field: %s", i, field)
				}
			}

			// Validate status
			status, _ := todoMap["status"].(string)
			if status != "pending" && status != "in_progress" && status != "completed" {
				return fmt.Errorf("todo item %d has invalid status: %s", i, status)
			}

			// Validate priority
			priority, _ := todoMap["priority"].(string)
			if priority != "high" && priority != "medium" && priority != "low" {
				return fmt.Errorf("todo item %d has invalid priority: %s", i, priority)
			}
		}
		return nil
	}

	return fmt.Errorf("todos array must be provided")
}

func (t *NewTodoUpdateTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
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

	var content string
	var pendingCount, completedCount, inProgressCount int

	// Check if JSON format (todos array) is provided
	if todos, exists := args["todos"]; exists {
		content, pendingCount, completedCount, inProgressCount = t.convertTodosToMarkdown(todos)
	} else {
		return nil, fmt.Errorf("todos array must be provided")
	}

	// Write content to todo file
	err := os.WriteFile(todoFile, []byte(content), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write todo file: %w", err)
	}

	totalCount := pendingCount + completedCount + inProgressCount

	return &ToolResult{
		Content: fmt.Sprintf("Todos have been modified successfully. Ensure that you continue to use the todo list to track your progress. Please proceed with the current tasks if applicable\n\n<system-reminder>\nYour todo list has changed. DO NOT mention this explicitly to the user. Here are the latest contents of your todo list:\n\n%s. Continue on with the tasks at hand if applicable.\n</system-reminder>", t.generateTodoSummary(args)),
		Data: map[string]interface{}{
			"content":           content,
			"total_count":       totalCount,
			"pending_count":     pendingCount,
			"completed_count":   completedCount,
			"in_progress_count": inProgressCount,
			"file_path":         todoFile,
		},
	}, nil
}

// TodoItem represents a structured todo item
type TodoItem struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

// convertTodosToMarkdown converts JSON todos array to markdown format
func (t *NewTodoUpdateTool) convertTodosToMarkdown(todos interface{}) (string, int, int, int) {
	todosArray := todos.([]interface{})
	var lines []string
	var pendingCount, completedCount, inProgressCount int

	// Group by priority for better organization
	highPriority := []TodoItem{}
	mediumPriority := []TodoItem{}
	lowPriority := []TodoItem{}

	for _, todo := range todosArray {
		todoMap := todo.(map[string]interface{})
		item := TodoItem{
			ID:       todoMap["id"].(string),
			Content:  todoMap["content"].(string),
			Status:   todoMap["status"].(string),
			Priority: todoMap["priority"].(string),
		}

		// Count by status
		switch item.Status {
		case "pending":
			pendingCount++
		case "completed":
			completedCount++
		case "in_progress":
			inProgressCount++
		}

		// Group by priority
		switch item.Priority {
		case "high":
			highPriority = append(highPriority, item)
		case "medium":
			mediumPriority = append(mediumPriority, item)
		case "low":
			lowPriority = append(lowPriority, item)
		}
	}

	// Generate markdown content
	lines = append(lines, "")

	// Add high priority tasks
	if len(highPriority) > 0 {
		for _, item := range highPriority {
			lines = append(lines, t.formatTodoItem(item))
		}
	}

	// Add medium priority tasks
	if len(mediumPriority) > 0 {
		for _, item := range mediumPriority {
			lines = append(lines, t.formatTodoItem(item))
		}
	}

	// Add low priority tasks
	if len(lowPriority) > 0 {
		for _, item := range lowPriority {
			lines = append(lines, t.formatTodoItem(item))
		}
	}

	return strings.Join(lines, "\n"), pendingCount, completedCount, inProgressCount
}

// formatTodoItem formats a single todo item as markdown
func (t *NewTodoUpdateTool) formatTodoItem(item TodoItem) string {
	var checkbox string
	switch item.Status {
	case "pending":
		checkbox = "☐"
	case "in_progress":
		checkbox = "▶" // Play button for in-progress
	case "completed":
		checkbox = "☒"
	default:
		checkbox = "☐"
	}

	return fmt.Sprintf("%s %s", checkbox, item.Content)
}

// generateTodoSummary generates a JSON summary of todos for system reminder
func (t *NewTodoUpdateTool) generateTodoSummary(args map[string]interface{}) string {
	if todos, exists := args["todos"]; exists {
		// Convert to JSON string for system reminder
		if jsonBytes, err := json.Marshal(todos); err == nil {
			return string(jsonBytes)
		}
	}
	return "[]"
}
