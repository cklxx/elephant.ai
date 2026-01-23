package main

import (
	"encoding/json"
	"strings"

	id "alex/internal/utils/id"
)

func stringParam(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func boolParam(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	switch v := m[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func intParam(m map[string]any, key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	switch v := m[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		parsed, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	default:
		return 0, false
	}
}

func sliceParam(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	if val, ok := m[key]; ok {
		if arr, ok := val.([]any); ok {
			return arr
		}
	}
	return nil
}

func mapParam(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	if val, ok := m[key]; ok {
		if mp, ok := val.(map[string]any); ok {
			return mp
		}
	}
	return nil
}

func stringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func randSuffix(length int) string {
	if length <= 0 {
		return ""
	}
	raw := id.NewKSUID()
	if raw == "" {
		return ""
	}
	if len(raw) <= length {
		return raw
	}
	return raw[len(raw)-length:]
}

