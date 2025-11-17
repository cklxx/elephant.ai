package app

import (
	"context"

	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
)

type userAttachmentsKey struct{}
type inheritedAttachmentsKey struct{}

// WithUserAttachments stores user-provided attachments in the context so they can
// be accessed during execution preparation.
func WithUserAttachments(ctx context.Context, attachments []ports.Attachment) context.Context {
	if len(attachments) == 0 {
		return ctx
	}
	cloned := make([]ports.Attachment, len(attachments))
	copy(cloned, attachments)
	return context.WithValue(ctx, userAttachmentsKey{}, cloned)
}

// GetUserAttachments extracts user-provided attachments from the context.
func GetUserAttachments(ctx context.Context) []ports.Attachment {
	if ctx == nil {
		return nil
	}
	if value, ok := ctx.Value(userAttachmentsKey{}).([]ports.Attachment); ok {
		return value
	}
	return nil
}

type inheritedAttachmentPayload struct {
	attachments map[string]ports.Attachment
	iterations  map[string]int
	parentTask  string
}

// WithInheritedAttachments shares generated attachments with delegated agents
// (e.g. subagent) so they can resolve placeholders in nested tasks.
func WithInheritedAttachments(ctx context.Context, attachments map[string]ports.Attachment, iterations map[string]int) context.Context {
	if len(attachments) == 0 {
		return ctx
	}
	parentTaskID := id.ParentTaskIDFromContext(ctx)

	cloned := cloneAttachmentMap(attachments)
	if parentTaskID != "" {
		for key, att := range cloned {
			if att.ParentTaskID == "" {
				att.ParentTaskID = parentTaskID
				cloned[key] = att
			}
		}
	}
	payload := inheritedAttachmentPayload{
		attachments: cloned,
		iterations:  cloneIterationMap(iterations),
		parentTask:  parentTaskID,
	}
	return context.WithValue(ctx, inheritedAttachmentsKey{}, payload)
}

// GetInheritedAttachments retrieves attachments propagated through
// WithInheritedAttachments. Returns nil maps when not present.
func GetInheritedAttachments(ctx context.Context) (map[string]ports.Attachment, map[string]int) {
	if ctx == nil {
		return nil, nil
	}
	value, ok := ctx.Value(inheritedAttachmentsKey{}).(inheritedAttachmentPayload)
	if !ok {
		return nil, nil
	}
	return cloneAttachmentMap(value.attachments), cloneIterationMap(value.iterations)
}

func cloneAttachmentMap(src map[string]ports.Attachment) map[string]ports.Attachment {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]ports.Attachment, len(src))
	for key, att := range src {
		cloned[key] = att
	}
	return cloned
}

func cloneIterationMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]int, len(src))
	for key, iter := range src {
		cloned[key] = iter
	}
	return cloned
}
