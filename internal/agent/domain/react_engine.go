package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
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
	EventListener      EventListener
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
		eventListener: cfg.EventListener,
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

func (e *ReactEngine) newBaseEvent(ctx context.Context, sessionID, taskID, parentTaskID string) BaseEvent {
	return newBaseEventWithIDs(e.getAgentLevel(ctx), sessionID, taskID, parentTaskID, e.clock.Now())
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

	ensureAttachmentStore(state)
	// Register attachments from preloaded messages so they are available for
	// placeholder substitution and multimodal requests.
	attachmentsChanged := false
	for _, existing := range state.Messages {
		if registerMessageAttachments(state, existing) {
			attachmentsChanged = true
		}
	}
	if attachmentsChanged {
		e.updateAttachmentCatalogMessage(state)
	}

	// Ensure the system prompt (if provided) is always present at the front
	e.ensureSystemPromptMessage(state)

	// ALWAYS append the new user task to messages (even if history exists)
	userMessage := Message{
		Role:    "user",
		Content: task,
		Source:  ports.MessageSourceUserInput,
	}
	if len(state.PendingUserAttachments) > 0 {
		attachments := make(map[string]ports.Attachment, len(state.PendingUserAttachments))
		for key, att := range state.PendingUserAttachments {
			attachments[key] = att
		}
		userMessage.Attachments = attachments
		state.PendingUserAttachments = nil
	}

	state.Messages = append(state.Messages, userMessage)
	if registerMessageAttachments(state, userMessage) {
		e.updateAttachmentCatalogMessage(state)
	}
	if len(userMessage.Attachments) > 0 {
		e.logger.Debug("Registered %d user attachments", len(userMessage.Attachments))
	}
	e.logger.Debug("Added user task to messages. Total messages: %d", len(state.Messages))

	// ReAct loop: Think → Act → Observe
	for state.Iterations < e.maxIterations {
		// Check if context is cancelled before starting iteration
		if ctx.Err() != nil {
			e.logger.Info("Context cancelled, stopping execution: %v", ctx.Err())
			finalResult := e.finalize(state, "cancelled")
			attachments := e.decorateFinalResult(state, finalResult)

			// EMIT: Task complete with cancellation
			e.emitEvent(&TaskCompleteEvent{
				BaseEvent:       e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				FinalAnswer:     finalResult.Answer,
				TotalIterations: finalResult.Iterations,
				TotalTokens:     finalResult.TokensUsed,
				StopReason:      "cancelled",
				Duration:        e.clock.Now().Sub(startTime),
				Attachments:     attachments,
			})

			return nil, ctx.Err()
		}

		state.Iterations++
		e.logger.Info("=== Iteration %d/%d ===", state.Iterations, e.maxIterations)

		// EMIT: Iteration started
		e.emitEvent(&IterationStartEvent{
			BaseEvent:  e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
			Iteration:  state.Iterations,
			TotalIters: e.maxIterations,
		})

		// 1. THINK: Get LLM reasoning
		e.logger.Debug("THINK phase: Calling LLM with %d messages", len(state.Messages))

		// EMIT: Thinking
		e.emitEvent(&ThinkingEvent{
			BaseEvent:    e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
			Iteration:    state.Iterations,
			MessageCount: len(state.Messages),
		})

		thought, err := e.think(ctx, state, services)
		if err != nil {
			e.logger.Error("Think step failed: %v", err)
			// EMIT: Error
			e.emitEvent(&ErrorEvent{
				BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				Iteration:   state.Iterations,
				Phase:       "think",
				Error:       err,
				Recoverable: false,
			})
			return nil, fmt.Errorf("think step failed: %w", err)
		}

		// Add thought to state
		if att := resolveContentAttachments(thought.Content, state); len(att) > 0 {
			thought.Attachments = att
		}
		trimmedThought := strings.TrimSpace(thought.Content)
		if trimmedThought != "" || len(thought.ToolCalls) > 0 {
			state.Messages = append(state.Messages, thought)
		}
		e.logger.Debug("LLM response: content_length=%d, tool_calls=%d",
			len(thought.Content), len(thought.ToolCalls))

		// 2. ACT: Parse and execute tool calls
		toolCalls := e.parseToolCalls(thought, services.Parser)
		e.logger.Info("Parsed %d tool calls", len(toolCalls))

		if len(toolCalls) == 0 {
			trimmed := strings.TrimSpace(thought.Content)
			if trimmed == "" {
				e.logger.Warn("No tool calls and empty content - continuing loop")
				continue
			}

			e.logger.Info("No tool calls with content - treating response as final answer")
			finalResult := e.finalize(state, "final_answer")
			attachments := e.decorateFinalResult(state, finalResult)
			e.emitEvent(&TaskCompleteEvent{
				BaseEvent:       e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				FinalAnswer:     finalResult.Answer,
				TotalIterations: finalResult.Iterations,
				TotalTokens:     finalResult.TokensUsed,
				StopReason:      "final_answer",
				Duration:        e.clock.Now().Sub(startTime),
				Attachments:     attachments,
			})
			return finalResult, nil
		} else {
			// EMIT: Think complete
			e.emitEvent(&ThinkCompleteEvent{
				BaseEvent:     e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				Iteration:     state.Iterations,
				Content:       thought.Content,
				ToolCallCount: len(thought.ToolCalls),
			})
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
		for idx := range validCalls {
			call := validCalls[idx]
			e.emitEvent(&ToolCallStartEvent{
				BaseEvent: e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				Iteration: state.Iterations,
				CallID:    call.ID,
				ToolName:  call.Name,
				Arguments: call.Arguments,
			})
		}

		results := e.executeToolsWithEvents(ctx, state, state.Iterations, validCalls, services.ToolExecutor)
		state.ToolResults = append(state.ToolResults, results...)
		e.observeToolResults(state, state.Iterations, results)

		// Log results (no stdout printing - let TUI handle display)
		for i, r := range results {
			if r.Error != nil {
				e.logger.Warn("Tool %d failed: %v", i, r.Error)
			} else {
				e.logger.Debug("Tool %d succeeded: result_length=%d", i, len(r.Content))
			}
		}

		// 3. OBSERVE: Add results to conversation
		toolMessages := e.buildToolMessages(results)
		state.Messages = append(state.Messages, toolMessages...)
		attachmentsChanged := false
		for _, msg := range toolMessages {
			if registerMessageAttachments(state, msg) {
				attachmentsChanged = true
			}
		}
		if attachmentsChanged {
			e.updateAttachmentCatalogMessage(state)
		}
		e.logger.Debug("OBSERVE phase: Added %d tool message(s) to state", len(toolMessages))

		// 4. Check context limits
		tokenCount := services.Context.EstimateTokens(state.Messages)
		state.TokenCount = tokenCount
		e.logger.Debug("Current token count: %d", tokenCount)

		// EMIT: Iteration complete
		e.emitEvent(&IterationCompleteEvent{
			BaseEvent:  e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
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
			Source:  ports.MessageSourceSystemPrompt,
		})

		// One final LLM call for answer
		finalThought, err := e.think(ctx, state, services)
		if err == nil && finalThought.Content != "" {
			if att := resolveContentAttachments(finalThought.Content, state); len(att) > 0 {
				finalThought.Attachments = att
			}
			state.Messages = append(state.Messages, finalThought)
			if registerMessageAttachments(state, finalThought) {
				e.updateAttachmentCatalogMessage(state)
			}
			finalResult.Answer = finalThought.Content
			e.logger.Info("Got final answer from retry: %d chars", len(finalResult.Answer))
		}
	}

	// EMIT: Task complete
	attachments := e.decorateFinalResult(state, finalResult)
	e.emitEvent(&TaskCompleteEvent{
		BaseEvent:       e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
		FinalAnswer:     finalResult.Answer,
		TotalIterations: finalResult.Iterations,
		TotalTokens:     finalResult.TokensUsed,
		StopReason:      finalResult.StopReason,
		Duration:        e.clock.Now().Sub(startTime),
		Attachments:     attachments,
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
	requestID := id.NewRequestID()
	filteredMessages, excluded := splitMessagesForLLM(state.Messages)

	e.logger.Debug(
		"Preparing LLM request (request_id=%s): messages=%d (filtered=%d, excluded=%d), tools=%d",
		requestID,
		len(state.Messages),
		len(filteredMessages),
		len(excluded),
		len(tools),
	)

	req := ports.CompletionRequest{
		Messages:    filteredMessages,
		Tools:       tools,
		Temperature: e.completion.temperature,
		MaxTokens:   e.completion.maxTokens,
		TopP:        e.completion.topP,
		Metadata: map[string]any{
			"request_id": requestID,
		},
	}

	if len(e.completion.stopSequences) > 0 {
		req.StopSequences = append([]string(nil), e.completion.stopSequences...)
	}

	timestamp := e.clock.Now()
	snapshot := NewContextSnapshotEvent(
		e.getAgentLevel(ctx),
		state.SessionID,
		state.TaskID,
		state.ParentTaskID,
		state.Iterations,
		state.Iterations,
		requestID,
		filteredMessages,
		excluded,
		timestamp,
	)
	e.emitEvent(snapshot)
	if services.Context != nil {
		summary := snapshotSummaryFromMessages(state.Messages)
		record := buildContextTurnRecord(state, filteredMessages, timestamp, summary)
		if err := services.Context.RecordTurn(ctx, record); err != nil {
			e.logger.Warn("Failed to persist turn snapshot: %v", err)
		}
	}

	// Call LLM (streaming when available)
	e.logger.Debug("Calling LLM (request_id=%s)...", requestID)

	var resp *ports.CompletionResponse
	var err error
	modelName := ""
	if services.LLM != nil {
		modelName = services.LLM.Model()
	}

	if streamingClient, ok := services.LLM.(ports.StreamingLLMClient); ok {
		callbacks := ports.CompletionStreamCallbacks{}

		callbacks.OnContentDelta = func(delta ports.ContentDelta) {
			if delta.Delta == "" && !delta.Final {
				return
			}
			event := &AssistantMessageEvent{
				BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				Iteration:   state.Iterations,
				Delta:       delta.Delta,
				Final:       delta.Final,
				CreatedAt:   e.clock.Now(),
				SourceModel: modelName,
			}
			e.emitEvent(event)
		}

		resp, err = streamingClient.StreamComplete(ctx, req, callbacks)
	} else {
		resp, err = services.LLM.Complete(ctx, req)
	}

	if err != nil {
		e.logger.Error("LLM call failed (request_id=%s): %v", requestID, err)
		return Message{}, fmt.Errorf("LLM call failed: %w", err)
	}

	e.logger.Debug("LLM response received (request_id=%s): content=%d bytes, tool_calls=%d",
		requestID, len(resp.Content), len(resp.ToolCalls))

	// Convert response to domain message
	return Message{
		Role:      "assistant",
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
		Source:    ports.MessageSourceAssistantReply,
	}, nil
}

// executeToolsWithEvents runs all tool calls in parallel and emits completion events
func (e *ReactEngine) executeToolsWithEvents(
	ctx context.Context,
	state *TaskState,
	iteration int,
	calls []ToolCall,
	registry ports.ToolRegistry,
) []ToolResult {
	results := make([]ToolResult, len(calls))
	e.logger.Debug("Executing %d tools in parallel", len(calls))

	// Execute in parallel using goroutines
	var (
		wg            sync.WaitGroup
		attachmentsMu sync.Mutex
	)
	attachmentsSnapshot, iterationSnapshot := snapshotAttachments(state)
	for i, call := range calls {
		wg.Add(1)
		go func(idx int, tc ToolCall) {
			defer wg.Done()

			tc.SessionID = state.SessionID
			tc.TaskID = state.TaskID
			tc.ParentTaskID = state.ParentTaskID

			tc.Arguments = e.expandPlaceholders(tc.Arguments, state)

			startTime := e.clock.Now()

			e.logger.Debug("Tool %d: Getting tool '%s' from registry", idx, tc.Name)
			tool, err := registry.Get(tc.Name)
			if err != nil {
				e.logger.Error("Tool %d: Tool '%s' not found in registry", idx, tc.Name)
				// EMIT: Tool error
				e.emitEvent(&ToolCallCompleteEvent{
					BaseEvent: e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
					CallID:    tc.ID,
					ToolName:  tc.Name,
					Result:    "",
					Error:     fmt.Errorf("tool not found: %s", tc.Name),
					Duration:  e.clock.Now().Sub(startTime),
				})
				results[idx] = ToolResult{
					CallID:       tc.ID,
					Content:      "",
					Error:        fmt.Errorf("tool not found: %s", tc.Name),
					SessionID:    state.SessionID,
					TaskID:       state.TaskID,
					ParentTaskID: state.ParentTaskID,
				}
				return
			}

			toolCtx := ports.WithAttachmentContext(ctx, attachmentsSnapshot, iterationSnapshot)

			formattedArgs := formatToolArgumentsForLog(tc.Arguments)
			e.logger.Debug("Tool %d: Executing '%s' with args: %s", idx, tc.Name, formattedArgs)
			result, err := tool.Execute(toolCtx, ports.ToolCall(tc))

			if err != nil {
				e.logger.Error("Tool %d: Execution failed: %v", idx, err)
				// EMIT: Tool error
				e.emitEvent(&ToolCallCompleteEvent{
					BaseEvent: e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
					CallID:    tc.ID,
					ToolName:  tc.Name,
					Result:    "",
					Error:     err,
					Duration:  e.clock.Now().Sub(startTime),
				})
				results[idx] = ToolResult{
					CallID:       tc.ID,
					Content:      "",
					Error:        err,
					SessionID:    state.SessionID,
					TaskID:       state.TaskID,
					ParentTaskID: state.ParentTaskID,
				}
				return
			}

			e.logger.Debug("Tool %d: Success, result=%d bytes", idx, len(result.Content))

			// EMIT: Tool success
			e.emitEvent(&ToolCallCompleteEvent{
				BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				CallID:      result.CallID,
				ToolName:    tc.Name,
				Result:      result.Content,
				Error:       result.Error,
				Duration:    e.clock.Now().Sub(startTime),
				Metadata:    result.Metadata,
				Attachments: result.Attachments,
			})

			if result.CallID == "" {
				result.CallID = tc.ID
			}
			if result.SessionID == "" {
				result.SessionID = state.SessionID
			}
			if result.TaskID == "" {
				result.TaskID = state.TaskID
			}
			if result.ParentTaskID == "" {
				result.ParentTaskID = state.ParentTaskID
			}

			if len(result.Attachments) > 0 {
				attachmentsMu.Lock()
				ensureAttachmentStore(state)
				for key, att := range result.Attachments {
					if att.Name == "" {
						att.Name = key
					}
					placeholder := key
					if placeholder == "" {
						placeholder = att.Name
					}
					if placeholder == "" {
						continue
					}
					state.Attachments[placeholder] = att
					if state.AttachmentIterations == nil {
						state.AttachmentIterations = make(map[string]int)
					}
					state.AttachmentIterations[placeholder] = state.Iterations
				}
				attachmentsMu.Unlock()
			}

			results[idx] = ToolResult{
				CallID:       result.CallID,
				Content:      result.Content,
				Error:        result.Error,
				Metadata:     result.Metadata,
				SessionID:    result.SessionID,
				TaskID:       result.TaskID,
				ParentTaskID: result.ParentTaskID,
				Attachments:  result.Attachments,
			}

			if result.Metadata != nil {
				if info, ok := result.Metadata["browser_info"].(map[string]any); ok {
					e.emitBrowserInfoEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID, info)
				}
			}
		}(i, call)
	}

	wg.Wait()
	e.logger.Debug("All %d tools completed execution", len(calls))
	return results
}

const (
	toolArgInlineLengthLimit = 256
	toolArgPreviewLength     = 64
	toolResultPreviewRunes   = 280
	maxFeedbackSignals       = 20
)

func formatToolArgumentsForLog(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	sanitized := sanitizeToolArgumentsForLog(args)
	if len(sanitized) == 0 {
		return "{}"
	}
	if encoded, err := json.Marshal(sanitized); err == nil {
		return string(encoded)
	}
	return fmt.Sprintf("%v", sanitized)
}

func sanitizeToolArgumentsForLog(args map[string]any) map[string]any {
	if args == nil {
		return nil
	}
	sanitized := make(map[string]any, len(args))
	for key, value := range args {
		sanitized[key] = summarizeToolArgumentValue(key, value)
	}
	return sanitized
}

func summarizeToolArgumentValue(key string, value any) any {
	switch v := value.(type) {
	case string:
		return summarizeToolArgumentString(key, v)
	case map[string]any:
		return sanitizeToolArgumentsForLog(v)
	case []any:
		summarized := make([]any, 0, len(v))
		for idx, item := range v {
			summarized = append(summarized, summarizeToolArgumentValue(fmt.Sprintf("%s[%d]", key, idx), item))
		}
		return summarized
	case []string:
		summarized := make([]string, 0, len(v))
		for _, item := range v {
			summarized = append(summarized, summarizeToolArgumentString(key, item))
		}
		return summarized
	default:
		return value
	}
}

func summarizeToolArgumentString(key, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return trimmed
	}

	lowerKey := strings.ToLower(key)
	if strings.HasPrefix(trimmed, "data:") {
		return summarizeDataURIForLog(trimmed)
	}

	if strings.Contains(lowerKey, "image") {
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			return trimmed
		}
		if len(trimmed) > toolArgInlineLengthLimit || looksLikeBinaryString(trimmed) {
			return summarizeBinaryLikeString(trimmed)
		}
		return trimmed
	}

	if looksLikeBinaryString(trimmed) {
		return summarizeBinaryLikeString(trimmed)
	}

	if len(trimmed) > toolArgInlineLengthLimit {
		return summarizeLongPlainString(trimmed)
	}

	return trimmed
}

