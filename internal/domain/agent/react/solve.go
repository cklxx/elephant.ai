package react

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	tokenutil "alex/internal/shared/token"

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
		filteredMessages = e.enforceContextBudget(ctx, filteredMessages, state, services)
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

// delayedSummaryTurns is how many additional ReAct iterations to wait before
// applying a pending compression summary.
const delayedSummaryTurns = 2

// enforceContextBudget implements a two-phase compression strategy:
//
// Phase A (proactive): when token usage reaches the compression threshold (~70%)
// but is still under the hard limit, generate a summary but do NOT replace yet.
// Store it as PendingSummary on the TaskState.
//
// Phase B (deferred apply): after 2 more iterations, apply the pending summary
// by replacing the messages that existed when the summary was generated.
//
// Phase C (safety net): if tokens actually exceed the hard limit, fall through
// to artifact compaction, immediate AutoCompact, aggressive trim, and force-fit.
func (e *ReactEngine) enforceContextBudget(
	ctx context.Context,
	messages []ports.Message,
	state *TaskState,
	services Services,
) []ports.Message {
	limit := e.resolveContextTokenLimit(services)
	if limit <= 0 {
		return messages
	}
	estimated := services.Context.EstimateTokens(messages)

	// Phase B: Apply a pending summary if enough turns have elapsed.
	if state.PendingSummary != "" && state.Iterations >= state.PendingSummaryAtIter+delayedSummaryTurns {
		applied := applyPendingSummary(messages, state)
		afterApply := services.Context.EstimateTokens(applied)
		e.logger.Info(
			"Deferred summary applied: %d \u2192 %d tokens (generated at iter %d, now iter %d)",
			estimated, afterApply, state.PendingSummaryAtIter, state.Iterations,
		)
		state.Messages = rebuildStateMessages(state.Messages, applied)
		clearPendingSummary(state)
		if afterApply <= limit {
			return applied
		}
		messages = applied
		estimated = afterApply
	}

	// Phase A: Proactively generate a summary when approaching the threshold,
	// but only if we have no pending summary and we are still under the hard limit.
	if estimated <= limit && state.PendingSummary == "" {
		if services.Context.ShouldCompress(messages, limit) {
			summary, msgCount := services.Context.BuildSummaryOnly(messages)
			if summary != "" && msgCount > 0 {
				state.PendingSummary = summary
				state.PendingSummaryAtIter = state.Iterations
				state.PendingSummaryMsgCount = len(messages)
				e.logger.Info(
					"Deferred summary generated: iter=%d msgs=%d compressible=%d ratio=%.2f",
					state.Iterations, len(messages), msgCount,
					float64(estimated)/float64(limit),
				)
			}
		}
		return messages
	}

	if estimated <= limit {
		return messages
	}

	// --- Safety net: tokens exceed the hard limit. ---
	e.logger.Warn("Context budget exceeded: estimated=%d limit=%d messages=%d \u2014 applying safety-net compression",
		estimated, limit, len(messages))

	// If we have a pending summary, apply it immediately as first resort.
	if state.PendingSummary != "" {
		applied := applyPendingSummary(messages, state)
		afterApply := services.Context.EstimateTokens(applied)
		e.logger.Info("Pending summary applied early (budget exceeded): %d \u2192 %d tokens", estimated, afterApply)
		state.Messages = rebuildStateMessages(state.Messages, applied)
		clearPendingSummary(state)
		if afterApply <= limit {
			return applied
		}
		messages = applied
		estimated = afterApply
	}

	// Layer 1a: Try artifact compaction (Manus-style placeholder + file).
	inCooldown := isCompactionInCooldown(state)
	if inCooldown {
		if state != nil {
			e.logger.Debug(
				"Context artifact compaction cooling down: iteration=%d next_allowed=%d",
				state.Iterations,
				state.NextCompactionAllowed,
			)
		}
	} else {
		compacted, ok := e.tryArtifactCompaction(ctx, state, services, messages, compactionReasonThreshold, false)
		if ok {
			afterCompact := services.Context.EstimateTokens(compacted)
			e.logger.Info("Artifact compaction reduced tokens: %d \u2192 %d (limit=%d)", estimated, afterCompact, limit)
			if afterCompact <= limit {
				state.Messages = rebuildStateMessages(state.Messages, compacted)
				return compacted
			}
			messages = compacted
			estimated = afterCompact
		}
	}

	// Layer 1b: Immediate AutoCompact (skip during cooldown).
	if !inCooldown {
		compacted, ok := services.Context.AutoCompact(messages, limit)
		if ok {
			afterCompact := services.Context.EstimateTokens(compacted)
			e.logger.Info("Auto-compact reduced tokens: %d \u2192 %d (limit=%d)", estimated, afterCompact, limit)
			if afterCompact <= limit {
				state.Messages = rebuildStateMessages(state.Messages, compacted)
				return compacted
			}
			messages = compacted
			estimated = afterCompact
		}
	}

	// Layer 2: Aggressive trim \u2014 keep system/important + last N turns.
	for turns := 4; turns >= 1; turns-- {
		trimmed := aggressiveTrimMessages(messages, turns)
		afterTrim := services.Context.EstimateTokens(trimmed)
		e.logger.Info("Aggressive trim (turns=%d): %d \u2192 %d tokens (limit=%d)",
			turns, estimated, afterTrim, limit)
		if afterTrim <= limit {
			state.Messages = rebuildStateMessages(state.Messages, trimmed)
			return trimmed
		}
	}

	// Last resort: keep only system/important messages + the very last message.
	e.logger.Warn("All trimming strategies insufficient \u2014 keeping only preserved messages + last message")
	trimmed := aggressiveTrimMessages(messages, 1)
	afterTrim := services.Context.EstimateTokens(trimmed)
	if afterTrim > limit {
		forceFitted := forceFitMessagesToLimit(trimmed, limit, services.Context.EstimateTokens)
		afterForceFit := services.Context.EstimateTokens(forceFitted)
		e.logger.Warn("Hard context clamp applied: %d \u2192 %d tokens (limit=%d)", afterTrim, afterForceFit, limit)
		trimmed = forceFitted
	}
	state.Messages = rebuildStateMessages(state.Messages, trimmed)
	return trimmed
}

