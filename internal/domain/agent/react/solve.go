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
