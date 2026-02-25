package llm

import "regexp"

var dataURIRedactionPattern = regexp.MustCompile(`data:([^\s";]+);base64,([A-Za-z0-9+/=_-]+)`)

func redactDataURIs(payload []byte) []byte {
	if len(payload) == 0 {
		return payload
	}
	redacted := dataURIRedactionPattern.ReplaceAllString(string(payload), "data:$1;base64,<redacted>")
	return []byte(redacted)
}
