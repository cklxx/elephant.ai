package react

import (
	"context"
	"fmt"
	"strings"
	"time"

	domain "alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	tokenutil "alex/internal/shared/token"
)

// minPrunableMessages is the minimum number of prunable messages required
// before a context checkpoint is applied. With fewer messages, pruning would
// remove too little to be worthwhile.
const minPrunableMessages = 4

// applyContextCheckpoint inspects the tool results from the current iteration
// for a context_checkpoint call. If found, it archives the prunable messages
// and replaces them with a single checkpoint summary message.
//
// Returns true if pruning was applied.
func (e *ReactEngine) applyContextCheckpoint(
	ctx context.Context,
	state *TaskState,
	services Services,
	toolResults []ToolResult,
	toolCalls []ToolCall,
) bool {
	if state == nil || len(toolResults) == 0 {
		return false
	}

	// Build a call-ID → ToolCall index for quick lookup.
	callIndex := make(map[string]*ToolCall, len(toolCalls))
	for i := range toolCalls {
		callIndex[toolCalls[i].ID] = &toolCalls[i]
	}

	// Find the context_checkpoint tool result by matching call IDs back to calls.
	var checkpointResult *ToolResult
	var checkpointCall *ToolCall
	for i := range toolResults {
		call, ok := callIndex[toolResults[i].CallID]
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(call.Name), "context_checkpoint") {
			checkpointResult = &toolResults[i]
			checkpointCall = call
			break
		}
	}
	if checkpointResult == nil || checkpointResult.Error != nil || checkpointCall == nil {
		return false
	}

	summary, _ := checkpointCall.Arguments["summary"].(string)
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return false
	}

	phaseLabel := "phase"
	if label, ok := checkpointCall.Arguments["phase_label"].(string); ok && strings.TrimSpace(label) != "" {
		phaseLabel = strings.TrimSpace(label)
	}

	// --- Locate boundaries ---

	// Find last checkpoint index (or first non-preserved message).
	lastCheckpointIdx := -1
	for i := range state.Messages {
		if state.Messages[i].Source == ports.MessageSourceCheckpoint {
			lastCheckpointIdx = i
		}
	}

	// If no prior checkpoint, find the first non-preserved message.
	pruneStart := 0
	if lastCheckpointIdx >= 0 {
		pruneStart = lastCheckpointIdx + 1
	} else {
		for i := range state.Messages {
			if !isPreservedSource(state.Messages[i].Source) {
				pruneStart = i
				break
			}
		}
	}

	// The current iteration's messages are the assistant message containing
	// the context_checkpoint tool call plus the tool result messages that
	// follow it. We identify this boundary by walking backwards from the end
	// to find the assistant message that issued the checkpoint call.
	currentIterStart := len(state.Messages)
	for i := len(state.Messages) - 1; i >= pruneStart; i-- {
		msg := state.Messages[i]
		if msg.Role == "assistant" && containsToolCall(msg, checkpointCall.ID) {
			currentIterStart = i
			break
		}
	}

	// Collect prunable messages in range [pruneStart, currentIterStart).
	var prunable []int
	for i := pruneStart; i < currentIterStart; i++ {
		if !isPreservedSource(state.Messages[i].Source) {
			prunable = append(prunable, i)
		}
	}

	if len(prunable) < minPrunableMessages {
		e.logger.Info("Context checkpoint: too few prunable messages (%d < %d), skipping",
			len(prunable), minPrunableMessages)
		return false
	}

	// --- Archive before pruning ---
	prunedTokens := 0
	var archivedMsgs []MessageState
	for _, idx := range prunable {
		msg := state.Messages[idx]
		prunedTokens += tokenutil.CountTokens(msg.Content)
		archivedMsgs = append(archivedMsgs, MessageState{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	if e.checkpointStore != nil {
		archive := &CheckpointArchive{
			SessionID:  state.SessionID,
			Seq:        nextArchiveSeq(state),
			PhaseLabel: phaseLabel,
			Messages:   archivedMsgs,
			TokenCount: prunedTokens,
			CreatedAt:  e.clock.Now(),
		}
		if err := e.checkpointStore.SaveArchive(ctx, archive); err != nil {
			e.logger.Warn("Context checkpoint: archive write failed (proceeding with prune): %v", err)
		}
	}

	// --- Build checkpoint message ---
	checkpointMsg := ports.Message{
		Role:    "user",
		Content: fmt.Sprintf("[Phase Complete: %s]\n\n%s", phaseLabel, summary),
		Source:  ports.MessageSourceCheckpoint,
		Metadata: map[string]any{
			"phase":         phaseLabel,
			"pruned_count":  len(prunable),
			"pruned_tokens": prunedTokens,
			"checkpoint_at": e.clock.Now().Format(time.RFC3339),
		},
	}

	// --- Reconstruct messages ---
	pruneSet := make(map[int]struct{}, len(prunable))
	for _, idx := range prunable {
		pruneSet[idx] = struct{}{}
	}

	var reconstructed []ports.Message
	// Messages before prune range (preserved).
	for i := 0; i < pruneStart; i++ {
		reconstructed = append(reconstructed, state.Messages[i])
	}
	// Preserved messages within prune range.
	for i := pruneStart; i < currentIterStart; i++ {
		if _, isPruned := pruneSet[i]; !isPruned {
			reconstructed = append(reconstructed, state.Messages[i])
		}
	}
	// Insert checkpoint message.
	reconstructed = append(reconstructed, checkpointMsg)
	// Current iteration messages.
	for i := currentIterStart; i < len(state.Messages); i++ {
		reconstructed = append(reconstructed, state.Messages[i])
	}

	state.Messages = reconstructed

	// --- Recalculate token count ---
	summaryTokens := tokenutil.CountTokens(checkpointMsg.Content)
	state.TokenCount = services.Context.EstimateTokens(state.Messages)

	// --- Emit diagnostic event ---
	e.emitEvent(domain.NewDiagnosticContextCheckpointEvent(
		e.newBaseEvent(ctx, state.SessionID, state.RunID, state.ParentRunID),
		phaseLabel, len(prunable), prunedTokens, summaryTokens, state.TokenCount,
	))

	e.logger.Info("Context checkpoint applied: phase=%s pruned=%d pruned_tokens=%d remaining_tokens=%d",
		phaseLabel, len(prunable), prunedTokens, state.TokenCount)

	return true
}

// isPreservedSource returns true for message sources that must survive pruning.
func isPreservedSource(src ports.MessageSource) bool {
	switch src {
	case ports.MessageSourceSystemPrompt, ports.MessageSourceImportant, ports.MessageSourceCheckpoint:
		return true
	default:
		return false
	}
}

// containsToolCall checks if an assistant message issued a tool call with the
// given ID.
func containsToolCall(msg ports.Message, callID string) bool {
	for _, tc := range msg.ToolCalls {
		if tc.ID == callID {
			return true
		}
	}
	return false
}

// nextArchiveSeq returns a monotonically increasing sequence number for
// checkpoint archives within a session. It counts existing checkpoint messages
// in state as a proxy (each checkpoint = one prior archive).
func nextArchiveSeq(state *TaskState) int {
	seq := 0
	for _, msg := range state.Messages {
		if msg.Source == ports.MessageSourceCheckpoint {
			seq++
		}
	}
	return seq
}
