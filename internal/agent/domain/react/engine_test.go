package react

import (
	"context"
	"fmt"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/agent/ports/mocks"
	tools "alex/internal/agent/ports/tools"
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

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

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
	services := Services{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}
	engine := newReactEngineForTest(1)
	state := &TaskState{
		RunID:        "task-abc",
		SystemPrompt: "Follow the user objective.",
		Messages: []ports.Message{
			{Role: "system", Content: "History", Source: ports.MessageSourceUserHistory},
			{Role: "assistant", Content: "Context loader output", Source: ports.MessageSourceToolResult, Metadata: map[string]any{"rag_preload": true, "rag_preload_task_id": "task-abc"}},
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
	if ragIdx >= userIdx {
		t.Fatalf("expected preloaded context to remain before user input, userIdx=%d ragIdx=%d", userIdx, ragIdx)
	}
	if state.Messages[0].Source != ports.MessageSourceSystemPrompt {
		t.Fatalf("expected system prompt to remain first, got source %q", state.Messages[0].Source)
	}
}

func TestReactEngine_PreservesHistoricalPreloadedContext(t *testing.T) {
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			return &ports.CompletionResponse{Content: "Done.", StopReason: "stop"}, nil
		},
	}
	services := Services{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}
	engine := newReactEngineForTest(1)
	state := &TaskState{
		RunID:        "task-def",
		SystemPrompt: "Follow the user objective.",
		Messages: []ports.Message{
			{Role: "system", Content: "History", Source: ports.MessageSourceUserHistory},
			{Role: "user", Content: "previous question", Source: ports.MessageSourceUserInput},
			{Role: "assistant", Content: "old context", Source: ports.MessageSourceToolResult, Metadata: map[string]any{"rag_preload": true, "rag_preload_task_id": "task-old"}},
			{Role: "assistant", Content: "fresh context 1", Source: ports.MessageSourceToolResult, Metadata: map[string]any{"rag_preload": true, "rag_preload_task_id": "task-def"}},
			{Role: "assistant", Content: "fresh context 2", Source: ports.MessageSourceToolResult, Metadata: map[string]any{"rag_preload": true, "rag_preload_task_id": "task-def"}},
		},
	}

	if _, err := engine.SolveTask(context.Background(), "second question", state, services); err != nil {
		t.Fatalf("SolveTask returned error: %v", err)
	}

	oldIdx := -1
	newUserIdx := -1
	var freshIdxs []int
	for idx, msg := range state.Messages {
		if msg.Content == "old context" {
			oldIdx = idx
		}
		if msg.Content == "second question" && msg.Source == ports.MessageSourceUserInput {
			newUserIdx = idx
		}
		if msg.Content == "fresh context 1" || msg.Content == "fresh context 2" {
			freshIdxs = append(freshIdxs, idx)
		}
	}
	if oldIdx == -1 {
		t.Fatalf("expected previous turn context to remain present: %+v", state.Messages)
	}
	if newUserIdx == -1 {
		t.Fatalf("expected new user input to be recorded: %+v", state.Messages)
	}
	if oldIdx > newUserIdx {
		t.Fatalf("expected previous turn context to remain before new user input, oldIdx=%d newIdx=%d", oldIdx, newUserIdx)
	}
	freshBefore := false
	freshAfter := false
	for _, idx := range freshIdxs {
		if idx < newUserIdx {
			freshBefore = true
		}
		if idx > newUserIdx {
			freshAfter = true
		}
	}
	if !freshBefore || freshAfter {
		t.Fatalf("expected preloaded context to remain before new user input: %+v", state.Messages)
	}
}

func TestReactEngine_SolveTask_WithToolCall(t *testing.T) {
	// Arrange
	callCount := 0
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			callCount++
			switch callCount {
			case 1:
				return &ports.CompletionResponse{
					Content: "读取 test.txt 并回答问题。",
					ToolCalls: []ports.ToolCall{
						{
							ID:   "call_plan",
							Name: "plan",
							Arguments: map[string]any{
								"run_id":          "test-run",
								"overall_goal_ui": "读取文件并回答问题。",
								"complexity":      "simple",
							},
						},
					},
					StopReason: "tool_calls",
				}, nil
			case 2:
				return &ports.CompletionResponse{
					Content: "I need to read the file",
					ToolCalls: []ports.ToolCall{
						{
							ID:        "call1",
							Name:      "file_read",
							Arguments: map[string]any{"path": "test.txt"},
						},
					},
					StopReason: "tool_calls",
				}, nil
			default:
				// Final answer
				return &ports.CompletionResponse{
					Content:    "The file contains: mock content. Final answer.",
					StopReason: "stop",
				}, nil
			}
		},
	}

	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			return &mocks.MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					if call.Name == "plan" {
						return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
					}
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

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	// Act
	result, err := engine.SolveTask(context.Background(), "Read test.txt", state, services)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Iterations != 4 {
		t.Errorf("Expected 4 iterations, got %d", result.Iterations)
	}
	if len(state.ToolResults) != 2 {
		t.Errorf("Expected 2 tool results, got %d", len(state.ToolResults))
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

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	temp := 0.25
	maxTokens := 4096
	topP := 0.85
	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 1,
		CompletionDefaults: CompletionDefaults{
			Temperature:   &temp,
			MaxTokens:     &maxTokens,
			TopP:          &topP,
			StopSequences: []string{"STOP"},
		},
	})

	// Act
	result, err := engine.SolveTask(context.Background(), "Provide answer", &TaskState{}, services)

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
					{ID: "call", Name: "file_read", Arguments: map[string]any{"path": "README.md"}},
				},
				StopReason: "tool_calls",
			}, nil
		},
	}

	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			return &mocks.MockToolExecutor{}, nil
		},
	}

	mockParser := &mocks.MockParser{}
	mockContext := &mocks.MockContextManager{}

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	maxIter := 3
	engine := newReactEngineForTest(maxIter)
	state := &TaskState{}

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
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			// Tool not found - return error
			return nil, fmt.Errorf("tool not found: %s", name)
		},
	}

	mockParser := &mocks.MockParser{}
	mockContext := &mocks.MockContextManager{}

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      mockContext,
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

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
	events []AgentEvent
}

func (c *capturingListener) OnEvent(evt AgentEvent) {
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

	services := Services{
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

	state := &TaskState{SessionID: "sess"}
	_, err := engine.SolveTask(context.Background(), "summarize", state, services)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(listener.events) == 0 {
		t.Fatalf("expected events to be captured")
	}

	foundTaskComplete := false
	for _, evt := range listener.events {
		if evt.EventType() == "workflow.result.final" {
			foundTaskComplete = true
			break
		}
	}
	if !foundTaskComplete {
		t.Fatalf("expected workflow.result.final event in captured events")
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

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(1)
	state := &TaskState{
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

	result, err := engine.SolveTask(context.Background(), "Describe the latest assets", state, services)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Answer != "All done." {
		t.Fatalf("expected final answer to remain unchanged, got %q", result.Answer)
	}
}
