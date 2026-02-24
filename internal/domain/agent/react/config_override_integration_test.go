package react

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/mocks"
	tools "alex/internal/domain/agent/ports/tools"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigOverride_IntegrationReactLoop verifies that an update_config tool
// call in iteration 1 causes iteration 2's CompletionRequest to use the new
// temperature and maxTokens values.
func TestConfigOverride_IntegrationReactLoop(t *testing.T) {
	var iteration int
	var capturedRequests []ports.CompletionRequest

	// Iteration 1: LLM returns an update_config tool call.
	// Iteration 2: LLM returns a final answer; we capture the request to verify overrides applied.
	// Note: CompleteFunc is called sequentially from the single-threaded ReAct loop.
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			iteration++
			i := iteration
			capturedRequests = append(capturedRequests, req)

			if i == 1 {
				// Iteration 1: the agent calls update_config.
				return &ports.CompletionResponse{
					Content: "Let me adjust my settings.",
					ToolCalls: []ports.ToolCall{
						{
							ID:   "call-config",
							Name: "update_config",
							Arguments: map[string]any{
								"temperature":    0.05,
								"max_tokens":     2048,
								"max_iterations": 50,
							},
						},
					},
					StopReason: "tool_calls",
				}, nil
			}

			// Iteration 2+: final answer.
			return &ports.CompletionResponse{
				Content:    "Done.",
				StopReason: "stop",
			}, nil
		},
	}

	// The update_config tool is already registered in the registry, but in the
	// integration test we wire it as a mock that stages via the context store.
	updateConfigTool := &mockUpdateConfigTool{}
	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			if name == "update_config" {
				return updateConfigTool, nil
			}
			return nil, nil
		},
		ListFunc: func() []ports.ToolDefinition {
			return []ports.ToolDefinition{
				{Name: "update_config"},
			}
		},
	}

	mockParser := &mocks.MockParser{}

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       mockParser,
		Context:      &mocks.MockContextManager{},
	}

	temp := 0.7
	maxTokens := 12000
	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		CompletionDefaults: CompletionDefaults{
			Temperature: &temp,
			MaxTokens:   &maxTokens,
		},
		FinalAnswerReview: FinalAnswerReviewConfig{
			Enabled: false, // Disable to keep test simple.
		},
	})

	state := &TaskState{}
	result, err := engine.SolveTask(context.Background(), "Do something", state, services)

	// Assertions.
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, iteration, "should have taken exactly 2 LLM calls")
	require.Len(t, capturedRequests, 2)

	// Iteration 1 should use original defaults.
	assert.Equal(t, 0.7, capturedRequests[0].Temperature)
	assert.Equal(t, 12000, capturedRequests[0].MaxTokens)

	// Iteration 2 should use overridden values (applied at iteration boundary).
	assert.Equal(t, 0.05, capturedRequests[1].Temperature)
	assert.Equal(t, 2048, capturedRequests[1].MaxTokens)

	// maxIterations should have been updated to 50.
	assert.Equal(t, 50, engine.maxIterations)
}

// mockUpdateConfigTool stages config overrides via the context store.
type mockUpdateConfigTool struct{}

func (t *mockUpdateConfigTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	store := agent.ConfigOverrideStoreFromContext(ctx)
	if store == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "no store",
			Error:   nil,
		}, nil
	}

	override := agent.ConfigOverride{}
	if v, ok := call.Arguments["temperature"].(float64); ok {
		override.Temperature = &v
	}
	if v := toInt(call.Arguments["max_tokens"]); v > 0 {
		override.MaxTokens = &v
	}
	if v := toInt(call.Arguments["max_iterations"]); v > 0 {
		override.MaxIterations = &v
	}

	if err := store.Stage(override); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "staged",
	}, nil
}

func (t *mockUpdateConfigTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: "update_config"}
}

func (t *mockUpdateConfigTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: "update_config", Category: "ui"}
}

func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case float64:
		return int(x)
	case int64:
		return int(x)
	default:
		return 0
	}
}
