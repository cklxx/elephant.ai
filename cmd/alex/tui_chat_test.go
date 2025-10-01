package main

import (
	"testing"
	"time"

	"alex/internal/agent/app"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// ═══════════════════════════════════════════════════════════════
// Data Structure Tests
// ═══════════════════════════════════════════════════════════════

func TestChatMessage_Creation(t *testing.T) {
	msg := ChatMessage{
		ID:        "test-1",
		Role:      RoleUser,
		Content:   "Hello, ALEX!",
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	assert.Equal(t, "test-1", msg.ID)
	assert.Equal(t, RoleUser, msg.Role)
	assert.Equal(t, "Hello, ALEX!", msg.Content)
	assert.NotNil(t, msg.Metadata)
}

func TestMessageRoles(t *testing.T) {
	tests := []struct {
		name     string
		role     MessageRole
		expected string
	}{
		{"User Role", RoleUser, "user"},
		{"Assistant Role", RoleAssistant, "assistant"},
		{"System Role", RoleSystem, "system"},
		{"Tool Role", RoleTool, "tool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.role))
		})
	}
}

func TestChatState_Values(t *testing.T) {
	tests := []struct {
		name     string
		state    ChatState
		expected int
	}{
		{"Idle State", StateIdle, 0},
		{"Waiting For Input", StateWaitingForInput, 1},
		{"Processing Request", StateProcessingRequest, 2},
		{"Streaming Response", StateStreamingResponse, 3},
		{"Executing Tools", StateExecutingTools, 4},
		{"Error State", StateError, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, int(tt.state))
		})
	}
}

func TestToolExecution_Structure(t *testing.T) {
	exec := ToolExecution{
		CallID:    "call-123",
		Name:      "file_read",
		Arguments: map[string]interface{}{"path": "/test/file.go"},
		StartTime: time.Now(),
		Duration:  100 * time.Millisecond,
	}

	assert.Equal(t, "call-123", exec.CallID)
	assert.Equal(t, "file_read", exec.Name)
	assert.NotNil(t, exec.Arguments)
	assert.Equal(t, 100*time.Millisecond, exec.Duration)
}

// ═══════════════════════════════════════════════════════════════
// Model Initialization Tests
// ═══════════════════════════════════════════════════════════════

func TestNewChatTUIModel_Initialization(t *testing.T) {
	// Note: This test would need a mock container
	// For now, we test the basic structure
	sessionID := "test-session-123"

	model := ChatTUIModel{
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
		sessionID:   sessionID,
		state:       StateWaitingForInput,
	}

	assert.Equal(t, sessionID, model.sessionID)
	assert.Equal(t, StateWaitingForInput, model.state)
	assert.Empty(t, model.messages)
	assert.Empty(t, model.activeTools)
}

func TestChatTUIModel_Init(t *testing.T) {
	model := ChatTUIModel{
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
		state:       StateWaitingForInput,
	}

	cmd := model.Init()
	assert.NotNil(t, cmd)
}

// ═══════════════════════════════════════════════════════════════
// Update Handler Tests
// ═══════════════════════════════════════════════════════════════

func TestUpdate_WindowSizeMsg(t *testing.T) {
	model := ChatTUIModel{
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
		state:       StateWaitingForInput,
	}

	msg := tea.WindowSizeMsg{
		Width:  100,
		Height: 40,
	}

	updatedModel, cmd := model.Update(msg)
	assert.NotNil(t, updatedModel)
	assert.Nil(t, cmd)

	chatModel := updatedModel.(ChatTUIModel)
	assert.Equal(t, 100, chatModel.width)
	assert.Equal(t, 40, chatModel.height)
}

func TestUpdate_WelcomeMsg(t *testing.T) {
	model := ChatTUIModel{
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
		state:       StateWaitingForInput,
	}

	msg := WelcomeMsg{}

	updatedModel, cmd := model.Update(msg)
	assert.NotNil(t, updatedModel)
	assert.Nil(t, cmd)

	chatModel := updatedModel.(ChatTUIModel)
	assert.Equal(t, 1, len(chatModel.messages))
	assert.Equal(t, "welcome", chatModel.messages[0].ID)
	assert.Equal(t, RoleSystem, chatModel.messages[0].Role)
}

func TestUpdate_SetProgramMsg(t *testing.T) {
	model := ChatTUIModel{
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
		state:       StateWaitingForInput,
	}

	mockProgram := &tea.Program{}
	msg := SetProgramMsg{Program: mockProgram}

	updatedModel, cmd := model.Update(msg)
	assert.NotNil(t, updatedModel)
	assert.Nil(t, cmd)

	chatModel := updatedModel.(ChatTUIModel)
	assert.Equal(t, mockProgram, chatModel.program)
}

