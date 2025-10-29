package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
)

// ReactEngine orchestrates the Think-Act-Observe cycle
type ReactEngine struct {
	maxIterations int
	stopReasons   []string
	logger        ports.Logger
	clock         ports.Clock
	eventListener EventListener // Optional event listener for TUI
	completion    completionConfig
}

type completionConfig struct {
	temperature   float64
	maxTokens     int
	topP          float64
	stopSequences []string
}

// CompletionDefaults defines optional overrides for LLM completion behaviour.
type CompletionDefaults struct {
	Temperature   *float64
	MaxTokens     *int
	TopP          *float64
	StopSequences []string
}

// ReactEngineConfig captures the dependencies required to construct a ReactEngine.
type ReactEngineConfig struct {
	MaxIterations      int
	StopReasons        []string
	Logger             ports.Logger
	Clock              ports.Clock
	CompletionDefaults CompletionDefaults
}

// NewReactEngine creates a new ReAct engine with injected infrastructure dependencies.
func NewReactEngine(cfg ReactEngineConfig) *ReactEngine {
	logger := cfg.Logger
	if logger == nil {
		logger = ports.NoopLogger{}
	}

	clock := cfg.Clock
	if clock == nil {
		clock = ports.SystemClock{}
	}

	stopReasons := cfg.StopReasons
	if len(stopReasons) == 0 {
		stopReasons = []string{"final_answer", "done", "complete"}
	}

	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 1
	}

	completion := buildCompletionDefaults(cfg.CompletionDefaults)

	return &ReactEngine{
		maxIterations: maxIterations,
		stopReasons:   stopReasons,
		logger:        logger,
		clock:         clock,
		eventListener: nil,
		completion:    completion,
	}
}

func buildCompletionDefaults(cfg CompletionDefaults) completionConfig {
	temperature := 0.7
	if cfg.Temperature != nil {
		temperature = *cfg.Temperature
	}

	maxTokens := 12000
	if cfg.MaxTokens != nil && *cfg.MaxTokens > 0 {
		maxTokens = *cfg.MaxTokens
	}

	topP := 1.0
	if cfg.TopP != nil {
		topP = *cfg.TopP
	}

	stopSequences := make([]string, len(cfg.StopSequences))
	copy(stopSequences, cfg.StopSequences)

	return completionConfig{
		temperature:   temperature,
		maxTokens:     maxTokens,
		topP:          topP,
		stopSequences: stopSequences,
	}
}

// SetEventListener configures event emission for TUI/streaming
func (e *ReactEngine) SetEventListener(listener EventListener) {
	e.eventListener = listener
}

// GetEventListener returns the current event listener (for saving/restoring)
func (e *ReactEngine) GetEventListener() EventListener {
	return e.eventListener
}

// getAgentLevel reads the current agent level from context
func (e *ReactEngine) getAgentLevel(ctx context.Context) ports.AgentLevel {
	if ctx == nil {
		return ports.LevelCore
	}
	outCtx := ports.GetOutputContext(ctx)
	if outCtx == nil {
		return ports.LevelCore
	}
	return outCtx.Level
}

// emitEvent sends event to listener if one is set
func (e *ReactEngine) emitEvent(event AgentEvent) {
	if e.eventListener != nil {
		e.logger.Debug("[emitEvent] Emitting event type=%s, sessionID=%s to listener", event.EventType(), event.GetSessionID())
		e.eventListener.OnEvent(event)
		e.logger.Debug("[emitEvent] Event emitted successfully")
	} else {
		e.logger.Debug("[emitEvent] No listener set, skipping event type=%s", event.EventType())
	}
}

func (e *ReactEngine) newBaseEvent(ctx context.Context, sessionID string) BaseEvent {
	return newBaseEventWithSession(e.getAgentLevel(ctx), sessionID, e.clock.Now())
}

