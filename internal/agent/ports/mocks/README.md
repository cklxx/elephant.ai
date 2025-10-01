# Mock Scenarios for Agent Testing

This package provides comprehensive mock data and scenarios for testing the ALEX agent's tool calling capabilities.

## Overview

The mock scenarios simulate realistic agent interactions including:
- Single and multiple tool calls
- Parallel tool execution
- Error handling and recovery
- Network-dependent operations (web search, web fetch)
- Complex workflows (code editing, git operations, todo management)
- Subagent delegation

## Usage

### Basic Usage

```go
import (
    "alex/internal/agent/domain"
    "alex/internal/agent/ports/mocks"
    "context"
    "testing"
)

func TestMyAgent(t *testing.T) {
    // Create a scenario
    scenario := mocks.NewFileReadScenario()

    // Set up services with mock LLM and tools
    services := domain.Services{
        LLM:          scenario.LLM,
        ToolExecutor: scenario.Registry,
        Parser:       &mocks.MockParser{},
        Context:      &mocks.MockContextManager{},
    }

    // Run the agent
    engine := domain.NewReactEngine(10)
    state := &domain.TaskState{}

    result, err := engine.SolveTask(context.Background(), "What is the API endpoint?", state, services)

    // Assertions
    if err != nil {
        t.Fatalf("Expected no error, got: %v", err)
    }
    // ... more assertions
}
```

### Using All Scenarios

```go
func TestAllScenarios(t *testing.T) {
    scenarios := mocks.GetAllScenarios()

    for _, scenario := range scenarios {
        t.Run(scenario.Name, func(t *testing.T) {
            // Test each scenario
        })
    }
}
```

## Available Scenarios

### 1. File Read Scenario
**Name:** `file_read`
**Description:** Agent reads a file and provides answer based on content

**Flow:**
1. LLM requests to read configuration file
2. Tool executes and returns JSON content
3. LLM provides final answer based on file content

**Usage:**
```go
scenario := mocks.NewFileReadScenario()
```

### 2. Multiple Tool Calls Scenario
**Name:** `multiple_tools`
**Description:** Agent uses multiple tools sequentially (read file, search code, execute bash)

**Flow:**
1. Read main.go file
2. Search for init function
3. Run tests
4. Provide final answer

**Usage:**
```go
scenario := mocks.NewMultipleToolCallsScenario()
```

### 3. Parallel Tool Calls Scenario
**Name:** `parallel_tools`
**Description:** Agent uses multiple tools in parallel (read multiple files)

**Flow:**
1. Read 3 configuration files in parallel (dev, prod, test)
2. Compare and analyze differences
3. Provide summary of differences

**Usage:**
```go
scenario := mocks.NewParallelToolCallsScenario()
```

### 4. Web Search Scenario
**Name:** `web_search`
**Description:** Agent performs web search and web fetch

**Flow:**
1. Search for "Go 1.22 new features"
2. Fetch detailed information from official blog
3. Summarize findings

**Usage:**
```go
scenario := mocks.NewWebSearchScenario()
```

**Note:** This scenario mocks network operations, no actual HTTP requests are made.

### 5. Code Edit Scenario
**Name:** `code_edit`
**Description:** Agent reads, edits, and tests code changes

**Flow:**
1. Read utils.go file
2. Edit file to add error handling
3. Run tests to verify changes
4. Confirm success

**Usage:**
```go
scenario := mocks.NewCodeEditScenario()
```

### 6. Tool Error Scenario
**Name:** `tool_error`
**Description:** Agent handles tool execution errors gracefully

**Flow:**
1. Attempt to read non-existent file (fails)
2. Engine stops with "completed" status

**Usage:**
```go
scenario := mocks.NewToolErrorScenario()
```

**Note:** The current engine implementation stops when all tools error. Future versions may implement retry/recovery logic.

### 7. Todo Management Scenario
**Name:** `todo_management`
**Description:** Agent reads and updates todo list

**Flow:**
1. Read current todo list (empty)
2. Add 3 new tasks
3. Mark first task as complete
4. Confirm updates

**Usage:**
```go
scenario := mocks.NewTodoManagementScenario()
```

