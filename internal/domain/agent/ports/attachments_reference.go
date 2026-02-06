package ports

import (
	"fmt"
	"strings"
)

// AttachmentReferenceValue returns the best-effort reference for an attachment.
// Prefer URI when present; otherwise fall back to a data URI built from Data.
func AttachmentReferenceValue(att Attachment) string {
	if uri := strings.TrimSpace(att.URI); uri != "" {
		return uri
	}

	data := strings.TrimSpace(att.Data)
	if data == "" {
		return ""
	}

	if strings.HasPrefix(strings.ToLower(data), "data:") {
		return data
	}

	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	return fmt.Sprintf("data:%s;base64,%s", mediaType, data)
}

// AttachmentInlineBase64 extracts a raw base64 payload suitable for provider APIs
// that require inline bytes (for example, Ollama's chat images field).
func AttachmentInlineBase64(att Attachment) string {
	if data := strings.TrimSpace(att.Data); data != "" {
		if b64, ok := extractDataURIBase64(data); ok {
			return b64
		}
		return data
	}

	if uri := strings.TrimSpace(att.URI); uri != "" {
		if b64, ok := extractDataURIBase64(uri); ok {
			return b64
		}
	}

	return ""
}

func extractDataURIBase64(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}

	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "data:") {
		return "", false
	}

	comma := strings.Index(trimmed, ",")
	if comma < 0 {
		return "", false
	}

	meta := strings.ToLower(trimmed[:comma])
	if !strings.Contains(meta, ";base64") {
		return "", false
	}

	payload := strings.TrimSpace(trimmed[comma+1:])
	if payload == "" {
		return "", false
	}
	return payload, true
}
