package react

import (
	"fmt"
	"sort"
	"strings"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
)

func ensureWorldStateMap(state *TaskState) {
	if state.WorldState == nil {
		state.WorldState = make(map[string]any)
	}
}

// summarizeToolResultForWorld trims tool results down to safe, compact
// metadata for world-state persistence.
func summarizeToolResultForWorld(result ToolResult) map[string]any {
	entry := map[string]any{
		"call_id": strings.TrimSpace(result.CallID),
	}
	status := "success"
	if result.Error != nil {
		status = "error"
		entry["error"] = result.Error.Error()
	}
	entry["status"] = status
	if preview := summarizeForWorld(result.Content, domain.ToolResultPreviewRunes); preview != "" {
		entry["output_preview"] = preview
	}
	if metadata := summarizeWorldMetadata(result.Metadata); len(metadata) > 0 {
		entry["metadata"] = metadata
	}
	if names := summarizeAttachmentNames(result.Attachments); len(names) > 0 {
		entry["attachments"] = names
	}
	return entry
}

func summarizeForWorld(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || limit <= 0 {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

func summarizeWorldMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	summarized := make(map[string]any, len(keys))
	for _, key := range keys {
		value := summarizeMetadataValue(metadata[key])
		if value == nil {
			continue
		}
		summarized[key] = value
	}
	if len(summarized) == 0 {
		return nil
	}
	return summarized
}

func summarizeMetadataValue(value any) any {
	switch v := value.(type) {
	case string:
		return summarizeForWorld(v, domain.ToolResultPreviewRunes/2)
	case fmt.Stringer:
		return summarizeForWorld(v.String(), domain.ToolResultPreviewRunes/2)
	case float64, float32, int, int64, int32, uint64, uint32, bool:
		return v
	case []string:
		copySlice := make([]string, 0, len(v))
		for _, item := range v {
			copySlice = append(copySlice, summarizeForWorld(item, domain.ToolResultPreviewRunes/4))
		}
		return copySlice
	case map[string]any:
		return summarizeWorldMetadata(v)
	default:
		if v == nil {
			return nil
		}
		return summarizeForWorld(fmt.Sprintf("%v", v), domain.ToolResultPreviewRunes/3)
	}
}

func summarizeAttachmentNames(attachments map[string]ports.Attachment) []string {
	if len(attachments) == 0 {
		return nil
	}
	names := make([]string, 0, len(attachments))
	for key, att := range attachments {
		name := strings.TrimSpace(att.Name)
		if name == "" {
			name = strings.TrimSpace(key)
		}
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	return names
}
