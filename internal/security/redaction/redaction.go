package redaction

import "strings"

const Placeholder = "[REDACTED]"

var (
	// nonSensitiveTokenKeys captures usage/config fields that contain the word "token"
	// but are not secrets (e.g. usage counters). These should not be redacted.
	nonSensitiveTokenKeys = map[string]struct{}{
		"tokens":            {},
		"token_count":       {},
		"tokens_used":       {},
		"total_tokens":      {},
		"input_tokens":      {},
		"output_tokens":     {},
		"prompt_tokens":     {},
		"completion_tokens": {},
		"max_tokens":        {},
		"remaining_tokens":  {},
		"estimated_tokens":  {},
		"token_budget":      {},
		"token_limit":       {},
	}

	sensitiveKeyFragments    = []string{"secret", "password", "authorization", "cookie", "credential", "session"}
	sensitiveValueIndicators = []string{"bearer ", "ghp_", "sk-", "xoxb-", "xoxp-", "-----begin", "api_key", "apikey", "access_token", "refresh_token"}
)

// IsSensitiveKey reports whether the provided key name likely references sensitive data.
func IsSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	if lowerKey == "" {
		return false
	}

	if _, ok := nonSensitiveTokenKeys[lowerKey]; ok {
		return false
	}

	if isLikelyTokenKey(lowerKey) || isLikelyKeyMaterialKey(lowerKey) {
		return true
	}

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

func isLikelyTokenKey(key string) bool {
	if key == "token" || strings.HasPrefix(key, "token_") || strings.HasSuffix(key, "_token") {
		return true
	}

	switch {
	case strings.Contains(key, "access_token"),
		strings.Contains(key, "refresh_token"),
		strings.Contains(key, "id_token"),
		strings.Contains(key, "auth_token"),
		strings.Contains(key, "session_token"),
		strings.Contains(key, "bearer_token"):
		return true
	}

	return false
}

func isLikelyKeyMaterialKey(key string) bool {
	if key == "key" || strings.HasPrefix(key, "key_") || strings.HasSuffix(key, "_key") {
		return true
	}

	switch {
	case strings.Contains(key, "api_key"),
		strings.Contains(key, "apikey"),
		strings.Contains(key, "private_key"),
		strings.Contains(key, "ssh_key"):
		return true
	}

	return false
}
