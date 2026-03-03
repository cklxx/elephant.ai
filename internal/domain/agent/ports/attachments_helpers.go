package ports

import (
	"alex/internal/shared/utils"
	"path/filepath"
	"strings"
)

// MergeAttachmentMaps merges src into dst using normalized attachment names.
// When override is false, existing dst entries win.
func MergeAttachmentMaps(dst, src map[string]Attachment, override bool) map[string]Attachment {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]Attachment, len(src))
	}
	for key, att := range src {
		name := strings.TrimSpace(key)
		if name == "" {
			name = strings.TrimSpace(att.Name)
		}
		if name == "" {
			continue
		}
		if _, exists := dst[name]; exists && !override {
			continue
		}
		if att.Name == "" {
			att.Name = name
		}
		dst[name] = att
	}
	return dst
}

// IsImageAttachment reports whether an attachment should be treated as an image.
// It checks media type first, then falls back to filename extension.
func IsImageAttachment(att Attachment, fallbackName string) bool {
	if strings.HasPrefix(utils.TrimLower(att.MediaType), "image/") {
		return true
	}

	name := strings.TrimSpace(att.Name)
	if name == "" {
		name = strings.TrimSpace(fallbackName)
	}
	if name == "" {
		return false
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		return true
	default:
		return false
	}
}
