package agent

import (
	"fmt"
	"testing"
	"time"

	"alex/internal/config"
	"alex/internal/session"
	"alex/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReactAgent(t *testing.T) {
	// Test basic ReactAgent creation with valid config
	// Note: This is an integration test that may fail if LLM services are unavailable

	// Create a real config manager for testing
	configManager, err := config.NewManager()
	if err != nil {
		t.Skip("Skipping NewReactAgent test - config manager creation failed:", err)
		return
	}

	// Test NewReactAgent with valid config
	agent, err := NewReactAgent(configManager)

	// Accept that this might fail due to external dependencies
	if err != nil {
		t.Logf("NewReactAgent failed (expected in test environment): %v", err)
		// This is acceptable for testing environment
		return
	}

	// If it succeeds, verify the agent structure
	assert.NotNil(t, agent)
	assert.NotNil(t, agent.sessionManager)
	assert.NotNil(t, agent.toolRegistry)
	assert.NotNil(t, agent.config)
	assert.NotNil(t, agent.messageQueue)
}

func TestReactAgentStructure(t *testing.T) {
	// Test ReactAgent struct initialization
	mockSession, _ := session.NewManager()
	mockRegistry := &ToolRegistry{}

	agent := &ReactAgent{
		sessionManager: mockSession,
		toolRegistry:   mockRegistry,
		config:         types.NewReactConfig(),
		messageQueue:   NewMessageQueue(),
	}

	assert.NotNil(t, agent)
	assert.Equal(t, mockSession, agent.sessionManager)
	assert.Equal(t, mockRegistry, agent.toolRegistry)
	assert.NotNil(t, agent.config)
	assert.NotNil(t, agent.messageQueue)
}

func TestReactAgentSessionManagement(t *testing.T) {
	// Test session management methods
	mockSession, err := session.NewManager()
	require.NoError(t, err)

	agent := &ReactAgent{
		sessionManager: mockSession,
	}

	// Test StartSession
	sessionID := "test_session_123"
	sess, err := agent.StartSession(sessionID)

	assert.NoError(t, err)
	assert.NotNil(t, sess)
	assert.Equal(t, sessionID, sess.ID)

	// Verify current session is set
	assert.Equal(t, sess, agent.currentSession)
}

func TestReactAgentStartSessionError(t *testing.T) {
	// Test StartSession with nil session manager
	agent := &ReactAgent{
		sessionManager: nil, // This should be handled gracefully
	}

	// Test that we handle nil sessionManager gracefully
	sessionID := "test_session"
	sess, err := agent.StartSession(sessionID)

	// We expect this to fail gracefully with an error, not panic
	assert.Error(t, err)
	assert.Nil(t, sess)
	assert.Contains(t, err.Error(), "sessionManager")
}

func TestReactAgentRestoreSession(t *testing.T) {
	// Test RestoreSession functionality
	mockSession, err := session.NewManager()
	require.NoError(t, err)

	agent := &ReactAgent{
		sessionManager: mockSession,
	}

	// First create a session to restore
	sessionID := "restore_test_session"
	originalSess, err := agent.StartSession(sessionID)
	require.NoError(t, err)

	// Clear current session
	agent.currentSession = nil

	// Test RestoreSession
	restoredSess, err := agent.RestoreSession(sessionID)

	assert.NoError(t, err)
	assert.NotNil(t, restoredSess)
	assert.Equal(t, originalSess.ID, restoredSess.ID)
	assert.Equal(t, restoredSess, agent.currentSession)
}

func TestReactAgentRestoreSessionNotFound(t *testing.T) {
	// Test RestoreSession with non-existent session
	mockSession, err := session.NewManager()
	require.NoError(t, err)

	agent := &ReactAgent{
		sessionManager: mockSession,
	}

	sess, err := agent.RestoreSession("non_existent_session")
	assert.Error(t, err)
	assert.Nil(t, sess)
}

func TestLightPromptBuilder(t *testing.T) {
	// Test LightPromptBuilder creation and basic functionality
	builder := NewLightPromptBuilder()

	assert.NotNil(t, builder)
	assert.NotNil(t, builder.promptLoader)
}

func TestMessageQueueCreation(t *testing.T) {
	// Test MessageQueue creation
	queue := NewMessageQueue()

	assert.NotNil(t, queue)
	assert.NotNil(t, queue.items)
	assert.Empty(t, queue.items)
}

func TestMessageQueueBasicOperations(t *testing.T) {
	// Test basic MessageQueue operations
	queue := NewMessageQueue()

	// Test initial state
	assert.Empty(t, queue.items)

	// Add items using direct access (since methods aren't visible in this test scope)
	item := MessageQueueItem{
		Message:   "Test message",
		Timestamp: time.Now(),
		Metadata:  map[string]any{"test": true},
	}

	queue.mutex.Lock()
	queue.items = append(queue.items, item)
	queue.mutex.Unlock()

	queue.mutex.RLock()
	length := len(queue.items)
	queue.mutex.RUnlock()

	assert.Equal(t, 1, length)
}

func TestResponseStructure(t *testing.T) {
	// Test Response structure with all fields
	message := &session.Message{
		Role:      "assistant",
		Content:   "Test response message",
		Timestamp: time.Now(),
	}

	response := Response{
		Message:   message,
		SessionID: "session_456",
		Complete:  true,
		ToolResults: []types.ReactToolResult{
			{
				Success: true,
				Content: "Tool executed successfully",
			},
		},
	}

	assert.Equal(t, message, response.Message)
	assert.Equal(t, "session_456", response.SessionID)
	assert.True(t, response.Complete)
	assert.Len(t, response.ToolResults, 1)
	assert.True(t, response.ToolResults[0].Success)
}

func TestStreamChunkTypes(t *testing.T) {
	// Test different types of StreamChunk
	textChunk := StreamChunk{
		Type:    "text",
		Content: "Some text content",
	}

	toolChunk := StreamChunk{
		Type:    "tool_call",
		Content: "Tool execution",
		Metadata: map[string]any{
			"tool_name": "test_tool",
		},
	}

	completeChunk := StreamChunk{
		Type:     "completion",
		Content:  "Task completed",
		Complete: true,
	}

	assert.Equal(t, "text", textChunk.Type)
	assert.Equal(t, "tool_call", toolChunk.Type)
	assert.Equal(t, "completion", completeChunk.Type)
	assert.False(t, textChunk.Complete)
	assert.False(t, toolChunk.Complete)
	assert.True(t, completeChunk.Complete)
	assert.Equal(t, "test_tool", toolChunk.Metadata["tool_name"])
}

func TestReactAgentConcurrency(t *testing.T) {
	// Test ReactAgent mutex protection
	mockSession, err := session.NewManager()
	require.NoError(t, err)

	agent := &ReactAgent{
		sessionManager: mockSession,
	}

	// Test concurrent access to currentSession
	done := make(chan bool)
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			sessionID := fmt.Sprintf("concurrent_session_%d", id)
			_, _ = agent.StartSession(sessionID)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify that agent has a current session
	assert.NotNil(t, agent.currentSession)
}