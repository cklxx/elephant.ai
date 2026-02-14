package app

import (
	"encoding/base64"
	"reflect"
	"strings"

	"alex/internal/domain/agent/ports"
)

const historyInlineAttachmentRetentionLimit = 128 * 1024

// stripBinaryPayloadsWithStore recursively sanitizes a value tree, stripping large
// binary blobs and normalising attachment payloads for event history storage.
// The store parameter is accepted for interface compatibility but may be nil.
func stripBinaryPayloadsWithStore(value any, _ any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case ports.Attachment:
		return sanitizeAttachmentForHistory(v)
	case *ports.Attachment:
		if v == nil {
			return nil
		}
		cleaned := sanitizeAttachmentForHistory(*v)
		return &cleaned
	case map[string]ports.Attachment:
		cleaned := make(map[string]ports.Attachment, len(v))
		for key, att := range v {
			cleaned[key] = sanitizeAttachmentForHistory(att)
		}
		return cleaned
	case []ports.Attachment:
		cleaned := make([]ports.Attachment, len(v))
		for i, att := range v {
			cleaned[i] = sanitizeAttachmentForHistory(att)
		}
		return cleaned
	case map[string]any:
		cleaned := make(map[string]any, len(v))
		for key, val := range v {
			cleaned[key] = stripBinaryPayloadsWithStore(val, nil)
		}
		return cleaned
	case []any:
		cleaned := make([]any, len(v))
		for i, val := range v {
			cleaned[i] = stripBinaryPayloadsWithStore(val, nil)
		}
		return cleaned
	}

	rv := reflect.ValueOf(value)
	if rv.IsValid() && rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
		return nil
	}

	return value
}

func sanitizeAttachmentForHistory(att ports.Attachment) ports.Attachment {
	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
		att.MediaType = mediaType
	}

	trimmedURI := strings.TrimSpace(att.URI)
	if att.Data == "" && trimmedURI != "" && !strings.HasPrefix(strings.ToLower(trimmedURI), "data:") {
		return att
	}

	inline := strings.TrimSpace(ports.AttachmentInlineBase64(att))
	if inline != "" {
		size := base64.StdEncoding.DecodedLen(len(inline))
		if shouldRetainInlinePayload(mediaType, size) {
			att.Data = inline
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(att.URI)), "data:") {
				att.URI = ""
			}
			return att
		}
	}

	att.Data = ""
	return att
}

func shouldRetainInlinePayload(mediaType string, size int) bool {
	if size <= 0 || size > historyInlineAttachmentRetentionLimit {
		return false
	}

	media := strings.ToLower(strings.TrimSpace(mediaType))
	if media == "" {
		return false
	}

	if strings.HasPrefix(media, "text/") {
		return true
	}

	return strings.Contains(media, "markdown") || strings.Contains(media, "json")
}
