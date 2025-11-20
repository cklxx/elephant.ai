package ports

// CloneAttachment returns a deep copy of the provided attachment so callers can
// safely mutate slices without affecting the original reference.
func CloneAttachment(att Attachment) Attachment {
	cloned := att
	if len(att.PreviewAssets) > 0 {
		assets := make([]AttachmentPreviewAsset, len(att.PreviewAssets))
		copy(assets, att.PreviewAssets)
		cloned.PreviewAssets = assets
	}
	return cloned
}

// CloneAttachmentMap returns a deep copy of a map of attachments.
func CloneAttachmentMap(src map[string]Attachment) map[string]Attachment {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]Attachment, len(src))
	for key, att := range src {
		cloned[key] = CloneAttachment(att)
	}
	return cloned
}

// cloneAttachmentMap mirrors the behaviour of CloneAttachmentMap but keeps the
// original unexported helper name used by older files. It exists so new code
// can depend on the canonical helper while legacy references continue to
// compile.
func cloneAttachmentMap(src map[string]Attachment) map[string]Attachment {
        return CloneAttachmentMap(src)
}

// keep the legacy helper referenced so static analysis doesn't drop it in
// builds where older files aren't compiled.
var _ = cloneAttachmentMap
