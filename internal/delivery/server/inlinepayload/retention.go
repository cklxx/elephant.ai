package inlinepayload

import (
	"alex/internal/shared/utils"
	"strings"
)

// ShouldRetain reports whether an inline attachment payload should be retained
// based on media type, payload size, and the caller-provided size limit.
func ShouldRetain(mediaType string, size int, limit int) bool {
	if size <= 0 || size > limit {
		return false
	}

	media := utils.TrimLower(mediaType)
	if media == "" {
		return false
	}

	if strings.HasPrefix(media, "text/") {
		return true
	}

	return strings.Contains(media, "markdown") || strings.Contains(media, "json")
}
