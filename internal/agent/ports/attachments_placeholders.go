package ports

import "strings"

// AttachmentPlaceholder formats a placeholder token (e.g. "[diagram.png]") for an attachment name.
func AttachmentPlaceholder(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if parsed, ok := AttachmentPlaceholderName(trimmed); ok {
		return "[" + parsed + "]"
	}
	return "[" + trimmed + "]"
}

// AttachmentPlaceholderName extracts the attachment name from a placeholder token.
func AttachmentPlaceholderName(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 2 {
		return "", false
	}
	if trimmed[0] != '[' || trimmed[len(trimmed)-1] != ']' {
		return "", false
	}
	name := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if name == "" {
		return "", false
	}
	return name, true
}

// AttachmentVisibleToLLM returns whether an attachment should be embedded in LLM inputs.
func AttachmentVisibleToLLM(att Attachment) bool {
	if att.ShowToLLM == nil {
		return true
	}
	return *att.ShowToLLM
}