func TestUpdate_IterationStartMsg(t *testing.T) {
	model := ChatTUIModel{
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
		state:       StateWaitingForInput,
	}

	msg := app.IterationStartMsg{
		Iteration:  1,
		TotalIters: 5,
	}

	updatedModel, cmd := model.Update(msg)
	assert.NotNil(t, updatedModel)
	assert.Nil(t, cmd)

	chatModel := updatedModel.(ChatTUIModel)
	assert.Equal(t, 1, chatModel.currentIteration)
	assert.Equal(t, 5, chatModel.totalIterations)
	assert.Equal(t, StateStreamingResponse, chatModel.state)
}

func TestUpdate_ToolCallStartMsg(t *testing.T) {
	model := ChatTUIModel{
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
		state:       StateWaitingForInput,
	}

	msg := app.ToolCallStartMsg{
		CallID:    "call-123",
		ToolName:  "file_read",
		Arguments: map[string]interface{}{"path": "/test/file.go"},
		Timestamp: time.Now(),
	}

	updatedModel, _ := model.Update(msg)
	chatModel := updatedModel.(ChatTUIModel)

	assert.Equal(t, StateExecutingTools, chatModel.state)
	assert.Equal(t, 1, len(chatModel.activeTools))
	assert.Contains(t, chatModel.activeTools, "call-123")
	assert.Equal(t, "file_read", chatModel.activeTools["call-123"].Name)
}

func TestUpdate_ToolCallCompleteMsg(t *testing.T) {
	model := ChatTUIModel{
		messages: make([]ChatMessage, 0),
		activeTools: map[string]ToolExecution{
			"call-123": {
				CallID:    "call-123",
				Name:      "file_read",
				StartTime: time.Now(),
			},
		},
		state: StateExecutingTools,
	}

	msg := app.ToolCallCompleteMsg{
		CallID:   "call-123",
		Result:   "file content here",
		Duration: 50 * time.Millisecond,
	}

	updatedModel, _ := model.Update(msg)
	chatModel := updatedModel.(ChatTUIModel)

	// Tool should be removed from active tools after completion
	assert.NotContains(t, chatModel.activeTools, "call-123")
}

func TestUpdate_ErrorMsg(t *testing.T) {
	model := ChatTUIModel{
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
		state:       StateProcessingRequest,
	}

	testErr := assert.AnError
	msg := app.ErrorMsg{
		Phase: "execution",
		Error: testErr,
	}

	updatedModel, _ := model.Update(msg)
	chatModel := updatedModel.(ChatTUIModel)

	assert.Equal(t, StateError, chatModel.state)
	assert.Equal(t, testErr, chatModel.err)
}

// ═══════════════════════════════════════════════════════════════
// Helper Function Tests
// ═══════════════════════════════════════════════════════════════

