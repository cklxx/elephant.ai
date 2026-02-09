package http

import (
	"encoding/base64"
	"strings"
)

var cookieTokenDecoders = []*base64.Encoding{
	base64.RawURLEncoding,
	base64.URLEncoding,
	base64.RawStdEncoding,
	base64.StdEncoding,
}

func encodeTokenCookieValue(token string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(token))
}

func decodeTokenCookieValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	for _, decoder := range cookieTokenDecoders {
		decoded, err := decoder.DecodeString(trimmed)
		if err != nil {
			continue
		}
		token := strings.TrimSpace(string(decoded))
		if token != "" {
			return token
		}
	}

	// Compatibility fallback for raw-token cookies used by older clients.
	return trimmed
}
