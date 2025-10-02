package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTodoRead_Metadata(t *testing.T) {
	tool := NewTodoRead()
	meta := tool.Metadata()

	assert.Equal(t, "todo_read", meta.Name)
	assert.Equal(t, "1.0.0", meta.Version)
	assert.Equal(t, "session", meta.Category)
	assert.Contains(t, meta.Tags, "todo")
}

func TestTodoRead_Definition(t *testing.T) {
	tool := NewTodoRead()
	def := tool.Definition()

	assert.Equal(t, "todo_read", def.Name)
	assert.Contains(t, def.Description, "todo list")
	assert.Contains(t, def.Description, "session")

	// Should have no parameters
	assert.Equal(t, "object", def.Parameters.Type)
	assert.Empty(t, def.Parameters.Properties)
}

func TestTodoRead_Execute_NoTodoFile(t *testing.T) {
	// Create temp directory for sessions
	tempDir := t.TempDir()
	tool := NewTodoReadWithSessionsDir(tempDir)

	ctx := context.Background()

	call := ports.ToolCall{
		ID:        "call-1",
		Name:      "todo_read",
		Arguments: map[string]any{},
	}

	result, err := tool.Execute(ctx, call)

	// Should succeed but return "no todo file found" message
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "call-1", result.CallID)
	assert.Contains(t, result.Content, "No todo")

	// Check metadata
	assert.False(t, result.Metadata["has_todos"].(bool))
	assert.Equal(t, 0, result.Metadata["task_count"])
}

func TestTodoRead_Execute_WithTodoFile(t *testing.T) {
	// Create temp directory for sessions
	tempDir := t.TempDir()
	tool := NewTodoReadWithSessionsDir(tempDir)

	// Create todo file content using new format
	todoContent := `# Task List

## Pending

☐ Implement authentication
☐ Create database schema

## In Progress

→ Build REST API

## Completed

✓ Write unit tests
✓ Setup project structure
`

	// Write todo file to sessions directory (default_todo.md)
	todoFile := filepath.Join(tempDir, "default_todo.md")
	err := os.WriteFile(todoFile, []byte(todoContent), 0644)
	require.NoError(t, err)

	ctx := context.Background()

	call := ports.ToolCall{
		ID:        "call-2",
		Name:      "todo_read",
		Arguments: map[string]any{},
	}

	result, err := tool.Execute(ctx, call)

	// Should succeed and return todo content
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "call-2", result.CallID)
	assert.Contains(t, result.Content, todoContent)

	// Check metadata
	assert.True(t, result.Metadata["has_todos"].(bool))
	assert.Equal(t, 5, result.Metadata["total_count"])
	assert.Equal(t, 2, result.Metadata["pending_count"])
	assert.Equal(t, 1, result.Metadata["in_progress_count"])
	assert.Equal(t, 2, result.Metadata["completed_count"])
}

func TestTodoRead_Execute_NoContext(t *testing.T) {
	// Create temp directory for sessions
	tempDir := t.TempDir()
	tool := NewTodoReadWithSessionsDir(tempDir)

	// No session ID in context - should use "default"
	call := ports.ToolCall{
		ID:        "call-3",
		Name:      "todo_read",
		Arguments: map[string]any{},
	}

	result, err := tool.Execute(context.Background(), call)

	// Should succeed with default session
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Content, "No todo")
}

func TestTodoRead_Execute_DefaultSessionID(t *testing.T) {
	// Create temp directory for sessions
	tempDir := t.TempDir()
	tool := NewTodoReadWithSessionsDir(tempDir)

	// Empty session ID should use "default"
	ctx := WithSessionID(context.Background(), "")

	call := ports.ToolCall{
		ID:        "call-4",
		Name:      "todo_read",
		Arguments: map[string]any{},
	}

	result, err := tool.Execute(ctx, call)

	// Should succeed with default session
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Content, "No todo")
}

func TestTodoRead_Execute_FileReadError(t *testing.T) {
	t.Skip("File read error handling not implemented - tool only handles file not found")
}

func TestTodoRead_Execute_EmptyTodoFile(t *testing.T) {
	// Create temp directory for sessions
	tempDir := t.TempDir()
	tool := NewTodoReadWithSessionsDir(tempDir)

	// Create empty todo file (default_todo.md)
	todoFile := filepath.Join(tempDir, "default_todo.md")
	err := os.WriteFile(todoFile, []byte(""), 0644)
	require.NoError(t, err)

	ctx := context.Background()

	call := ports.ToolCall{
		ID:        "call-6",
		Name:      "todo_read",
		Arguments: map[string]any{},
	}

	result, err := tool.Execute(ctx, call)

	// Should succeed but have no tasks
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Metadata["has_todos"].(bool))
	assert.Equal(t, 0, result.Metadata["total_count"])
}

func TestTodoRead_NewTodoRead(t *testing.T) {
	tool := NewTodoRead()
	assert.NotNil(t, tool)

	// Check that it's a todoRead instance
	todoReadTool, ok := tool.(*todoRead)
	require.True(t, ok)
	// sessionsDir should be set to .alex-sessions
	home, _ := os.UserHomeDir()
	expectedDir := filepath.Join(home, ".alex-sessions")
	assert.Equal(t, expectedDir, todoReadTool.sessionsDir)
}

func TestTodoRead_NewTodoReadWithSessionsDir(t *testing.T) {
	tempDir := t.TempDir()
	tool := NewTodoReadWithSessionsDir(tempDir)
	assert.NotNil(t, tool)

	// Check that the tool's sessionsDir is set correctly
	todoReadTool, ok := tool.(*todoRead)
	require.True(t, ok)
	assert.Equal(t, tempDir, todoReadTool.sessionsDir)
}

func TestTodoRead_NewTodoReadWithSessionsDir_TildeExpansion(t *testing.T) {
	tool := NewTodoReadWithSessionsDir("~/.test-sessions")
	assert.NotNil(t, tool)

	// Check that tilde is expanded
	todoReadTool, ok := tool.(*todoRead)
	require.True(t, ok)

	home, _ := os.UserHomeDir()
	expectedDir := filepath.Join(home, ".test-sessions")
	assert.Equal(t, expectedDir, todoReadTool.sessionsDir)
}

func TestWithSessionID(t *testing.T) {
	ctx := context.Background()
	sessionID := "test-session-id"

	// Add session ID to context
	ctx = WithSessionID(ctx, sessionID)

	// Retrieve session ID from context
	retrievedID, ok := GetSessionID(ctx)
	assert.True(t, ok)
	assert.Equal(t, sessionID, retrievedID)
}
