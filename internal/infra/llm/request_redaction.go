package llm

import "regexp"

var dataURIRedactionPattern = regexp.MustCompile(`data:([^\s";]+);base64,([A-Za-z0-9+/=_-]+)`)

// redactDataURIs replaces base64 data URIs with a short placeholder.
// Uses ReplaceAll on []byte directly to avoid string↔[]byte round-trip copies.
func redactDataURIs(payload []byte) []byte {
	if len(payload) == 0 {
		return payload
	}
	return dataURIRedactionPattern.ReplaceAll(payload, []byte("data:${1};base64,<redacted>"))
}
