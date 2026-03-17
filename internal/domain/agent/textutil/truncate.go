package textutil

import "alex/internal/shared/utils"

// TruncateWithEllipsis shortens input and appends an ellipsis.
func TruncateWithEllipsis(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	return utils.Truncate(input, limit, "…")
}
