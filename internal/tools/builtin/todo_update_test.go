package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"alex/internal/tools/builtin/shared"
)

func TestTodoUpdate_Metadata(t *testing.T) {
	tool := NewTodoUpdate()
	meta := tool.Metadata()

	assert.Equal(t, "todo_update", meta.Name)
	assert.Equal(t, "1.0.0", meta.Version)
	assert.Equal(t, "session", meta.Category)
	assert.Contains(t, meta.Tags, "todo")
}

func TestTodoUpdate_Definition(t *testing.T) {
	tool := NewTodoUpdate()
	def := tool.Definition()

	assert.Equal(t, "todo_update", def.Name)
	assert.Contains(t, def.Description, "todo list")
	assert.Contains(t, def.Description, "session")

	// Should have todos parameter
	assert.Equal(t, "object", def.Parameters.Type)
	assert.Contains(t, def.Parameters.Properties, "todos")
	assert.Equal(t, []string{"todos"}, def.Parameters.Required)

	// Check todos parameter
	todosParam := def.Parameters.Properties["todos"]
	assert.Equal(t, "array", todosParam.Type)
}

func TestTodoUpdate_Execute_NoTodos(t *testing.T) {
	tempDir := t.TempDir()
	tool := NewTodoUpdateWithSessionsDir(tempDir)

	ctx := shared.WithSessionID(context.Background(), "test_session")

	call := ports.ToolCall{
		ID:        "call-1",
		Name:      "todo_update",
		Arguments: map[string]any{},
	}

	result, err := tool.Execute(ctx, call)

	// Should return error
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "missing todos")
}

func TestTodoUpdate_Execute_InvalidTodos(t *testing.T) {
	tempDir := t.TempDir()
	tool := NewTodoUpdateWithSessionsDir(tempDir)

	ctx := shared.WithSessionID(context.Background(), "test_session")

	call := ports.ToolCall{
		ID:   "call-2",
		Name: "todo_update",
		Arguments: map[string]any{
			"todos": "not-an-array",
		},
	}

	result, err := tool.Execute(ctx, call)

	// Should return error
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "todos must be an array")
}

func TestTodoUpdate_Execute_Success(t *testing.T) {
	tempDir := t.TempDir()
	tool := NewTodoUpdateWithSessionsDir(tempDir)

	sessionID := "test_session_update"
	ctx := shared.WithSessionID(context.Background(), sessionID)

	todos := []any{
		map[string]any{
			"content":    "Implement authentication",
			"status":     "pending",
			"activeForm": "Implementing authentication",
		},
		map[string]any{
			"content":    "Write tests",
			"status":     "in_progress",
			"activeForm": "Writing tests",
		},
		map[string]any{
			"content": "Setup database",
			"status":  "completed",
		},
	}

	call := ports.ToolCall{
		ID:   "call-3",
		Name: "todo_update",
		Arguments: map[string]any{
			"todos": todos,
		},
	}

	result, err := tool.Execute(ctx, call)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Nil(t, result.Error)
	assert.Equal(t, "call-3", result.CallID)
	assert.Contains(t, result.Content, "Todos have been modified successfully")
	assert.Contains(t, result.Content, "system-reminder")

	// Verify metadata
	assert.Equal(t, 3, result.Metadata["total_count"])
	assert.Equal(t, 1, result.Metadata["pending_count"])
	assert.Equal(t, 1, result.Metadata["in_progress_count"])
	assert.Equal(t, 1, result.Metadata["completed_count"])

	// Verify the file was created
	todoFile := filepath.Join(tempDir, sessionID+"_todo.md")
	assert.FileExists(t, todoFile)

	// Read and verify content
	content, err := os.ReadFile(todoFile)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# Task List")
	assert.Contains(t, contentStr, "Implement authentication")
	assert.Contains(t, contentStr, "Write tests")
	assert.Contains(t, contentStr, "Setup database")
	assert.Contains(t, contentStr, "☐") // pending checkbox
	assert.Contains(t, contentStr, "→") // in_progress indicator
	assert.Contains(t, contentStr, "✓") // completed checkbox
}

