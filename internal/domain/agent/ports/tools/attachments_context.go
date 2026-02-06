package tools

import (
	"context"

	core "alex/internal/agent/ports"
)

// attachmentsCtxKey stores attachment snapshots in a context so tools can
// access binary placeholders without depending on domain internals.
type attachmentsCtxKey struct{}

type attachmentContext struct {
	attachments map[string]core.Attachment
	iterations  map[string]int
}

// WithAttachmentContext annotates ctx with a snapshot of available attachments
// and their iteration metadata so downstream tools (e.g. subagent) can reuse
// them.
func WithAttachmentContext(ctx context.Context, attachments map[string]core.Attachment, iterations map[string]int) context.Context {
	if len(attachments) == 0 {
		return ctx
	}

	payload := attachmentContext{
		attachments: core.CloneAttachmentMap(attachments),
		iterations:  cloneIterationMap(iterations),
	}

	return context.WithValue(ctx, attachmentsCtxKey{}, payload)
}

// GetAttachmentContext extracts the attachment snapshot (if any) from ctx.
func GetAttachmentContext(ctx context.Context) (map[string]core.Attachment, map[string]int) {
	if ctx == nil {
		return nil, nil
	}

	value, ok := ctx.Value(attachmentsCtxKey{}).(attachmentContext)
	if !ok {
		return nil, nil
	}

	attachments := core.CloneAttachmentMap(value.attachments)
	iterations := cloneIterationMap(value.iterations)
	return attachments, iterations
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
