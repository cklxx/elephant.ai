package react

import (
	"strings"
	"unicode/utf8"

	agent "alex/internal/domain/agent/ports/agent"
	jsonx "alex/internal/shared/json"
)

const (
	stewardStateOpenTag  = "<NEW_STATE>"
	stewardStateCloseTag = "</NEW_STATE>"

	// maxStewardStateBytes is a generous upper bound (~1400 CJK chars × 3 bytes).
	maxStewardStateBytes = 4200
)

// ExtractNewState scans content for a <NEW_STATE>...</NEW_STATE> block, parses
// the contained JSON into a StewardState, and returns:
//   - state: the parsed state (nil on failure or absence)
//   - cleaned: the content with the <NEW_STATE> block removed
//   - err: non-nil only for structural parse failures (caller should not abort)
func ExtractNewState(content string) (state *agent.StewardState, cleaned string, err error) {
	openIdx := strings.Index(content, stewardStateOpenTag)
	if openIdx == -1 {
		return nil, content, nil
	}

	bodyStart := openIdx + len(stewardStateOpenTag)
	closeIdx := strings.Index(content[bodyStart:], stewardStateCloseTag)
	if closeIdx == -1 {
		// Unclosed tag — treat as absent.
		return nil, content, nil
	}
	closeIdx += bodyStart

	raw := strings.TrimSpace(content[bodyStart:closeIdx])
	if raw == "" {
		return nil, removeBlock(content, openIdx, closeIdx+len(stewardStateCloseTag)), nil
	}

	// Parse as JSON.
	var parsed agent.StewardState
	if jsonErr := jsonx.Unmarshal([]byte(raw), &parsed); jsonErr != nil {
		// Try YAML-like (not implemented — JSON only for now).
		return nil, removeBlock(content, openIdx, closeIdx+len(stewardStateCloseTag)), jsonErr
	}

	// Validate size.
	rendered := agent.RenderAsReminder(&parsed)
	if utf8.RuneCountInString(rendered) > agent.MaxStewardStateChars {
		return nil, removeBlock(content, openIdx, closeIdx+len(stewardStateCloseTag)), &stewardStateOversizeError{
			chars: utf8.RuneCountInString(rendered),
			limit: agent.MaxStewardStateChars,
		}
	}

	cleaned = removeBlock(content, openIdx, closeIdx+len(stewardStateCloseTag))
	return &parsed, cleaned, nil
}

// removeBlock excises content[start:end] and collapses surrounding whitespace.
func removeBlock(content string, start, end int) string {
	before := content[:start]
	after := ""
	if end < len(content) {
		after = content[end:]
	}
	result := strings.TrimSpace(before) + "\n" + strings.TrimSpace(after)
	return strings.TrimSpace(result)
}

type stewardStateOversizeError struct {
	chars int
	limit int
}

func (e *stewardStateOversizeError) Error() string {
	return "steward state exceeds character limit"
}

// IsStewardStateOversize reports whether an error indicates the parsed state
// was too large.
func IsStewardStateOversize(err error) bool {
	_, ok := err.(*stewardStateOversizeError)
	return ok
}
