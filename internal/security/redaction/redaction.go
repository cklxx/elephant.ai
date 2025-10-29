package redaction

import "strings"

const Placeholder = "[REDACTED]"

var (
	sensitiveKeyFragments    = []string{"token", "secret", "password", "key", "authorization", "cookie", "credential", "session"}
	sensitiveValueIndicators = []string{"bearer ", "ghp_", "sk-", "xoxb-", "xoxp-", "-----begin", "api_key", "apikey", "access_token", "refresh_token"}
)

// IsSensitiveKey reports whether the provided key name likely references sensitive data.
func IsSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(lowerKey, fragment) {
			return true
		}
	}
	return false
}

// LooksLikeSecret reports whether the provided value appears to contain secret material.
func LooksLikeSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	lowerValue := strings.ToLower(trimmed)
	for _, indicator := range sensitiveValueIndicators {
		if strings.Contains(lowerValue, indicator) {
			return true
		}
	}

	if len(trimmed) >= 32 && !strings.ContainsAny(trimmed, " \n\t") {
		return true
	}

	return false
}

// RedactStringValue returns a redacted placeholder if the key or value appear sensitive.
func RedactStringValue(key, value string) string {
	if value == "" {
		return value
	}

	if IsSensitiveKey(key) || LooksLikeSecret(value) {
		return Placeholder
	}

	return value
}

// RedactStringMap clones and redacts the provided map of string key/value pairs.
func RedactStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}

	sanitized := make(map[string]string, len(values))
	for k, v := range values {
		sanitized[k] = RedactStringValue(k, v)
	}

	return sanitized
}
