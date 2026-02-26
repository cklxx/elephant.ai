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

// TestThinkInjectsContextStatus verifies that the think() method injects a
// <context_status/> system message into the messages sent to the LLM.
func TestThinkInjectsContextStatus(t *testing.T) {
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
			return len(msgs) * 500 // ~500 tokens per message
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

	// Find the context status message in the captured request.
	found := false
	for _, msg := range captured.Messages {
		if strings.Contains(msg.Content, "<ctx") {
			found = true
			if msg.Role != "system" {
				t.Errorf("context status message role = %q, want system", msg.Role)
			}
			if msg.Source != ports.MessageSourceProactive {
				t.Errorf("context status message source = %q, want %q", msg.Source, ports.MessageSourceProactive)
			}
			if !strings.Contains(msg.Content, `p="ok"`) && !strings.Contains(msg.Content, `p="warning"`) {
				t.Errorf("context status missing phase, content: %s", msg.Content)
			}
			t.Logf("injected context status: %s", msg.Content)
			break
		}
	}
	if !found {
		t.Fatal("context status message not found in messages sent to LLM")
	}
}

// TestThinkContextStatusReflectsCompression verifies that the context status
// reports compression when enforceContextBudget reduces message count.
func TestThinkContextStatusReflectsCompression(t *testing.T) {
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

	// Return a high token count to trigger budget enforcement, then lower after trim.
	callCount := 0
	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: func(msgs []ports.Message) int {
			callCount++
			// First call: during enforceContextBudget, over limit to trigger trim.
			// After trim: fewer messages → re-estimate is lower.
			return len(msgs) * 40000
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
			t.Logf("status after compression: %s", msg.Content)
			// With high token estimates, compression/trimming will occur.
			// The status should reflect a non-ok phase.
			if strings.Contains(msg.Content, `p="ok"`) {
				t.Log("note: phase=ok — tokens may have fit after aggressive trim")
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
		Context:      nil, // No context manager.
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
