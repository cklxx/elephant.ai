package react

import (
	"context"

	"alex/internal/domain/agent/ports"
)

// delayedSummaryTurns is how many additional ReAct iterations to wait before
// applying a pending compression summary.
const delayedSummaryTurns = 2

// enforceContextBudgetWithLimit implements a two-phase compression strategy:
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
func (e *ReactEngine) enforceContextBudgetWithLimit(
	ctx context.Context,
	messages []ports.Message,
	state *TaskState,
	services Services,
	limit int,
) []ports.Message {
	if limit <= 0 {
		return messages
	}
	estimated := services.Context.EstimateTokens(messages)

	// Phase B: Apply a pending summary if enough turns have elapsed.
	if state.PendingSummary != "" && state.Iterations >= state.PendingSummaryAtIter+delayedSummaryTurns {
		applied := applyPendingSummary(messages, state)
		afterApply := services.Context.EstimateTokens(applied)
		e.logger.Info(
			"Deferred summary applied: %d → %d tokens (generated at iter %d, now iter %d)",
			estimated, afterApply, state.PendingSummaryAtIter, state.Iterations,
		)
		state.Messages = applied
		clearPendingSummary(state)
		if afterApply <= limit {
			return applied
		}
		messages = applied
		estimated = afterApply
	}

	// --- Unified compression: all paths share the ShouldCompress threshold. ---
	shouldCompress := services.Context.ShouldCompress(messages, limit)

	// Phase A: Artifact compaction at the threshold — preserves compressed
	// messages as reviewable files before replacing them with a placeholder.
	if shouldCompress && !isCompactionInCooldown(state) {
		compacted, ok := e.tryArtifactCompaction(ctx, state, services, messages, compactionReasonThreshold, false)
		if ok {
			afterCompact := services.Context.EstimateTokens(compacted)
			e.logger.Info("Artifact compaction at threshold: %d → %d tokens (limit=%d)", estimated, afterCompact, limit)
			state.Messages = compacted
			if afterCompact <= limit {
				return compacted
			}
			messages = compacted
			estimated = afterCompact
		}
	}

	// Phase A fallback: If artifact compaction didn't fire (cooldown, no writer,
	// etc.), generate a deferred summary as before.
	if shouldCompress && estimated <= limit && state.PendingSummary == "" {
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
		return messages
	}

	if estimated <= limit {
		return messages
	}

	// --- Safety net: tokens exceed the hard limit. ---
	e.logger.Warn("Context budget exceeded: estimated=%d limit=%d messages=%d — applying safety-net compression",
		estimated, limit, len(messages))

	// If we have a pending summary, apply it immediately.
	if state.PendingSummary != "" {
		applied := applyPendingSummary(messages, state)
		afterApply := services.Context.EstimateTokens(applied)
		e.logger.Info("Pending summary applied early (budget exceeded): %d → %d tokens", estimated, afterApply)
		state.Messages = applied
		clearPendingSummary(state)
		if afterApply <= limit {
			return applied
		}
		messages = applied
		estimated = afterApply
	}

	// Layer 1a: Forced artifact compaction (bypass cooldown in emergency).
	{
		compacted, ok := e.tryArtifactCompaction(ctx, state, services, messages, compactionReasonOverflow, true)
		if ok {
			afterCompact := services.Context.EstimateTokens(compacted)
			e.logger.Info("Artifact compaction (overflow): %d → %d tokens (limit=%d)", estimated, afterCompact, limit)
			if afterCompact <= limit {
				state.Messages = compacted
				return compacted
			}
			messages = compacted
			estimated = afterCompact
		}
	}

	// Layer 1b: Immediate AutoCompact.
	{
		compacted, ok := services.Context.AutoCompact(messages, limit)
		if ok {
			afterCompact := services.Context.EstimateTokens(compacted)
			e.logger.Info("Auto-compact reduced tokens: %d → %d (limit=%d)", estimated, afterCompact, limit)
			if afterCompact <= limit {
				state.Messages = compacted
				return compacted
			}
			messages = compacted
			estimated = afterCompact
		}
	}

	// Layer 2: Aggressive trim — keep system/important + last N turns.
	for turns := 4; turns >= 1; turns-- {
		trimmed := aggressiveTrimMessages(messages, turns)
		afterTrim := services.Context.EstimateTokens(trimmed)
		e.logger.Info("Aggressive trim (turns=%d): %d → %d tokens (limit=%d)",
			turns, estimated, afterTrim, limit)
		if afterTrim <= limit {
			state.Messages = trimmed
			return trimmed
		}
	}

	// Last resort: keep only system/important messages + the very last message.
	e.logger.Warn("All trimming strategies insufficient — keeping only preserved messages + last message")
	trimmed := aggressiveTrimMessages(messages, 1)
	afterTrim := services.Context.EstimateTokens(trimmed)
	if afterTrim > limit {
		forceFitted := forceFitMessagesToLimit(trimmed, limit, services.Context.EstimateTokens)
		afterForceFit := services.Context.EstimateTokens(forceFitted)
		e.logger.Warn("Hard context clamp applied: %d → %d tokens (limit=%d)", afterTrim, afterForceFit, limit)
		trimmed = forceFitted
	}
	state.Messages = trimmed
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
	kept := ports.KeepRecentTurns(conversation[:convCountAtGen], 1)
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
