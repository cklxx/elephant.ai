package react

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

func checkpointFromState(state *TaskState, pending []ToolCallState, idGenerator agent.IDGenerator) *Checkpoint {
	if state == nil {
		return nil
	}
	messages := make([]MessageState, 0, len(state.Messages))
	for _, msg := range state.Messages {
		messages = append(messages, MessageState{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	cp := &Checkpoint{
		ID:            idGenerator.NewUUIDv7(),
		SessionID:     state.SessionID,
		Iteration:     state.Iterations,
		Messages:      messages,
		CreatedAt:     time.Now(),
		Version:       CheckpointVersion,
		MaxIterations: 0,
	}
	if len(pending) > 0 {
		cp.PendingTools = pending
	}
	return cp
}

func stateFromCheckpoint(cp *Checkpoint) *TaskState {
	if cp == nil {
		return nil
	}
	messages := make([]Message, 0, len(cp.Messages))
	for _, msg := range cp.Messages {
		messages = append(messages, Message{
			Role:    msg.Role,
			Content: msg.Content,
			Source:  ports.MessageSourceUnknown,
		})
	}
	return &TaskState{
		SessionID:  cp.SessionID,
		Messages:   messages,
		Iterations: cp.Iteration,
	}
}

func toolCallFromCheckpoint(state ToolCallState) ToolCall {
	return ToolCall{
		ID:        state.ID,
		Name:      state.Name,
		Arguments: state.Arguments,
	}
}

func isPendingToolStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "pending", "running":
		return true
	default:
		return false
	}
}

func (e *ReactEngine) ResumeFromCheckpoint(ctx context.Context, sessionID string, state *TaskState, services Services) (bool, error) {
	if e == nil || e.checkpointStore == nil {
		return false, nil
	}
	if state == nil {
		return false, nil
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(state.SessionID)
	}
	if sessionID == "" {
		return false, nil
	}

	cp, err := e.checkpointStore.Load(ctx, sessionID)
	if err != nil {
		return false, err
	}
	if cp == nil {
		return false, nil
	}
	if cp.Version != 0 && cp.Version != CheckpointVersion {
		e.logger.Warn("Checkpoint version mismatch (got=%d want=%d); skipping restore", cp.Version, CheckpointVersion)
		if err := e.checkpointStore.Delete(ctx, sessionID); err != nil {
			e.logger.Warn("Failed to delete incompatible checkpoint: %v", err)
		}
		return false, nil
	}

	if cp.MaxIterations > 0 {
		e.maxIterations = cp.MaxIterations
	}

	restored := stateFromCheckpoint(cp)
	if restored != nil {
		state.Messages = restored.Messages
		state.Iterations = restored.Iterations
		if strings.TrimSpace(state.SessionID) == "" {
			state.SessionID = restored.SessionID
		}
	}

	ensureAttachmentStore(state)
	offloadMessageThinking(state)
	e.ensureSystemPromptMessage(state)
	attachmentsChanged := false
	for idx := range state.Messages {
		if registerMessageAttachments(ctx, state, &state.Messages[idx], e.attachmentPersister) {
			attachmentsChanged = true
		}
	}
	if attachmentsChanged {
		e.updateAttachmentCatalogMessage(state)
	}

	if len(cp.PendingTools) > 0 {
		calls, results, err := e.recoverPendingTools(ctx, state, services, cp)
		if err != nil {
			return false, err
		}
		if len(results) > 0 {
			state.ToolResults = append(state.ToolResults, results...)
			e.observeToolResults(ctx, state, cp.Iteration, results)
			e.updateGoalPlanPrompts(state, calls, results)

			toolMessages := e.buildToolMessages(results)
			toolMessages = e.appendGoalPlanReminder(state, toolMessages)
			startIdx := len(state.Messages)
			state.Messages = append(state.Messages, toolMessages...)
			attachmentsChanged = false
			for i := range toolMessages {
				if registerMessageAttachments(ctx, state, &state.Messages[startIdx+i], e.attachmentPersister) {
					attachmentsChanged = true
				}
			}
			if attachmentsChanged {
				e.updateAttachmentCatalogMessage(state)
			}
			offloadMessageAttachmentData(state)
		}
	}

	if err := e.checkpointStore.Delete(ctx, sessionID); err != nil {
		e.logger.Warn("Failed to delete checkpoint after restore: %v", err)
	}

	return true, nil
}

func (e *ReactEngine) recoverPendingTools(ctx context.Context, state *TaskState, services Services, cp *Checkpoint) ([]ToolCall, []ToolResult, error) {
	if cp == nil || len(cp.PendingTools) == 0 {
		return nil, nil, nil
	}

	execCalls := make([]ToolCall, 0, len(cp.PendingTools))
	for _, pending := range cp.PendingTools {
		if isPendingToolStatus(pending.Status) {
			execCalls = append(execCalls, toolCallFromCheckpoint(pending))
		}
	}

	var execResults []ToolResult
	if len(execCalls) > 0 {
		if services.ToolExecutor == nil {
			return nil, nil, fmt.Errorf("checkpoint: tool executor required for pending tool recovery")
		}
		execResults = newToolCallBatch(
			e,
			ctx,
			state,
			cp.Iteration,
			execCalls,
			services.ToolExecutor,
			services.ToolLimiter,
			nil,
		).execute()
	}

	calls := make([]ToolCall, 0, len(cp.PendingTools))
	results := make([]ToolResult, 0, len(cp.PendingTools))
	execIdx := 0

	for _, pending := range cp.PendingTools {
		call := toolCallFromCheckpoint(pending)
		status := strings.ToLower(strings.TrimSpace(pending.Status))
		switch status {
		case "completed":
			content := ""
			if pending.Result != nil {
				content = *pending.Result
			}
			res := ToolResult{CallID: call.ID, Content: content}
			res = e.normalizeToolResult(call, state, res)
			calls = append(calls, call)
			results = append(results, res)
		case "failed":
			errMsg := "tool failed before checkpoint"
			if pending.Result != nil && strings.TrimSpace(*pending.Result) != "" {
				errMsg = fmt.Sprintf("tool failed before checkpoint: %s", strings.TrimSpace(*pending.Result))
			}
			res := ToolResult{CallID: call.ID, Error: fmt.Errorf("%s", errMsg)}
			res = e.normalizeToolResult(call, state, res)
			calls = append(calls, call)
			results = append(results, res)
		default:
			if execIdx >= len(execResults) {
				continue
			}
			res := execResults[execIdx]
			execIdx++
			res = e.normalizeToolResult(call, state, res)
			calls = append(calls, call)
			results = append(results, res)
		}
	}

	return calls, results, nil
}

func pendingToolStates(calls []ToolCall) []ToolCallState {
	if len(calls) == 0 {
		return nil
	}
	pending := make([]ToolCallState, 0, len(calls))
	for _, call := range calls {
		pending = append(pending, ToolCallState{
			ID:        call.ID,
			Name:      call.Name,
			Arguments: call.Arguments,
			Status:    "pending",
		})
	}
	return pending
}

func (e *ReactEngine) saveCheckpoint(ctx context.Context, state *TaskState, pending []ToolCallState) {
	if e == nil || e.checkpointStore == nil {
		return
	}
	cp := checkpointFromState(state, pending, e.idGenerator)
	if cp == nil {
		return
	}
	cp.MaxIterations = e.maxIterations
	cp.CreatedAt = e.clock.Now()
	cp.Version = CheckpointVersion
	if cp.ID == "" {
		cp.ID = e.idGenerator.NewUUIDv7()
	}

	if err := e.checkpointStore.Save(ctx, cp); err != nil {
		e.logger.Warn("Failed to save checkpoint: %v", err)
	}
}

func (e *ReactEngine) clearCheckpoint(ctx context.Context, sessionID string) {
	if e == nil || e.checkpointStore == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	if err := e.checkpointStore.Delete(ctx, sessionID); err != nil {
		e.logger.Warn("Failed to delete checkpoint after completion: %v", err)
	}
}
