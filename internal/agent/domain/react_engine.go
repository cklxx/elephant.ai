package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	materialapi "alex/internal/materials/api"
	"alex/internal/materials/legacy"
	id "alex/internal/utils/id"
)

// ReactEngine orchestrates the Think-Act-Observe cycle
type ReactEngine struct {
	maxIterations      int
	stopReasons        []string
	logger             ports.Logger
	clock              ports.Clock
	eventListener      EventListener // Optional event listener for TUI
	completion         completionConfig
	attachmentMigrator legacy.Migrator
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
	AttachmentMigrator legacy.Migrator
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
	e.normalizeMessageHistoryAttachments(ctx, state)

	attachmentsChanged := false
	for idx := range state.Messages {
		if registerMessageAttachments(state, state.Messages[idx]) {
			attachmentsChanged = true
		}
	}
	if attachmentsChanged {
		e.updateAttachmentCatalogMessage(state)
	}

	preloadedContext := e.extractPreloadedContextMessages(state)

	e.ensureSystemPromptMessage(state)

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
		userMessage.Attachments = e.normalizeAttachmentsWithMigrator(ctx, state, legacy.MigrationRequest{
			Context:     e.materialRequestContext(state, ""),
			Attachments: userMessage.Attachments,
			Status:      materialapi.MaterialStatusInput,
			Origin:      string(userMessage.Source),
		})
		state.PendingUserAttachments = nil
	}

	state.Messages = append(state.Messages, userMessage)
	if registerMessageAttachments(state, userMessage) {
		e.updateAttachmentCatalogMessage(state)
	}
	if len(userMessage.Attachments) > 0 {
		e.logger.Debug("Registered %d user attachments", len(userMessage.Attachments))
	}
	if len(preloadedContext) > 0 {
		state.Messages = append(state.Messages, preloadedContext...)
	}
	e.logger.Debug("Added user task to messages. Total messages: %d", len(state.Messages))

	for state.Iterations < e.maxIterations {

		if ctx.Err() != nil {
			e.logger.Info("Context cancelled, stopping execution: %v", ctx.Err())
			finalResult := e.finalize(state, "cancelled", e.clock.Now().Sub(startTime))
			attachments := e.decorateFinalResult(state, finalResult)

			e.emitEvent(&TaskCompleteEvent{
				BaseEvent:       e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				FinalAnswer:     finalResult.Answer,
				TotalIterations: finalResult.Iterations,
				TotalTokens:     finalResult.TokensUsed,
				StopReason:      "cancelled",
				Duration:        e.clock.Now().Sub(startTime),
				StreamFinished:  true,
				Attachments:     attachments,
			})

			return nil, ctx.Err()
		}

		state.Iterations++
		e.logger.Info("=== Iteration %d/%d ===", state.Iterations, e.maxIterations)

		e.emitEvent(&IterationStartEvent{
			BaseEvent:  e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
			Iteration:  state.Iterations,
			TotalIters: e.maxIterations,
		})

		e.logger.Debug("THINK phase: Calling LLM with %d messages", len(state.Messages))

		e.emitEvent(&ThinkingEvent{
			BaseEvent:    e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
			Iteration:    state.Iterations,
			MessageCount: len(state.Messages),
		})

		thought, err := e.think(ctx, state, services)
		if err != nil {
			e.logger.Error("Think step failed: %v", err)

			e.emitEvent(&ErrorEvent{
				BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				Iteration:   state.Iterations,
				Phase:       "think",
				Error:       err,
				Recoverable: false,
			})
			return nil, fmt.Errorf("think step failed: %w", err)
		}

		if att := resolveContentAttachments(thought.Content, state); len(att) > 0 {
			thought.Attachments = att
		}
		trimmedThought := strings.TrimSpace(thought.Content)
		if trimmedThought != "" || len(thought.ToolCalls) > 0 {
			state.Messages = append(state.Messages, thought)
		}
		e.logger.Debug("LLM response: content_length=%d, tool_calls=%d",
			len(thought.Content), len(thought.ToolCalls))

		toolCalls := e.parseToolCalls(thought, services.Parser)
		e.logger.Info("Parsed %d tool calls", len(toolCalls))

		if len(toolCalls) == 0 {
			trimmed := strings.TrimSpace(thought.Content)
			if trimmed == "" {
				e.logger.Warn("No tool calls and empty content - continuing loop")
				continue
			}

			e.logger.Info("No tool calls with content - treating response as final answer")
			finalResult := e.finalize(state, "final_answer", e.clock.Now().Sub(startTime))
			return finalResult, nil
		} else {

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

			if strings.Contains(tc.Name, "<|") || strings.Contains(tc.Name, "functions.") || strings.Contains(tc.Name, "user<") {
				e.logger.Warn("Filtering out invalid tool call with leaked markers: %s", tc.Name)
				continue
			}
			validCalls = append(validCalls, tc)
			e.logger.Debug("Tool call: %s (id=%s)", tc.Name, tc.ID)
		}

		if len(validCalls) == 0 {
			e.logger.Warn("All tool calls were invalid, continuing loop")
			continue
		}

		if thought.Content != "" {
			thought.Content = e.cleanToolCallMarkers(thought.Content)
		}

		e.logger.Debug("EXECUTE phase: Running %d tools in parallel", len(validCalls))

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

		for i, r := range results {
			if r.Error != nil {
				e.logger.Warn("Tool %d failed: %v", i, r.Error)
			} else {
				e.logger.Debug("Tool %d succeeded: result_length=%d", i, len(r.Content))
			}
		}

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

		tokenCount := services.Context.EstimateTokens(state.Messages)
		state.TokenCount = tokenCount
		e.logger.Debug("Current token count: %d", tokenCount)

		e.emitEvent(&IterationCompleteEvent{
			BaseEvent:  e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
			Iteration:  state.Iterations,
			TokensUsed: state.TokenCount,
			ToolsRun:   len(results),
		})

		e.logger.Debug("Iteration %d complete, continuing to next iteration", state.Iterations)
	}

	e.logger.Warn("Max iterations (%d) reached, requesting final answer", e.maxIterations)
	finalResult := e.finalize(state, "max_iterations", e.clock.Now().Sub(startTime))

	if finalResult.Answer == "" || len(strings.TrimSpace(finalResult.Answer)) == 0 {
		e.logger.Info("No final answer found, requesting explicit answer")
		state.Messages = append(state.Messages, Message{
			Role:    "user",
			Content: "Please provide your final answer to the user's question now.",
			Source:  ports.MessageSourceSystemPrompt,
		})

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

	return finalResult, nil
}

