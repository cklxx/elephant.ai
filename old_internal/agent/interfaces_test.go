package agent

import (
	"context"
	"testing"

	"alex/internal/llm"
	"alex/internal/session"
	"alex/internal/tools/builtin"
	"alex/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for testing interfaces

// MockLLMClient - Mock implementation of LLMClient interface
type MockLLMClient struct {
	mock.Mock
}

func (m *MockLLMClient) Chat(ctx context.Context, req *llm.ChatRequest, sessionID string) (*llm.ChatResponse, error) {
	args := m.Called(ctx, req, sessionID)
	return args.Get(0).(*llm.ChatResponse), args.Error(1)
}

func (m *MockLLMClient) ChatStream(ctx context.Context, req *llm.ChatRequest, sessionID string) (<-chan llm.StreamDelta, error) {
	args := m.Called(ctx, req, sessionID)
	return args.Get(0).(<-chan llm.StreamDelta), args.Error(1)
}

func (m *MockLLMClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockToolExecutor - Mock implementation of ToolExecutor interface
type MockToolExecutor struct {
	mock.Mock
}

func (m *MockToolExecutor) Execute(ctx context.Context, name string, args map[string]interface{}, callID string) (*types.ReactToolResult, error) {
	mockArgs := m.Called(ctx, name, args, callID)
	return mockArgs.Get(0).(*types.ReactToolResult), mockArgs.Error(1)
}

func (m *MockToolExecutor) ListTools(ctx context.Context) []string {
	args := m.Called(ctx)
	return args.Get(0).([]string)
}

func (m *MockToolExecutor) GetAllToolDefinitions(ctx context.Context) []llm.Tool {
	args := m.Called(ctx)
	return args.Get(0).([]llm.Tool)
}

func (m *MockToolExecutor) GetTool(ctx context.Context, name string) (builtin.Tool, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(builtin.Tool), args.Error(1)
}

// MockSessionManager - Mock implementation of SessionManager interface
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) StartSession(sessionID string) (*session.Session, error) {
	args := m.Called(sessionID)
	return args.Get(0).(*session.Session), args.Error(1)
}

func (m *MockSessionManager) RestoreSession(sessionID string) (*session.Session, error) {
	args := m.Called(sessionID)
	return args.Get(0).(*session.Session), args.Error(1)
}

func (m *MockSessionManager) GetCurrentSession() *session.Session {
	args := m.Called()
	return args.Get(0).(*session.Session)
}

func (m *MockSessionManager) SaveSession(sess *session.Session) error {
	args := m.Called(sess)
	return args.Error(0)
}

// MockMessageProcessor - Mock implementation of MessageProcessor interface
type MockMessageProcessor struct {
	mock.Mock
}

func (m *MockMessageProcessor) ProcessMessage(ctx context.Context, message string, callback StreamCallback) (*types.ReactTaskResult, error) {
	args := m.Called(ctx, message, callback)
	return args.Get(0).(*types.ReactTaskResult), args.Error(1)
}

func (m *MockMessageProcessor) ConvertSessionToLLM(messages []*session.Message) []llm.Message {
	args := m.Called(messages)
	return args.Get(0).([]llm.Message)
}

// MockReactEngine - Mock implementation of ReactEngine interface
type MockReactEngine struct {
	mock.Mock
}

func (m *MockReactEngine) ProcessTask(ctx context.Context, task string, callback StreamCallback) (*types.ReactTaskResult, error) {
	args := m.Called(ctx, task, callback)
	return args.Get(0).(*types.ReactTaskResult), args.Error(1)
}

func (m *MockReactEngine) ExecuteTaskCore(ctx context.Context, execCtx *TaskExecutionContext, callback StreamCallback) (*types.ReactExecutionResult, error) {
	args := m.Called(ctx, execCtx, callback)
	return args.Get(0).(*types.ReactExecutionResult), args.Error(1)
}

// Tests for interface implementations

func TestLLMClientInterface(t *testing.T) {
	mockClient := new(MockLLMClient)

	// Test that mock implements the interface
	var _ LLMClient = mockClient

	// Test Chat method
	ctx := context.Background()
	req := &llm.ChatRequest{Messages: []llm.Message{{Role: "user", Content: "test"}}}
	expectedResponse := &llm.ChatResponse{
		ID:    "test-response",
		Model: "test-model",
		Choices: []llm.Choice{
			{Message: llm.Message{Role: "assistant", Content: "response"}},
		},
	}

	mockClient.On("Chat", ctx, req, "session123").Return(expectedResponse, nil)

	response, err := mockClient.Chat(ctx, req, "session123")

	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)
	mockClient.AssertExpectations(t)
}

