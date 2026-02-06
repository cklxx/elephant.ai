package react

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

// formatToolArgumentsForLog renders tool arguments into a log-friendly JSON
// string while stripping bulky or sensitive values.
func formatToolArgumentsForLog(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	sanitized := sanitizeToolArgumentsForLog(args)
	if len(sanitized) == 0 {
		return "{}"
	}
	if encoded, err := json.Marshal(sanitized); err == nil {
		return string(encoded)
	}
	return fmt.Sprintf("%v", sanitized)
}

func sanitizeToolArgumentsForLog(args map[string]any) map[string]any {
	if args == nil {
		return nil
	}
	sanitized := make(map[string]any, len(args))
	for key, value := range args {
		sanitized[key] = summarizeToolArgumentValue(key, value)
	}
	return sanitized
}

func summarizeToolArgumentValue(key string, value any) any {
	switch v := value.(type) {
	case string:
		return summarizeToolArgumentString(key, v)
	case map[string]any:
		return sanitizeToolArgumentsForLog(v)
	case []any:
		summarized := make([]any, 0, len(v))
		for idx, item := range v {
			summarized = append(summarized, summarizeToolArgumentValue(fmt.Sprintf("%s[%d]", key, idx), item))
		}
		return summarized
	case []string:
		summarized := make([]string, 0, len(v))
		for _, item := range v {
			summarized = append(summarized, summarizeToolArgumentString(key, item))
		}
		return summarized
	default:
		return value
	}
}

func summarizeToolArgumentString(key, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return trimmed
	}

	lowerKey := strings.ToLower(key)
	if strings.HasPrefix(trimmed, "data:") {
		return summarizeDataURIForLog(trimmed)
	}

	if strings.Contains(lowerKey, "image") {
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			return trimmed
		}
		if len(trimmed) > toolArgInlineLengthLimit || looksLikeBinaryString(trimmed) {
			return summarizeBinaryLikeString(trimmed)
		}
		return trimmed
	}

	if looksLikeBinaryString(trimmed) {
		return summarizeBinaryLikeString(trimmed)
	}

	if len(trimmed) > toolArgInlineLengthLimit {
		return summarizeLongPlainString(trimmed)
	}

	return trimmed
}

func summarizeDataURIForLog(value string) string {
	comma := strings.Index(value, ",")
	if comma == -1 {
		return fmt.Sprintf("data_uri(len=%d)", len(value))
	}
	header := value[:comma]
	payload := value[comma+1:]
	preview := truncateStringForLog(payload, toolArgPreviewLength)
	if len(payload) > len(preview) {
		preview += "..."
	}
	return fmt.Sprintf("data_uri(header=%q,len=%d,payload_prefix=%q)", header, len(value), preview)
}

func summarizeBinaryLikeString(value string) string {
	preview := truncateStringForLog(value, toolArgPreviewLength)
	if len(value) > len(preview) {
		preview += "..."
	}
	return fmt.Sprintf("base64(len=%d,prefix=%q)", len(value), preview)
}

func summarizeLongPlainString(value string) string {
	preview := truncateStringForLog(value, toolArgPreviewLength)
	if len(value) > len(preview) {
		preview += "..."
	}
	return fmt.Sprintf("%s (len=%d)", preview, len(value))
}

func looksLikeBinaryString(value string) bool {
	if len(value) < toolArgInlineLengthLimit {
		return false
	}
	sample := value
	const sampleSize = 128
	if len(sample) > sampleSize {
		sample = sample[:sampleSize]
	}
	for i := 0; i < len(sample); i++ {
		c := sample[i]
		if c < 0x20 || c > 0x7E {
			return true
		}
	}
	return false
}

func truncateStringForLog(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runeCount := 0
	for idx := range value {
		if runeCount == limit {
			return value[:idx]
		}
		runeCount++
	}
	return value
}

func compactToolCallArguments(call ToolCall, result ToolResult) (map[string]any, bool) {
	if len(call.Arguments) == 0 {
		return nil, false
	}
	ref := toolArgumentContentRef(call, result)
	compacted := make(map[string]any, len(call.Arguments))
	changed := false
	for key, value := range call.Arguments {
		next, replaced := compactToolArgumentValue(ref, value)
		if replaced {
			changed = true
		}
		compacted[key] = next
	}
	if !changed {
		return nil, false
	}
	return compacted, true
}

func compactToolArgumentValue(ref string, value any) (any, bool) {
	switch v := value.(type) {
	case map[string]any:
		if isContentReferenceMap(v) {
			return value, false
		}
		compacted := make(map[string]any, len(v))
		changed := false
		for key, entry := range v {
			next, replaced := compactToolArgumentValue(ref, entry)
			if replaced {
				changed = true
			}
			compacted[key] = next
		}
		if !changed {
			return value, false
		}
		return compacted, true
	case []any:
		compacted := make([]any, len(v))
		changed := false
		for i, entry := range v {
			next, replaced := compactToolArgumentValue(ref, entry)
			if replaced {
				changed = true
			}
			compacted[i] = next
		}
		if !changed {
			return value, false
		}
		return compacted, true
	case string:
		if !shouldCompactToolArgString(v) {
			return value, false
		}
		sum := sha256.Sum256([]byte(v))
		replacement := map[string]any{
			"content_len":    len(v),
			"content_sha256": fmt.Sprintf("%x", sum),
		}
		if ref != "" {
			replacement["content_ref"] = ref
		}
		return replacement, true
	default:
		return value, false
	}
}

func shouldCompactToolArgString(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "data:") {
		return true
	}
	if looksLikeBinaryString(trimmed) {
		return true
	}
	return len(trimmed) > toolArgHistoryInlineLimit
}

func isContentReferenceMap(value map[string]any) bool {
	if value == nil {
		return false
	}
	_, hasLen := value["content_len"]
	_, hasHash := value["content_sha256"]
	return hasLen && hasHash
}

func toolArgumentContentRef(call ToolCall, result ToolResult) string {
	toolName := strings.ToLower(strings.TrimSpace(call.Name))
	switch toolName {
	case "file_write":
		if ref := stringFromMap(result.Metadata, "path", "resolved_path", "file_path"); ref != "" {
			return ref
		}
		return stringFromMap(call.Arguments, "path")
	case "file_edit":
		if ref := stringFromMap(result.Metadata, "resolved_path", "file_path"); ref != "" {
			return ref
		}
		return stringFromMap(call.Arguments, "file_path")
	case "artifacts_write":
		return stringFromMap(call.Arguments, "name")
	default:
		return ""
	}
}

func stringFromMap(values map[string]any, keys ...string) string {
	if len(values) == 0 {
		return ""
	}
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		if text, ok := raw.(string); ok {
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}
