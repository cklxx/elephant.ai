package domain

import (
	"strings"
	"time"

	"alex/internal/agent/ports"
)

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
		Important:    ports.CloneImportantNotes(state.Important),
		Duration:     duration,
	}
}

func (e *ReactEngine) decorateFinalResult(state *TaskState, result *TaskResult) map[string]ports.Attachment {
	if state == nil || result == nil {
		return nil
	}

	attachments := resolveContentAttachments(result.Answer, state)

	result.Answer = ensureAttachmentPlaceholders(result.Answer, attachments)

	a2uiAttachments := collectA2UIAttachments(state)
	if len(a2uiAttachments) == 0 {
		if len(attachments) == 0 {
			return nil
		}
		return attachments
	}

	if attachments == nil {
		attachments = make(map[string]ports.Attachment, len(a2uiAttachments))
	}
	for key, att := range a2uiAttachments {
		if _, ok := attachments[key]; ok {
			continue
		}
		attachments[key] = att
	}
	return attachments
}
