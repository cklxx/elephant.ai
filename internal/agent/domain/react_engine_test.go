package domain_test

import (
	"context"
	"fmt"
	"testing"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/ports/mocks"
)

func TestReactEngine_SolveTask_SingleIteration(t *testing.T) {
	// Arrange
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			return &ports.CompletionResponse{
				Content:    "The answer is 42. This is the final answer.",
				StopReason: "stop",
				Usage:      ports.TokenUsage{TotalTokens: 50},
			}, nil
		},
	}

	mockTools := &mocks.MockToolRegistry{}
	mockParser := &mocks.MockParser{}
	mockContext := &mocks.MockContextManager{}

	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	engine := newReactEngineForTest(10)
	state := &domain.TaskState{}

	// Act
	result, err := engine.SolveTask(context.Background(), "What is the answer?", state, services)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if result.Iterations != 1 {
		t.Errorf("Expected 1 iteration, got %d", result.Iterations)
	}
	if result.StopReason != "final_answer" {
		t.Errorf("Expected stop reason 'final_answer', got '%s'", result.StopReason)
	}
}

func TestReactEngine_AppendsRAGContextAfterUserInput(t *testing.T) {
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			return &ports.CompletionResponse{Content: "Done.", StopReason: "stop"}, nil
		},
	}
	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}
	engine := newReactEngineForTest(1)
	state := &domain.TaskState{
		SystemPrompt: "Follow the user objective.",
		Messages: []ports.Message{
			{Role: "system", Content: "History", Source: ports.MessageSourceUserHistory},
			{Role: "assistant", Content: "Context loader output", Source: ports.MessageSourceToolResult, Metadata: map[string]any{"rag_preload": true}},
		},
	}

	if _, err := engine.SolveTask(context.Background(), "analyze repo", state, services); err != nil {
		t.Fatalf("SolveTask returned error: %v", err)
	}

	userIdx := -1
	ragIdx := -1
	for idx, msg := range state.Messages {
		if msg.Source == ports.MessageSourceUserInput && msg.Content == "analyze repo" {
			userIdx = idx
		}
		if msg.Metadata != nil {
			if flagged, ok := msg.Metadata["rag_preload"].(bool); ok && flagged {
				ragIdx = idx
			}
		}
	}
	if userIdx == -1 {
		t.Fatalf("expected user input message to be recorded: %+v", state.Messages)
	}
	if ragIdx == -1 {
		t.Fatalf("expected preloaded message to remain present: %+v", state.Messages)
	}
	if ragIdx <= userIdx {
		t.Fatalf("expected preloaded context to follow user input, userIdx=%d ragIdx=%d", userIdx, ragIdx)
	}
	if state.Messages[0].Source != ports.MessageSourceSystemPrompt {
		t.Fatalf("expected system prompt to remain first, got source %q", state.Messages[0].Source)
	}
}

func TestReactEngine_SolveTask_WithToolCall(t *testing.T) {
	// Arrange
	callCount := 0
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			callCount++
			if callCount == 1 {
				// First call: request tool
				return &ports.CompletionResponse{
					Content: "I need to read the file",
					ToolCalls: []ports.ToolCall{
						{ID: "call1", Name: "file_read", Arguments: map[string]any{"path": "test.txt"}},
					},
					StopReason: "tool_calls",
				}, nil
			}
			// Second call: final answer
			return &ports.CompletionResponse{
				Content:    "The file contains: mock content. Final answer.",
				StopReason: "stop",
			}, nil
		},
	}

	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (ports.ToolExecutor, error) {
			return &mocks.MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					return &ports.ToolResult{
						CallID:  call.ID,
						Content: "mock file content",
					}, nil
				},
			}, nil
		},
	}

	mockParser := &mocks.MockParser{}
	mockContext := &mocks.MockContextManager{}

	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	engine := newReactEngineForTest(10)
	state := &domain.TaskState{}

	// Act
	result, err := engine.SolveTask(context.Background(), "Read test.txt", state, services)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Iterations != 2 {
		t.Errorf("Expected 2 iterations, got %d", result.Iterations)
	}
	if len(state.ToolResults) != 1 {
		t.Errorf("Expected 1 tool result, got %d", len(state.ToolResults))
	}
}

func TestReactEngine_UsesCompletionDefaults(t *testing.T) {
	// Arrange
	var capturedReq ports.CompletionRequest
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			capturedReq = req
			return &ports.CompletionResponse{
				Content:    "Final answer.",
				StopReason: "stop",
			}, nil
		},
	}

	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	temp := 0.25
	maxTokens := 4096
	topP := 0.85
	engine := domain.NewReactEngine(domain.ReactEngineConfig{
		MaxIterations: 1,
		CompletionDefaults: domain.CompletionDefaults{
			Temperature:   &temp,
			MaxTokens:     &maxTokens,
			TopP:          &topP,
			StopSequences: []string{"STOP"},
		},
	})

	// Act
	result, err := engine.SolveTask(context.Background(), "Provide answer", &domain.TaskState{}, services)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if capturedReq.Temperature != temp {
		t.Errorf("expected temperature %.2f, got %.2f", temp, capturedReq.Temperature)
	}
	if capturedReq.MaxTokens != maxTokens {
		t.Errorf("expected max tokens %d, got %d", maxTokens, capturedReq.MaxTokens)
	}
	if capturedReq.TopP != topP {
		t.Errorf("expected top-p %.2f, got %.2f", topP, capturedReq.TopP)
	}
	if len(capturedReq.StopSequences) != 1 || capturedReq.StopSequences[0] != "STOP" {
		t.Fatalf("expected stop sequences [STOP], got %v", capturedReq.StopSequences)
	}
}

