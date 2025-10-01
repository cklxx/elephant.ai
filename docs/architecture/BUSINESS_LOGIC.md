# Business Logic Layer Specification

**Version**: 1.0
**Date**: 2025-09-30
**Parent Document**: [REFACTORING_PROPOSAL.md](./REFACTORING_PROPOSAL.md)

---

## Overview

This document provides detailed specifications for the business logic layer in the refactored ALEX architecture, consisting of:

1. **Domain Layer** - Pure business logic with zero infrastructure dependencies
2. **Application Layer** - Thin orchestration coordinating domain and infrastructure

---

## Table of Contents

1. [Domain Layer: ReAct Engine](#1-domain-layer-react-engine)
2. [Application Layer: Agent Coordinator](#2-application-layer-agent-coordinator)
3. [SubAgent Orchestrator](#3-subagent-orchestrator)
4. [Execution Strategies](#4-execution-strategies)
5. [CLI Layer](#5-cli-layer)

---

## 1. Domain Layer: ReAct Engine

**Package**: `internal/agent/domain/`
**Purpose**: Pure ReAct loop business logic with zero infrastructure dependencies

### 1.1 Core Types

```go
// internal/agent/domain/types.go
package domain

// Message represents a conversation message
type Message struct {
    Role     string            // "user", "assistant", "system"
    Content  string            // Message content
    ToolCalls []ToolCall       // Requested tool calls (if any)
    ToolResults []ToolResult   // Tool execution results (if any)
    Metadata map[string]any    // Additional data
}

// ToolCall represents a tool invocation request
type ToolCall struct {
    ID        string
    Name      string
    Arguments map[string]any
}

// ToolResult is the result of tool execution
type ToolResult struct {
    CallID   string
    Content  string
    Error    error
    Metadata map[string]any
}

// TaskState tracks execution state during ReAct loop
type TaskState struct {
    // Messages in current conversation
    Messages []Message

    // Current iteration count
    Iterations int

    // Total token count
    TokenCount int

    // Tool results accumulated
    ToolResults []ToolResult

    // Whether task is complete
    Complete bool

    // Final answer (if complete)
    FinalAnswer string
}

// TaskResult is the final result of task execution
type TaskResult struct {
    Answer      string
    Messages    []Message
    Iterations  int
    TokensUsed  int
    StopReason  string  // "max_iterations", "final_answer", "error"
}
```

### 1.2 Services Interface

```go
// internal/agent/domain/services.go
package domain

import (
    "context"
    "github.com/yourusername/alex/internal/agent/ports"
)

// Services aggregates all injected dependencies for domain logic
type Services struct {
    LLM          ports.LLMClient
    ToolExecutor ports.ToolRegistry
    Parser       ports.FunctionCallParser
    Context      ports.ContextManager
}
```

### 1.3 ReAct Engine Implementation

```go
// internal/agent/domain/react_engine.go
package domain

import (
    "context"
    "fmt"
)

// ReactEngine orchestrates the Think-Act-Observe cycle
type ReactEngine struct {
    maxIterations int
    stopReasons   []string
}

// NewReactEngine creates a new ReAct engine
func NewReactEngine(maxIterations int) *ReactEngine {
    return &ReactEngine{
        maxIterations: maxIterations,
        stopReasons:   []string{"final_answer", "done", "complete"},
    }
}

// SolveTask is the main ReAct loop - pure business logic
func (e *ReactEngine) SolveTask(
    ctx context.Context,
    task string,
    state *TaskState,
    services Services,
) (*TaskResult, error) {
    // Initialize state if empty
    if len(state.Messages) == 0 {
        state.Messages = []Message{
            {Role: "user", Content: task},
        }
    }

    // ReAct loop: Think → Act → Observe
    for state.Iterations < e.maxIterations {
        state.Iterations++

        // 1. THINK: Get LLM reasoning
        thought, err := e.think(ctx, state, services)
        if err != nil {
            return nil, fmt.Errorf("think step failed: %w", err)
        }

        // Add thought to state
        state.Messages = append(state.Messages, thought)

        // 2. Check if final answer reached
        if e.isFinalAnswer(thought) {
            return e.finalize(state, "final_answer"), nil
        }

        // 3. ACT: Parse and execute tool calls
        toolCalls := e.parseToolCalls(thought, services.Parser)
        if len(toolCalls) == 0 {
            // No tool calls means final answer in content
            return e.finalize(state, "final_answer"), nil
        }

        results := e.executeTools(ctx, toolCalls, services.ToolExecutor)
        state.ToolResults = append(state.ToolResults, results...)

        // 4. OBSERVE: Add results to conversation
        observation := e.buildObservation(results)
        state.Messages = append(state.Messages, observation)

        // 5. Check context limits
        tokenCount := services.Context.EstimateTokens(convertMessagesToPortsMessages(state.Messages))
        state.TokenCount = tokenCount

        // Check stop conditions
        if e.shouldStop(state, results) {
            return e.finalize(state, "completed"), nil
        }
    }

    // Max iterations reached
    return e.finalize(state, "max_iterations"), nil
}

// think sends current state to LLM for reasoning
func (e *ReactEngine) think(
    ctx context.Context,
    state *TaskState,
    services Services,
) (Message, error) {
    // Convert state to LLM request
    req := ports.CompletionRequest{
        Messages:    convertMessagesToPortsMessages(state.Messages),
        Tools:       services.ToolExecutor.List(),
        Temperature: 0.7,
        MaxTokens:   4000,
    }

    // Call LLM
    resp, err := services.LLM.Complete(ctx, req)
    if err != nil {
        return Message{}, fmt.Errorf("LLM call failed: %w", err)
    }

    // Convert response to domain message
    return Message{
        Role:      "assistant",
        Content:   resp.Content,
        ToolCalls: convertToolCalls(resp.ToolCalls),
    }, nil
}

// executeTools runs all tool calls in parallel
func (e *ReactEngine) executeTools(
    ctx context.Context,
    calls []ToolCall,
    registry ports.ToolRegistry,
) []ToolResult {
    results := make([]ToolResult, len(calls))

    // Execute in parallel using goroutines
    var wg sync.WaitGroup
    for i, call := range calls {
        wg.Add(1)
        go func(idx int, tc ToolCall) {
            defer wg.Done()

            tool, err := registry.Get(tc.Name)
            if err != nil {
                results[idx] = ToolResult{
                    CallID:  tc.ID,
                    Content: "",
                    Error:   fmt.Errorf("tool not found: %s", tc.Name),
                }
                return
            }

            result, err := tool.Execute(ctx, ports.ToolCall{
                ID:        tc.ID,
                Name:      tc.Name,
                Arguments: tc.Arguments,
            })

            if err != nil {
                results[idx] = ToolResult{
                    CallID:  tc.ID,
                    Content: "",
                    Error:   err,
                }
                return
            }

            results[idx] = ToolResult{
                CallID:   result.CallID,
                Content:  result.Content,
                Error:    result.Error,
                Metadata: result.Metadata,
            }
        }(i, call)
    }

    wg.Wait()
    return results
}

// parseToolCalls extracts tool calls from assistant message
func (e *ReactEngine) parseToolCalls(msg Message, parser ports.FunctionCallParser) []ToolCall {
    // If message has explicit tool calls (native function calling)
    if len(msg.ToolCalls) > 0 {
        return msg.ToolCalls
    }

    // Otherwise, parse from content (XML or JSON format)
    parsed, err := parser.Parse(msg.Content)
    if err != nil {
        return nil
    }

    // Convert ports.ToolCall to domain.ToolCall
    var calls []ToolCall
    for _, p := range parsed {
        calls = append(calls, ToolCall{
            ID:        p.ID,
            Name:      p.Name,
            Arguments: p.Arguments,
        })
    }

    return calls
}

// buildObservation creates a message with tool results
func (e *ReactEngine) buildObservation(results []ToolResult) Message {
    var content string

    for _, result := range results {
        if result.Error != nil {
            content += fmt.Sprintf("Tool %s failed: %v\n", result.CallID, result.Error)
        } else {
            content += fmt.Sprintf("Tool %s result:\n%s\n", result.CallID, result.Content)
        }
    }

    return Message{
        Role:        "user",  // Observations come back as user messages
        Content:     content,
        ToolResults: results,
    }
}

// isFinalAnswer checks if message contains final answer
func (e *ReactEngine) isFinalAnswer(msg Message) bool {
    for _, reason := range e.stopReasons {
        if contains(msg.Content, reason) {
            return true
        }
    }
    return len(msg.ToolCalls) == 0 && msg.Content != ""
}

// shouldStop determines if ReAct loop should terminate
func (e *ReactEngine) shouldStop(state *TaskState, results []ToolResult) bool {
    // Stop if all tools errored
    allErrored := true
    for _, r := range results {
        if r.Error == nil {
            allErrored = false
            break
        }
    }

    return allErrored
}

// finalize creates the final task result
func (e *ReactEngine) finalize(state *TaskState, stopReason string) *TaskResult {
    // Extract final answer from last assistant message
    var finalAnswer string
    for i := len(state.Messages) - 1; i >= 0; i-- {
        if state.Messages[i].Role == "assistant" {
            finalAnswer = state.Messages[i].Content
            break
        }
    }

    return &TaskResult{
        Answer:      finalAnswer,
        Messages:    state.Messages,
        Iterations:  state.Iterations,
        TokensUsed:  state.TokenCount,
        StopReason:  stopReason,
    }
}

// Helper functions
func convertMessagesToPortsMessages(messages []Message) []ports.Message {
    // Implementation...
}

func convertToolCalls(calls []ports.ToolCall) []ToolCall {
    // Implementation...
}

func contains(s, substr string) bool {
    return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
```

### 1.4 Domain Layer Testing

```go
// internal/agent/domain/react_engine_test.go
package domain_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/yourusername/alex/internal/agent/domain"
    "github.com/yourusername/alex/internal/agent/ports/mocks"
)

func TestReactEngine_SolveTask_SingleToolCall(t *testing.T) {
    // Arrange: Create mocks
    mockLLM := &mocks.MockLLMClient{}
    mockTools := &mocks.MockToolRegistry{}
    mockParser := &mocks.MockParser{}
    mockContext := &mocks.MockContextManager{}

    services := domain.Services{
        LLM:          mockLLM,
        ToolExecutor: mockTools,
        Parser:       mockParser,
        Context:      mockContext,
    }

    engine := domain.NewReactEngine(10)

    // Setup expectations: LLM requests tool, then provides final answer
    mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(req ports.CompletionRequest) bool {
        return len(req.Messages) == 1  // First call
    })).Return(&ports.CompletionResponse{
        Content: "I need to read the file",
        ToolCalls: []ports.ToolCall{
            {ID: "call1", Name: "file_read", Arguments: map[string]any{"path": "test.go"}},
        },
    }, nil).Once()

    mockTools.On("List").Return([]ports.ToolDefinition{})
    mockTools.On("Get", "file_read").Return(&mocks.MockToolExecutor{
        ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
            return &ports.ToolResult{
                CallID:  call.ID,
                Content: "package main\n\nfunc main() {}",
            }, nil
        },
    }, nil)

    mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(req ports.CompletionRequest) bool {
        return len(req.Messages) == 3  // Second call after tool result
    })).Return(&ports.CompletionResponse{
        Content: "The file contains a simple main function. Final answer: It's a basic Go program.",
    }, nil).Once()

    mockContext.On("EstimateTokens", mock.Anything).Return(100)

    // Act
    state := &domain.TaskState{}
    result, err := engine.SolveTask(context.Background(), "what's in test.go?", state, services)

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, "final_answer", result.StopReason)
    assert.Contains(t, result.Answer, "basic Go program")
    assert.Equal(t, 2, result.Iterations)

    mockLLM.AssertExpectations(t)
    mockTools.AssertExpectations(t)
}

func TestReactEngine_SolveTask_MaxIterations(t *testing.T) {
    // Test that engine stops after max iterations
    mockLLM := &mocks.MockLLMClient{}
    mockTools := &mocks.MockToolRegistry{}
    mockParser := &mocks.MockParser{}
    mockContext := &mocks.MockContextManager{}

    services := domain.Services{
        LLM:          mockLLM,
        ToolExecutor: mockTools,
        Parser:       mockParser,
        Context:      mockContext,
    }

    engine := domain.NewReactEngine(3)  // Only 3 iterations

    // Setup: LLM always requests more tools
    mockLLM.On("Complete", mock.Anything, mock.Anything).Return(&ports.CompletionResponse{
        Content: "Let me check another file",
        ToolCalls: []ports.ToolCall{
            {ID: "call", Name: "file_read", Arguments: map[string]any{"path": "test.go"}},
        },
    }, nil)

    mockTools.On("List").Return([]ports.ToolDefinition{})
    mockTools.On("Get", "file_read").Return(&mocks.MockToolExecutor{}, nil)
    mockContext.On("EstimateTokens", mock.Anything).Return(100)

    // Act
    state := &domain.TaskState{}
    result, err := engine.SolveTask(context.Background(), "test task", state, services)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "max_iterations", result.StopReason)
    assert.Equal(t, 3, result.Iterations)
}

func TestReactEngine_SolveTask_ToolExecutionError(t *testing.T) {
    // Test error handling during tool execution
    // ... implementation
}

func TestReactEngine_SolveTask_NoToolCalls(t *testing.T) {
    // Test when LLM provides direct answer without tools
    // ... implementation
}
```

**Key Characteristics of Domain Layer**:
- ✅ Zero imports from `internal/llm`, `internal/tools`, `internal/session`, etc.
- ✅ All dependencies injected via `Services` interface bundle
- ✅ 100% testable with mocks
- ✅ Pure business logic - no side effects
- ✅ Fast tests (no real LLM calls, no I/O)

---

## 2. Application Layer: Agent Coordinator

**Package**: `internal/agent/app/`
**Purpose**: Thin orchestration layer coordinating domain and infrastructure

### 2.1 Agent Coordinator Implementation

```go
// internal/agent/app/coordinator.go
package app

import (
    "context"
    "fmt"

    "github.com/yourusername/alex/internal/agent/domain"
    "github.com/yourusername/alex/internal/agent/ports"
    "github.com/yourusername/alex/internal/llm"
)

// AgentCoordinator manages session lifecycle and delegates to domain
type AgentCoordinator struct {
    // Infrastructure dependencies (injected)
    llmFactory   *llm.Factory
    toolRegistry ports.ToolRegistry
    sessionStore ports.SessionStore
    contextMgr   ports.ContextManager
    parser       ports.FunctionCallParser
    messageQueue ports.MessageQueue

    // Domain logic (injected)
    reactEngine *domain.ReactEngine

    // Configuration
    config Config
}

// Config holds coordinator configuration
type Config struct {
    LLMProvider    string
    LLMModel       string
    MaxTokens      int
    MaxIterations  int
}

// NewAgentCoordinator creates a new coordinator with all dependencies injected
func NewAgentCoordinator(
    llmFactory *llm.Factory,
    toolRegistry ports.ToolRegistry,
    sessionStore ports.SessionStore,
    contextMgr ports.ContextManager,
    parser ports.FunctionCallParser,
    messageQueue ports.MessageQueue,
    reactEngine *domain.ReactEngine,
    config Config,
) *AgentCoordinator {
    return &AgentCoordinator{
        llmFactory:   llmFactory,
        toolRegistry: toolRegistry,
        sessionStore: sessionStore,
        contextMgr:   contextMgr,
        parser:       parser,
        messageQueue: messageQueue,
        reactEngine:  reactEngine,
        config:       config,
    }
}

// ExecuteTask is the main entry point for task execution
func (c *AgentCoordinator) ExecuteTask(
    ctx context.Context,
    task string,
    sessionID string,
    callback StreamCallback,
) (*domain.TaskResult, error) {
    // 1. Load or create session
    session, err := c.getSession(ctx, sessionID)
    if err != nil {
        return nil, fmt.Errorf("failed to get session: %w", err)
    }

    // 2. Check context limits & compress if needed
    if c.contextMgr.ShouldCompress(session.Messages, c.config.MaxTokens) {
        compressed, err := c.contextMgr.Compress(session.Messages, c.config.MaxTokens*80/100)
        if err != nil {
            return nil, fmt.Errorf("failed to compress context: %w", err)
        }
        session.Messages = compressed
    }

    // 3. Get appropriate LLM client
    llmClient, err := c.llmFactory.GetClient(c.config.LLMProvider, c.config.LLMModel, llm.Config{})
    if err != nil {
        return nil, fmt.Errorf("failed to get LLM client: %w", err)
    }

    // 4. Prepare task state from session
    state := &domain.TaskState{
        Messages: convertSessionMessages(session.Messages),
    }

    // 5. Create services bundle for domain layer
    services := domain.Services{
        LLM:          llmClient,
        ToolExecutor: c.toolRegistry,
        Parser:       c.parser,
        Context:      c.contextMgr,
    }

    // 6. Wrap LLM client with streaming callback if provided
    if callback != nil {
        services.LLM = c.wrapWithCallback(llmClient, callback)
    }

    // 7. Delegate to domain logic
    result, err := c.reactEngine.SolveTask(ctx, task, state, services)
    if err != nil {
        return nil, fmt.Errorf("task execution failed: %w", err)
    }

    // 8. Update session with results
    session.Messages = append(session.Messages, convertDomainMessages(result.Messages)...)
    session.UpdatedAt = time.Now()

    if err := c.sessionStore.Save(ctx, session); err != nil {
        return nil, fmt.Errorf("failed to save session: %w", err)
    }

    return result, nil
}

// ExecuteTaskAsync queues a task for async execution
func (c *AgentCoordinator) ExecuteTaskAsync(
    ctx context.Context,
    task string,
    sessionID string,
) error {
    return c.messageQueue.Enqueue(ports.UserMessage{
        Content:   task,
        SessionID: sessionID,
    })
}

// ProcessQueue processes queued messages
func (c *AgentCoordinator) ProcessQueue(ctx context.Context) error {
    for {
        msg, err := c.messageQueue.Dequeue(ctx)
        if err != nil {
            return err
        }

        _, err = c.ExecuteTask(ctx, msg.Content, msg.SessionID, nil)
        if err != nil {
            // Log error but continue processing
            log.Printf("Failed to process message: %v", err)
        }
    }
}

// ResumeSession resumes an existing session
func (c *AgentCoordinator) ResumeSession(
    ctx context.Context,
    sessionID string,
) (*ports.Session, error) {
    return c.sessionStore.Get(ctx, sessionID)
}

// ListSessions returns all available sessions
func (c *AgentCoordinator) ListSessions(ctx context.Context) ([]string, error) {
    return c.sessionStore.List(ctx)
}

// getSession retrieves existing session or creates new one
func (c *AgentCoordinator) getSession(ctx context.Context, id string) (*ports.Session, error) {
    if id == "" {
        return c.sessionStore.Create(ctx)
    }
    return c.sessionStore.Get(ctx, id)
}

// wrapWithCallback wraps LLM client to call streaming callback
func (c *AgentCoordinator) wrapWithCallback(
    client ports.LLMClient,
    callback StreamCallback,
) ports.LLMClient {
    return &streamingLLMWrapper{
        underlying: client,
        callback:   callback,
    }
}

// StreamCallback handles streaming responses
type StreamCallback func(chunk string) error

// Helper functions
func convertSessionMessages(msgs []ports.Message) []domain.Message {
    // Implementation...
}

func convertDomainMessages(msgs []domain.Message) []ports.Message {
    // Implementation...
}
```

### 2.2 Streaming Wrapper

```go
// internal/agent/app/streaming_wrapper.go
package app

type streamingLLMWrapper struct {
    underlying ports.LLMClient
    callback   StreamCallback
}

func (w *streamingLLMWrapper) Complete(
    ctx context.Context,
    req ports.CompletionRequest,
) (*ports.CompletionResponse, error) {
    if w.callback == nil {
        return w.underlying.Complete(ctx, req)
    }

    // Use streaming if callback provided
    stream, err := w.underlying.Stream(ctx, req)
    if err != nil {
        return nil, err
    }
    defer stream.Close()

    var fullContent string
    var toolCalls []ports.ToolCall
    var usage *ports.TokenUsage

    for {
        chunk, err := stream.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, err
        }

        if chunk.Delta != "" {
            fullContent += chunk.Delta
            if err := w.callback(chunk.Delta); err != nil {
                return nil, err
            }
        }

        if chunk.ToolCall != nil {
            toolCalls = append(toolCalls, *chunk.ToolCall)
        }

        if chunk.Done {
            usage = chunk.Usage
            break
        }
    }

    return &ports.CompletionResponse{
        Content:   fullContent,
        ToolCalls: toolCalls,
        Usage:     *usage,
    }, nil
}

func (w *streamingLLMWrapper) Stream(ctx context.Context, req ports.CompletionRequest) (ports.ResponseStream, error) {
    return w.underlying.Stream(ctx, req)
}

func (w *streamingLLMWrapper) Model() string {
    return w.underlying.Model()
}
```

### 2.3 Application Layer Testing

```go
// internal/agent/app/coordinator_test.go
package app_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/yourusername/alex/internal/agent/app"
    "github.com/yourusername/alex/internal/agent/domain"
    "github.com/yourusername/alex/internal/agent/ports/mocks"
    "github.com/yourusername/alex/internal/context"
    "github.com/yourusername/alex/internal/parser"
    "github.com/yourusername/alex/internal/session/memstore"
    "github.com/yourusername/alex/internal/tools"
)

func TestCoordinator_ExecuteTask_WithRealComponents(t *testing.T) {
    // Arrange: Use real components except LLM (mock that)
    mockLLM := &mocks.MockLLMClient{}
    mockLLM.On("Complete", mock.Anything, mock.Anything).Return(&ports.CompletionResponse{
        Content: "Final answer: The task is complete",
    }, nil)
    mockLLM.On("Model").Return("test-model")

    llmFactory := &testFactory{client: mockLLM}
    toolRegistry := tools.NewRegistry()          // Real registry
    sessionStore := memstore.New()               // In-memory store
    contextMgr := context.NewManager()           // Real manager
    parser := parser.New()                       // Real parser
    messageQueue := messaging.NewQueue(10)       // Real queue

    engine := domain.NewReactEngine(10)

    coordinator := app.NewAgentCoordinator(
        llmFactory,
        toolRegistry,
        sessionStore,
        contextMgr,
        parser,
        messageQueue,
        engine,
        app.Config{
            LLMProvider:   "test",
            LLMModel:      "test-model",
            MaxTokens:     100000,
            MaxIterations: 10,
        },
    )

    // Act: Execute task
    result, err := coordinator.ExecuteTask(context.Background(), "test task", "", nil)

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Contains(t, result.Answer, "complete")
}

func TestCoordinator_SessionPersistence(t *testing.T) {
    // Test that session is saved and can be resumed
    // ... implementation
}

func TestCoordinator_ContextCompression(t *testing.T) {
    // Test compression triggers when context limit approached
    // ... implementation
}

func TestCoordinator_StreamingCallback(t *testing.T) {
    // Test streaming callback is called with chunks
    // ... implementation
}
```

---

## 3. SubAgent Orchestrator

**Package**: `internal/agent/app/`
**Purpose**: Manage sub-task delegation and parallel execution

### 3.1 SubAgent Orchestrator Implementation

```go
// internal/agent/app/subagent.go
package app

import (
    "context"
    "fmt"

    "golang.org/x/sync/errgroup"
)

// SubAgentOrchestrator handles task decomposition and delegation
type SubAgentOrchestrator struct {
    coordinator *AgentCoordinator
    strategy    ExecutionStrategy
}

func NewSubAgentOrchestrator(
    coordinator *AgentCoordinator,
    strategy ExecutionStrategy,
) *SubAgentOrchestrator {
    return &SubAgentOrchestrator{
        coordinator: coordinator,
        strategy:    strategy,
    }
}

// ExecuteSubTasks delegates sub-tasks to independent agents
func (o *SubAgentOrchestrator) ExecuteSubTasks(
    ctx context.Context,
    tasks []SubTask,
    sessionID string,
) ([]domain.TaskResult, error) {
    return o.strategy.Execute(ctx, o.coordinator, tasks, sessionID)
}

// SubTask represents a delegated sub-task
type SubTask struct {
    ID          string
    Description string
    Context     string  // Additional context for the sub-task
}
```

---

## 4. Execution Strategies

**Package**: `internal/agent/app/`
**Purpose**: Different strategies for executing sub-tasks

### 4.1 Strategy Interface

```go
// internal/agent/app/strategy.go
package app

// ExecutionStrategy determines how sub-tasks are executed
type ExecutionStrategy interface {
    Execute(
        ctx context.Context,
        coordinator *AgentCoordinator,
        tasks []SubTask,
        sessionID string,
    ) ([]domain.TaskResult, error)
}
```

### 4.2 Serial Strategy

```go
// internal/agent/app/strategy_serial.go
package app

// SerialStrategy executes tasks one after another
type SerialStrategy struct{}

func NewSerialStrategy() ExecutionStrategy {
    return &SerialStrategy{}
}

func (s *SerialStrategy) Execute(
    ctx context.Context,
    coordinator *AgentCoordinator,
    tasks []SubTask,
    sessionID string,
) ([]domain.TaskResult, error) {
    results := make([]domain.TaskResult, len(tasks))

    for i, task := range tasks {
        result, err := coordinator.ExecuteTask(ctx, task.Description, sessionID, nil)
        if err != nil {
            return nil, fmt.Errorf("task %s failed: %w", task.ID, err)
        }
        results[i] = *result
    }

    return results, nil
}
```

### 4.3 Parallel Strategy

```go
// internal/agent/app/strategy_parallel.go
package app

import "golang.org/x/sync/errgroup"

// ParallelStrategy executes tasks concurrently
type ParallelStrategy struct {
    maxWorkers int
}

func NewParallelStrategy(maxWorkers int) ExecutionStrategy {
    return &ParallelStrategy{maxWorkers: maxWorkers}
}

func (s *ParallelStrategy) Execute(
    ctx context.Context,
    coordinator *AgentCoordinator,
    tasks []SubTask,
    sessionID string,
) ([]domain.TaskResult, error) {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(s.maxWorkers)

    results := make([]domain.TaskResult, len(tasks))

    for i, task := range tasks {
        i, task := i, task  // Capture loop variables

        g.Go(func() error {
            result, err := coordinator.ExecuteTask(ctx, task.Description, "", nil)  // New session per task
            if err != nil {
                return fmt.Errorf("task %s failed: %w", task.ID, err)
            }
            results[i] = *result
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        return nil, err
    }

    return results, nil
}
```

### 4.4 Auto Strategy

```go
// internal/agent/app/strategy_auto.go
package app

// AutoStrategy automatically chooses serial or parallel based on task count
type AutoStrategy struct {
    parallelThreshold int
    maxWorkers        int
}

func NewAutoStrategy(parallelThreshold, maxWorkers int) ExecutionStrategy {
    return &AutoStrategy{
        parallelThreshold: parallelThreshold,
        maxWorkers:        maxWorkers,
    }
}

func (s *AutoStrategy) Execute(
    ctx context.Context,
    coordinator *AgentCoordinator,
    tasks []SubTask,
    sessionID string,
) ([]domain.TaskResult, error) {
    if len(tasks) >= s.parallelThreshold {
        // Use parallel strategy
        parallel := NewParallelStrategy(s.maxWorkers)
        return parallel.Execute(ctx, coordinator, tasks, sessionID)
    }

    // Use serial strategy
    serial := NewSerialStrategy()
    return serial.Execute(ctx, coordinator, tasks, sessionID)
}
```

---

## 5. CLI Layer

**Package**: `cmd/alex/`
**Purpose**: User interface and command-line interaction

### 5.1 Main Entry Point

```go
// cmd/alex/main.go
package main

import (
    "context"
    "fmt"
    "os"
)

func main() {
    // Build dependency injection container
    container, err := buildContainer()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
        os.Exit(1)
    }
    defer container.Cleanup()

    // Create CLI handler
    cli := NewCLI(container)

    // Parse arguments and execute
    if err := cli.Run(context.Background(), os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### 5.2 Dependency Injection Container

```go
// cmd/alex/container.go
package main

import (
    "github.com/yourusername/alex/internal/agent/app"
    "github.com/yourusername/alex/internal/agent/domain"
    "github.com/yourusername/alex/internal/context"
    "github.com/yourusername/alex/internal/llm"
    "github.com/yourusername/alex/internal/messaging"
    "github.com/yourusername/alex/internal/parser"
    "github.com/yourusername/alex/internal/session/filestore"
    "github.com/yourusername/alex/internal/tools"
)

// Container holds all application dependencies
type Container struct {
    Coordinator *app.AgentCoordinator
    UI          *UI
}

// buildContainer constructs all dependencies with proper injection
func buildContainer() (*Container, error) {
    // Load configuration
    config := loadConfig()

    // Infrastructure Layer
    llmFactory := llm.NewFactory()
    toolRegistry := tools.NewRegistry()
    sessionStore := filestore.New("~/.alex-sessions")
    contextMgr := context.NewManager()
    parser := parser.New()
    messageQueue := messaging.NewQueue(100)

    // Domain Layer
    reactEngine := domain.NewReactEngine(config.MaxIterations)

    // Application Layer
    coordinator := app.NewAgentCoordinator(
        llmFactory,
        toolRegistry,
        sessionStore,
        contextMgr,
        parser,
        messageQueue,
        reactEngine,
        app.Config{
            LLMProvider:   config.LLMProvider,
            LLMModel:      config.LLMModel,
            MaxTokens:     config.MaxTokens,
            MaxIterations: config.MaxIterations,
        },
    )

    // UI Layer
    ui := NewUI()

    return &Container{
        Coordinator: coordinator,
        UI:          ui,
    }, nil
}

func (c *Container) Cleanup() {
    // Cleanup resources if needed
}

func loadConfig() Config {
    // Load from ~/.alex-config.json or environment
    return Config{
        LLMProvider:   "openai",
        LLMModel:      "gpt-4",
        MaxTokens:     100000,
        MaxIterations: 10,
    }
}

type Config struct {
    LLMProvider   string
    LLMModel      string
    MaxTokens     int
    MaxIterations int
}
```

### 5.3 CLI Handler

```go
// cmd/alex/cli.go
package main

import (
    "context"
    "fmt"
)

type CLI struct {
    container *Container
}

func NewCLI(container *Container) *CLI {
    return &CLI{container: container}
}

func (c *CLI) Run(ctx context.Context, args []string) error {
    if len(args) == 0 {
        return fmt.Errorf("no command provided")
    }

    switch args[0] {
    case "ask", "query":
        return c.handleAsk(ctx, args[1:])
    case "resume":
        return c.handleResume(ctx, args[1:])
    case "list":
        return c.handleList(ctx)
    default:
        // Treat unknown command as a task
        return c.handleAsk(ctx, args)
    }
}

func (c *CLI) handleAsk(ctx context.Context, args []string) error {
    task := joinArgs(args)

    c.container.UI.ShowThinking("Processing your request...")

    result, err := c.container.Coordinator.ExecuteTask(
        ctx,
        task,
        "",  // New session
        c.streamCallback(),
    )
    if err != nil {
        return err
    }

    c.container.UI.ShowResult(result)
    return nil
}

func (c *CLI) handleResume(ctx context.Context, args []string) error {
    if len(args) == 0 {
        return fmt.Errorf("session ID required")
    }

    sessionID := args[0]
    task := joinArgs(args[1:])

    result, err := c.container.Coordinator.ExecuteTask(ctx, task, sessionID, c.streamCallback())
    if err != nil {
        return err
    }

    c.container.UI.ShowResult(result)
    return nil
}

func (c *CLI) handleList(ctx context.Context) error {
    sessions, err := c.container.Coordinator.ListSessions(ctx)
    if err != nil {
        return err
    }

    c.container.UI.ShowSessions(sessions)
    return nil
}

func (c *CLI) streamCallback() app.StreamCallback {
    return func(chunk string) error {
        c.container.UI.ShowChunk(chunk)
        return nil
    }
}

func joinArgs(args []string) string {
    result := ""
    for _, arg := range args {
        if result != "" {
            result += " "
        }
        result += arg
    }
    return result
}
```

### 5.4 UI Layer

```go
// cmd/alex/ui.go
package main

import (
    "fmt"

    "github.com/fatih/color"
)

type UI struct {
    thinking *color.Color
    result   *color.Color
    error    *color.Color
}

func NewUI() *UI {
    return &UI{
        thinking: color.New(color.FgYellow),
        result:   color.New(color.FgGreen),
        error:    color.New(color.FgRed),
    }
}

func (ui *UI) ShowThinking(msg string) {
    ui.thinking.Println(msg)
}

func (ui *UI) ShowChunk(chunk string) {
    fmt.Print(chunk)
}

func (ui *UI) ShowResult(result *domain.TaskResult) {
    fmt.Println()
    ui.result.Printf("\n✓ Task completed in %d iterations\n", result.Iterations)
    fmt.Println(result.Answer)
}

func (ui *UI) ShowSessions(sessions []string) {
    fmt.Println("Available sessions:")
    for _, s := range sessions {
        fmt.Printf("  - %s\n", s)
    }
}

func (ui *UI) ShowError(err error) {
    ui.error.Printf("✗ Error: %v\n", err)
}
```

---

## Summary

The business logic layer provides:

- ✅ **Pure domain logic** in `internal/agent/domain/` with zero infrastructure dependencies
- ✅ **Thin orchestration** in `internal/agent/app/` coordinating domain and infrastructure
- ✅ **Clear separation** between business rules and technical concerns
- ✅ **Easy testing** with comprehensive mock support
- ✅ **Flexible execution** with pluggable strategies (serial, parallel, auto)
- ✅ **Clean CLI** with dependency injection from `cmd/alex/`

All components are independently testable, maintainable, and extensible.

---

**Next Steps**: Review this design and proceed with [Phase 1 implementation](./REFACTORING_PROPOSAL.md#phase-1-foundation-week-1-2)