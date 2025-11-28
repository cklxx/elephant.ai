package builtin

import (
	"fmt"
	"strings"
)

// stringArg fetches a string-like argument from the tool call map, returning an
// empty string when the key is absent or nil.
func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, ok := args[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

// stringSliceArg coalesces array-like arguments into a trimmed slice of
// strings, handling both []any and singular string inputs.
func stringSliceArg(args map[string]any, key string) []string {
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

// uint64Arg parses a positive integer-ish argument into a uint64, returning 0
// on missing or invalid inputs.
func uint64Arg(args map[string]any, key string) uint64 {
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

// contentSnippet returns a trimmed prefix of content to use as a lightweight
// preview, avoiding empty strings and over-long slices.
func contentSnippet(content string, max int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max]
}

// previewProfile normalizes the attachment preview profile based on MIME type
// or format so downstream consumers can pick a renderer.
func previewProfile(mediaType, format string) string {
	media := strings.ToLower(mediaType)
	fmtFormat := strings.ToLower(format)

	switch {
	case strings.Contains(media, "markdown") || fmtFormat == "markdown" || fmtFormat == "md":
		return "document.markdown"
	case strings.Contains(media, "html") || fmtFormat == "html":
		return "document.html"
	case strings.HasPrefix(media, "image/"):
		return "document.image"
	}
	return "document"
}