func TestReactEngine_SolveTask_MaxIterations(t *testing.T) {
	// Arrange
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			// Always request more tools (never finishes)
			return &ports.CompletionResponse{
				Content: "Let me check another thing",
				ToolCalls: []ports.ToolCall{
					{ID: "call", Name: "think", Arguments: map[string]any{"thought": "thinking"}},
				},
				StopReason: "tool_calls",
			}, nil
		},
	}

	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (ports.ToolExecutor, error) {
			return &mocks.MockToolExecutor{}, nil
		},
	}

	mockParser := &mocks.MockParser{}
	mockContext := &mocks.MockContextManager{}

	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	maxIter := 3
	engine := newReactEngineForTest(maxIter)
	state := &domain.TaskState{}

	// Act
	result, err := engine.SolveTask(context.Background(), "Test task", state, services)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Iterations != maxIter {
		t.Errorf("Expected %d iterations, got %d", maxIter, result.Iterations)
	}
	if result.StopReason != "max_iterations" {
		t.Errorf("Expected stop reason 'max_iterations', got '%s'", result.StopReason)
	}
}

func TestReactEngine_SolveTask_ToolError(t *testing.T) {
	// Arrange
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			return &ports.CompletionResponse{
				Content: "Using tool",
				ToolCalls: []ports.ToolCall{
					{ID: "call1", Name: "nonexistent_tool", Arguments: map[string]any{}},
				},
			}, nil
		},
	}

	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (ports.ToolExecutor, error) {
			// Tool not found - return error
			return nil, fmt.Errorf("tool not found: %s", name)
		},
	}

	mockParser := &mocks.MockParser{}
	mockContext := &mocks.MockContextManager{}

	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	engine := newReactEngineForTest(10)
	state := &domain.TaskState{}

	// Act
	result, err := engine.SolveTask(context.Background(), "Test", state, services)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	// LLM doesn't provide more responses, so reaches max iterations
	if result.StopReason != "max_iterations" {
		t.Errorf("Expected stop reason 'max_iterations' (LLM decides when to stop), got '%s'", result.StopReason)
	}
}

type capturingListener struct {
	events []domain.AgentEvent
}

func (c *capturingListener) OnEvent(evt domain.AgentEvent) {
	c.events = append(c.events, evt)
}

func TestReactEngine_EventListenerReceivesEvents(t *testing.T) {
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			return &ports.CompletionResponse{
				Content:    "",
				StopReason: "continue",
			}, nil
		},
	}

	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(1)
	listener := &capturingListener{}
	engine.SetEventListener(listener)

	if engine.GetEventListener() != listener {
		t.Fatalf("expected listener to be registered")
	}

	_, err := engine.SolveTask(context.Background(), "summarize", &domain.TaskState{SessionID: "sess"}, services)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(listener.events) == 0 {
		t.Fatalf("expected events to be captured")
	}

	foundTaskComplete := false
	for _, evt := range listener.events {
		if evt.EventType() == "task_complete" {
			foundTaskComplete = true
			break
		}
	}
	if !foundTaskComplete {
		t.Fatalf("expected task_complete event in captured events")
	}
}

func TestReactEngine_TaskCompleteSkipsUnreferencedGeneratedAttachments(t *testing.T) {
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			return &ports.CompletionResponse{
				Content:    "All done.",
				StopReason: "stop",
			}, nil
		},
	}

	services := domain.Services{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(1)
	state := &domain.TaskState{
		Attachments: map[string]ports.Attachment{
			"cat.png": {
				Name:      "cat.png",
				MediaType: "image/png",
				Data:      "YmFzZTY0",
				Source:    "seedream",
			},
			"user.png": {
				Name:      "user.png",
				MediaType: "image/png",
				Source:    "user_upload",
			},
		},
		AttachmentIterations: map[string]int{
			"cat.png":  1,
			"user.png": 0,
		},
		Iterations: 0,
	}

	var finalEvent *domain.TaskCompleteEvent
	engine.SetEventListener(domain.EventListenerFunc(func(evt domain.AgentEvent) {
		if e, ok := evt.(*domain.TaskCompleteEvent); ok {
			finalEvent = e
		}
	}))

	if _, err := engine.SolveTask(context.Background(), "Describe the latest assets", state, services); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if finalEvent == nil {
		t.Fatal("expected a task complete event to be emitted")
	}

	if len(finalEvent.Attachments) != 0 {
		t.Fatalf("expected no attachments to be emitted, got %d", len(finalEvent.Attachments))
	}
	if finalEvent.FinalAnswer != "All done." {
		t.Fatalf("expected final answer to remain unchanged, got %q", finalEvent.FinalAnswer)
	}
}
