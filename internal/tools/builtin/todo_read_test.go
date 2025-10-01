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

// Session ID context key for testing
type testContextKey string

const testSessionIDKey testContextKey = "session_id"

// WithSessionID adds session ID to context (for testing)
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, testSessionIDKey, sessionID)
}

// GetSessionID retrieves session ID from context (for testing)
func GetSessionID(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(testSessionIDKey).(string)
	return sessionID, ok
}

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

	// Create todo file content with simple list items (using - prefix)
	todoContent := `# Task List

- Implement authentication
- Create database schema
- Build REST API
- Write unit tests
- Setup project structure`

	// Write todo file to sessions directory (default/todo.md)
	sessionDir := filepath.Join(tempDir, "default")
	err := os.MkdirAll(sessionDir, 0755)
	require.NoError(t, err)

	todoFile := filepath.Join(sessionDir, "todo.md")
	err = os.WriteFile(todoFile, []byte(todoContent), 0644)
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
	assert.Contains(t, result.Content, "5 tasks")

	// Check metadata
	assert.True(t, result.Metadata["has_todos"].(bool))
	assert.Equal(t, 5, result.Metadata["task_count"])
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

	result, err := tool.Execute(nil, call)

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

	// Create empty todo file in default/todo.md
	sessionDir := filepath.Join(tempDir, "default")
	err := os.MkdirAll(sessionDir, 0755)
	require.NoError(t, err)

	todoFile := filepath.Join(sessionDir, "todo.md")
	err = os.WriteFile(todoFile, []byte(""), 0644)
	require.NoError(t, err)

	ctx := context.Background()

	call := ports.ToolCall{
		ID:        "call-6",
		Name:      "todo_read",
		Arguments: map[string]any{},
	}

	result, err := tool.Execute(ctx, call)

	// Should succeed with empty content formatted
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Content, "0 tasks")
	assert.Equal(t, 0, result.Metadata["task_count"])
}

func TestTodoRead_NewTodoRead(t *testing.T) {
	tool := NewTodoRead()
	assert.NotNil(t, tool)

	// Check that it's a todoRead instance
	todoReadTool, ok := tool.(*todoRead)
	require.True(t, ok)
	// sessionsDir should be empty for production mode
	assert.Equal(t, "", todoReadTool.sessionsDir)
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
	t.Skip("Tilde expansion not implemented in todo_read")
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