// SolveTask is the main ReAct loop - pure business logic
func (e *ReactEngine) SolveTask(
	ctx context.Context,
	task string,
	state *TaskState,
	services Services,
) (*TaskResult, error) {
	e.logger.Info("Starting ReAct loop for task: %s", task)
	startTime := e.clock.Now()

	// Initialize state if empty
	if len(state.Messages) == 0 {
		// Add system prompt first if available
		if state.SystemPrompt != "" {
			state.Messages = []Message{
				{Role: "system", Content: state.SystemPrompt},
			}
			e.logger.Debug("Initialized state with system prompt")
		}
	}

	// ALWAYS append the new user task to messages (even if history exists)
	state.Messages = append(state.Messages, Message{
		Role:    "user",
		Content: task,
	})
	e.logger.Debug("Added user task to messages. Total messages: %d", len(state.Messages))

	// ReAct loop: Think → Act → Observe
	for state.Iterations < e.maxIterations {
		// Check if context is cancelled before starting iteration
		if ctx.Err() != nil {
			e.logger.Info("Context cancelled, stopping execution: %v", ctx.Err())
			finalResult := e.finalize(state, "cancelled")

			// EMIT: Task complete with cancellation
			e.emitEvent(&TaskCompleteEvent{
				BaseEvent:       e.newBaseEvent(ctx, state.SessionID),
				FinalAnswer:     finalResult.Answer,
				TotalIterations: finalResult.Iterations,
				TotalTokens:     finalResult.TokensUsed,
				StopReason:      "cancelled",
				Duration:        e.clock.Now().Sub(startTime),
			})

			return nil, ctx.Err()
		}

		state.Iterations++
		e.logger.Info("=== Iteration %d/%d ===", state.Iterations, e.maxIterations)

		// EMIT: Iteration started
		e.emitEvent(&IterationStartEvent{
			BaseEvent:  e.newBaseEvent(ctx, state.SessionID),
			Iteration:  state.Iterations,
			TotalIters: e.maxIterations,
		})

		// 1. THINK: Get LLM reasoning
		e.logger.Debug("THINK phase: Calling LLM with %d messages", len(state.Messages))

		// EMIT: Thinking
		e.emitEvent(&ThinkingEvent{
			BaseEvent:    e.newBaseEvent(ctx, state.SessionID),
			Iteration:    state.Iterations,
			MessageCount: len(state.Messages),
		})

		thought, err := e.think(ctx, state, services)
		if err != nil {
			e.logger.Error("Think step failed: %v", err)
			// EMIT: Error
			e.emitEvent(&ErrorEvent{
				BaseEvent:   e.newBaseEvent(ctx, state.SessionID),
				Iteration:   state.Iterations,
				Phase:       "think",
				Error:       err,
				Recoverable: false,
			})
			return nil, fmt.Errorf("think step failed: %w", err)
		}

		// Add thought to state
		state.Messages = append(state.Messages, thought)
		e.logger.Debug("LLM response: content_length=%d, tool_calls=%d",
			len(thought.Content), len(thought.ToolCalls))

		// EMIT: Think complete
		e.emitEvent(&ThinkCompleteEvent{
			BaseEvent:     e.newBaseEvent(ctx, state.SessionID),
			Iteration:     state.Iterations,
			Content:       thought.Content,
			ToolCallCount: len(thought.ToolCalls),
		})

		// 2. ACT: Parse and execute tool calls
		toolCalls := e.parseToolCalls(thought, services.Parser)
		e.logger.Info("Parsed %d tool calls", len(toolCalls))

		if len(toolCalls) == 0 {
			// No tool calls - check if this is a final answer
			if len(strings.TrimSpace(thought.Content)) > 0 {
				e.logger.Info("No tool calls and has content - treating as final answer")
				return e.finalize(state, "final_answer"), nil
			}
			// Empty response - continue loop
			e.logger.Warn("No tool calls and empty content - continuing loop")
			continue
		}

		// Filter valid tool calls (no stdout printing)
		var validCalls []ToolCall
		for _, tc := range toolCalls {
			// Skip invalid tool calls with leaked markers
			if strings.Contains(tc.Name, "<|") || strings.Contains(tc.Name, "functions.") || strings.Contains(tc.Name, "user<") {
				e.logger.Warn("Filtering out invalid tool call with leaked markers: %s", tc.Name)
				continue
			}
			validCalls = append(validCalls, tc)
			e.logger.Debug("Tool call: %s (id=%s)", tc.Name, tc.ID)
		}

		// If no valid calls, continue
		if len(validCalls) == 0 {
			e.logger.Warn("All tool calls were invalid, continuing loop")
			continue
		}

		// Filter out any leaked tool call markers from thought content
		if thought.Content != "" {
			thought.Content = e.cleanToolCallMarkers(thought.Content)
		}

		// Execute tools
		e.logger.Debug("EXECUTE phase: Running %d tools in parallel", len(validCalls))

		// EMIT: Tool calls starting
		for _, call := range validCalls {
			e.emitEvent(&ToolCallStartEvent{
				BaseEvent: e.newBaseEvent(ctx, state.SessionID),
				Iteration: state.Iterations,
				CallID:    call.ID,
				ToolName:  call.Name,
				Arguments: call.Arguments,
			})
		}

		results := e.executeToolsWithEvents(ctx, state.SessionID, state.Iterations, validCalls, services.ToolExecutor)
		state.ToolResults = append(state.ToolResults, results...)

		// Log results (no stdout printing - let TUI handle display)
		for i, r := range results {
			if r.Error != nil {
				e.logger.Warn("Tool %d failed: %v", i, r.Error)
			} else {
				e.logger.Debug("Tool %d succeeded: result_length=%d", i, len(r.Content))
			}
		}

		// 3. OBSERVE: Add results to conversation
		observation := e.buildObservation(results)
		state.Messages = append(state.Messages, observation)
		e.logger.Debug("OBSERVE phase: Added observation to state")

		// 4. Check context limits
		tokenCount := services.Context.EstimateTokens(state.Messages)
		state.TokenCount = tokenCount
		e.logger.Debug("Current token count: %d", tokenCount)

		// EMIT: Iteration complete
		e.emitEvent(&IterationCompleteEvent{
			BaseEvent:  e.newBaseEvent(ctx, state.SessionID),
			Iteration:  state.Iterations,
			TokensUsed: state.TokenCount,
			ToolsRun:   len(results),
		})

		// LLM decides when to stop - no hardcoded stop conditions
		e.logger.Debug("Iteration %d complete, continuing to next iteration", state.Iterations)
	}

	// Max iterations reached - try to get final answer
	e.logger.Warn("Max iterations (%d) reached, requesting final answer", e.maxIterations)
	finalResult := e.finalize(state, "max_iterations")

	// If no answer, try one more time to ask for final answer
	if finalResult.Answer == "" || len(strings.TrimSpace(finalResult.Answer)) == 0 {
		e.logger.Info("No final answer found, requesting explicit answer")
		state.Messages = append(state.Messages, Message{
			Role:    "user",
			Content: "Please provide your final answer to the user's question now.",
		})

		// One final LLM call for answer
		finalThought, err := e.think(ctx, state, services)
		if err == nil && finalThought.Content != "" {
			finalResult.Answer = finalThought.Content
			e.logger.Info("Got final answer from retry: %d chars", len(finalResult.Answer))
		}
	}

	// EMIT: Task complete
	e.emitEvent(&TaskCompleteEvent{
		BaseEvent:       e.newBaseEvent(ctx, state.SessionID),
		FinalAnswer:     finalResult.Answer,
		TotalIterations: finalResult.Iterations,
		TotalTokens:     finalResult.TokensUsed,
		StopReason:      finalResult.StopReason,
		Duration:        e.clock.Now().Sub(startTime),
	})

	return finalResult, nil
}

