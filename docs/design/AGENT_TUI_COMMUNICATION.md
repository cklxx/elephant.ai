# Agent-TUI Communication Architecture
> Last updated: 2025-11-18


**Date:** 2025-10-01
**Status:** Design Document
**Version:** 1.0

---

## Overview

This document defines the communication architecture between ALEX's ReAct agent and the TUI (Terminal User Interface) for streaming, real-time display of agent execution.

## Design Goals

1. **Non-Intrusive**: Minimal changes to existing ReAct engine
2. **Real-Time**: Stream all agent actions as they happen
3. **Decoupled**: TUI can be enabled/disabled without affecting agent logic
4. **Rich Information**: Include thought process, tool calls, results, and errors
5. **Performance**: Low overhead, non-blocking

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      User Request                           │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                    CLI Entry Point                          │
│  ┌──────────────┐         ┌──────────────┐                │
│  │  REPL Mode   │         │   TUI Mode   │                │
│  └──────────────┘         └──────┬───────┘                │
└─────────────────────────────────┼─────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────┐
│              TUI Program (Bubble Tea)                       │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Root Model (Message Router)                         │  │
│  │    ├─ StatusBar  ├─ Tree  ├─ Output  ├─ Help       │  │
│  └──────────────────┬───────────────────────────────────┘  │
└────────────────────┼────────────────────────────────────────┘
                     │
            ┌────────┴────────┐
            │                 │
      Event Channel     Command Channel
            │                 │
            ▼                 ▼
┌─────────────────────────────────────────────────────────────┐
│            Coordinator (Application Layer)                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  EventBridge: Converts domain events to TUI msgs    │  │
│  └──────────────────┬───────────────────────────────────┘  │
└────────────────────┼────────────────────────────────────────┘
                     │
                     ▼ (Callbacks)
┌─────────────────────────────────────────────────────────────┐
│              ReactEngine (Domain Layer)                     │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Think → Act → Observe Loop                          │  │
│  │    ├─ Emit: IterationStart                           │  │
│  │    ├─ Emit: ThinkComplete                            │  │
│  │    ├─ Emit: ToolCallStart                            │  │
│  │    ├─ Emit: ToolCallComplete                         │  │
│  │    └─ Emit: IterationComplete                        │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## Event System Design

### 1. Domain Events (internal/agent/domain/events.go)

```go
// AgentEvent is the base interface for all agent events
type AgentEvent interface {
    EventType() string
    Timestamp() time.Time
}

// IterationStartEvent - emitted at start of each ReAct iteration
type IterationStartEvent struct {
    timestamp  time.Time
    Iteration  int
    TotalIters int
}

// ThinkingEvent - emitted when LLM is generating response
type ThinkingEvent struct {
    timestamp   time.Time
    Iteration   int
    MessageCount int
}

// ThinkCompleteEvent - emitted when LLM response received
type ThinkCompleteEvent struct {
    timestamp    time.Time
    Iteration    int
    Content      string
    ToolCallCount int
}

// ToolCallStartEvent - emitted when tool execution begins
type ToolCallStartEvent struct {
    timestamp  time.Time
    Iteration  int
    CallID     string
    ToolName   string
    Arguments  map[string]interface{}
}

// ToolCallStreamEvent - emitted during tool execution (for streaming tools)
type ToolCallStreamEvent struct {
    timestamp  time.Time
    CallID     string
    Chunk      string
    IsComplete bool
}

// ToolCallCompleteEvent - emitted when tool execution finishes
type ToolCallCompleteEvent struct {
    timestamp  time.Time
    CallID     string
    ToolName   string
    Result     string
    Error      error
    Duration   time.Duration
}

// IterationCompleteEvent - emitted at end of iteration
type IterationCompleteEvent struct {
    timestamp   time.Time
    Iteration   int
    TokensUsed  int
    ToolsRun    int
}

// TaskCompleteEvent - emitted when entire task finishes
type TaskCompleteEvent struct {
    timestamp      time.Time
    FinalAnswer    string
    TotalIterations int
    TotalTokens    int
    StopReason     string
    Duration       time.Duration
}

// ErrorEvent - emitted on errors
type ErrorEvent struct {
    timestamp  time.Time
    Iteration  int
    Phase      string  // "think", "execute", "observe"
    Error      error
    Recoverable bool
}
```