// applyPendingSummary replaces older conversation messages with the pre-generated
// summary. It preserves system/important/checkpoint messages and the messages
// added after the summary was generated.
func applyPendingSummary(messages []ports.Message, state *TaskState) []ports.Message {
	if state.PendingSummary == "" {
		return messages
	}

	// Separate preserved messages from conversation.
	var preserved []ports.Message
	var conversation []ports.Message
	for _, msg := range messages {
		switch msg.Source {
		case ports.MessageSourceSystemPrompt, ports.MessageSourceImportant, ports.MessageSourceCheckpoint:
			preserved = append(preserved, msg)
		default:
			conversation = append(conversation, msg)
		}
	}

	if len(conversation) == 0 {
		return messages
	}

	// Determine how many conversation messages existed at summary generation time.
	preservedAtGen := len(messages) - len(conversation)
	convCountAtGen := state.PendingSummaryMsgCount - preservedAtGen
	if convCountAtGen <= 0 {
		convCountAtGen = len(conversation) - 1
	}
	if convCountAtGen > len(conversation) {
		convCountAtGen = len(conversation)
	}

	// Keep at least the most recent turn from the old conversation.
	kept := keepRecentTurnsLocal(conversation[:convCountAtGen], 1)
	replaceCount := convCountAtGen - len(kept)
	if replaceCount <= 0 {
		return messages
	}

	summaryMsg := ports.Message{
		Role:    "assistant",
		Content: state.PendingSummary,
		Source:  ports.MessageSourceUserHistory,
	}

	result := make([]ports.Message, 0, len(preserved)+1+len(conversation)-replaceCount)
	result = append(result, preserved...)
	result = append(result, summaryMsg)
	result = append(result, conversation[replaceCount:]...)
	return result
}

