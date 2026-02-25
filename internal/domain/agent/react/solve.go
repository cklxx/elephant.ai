package react

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"

	"go.opentelemetry.io/otel/attribute"
)

// SolveTask is the main ReAct loop - pure business logic
func (e *ReactEngine) SolveTask(
	ctx context.Context,
	task string,
	state *TaskState,
	services Services,
) (*TaskResult, error) {
	runtime := newReactRuntime(e, ctx, task, state, services, func() {
		e.prepareUserTaskContext(ctx, task, state)
	})
	return runtime.run()
}

// RecordUserInput appends the user input to the task state without running ReAct.
func (e *ReactEngine) RecordUserInput(ctx context.Context, task string, state *TaskState) {
	if state == nil || strings.TrimSpace(task) == "" {
		return
	}
	e.prepareUserTaskContext(ctx, task, state)
}

// think sends current state to LLM for reasoning
func (e *ReactEngine) think(
	ctx context.Context,
	state *TaskState,
	services Services,
) (Message, error) {

	tools := services.ToolExecutor.List()
	requestID := e.idGenerator.NewRequestIDWithLogID(e.idContextReader.LogIDFromContext(ctx))
	normalizeContextMessages(state)
	filteredMessages, excluded := splitMessagesForLLM(state.Messages)

	// Pre-flight context budget enforcement: estimate full token count and
	// trim messages before sending to prevent context_length_exceeded errors.
	if services.Context != nil {
		filteredMessages = e.enforceContextBudget(filteredMessages, state, services)
	}

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
		Thinking: ports.ThinkingConfig{
			Enabled: true,
		},
		Metadata: map[string]any{
			"request_id": requestID,
		},
	}

	if len(e.completion.stopSequences) > 0 {
		req.StopSequences = append([]string(nil), e.completion.stopSequences...)
	}

	timestamp := e.clock.Now()
	snapshot := domain.NewDiagnosticContextSnapshotEvent(
		e.getAgentLevel(ctx),
		state.SessionID,
		state.RunID,
		state.ParentRunID,
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
	if modelName == "" {
		modelName = "unknown"
	}

	ctx, llmSpan := startReactSpan(
		ctx,
		traceSpanLLMGenerate,
		state,
		attribute.Int(traceAttrIteration, state.Iterations),
		attribute.String(traceAttrModel, modelName),
		attribute.String("alex.llm.request_id", requestID),
		attribute.Int("alex.llm.filtered_messages", len(filteredMessages)),
		attribute.Int("alex.llm.tools", len(tools)),
		attribute.Bool("alex.llm.streaming", true),
	)
	var llmErr error
	defer func() {
		markSpanResult(llmSpan, llmErr)
		llmSpan.End()
	}()

	llmCallStarted := time.Now()
	const streamChunkMinChars = 64
	var streamBuffer strings.Builder
	streamedContent := false

	callbacks := ports.CompletionStreamCallbacks{
		OnContentDelta: func(delta ports.ContentDelta) {
			if delta.Delta != "" {
				streamedContent = true
				streamBuffer.WriteString(delta.Delta)
				if streamBuffer.Len() >= streamChunkMinChars {
					chunk := streamBuffer.String()
					streamBuffer.Reset()
					if chunk != "" {
						e.emitEvent(domain.NewNodeOutputDeltaEvent(
							e.newBaseEvent(ctx, state.SessionID, state.RunID, state.ParentRunID),
							state.Iterations, 0, chunk, false, e.clock.Now(), modelName,
						))
					}
				}
			}
			if delta.Final {
				if streamBuffer.Len() > 0 {
					chunk := streamBuffer.String()
					streamBuffer.Reset()
					if chunk != "" {
						e.emitEvent(domain.NewNodeOutputDeltaEvent(
							e.newBaseEvent(ctx, state.SessionID, state.RunID, state.ParentRunID),
							state.Iterations, 0, chunk, false, e.clock.Now(), modelName,
						))
					}
				}
			}
		},
	}
	resp, err := services.LLM.StreamComplete(ctx, req, callbacks)
	llmDuration := time.Since(llmCallStarted)
	llmSpan.SetAttributes(attribute.Int64("alex.llm.duration_ms", llmDuration.Milliseconds()))
	e.latencyReporter(ctx,
		"[latency] llm_complete_ms=%.2f iteration=%d model=%s request_id=%s\n",
		float64(llmDuration)/float64(time.Millisecond),
		state.Iterations,
		strings.TrimSpace(modelName),
		requestID,
	)

	if err != nil {
		llmErr = err
		e.logger.Error("LLM call failed (request_id=%s): %v", requestID, err)
		return Message{}, fmt.Errorf("LLM call failed: %w", err)
	}
	if resp == nil {
		llmErr = fmt.Errorf("LLM call failed: nil response")
		e.logger.Error("LLM call returned nil response (request_id=%s)", requestID)
		return Message{}, fmt.Errorf("LLM call failed: nil response")
	}

	llmSpan.SetAttributes(
		attribute.Int("alex.llm.input_tokens", resp.Usage.PromptTokens),
		attribute.Int("alex.llm.output_tokens", resp.Usage.CompletionTokens),
		attribute.Int("alex.llm.token_count", resp.Usage.TotalTokens),
		attribute.String("alex.llm.stop_reason", strings.TrimSpace(resp.StopReason)),
	)

	if streamBuffer.Len() > 0 {
		chunk := streamBuffer.String()
		streamBuffer.Reset()
		if chunk != "" {
			streamedContent = true
			e.emitEvent(domain.NewNodeOutputDeltaEvent(
				e.newBaseEvent(ctx, state.SessionID, state.RunID, state.ParentRunID),
				state.Iterations, 0, chunk, false, e.clock.Now(), modelName,
			))
		}
	}

	finalDelta := ""
	if !streamedContent {
		finalDelta = resp.Content
	}
	e.emitEvent(domain.NewNodeOutputDeltaEvent(
		e.newBaseEvent(ctx, state.SessionID, state.RunID, state.ParentRunID),
		state.Iterations, 0, finalDelta, true, e.clock.Now(), modelName,
	))

	e.logger.Debug("LLM response received (request_id=%s): content=%d bytes, tool_calls=%d",
		requestID, len(resp.Content), len(resp.ToolCalls))

	meta := map[string]any{}
	if llmDuration > 0 {
		meta["llm_duration_ms"] = llmDuration.Milliseconds()
	}
	if requestID != "" {
		meta["llm_request_id"] = requestID
	}
	if modelName != "" {
		meta["llm_model"] = modelName
	}
	if len(meta) == 0 {
		meta = nil
	}

	return Message{
		Role:      "assistant",
		Content:   resp.Content,
		Thinking:  resp.Thinking,
		ToolCalls: resp.ToolCalls,
		Metadata:  meta,
		Source:    ports.MessageSourceAssistantReply,
	}, nil
}

