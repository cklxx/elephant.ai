package react

import (
	"context"
	"strings"
	"sync"
	"testing"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/mocks"
)

// TestThinkSkipsContextStatusWhenOK verifies that no context status message
// is injected when the phase is "ok" (low usage, no compression).
func TestThinkSkipsContextStatusWhenOK(t *testing.T) {
	var captured ports.CompletionRequest
	var mu sync.Mutex

	mockLLM := &mocks.MockLLMClient{
		StreamCompleteFunc: func(_ context.Context, req ports.CompletionRequest, cb ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
			mu.Lock()
			captured = req
			mu.Unlock()
			if cb.OnContentDelta != nil {
				cb.OnContentDelta(ports.ContentDelta{Delta: "done", Final: true})
			}
			return &ports.CompletionResponse{
				Content:    "done",
				StopReason: "stop",
				Usage:      ports.TokenUsage{TotalTokens: 50},
			}, nil
		},
	}

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: func(msgs []ports.Message) int {
			return len(msgs) * 500 // Low usage → phase=ok
		},
	}

	services := agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	}

	engine := newReactEngineForTest(10)
	state := &agent.TaskState{
		Iterations: 3,
		Messages: []ports.Message{
			{Role: "system", Content: "system prompt", Source: ports.MessageSourceSystemPrompt},
			{Role: "user", Content: "hello", Source: ports.MessageSourceUserInput},
		},
	}

	_, err := engine.think(context.Background(), state, services)
	if err != nil {
		t.Fatalf("think() failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	for _, msg := range captured.Messages {
		if strings.Contains(msg.Content, "<ctx") {
			t.Fatalf("context status should NOT be injected when phase=ok, got: %s", msg.Content)
		}
	}
}

// TestThinkInjectsContextStatusOnCompression verifies that the context status
// message IS injected when enforceContextBudget compresses messages.
func TestThinkInjectsContextStatusOnCompression(t *testing.T) {
	var captured ports.CompletionRequest
	var mu sync.Mutex

	mockLLM := &mocks.MockLLMClient{
		StreamCompleteFunc: func(_ context.Context, req ports.CompletionRequest, cb ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
			mu.Lock()
			captured = req
			mu.Unlock()
			if cb.OnContentDelta != nil {
				cb.OnContentDelta(ports.ContentDelta{Delta: "done", Final: true})
			}
			return &ports.CompletionResponse{
				Content:    "done",
				StopReason: "stop",
				Usage:      ports.TokenUsage{TotalTokens: 50},
			}, nil
		},
	}

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: func(msgs []ports.Message) int {
			return len(msgs) * 40000 // High usage → triggers trim
		},
		ShouldCompressFunc: func(_ []ports.Message, _ int) bool {
			return true
		},
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) {
			return "summary of earlier conversation", len(msgs)
		},
	}

	ctxLimit := 125000
	services := agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &ctxLimit,
		},
	})

	state := &agent.TaskState{
		Iterations: 5,
		Messages: []ports.Message{
			{Role: "system", Content: "system prompt", Source: ports.MessageSourceSystemPrompt},
			{Role: "user", Content: "msg 1", Source: ports.MessageSourceUserInput},
			{Role: "assistant", Content: "reply 1", Source: ports.MessageSourceAssistantReply},
			{Role: "user", Content: "msg 2", Source: ports.MessageSourceUserInput},
			{Role: "assistant", Content: "reply 2", Source: ports.MessageSourceAssistantReply},
			{Role: "user", Content: "latest", Source: ports.MessageSourceUserInput},
		},
	}

	_, err := engine.think(context.Background(), state, services)
	if err != nil {
		t.Fatalf("think() failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	for _, msg := range captured.Messages {
		if strings.Contains(msg.Content, "<ctx") {
			t.Logf("injected: %s", msg.Content)
			if msg.Role != "system" {
				t.Errorf("role = %q, want system", msg.Role)
			}
			if msg.Source != ports.MessageSourceProactive {
				t.Errorf("source = %q, want %q", msg.Source, ports.MessageSourceProactive)
			}
			if !strings.Contains(msg.Content, `phase="compressed"`) && !strings.Contains(msg.Content, `phase="trimmed"`) {
				t.Errorf("expected non-ok phase, got: %s", msg.Content)
			}
			return
		}
	}
	t.Fatal("context status message not found after compression")
}

// TestThinkNoContextStatusWithoutContextManager verifies that no context status
// is injected when services.Context is nil.
func TestThinkNoContextStatusWithoutContextManager(t *testing.T) {
	var captured ports.CompletionRequest
	var mu sync.Mutex

	mockLLM := &mocks.MockLLMClient{
		StreamCompleteFunc: func(_ context.Context, req ports.CompletionRequest, cb ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
			mu.Lock()
			captured = req
			mu.Unlock()
			if cb.OnContentDelta != nil {
				cb.OnContentDelta(ports.ContentDelta{Delta: "done", Final: true})
			}
			return &ports.CompletionResponse{
				Content:    "done",
				StopReason: "stop",
				Usage:      ports.TokenUsage{TotalTokens: 50},
			}, nil
		},
	}

	services := agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      nil,
	}

	engine := newReactEngineForTest(10)
	state := &agent.TaskState{
		Iterations: 1,
		Messages: []ports.Message{
			{Role: "system", Content: "prompt", Source: ports.MessageSourceSystemPrompt},
			{Role: "user", Content: "hi", Source: ports.MessageSourceUserInput},
		},
	}

	_, err := engine.think(context.Background(), state, services)
	if err != nil {
		t.Fatalf("think() failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	for _, msg := range captured.Messages {
		if strings.Contains(msg.Content, "<ctx") {
			t.Fatal("context status should NOT be injected when Context is nil")
		}
	}
}
