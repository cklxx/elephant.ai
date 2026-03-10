package react

import (
	"context"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/utils"
)

// snapshotAttachments clones the attachment store and returns per-attachment
// iteration indices for later reconciliation.
func snapshotAttachments(state *TaskState) (map[string]ports.Attachment, map[string]int) {
	if state == nil {
		return nil, nil
	}
	var attachments map[string]ports.Attachment
	if len(state.Attachments) > 0 {
		attachments = make(map[string]ports.Attachment, len(state.Attachments))
		for key, att := range state.Attachments {
			attachments[key] = att
		}
	}
	var iterations map[string]int
	if len(state.AttachmentIterations) > 0 {
		iterations = make(map[string]int, len(state.AttachmentIterations))
		for key, iter := range state.AttachmentIterations {
			iterations[key] = iter
		}
	}
	return attachments, iterations
}

// normalizeToolAttachments standardizes tool attachments by filling defaults
// and trimming empty entries before storage.
func normalizeToolAttachments(attachments map[string]ports.Attachment) map[string]ports.Attachment {
	if len(attachments) == 0 {
		return nil
	}
	normalized := make(map[string]ports.Attachment, len(attachments))
	for _, key := range sortedAttachmentKeys(attachments) {
		placeholder, att, ok := normalizeAttachmentEntry(key, attachments[key], false)
		if !ok {
			continue
		}
		normalized[placeholder] = att
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func sortedAttachmentKeys(attachments map[string]ports.Attachment) []string {
	if len(attachments) == 0 {
		return nil
	}
	keys := make([]string, 0, len(attachments))
	seen := make(map[string]bool, len(attachments))
	for key := range attachments {
		name := strings.TrimSpace(key)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		keys = append(keys, name)
	}
	return keys
}

// applyAttachmentMutationsToState merges incoming attachment mutations with
// the task state, ensuring delete/update/replace semantics stay consistent.
// When persister is non-nil, inline payloads are eagerly written to durable
// storage so that state.Attachments holds URI references instead of base64.
func applyAttachmentMutationsToState(
	ctx context.Context,
	state *TaskState,
	merged map[string]ports.Attachment,
	mutations *attachmentMutations,
	defaultSource string,
	persister ports.AttachmentPersister,
) {
	if state == nil {
		return
	}
	if defaultSource = strings.TrimSpace(defaultSource); defaultSource == "" {
		defaultSource = "tool"
	}

	ensureAttachmentStore(state)

	if mutations != nil && mutations.replace != nil {
		state.Attachments = make(map[string]ports.Attachment, len(mutations.replace))
		state.AttachmentIterations = make(map[string]int, len(mutations.replace))
		for key, att := range mutations.replace {
			att.Source = coalesceAttachmentSource(att.Source, defaultSource)
			att = persistAttachmentIfNeeded(ctx, att, persister)
			state.Attachments[key] = att
			state.AttachmentIterations[key] = state.Iterations
		}
	}

	if mutations != nil && len(mutations.remove) > 0 {
		for _, key := range mutations.remove {
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				continue
			}
			delete(state.Attachments, trimmed)
			if state.AttachmentIterations != nil {
				delete(state.AttachmentIterations, trimmed)
			}
		}
	}

	if len(merged) == 0 {
		return
	}

	for key, att := range merged {
		att.Source = coalesceAttachmentSource(att.Source, defaultSource)
		att = persistAttachmentIfNeeded(ctx, att, persister)
		state.Attachments[key] = att
		if state.AttachmentIterations == nil {
			state.AttachmentIterations = make(map[string]int)
		}
		state.AttachmentIterations[key] = state.Iterations
	}
}

// persistAttachmentIfNeeded writes the attachment's inline payload to durable
// storage when a persister is available. On failure the original attachment is
// returned unchanged for graceful degradation.
func persistAttachmentIfNeeded(ctx context.Context, att ports.Attachment, persister ports.AttachmentPersister) ports.Attachment {
	if persister == nil {
		return att
	}
	if shouldSkipPersist(att) {
		return att
	}
	persisted, err := persister.Persist(ctx, att)
	if err != nil {
		return att
	}
	return persisted
}

func coalesceAttachmentSource(source, fallback string) string {
	if utils.HasContent(source) {
		return source
	}
	return fallback
}

func mergeAttachmentMutations(
	base map[string]ports.Attachment,
	mutations *attachmentMutations,
	existing map[string]ports.Attachment,
) map[string]ports.Attachment {
	merged := make(map[string]ports.Attachment)

	switch {
	case mutations != nil && mutations.replace != nil:
		mergeAttachmentMap(merged, mutations.replace)
	case len(base) > 0:
		mergeAttachmentMap(merged, base)
	case len(existing) > 0:
		mergeAttachmentMap(merged, existing)
	}

	if mutations != nil {
		if mutations.add != nil {
			mergeAttachmentMap(merged, mutations.add)
		}
		if mutations.update != nil {
			mergeAttachmentMap(merged, mutations.update)
		}
		for _, key := range mutations.remove {
			trimmed := strings.TrimSpace(key)
			if trimmed != "" {
				delete(merged, trimmed)
			}
		}
	}

	if len(merged) == 0 {
		return nil
	}
	return merged
}

func shouldSkipPersist(att ports.Attachment) bool {
	if att.Fingerprint == "" {
		return false
	}
	if att.Data != "" {
		return false
	}
	return !strings.HasPrefix(utils.TrimLower(att.URI), "data:")
}
func mergeAttachmentMap(dst, src map[string]ports.Attachment) {
	for key, att := range src {
		dst[key] = att
	}
}

func normalizeAttachmentEntry(key string, att ports.Attachment, fillNameWhenTrimmedEmpty bool) (string, ports.Attachment, bool) {
	placeholder := strings.TrimSpace(key)
	if placeholder == "" {
		placeholder = strings.TrimSpace(att.Name)
	}
	if placeholder == "" {
		return "", att, false
	}

	if fillNameWhenTrimmedEmpty {
		if utils.IsBlank(att.Name) {
			att.Name = placeholder
		}
		return placeholder, att, true
	}

	if att.Name == "" {
		att.Name = placeholder
	}
	return placeholder, att, true
}

// ensureAttachmentStore initializes the attachment map on the task state.
func ensureAttachmentStore(state *TaskState) {
	if state.Attachments == nil {
		state.Attachments = make(map[string]ports.Attachment)
	}
	if state.AttachmentIterations == nil {
		state.AttachmentIterations = make(map[string]int)
	}
}

// registerMessageAttachments pulls attachments from a message into the shared
// task store, returning true when the catalog changed.
func registerMessageAttachments(ctx context.Context, state *TaskState, msg *Message, persister ports.AttachmentPersister) bool {
	if msg == nil || len(msg.Attachments) == 0 {
		return false
	}
	ensureAttachmentStore(state)
	changed := false
	for key, att := range msg.Attachments {
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		if att.Name == "" {
			att.Name = placeholder
		}
		att = persistAttachmentIfNeeded(ctx, att, persister)
		if existing, ok := state.Attachments[placeholder]; !ok || !attachmentsEqual(existing, att) {
			state.Attachments[placeholder] = att
			changed = true
		}
		if !attachmentsEqual(msg.Attachments[key], att) {
			msg.Attachments[key] = att
		}
		if state.AttachmentIterations == nil {
			state.AttachmentIterations = make(map[string]int)
		}
		state.AttachmentIterations[placeholder] = state.Iterations
	}
	return changed
}

func attachmentsEqual(a, b ports.Attachment) bool {
	if a.Name != b.Name ||
		a.MediaType != b.MediaType ||
		a.Data != b.Data ||
		a.URI != b.URI ||
		a.Fingerprint != b.Fingerprint ||
		a.Source != b.Source ||
		a.Description != b.Description ||
		a.Kind != b.Kind ||
		a.Format != b.Format ||
		a.PreviewProfile != b.PreviewProfile {
		return false
	}
	return previewAssetsEqual(a.PreviewAssets, b.PreviewAssets)
}

func previewAssetsEqual(a, b []ports.AttachmentPreviewAsset) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
