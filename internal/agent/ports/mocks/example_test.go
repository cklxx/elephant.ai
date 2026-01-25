package mocks_test

import (
	"context"
	"fmt"
	"testing"

	react "alex/internal/agent/domain/react"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/ports/mocks"
)

// Example_basicScenario demonstrates basic usage of a mock scenario
func Example_basicScenario() {
	// Create a file read scenario
	scenario := mocks.NewFileReadScenario()

	// Set up agent services with mocks
	services := react.Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	// Create engine and execute task
	engine := react.NewReactEngine(react.ReactEngineConfig{MaxIterations: 10, Logger: agent.NoopLogger{}, Clock: agent.SystemClock{}})
	state := &react.TaskState{}

	result, err := engine.SolveTask(
		context.Background(),
		"What is the API endpoint?",
		state,
		services,
	)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Iterations: %d\n", result.Iterations)
	fmt.Printf("Stop Reason: %s\n", result.StopReason)
	fmt.Printf("Has Answer: %v\n", result.Answer != "")
	// Output:
	// Iterations: 3
	// Stop Reason: final_answer
	// Has Answer: true
}

// Example_multipleToolCalls demonstrates a scenario with multiple sequential tool calls
func Example_multipleToolCalls() {
	scenario := mocks.NewMultipleToolCallsScenario()

	services := react.Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := react.NewReactEngine(react.ReactEngineConfig{MaxIterations: 10, Logger: agent.NoopLogger{}, Clock: agent.SystemClock{}})
	state := &react.TaskState{}

	result, _ := engine.SolveTask(
		context.Background(),
		"Check if tests pass",
		state,
		services,
	)

	fmt.Printf("Tools executed: %d\n", len(state.ToolResults))
	fmt.Printf("Iterations: %d\n", result.Iterations)
	// Output:
	// Tools executed: 5
	// Iterations: 6
}

// Example_parallelToolCalls demonstrates parallel tool execution
func Example_parallelToolCalls() {
	scenario := mocks.NewParallelToolCallsScenario()

	services := react.Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := react.NewReactEngine(react.ReactEngineConfig{MaxIterations: 10, Logger: agent.NoopLogger{}, Clock: agent.SystemClock{}})
	state := &react.TaskState{}

	result, _ := engine.SolveTask(
		context.Background(),
		"Compare config files",
		state,
		services,
	)

	fmt.Printf("Parallel tools: %d\n", len(state.ToolResults))
	fmt.Printf("Iterations: %d\n", result.Iterations)
	// Output:
	// Parallel tools: 4
	// Iterations: 5
}

// Example_errorHandling demonstrates error handling in tool execution
func Example_errorHandling() {
	scenario := mocks.NewToolErrorScenario()

	services := react.Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := react.NewReactEngine(react.ReactEngineConfig{MaxIterations: 10, Logger: agent.NoopLogger{}, Clock: agent.SystemClock{}})
	state := &react.TaskState{}

	result, _ := engine.SolveTask(
		context.Background(),
		"Read /nonexistent/file.txt",
		state,
		services,
	)

	hasError := false
	for _, res := range state.ToolResults {
		if res.Error != nil {
			hasError = true
			break
		}
	}
	fmt.Printf("Has error: %v\n", hasError)
	fmt.Printf("Iterations: %d\n", result.Iterations)
	// Output:
	// Has error: true
	// Iterations: 4
}

// TestScenarioCustomization shows how to customize scenarios for specific tests
func TestScenarioCustomization(t *testing.T) {
	// Start with a base scenario
	scenario := mocks.NewFileReadScenario()

	// Customize the LLM mock for your specific test
	customLLM := scenario.LLM
	// You can wrap or modify the CompleteFunc as needed

	services := react.Services{
		LLM:          customLLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := react.NewReactEngine(react.ReactEngineConfig{MaxIterations: 10, Logger: agent.NoopLogger{}, Clock: agent.SystemClock{}})
	state := &react.TaskState{}

	result, err := engine.SolveTask(
		context.Background(),
		"Custom task",
		state,
		services,
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Your custom assertions
	if result.Iterations < 1 {
		t.Error("Expected at least one iteration")
	}
}

// TestIteratingAllScenarios shows how to run all scenarios in a loop
func TestIteratingAllScenarios(t *testing.T) {
	scenarios := mocks.GetAllScenarios()

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			services := react.Services{
				LLM:          scenario.LLM,
				ToolExecutor: scenario.Registry,
				Parser:       &mocks.MockParser{},
				Context:      &mocks.MockContextManager{},
			}

			engine := react.NewReactEngine(react.ReactEngineConfig{MaxIterations: 10, Logger: agent.NoopLogger{}, Clock: agent.SystemClock{}})
			state := &react.TaskState{}

			result, err := engine.SolveTask(
				context.Background(),
				scenario.Description,
				state,
				services,
			)

			if err != nil {
				t.Errorf("Scenario %s failed: %v", scenario.Name, err)
			}

			// Common assertions for all scenarios
			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if result.Iterations == 0 {
				t.Error("Expected at least one iteration")
			}

			t.Logf("✓ Scenario '%s': %d iterations, %d tokens",
				scenario.Name, result.Iterations, result.TokensUsed)
		})
	}
}