func TestToolExecutorInterface(t *testing.T) {
	mockExecutor := new(MockToolExecutor)

	// Test that mock implements the interface
	var _ ToolExecutor = mockExecutor

	// Test Execute method
	ctx := context.Background()
	expectedResult := &types.ReactToolResult{
		Success: true,
		Content: "Tool executed successfully",
	}

	mockExecutor.On("Execute", ctx, "test_tool", map[string]interface{}{"arg": "value"}, "call123").Return(expectedResult, nil)

	result, err := mockExecutor.Execute(ctx, "test_tool", map[string]interface{}{"arg": "value"}, "call123")

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)

	// Test ListTools method
	expectedTools := []string{"tool1", "tool2", "tool3"}
	mockExecutor.On("ListTools", ctx).Return(expectedTools)

	tools := mockExecutor.ListTools(ctx)
	assert.Equal(t, expectedTools, tools)

	mockExecutor.AssertExpectations(t)
}

func TestSessionManagerInterface(t *testing.T) {
	mockManager := new(MockSessionManager)

	// Test that mock implements the interface
	var _ SessionManager = mockManager

	// Test StartSession method
	expectedSession := &session.Session{ID: "test_session"}
	mockManager.On("StartSession", "test_session").Return(expectedSession, nil)

	sess, err := mockManager.StartSession("test_session")

	assert.NoError(t, err)
	assert.Equal(t, expectedSession, sess)

	// Test GetCurrentSession method
	mockManager.On("GetCurrentSession").Return(expectedSession)

	currentSess := mockManager.GetCurrentSession()
	assert.Equal(t, expectedSession, currentSess)

	mockManager.AssertExpectations(t)
}

func TestMessageProcessorInterface(t *testing.T) {
	mockProcessor := new(MockMessageProcessor)

	// Test that mock implements the interface
	var _ MessageProcessor = mockProcessor

	// Test ProcessMessage method
	ctx := context.Background()
	expectedResult := &types.ReactTaskResult{
		Success: true,
		Answer:  "Processed successfully",
	}

	var callback StreamCallback = func(chunk StreamChunk) {}

	mockProcessor.On("ProcessMessage", ctx, "test message", mock.AnythingOfType("agent.StreamCallback")).Return(expectedResult, nil)

	result, err := mockProcessor.ProcessMessage(ctx, "test message", callback)

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)

	mockProcessor.AssertExpectations(t)
}

func TestReactEngineInterface(t *testing.T) {
	mockEngine := new(MockReactEngine)

	// Test that mock implements the interface
	var _ ReactEngine = mockEngine

	// Test ProcessTask method
	ctx := context.Background()
	expectedResult := &types.ReactTaskResult{
		Success: true,
		Answer:  "Task processed successfully",
	}

	var callback StreamCallback = func(chunk StreamChunk) {}

	mockEngine.On("ProcessTask", ctx, "test task", mock.AnythingOfType("agent.StreamCallback")).Return(expectedResult, nil)

	result, err := mockEngine.ProcessTask(ctx, "test task", callback)

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)

	mockEngine.AssertExpectations(t)
}

func TestStreamCallback(t *testing.T) {
	// Test that StreamCallback can be defined and called
	var receivedChunk StreamChunk

	callback := func(chunk StreamChunk) {
		receivedChunk = chunk
	}

	testChunk := StreamChunk{
		Type:     "text",
		Content:  "test content",
		Complete: false,
	}

	callback(testChunk)

	assert.Equal(t, testChunk, receivedChunk)
}