func TestTodoUpdate_Execute_EmptyTodos(t *testing.T) {
	tempDir := t.TempDir()
	tool := NewTodoUpdateWithSessionsDir(tempDir)

	sessionID := "test_session_empty"
	ctx := shared.WithSessionID(context.Background(), sessionID)

	call := ports.ToolCall{
		ID:   "call-4",
		Name: "todo_update",
		Arguments: map[string]any{
			"todos": []any{},
		},
	}

	result, err := tool.Execute(ctx, call)

	// Should succeed with empty todos
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.Content, "Todos have been modified successfully")

	// Verify metadata
	assert.Equal(t, 0, result.Metadata["total_count"])
	assert.Equal(t, 0, result.Metadata["pending_count"])
	assert.Equal(t, 0, result.Metadata["in_progress_count"])
	assert.Equal(t, 0, result.Metadata["completed_count"])

	// Verify file exists
	todoFile := filepath.Join(tempDir, sessionID+"_todo.md")
	assert.FileExists(t, todoFile)
}

func TestTodoUpdate_Execute_OnlyPending(t *testing.T) {
	tempDir := t.TempDir()
	tool := NewTodoUpdateWithSessionsDir(tempDir)

	sessionID := "test_session_pending"
	ctx := shared.WithSessionID(context.Background(), sessionID)

	todos := []any{
		map[string]any{
			"content": "Task 1",
			"status":  "pending",
		},
		map[string]any{
			"content": "Task 2",
			"status":  "pending",
		},
	}

	call := ports.ToolCall{
		ID:   "call-5",
		Name: "todo_update",
		Arguments: map[string]any{
			"todos": todos,
		},
	}

	result, err := tool.Execute(ctx, call)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Nil(t, result.Error)

	// Verify metadata
	assert.Equal(t, 2, result.Metadata["total_count"])
	assert.Equal(t, 2, result.Metadata["pending_count"])
	assert.Equal(t, 0, result.Metadata["in_progress_count"])
	assert.Equal(t, 0, result.Metadata["completed_count"])

	// Read and verify content
	todoFile := filepath.Join(tempDir, sessionID+"_todo.md")
	content, err := os.ReadFile(todoFile)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "## Pending")
	assert.Contains(t, contentStr, "☐ Task 1")
	assert.Contains(t, contentStr, "☐ Task 2")
	assert.NotContains(t, contentStr, "## In Progress")
	assert.NotContains(t, contentStr, "## Completed")
}

func TestTodoUpdate_Execute_MixedStatus(t *testing.T) {
	tempDir := t.TempDir()
	tool := NewTodoUpdateWithSessionsDir(tempDir)

	sessionID := "test_session_mixed"
	ctx := shared.WithSessionID(context.Background(), sessionID)

	todos := []any{
		map[string]any{
			"content": "In progress task",
			"status":  "in_progress",
		},
		map[string]any{
			"content": "Pending task",
			"status":  "pending",
		},
		map[string]any{
			"content": "Completed task",
			"status":  "completed",
		},
		map[string]any{
			"content": "Another pending",
			"status":  "pending",
		},
	}

	call := ports.ToolCall{
		ID:   "call-6",
		Name: "todo_update",
		Arguments: map[string]any{
			"todos": todos,
		},
	}

	result, err := tool.Execute(ctx, call)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Nil(t, result.Error)

	// Read and verify content structure
	todoFile := filepath.Join(tempDir, sessionID+"_todo.md")
	content, err := os.ReadFile(todoFile)
	require.NoError(t, err)

	contentStr := string(content)

	// Verify all sections exist
	assert.Contains(t, contentStr, "## In Progress")
	assert.Contains(t, contentStr, "## Pending")
	assert.Contains(t, contentStr, "## Completed")

	// Verify order: In Progress should come before Pending, which should come before Completed
	inProgressIdx := indexOf(contentStr, "## In Progress")
	pendingIdx := indexOf(contentStr, "## Pending")
	completedIdx := indexOf(contentStr, "## Completed")

	assert.True(t, inProgressIdx < pendingIdx)
	assert.True(t, pendingIdx < completedIdx)
}

func TestTodoUpdate_Execute_DefaultStatus(t *testing.T) {
	tempDir := t.TempDir()
	tool := NewTodoUpdateWithSessionsDir(tempDir)

	sessionID := "test_session_default"
	ctx := shared.WithSessionID(context.Background(), sessionID)

	// Todo without status should default to pending
	todos := []any{
		map[string]any{
			"content": "Task without status",
		},
	}

	call := ports.ToolCall{
		ID:   "call-7",
		Name: "todo_update",
		Arguments: map[string]any{
			"todos": todos,
		},
	}

	result, err := tool.Execute(ctx, call)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Nil(t, result.Error)

	// Should be counted as pending
	assert.Equal(t, 1, result.Metadata["pending_count"])

	// Read and verify content
	todoFile := filepath.Join(tempDir, sessionID+"_todo.md")
	content, err := os.ReadFile(todoFile)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "☐ Task without status")
}

