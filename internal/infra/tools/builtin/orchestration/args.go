package orchestration

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func parseStringList(args map[string]any, key string) ([]string, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case []string:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if strings.TrimSpace(item) != "" {
				result = append(result, strings.TrimSpace(item))
			}
		}
		return result, nil
	case []any:
		result := make([]string, 0, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s[%d] must be a string", key, i)
			}
			if trimmed := strings.TrimSpace(str); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%s must be an array of strings when provided", key)
	}
}

func parseStringMap(args map[string]any, key string) (map[string]string, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case map[string]string:
		out := make(map[string]string, len(v))
		for k, val := range v {
			if strings.TrimSpace(k) == "" {
				return nil, fmt.Errorf("%s keys must not be empty", key)
			}
			out[strings.TrimSpace(k)] = strings.TrimSpace(val)
		}
		return out, nil
	case map[string]any:
		out := make(map[string]string, len(v))
		for k, val := range v {
			if strings.TrimSpace(k) == "" {
				return nil, fmt.Errorf("%s keys must not be empty", key)
			}
			str, ok := val.(string)
			if !ok {
				return nil, fmt.Errorf("%s.%s must be a string", key, k)
			}
			out[strings.TrimSpace(k)] = strings.TrimSpace(str)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be an object of string values", key)
	}
}

func parseOptionalBool(args map[string]any, key string) (bool, bool, error) {
	raw, exists := args[key]
	if !exists {
		return false, false, nil
	}
	if raw == nil {
		return false, true, nil
	}
	val, ok := raw.(bool)
	if !ok {
		return false, true, fmt.Errorf("%s must be a boolean", key)
	}
	return val, true, nil
}

func parseOptionalInt(args map[string]any, key string) (int, bool, error) {
	raw, exists := args[key]
	if !exists {
		return 0, false, nil
	}
	if raw == nil {
		return 0, true, nil
	}

	switch v := raw.(type) {
	case int:
		return v, true, nil
	case int32:
		return int(v), true, nil
	case int64:
		return int(v), true, nil
	case float64:
		if math.Trunc(v) != v {
			return 0, true, fmt.Errorf("%s must be an integer", key)
		}
		return int(v), true, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, true, nil
		}
		num, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, true, fmt.Errorf("%s must be an integer", key)
		}
		return num, true, nil
	default:
		return 0, true, fmt.Errorf("%s must be an integer", key)
	}
}

func canonicalAgentType(raw string) string {
	trimmed := strings.TrimSpace(raw)
	switch strings.ToLower(trimmed) {
	case "":
		return ""
	case "internal":
		return "internal"
	case "codex":
		return "codex"
	case "claude_code", "claude-code", "claude code":
		return "claude_code"
	default:
		return trimmed
	}
}

func isCodingExternalAgent(agentType string) bool {
	switch canonicalAgentType(agentType) {
	case "codex", "claude_code":
		return true
	default:
		return false
	}
}