### 2. Event Listener Interface

```go
// EventListener receives agent events
type EventListener interface {
    OnEvent(event AgentEvent)
}

// EventListenerFunc is a function adapter for EventListener
type EventListenerFunc func(AgentEvent)

func (f EventListenerFunc) OnEvent(event AgentEvent) {
    f(event)
}
```

### 3. Enhanced ReAct Engine

**Minimal changes to existing code:**

```go
// Add to ReactEngine struct
type ReactEngine struct {
    maxIterations int
    stopReasons   []string
    logger        *utils.Logger
    formatter     *ToolFormatter

    // NEW: Optional event listener
    eventListener EventListener  // nil-safe, only emit if set
}

// NEW: SetEventListener configures event emission
func (e *ReactEngine) SetEventListener(listener EventListener) {
    e.eventListener = listener
}

// Helper to emit events (nil-safe)
func (e *ReactEngine) emit(event AgentEvent) {
    if e.eventListener != nil {
        e.eventListener.OnEvent(event)
    }
}
```

### 4. Event Emission Points in SolveTask

```go
func (e *ReactEngine) SolveTask(
    ctx context.Context,
    task string,
    state *TaskState,
    services Services,
) (*TaskResult, error) {
    startTime := time.Now()

    // ... initialization ...

    // ReAct loop
    for state.Iterations < e.maxIterations {
        state.Iterations++

        // EMIT: Iteration started
        e.emit(&IterationStartEvent{
            timestamp:  time.Now(),
            Iteration:  state.Iterations,
            TotalIters: e.maxIterations,
        })

        // 1. THINK
        e.emit(&ThinkingEvent{
            timestamp:    time.Now(),
            Iteration:    state.Iterations,
            MessageCount: len(state.Messages),
        })

        thought, err := e.think(ctx, state, services)
        if err != nil {
            e.emit(&ErrorEvent{
                timestamp:   time.Now(),
                Iteration:   state.Iterations,
                Phase:       "think",
                Error:       err,
                Recoverable: false,
            })
            return nil, err
        }

        // EMIT: Think complete
        e.emit(&ThinkCompleteEvent{
            timestamp:     time.Now(),
            Iteration:     state.Iterations,
            Content:       thought.Content,
            ToolCallCount: len(thought.ToolCalls),
        })

        // 2. ACT
        toolCalls := e.parseToolCalls(thought, services.Parser)

        for _, call := range toolCalls {
            // EMIT: Tool call starting
            e.emit(&ToolCallStartEvent{
                timestamp:  time.Now(),
                Iteration:  state.Iterations,
                CallID:     call.ID,
                ToolName:   call.Name,
                Arguments:  call.Arguments,
            })
        }

        // Execute tools (parallel)
        results := e.executeToolsWithEvents(ctx, toolCalls, services.ToolExecutor)

        // 3. OBSERVE
        // ...

        // EMIT: Iteration complete
        e.emit(&IterationCompleteEvent{
            timestamp:  time.Now(),
            Iteration:  state.Iterations,
            TokensUsed: state.TokenCount,
            ToolsRun:   len(results),
        })
    }

    // EMIT: Task complete
    finalResult := e.finalize(state, "completed")
    e.emit(&TaskCompleteEvent{
        timestamp:       time.Now(),
        FinalAnswer:     finalResult.Answer,
        TotalIterations: finalResult.Iterations,
        TotalTokens:     finalResult.TokensUsed,
        StopReason:      finalResult.StopReason,
        Duration:        time.Since(startTime),
    })

    return finalResult, nil
}
```