func TestTodoUpdate_Execute_NoSessionID(t *testing.T) {
	tempDir := t.TempDir()
	tool := NewTodoUpdateWithSessionsDir(tempDir)

	// No session ID - should use "default"
	ctx := context.Background()

	todos := []any{
		map[string]any{
			"content": "Default session task",
			"status":  "pending",
		},
	}

	call := ports.ToolCall{
		ID:   "call-8",
		Name: "todo_update",
		Arguments: map[string]any{
			"todos": todos,
		},
	}

	result, err := tool.Execute(ctx, call)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Nil(t, result.Error)

	// Verify file was created with default session ID
	todoFile := filepath.Join(tempDir, "default_todo.md")
	assert.FileExists(t, todoFile)
}

func TestTodoUpdate_Execute_FileWriteError(t *testing.T) {
	tempDir := t.TempDir()

	tool := NewTodoUpdateWithSessionsDir(tempDir)
	todoTool, ok := tool.(*todoUpdate)
	require.True(t, ok)
	todoTool.writeFile = func(string, []byte, fs.FileMode) error {
		return errors.New("disk quota exceeded")
	}

	sessionID := "test_session_error"
	ctx := shared.WithSessionID(context.Background(), sessionID)

	todos := []any{
		map[string]any{
			"content": "Test",
			"status":  "pending",
		},
	}

	call := ports.ToolCall{
		ID:   "call-9",
		Name: "todo_update",
		Arguments: map[string]any{
			"todos": todos,
		},
	}

	result, err := tool.Execute(ctx, call)

	// Should return error
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "failed to write todo file")
}

func TestTodoUpdate_NewTodoUpdate(t *testing.T) {
	tool := NewTodoUpdate()
	assert.NotNil(t, tool)

	// Verify it creates the sessions directory
	home, _ := os.UserHomeDir()
	expectedDir := filepath.Join(home, ".alex-sessions")

	// Check that the tool's sessionsDir is set correctly
	todoUpdateTool, ok := tool.(*todoUpdate)
	require.True(t, ok)
	assert.Equal(t, expectedDir, todoUpdateTool.sessionsDir)
}

func TestTodoUpdate_NewTodoUpdateWithSessionsDir(t *testing.T) {
	tempDir := t.TempDir()
	tool := NewTodoUpdateWithSessionsDir(tempDir)
	assert.NotNil(t, tool)

	// Check that the tool's sessionsDir is set correctly
	todoUpdateTool, ok := tool.(*todoUpdate)
	require.True(t, ok)
	assert.Equal(t, tempDir, todoUpdateTool.sessionsDir)
}

func TestTodoUpdate_NewTodoUpdateWithSessionsDir_TildeExpansion(t *testing.T) {
	tool := NewTodoUpdateWithSessionsDir("~/.test-sessions")
	assert.NotNil(t, tool)

	// Check that tilde is expanded
	todoUpdateTool, ok := tool.(*todoUpdate)
	require.True(t, ok)

	home, _ := os.UserHomeDir()
	expectedDir := filepath.Join(home, ".test-sessions")
	assert.Equal(t, expectedDir, todoUpdateTool.sessionsDir)
}

func TestTodoUpdate_IntegrationWithRead(t *testing.T) {
	// Integration test: write with update, read with read
	tempDir := t.TempDir()

	updateTool := NewTodoUpdateWithSessionsDir(tempDir)
	readTool := NewTodoReadWithSessionsDir(tempDir)

	sessionID := "integration_test"
	ctx := shared.WithSessionID(context.Background(), sessionID)

	// Create todos
	todos := []any{
		map[string]any{
			"content": "Integration test task",
			"status":  "in_progress",
		},
	}

	// Update
	updateCall := ports.ToolCall{
		ID:   "call-update",
		Name: "todo_update",
		Arguments: map[string]any{
			"todos": todos,
		},
	}

	updateResult, err := updateTool.Execute(ctx, updateCall)
	require.NoError(t, err)
	require.Nil(t, updateResult.Error)

	// Read
	readCall := ports.ToolCall{
		ID:        "call-read",
		Name:      "todo_read",
		Arguments: map[string]any{},
	}

	readResult, err := readTool.Execute(ctx, readCall)
	require.NoError(t, err)
	require.Nil(t, readResult.Error)

	// Verify content matches
	assert.Contains(t, readResult.Content, "Integration test task")
	assert.Contains(t, readResult.Content, "→")
	assert.Equal(t, 1, readResult.Metadata["total_count"])
	assert.Equal(t, 1, readResult.Metadata["in_progress_count"])
}

// Helper function to find substring index
func indexOf(s, substr string) int {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
