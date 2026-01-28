package react

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/utils/clilatency"
	id "alex/internal/utils/id"
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
	requestID := id.NewRequestIDWithLogID(id.LogIDFromContext(ctx))
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
	snapshot := domain.NewWorkflowDiagnosticContextSnapshotEvent(
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

	llmCallStarted := time.Now()
	const streamChunkMinChars = 1
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
					e.emitEvent(&domain.WorkflowNodeOutputDeltaEvent{
						BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
						Iteration:   state.Iterations,
						Delta:       chunk,
						Final:       false,
						CreatedAt:   e.clock.Now(),
						SourceModel: modelName,
					})
				}
			}
			if delta.Final {
				if streamBuffer.Len() > 0 {
					chunk := streamBuffer.String()
					streamBuffer.Reset()
					e.emitEvent(&domain.WorkflowNodeOutputDeltaEvent{
						BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
						Iteration:   state.Iterations,
						Delta:       chunk,
						Final:       false,
						CreatedAt:   e.clock.Now(),
						SourceModel: modelName,
					})
				}
			}
		},
	}
	resp, err := services.LLM.StreamComplete(ctx, req, callbacks)
	llmDuration := time.Since(llmCallStarted)
	clilatency.PrintfWithContext(ctx,
		"[latency] llm_complete_ms=%.2f iteration=%d model=%s request_id=%s\n",
		float64(llmDuration)/float64(time.Millisecond),
		state.Iterations,
		strings.TrimSpace(modelName),
		requestID,
	)

	if err != nil {
		e.logger.Error("LLM call failed (request_id=%s): %v", requestID, err)
		return Message{}, fmt.Errorf("LLM call failed: %w", err)
	}
	if resp == nil {
		e.logger.Error("LLM call returned nil response (request_id=%s)", requestID)
		return Message{}, fmt.Errorf("LLM call failed: nil response")
	}

	if streamBuffer.Len() > 0 {
		chunk := streamBuffer.String()
		streamBuffer.Reset()
		if chunk != "" {
			streamedContent = true
			e.emitEvent(&domain.WorkflowNodeOutputDeltaEvent{
				BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				Iteration:   state.Iterations,
				Delta:       chunk,
				Final:       false,
				CreatedAt:   e.clock.Now(),
				SourceModel: modelName,
			})
		}
	}

	finalDelta := ""
	if !streamedContent {
		finalDelta = resp.Content
	}
	e.emitEvent(&domain.WorkflowNodeOutputDeltaEvent{
		BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
		Iteration:   state.Iterations,
		Delta:       finalDelta,
		Final:       true,
		CreatedAt:   e.clock.Now(),
		SourceModel: modelName,
	})

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
