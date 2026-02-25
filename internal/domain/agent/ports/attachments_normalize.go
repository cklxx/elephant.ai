package ports

import "strings"

// NormalizeAttachmentMap normalizes attachment map keys by trimming
// placeholders and falling back to attachment names when needed.
// It fills Attachment.Name only when the current value is exactly empty.
func NormalizeAttachmentMap(values map[string]Attachment) map[string]Attachment {
	return normalizeAttachmentMap(values, false)
}

// NormalizeAttachmentMapFillBlankName normalizes attachment maps like
// NormalizeAttachmentMap, but fills Attachment.Name when the current value is
// empty after trimming whitespace.
func NormalizeAttachmentMapFillBlankName(values map[string]Attachment) map[string]Attachment {
	return normalizeAttachmentMap(values, true)
}

func normalizeAttachmentMap(values map[string]Attachment, fillBlankName bool) map[string]Attachment {
	if len(values) == 0 {
		return nil
	}

	normalized := make(map[string]Attachment, len(values))
	for key, att := range values {
		name := strings.TrimSpace(key)
		if name == "" {
			name = strings.TrimSpace(att.Name)
		}
		if name == "" {
			continue
		}

		if fillBlankName {
			if strings.TrimSpace(att.Name) == "" {
				att.Name = name
			}
		} else if att.Name == "" {
			att.Name = name
		}

		normalized[name] = att
	}

	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