### 5. Tool Execution with Events

```go
func (e *ReactEngine) executeToolsWithEvents(
    ctx context.Context,
    calls []ToolCall,
    registry ports.ToolRegistry,
) []ToolResult {
    results := make([]ToolResult, len(calls))
    var wg sync.WaitGroup

    for i, call := range calls {
        wg.Add(1)
        go func(idx int, tc ToolCall) {
            defer wg.Done()

            startTime := time.Now()

            tool, err := registry.Get(tc.Name)
            if err != nil {
                // EMIT: Tool error
                e.emit(&ToolCallCompleteEvent{
                    timestamp: time.Now(),
                    CallID:    tc.ID,
                    ToolName:  tc.Name,
                    Error:     err,
                    Duration:  time.Since(startTime),
                })
                results[idx] = ToolResult{
                    CallID:  tc.ID,
                    Error:   err,
                }
                return
            }

            result, err := tool.Execute(ctx, ports.ToolCall{
                ID:        tc.ID,
                Name:      tc.Name,
                Arguments: tc.Arguments,
            })

            // EMIT: Tool complete
            e.emit(&ToolCallCompleteEvent{
                timestamp: time.Now(),
                CallID:    tc.ID,
                ToolName:  tc.Name,
                Result:    result.Content,
                Error:     err,
                Duration:  time.Since(startTime),
            })

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
```

---

## TUI Integration (Application Layer)

### 1. Event Bridge (internal/agent/app/event_bridge.go)

```go
package app

import (
    "alex/internal/agent/domain"
    tea "github.com/charmbracelet/bubbletea"
)

// EventBridge converts domain events to Bubble Tea messages
type EventBridge struct {
    program *tea.Program
}

func NewEventBridge(program *tea.Program) *EventBridge {
    return &EventBridge{program: program}
}

// OnEvent implements domain.EventListener
func (b *EventBridge) OnEvent(event domain.AgentEvent) {
    // Convert to Bubble Tea message and send
    msg := b.convertToTUIMessage(event)
    if msg != nil {
        b.program.Send(msg)
    }
}

func (b *EventBridge) convertToTUIMessage(event domain.AgentEvent) tea.Msg {
    switch e := event.(type) {
    case *domain.IterationStartEvent:
        return IterationStartMsg{
            Iteration:  e.Iteration,
            TotalIters: e.TotalIters,
        }

    case *domain.ThinkingEvent:
        return ThinkingMsg{
            Iteration: e.Iteration,
        }

    case *domain.ThinkCompleteEvent:
        return ThinkCompleteMsg{
            Iteration:     e.Iteration,
            Content:       e.Content,
            ToolCallCount: e.ToolCallCount,
        }

    case *domain.ToolCallStartEvent:
        return ToolCallStartMsg{
            CallID:    e.CallID,
            ToolName:  e.ToolName,
            Arguments: e.Arguments,
        }

    case *domain.ToolCallCompleteEvent:
        return ToolCallCompleteMsg{
            CallID:   e.CallID,
            ToolName: e.ToolName,
            Result:   e.Result,
            Error:    e.Error,
            Duration: e.Duration,
        }

    case *domain.TaskCompleteEvent:
        return TaskCompleteMsg{
            FinalAnswer:     e.FinalAnswer,
            TotalIterations: e.TotalIterations,
            TotalTokens:     e.TotalTokens,
            Duration:        e.Duration,
        }

    case *domain.ErrorEvent:
        return ErrorMsg{
            Phase: e.Phase,
            Error: e.Error,
        }

    default:
        return nil
    }
}

// TUI Messages (what Bubble Tea sees)
type IterationStartMsg struct {
    Iteration  int
    TotalIters int
}

type ThinkingMsg struct {
    Iteration int
}

type ThinkCompleteMsg struct {
    Iteration     int
    Content       string
    ToolCallCount int
}

type ToolCallStartMsg struct {
    CallID    string
    ToolName  string
    Arguments map[string]interface{}
}

type ToolCallCompleteMsg struct {
    CallID   string
    ToolName string
    Result   string
    Error    error
    Duration time.Duration
}

type TaskCompleteMsg struct {
    FinalAnswer     string
    TotalIterations int
    TotalTokens     int
    Duration        time.Duration
}

type ErrorMsg struct {
    Phase string
    Error error
}
```

