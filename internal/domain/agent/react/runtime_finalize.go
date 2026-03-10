package react

import (
	"strings"

	domain "alex/internal/domain/agent"
	"alex/internal/shared/utils"
)

func (r *reactRuntime) finalizeResult(stopReason string, result *TaskResult, emitCompletionEvent bool, workflowErr error) *TaskResult {
	r.resultOnce.Do(func() {
		if result == nil {
			result = r.engine.finalize(r.state, stopReason, r.engine.clock.Now().Sub(r.startTime))
		} else {
			result.StopReason = stopReason
			if result.Duration == 0 {
				result.Duration = r.engine.clock.Now().Sub(r.startTime)
			}
		}

		// Log precise LLM token breakdown at task completion.
		tb := result.TokenBreakdown
		r.engine.logger.Info(
			"Token breakdown: think=%d act=%d observe=%d total=%d (prompt=%d completion=%d llm_calls=%d)",
			tb.ThinkPromptTokens+tb.ThinkCompletionTokens,
			tb.ActPromptTokens+tb.ActCompletionTokens,
			tb.ObservePromptTokens+tb.ObserveCompletionTokens,
			tb.TotalTokens,
			tb.TotalPromptTokens,
			tb.TotalCompletionTokens,
			tb.LLMCalls,
		)

		attachments := r.engine.decorateFinalResult(r.state, result)
		if emitCompletionEvent {
			r.emitFinalAnswerStream(stopReason, result)
			r.engine.emitEvent(domain.NewResultFinalEvent(
				r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
				result.Answer, result.Iterations, result.TokensUsed, stopReason,
				result.Duration, false, true, attachments,
			))
		}

		r.engine.clearCheckpoint(r.ctx, r.state.SessionID)
		r.finishWorkflow(stopReason, result, workflowErr)
	})

	return result
}

func (r *reactRuntime) emitFinalAnswerStream(stopReason string, result *TaskResult) {
	if result == nil {
		return
	}
	answer := result.Answer
	if utils.IsBlank(answer) {
		return
	}

	const chunkSize = 800
	runes := []rune(answer)
	if len(runes) == 0 {
		return
	}

	// Emit cumulative chunks up to (but not including) the full length.
	// The caller emits the final complete ResultFinalEvent with streamFinished=true.
	for end := chunkSize; end < len(runes); end += chunkSize {
		r.engine.emitEvent(domain.NewResultFinalEvent(
			r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
			string(runes[:end]), result.Iterations, result.TokensUsed, stopReason,
			result.Duration, true, false, nil,
		))
	}
}

func (r *reactRuntime) maybeStopAfterRepeatedToolFailures(results []ToolResult) (*TaskResult, bool) {
	if len(results) == 0 {
		return nil, false
	}

	hasSuccess := false
	matchedNonRetryable := false

	for i := range results {
		res := results[i]
		if res.Error == nil {
			hasSuccess = true
			continue
		}

		failure, ok := classifyNonRetryableToolFailure(res.Error)
		if !ok {
			continue
		}
		matchedNonRetryable = true

		if failure.signature == r.lastNonRetryableToolFailure {
			r.consecutiveNonRetryableFails++
		} else {
			r.lastNonRetryableToolFailure = failure.signature
			r.consecutiveNonRetryableFails = 1
		}

		if r.consecutiveNonRetryableFails >= repeatedNonRetryableToolFailureThreshold {
			r.engine.logger.Warn(
				"Stopping after repeated non-recoverable tool failure: signature=%s count=%d",
				failure.signature,
				r.consecutiveNonRetryableFails,
			)
			return r.finalizeRepeatedToolFailure(failure.hint, res.Error), true
		}
	}

	if hasSuccess || !matchedNonRetryable {
		r.resetNonRetryableToolFailures()
	}

	return nil, false
}

func (r *reactRuntime) finalizeRepeatedToolFailure(hint string, lastErr error) *TaskResult {
	result := r.engine.finalize(r.state, "repeated_tool_failure", r.engine.clock.Now().Sub(r.startTime))

	var summary strings.Builder
	summary.WriteString("Stopped after repeated non-recoverable tool errors to avoid retry loops.")
	if trimmed := strings.TrimSpace(hint); trimmed != "" {
		summary.WriteString("\n")
		summary.WriteString(trimmed)
	}
	if lastErr != nil {
		summary.WriteString("\nLast error: ")
		summary.WriteString(strings.TrimSpace(lastErr.Error()))
	}
	result.Answer = strings.TrimSpace(summary.String())

	r.resetNonRetryableToolFailures()
	return r.finalizeResult("repeated_tool_failure", result, true, nil)
}

func (r *reactRuntime) resetNonRetryableToolFailures() {
	r.lastNonRetryableToolFailure = ""
	r.consecutiveNonRetryableFails = 0
}

type nonRetryableToolFailure struct {
	signature string
	hint      string
}

func classifyNonRetryableToolFailure(err error) (nonRetryableToolFailure, bool) {
	if err == nil {
		return nonRetryableToolFailure{}, false
	}

	text := utils.TrimLower(err.Error())
	if text == "" {
		return nonRetryableToolFailure{}, false
	}

	switch {
	case strings.Contains(text, "path must stay within the working directory"):
		return nonRetryableToolFailure{
			signature: "path_guard",
			hint:      "Use relative paths or set exec_dir under the current working directory.",
		}, true
	case strings.Contains(text, "template \"") && strings.Contains(text, "not found"):
		return nonRetryableToolFailure{
			signature: "template_not_found",
			hint:      "Run `alex team run --template list` first, then choose one of the listed templates and call `alex team run --template <name> --goal \"...\"`.",
		}, true
	default:
		return nonRetryableToolFailure{}, false
	}
}

func extractLLMMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]any)
	for _, key := range []string{"llm_duration_ms", "llm_request_id", "llm_model"} {
		if val, ok := metadata[key]; ok {
			out[key] = val
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (r *reactRuntime) applyIterationHook(iteration int) {
	if r.engine.iterationHook == nil || iteration == 0 {
		return
	}
	result := r.engine.iterationHook.OnIteration(r.ctx, r.state, iteration)
	if result.MemoriesInjected <= 0 {
		return
	}
	r.engine.emitEvent(domain.NewProactiveContextRefreshEvent(
		r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
		iteration, result.MemoriesInjected,
	))
}

// persistSessionAfterIteration calls the optional sessionPersister callback
// to asynchronously persist session state after each successful iteration.
// The callback is responsible for async behavior and error handling.
func (r *reactRuntime) persistSessionAfterIteration() {
	if r.engine.sessionPersister != nil {
		r.engine.sessionPersister(r.ctx, nil, r.state)
	}
}

// isContextLengthExceeded checks whether the error indicates the LLM provider
// rejected the request because the input exceeded the model's context window.
// Maintained as a compatibility wrapper for existing tests.
func isContextLengthExceeded(err error) bool {
	return classifyContextOverflow(err).Matched
}

// emergencyTrimState applies aggressive trimming to state.Messages when the
// LLM rejects the request due to context length. This is the last-resort
// safety net after pre-flight enforcement has already been attempted.
func emergencyTrimState(state *TaskState, services Services) {
	trimmed := aggressiveTrimMessages(state.Messages, 2)
	state.Messages = trimmed
}
