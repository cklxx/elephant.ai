package claudecode

import (
	"encoding/json"
	"fmt"
	"strings"
)

// StreamMessage represents a single Claude Code stream-json line.
type StreamMessage struct {
	Type string
	Raw  map[string]any
}

// ParseStreamMessage parses a JSON line into a StreamMessage.
func ParseStreamMessage(line []byte) (StreamMessage, error) {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return StreamMessage{}, err
	}
	msgType, _ := raw["type"].(string)
	return StreamMessage{Type: strings.TrimSpace(msgType), Raw: raw}, nil
}

func (m StreamMessage) ExtractText() string {
	if m.Raw == nil {
		return ""
	}
	if val, ok := m.Raw["result"].(string); ok {
		return val
	}
	if val, ok := m.Raw["output"].(string); ok {
		return val
	}
	if msg, ok := m.Raw["message"].(map[string]any); ok {
		return extractContentText(msg["content"])
	}
	if content, ok := m.Raw["content"]; ok {
		return extractContentText(content)
	}
	return ""
}

func (m StreamMessage) ExtractUsage() (tokens int, cost float64) {
	if m.Raw == nil {
		return 0, 0
	}
	if usage, ok := m.Raw["usage"].(map[string]any); ok {
		input := numberAsInt(usage["input_tokens"])
		output := numberAsInt(usage["output_tokens"])
		tokens = input + output
	}
	if costVal, ok := m.Raw["cost"].(float64); ok {
		cost = costVal
	}
	return tokens, cost
}

func (m StreamMessage) ExtractToolEvent() (toolName string, toolArgs string) {
	if m.Raw == nil {
		return "", ""
	}
	if toolName, ok := m.Raw["tool_name"].(string); ok {
		return toolName, stringifyArgs(m.Raw["tool_args"])
	}
	if msg, ok := m.Raw["message"].(map[string]any); ok {
		if tool, ok := msg["tool_use"].(map[string]any); ok {
			if name, ok := tool["name"].(string); ok {
				return name, stringifyArgs(tool["input"])
			}
		}
	}
	return "", ""
}

func extractContentText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var sb strings.Builder
		for _, item := range v {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if entryType, _ := entry["type"].(string); entryType == "text" {
				if text, ok := entry["text"].(string); ok {
					sb.WriteString(text)
				}
			}
		}
		return sb.String()
	default:
		return ""
	}
}

func stringifyArgs(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	default:
		if encoded, err := json.Marshal(v); err == nil {
			return string(encoded)
		}
	}
	return fmt.Sprintf("%v", val)
}

func numberAsInt(val any) int {
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
