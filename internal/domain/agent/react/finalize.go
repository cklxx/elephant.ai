package react

import (
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
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

	attachments := collectAllToolGeneratedAttachments(state)
	finalAnswer = stripAttachmentPlaceholders(finalAnswer)

	return &TaskResult{
		Answer:      finalAnswer,
		Messages:    state.Messages,
		Iterations:  state.Iterations,
		TokensUsed:  state.TokenCount,
		StopReason:  stopReason,
		SessionID:   state.SessionID,
		RunID:       state.RunID,
		ParentRunID: state.ParentRunID,
		Important:   ports.CloneImportantNotes(state.Important),
		Duration:    duration,
		Attachments: attachments,
	}
}

func (e *ReactEngine) decorateFinalResult(state *TaskState, result *TaskResult) map[string]ports.Attachment {
	if state == nil || result == nil {
		return nil
	}

	attachments := result.Attachments
	result.Answer = stripAttachmentPlaceholders(result.Answer)

	a2uiAttachments := collectA2UIAttachments(state)
	if len(a2uiAttachments) > 0 {
		if attachments == nil {
			attachments = make(map[string]ports.Attachment, len(a2uiAttachments))
		}
		for key, att := range a2uiAttachments {
			if _, ok := attachments[key]; ok {
				continue
			}
			attachments[key] = att
		}
	}

	result.Attachments = attachments

	if len(attachments) == 0 {
		return nil
	}
	return attachments
}
