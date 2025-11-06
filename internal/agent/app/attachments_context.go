package app

import (
	"context"

	"alex/internal/agent/ports"
)

type userAttachmentsKey struct{}

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