// think sends current state to LLM for reasoning
func (e *ReactEngine) think(
	ctx context.Context,
	state *TaskState,
	services Services,
) (Message, error) {
	// Convert state to LLM request
	tools := services.ToolExecutor.List()
	e.logger.Debug("Preparing LLM request: messages=%d, tools=%d", len(state.Messages), len(tools))

	req := ports.CompletionRequest{
		Messages:    state.Messages,
		Tools:       tools,
		Temperature: e.completion.temperature,
		MaxTokens:   e.completion.maxTokens,
		TopP:        e.completion.topP,
	}

	if len(e.completion.stopSequences) > 0 {
		req.StopSequences = append([]string(nil), e.completion.stopSequences...)
	}

	// Call LLM
	e.logger.Debug("Calling LLM...")
	resp, err := services.LLM.Complete(ctx, req)
	if err != nil {
		e.logger.Error("LLM call failed: %v", err)
		return Message{}, fmt.Errorf("LLM call failed: %w", err)
	}

	e.logger.Debug("LLM response received: content=%d bytes, tool_calls=%d",
		len(resp.Content), len(resp.ToolCalls))

	// Convert response to domain message
	return Message{
		Role:      "assistant",
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
	}, nil
}

// executeToolsWithEvents runs all tool calls in parallel and emits completion events
func (e *ReactEngine) executeToolsWithEvents(
	ctx context.Context,
	sessionID string,
	iteration int,
	calls []ToolCall,
	registry ports.ToolRegistry,
) []ToolResult {
	results := make([]ToolResult, len(calls))
	e.logger.Debug("Executing %d tools in parallel", len(calls))

	// Execute in parallel using goroutines
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(idx int, tc ToolCall) {
			defer wg.Done()

			startTime := e.clock.Now()

			e.logger.Debug("Tool %d: Getting tool '%s' from registry", idx, tc.Name)
			tool, err := registry.Get(tc.Name)
			if err != nil {
				e.logger.Error("Tool %d: Tool '%s' not found in registry", idx, tc.Name)
				// EMIT: Tool error
				e.emitEvent(&ToolCallCompleteEvent{
					BaseEvent: e.newBaseEvent(ctx, sessionID),
					CallID:    tc.ID,
					ToolName:  tc.Name,
					Result:    "",
					Error:     fmt.Errorf("tool not found: %s", tc.Name),
					Duration:  e.clock.Now().Sub(startTime),
				})
				results[idx] = ToolResult{
					CallID:  tc.ID,
					Content: "",
					Error:   fmt.Errorf("tool not found: %s", tc.Name),
				}
				return
			}

			e.logger.Debug("Tool %d: Executing '%s' with args: %s", idx, tc.Name, tc.Arguments)
			result, err := tool.Execute(ctx, ports.ToolCall(tc))

			if err != nil {
				e.logger.Error("Tool %d: Execution failed: %v", idx, err)
				// EMIT: Tool error
				e.emitEvent(&ToolCallCompleteEvent{
					BaseEvent: e.newBaseEvent(ctx, sessionID),
					CallID:    tc.ID,
					ToolName:  tc.Name,
					Result:    "",
					Error:     err,
					Duration:  e.clock.Now().Sub(startTime),
				})
				results[idx] = ToolResult{
					CallID:  tc.ID,
					Content: "",
					Error:   err,
				}
				return
			}

			e.logger.Debug("Tool %d: Success, result=%d bytes", idx, len(result.Content))

			// EMIT: Tool success
			e.emitEvent(&ToolCallCompleteEvent{
				BaseEvent: e.newBaseEvent(ctx, sessionID),
				CallID:    result.CallID,
				ToolName:  tc.Name,
				Result:    result.Content,
				Error:     result.Error,
				Duration:  e.clock.Now().Sub(startTime),
				Metadata:  result.Metadata,
			})

			results[idx] = ToolResult{
				CallID:   result.CallID,
				Content:  result.Content,
				Error:    result.Error,
				Metadata: result.Metadata,
			}

			if result.Metadata != nil {
				if info, ok := result.Metadata["browser_info"].(map[string]any); ok {
					e.emitBrowserInfoEvent(ctx, sessionID, info)
				}
			}
		}(i, call)
	}

	wg.Wait()
	e.logger.Debug("All %d tools completed execution", len(calls))
	return results
}

