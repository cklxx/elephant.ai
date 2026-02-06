package shared

import (
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/jsonx"
)

// StringArg fetches a string-like argument from the tool call map, returning an
// empty string when the key is absent or nil.
func StringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, ok := args[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

// StringSliceArg coalesces array-like arguments into a trimmed slice of
// strings, handling both []any and singular string inputs.
func StringSliceArg(args map[string]any, key string) []string {
	raw, ok := args[key]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return typed
	case []any:
		var result []string
		for _, item := range typed {
			if str := strings.TrimSpace(fmt.Sprint(item)); str != "" {
				result = append(result, str)
			}
		}
		return result
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			return []string{trimmed}
		}
	}
	return nil
}

// StringArgStrict returns a string-like argument, accepting only string,
// jsonx.Number, or fmt.Stringer values.
func StringArgStrict(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if value, ok := args[key]; ok {
		switch v := value.(type) {
		case string:
			return v
		case jsonx.Number:
			return v.String()
		case fmt.Stringer:
			return v.String()
		}
	}
	return ""
}

// StringMapArg coalesces object-like arguments into a trimmed string map,
// discarding empty keys/values.
func StringMapArg(args map[string]any, key string) map[string]string {
	if args == nil {
		return nil
	}
	raw, ok := args[key]
	if !ok {
		return nil
	}
	obj, ok := raw.(map[string]any)
	if !ok || len(obj) == 0 {
		return nil
	}

	out := make(map[string]string, len(obj))
	for k, v := range obj {
		text, ok := v.(string)
		if !ok {
			continue
		}
		keyTrimmed := strings.TrimSpace(k)
		valTrimmed := strings.TrimSpace(text)
		if keyTrimmed == "" || valTrimmed == "" {
			continue
		}
		out[keyTrimmed] = valTrimmed
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// IntArg parses an integer-like argument into an int, returning (0,false) if absent or invalid.
func IntArg(args map[string]any, key string) (int, bool) {
	if args == nil {
		return 0, false
	}
	value, ok := args[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case jsonx.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

// FloatArg parses a float-like argument into a float64, returning (0,false) if absent or invalid.
func FloatArg(args map[string]any, key string) (float64, bool) {
	if args == nil {
		return 0, false
	}
	value, ok := args[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case jsonx.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

// BoolArgWithDefault returns a boolean argument or the provided default.
func BoolArgWithDefault(args map[string]any, key string, def bool) bool {
	if args == nil {
		return def
	}
	value, ok := args[key]
	if !ok {
		return def
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(v))
		switch trimmed {
		case "", "default":
			return def
		case "true", "1", "yes", "y", "on":
			return true
		case "false", "0", "no", "n", "off":
			return false
		}
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case jsonx.Number:
		if i, err := v.Int64(); err == nil {
			return i != 0
		}
	}
	return def
}

// Uint64Arg parses a positive integer-ish argument into a uint64, returning 0
// on missing or invalid inputs.
func Uint64Arg(args map[string]any, key string) uint64 {
	if args == nil {
		return 0
	}
	switch value := args[key].(type) {
	case int:
		if value > 0 {
			return uint64(value)
		}
	case int64:
		if value > 0 {
			return uint64(value)
		}
	case float64:
		if value > 0 {
			return uint64(value)
		}
	case uint:
		return uint64(value)
	case uint64:
		return value
	case jsonNumber:
		if parsed, err := value.Int64(); err == nil && parsed > 0 {
			return uint64(parsed)
		}
	}
	return 0
}

// jsonNumber bridges between encoding/json's Number without importing it at call sites.
type jsonNumber interface {
	Int64() (int64, error)
}

// ContentSnippet returns a trimmed prefix of content to use as a lightweight
// preview, avoiding empty strings and over-long slices.
func ContentSnippet(content string, max int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max]
}

// PreviewProfile normalizes the attachment preview profile based on MIME type
// or format so downstream consumers can pick a renderer.
func PreviewProfile(mediaType, format string) string {
	media := strings.ToLower(mediaType)
	fmtFormat := strings.ToLower(format)

	switch {
	case strings.Contains(media, "a2ui") || fmtFormat == "a2ui":
		return "document.a2ui"
	case strings.Contains(media, "markdown") || fmtFormat == "markdown" || fmtFormat == "md":
		return "document.markdown"
	case strings.Contains(media, "html") || fmtFormat == "html":
		return "document.html"
	case strings.HasPrefix(media, "image/"):
		return "document.image"
	}
	return "document"
}

// ToolError constructs a failed ToolResult from a formatted error message.
func ToolError(callID string, format string, args ...any) (*ports.ToolResult, error) {
	err := fmt.Errorf(format, args...)
	return &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}, nil
}

// RequireStringArg extracts a required non-empty string argument, returning a
// ToolResult error if the argument is missing, not a string, or blank.
func RequireStringArg(args map[string]any, callID, key string) (string, *ports.ToolResult) {
	raw, ok := args[key].(string)
	if !ok {
		err := fmt.Errorf("missing '%s'", key)
		return "", &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		err := fmt.Errorf("%s cannot be empty", key)
		return "", &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	return trimmed, nil
}