### 2. Enhanced Coordinator

```go
// Add to AgentCoordinator
func (c *AgentCoordinator) ExecuteTaskWithTUI(
    ctx context.Context,
    task string,
    sessionID string,
    tuiProgram *tea.Program,
) (*domain.TaskResult, error) {
    // ... existing setup ...

    // Create event bridge
    bridge := NewEventBridge(tuiProgram)

    // Set listener on ReactEngine
    c.reactEngine.SetEventListener(bridge)

    // Execute task (events will flow to TUI)
    result, err := c.reactEngine.SolveTask(ctx, task, state, services)

    // Clear listener
    c.reactEngine.SetEventListener(nil)

    return result, err
}
```

---

## TUI Model Design (cmd/alex/tui/model.go)

### 1. Root Model

```go
package tui

import (
    "time"
    tea "github.com/charmbracelet/bubbletea"
    "alex/internal/agent/app"
)

type RootModel struct {
    // State
    width   int
    height  int
    focused FocusMode

    // Sub-models
    statusBar StatusBarModel
    tree      TreeModel
    output    OutputModel
    help      HelpModel

    // Task state
    currentIteration int
    totalIterations  int
    taskStartTime    time.Time
    taskComplete     bool

    // Active tool calls
    activeTools map[string]ToolCallInfo
}

type ToolCallInfo struct {
    ToolName  string
    StartTime time.Time
    TreeNodeID string
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    // Window resize
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.resizeChildren()

    // Iteration started
    case app.IterationStartMsg:
        m.currentIteration = msg.Iteration
        m.totalIterations = msg.TotalIters

        // Update status bar
        m.statusBar.SetPhase(fmt.Sprintf("Iteration %d/%d", msg.Iteration, msg.TotalIters))

        // Add iteration node to tree
        nodeID := m.tree.AddIterationNode(msg.Iteration)

        return m, nil

    // LLM thinking
    case app.ThinkingMsg:
        // Update status
        m.statusBar.SetStatus("Thinking...")

        // Add thinking indicator to output
        m.output.AppendThinking(msg.Iteration)

        return m, nil

    // LLM response received
    case app.ThinkCompleteMsg:
        // Add thought to output
        m.output.AppendThought(msg.Content)

        // Update status
        if msg.ToolCallCount > 0 {
            m.statusBar.SetStatus(fmt.Sprintf("Executing %d tools...", msg.ToolCallCount))
        } else {
            m.statusBar.SetStatus("Processing...")
        }

        return m, nil

    // Tool execution started
    case app.ToolCallStartMsg:
        // Track active tool
        m.activeTools[msg.CallID] = ToolCallInfo{
            ToolName:  msg.ToolName,
            StartTime: time.Now(),
        }

        // Add tool node to tree
        nodeID := m.tree.AddToolNode(msg.ToolName, msg.Arguments)
        m.activeTools[msg.CallID].TreeNodeID = nodeID

        // Add to output
        m.output.AppendToolCall(msg.ToolName, msg.Arguments)

        return m, nil

    // Tool execution completed
    case app.ToolCallCompleteMsg:
        // Get tool info
        info, exists := m.activeTools[msg.CallID]
        if !exists {
            return m, nil
        }

        // Update tree node
        if msg.Error != nil {
            m.tree.MarkNodeError(info.TreeNodeID, msg.Error)
        } else {
            m.tree.MarkNodeComplete(info.TreeNodeID)
        }

        // Add result to output
        m.output.AppendToolResult(msg.ToolName, msg.Result, msg.Error, msg.Duration)

        // Remove from active
        delete(m.activeTools, msg.CallID)

        return m, nil

    // Task completed
    case app.TaskCompleteMsg:
        m.taskComplete = true
        m.statusBar.SetStatus("Complete!")
        m.output.AppendFinalAnswer(msg.FinalAnswer)
        m.tree.MarkAllComplete()

        return m, nil

    // Error
    case app.ErrorMsg:
        m.output.AppendError(msg.Phase, msg.Error)
        return m, nil

    // Keyboard input
    case tea.KeyMsg:
        return m.handleKeyPress(msg)

    // Timer tick (for spinner)
    case tea.TickMsg:
        newStatus, cmd := m.statusBar.Update(msg)
        m.statusBar = newStatus.(StatusBarModel)
        return m, cmd
    }

    return m, nil
}
```