// think sends current state to LLM for reasoning
func (e *ReactEngine) think(
	ctx context.Context,
	state *TaskState,
	services Services,
) (Message, error) {

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

	e.logger.Debug("Calling LLM (request_id=%s)...", requestID)

	modelName := ""
	if services.LLM != nil {
		modelName = services.LLM.Model()
	}

	resp, err := services.LLM.Complete(ctx, req)

	if err != nil {
		e.logger.Error("LLM call failed (request_id=%s): %v", requestID, err)
		return Message{}, fmt.Errorf("LLM call failed: %w", err)
	}

	e.emitEvent(&AssistantMessageEvent{
		BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
		Iteration:   state.Iterations,
		Delta:       resp.Content,
		Final:       true,
		CreatedAt:   e.clock.Now(),
		SourceModel: modelName,
	})

	e.logger.Debug("LLM response received (request_id=%s): content=%d bytes, tool_calls=%d",
		requestID, len(resp.Content), len(resp.ToolCalls))

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
	subagentSnapshots := make([]*ports.TaskState, len(calls))
	expandedCalls := make([]ToolCall, len(calls))
	for i, call := range calls {
		tc := call
		tc.Arguments = e.expandPlaceholders(tc.Arguments, state)
		expandedCalls[i] = tc
		if tc.Name == "subagent" {
			subagentSnapshots[i] = buildSubagentStateSnapshot(state, tc)
		}
	}
	for i, call := range expandedCalls {
		wg.Add(1)
		go func(idx int, tc ToolCall) {
			defer wg.Done()

			tc.SessionID = state.SessionID
			tc.TaskID = state.TaskID
			tc.ParentTaskID = state.ParentTaskID

			startTime := e.clock.Now()

			e.logger.Debug("Tool %d: Getting tool '%s' from registry", idx, tc.Name)
			tool, err := registry.Get(tc.Name)
			if err != nil {
				e.logger.Error("Tool %d: Tool '%s' not found in registry", idx, tc.Name)

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
			if tc.Name == "subagent" {
				if snapshot := subagentSnapshots[idx]; snapshot != nil {
					toolCtx = ports.WithClonedTaskStateSnapshot(toolCtx, snapshot)
				}
			}

			formattedArgs := formatToolArgumentsForLog(tc.Arguments)
			e.logger.Debug("Tool %d: Executing '%s' with args: %s", idx, tc.Name, formattedArgs)
			result, err := tool.Execute(toolCtx, ports.ToolCall(tc))

			if err != nil {
				e.logger.Error("Tool %d: Execution failed: %v", idx, err)

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

			result.Attachments = e.applyToolAttachmentMutations(
				ctx,
				state,
				tc,
				result.Attachments,
				result.Metadata,
				&attachmentsMu,
			)

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

// parseToolCalls extracts tool calls from assistant message
func (e *ReactEngine) parseToolCalls(msg Message, parser ports.FunctionCallParser) []ToolCall {

	if len(msg.ToolCalls) > 0 {
		e.logger.Debug("Using native tool calls from message: count=%d", len(msg.ToolCalls))
		return msg.ToolCalls
	}

	e.logger.Debug("Parsing tool calls from content: length=%d", len(msg.Content))
	parsed, err := parser.Parse(msg.Content)
	if err != nil {
		e.logger.Warn("Failed to parse tool calls from content: %v", err)
		return nil
	}

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
func (e *ReactEngine) finalize(state *TaskState, stopReason string, duration time.Duration) *TaskResult {

	finalAnswer := strings.TrimSpace(state.FinalAnswer)
	if finalAnswer == "" {
		for i := len(state.Messages) - 1; i >= 0; i-- {
			if state.Messages[i].Role == "assistant" {
				finalAnswer = state.Messages[i].Content
				break
			}
		}
	}

	attachments := resolveContentAttachments(finalAnswer, state)
	finalAnswer = ensureAttachmentPlaceholders(finalAnswer, attachments)

	return &TaskResult{
		Answer:       finalAnswer,
		Messages:     state.Messages,
		Iterations:   state.Iterations,
		TokensUsed:   state.TokenCount,
		StopReason:   stopReason,
		SessionID:    state.SessionID,
		TaskID:       state.TaskID,
		ParentTaskID: state.ParentTaskID,
		Duration:     duration,
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

	for idx := range state.Messages {
		role := strings.ToLower(strings.TrimSpace(state.Messages[idx].Role))
		if role != "system" {
			continue
		}
		source := ports.MessageSource(strings.TrimSpace(string(state.Messages[idx].Source)))
		if source != "" && source != ports.MessageSourceSystemPrompt {

			continue
		}

		if strings.TrimSpace(state.Messages[idx].Content) != prompt {
			state.Messages[idx].Content = state.SystemPrompt
			state.Messages[idx].Source = ports.MessageSourceSystemPrompt
			e.logger.Debug("Updated existing system prompt in message history")
		} else if source == "" {
			state.Messages[idx].Source = ports.MessageSourceSystemPrompt
		}

		existing := state.Messages[idx]
		if idx > 0 {
			state.Messages = append(state.Messages[:idx], state.Messages[idx+1:]...)
			state.Messages = append([]Message{existing}, state.Messages...)
		} else {
			state.Messages[0] = existing
		}
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

func (e *ReactEngine) normalizeAttachmentsWithMigrator(ctx context.Context, state *TaskState, req legacy.MigrationRequest) map[string]ports.Attachment {
	if len(req.Attachments) == 0 || e.attachmentMigrator == nil {
		return req.Attachments
	}
	if req.Context == nil {
		req.Context = e.materialRequestContext(state, "")
	}
	normalized, err := e.attachmentMigrator.Normalize(ctx, req)
	if err != nil {
		e.logger.Warn("attachment migration failed: %v", err)
		return req.Attachments
	}
	return normalized
}

type attachmentMutations struct {
	replace map[string]ports.Attachment
	add     map[string]ports.Attachment
	update  map[string]ports.Attachment
	remove  []string
}

func (e *ReactEngine) applyToolAttachmentMutations(
	ctx context.Context,
	state *TaskState,
	call ToolCall,
	attachments map[string]ports.Attachment,
	metadata map[string]any,
	attachmentsMu *sync.Mutex,
) map[string]ports.Attachment {
	normalized := normalizeAttachmentMap(attachments)
	mutations := normalizeAttachmentMutations(metadata)

	if normalized != nil {
		normalized = e.normalizeAttachmentsWithMigrator(ctx, state, legacy.MigrationRequest{
			Context:     e.materialRequestContext(state, call.ID),
			Attachments: normalized,
			Status:      materialapi.MaterialStatusIntermediate,
			Origin:      call.Name,
		})
	}

	if mutations != nil {
		mutations.replace = e.normalizeAttachmentsWithMigrator(ctx, state, legacy.MigrationRequest{
			Context:     e.materialRequestContext(state, call.ID),
			Attachments: mutations.replace,
			Status:      materialapi.MaterialStatusIntermediate,
			Origin:      call.Name,
		})
		mutations.add = e.normalizeAttachmentsWithMigrator(ctx, state, legacy.MigrationRequest{
			Context:     e.materialRequestContext(state, call.ID),
			Attachments: mutations.add,
			Status:      materialapi.MaterialStatusIntermediate,
			Origin:      call.Name,
		})
		mutations.update = e.normalizeAttachmentsWithMigrator(ctx, state, legacy.MigrationRequest{
			Context:     e.materialRequestContext(state, call.ID),
			Attachments: mutations.update,
			Status:      materialapi.MaterialStatusIntermediate,
			Origin:      call.Name,
		})
	}

	var existing map[string]ports.Attachment
	if state != nil {
		existing = normalizeAttachmentMap(state.Attachments)
	}

	merged := mergeAttachmentMutations(normalized, mutations, existing)
	if attachmentsMu != nil {
		attachmentsMu.Lock()
		defer attachmentsMu.Unlock()
	}
	applyAttachmentMutationsToState(state, merged, mutations, call.Name)
	return merged
}

func (e *ReactEngine) normalizeMessageHistoryAttachments(ctx context.Context, state *TaskState) {
	if state == nil {
		return
	}
	for idx := range state.Messages {
		msg := state.Messages[idx]
		if len(msg.Attachments) == 0 {
			continue
		}
		normalized := e.normalizeAttachmentsWithMigrator(ctx, state, legacy.MigrationRequest{
			Context:     e.materialRequestContext(state, msg.ToolCallID),
			Attachments: msg.Attachments,
			Status:      messageMaterialStatus(msg),
			Origin:      messageMaterialOrigin(msg),
		})
		if normalized != nil {
			state.Messages[idx].Attachments = normalized
		}
	}
}

func (e *ReactEngine) materialRequestContext(state *TaskState, toolCallID string) *materialapi.RequestContext {
	if state == nil {
		return nil
	}
	return &materialapi.RequestContext{
		RequestID:      state.TaskID,
		TaskID:         state.TaskID,
		AgentIteration: uint32(state.Iterations),
		ToolCallID:     toolCallID,
		ConversationID: state.SessionID,
		UserID:         state.SessionID,
	}
}

func (e *ReactEngine) extractPreloadedContextMessages(state *TaskState) []Message {
	if state == nil || len(state.Messages) == 0 {
		return nil
	}

	idx := len(state.Messages)
	for idx > 0 {
		if !isCurrentPreloadedContextMessage(state.Messages[idx-1], state.TaskID) {
			break
		}
		idx--
	}
	if idx == len(state.Messages) {
		return nil
	}

	preloaded := make([]Message, len(state.Messages)-idx)
	copy(preloaded, state.Messages[idx:])
	state.Messages = state.Messages[:idx]
	return preloaded
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

type attachmentCandidate struct {
	key        string
	attachment ports.Attachment
	iteration  int
	generated  bool
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

const snapshotSummaryLimit = 160

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
