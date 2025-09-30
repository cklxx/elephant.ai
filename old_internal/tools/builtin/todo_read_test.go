package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"alex/internal/session"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTodoReadTool_Name(t *testing.T) {
	tool := CreateTodoReadTool()
	assert.Equal(t, "todo_read", tool.Name())
}

func TestTodoReadTool_Description(t *testing.T) {
	tool := CreateTodoReadTool()
	desc := tool.Description()
	assert.Contains(t, desc, "todo list")
	assert.Contains(t, desc, "session-specific")
	assert.Contains(t, desc, "markdown")
}

func TestTodoReadTool_Parameters(t *testing.T) {
	tool := CreateTodoReadTool()
	params := tool.Parameters()

	// Should have no parameters
	assert.NotNil(t, params)
	assert.Equal(t, "object", params["type"])

	properties, ok := params["properties"].(map[string]interface{})
	assert.True(t, ok)
	assert.Empty(t, properties)
}

func TestTodoReadTool_Validate(t *testing.T) {
	tool := CreateTodoReadTool()

	// Should always pass validation since no parameters required
	err := tool.Validate(map[string]interface{}{})
	assert.NoError(t, err)

	err = tool.Validate(map[string]interface{}{"extra": "param"})
	assert.NoError(t, err)
}

func TestTodoReadTool_Execute_NoSessionManager(t *testing.T) {
	tool := CreateTodoReadTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "session manager")
}

func TestTodoReadTool_Execute_NoSessionID(t *testing.T) {
	// Create a session manager without an active session
	sessionManager, err := session.NewManager()
	require.NoError(t, err)

	tool := CreateTodoReadToolWithSessionManager(sessionManager)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "session ID not provided")
}

func TestTodoReadTool_Execute_NoTodoFile(t *testing.T) {
	// Create session manager
	sessionManager, err := session.NewManager()
	require.NoError(t, err)

	// Create a session to get a session ID
	_, err = sessionManager.StartSession("test_session_123")
	require.NoError(t, err)

	tool := CreateTodoReadToolWithSessionManager(sessionManager)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{})

	// Should succeed but return "no todo file found" message
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Content, "No todo file found")

	// Check data structure
	data := result.Data
	assert.False(t, data["has_todos"].(bool))
	assert.Equal(t, "", data["final_goal"])
	assert.Equal(t, 0, data["total_count"])
	assert.Equal(t, 0, data["pending_count"])
}

func TestTodoReadTool_Execute_WithTodoFile(t *testing.T) {
	// Create session manager
	sessionManager, err := session.NewManager()
	require.NoError(t, err)

	// Create a session
	_, err = sessionManager.StartSession("test_session_456")
	require.NoError(t, err)

	// Create todo file content
	todoContent := `# Project Goals
- ☐ Implement user authentication
- ☒ Create database schema
- ☐ Build REST API
- ☒ Write unit tests

## Notes
This is a test todo file with some tasks.`

	// Write todo file to sessions directory
	sessionsDir := sessionManager.GetSessionsDir()
	todoFile := filepath.Join(sessionsDir, "test_session_456_todo.md")
	err = os.WriteFile(todoFile, []byte(todoContent), 0644)
	require.NoError(t, err)

	tool := CreateTodoReadToolWithSessionManager(sessionManager)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{})

	// Should succeed and return todo content
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, todoContent, result.Content)

	// Check data structure
	data := result.Data
	assert.Equal(t, todoContent, data["content"])
}

func TestTodoReadTool_Execute_FileReadError(t *testing.T) {
	// Create session manager
	sessionManager, err := session.NewManager()
	require.NoError(t, err)

	// Create a session
	_, err = sessionManager.StartSession("test_session_789")
	require.NoError(t, err)

	// Create todo file with restrictive permissions (unreadable)
	sessionsDir := sessionManager.GetSessionsDir()
	todoFile := filepath.Join(sessionsDir, "test_session_789_todo.md")
	err = os.WriteFile(todoFile, []byte("test content"), 0000) // No read permissions
	require.NoError(t, err)

	// Ensure cleanup restores permissions
	t.Cleanup(func() {
		os.Chmod(todoFile, 0644)
		os.Remove(todoFile)
	})

	tool := CreateTodoReadToolWithSessionManager(sessionManager)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{})

	// Should fail with read error
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to read todo file")
}

func TestCreateTodoReadToolWithSessionManager(t *testing.T) {
	sessionManager, err := session.NewManager()
	require.NoError(t, err)

	tool := CreateTodoReadToolWithSessionManager(sessionManager)
	assert.NotNil(t, tool)
	assert.Equal(t, sessionManager, tool.sessionManager)
}

func TestCreateTodoReadTool(t *testing.T) {
	tool := CreateTodoReadTool()
	assert.NotNil(t, tool)
	assert.Nil(t, tool.sessionManager)
}