---

## Output Display Strategies

### 1. Streaming Tool Actions

```go
// In OutputModel
func (m *OutputModel) AppendToolCall(name string, args map[string]interface{}) {
    // Format tool call nicely
    formatted := m.formatToolCall(name, args)

    // Add to content with styling
    style := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
    line := style.Render(formatted)

    m.content.WriteString(line + "\n")
    m.viewport.SetContent(m.content.String())

    if m.autoScroll {
        m.viewport.GotoBottom()
    }
}

func (m *OutputModel) formatToolCall(name string, args map[string]interface{}) string {
    // Compact format
    argsStr := m.formatArgs(args)
    return fmt.Sprintf("🔧 %s(%s)", name, argsStr)
}

func (m *OutputModel) formatArgs(args map[string]interface{}) string {
    // Show only important args, truncate long values
    var parts []string
    for k, v := range args {
        valStr := fmt.Sprintf("%v", v)
        if len(valStr) > 50 {
            valStr = valStr[:47] + "..."
        }
        parts = append(parts, fmt.Sprintf("%s=%s", k, valStr))
    }
    return strings.Join(parts, ", ")
}
```

### 2. Tool Result Preview

```go
func (m *OutputModel) AppendToolResult(name string, result string, err error, duration time.Duration) {
    if err != nil {
        // Error format
        style := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
        line := style.Render(fmt.Sprintf("  ✗ %s failed: %v (%s)", name, err, duration))
        m.content.WriteString(line + "\n")
    } else {
        // Success with preview
        preview := m.createPreview(name, result)
        style := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
        line := style.Render(fmt.Sprintf("  ✓ %s: %s (%s)", name, preview, duration))
        m.content.WriteString(line + "\n")
    }

    m.viewport.SetContent(m.content.String())
    if m.autoScroll {
        m.viewport.GotoBottom()
    }
}

func (m *OutputModel) createPreview(toolName, result string) string {
    switch toolName {
    case "file_read":
        lines := strings.Split(result, "\n")
        return fmt.Sprintf("%d lines read", len(lines))

    case "grep", "ripgrep":
        matches := strings.Count(result, "\n")
        return fmt.Sprintf("%d matches found", matches)

    case "bash":
        if len(result) == 0 {
            return "success (no output)"
        }
        firstLine := strings.Split(result, "\n")[0]
        if len(firstLine) > 50 {
            firstLine = firstLine[:47] + "..."
        }
        return firstLine

    default:
        if len(result) > 80 {
            return result[:77] + "..."
        }
        return result
    }
}
```

---

## Performance Considerations

### 1. Event Throttling

```go
// For high-frequency events (streaming output), throttle updates
type ThrottledListener struct {
    inner      domain.EventListener
    lastUpdate time.Time
    minInterval time.Duration
}

func (t *ThrottledListener) OnEvent(event domain.AgentEvent) {
    now := time.Now()
    if now.Sub(t.lastUpdate) < t.minInterval {
        return // Skip this event
    }
    t.lastUpdate = now
    t.inner.OnEvent(event)
}
```

