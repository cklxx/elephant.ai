package aliases

import (
	"strconv"
	"strings"

	"alex/internal/jsonx"
)

func boolArgOptional(args map[string]any, key string) (bool, bool) {
	if args == nil {
		return false, false
	}
	value, ok := args[key]
	if !ok || value == nil {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(typed))
		if trimmed == "" {
			return false, false
		}
		return trimmed == "true" || trimmed == "1" || trimmed == "yes", true
	case int:
		return typed != 0, true
	case int64:
		return typed != 0, true
	case float64:
		return typed != 0, true
	case jsonx.Number:
		if parsed, err := typed.Int64(); err == nil {
			return parsed != 0, true
		}
	}
	return false, false
}

func intArgOptional(args map[string]any, key string) (int, bool) {
	if args == nil {
		return 0, false
	}
	value, ok := args[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case jsonx.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed), true
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		if parsed, err := strconv.Atoi(trimmed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func floatArgOptional(args map[string]any, key string) (float64, bool) {
	if args == nil {
		return 0, false
	}
	value, ok := args[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case jsonx.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed, true
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}
