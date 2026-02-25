package react

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/domain/agent/ports/mocks"
)

func TestReactEngine_FileReadScenario(t *testing.T) {
	scenario := mocks.NewFileReadScenario()

	services := Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "What is the API endpoint?", state, services)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Iterations != 4 {
		t.Errorf("Expected 4 iterations, got %d", result.Iterations)
	}
	if len(state.ToolResults) != 2 {
		t.Errorf("Expected 2 tool results (plan, file_read), got %d", len(state.ToolResults))
	}
	if result.Answer == "" {
		t.Error("Expected non-empty answer")
	}
	// Verify token tracking
	if result.TokensUsed == 0 {
		t.Error("Expected token usage to be tracked")
	}
}

func TestReactEngine_MultipleToolCallsScenario(t *testing.T) {
	scenario := mocks.NewMultipleToolCallsScenario()

	services := Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "Check if tests pass", state, services)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have 7 iterations: plan + clarify + 3 tool calls + review + final answer
	if result.Iterations != 7 {
		t.Errorf("Expected 7 iterations, got %d", result.Iterations)
	}

	// Should have 5 tool results: plan, clarify, file_read, ripgrep, bash
	if len(state.ToolResults) != 5 {
		t.Errorf("Expected 5 tool results, got %d", len(state.ToolResults))
	}

	// Verify tool execution order by checking call IDs
	for i := range state.ToolResults {
		if i >= len(state.ToolResults) {
			t.Errorf("Missing tool result at index %d", i)
			continue
		}
		// Verify call ID format (should be call_001, call_002, etc.)
		expectedPrefix := "call_"
		if !hasPrefix(state.ToolResults[i].CallID, expectedPrefix) {
			t.Errorf("Tool at index %d has wrong call ID format: got %s", i, state.ToolResults[i].CallID)
		}
	}
}

func TestReactEngine_ParallelToolCallsScenario(t *testing.T) {
	scenario := mocks.NewParallelToolCallsScenario()

	services := Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "Compare config files", state, services)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// 6 iterations: plan + 3 file reads + review + final answer
	if result.Iterations != 6 {
		t.Errorf("Expected 6 iterations, got %d", result.Iterations)
	}

	// Should have 4 tool results
	if len(state.ToolResults) != 4 {
		t.Errorf("Expected 4 tool results, got %d", len(state.ToolResults))
	}

	// All tool results should have same iteration number (parallel execution)
	// This assumes ToolResult has an iteration field or similar tracking
}

func TestReactEngine_WebSearchScenario(t *testing.T) {
	scenario := mocks.NewWebSearchScenario()

	services := Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "What's new in Go 1.22?", state, services)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// 5 iterations: plan, web_search, web_fetch, review, final reasoning
	if result.Iterations != 5 {
		t.Errorf("Expected 5 iterations, got %d", result.Iterations)
	}

	if len(state.ToolResults) != 3 {
		t.Errorf("Expected 3 tool results, got %d", len(state.ToolResults))
	}
}

func TestReactEngine_CodeEditScenario(t *testing.T) {
	scenario := mocks.NewCodeEditScenario()

	services := Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "Add error handling to utils.go", state, services)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// 7 iterations: plan, clarify, file_read, file_edit, bash (test), review, final answer
	if result.Iterations != 7 {
		t.Errorf("Expected 7 iterations, got %d", result.Iterations)
	}

	if len(state.ToolResults) != 5 {
		t.Errorf("Expected 5 tool results, got %d", len(state.ToolResults))
	}
}

func TestReactEngine_ToolErrorScenario(t *testing.T) {
	scenario := mocks.NewToolErrorScenario()

	services := Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "Read /nonexistent/file.txt", state, services)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// LLM decides when to stop - should retry after error
	// Expected flow: file_read fails → find succeeds → final answer
	if result.Iterations < 4 {
		t.Errorf("Expected at least 4 iterations (plan+retry), got %d", result.Iterations)
	}

	if result.StopReason != "final_answer" && result.StopReason != "max_iterations" {
		t.Errorf("Expected stop reason 'final_answer' or 'max_iterations', got '%s'", result.StopReason)
	}

	// Should have at least one tool result with error
	if len(state.ToolResults) < 1 {
		t.Fatalf("Expected at least 1 tool result, got %d", len(state.ToolResults))
	}

	foundError := false
	for _, res := range state.ToolResults {
		if res.Error != nil {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("Expected at least one tool result to have error")
	}
}