// parseToolCalls extracts tool calls from assistant message
func (e *ReactEngine) parseToolCalls(msg Message, parser ports.FunctionCallParser) []ToolCall {
	// If message has explicit tool calls (native function calling)
	if len(msg.ToolCalls) > 0 {
		e.logger.Debug("Using native tool calls from message: count=%d", len(msg.ToolCalls))
		return msg.ToolCalls
	}

	// Otherwise, parse from content (XML or JSON format)
	e.logger.Debug("Parsing tool calls from content: length=%d", len(msg.Content))
	parsed, err := parser.Parse(msg.Content)
	if err != nil {
		e.logger.Warn("Failed to parse tool calls from content: %v", err)
		return nil
	}

	// Convert ports.ToolCall to domain.ToolCall
	calls := make([]ToolCall, 0, len(parsed))
	calls = append(calls, parsed...)

	e.logger.Debug("Parsed %d tool calls from content", len(calls))
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
		Role:        "user", // Observations come back as user messages
		Content:     content,
		ToolResults: results,
	}
}

func coerceToInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}

func (e *ReactEngine) emitBrowserInfoEvent(ctx context.Context, sessionID string, metadata map[string]any) {
	level := ports.GetOutputContext(ctx).Level
	captured := e.clock.Now()
	if tsRaw, ok := metadata["captured_at"].(string); ok {
		if ts, err := time.Parse(time.RFC3339, tsRaw); err == nil {
			captured = ts
		}
	}

	var successPtr *bool
	switch v := metadata["success"].(type) {
	case bool:
		success := v
		successPtr = &success
	case *bool:
		successPtr = v
	}

	message, _ := metadata["message"].(string)
	userAgent, _ := metadata["user_agent"].(string)
	cdpURL, _ := metadata["cdp_url"].(string)
	vncURL, _ := metadata["vnc_url"].(string)

	viewportWidth := coerceToInt(metadata["viewport_width"])
	viewportHeight := coerceToInt(metadata["viewport_height"])

	event := NewBrowserInfoEvent(level, sessionID, captured, successPtr, message, userAgent, cdpURL, vncURL, viewportWidth, viewportHeight)
	e.emitEvent(event)
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
		Answer:     finalAnswer,
		Messages:   state.Messages,
		Iterations: state.Iterations,
		TokensUsed: state.TokenCount,
		StopReason: stopReason,
	}
}

// cleanToolCallMarkers removes leaked tool call XML markers from content
func (e *ReactEngine) cleanToolCallMarkers(content string) string {
	// Remove incomplete tool call markers that LLM might output
	patterns := []string{
		`<\|tool_call_begin\|>.*`,
		`<tool_call>.*(?:</tool_call>)?$`,
		`user<\|tool_call_begin\|>.*`,
		`functions\.[\w_]+:\d+\(.*`,
	}

	cleaned := content
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	return strings.TrimSpace(cleaned)
}
