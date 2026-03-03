package app

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"

	"alex/internal/delivery/server/inlinepayload"
	"alex/internal/domain/agent/ports"
	"alex/internal/shared/utils"
)

const (
	historyInlineAttachmentRetentionLimit = 128 * 1024
	historyMaxStringBytes                 = 16 * 1024
	historyMaxCollectionItems             = 256
	historyMaxWorkflowNodes               = 128
	historyNodeOutputPreviewBytes         = 1024
)

// stripBinaryPayloadsWithStore recursively sanitizes a value tree, stripping large
// binary blobs and normalising attachment payloads for event history storage.
// The store parameter is accepted for interface compatibility but may be nil.
func stripBinaryPayloadsWithStore(value any, _ any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return truncateHistoryString(v, historyMaxStringBytes)
	case ports.Attachment:
		return sanitizeAttachmentForHistory(v)
	case *ports.Attachment:
		if v == nil {
			return nil
		}
		cleaned := sanitizeAttachmentForHistory(*v)
		return &cleaned
	case map[string]ports.Attachment:
		cleaned := make(map[string]ports.Attachment, len(v))
		for key, att := range v {
			cleaned[key] = sanitizeAttachmentForHistory(att)
		}
		return cleaned
	case []ports.Attachment:
		cleaned := make([]ports.Attachment, len(v))
		for i, att := range v {
			cleaned[i] = sanitizeAttachmentForHistory(att)
		}
		return cleaned
	case map[string]any:
		cleaned := make(map[string]any, len(v))
		for key, val := range v {
			switch key {
			case "workflow":
				cleaned[key] = sanitizeWorkflowSnapshotForHistory(val)
			case "node":
				cleaned[key] = sanitizeWorkflowNodeForHistory(val)
			default:
				cleaned[key] = stripBinaryPayloadsWithStore(val, nil)
			}
		}
		return cleaned
	case []any:
		limit := len(v)
		if limit > historyMaxCollectionItems {
			limit = historyMaxCollectionItems
		}
		cleaned := make([]any, 0, limit+1)
		for i := 0; i < limit; i++ {
			val := v[i]
			cleaned = append(cleaned, stripBinaryPayloadsWithStore(val, nil))
		}
		if len(v) > limit {
			cleaned = append(cleaned, map[string]any{"truncated_items": len(v) - limit})
		}
		return cleaned
	}

	rv := reflect.ValueOf(value)
	if rv.IsValid() && rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
		return nil
	}

	return value
}

func sanitizeAttachmentForHistory(att ports.Attachment) ports.Attachment {
	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
		att.MediaType = mediaType
	}

	trimmedURI := strings.TrimSpace(att.URI)
	if att.Data == "" && trimmedURI != "" && !strings.HasPrefix(strings.ToLower(trimmedURI), "data:") {
		return att
	}

	inline := strings.TrimSpace(ports.AttachmentInlineBase64(att))
	if inline != "" {
		size := base64.StdEncoding.DecodedLen(len(inline))
		if inlinepayload.ShouldRetain(mediaType, size, historyInlineAttachmentRetentionLimit) {
			att.Data = inline
			if strings.HasPrefix(utils.TrimLower(att.URI), "data:") {
				att.URI = ""
			}
			return att
		}
	}

	att.Data = ""
	return att
}

func truncateHistoryString(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit] + fmt.Sprintf("...[truncated %d bytes]", len(s)-limit)
}

func sanitizeWorkflowSnapshotForHistory(value any) any {
	workflow, ok := value.(map[string]any)
	if !ok {
		return stripBinaryPayloadsWithStore(value, nil)
	}

	cleaned := make(map[string]any, len(workflow)+1)
	for key, raw := range workflow {
		switch key {
		case "nodes":
			nodes, ok := raw.([]any)
			if !ok {
				cleaned[key] = stripBinaryPayloadsWithStore(raw, nil)
				continue
			}
			cleaned["nodes_count"] = len(nodes)
			cleaned[key] = sanitizeWorkflowNodesForHistory(nodes)
		default:
			cleaned[key] = stripBinaryPayloadsWithStore(raw, nil)
		}
	}
	return cleaned
}

func sanitizeWorkflowNodesForHistory(nodes []any) []any {
	limit := len(nodes)
	if limit > historyMaxWorkflowNodes {
		limit = historyMaxWorkflowNodes
	}
	cleaned := make([]any, 0, limit+1)
	for i := 0; i < limit; i++ {
		cleaned = append(cleaned, sanitizeWorkflowNodeForHistory(nodes[i]))
	}
	if len(nodes) > limit {
		cleaned = append(cleaned, map[string]any{"truncated_nodes": len(nodes) - limit})
	}
	return cleaned
}

func sanitizeWorkflowNodeForHistory(value any) any {
	node, ok := value.(map[string]any)
	if !ok {
		return stripBinaryPayloadsWithStore(value, nil)
	}

	cleaned := make(map[string]any, 8)
	if id, ok := node["id"]; ok {
		cleaned["id"] = stripBinaryPayloadsWithStore(id, nil)
	}
	if status, ok := node["status"]; ok {
		cleaned["status"] = stripBinaryPayloadsWithStore(status, nil)
	}
	if startedAt, ok := node["started_at"]; ok {
		cleaned["started_at"] = stripBinaryPayloadsWithStore(startedAt, nil)
	}
	if completedAt, ok := node["completed_at"]; ok {
		cleaned["completed_at"] = stripBinaryPayloadsWithStore(completedAt, nil)
	}
	if duration, ok := node["duration"]; ok {
		cleaned["duration"] = stripBinaryPayloadsWithStore(duration, nil)
	}
	if output, ok := node["output"]; ok {
		cleaned["output_preview"] = stripBinaryPayloadsWithStore(fmt.Sprint(output), nil)
	}
	if result, ok := node["result"]; ok {
		cleaned["result_preview"] = stripBinaryPayloadsWithStore(fmt.Sprint(result), nil)
	}
	if errText, ok := node["error"]; ok {
		cleaned["error_preview"] = stripBinaryPayloadsWithStore(fmt.Sprint(errText), nil)
	}

	// Keep a bounded preview of text-heavy fields when present.
	if preview, ok := cleaned["output_preview"].(string); ok {
		cleaned["output_preview"] = truncateHistoryString(preview, historyNodeOutputPreviewBytes)
	}
	if preview, ok := cleaned["result_preview"].(string); ok {
		cleaned["result_preview"] = truncateHistoryString(preview, historyNodeOutputPreviewBytes)
	}
	if preview, ok := cleaned["error_preview"].(string); ok {
		cleaned["error_preview"] = truncateHistoryString(preview, historyNodeOutputPreviewBytes)
	}

	return cleaned
}
