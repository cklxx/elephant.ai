package react

import (
	"context"

	"alex/internal/agent/ports"
)

// prepareUserTaskContext mutates the provided task state so it is ready for a new
// user task turn: system prompt anchored, user task appended, and attachments
// catalogued without rewriting existing history.
func (e *ReactEngine) prepareUserTaskContext(ctx context.Context, task string, state *TaskState) {
	ensureAttachmentStore(state)
	offloadMessageThinking(state)

	attachmentsChanged := false
	for idx := range state.Messages {
		if registerMessageAttachments(ctx, state, &state.Messages[idx], e.attachmentPersister) {
			attachmentsChanged = true
		}
	}
	if attachmentsChanged {
		e.updateAttachmentCatalogMessage(state)
	}

	e.ensureSystemPromptMessage(state)

	userMessage := Message{
		Role:    "user",
		Content: task,
		Source:  ports.MessageSourceUserInput,
	}

	if len(state.PendingUserAttachments) > 0 {
		attachments := make(map[string]ports.Attachment, len(state.PendingUserAttachments))
		for key, att := range state.PendingUserAttachments {
			attachments[key] = att
		}
		userMessage.Attachments = attachments
		state.PendingUserAttachments = nil
	}

	if resolved := resolveContentAttachments(userMessage.Content, state); len(resolved) > 0 {
		if userMessage.Attachments == nil {
			userMessage.Attachments = make(map[string]ports.Attachment, len(resolved))
		}
		for key, att := range resolved {
			if _, exists := userMessage.Attachments[key]; exists {
				continue
			}
			userMessage.Attachments[key] = att
		}
	}

	state.Messages = append(state.Messages, userMessage)
	if registerMessageAttachments(ctx, state, &state.Messages[len(state.Messages)-1], e.attachmentPersister) {
		e.updateAttachmentCatalogMessage(state)
	}
}

func offloadMessageThinking(state *TaskState) {
	if state == nil {
		return
	}
	for idx := range state.Messages {
		if len(state.Messages[idx].Thinking.Parts) == 0 {
			continue
		}
		state.Messages[idx].Thinking = ports.Thinking{}
	}
}
