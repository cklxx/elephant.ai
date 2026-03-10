package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/shared/utils"
)

// hookPayload represents the JSON payload from a Claude Code hook event.
type hookPayload struct {
	Event     string          `json:"event"` // e.g. "PostToolUse", "Stop", "PreToolUse"
	SessionID string          `json:"session_id"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	Thinking  string          `json:"thinking,omitempty"`
	Output    string          `json:"output"`
	Error     string          `json:"error"`
	// Stop event fields
	StopReason string `json:"stop_reason"`
	Answer     string `json:"answer"`
}

// decodeHookPayload parses hook payloads leniently so we can accept
// null/variant field types from different hook emitters.
func decodeHookPayload(body []byte) (hookPayload, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return hookPayload{}, err
	}

	payload := hookPayload{
		Event:      normalizeHookEvent(firstString(raw, "event", "hook_event_name", "event_name")),
		SessionID:  firstString(raw, "session_id", "session", "sessionId"),
		ToolName:   firstString(raw, "tool_name", "tool", "name"),
		Output:     firstString(raw, "output", "tool_response", "result"),
		Error:      firstString(raw, "error", "err"),
		StopReason: firstString(raw, "stop_reason", "reason", "stop"),
		Answer:     firstString(raw, "answer", "final_answer", "finalAnswer", "response"),
	}
	if payload.Answer == "" {
		// Some emitters put terminal text in `output`.
		payload.Answer = payload.Output
	}
	if toolInput, ok := firstValue(raw, "tool_input", "tool_args", "input", "arguments", "args"); ok {
		if data, err := json.Marshal(toolInput); err == nil && string(data) != "null" {
			payload.ToolInput = json.RawMessage(data)
		}
	}
	payload.Thinking = extractHookThinking(raw)
	return payload, nil
}

// normalizeHookEvent maps variant event names to canonical form.
func normalizeHookEvent(raw string) string {
	s := utils.TrimLower(raw)
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")

	switch s {
	case "posttooluse", "tooluse", "tool":
		return "PostToolUse"
	case "pretooluse", "pretool":
		return "PreToolUse"
	case "stop", "complete", "completed", "taskcomplete", "taskcompleted":
		return "Stop"
	default:
		return strings.TrimSpace(raw)
	}
}

// firstValue returns the first non-missing value for the given keys.
func firstValue(m map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v, true
		}
	}
	return nil, false
}

// firstString returns the first non-empty coerced string for the given keys.
func firstString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s := coerceString(v); s != "" {
				return s
			}
		}
	}
	return ""
}

// coerceString converts an arbitrary JSON-decoded value to a trimmed string.
func coerceString(v interface{}) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	case float64, float32, int, int32, int64, uint, uint32, uint64, bool:
		return strings.TrimSpace(fmt.Sprint(value))
	case map[string]interface{}:
		return coerceMapToString(value)
	default:
		return coerceFallbackToString(value)
	}
}

// coerceMapToString attempts to extract a string from a nested object,
// falling back to JSON serialization.
func coerceMapToString(m map[string]interface{}) string {
	if nested := firstString(m, "name", "type", "event", "value"); nested != "" {
		return nested
	}
	data, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// coerceFallbackToString serializes an unknown value to string.
func coerceFallbackToString(v interface{}) string {
	data, err := json.Marshal(v)
	if err == nil && string(data) != "null" {
		return strings.TrimSpace(string(data))
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// extractHookThinking searches multiple field names for thinking content.
func extractHookThinking(raw map[string]interface{}) string {
	if raw == nil {
		return ""
	}
	for _, key := range []string{"thinking", "reasoning", "thought", "pre_tool_thinking"} {
		if value, ok := raw[key]; ok {
			if text := extractThinkingText(value); text != "" {
				return text
			}
		}
	}
	if toolInput, ok := firstValue(raw, "tool_input", "tool_args", "input", "arguments", "args"); ok {
		return extractThinkingText(toolInput)
	}
	return ""
}

// extractThinkingText recursively extracts thinking text from nested structures.
func extractThinkingText(v interface{}) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return compactHookText(value)
	case []interface{}:
		return extractThinkingFromSlice(value)
	case map[string]interface{}:
		return extractThinkingFromMap(value)
	default:
		return ""
	}
}

// extractThinkingFromSlice joins thinking text from array elements.
func extractThinkingFromSlice(items []interface{}) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if text := extractThinkingText(item); text != "" {
			parts = append(parts, text)
		}
	}
	return compactHookText(strings.Join(parts, " "))
}

// extractThinkingFromMap searches known keys in a map for thinking content.
func extractThinkingFromMap(m map[string]interface{}) string {
	for _, key := range []string{"thinking", "reasoning", "thought", "summary", "text", "content"} {
		if nested, ok := m[key]; ok {
			if text := extractThinkingText(nested); text != "" {
				return text
			}
		}
	}
	for _, key := range []string{"parts", "segments", "items", "messages"} {
		if nested, ok := m[key]; ok {
			if text := extractThinkingText(nested); text != "" {
				return text
			}
		}
	}
	return ""
}
