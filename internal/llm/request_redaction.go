package llm

import (
	"fmt"
	"regexp"
)

var dataURIRedactionPattern = regexp.MustCompile(`data:([^\s";]+);base64,([A-Za-z0-9+/=_-]+)`)

func redactDataURIs(payload []byte) []byte {
	if len(payload) == 0 {
		return payload
	}
	redacted := dataURIRedactionPattern.ReplaceAllStringFunc(string(payload), func(match string) string {
		parts := dataURIRedactionPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return "data:application/octet-stream;base64,<redacted>"
		}
		return fmt.Sprintf("data:%s;base64,<redacted:%d>", parts[1], len(parts[2]))
	})
	return []byte(redacted)
}