// clearPendingSummary resets the deferred summary fields on the state.
func clearPendingSummary(state *TaskState) {
	state.PendingSummary = ""
	state.PendingSummaryAtIter = 0
	state.PendingSummaryMsgCount = 0
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
	systemPromptKept := false
	for _, msg := range messages {
		switch {
		case msg.Source == ports.MessageSourceImportant, msg.Source == ports.MessageSourceCheckpoint:
			preserved = append(preserved, msg)
		case msg.Source == ports.MessageSourceSystemPrompt:
			if !systemPromptKept && isPrimarySystemPromptForTrim(msg) {
				preserved = append(preserved, msg)
				systemPromptKept = true
				continue
			}
			conversation = append(conversation, msg)
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

func isPrimarySystemPromptForTrim(msg ports.Message) bool {
	role := strings.ToLower(strings.TrimSpace(msg.Role))
	return role == "system" && strings.TrimSpace(msg.Content) != ""
}

func forceFitMessagesToLimit(messages []ports.Message, limit int, estimate func([]ports.Message) int) []ports.Message {
	if len(messages) == 0 || limit <= 0 || estimate == nil {
		return messages
	}

	fitted := append([]ports.Message(nil), messages...)
	if estimate(fitted) <= limit {
		return fitted
	}

	// Phase 1: iteratively halve the largest message until we fit or run out.
	for attempt := 0; attempt < 24 && estimate(fitted) > limit; attempt++ {
		idx := indexOfLargestMessageContent(fitted)
		if idx < 0 {
			break
		}
		content := strings.TrimSpace(fitted[idx].Content)
		if content == "" {
			fitted[idx].Content = "[context truncated]"
			continue
		}
		currentTokens := tokenutil.CountTokens(content)
		if currentTokens <= 64 {
			fitted[idx].Content = "[context truncated]"
			continue
		}
		fitted[idx].Content = tokenutil.TruncateToTokens(content, currentTokens/2)
	}
	if estimate(fitted) <= limit {
		return fitted
	}

	// Phase 2: deterministic minimal payload (canonical system + latest message).
	systemIdx := -1
	for i, msg := range messages {
		if msg.Source == ports.MessageSourceSystemPrompt {
			systemIdx = i
			break
		}
	}
	if systemIdx < 0 {
		systemIdx = 0
	}
	lastIdx := len(messages) - 1

	minimal := make([]ports.Message, 0, 3)
	sys := messages[systemIdx]
	sysContent := strings.TrimSpace(sys.Content)
	if sysContent != "" {
		sys.Content = tokenutil.TruncateToTokens(sysContent, 512)
	}
	minimal = append(minimal, sys)

	if lastIdx != systemIdx {
		last := messages[lastIdx]
		lastContent := strings.TrimSpace(last.Content)
		if lastContent != "" {
			last.Content = tokenutil.TruncateToTokens(lastContent, 256)
		}
		minimal = append(minimal, ports.Message{
			Role:    "assistant",
			Content: "[Additional context truncated to satisfy model window.]",
			Source:  ports.MessageSourceUserHistory,
		})
		minimal = append(minimal, last)
	}

	// Final nudge if still above limit: repeatedly halve largest content.
	for attempt := 0; attempt < 12 && estimate(minimal) > limit; attempt++ {
		idx := indexOfLargestMessageContent(minimal)
		if idx < 0 {
			break
		}
		content := strings.TrimSpace(minimal[idx].Content)
		if content == "" {
			minimal[idx].Content = "[context truncated]"
			continue
		}
		tokens := tokenutil.CountTokens(content)
		if tokens <= 32 {
			minimal[idx].Content = "[context truncated]"
			continue
		}
		minimal[idx].Content = tokenutil.TruncateToTokens(content, tokens/2)
	}

	return minimal
}

func indexOfLargestMessageContent(messages []ports.Message) int {
	longestIdx := -1
	longest := 0
	for i, msg := range messages {
		length := len([]rune(strings.TrimSpace(msg.Content)))
		if length > longest {
			longest = length
			longestIdx = i
		}
	}
	return longestIdx
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