func TestReactEngine_FinalAnswerReviewNotTriggeredWithoutTools(t *testing.T) {
	services := Services{
		LLM: &mocks.MockLLMClient{
			CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
				return &ports.CompletionResponse{
					Content:    "hello",
					StopReason: "stop",
					Usage:      ports.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
				}, nil
			},
		},
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "hello", state, services)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Iterations != 1 {
		t.Errorf("Expected 1 iteration, got %d", result.Iterations)
	}
	if len(state.ToolResults) != 0 {
		t.Errorf("Expected 0 tool results, got %d", len(state.ToolResults))
	}
}

func TestReactEngine_FinalAnswerReviewNotTriggeredWhenNoIterationBudget(t *testing.T) {
	scenario := mocks.NewFileReadScenario()
	services := Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(3)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "What is the API endpoint?", state, services)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Iterations != 3 {
		t.Errorf("Expected 3 iterations, got %d", result.Iterations)
	}
}

func TestReactEngine_TodoManagementScenario(t *testing.T) {
	scenario := mocks.NewTodoManagementScenario()

	services := Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "Add tasks and mark first as complete", state, services)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// 6 iterations: plan, todo_read, todo_update (add), todo_update (complete), review, final answer
	if result.Iterations != 6 {
		t.Errorf("Expected 6 iterations, got %d", result.Iterations)
	}

	if len(state.ToolResults) != 4 {
		t.Errorf("Expected 4 tool results, got %d", len(state.ToolResults))
	}
}

func TestReactEngine_SubagentDelegationScenario(t *testing.T) {
	scenario := mocks.NewSubagentDelegationScenario()

	services := Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	state := &TaskState{}

	result, err := engine.SolveTask(context.Background(), "Optimize the codebase", state, services)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// 5 iterations: plan, clarify, subagent call, review, final answer
	if result.Iterations != 5 {
		t.Errorf("Expected 5 iterations, got %d", result.Iterations)
	}

	if len(state.ToolResults) != 3 {
		t.Errorf("Expected 3 tool results, got %d", len(state.ToolResults))
	}

	// Verify subagent result contains analysis
	foundDetail := false
	for _, res := range state.ToolResults {
		if len(res.Content) >= 100 {
			foundDetail = true
			break
		}
	}
	if !foundDetail {
		t.Error("Expected detailed tool result (subagent analysis)")
	}
}

// TestAllScenarios runs all available scenarios
func TestAllScenarios(t *testing.T) {
	scenarios := mocks.GetAllScenarios()

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			services := Services{
				LLM:          scenario.LLM,
				ToolExecutor: scenario.Registry,
				Parser:       &mocks.MockParser{},
				Context:      &mocks.MockContextManager{},
			}

			engine := newReactEngineForTest(10)
			state := &TaskState{}

			result, err := engine.SolveTask(context.Background(), scenario.Description, state, services)

			if err != nil {
				t.Fatalf("Scenario %s failed: %v", scenario.Name, err)
			}

			if result == nil {
				t.Fatalf("Scenario %s returned nil result", scenario.Name)
			}

			if result.Iterations == 0 {
				t.Errorf("Scenario %s: Expected iterations > 0, got %d", scenario.Name, result.Iterations)
			}

			t.Logf("Scenario %s: %d iterations, %d tokens, stop reason: %s",
				scenario.Name, result.Iterations, result.TokensUsed, result.StopReason)
		})
	}
}

// Helper function
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// BenchmarkScenarios benchmarks all scenarios
func BenchmarkScenarios(b *testing.B) {
	scenarios := mocks.GetAllScenarios()

	for _, scenario := range scenarios {
		b.Run(scenario.Name, func(b *testing.B) {
			engine := newReactEngineForTest(10)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				services := Services{
					LLM:          scenario.LLM,
					ToolExecutor: scenario.Registry,
					Parser:       &mocks.MockParser{},
					Context:      &mocks.MockContextManager{},
				}

				state := &TaskState{}
				_, _ = engine.SolveTask(context.Background(), scenario.Description, state, services)
			}
		})
	}
}