func summarizeDataURIForLog(value string) string {
	comma := strings.Index(value, ",")
	if comma == -1 {
		return fmt.Sprintf("data_uri(len=%d)", len(value))
	}
	header := value[:comma]
	payload := value[comma+1:]
	preview := truncateStringForLog(payload, toolArgPreviewLength)
	if len(payload) > len(preview) {
		preview += "..."
	}
	return fmt.Sprintf("data_uri(header=%q,len=%d,payload_prefix=%q)", header, len(value), preview)
}

func summarizeBinaryLikeString(value string) string {
	preview := truncateStringForLog(value, toolArgPreviewLength)
	if len(value) > len(preview) {
		preview += "..."
	}
	return fmt.Sprintf("base64(len=%d,prefix=%q)", len(value), preview)
}

func summarizeLongPlainString(value string) string {
	preview := truncateStringForLog(value, toolArgPreviewLength)
	if len(value) > len(preview) {
		preview += "..."
	}
	return fmt.Sprintf("%s (len=%d)", preview, len(value))
}

func looksLikeBinaryString(value string) bool {
	if len(value) < toolArgInlineLengthLimit {
		return false
	}
	sample := value
	const sampleSize = 128
	if len(sample) > sampleSize {
		sample = sample[:sampleSize]
	}
	for i := 0; i < len(sample); i++ {
		c := sample[i]
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '+' || c == '/' || c == '=' || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func truncateStringForLog(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runeCount := 0
	for idx := range value {
		if runeCount == limit {
			return value[:idx]
		}
		runeCount++
	}
	return value
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

// buildToolMessages converts tool results into messages sent back to the LLM.
func (e *ReactEngine) buildToolMessages(results []ToolResult) []Message {
	messages := make([]Message, 0, len(results))

	for _, result := range results {
		var content string
		if result.Error != nil {
			content = fmt.Sprintf("Tool %s failed: %v", result.CallID, result.Error)
		} else if trimmed := strings.TrimSpace(result.Content); trimmed != "" {
			content = trimmed
		} else {
			content = fmt.Sprintf("Tool %s completed successfully.", result.CallID)
		}

		content = ensureToolAttachmentReferences(content, result.Attachments)

		msg := Message{
			Role:        "tool",
			Content:     content,
			ToolCallID:  result.CallID,
			ToolResults: []ToolResult{result},
			Source:      ports.MessageSourceToolResult,
		}

		msg.Attachments = normalizeToolAttachments(result.Attachments)

		messages = append(messages, msg)
	}

	return messages
}

func ensureToolAttachmentReferences(content string, attachments map[string]ports.Attachment) string {
	if len(attachments) == 0 {
		return strings.TrimSpace(content)
	}

	normalized := strings.TrimSpace(content)
	mentioned := make(map[string]bool, len(attachments))

	keys := sortedAttachmentKeys(attachments)
	for _, name := range keys {
		placeholder := fmt.Sprintf("[%s]", name)
		if strings.Contains(normalized, placeholder) {
			mentioned[name] = true
		}
	}

	var builder strings.Builder
	if normalized != "" {
		builder.WriteString(normalized)
		builder.WriteString("\n\n")
	}
	builder.WriteString("Attachments available for follow-up steps:\n")
	for _, name := range keys {
		fmt.Fprintf(&builder, "- [%s]%s\n", name, boolToStar(mentioned[name]))
	}

	return strings.TrimSpace(builder.String())
}

func snapshotAttachments(state *TaskState) (map[string]ports.Attachment, map[string]int) {
	if state == nil {
		return nil, nil
	}
	var attachments map[string]ports.Attachment
	if len(state.Attachments) > 0 {
		attachments = make(map[string]ports.Attachment, len(state.Attachments))
		for key, att := range state.Attachments {
			attachments[key] = att
		}
	}
	var iterations map[string]int
	if len(state.AttachmentIterations) > 0 {
		iterations = make(map[string]int, len(state.AttachmentIterations))
		for key, iter := range state.AttachmentIterations {
			iterations[key] = iter
		}
	}
	return attachments, iterations
}

func boolToStar(b bool) string {
	if b {
		return " (referenced)"
	}
	return ""
}

func normalizeToolAttachments(attachments map[string]ports.Attachment) map[string]ports.Attachment {
	if len(attachments) == 0 {
		return nil
	}
	normalized := make(map[string]ports.Attachment, len(attachments))
	for _, key := range sortedAttachmentKeys(attachments) {
		att := attachments[key]
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		if att.Name == "" {
			att.Name = placeholder
		}
		normalized[placeholder] = att
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func splitMessagesForLLM(messages []Message) ([]Message, []Message) {
	if len(messages) == 0 {
		return nil, nil
	}
	filtered := make([]Message, 0, len(messages))
	excluded := make([]Message, 0)
	for _, msg := range messages {
		cloned := cloneMessageForLLM(msg)
		switch msg.Source {
		case ports.MessageSourceDebug, ports.MessageSourceEvaluation:
			excluded = append(excluded, cloned)
		default:
			filtered = append(filtered, cloned)
		}
	}
	return filtered, excluded
}

func cloneMessageForLLM(msg Message) Message {
	cloned := msg
	if len(msg.ToolCalls) > 0 {
		cloned.ToolCalls = append([]ToolCall(nil), msg.ToolCalls...)
	}
	if len(msg.ToolResults) > 0 {
		cloned.ToolResults = make([]ToolResult, len(msg.ToolResults))
		for i, result := range msg.ToolResults {
			cloned.ToolResults[i] = cloneToolResultForLLM(result)
		}
	}
	if len(msg.Metadata) > 0 {
		metadata := make(map[string]any, len(msg.Metadata))
		for key, value := range msg.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(msg.Attachments) > 0 {
		cloned.Attachments = cloneAttachmentMapForLLM(msg.Attachments)
	}
	return cloned
}

func cloneToolResultForLLM(result ToolResult) ToolResult {
	cloned := result
	if len(result.Metadata) > 0 {
		metadata := make(map[string]any, len(result.Metadata))
		for key, value := range result.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(result.Attachments) > 0 {
		cloned.Attachments = cloneAttachmentMapForLLM(result.Attachments)
	}
	return cloned
}

func cloneAttachmentMapForLLM(values map[string]ports.Attachment) map[string]ports.Attachment {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]ports.Attachment, len(values))
	for key, att := range values {
		cloned[key] = att
	}
	return cloned
}

func sortedAttachmentKeys(attachments map[string]ports.Attachment) []string {
	if len(attachments) == 0 {
		return nil
	}
	keys := make([]string, 0, len(attachments))
	seen := make(map[string]bool, len(attachments))
	for key := range attachments {
		name := strings.TrimSpace(key)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
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

func (e *ReactEngine) emitBrowserInfoEvent(ctx context.Context, sessionID, taskID, parentTaskID string, metadata map[string]any) {
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

	event := NewBrowserInfoEvent(level, sessionID, taskID, parentTaskID, captured, successPtr, message, userAgent, cdpURL, vncURL, viewportWidth, viewportHeight)
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
		Answer:       finalAnswer,
		Messages:     state.Messages,
		Iterations:   state.Iterations,
		TokensUsed:   state.TokenCount,
		StopReason:   stopReason,
		SessionID:    state.SessionID,
		TaskID:       state.TaskID,
		ParentTaskID: state.ParentTaskID,
	}
}

func (e *ReactEngine) decorateFinalResult(state *TaskState, result *TaskResult) map[string]ports.Attachment {
	if state == nil || result == nil {
		return nil
	}

	attachments := resolveContentAttachments(result.Answer, state)
	result.Answer = ensureAttachmentPlaceholders(result.Answer, attachments)
	if len(attachments) == 0 {
		return nil
	}
	return attachments
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

func (e *ReactEngine) ensureSystemPromptMessage(state *TaskState) {
if state == nil {
return
}

prompt := strings.TrimSpace(state.SystemPrompt)
if prompt == "" {
return
}

rewriteExisting := func(idx int) {
if idx < 0 || idx >= len(state.Messages) {
return
}
updated := state.Messages[idx]
updated.Role = "system"
updated.Source = ports.MessageSourceSystemPrompt
updated.Content = state.SystemPrompt
if idx == 0 {
state.Messages[0] = updated
return
}
state.Messages = append([]Message{updated}, append(state.Messages[:idx], state.Messages[idx+1:]...)...)
}

for idx, msg := range state.Messages {
role := strings.ToLower(strings.TrimSpace(msg.Role))
if msg.Source != ports.MessageSourceSystemPrompt && role != "system" {
continue
}
if msg.Source != ports.MessageSourceSystemPrompt && role == "system" {
// Treat legacy system-role messages (without explicit source) as candidates.
rewriteExisting(idx)
return
}
if strings.TrimSpace(msg.Content) == prompt {
if idx == 0 {
return
}
state.Messages = append([]Message{msg}, append(state.Messages[:idx], state.Messages[idx+1:]...)...)
return
}
rewriteExisting(idx)
return
}

systemMessage := Message{
Role:    "system",
Content: state.SystemPrompt,
Source:  ports.MessageSourceSystemPrompt,
}

state.Messages = append([]Message{systemMessage}, state.Messages...)
e.logger.Debug("Inserted system prompt into message history")
}

func ensureAttachmentStore(state *TaskState) {
	if state.Attachments == nil {
		state.Attachments = make(map[string]ports.Attachment)
	}
	if state.AttachmentIterations == nil {
		state.AttachmentIterations = make(map[string]int)
	}
}

func registerMessageAttachments(state *TaskState, msg Message) bool {
	if len(msg.Attachments) == 0 {
		return false
	}
	ensureAttachmentStore(state)
	changed := false
	for key, att := range msg.Attachments {
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		if att.Name == "" {
			att.Name = placeholder
		}
		if existing, ok := state.Attachments[placeholder]; !ok || existing != att {
			state.Attachments[placeholder] = att
			changed = true
		}
		if state.AttachmentIterations == nil {
			state.AttachmentIterations = make(map[string]int)
		}
		state.AttachmentIterations[placeholder] = state.Iterations
	}
	return changed
}

const attachmentCatalogMetadataKey = "attachment_catalog"

func (e *ReactEngine) updateAttachmentCatalogMessage(state *TaskState) {
	if state == nil {
		return
	}
	content := buildAttachmentCatalogContent(state)
	if strings.TrimSpace(content) == "" {
		removeAttachmentCatalogMessage(state)
		return
	}

	if idx := findAttachmentCatalogMessageIndex(state); idx >= 0 {
		state.Messages = append(state.Messages[:idx], state.Messages[idx+1:]...)
	}
	note := Message{
		Role:    "system",
		Content: content,
		Source:  ports.MessageSourceSystemPrompt,
		Metadata: map[string]any{
			attachmentCatalogMetadataKey: true,
		},
	}
	state.Messages = append(state.Messages, note)
}

func buildAttachmentCatalogContent(state *TaskState) string {
	if state == nil || len(state.Attachments) == 0 {
		return ""
	}
	keys := sortedAttachmentKeys(state.Attachments)
	if len(keys) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("Attachment catalog (for model reference only).\n")
	builder.WriteString("Reference assets by typing their placeholders exactly as shown (e.g., [diagram.png]).\n\n")

	for i, key := range keys {
		att := state.Attachments[key]
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("%d. [%s]", i+1, placeholder))
		description := strings.TrimSpace(att.Description)
		if description != "" {
			builder.WriteString(" — " + description)
		}
		builder.WriteString("\n")
	}

	builder.WriteString("\nUse the placeholders verbatim to work with these attachments in follow-up steps.")
	if hint := attachmentSandboxPathHint(state); hint != "" {
		builder.WriteString("\nFiles are mirrored inside the sandbox under ")
		builder.WriteString(hint)
		builder.WriteString(".")
	}

	return strings.TrimSpace(builder.String())
}

func attachmentSandboxPathHint(state *TaskState) string {
	if state == nil {
		return ""
	}
	session := strings.TrimSpace(state.SessionID)
	if session == "" {
		return "/workspace/.alex/sessions/<session>/attachments"
	}
	return fmt.Sprintf("/workspace/.alex/sessions/%s/attachments", session)
}

func findAttachmentCatalogMessageIndex(state *TaskState) int {
	if state == nil || len(state.Messages) == 0 {
		return -1
	}
	for i := len(state.Messages) - 1; i >= 0; i-- {
		msg := state.Messages[i]
		if msg.Metadata == nil {
			continue
		}
		if flag, ok := msg.Metadata[attachmentCatalogMetadataKey]; ok {
			if enabled, ok := flag.(bool); ok && enabled {
				return i
			}
		}
	}
	return -1
}

func removeAttachmentCatalogMessage(state *TaskState) {
	idx := findAttachmentCatalogMessageIndex(state)
	if idx < 0 {
		return
	}
	state.Messages = append(state.Messages[:idx], state.Messages[idx+1:]...)
}

func (e *ReactEngine) expandPlaceholders(args map[string]any, state *TaskState) map[string]any {
	if len(args) == 0 {
		return args
	}
	expanded := make(map[string]any, len(args))
	for key, value := range args {
		expanded[key] = e.expandPlaceholderValue(value, state)
	}
	return expanded
}

func (e *ReactEngine) expandPlaceholderValue(value any, state *TaskState) any {
	switch v := value.(type) {
	case string:
		if replacement, ok := e.resolveStringAttachmentValue(v, state); ok {
			return replacement
		}
		return v
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = e.expandPlaceholderValue(item, state)
		}
		return out
	case []string:
		out := make([]string, len(v))
		for i, item := range v {
			if replacement, ok := e.resolveStringAttachmentValue(item, state); ok {
				out[i] = replacement
				continue
			}
			out[i] = item
		}
		return out
	case map[string]any:
		nested := make(map[string]any, len(v))
		for key, item := range v {
			nested[key] = e.expandPlaceholderValue(item, state)
		}
		return nested
	case map[string]string:
		nested := make(map[string]string, len(v))
		for key, item := range v {
			if replacement, ok := e.resolveStringAttachmentValue(item, state); ok {
				nested[key] = replacement
				continue
			}
			nested[key] = item
		}
		return nested
	default:
		return value
	}
}

func attachmentReferenceValue(att ports.Attachment) string {
	if uri := strings.TrimSpace(att.URI); uri != "" {
		return uri
	}
	data := strings.TrimSpace(att.Data)
	if data != "" {
		if strings.HasPrefix(data, "data:") {
			return data
		}
		mediaType := strings.TrimSpace(att.MediaType)
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		return fmt.Sprintf("data:%s;base64,%s", mediaType, data)
	}
	return ""
}

var genericImageAliasPattern = regexp.MustCompile(`(?i)^image(?:[_\-\s]?(\d+))?(?:\.[a-z0-9]+)?$`)

func (e *ReactEngine) lookupAttachmentByName(name string, state *TaskState) (ports.Attachment, string, bool) {
	att, canonical, kind, ok := lookupAttachmentByNameInternal(name, state)
	if !ok {
		return ports.Attachment{}, "", false
	}

	switch kind {
	case attachmentMatchSeedreamAlias:
		e.logger.Info("Resolved Seedream placeholder alias [%s] -> [%s]", name, canonical)
	case attachmentMatchGeneric:
		e.logger.Info("Mapping generic image placeholder [%s] to attachment [%s]", name, canonical)
	}

	return att, canonical, true
}

const (
	attachmentMatchExact           = "exact"
	attachmentMatchCaseInsensitive = "case_insensitive"
	attachmentMatchSeedreamAlias   = "seedream_alias"
	attachmentMatchGeneric         = "generic_alias"
)

func lookupAttachmentByNameInternal(name string, state *TaskState) (ports.Attachment, string, string, bool) {
	if state == nil {
		return ports.Attachment{}, "", "", false
	}

	if att, ok := state.Attachments[name]; ok {
		return att, name, attachmentMatchExact, true
	}

	for key, att := range state.Attachments {
		if strings.EqualFold(key, name) {
			return att, key, attachmentMatchCaseInsensitive, true
		}
	}

	if canonical, att, ok := matchSeedreamPlaceholderAlias(name, state); ok {
		return att, canonical, attachmentMatchSeedreamAlias, true
	}

	if canonical, att, ok := matchGenericImageAlias(name, state); ok {
		return att, canonical, attachmentMatchGeneric, true
	}

	return ports.Attachment{}, "", "", false
}

func matchSeedreamPlaceholderAlias(name string, state *TaskState) (string, ports.Attachment, bool) {
	if state == nil || len(state.Attachments) == 0 {
		return "", ports.Attachment{}, false
	}

	trimmed := strings.TrimSpace(name)
	dot := strings.LastIndex(trimmed, ".")
	if dot <= 0 {
		return "", ports.Attachment{}, false
	}

	ext := strings.ToLower(trimmed[dot:])
	base := trimmed[:dot]
	underscore := strings.LastIndex(base, "_")
	if underscore <= 0 {
		return "", ports.Attachment{}, false
	}

	indexPart := base[underscore+1:]
	if _, err := strconv.Atoi(indexPart); err != nil {
		return "", ports.Attachment{}, false
	}

	prefix := strings.ToLower(strings.TrimSpace(base[:underscore]))
	if prefix == "" {
		return "", ports.Attachment{}, false
	}

	prefixWithSeparator := prefix + "_"
	suffix := fmt.Sprintf("_%s%s", indexPart, ext)

	var (
		chosenKey  string
		chosenAtt  ports.Attachment
		chosenIter int
		found      bool
	)

	for key, att := range state.Attachments {
		if !strings.EqualFold(strings.TrimSpace(att.Source), "seedream") {
			continue
		}
		lowerKey := strings.ToLower(key)
		if !strings.HasSuffix(lowerKey, suffix) {
			continue
		}
		if !strings.HasPrefix(lowerKey, prefixWithSeparator) {
			continue
		}
		middle := strings.TrimSuffix(strings.TrimPrefix(lowerKey, prefixWithSeparator), suffix)
		if middle == "" {
			continue
		}

		iter := 0
		if state.AttachmentIterations != nil {
			iter = state.AttachmentIterations[key]
		}

		if !found || iter > chosenIter {
			found = true
			chosenKey = key
			chosenAtt = att
			chosenIter = iter
		}
	}

	if !found {
		return "", ports.Attachment{}, false
	}

	return chosenKey, chosenAtt, true
}

func matchGenericImageAlias(name string, state *TaskState) (string, ports.Attachment, bool) {
	trimmed := strings.TrimSpace(name)
	match := genericImageAliasPattern.FindStringSubmatch(trimmed)
	if match == nil {
		return "", ports.Attachment{}, false
	}

	candidates := collectImageAttachmentCandidates(state)
	if len(candidates) == 0 {
		return "", ports.Attachment{}, false
	}

	index := len(candidates) - 1
	if len(match) > 1 && match[1] != "" {
		if parsed, err := strconv.Atoi(match[1]); err == nil && parsed > 0 {
			idx := parsed - 1
			if idx < len(candidates) {
				index = idx
			}
		}
	}

	chosen := candidates[index]
	return chosen.key, chosen.attachment, true
}

type attachmentCandidate struct {
	key        string
	attachment ports.Attachment
	iteration  int
	generated  bool
}

func collectImageAttachmentCandidates(state *TaskState) []attachmentCandidate {
	if state == nil || len(state.Attachments) == 0 {
		return nil
	}
	candidates := make([]attachmentCandidate, 0)
	for key, att := range state.Attachments {
		mediaType := strings.ToLower(strings.TrimSpace(att.MediaType))
		if !strings.HasPrefix(mediaType, "image/") {
			continue
		}
		iter := 0
		if state.AttachmentIterations != nil {
			iter = state.AttachmentIterations[key]
		}
		generated := !strings.EqualFold(strings.TrimSpace(att.Source), "user_upload")
		candidates = append(candidates, attachmentCandidate{
			key:        key,
			attachment: att,
			iteration:  iter,
			generated:  generated,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].generated != candidates[j].generated {
			return candidates[i].generated && !candidates[j].generated
		}
		if candidates[i].iteration == candidates[j].iteration {
			return candidates[i].key < candidates[j].key
		}
		return candidates[i].iteration < candidates[j].iteration
	})

	return candidates
}

func extractPlaceholderName(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 3 {
		return "", false
	}
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return "", false
	}
	name := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if name == "" {
		return "", false
	}
	return name, true
}

var contentPlaceholderPattern = regexp.MustCompile(`\[([^\[\]]+)\]`)

func (e *ReactEngine) resolveStringAttachmentValue(value string, state *TaskState) (string, bool) {
	alias, att, canonical, ok := matchAttachmentReference(value, state)
	if !ok {
		return "", false
	}
	replacement := attachmentReferenceValue(att)
	if replacement == "" {
		return "", false
	}
	if canonical != "" && canonical != alias {
		e.logger.Info("Resolved placeholder [%s] as alias for attachment [%s]", alias, canonical)
	}
	return replacement, true
}

func matchAttachmentReference(raw string, state *TaskState) (string, ports.Attachment, string, bool) {
	if name, ok := extractPlaceholderName(raw); ok {
		att, canonical, _, resolved := lookupAttachmentByNameInternal(name, state)
		if !resolved {
			return "", ports.Attachment{}, "", false
		}
		return name, att, canonical, true
	}

	trimmed := strings.TrimSpace(raw)
	if !looksLikeDirectAttachmentReference(trimmed) {
		return "", ports.Attachment{}, "", false
	}
	att, canonical, _, ok := lookupAttachmentByNameInternal(trimmed, state)
	if !ok {
		return "", ports.Attachment{}, "", false
	}
	return trimmed, att, canonical, true
}

func looksLikeDirectAttachmentReference(value string) bool {
	if value == "" {
		return false
	}
	if strings.ContainsAny(value, "\n\r\t") {
		return false
	}
	if strings.Contains(value, " ") {
		return false
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") {
		return false
	}
	if strings.HasPrefix(lower, "[") && strings.HasSuffix(lower, "]") {
		return false
	}
	return strings.Contains(value, ".")
}

func resolveContentAttachments(content string, state *TaskState) map[string]ports.Attachment {
	if state == nil || len(state.Attachments) == 0 || strings.TrimSpace(content) == "" {
		return nil
	}
	matches := contentPlaceholderPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	resolved := make(map[string]ports.Attachment)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if name == "" {
			continue
		}
		att, _, _, ok := lookupAttachmentByNameInternal(name, state)
		if !ok {
			continue
		}
		if att.Name == "" {
			att.Name = name
		}
		resolved[name] = att
	}
	if len(resolved) == 0 {
		return nil
	}
	return resolved
}

func collectGeneratedAttachments(state *TaskState, iteration int) map[string]ports.Attachment {
	if state == nil || len(state.Attachments) == 0 {
		return nil
	}
	generated := make(map[string]ports.Attachment)
	for key, att := range state.Attachments {
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		if state.AttachmentIterations != nil {
			if iter, ok := state.AttachmentIterations[placeholder]; ok && iter > iteration {
				continue
			}
		}
		if strings.EqualFold(strings.TrimSpace(att.Source), "user_upload") {
			continue
		}
		cloned := att
		if cloned.Name == "" {
			cloned.Name = placeholder
		}
		generated[placeholder] = cloned
	}
	if len(generated) == 0 {
		return nil
	}
	return generated
}

const snapshotSummaryLimit = 160

func snapshotSummaryFromMessages(messages []ports.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		content := normalizeWhitespace(msg.Content)
		if content == "" {
			continue
		}
		prefix := roleSummaryPrefix(msg.Role)
		summary := prefix + content
		return truncateWithEllipsis(summary, snapshotSummaryLimit)
	}
	return ""
}

func normalizeWhitespace(input string) string {
	fields := strings.Fields(input)
	return strings.Join(fields, " ")
}

func roleSummaryPrefix(role string) string {
	trimmed := strings.TrimSpace(role)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "assistant":
		return "Assistant: "
	case "user":
		return "User: "
	case "tool":
		return "Tool: "
	case "system":
		return ""
	default:
		if len(trimmed) == 1 {
			return strings.ToUpper(trimmed) + ": "
		}
		return strings.ToUpper(trimmed[:1]) + strings.ToLower(trimmed[1:]) + ": "
	}
}

func truncateWithEllipsis(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	if limit == 1 {
		return "…"
	}
	trimmed := strings.TrimSpace(string(runes[:limit-1]))
	if trimmed == "" {
		trimmed = string(runes[:limit-1])
	}
	return trimmed + "…"
}

func buildContextTurnRecord(state *ports.TaskState, messages []ports.Message, timestamp time.Time, summary string) ports.ContextTurnRecord {
	record := ports.ContextTurnRecord{
		Timestamp: timestamp,
		Summary:   summary,
		Messages:  append([]ports.Message(nil), messages...),
	}
	if state == nil {
		return record
	}
	record.SessionID = state.SessionID
	record.TurnID = state.Iterations
	record.LLMTurnSeq = state.Iterations
	record.Plans = clonePlanNodes(state.Plans)
	record.Beliefs = cloneBeliefs(state.Beliefs)
	record.KnowledgeRefs = cloneKnowledgeReferences(state.KnowledgeRefs)
	record.World = cloneMapAny(state.WorldState)
	record.Diff = cloneMapAny(state.WorldDiff)
	record.Feedback = cloneFeedbackSignals(state.FeedbackSignals)
	return record
}

func clonePlanNodes(nodes []ports.PlanNode) []ports.PlanNode {
	if len(nodes) == 0 {
		return nil
	}
	cloned := make([]ports.PlanNode, 0, len(nodes))
	for _, node := range nodes {
		copyNode := ports.PlanNode{
			ID:          node.ID,
			Title:       node.Title,
			Status:      node.Status,
			Description: node.Description,
		}
		copyNode.Children = clonePlanNodes(node.Children)
		cloned = append(cloned, copyNode)
	}
	return cloned
}

func cloneBeliefs(beliefs []ports.Belief) []ports.Belief {
	if len(beliefs) == 0 {
		return nil
	}
	cloned := make([]ports.Belief, 0, len(beliefs))
	for _, belief := range beliefs {
		cloned = append(cloned, ports.Belief{
			Statement:  belief.Statement,
			Confidence: belief.Confidence,
			Source:     belief.Source,
		})
	}
	return cloned
}

func cloneKnowledgeReferences(refs []ports.KnowledgeReference) []ports.KnowledgeReference {
	if len(refs) == 0 {
		return nil
	}
	cloned := make([]ports.KnowledgeReference, 0, len(refs))
	for _, ref := range refs {
		copyRef := ports.KnowledgeReference{
			ID:          ref.ID,
			Description: ref.Description,
		}
		copyRef.SOPRefs = append([]string(nil), ref.SOPRefs...)
		copyRef.RAGCollections = append([]string(nil), ref.RAGCollections...)
		copyRef.MemoryKeys = append([]string(nil), ref.MemoryKeys...)
		cloned = append(cloned, copyRef)
	}
	return cloned
}

func cloneFeedbackSignals(signals []ports.FeedbackSignal) []ports.FeedbackSignal {
	if len(signals) == 0 {
		return nil
	}
	cloned := make([]ports.FeedbackSignal, len(signals))
	copy(cloned, signals)
	return cloned
}

func cloneMapAny(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = cloneWorldValue(value)
	}
	return cloned
}

func cloneWorldValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMapAny(v)
	case []map[string]any:
		if len(v) == 0 {
			return nil
		}
		cloned := make([]map[string]any, len(v))
		for i := range v {
			cloned[i] = cloneMapAny(v[i])
		}
		return cloned
	case []string:
		return append([]string(nil), v...)
	case []any:
		if len(v) == 0 {
			return nil
		}
		cloned := make([]any, len(v))
		for i := range v {
			cloned[i] = cloneWorldValue(v[i])
		}
		return cloned
	default:
		return v
	}
}

func (e *ReactEngine) observeToolResults(state *TaskState, iteration int, results []ToolResult) {
	if state == nil || len(results) == 0 {
		return
	}
	ensureWorldStateMap(state)
	updates := make([]map[string]any, 0, len(results))
	for _, result := range results {
		updates = append(updates, summarizeToolResultForWorld(result))
	}
	state.WorldState["last_tool_results"] = updates
	state.WorldState["last_iteration"] = iteration
	state.WorldState["last_updated_at"] = e.clock.Now().Format(time.RFC3339)
	state.WorldDiff = map[string]any{
		"iteration":    iteration,
		"tool_results": updates,
	}
	e.appendFeedbackSignals(state, results)
}

func ensureWorldStateMap(state *TaskState) {
	if state.WorldState == nil {
		state.WorldState = make(map[string]any)
	}
}

