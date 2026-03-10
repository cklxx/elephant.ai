package formatter

import (
	"fmt"
	"strings"
)

func countLines(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(value, "\n") + 1
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

// Helper functions to extract typed arguments
func (tf *ToolFormatter) getStringArg(args map[string]any, key, defaultVal string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return defaultVal
}

func (tf *ToolFormatter) getStringSliceArg(args map[string]any, key string) []string {
	if raw, ok := args[key]; ok {
		switch typed := raw.(type) {
		case []string:
			return append([]string(nil), typed...)
		case []any:
			var values []string
			for _, item := range typed {
				if str, ok := item.(string); ok {
					trimmed := strings.TrimSpace(str)
					if trimmed != "" {
						values = append(values, trimmed)
					}
				}
			}
			return values
		}
	}
	return nil
}

func (tf *ToolFormatter) getIntArg(args map[string]any, key string, defaultVal int) int {
	if val, ok := args[key].(float64); ok {
		return int(val)
	}
	if val, ok := args[key].(int); ok {
		return val
	}
	return defaultVal
}

// formatValue safely converts any value to string
func (tf *ToolFormatter) formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case nil:
		return "<nil>"
	default:
		return fmt.Sprintf("%v", val)
	}
}
