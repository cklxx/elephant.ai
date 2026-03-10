package channels

import "alex/internal/shared/errsanitize"

// SanitizeErrorForUser delegates to errsanitize.ForUser.
// Kept for backward compatibility within the delivery layer.
func SanitizeErrorForUser(errText string) string {
	return errsanitize.ForUser(errText)
}