func TestGetStateString(t *testing.T) {
	tests := []struct {
		name     string
		state    ChatState
		expected string
	}{
		{"Idle", StateIdle, "Idle"},
		{"Ready", StateWaitingForInput, "Ready"},
		{"Processing", StateProcessingRequest, "Processing..."},
		{"Thinking", StateStreamingResponse, "Thinking..."},
		{"Error", StateError, "Error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := ChatTUIModel{state: tt.state, activeTools: make(map[string]ToolExecution)}
			result := model.getStateString()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStateString_WithActiveTools(t *testing.T) {
	model := ChatTUIModel{
		state: StateExecutingTools,
		activeTools: map[string]ToolExecution{
			"call-1": {Name: "file_read"},
			"call-2": {Name: "grep"},
		},
	}

	result := model.getStateString()
	assert.Equal(t, "Running 2 tools", result)
}

func TestGetRoleStyle(t *testing.T) {
	model := ChatTUIModel{}

	tests := []MessageRole{
		RoleUser,
		RoleAssistant,
		RoleSystem,
		RoleTool,
	}

	for _, role := range tests {
		t.Run(string(role), func(t *testing.T) {
			style := model.getRoleStyle(role)
			assert.NotNil(t, style)
		})
	}
}

func TestAddSystemMessage(t *testing.T) {
	model := ChatTUIModel{
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
	}

	model.addSystemMessage("Test system message")

	assert.Equal(t, 1, len(model.messages))
	assert.Equal(t, RoleSystem, model.messages[0].Role)
	assert.Equal(t, "Test system message", model.messages[0].Content)
}

// ═══════════════════════════════════════════════════════════════
// View Rendering Tests
// ═══════════════════════════════════════════════════════════════

func TestView_NotReady(t *testing.T) {
	model := ChatTUIModel{
		ready:       false,
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
	}

	view := model.View()
	assert.Equal(t, "Initializing...", view)
}

func TestView_Ready(t *testing.T) {
	model := ChatTUIModel{
		ready:       true,
		width:       100,
		height:      40,
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
		state:       StateWaitingForInput,
		startTime:   time.Now(),
	}

	view := model.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "ALEX Agent Chat")
}

func TestBuildFooter(t *testing.T) {
	model := ChatTUIModel{
		activeTools: make(map[string]ToolExecution),
	}

	footer := model.buildFooter()
	assert.NotEmpty(t, footer)
	assert.Contains(t, footer, "Enter")
	assert.Contains(t, footer, "Ctrl+C")
}

func TestBuildFooter_WithError(t *testing.T) {
	model := ChatTUIModel{
		activeTools: make(map[string]ToolExecution),
		err:         assert.AnError,
	}

	footer := model.buildFooter()
	assert.NotEmpty(t, footer)
	assert.Contains(t, footer, "Error")
}

// ═══════════════════════════════════════════════════════════════
// Integration Tests (Message Flow)
// ═══════════════════════════════════════════════════════════════

func TestMessageFlow_CompleteIteration(t *testing.T) {
	model := ChatTUIModel{
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
		state:       StateWaitingForInput,
	}

	// Start iteration
	updatedModel, _ := model.Update(app.IterationStartMsg{
		Iteration:  1,
		TotalIters: 1,
	})
	model = updatedModel.(ChatTUIModel)
	assert.Equal(t, StateStreamingResponse, model.state)

	// Start tool call
	updatedModel, _ = model.Update(app.ToolCallStartMsg{
		CallID:    "call-1",
		ToolName:  "file_read",
		Arguments: map[string]interface{}{"path": "/test.go"},
		Timestamp: time.Now(),
	})
	model = updatedModel.(ChatTUIModel)
	assert.Equal(t, StateExecutingTools, model.state)
	assert.Equal(t, 1, len(model.activeTools))

	// Complete tool call
	updatedModel, _ = model.Update(app.ToolCallCompleteMsg{
		CallID:   "call-1",
		Result:   "file content",
		Duration: 50 * time.Millisecond,
	})
	model = updatedModel.(ChatTUIModel)
	assert.Equal(t, 0, len(model.activeTools))

	// Complete iteration
	updatedModel, _ = model.Update(app.IterationCompleteMsg{
		TokensUsed: 100,
		ToolsRun:   1,
	})
	model = updatedModel.(ChatTUIModel)

	// Complete task
	updatedModel, _ = model.Update(app.TaskCompleteMsg{
		TotalIterations: 1,
		TotalTokens:     100,
		Duration:        1 * time.Second,
		FinalAnswer:     "Task completed",
	})
	model = updatedModel.(ChatTUIModel)
	assert.Equal(t, StateWaitingForInput, model.state)
}

// ═══════════════════════════════════════════════════════════════
// Custom Message Tests
// ═══════════════════════════════════════════════════════════════

func TestStreamChunkMsg_Structure(t *testing.T) {
	msg := StreamChunkMsg{
		Content: "chunk of text",
		Done:    false,
	}

	assert.Equal(t, "chunk of text", msg.Content)
	assert.False(t, msg.Done)
}

func TestUpdate_StreamingFlow(t *testing.T) {
	model := ChatTUIModel{
		messages:    make([]ChatMessage, 0),
		activeTools: make(map[string]ToolExecution),
		state:       StateStreamingResponse,
	}

	// Send first chunk
	updatedModel, _ := model.Update(StreamChunkMsg{
		Content: "Hello ",
		Done:    false,
	})
	model = updatedModel.(ChatTUIModel)
	assert.NotNil(t, model.streamingMessage)
	assert.Equal(t, "Hello ", model.streamingMessage.Content)

	// Send second chunk
	updatedModel, _ = model.Update(StreamChunkMsg{
		Content: "World!",
		Done:    false,
	})
	model = updatedModel.(ChatTUIModel)
	assert.Equal(t, "Hello World!", model.streamingMessage.Content)

	// Finalize stream
	updatedModel, _ = model.Update(StreamChunkMsg{
		Done: true,
	})
	model = updatedModel.(ChatTUIModel)
	assert.Nil(t, model.streamingMessage)
	assert.Equal(t, 1, len(model.messages))
	assert.Equal(t, "Hello World!", model.messages[0].Content)
}