### 8. Subagent Delegation Scenario
**Name:** `subagent_delegation`
**Description:** Agent delegates complex task to subagent

**Flow:**
1. Delegate performance analysis to specialized subagent
2. Receive detailed analysis report
3. Summarize recommendations

**Usage:**
```go
scenario := mocks.NewSubagentDelegationScenario()
```

### 9. Git Operations Scenario
**Name:** `git_operations`
**Description:** Agent performs git operations (history, commit, PR)

**Flow:**
1. Check git history (5 recent commits)
2. Create commit with changes
3. Create pull request
4. Confirm PR creation

**Usage:**
```go
scenario := mocks.NewGitOperationsScenario()
```

## Scenario Structure

Each scenario provides:

```go
type ToolScenario struct {
    Name        string              // Unique scenario identifier
    Description string              // Human-readable description
    LLM         *MockLLMClient     // Mock LLM with pre-programmed responses
    Registry    *MockToolRegistry  // Mock tool registry with tool implementations
}
```

### Mock LLM Client

The `MockLLMClient` simulates LLM responses:
- Tracks iteration count
- Returns different responses based on call count
- Simulates tool call requests
- Provides realistic token usage stats

### Mock Tool Registry

The `MockToolRegistry` provides tool executors:
- Returns mock results for each tool
- Simulates tool execution time
- Can simulate errors and failures

## Testing Patterns

### Test Iterations

```go
if result.Iterations != expectedIterations {
    t.Errorf("Expected %d iterations, got %d", expectedIterations, result.Iterations)
}
```

### Test Tool Results

```go
if len(state.ToolResults) != expectedCount {
    t.Errorf("Expected %d tool results, got %d", expectedCount, len(state.ToolResults))
}
```

### Test Token Usage

```go
if result.TokensUsed == 0 {
    t.Error("Expected token usage to be tracked")
}
```

### Test Stop Reasons

```go
if result.StopReason != "final_answer" {
    t.Errorf("Expected stop reason 'final_answer', got '%s'", result.StopReason)
}
```

## Benchmarking

Run benchmarks for all scenarios:

```bash
go test ./internal/agent/domain/ -bench=BenchmarkScenarios -benchmem
```

## Adding New Scenarios

To add a new scenario:

1. Create a new function following the naming convention:
```go
func NewMyScenario() ToolScenario {
    callCount := 0
    return ToolScenario{
        Name:        "my_scenario",
        Description: "Description of what this scenario tests",
        LLM: &MockLLMClient{
            CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
                callCount++
                // Implement your LLM response logic
            },
        },
        Registry: &MockToolRegistry{
            GetFunc: func(name string) (ports.ToolExecutor, error) {
                // Implement your tool execution logic
            },
        },
    }
}
```

2. Add to `GetAllScenarios()`:
```go
func GetAllScenarios() []ToolScenario {
    return []ToolScenario{
        // ... existing scenarios
        NewMyScenario(),
    }
}
```

3. Create a dedicated test if needed:
```go
func TestReactEngine_MyScenario(t *testing.T) {
    scenario := mocks.NewMyScenario()
    // ... test implementation
}
```

## Best Practices

1. **Realistic Token Counts**: Use realistic token counts in mock responses to test token tracking
2. **Error Handling**: Include error scenarios to test resilience
3. **Iteration Counts**: Test both quick tasks (1-2 iterations) and complex tasks (4-5 iterations)
4. **Tool Results**: Provide realistic tool output that LLM would actually receive
5. **Naming**: Use descriptive scenario names that reflect the test purpose

## Running Tests

```bash
# Run all scenario tests
go test ./internal/agent/domain/ -v -run Scenario

# Run specific scenario
go test ./internal/agent/domain/ -v -run TestReactEngine_FileReadScenario

# Run all tests including scenarios
go test ./internal/agent/domain/ -v

# Run with coverage
go test ./internal/agent/domain/ -cover
```

## See Also

- `internal/agent/domain/react_engine.go` - ReAct engine implementation
- `internal/agent/domain/react_engine_test.go` - Basic unit tests
- `internal/agent/domain/react_engine_scenarios_test.go` - Scenario-based tests
- `internal/agent/ports/` - Port interfaces