// enforceContextBudget trims messages when estimated tokens exceed the context
// budget. It first tries AutoCompact (summarize older messages), then falls
// back to aggressive trimming (keep only recent turns).
func (e *ReactEngine) enforceContextBudget(
	messages []ports.Message,
	state *TaskState,
	services Services,
) []ports.Message {
	limit := e.resolveContextTokenLimit(services)
	if limit <= 0 {
		return messages
	}
	estimated := services.Context.EstimateTokens(messages)
	if estimated <= limit {
		return messages
	}

	e.logger.Warn("Context budget exceeded: estimated=%d limit=%d messages=%d — applying auto-compact",
		estimated, limit, len(messages))

	// Layer 1: Try AutoCompact (summarize older messages).
	compacted, ok := services.Context.AutoCompact(messages, limit)
	if ok {
		afterCompact := services.Context.EstimateTokens(compacted)
		e.logger.Info("Auto-compact reduced tokens: %d → %d (limit=%d)", estimated, afterCompact, limit)
		if afterCompact <= limit {
			// Also update state.Messages so subsequent iterations benefit.
			state.Messages = rebuildStateMessages(state.Messages, compacted)
			return compacted
		}
		messages = compacted
		estimated = afterCompact
	}

	// Layer 2: Aggressive trim — keep system/important + last N turns.
	// Start with 4 turns, reduce until under budget.
	for turns := 4; turns >= 1; turns-- {
		trimmed := aggressiveTrimMessages(messages, turns)
		afterTrim := services.Context.EstimateTokens(trimmed)
		e.logger.Info("Aggressive trim (turns=%d): %d → %d tokens (limit=%d)",
			turns, estimated, afterTrim, limit)
		if afterTrim <= limit {
			state.Messages = rebuildStateMessages(state.Messages, trimmed)
			return trimmed
		}
	}

	// Last resort: keep only system/important messages + the very last message.
	e.logger.Warn("All trimming strategies insufficient — keeping only preserved messages + last message")
	trimmed := aggressiveTrimMessages(messages, 1)
	state.Messages = rebuildStateMessages(state.Messages, trimmed)
	return trimmed
}

// aggressiveTrimMessages keeps system/important/checkpoint messages and the
// last N user-initiated turns, inserting a compression summary for removed
// messages. This is a package-internal wrapper around the same logic used by
// the context package's AggressiveTrim.
func aggressiveTrimMessages(messages []ports.Message, maxTurns int) []ports.Message {
	if maxTurns <= 0 {
		maxTurns = 1
	}

	var preserved, conversation []ports.Message
	for _, msg := range messages {
		switch msg.Source {
		case ports.MessageSourceSystemPrompt, ports.MessageSourceImportant, ports.MessageSourceCheckpoint:
			preserved = append(preserved, msg)
		default:
			conversation = append(conversation, msg)
		}
	}

	kept := keepRecentTurnsLocal(conversation, maxTurns)

	result := make([]ports.Message, 0, len(preserved)+len(kept)+1)
	result = append(result, preserved...)
	if len(kept) > 0 && len(conversation) > len(kept) {
		result = append(result, ports.Message{
			Role:    "assistant",
			Content: "[Context trimmed to fit model window. Earlier conversation was removed.]",
			Source:  ports.MessageSourceUserHistory,
		})
	}
	result = append(result, kept...)
	return result
}

// keepRecentTurnsLocal mirrors context.keepRecentTurns for use within react.
func keepRecentTurnsLocal(messages []ports.Message, maxTurns int) []ports.Message {
	if len(messages) == 0 || maxTurns <= 0 {
		return nil
	}
	var turnStarts []int
	for i, msg := range messages {
		if strings.EqualFold(strings.TrimSpace(msg.Role), "user") {
			turnStarts = append(turnStarts, i)
		}
	}
	if len(turnStarts) == 0 {
		if len(messages) <= maxTurns {
			return messages
		}
		return messages[len(messages)-maxTurns:]
	}
	start := 0
	if len(turnStarts) > maxTurns {
		start = turnStarts[len(turnStarts)-maxTurns]
	} else {
		start = turnStarts[0]
	}
	return messages[start:]
}

// rebuildStateMessages replaces non-debug/non-eval messages in the original
// state with the trimmed set. This is a best-effort update — if the trimmed
// set is a subset, we just use it directly.
func rebuildStateMessages(original, trimmed []ports.Message) []ports.Message {
	// Simple approach: use the trimmed messages as the new state.
	// The debug/eval messages were already excluded by splitMessagesForLLM.
	return trimmed
}