### 2. Buffered Channel

```go
// Use buffered channel to prevent blocking agent
type BufferedBridge struct {
    program   *tea.Program
    eventChan chan domain.AgentEvent
}

func NewBufferedBridge(program *tea.Program) *BufferedBridge {
    b := &BufferedBridge{
        program:   program,
        eventChan: make(chan domain.AgentEvent, 100), // Buffer 100 events
    }
    go b.processEvents()
    return b
}

func (b *BufferedBridge) OnEvent(event domain.AgentEvent) {
    select {
    case b.eventChan <- event:
        // Event queued
    default:
        // Buffer full, drop event (or log warning)
    }
}

func (b *BufferedBridge) processEvents() {
    for event := range b.eventChan {
        msg := convertToTUIMessage(event)
        if msg != nil {
            b.program.Send(msg)
        }
    }
}
```

---

## Testing Strategy

### 1. Unit Tests for Events

```go
func TestReactEngineEmitsEvents(t *testing.T) {
    // Create mock listener
    var receivedEvents []domain.AgentEvent
    listener := domain.EventListenerFunc(func(e domain.AgentEvent) {
        receivedEvents = append(receivedEvents, e)
    })

    // Create engine with listener
    engine := domain.NewReactEngine(3)
    engine.SetEventListener(listener)

    // Execute task
    // ... mock LLM, tools ...

    // Verify events
    assert.True(t, len(receivedEvents) > 0)

    // Check for expected event types
    var hasIterationStart, hasToolCall bool
    for _, e := range receivedEvents {
        if _, ok := e.(*domain.IterationStartEvent); ok {
            hasIterationStart = true
        }
        if _, ok := e.(*domain.ToolCallStartEvent); ok {
            hasToolCall = true
        }
    }

    assert.True(t, hasIterationStart)
    assert.True(t, hasToolCall)
}
```

### 2. TUI Integration Tests

```go
func TestTUIReceivesEvents(t *testing.T) {
    // Create TUI model
    model := NewRootModel()

    // Simulate events
    model, _ = model.Update(app.IterationStartMsg{Iteration: 1, TotalIters: 5})
    model, _ = model.Update(app.ToolCallStartMsg{
        ToolName: "file_read",
        Arguments: map[string]interface{}{"path": "test.go"},
    })

    // Verify state updated
    rootModel := model.(RootModel)
    assert.Equal(t, 1, rootModel.currentIteration)
    assert.NotEmpty(t, rootModel.activeTools)
}
```

---

## Implementation Checklist

- [ ] Create `internal/agent/domain/events.go` with event types
- [ ] Add `EventListener` interface to domain layer
- [ ] Enhance `ReactEngine` to emit events at key points
- [ ] Create `internal/agent/app/event_bridge.go`
- [ ] Add TUI message types in app layer
- [ ] Implement `ExecuteTaskWithTUI` in coordinator
- [ ] Create basic TUI models (StatusBar, Tree, Output)
- [ ] Wire up event flow in CLI entry point
- [ ] Add throttling/buffering for performance
- [ ] Write unit tests for event emission
- [ ] Write integration tests for TUI updates

---

## Next Steps

1. Implement event system in domain layer ✅
2. Create event bridge in app layer ✅
3. Build basic TUI components ✅
4. Integrate and test end-to-end ✅
5. Add advanced features (syntax highlighting, search) ⏭️

---

## Summary

This architecture provides:

✅ **Clean separation**: Domain emits events, app layer bridges, TUI displays
✅ **Non-intrusive**: Existing code needs minimal changes
✅ **Flexible**: Easy to add new event types
✅ **Performant**: Buffered, non-blocking communication
✅ **Testable**: Each layer can be tested independently
✅ **Streaming**: Real-time display of all agent actions

The design follows ALEX's hexagonal architecture principles and integrates seamlessly with the existing ReAct engine.
