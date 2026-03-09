package context

import (
	"context"

	"alex/internal/domain/agent/ports"
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
	for i, att := range attachments {
		cloned[i] = ports.CloneAttachment(att)
	}
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
}

// GetInheritedAttachments retrieves inherited attachments from the context.
// Returns nil maps when not present.
func GetInheritedAttachments(ctx context.Context) (map[string]ports.Attachment, map[string]int) {
	if ctx == nil {
		return nil, nil
	}
	value, ok := ctx.Value(inheritedAttachmentsKey{}).(inheritedAttachmentPayload)
	if !ok {
		return nil, nil
	}
	return ports.CloneAttachmentMap(value.attachments), ports.CloneStringIntMap(value.iterations)
}