func summarizeToolResultForWorld(result ToolResult) map[string]any {
	entry := map[string]any{
		"call_id": strings.TrimSpace(result.CallID),
	}
	status := "success"
	if result.Error != nil {
		status = "error"
		entry["error"] = result.Error.Error()
	}
	entry["status"] = status
	if preview := summarizeForWorld(result.Content, toolResultPreviewRunes); preview != "" {
		entry["output_preview"] = preview
	}
	if metadata := summarizeWorldMetadata(result.Metadata); len(metadata) > 0 {
		entry["metadata"] = metadata
	}
	if names := summarizeAttachmentNames(result.Attachments); len(names) > 0 {
		entry["attachments"] = names
	}
	return entry
}

func summarizeForWorld(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || limit <= 0 {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

func summarizeWorldMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	summarized := make(map[string]any, len(keys))
	for _, key := range keys {
		value := summarizeMetadataValue(metadata[key])
		if value == nil {
			continue
		}
		summarized[key] = value
	}
	if len(summarized) == 0 {
		return nil
	}
	return summarized
}

func summarizeMetadataValue(value any) any {
	switch v := value.(type) {
	case string:
		return summarizeForWorld(v, toolResultPreviewRunes/2)
	case fmt.Stringer:
		return summarizeForWorld(v.String(), toolResultPreviewRunes/2)
	case float64, float32, int, int64, int32, uint64, uint32, bool:
		return v
	case []string:
		copySlice := make([]string, 0, len(v))
		for _, item := range v {
			copySlice = append(copySlice, summarizeForWorld(item, toolResultPreviewRunes/4))
		}
		return copySlice
	case map[string]any:
		return summarizeWorldMetadata(v)
	default:
		if v == nil {
			return nil
		}
		return summarizeForWorld(fmt.Sprintf("%v", v), toolResultPreviewRunes/3)
	}
}

func summarizeAttachmentNames(attachments map[string]ports.Attachment) []string {
	if len(attachments) == 0 {
		return nil
	}
	names := make([]string, 0, len(attachments))
	for key, att := range attachments {
		name := strings.TrimSpace(att.Name)
		if name == "" {
			name = strings.TrimSpace(key)
		}
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	return names
}

func (e *ReactEngine) appendFeedbackSignals(state *TaskState, results []ToolResult) {
	if state == nil || len(results) == 0 {
		return
	}
	now := e.clock.Now()
	for _, result := range results {
		signal := ports.FeedbackSignal{
			Kind:      "tool_result",
			Message:   buildFeedbackMessage(result),
			Value:     deriveFeedbackValue(result),
			CreatedAt: now,
		}
		state.FeedbackSignals = append(state.FeedbackSignals, signal)
	}
	if len(state.FeedbackSignals) > maxFeedbackSignals {
		state.FeedbackSignals = state.FeedbackSignals[len(state.FeedbackSignals)-maxFeedbackSignals:]
	}
}

func deriveFeedbackValue(result ToolResult) float64 {
	if reward, ok := extractRewardValue(result.Metadata); ok {
		return reward
	}
	if result.Error != nil {
		return -1
	}
	return 1
}

func buildFeedbackMessage(result ToolResult) string {
	label := strings.TrimSpace(result.CallID)
	if label == "" {
		label = "tool"
	}
	status := "completed"
	if result.Error != nil {
		status = "errored"
	}
	if preview := summarizeForWorld(result.Content, toolResultPreviewRunes/3); preview != "" {
		return fmt.Sprintf("%s %s: %s", label, status, preview)
	}
	return fmt.Sprintf("%s %s", label, status)
}

func extractRewardValue(metadata map[string]any) (float64, bool) {
	if len(metadata) == 0 {
		return 0, false
	}
	for _, key := range []string{"reward", "score", "value"} {
		raw, ok := metadata[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case float64:
			return v, true
		case float32:
			return float64(v), true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		case int32:
			return float64(v), true
		case uint64:
			return float64(v), true
		case uint32:
			return float64(v), true
		case string:
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func ensureAttachmentPlaceholders(answer string, attachments map[string]ports.Attachment) string {
	normalized := strings.TrimSpace(answer)

	var used map[string]bool
	replaced := contentPlaceholderPattern.ReplaceAllStringFunc(normalized, func(match string) string {
		name := strings.TrimSpace(match[1 : len(match)-1])
		if name == "" {
			return ""
		}
		if len(attachments) == 0 {
			return ""
		}
		att, ok := attachments[name]
		if !ok {
			return ""
		}
		if used == nil {
			used = make(map[string]bool, len(attachments))
		}
		used[name] = true
		return attachmentMarkdown(name, att)
	})

	replaced = strings.TrimSpace(replaced)
	if len(attachments) == 0 {
		return replaced
	}

	var missing []string
	for key := range attachments {
		name := strings.TrimSpace(key)
		if name == "" || (used != nil && used[name]) {
			continue
		}
		missing = append(missing, name)
	}

	if len(missing) == 0 {
		return replaced
	}

	sort.Strings(missing)
	var builder strings.Builder
	if replaced != "" {
		builder.WriteString(replaced)
		builder.WriteString("\n\n")
	}
	for _, name := range missing {
		builder.WriteString(attachmentMarkdown(name, attachments[name]))
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String())
}

func attachmentMarkdown(name string, att ports.Attachment) string {
	display := strings.TrimSpace(att.Description)
	if display == "" {
		display = strings.TrimSpace(att.Name)
	}
	if display == "" {
		display = name
	}

	uri := strings.TrimSpace(att.URI)
	if uri == "" {
		uri = attachmentReferenceValue(att)
	}

	mediaType := strings.ToLower(strings.TrimSpace(att.MediaType))
	if uri == "" {
		return display
	}

	if strings.HasPrefix(mediaType, "image/") || strings.HasPrefix(uri, "data:image") {
		return fmt.Sprintf("![%s](%s)", display, uri)
	}
	return fmt.Sprintf("[%s](%s)", display, uri)
}
