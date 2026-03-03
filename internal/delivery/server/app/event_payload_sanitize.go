package app

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"

	"alex/internal/delivery/server/inlinepayload"
	domain "alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	"alex/internal/domain/agent/types"
	"alex/internal/domain/workflow"
	"alex/internal/shared/utils"
)

const (
	historyInlineAttachmentRetentionLimit = 128 * 1024
	historyMaxStringBytes                 = 16 * 1024
	historyMaxCollectionItems             = 256
	historyMaxContextMessages             = 64
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
	case workflow.WorkflowSnapshot:
		snapshot := v
		return sanitizeWorkflowSnapshotForHistory(&snapshot)
	case *workflow.WorkflowSnapshot:
		if v == nil {
			return nil
		}
		return sanitizeWorkflowSnapshotForHistory(v)
	case workflow.NodeSnapshot:
		node := v
		return sanitizeWorkflowNodeForHistory(&node)
	case *workflow.NodeSnapshot:
		if v == nil {
			return nil
		}
		return sanitizeWorkflowNodeForHistory(v)
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

	if att.Data != "" && len(att.Data) <= historyInlineAttachmentRetentionLimit {
		att.Data = truncateHistoryString(att.Data, historyInlineAttachmentRetentionLimit)
		if strings.HasPrefix(utils.TrimLower(att.URI), "data:") {
			att.URI = ""
		}
		return att
	}

	att.Data = ""
	return att
}

func sanitizeContextAttachmentForHistory(att ports.Attachment) ports.Attachment {
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
		if size > 0 && size <= historyInlineAttachmentRetentionLimit {
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
	switch typed := value.(type) {
	case workflow.WorkflowSnapshot:
		snapshot := typed
		return sanitizeWorkflowSnapshotStructForHistory(&snapshot)
	case *workflow.WorkflowSnapshot:
		return sanitizeWorkflowSnapshotStructForHistory(typed)
	case map[string]any:
		cleaned := make(map[string]any, len(typed)+1)
		for key, raw := range typed {
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
	default:
		return stripBinaryPayloadsWithStore(value, nil)
	}
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
	switch typed := value.(type) {
	case workflow.NodeSnapshot:
		return sanitizeWorkflowNodeSnapshotStructForHistory(typed)
	case *workflow.NodeSnapshot:
		if typed == nil {
			return nil
		}
		return sanitizeWorkflowNodeSnapshotStructForHistory(*typed)
	case map[string]any:
		cleaned := make(map[string]any, 8)
		if id, ok := typed["id"]; ok {
			cleaned["id"] = stripBinaryPayloadsWithStore(id, nil)
		}
		if status, ok := typed["status"]; ok {
			cleaned["status"] = stripBinaryPayloadsWithStore(status, nil)
		}
		if startedAt, ok := typed["started_at"]; ok {
			cleaned["started_at"] = stripBinaryPayloadsWithStore(startedAt, nil)
		}
		if completedAt, ok := typed["completed_at"]; ok {
			cleaned["completed_at"] = stripBinaryPayloadsWithStore(completedAt, nil)
		}
		if duration, ok := typed["duration"]; ok {
			cleaned["duration"] = stripBinaryPayloadsWithStore(duration, nil)
		}
		if input, ok := typed["input"]; ok {
			cleaned["input_preview"] = previewValueForHistory(input, historyNodeOutputPreviewBytes)
		}
		if output, ok := typed["output"]; ok {
			cleaned["output_preview"] = previewValueForHistory(output, historyNodeOutputPreviewBytes)
		}
		if result, ok := typed["result"]; ok {
			cleaned["result_preview"] = previewValueForHistory(result, historyNodeOutputPreviewBytes)
		}
		if errText, ok := typed["error"]; ok {
			cleaned["error_preview"] = previewValueForHistory(errText, historyNodeOutputPreviewBytes)
		}
		return cleaned
	default:
		return stripBinaryPayloadsWithStore(value, nil)
	}
}

func sanitizeWorkflowSnapshotStructForHistory(snapshot *workflow.WorkflowSnapshot) map[string]any {
	if snapshot == nil {
		return nil
	}

	cleaned := map[string]any{
		"id":          truncateHistoryString(snapshot.ID, historyMaxStringBytes),
		"phase":       snapshot.Phase,
		"nodes_count": len(snapshot.Nodes),
	}
	if !snapshot.StartedAt.IsZero() {
		cleaned["started_at"] = snapshot.StartedAt
	}
	if !snapshot.CompletedAt.IsZero() {
		cleaned["completed_at"] = snapshot.CompletedAt
	}
	if snapshot.Duration > 0 {
		cleaned["duration"] = snapshot.Duration
	}
	if len(snapshot.Summary) > 0 {
		cleaned["summary"] = stripBinaryPayloadsWithStore(snapshot.Summary, nil)
	}
	if len(snapshot.Order) > 0 {
		limit := len(snapshot.Order)
		if limit > historyMaxWorkflowNodes {
			limit = historyMaxWorkflowNodes
		}
		order := make([]string, 0, limit)
		for i := 0; i < limit; i++ {
			order = append(order, truncateHistoryString(snapshot.Order[i], historyMaxStringBytes))
		}
		cleaned["order"] = order
		if len(snapshot.Order) > limit {
			cleaned["truncated_order_items"] = len(snapshot.Order) - limit
		}
	}
	if len(snapshot.Nodes) > 0 {
		limit := len(snapshot.Nodes)
		if limit > historyMaxWorkflowNodes {
			limit = historyMaxWorkflowNodes
		}
		nodes := make([]any, 0, limit+1)
		for i := 0; i < limit; i++ {
			nodes = append(nodes, sanitizeWorkflowNodeSnapshotStructForHistory(snapshot.Nodes[i]))
		}
		if len(snapshot.Nodes) > limit {
			nodes = append(nodes, map[string]any{"truncated_nodes": len(snapshot.Nodes) - limit})
		}
		cleaned["nodes"] = nodes
	}

	return cleaned
}

func sanitizeWorkflowNodeSnapshotStructForHistory(node workflow.NodeSnapshot) map[string]any {
	cleaned := map[string]any{
		"id":     truncateHistoryString(node.ID, historyMaxStringBytes),
		"status": node.Status,
	}
	if !node.StartedAt.IsZero() {
		cleaned["started_at"] = node.StartedAt
	}
	if !node.CompletedAt.IsZero() {
		cleaned["completed_at"] = node.CompletedAt
	}
	if node.Duration > 0 {
		cleaned["duration"] = node.Duration
	}
	if node.Input != nil {
		cleaned["input_preview"] = previewValueForHistory(node.Input, historyNodeOutputPreviewBytes)
	}
	if node.Output != nil {
		cleaned["output_preview"] = previewValueForHistory(node.Output, historyNodeOutputPreviewBytes)
	}
	if node.Error != "" {
		cleaned["error_preview"] = previewValueForHistory(node.Error, historyNodeOutputPreviewBytes)
	}
	return cleaned
}

func previewValueForHistory(value any, limit int) string {
	if value == nil {
		return ""
	}

	switch typed := value.(type) {
	case string:
		return truncateHistoryString(typed, limit)
	case []byte:
		return fmt.Sprintf("<[]byte len=%d>", len(typed))
	case error:
		return truncateHistoryString(typed.Error(), limit)
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr, float32, float64:
		return fmt.Sprint(typed)
	}

	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return ""
	}

	switch rv.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		return fmt.Sprintf("<%T len=%d>", value, rv.Len())
	default:
		return fmt.Sprintf("<%T>", value)
	}
}

func sanitizeDomainEventDataForHistory(kind string, data domain.EventData) domain.EventData {
	cleaned := data
	cleaned.Task = truncateHistoryString(cleaned.Task, historyMaxStringBytes)
	cleaned.Content = truncateHistoryString(cleaned.Content, historyMaxStringBytes)
	cleaned.Delta = truncateHistoryString(cleaned.Delta, historyMaxStringBytes)
	cleaned.FinalAnswer = truncateHistoryString(cleaned.FinalAnswer, historyMaxStringBytes)
	cleaned.Answer = truncateHistoryString(cleaned.Answer, historyMaxStringBytes)
	cleaned.Description = truncateHistoryString(cleaned.Description, historyMaxStringBytes)
	cleaned.Prompt = truncateHistoryString(cleaned.Prompt, historyMaxStringBytes)
	cleaned.RequestID = truncateHistoryString(cleaned.RequestID, historyMaxStringBytes)
	cleaned.Message = truncateHistoryString(cleaned.Message, historyMaxStringBytes)

	if len(cleaned.Attachments) > 0 {
		sanitized := make(map[string]ports.Attachment, len(cleaned.Attachments))
		for key, att := range cleaned.Attachments {
			sanitized[key] = sanitizeAttachmentForHistory(att)
		}
		cleaned.Attachments = sanitized
	}

	if len(cleaned.Metadata) > 0 {
		if sanitized, ok := stripBinaryPayloadsWithStore(cleaned.Metadata, nil).(map[string]any); ok {
			cleaned.Metadata = sanitized
		}
	}

	if cleaned.Workflow != nil {
		cleaned.Workflow = sanitizeWorkflowSnapshotForEventData(cleaned.Workflow)
	}

	switch kind {
	case types.EventDiagnosticContextSnapshot:
		cleaned.Messages = sanitizeHistoryMessages(cleaned.Messages)
		cleaned.Excluded = sanitizeHistoryMessages(cleaned.Excluded)
	default:
		cleaned.Messages = nil
		cleaned.Excluded = nil
	}

	return cleaned
}

func sanitizeWorkflowSnapshotForEventData(snapshot *workflow.WorkflowSnapshot) *workflow.WorkflowSnapshot {
	if snapshot == nil {
		return nil
	}

	cleaned := &workflow.WorkflowSnapshot{
		ID:          truncateHistoryString(snapshot.ID, historyMaxStringBytes),
		Phase:       snapshot.Phase,
		StartedAt:   snapshot.StartedAt,
		CompletedAt: snapshot.CompletedAt,
		Duration:    snapshot.Duration,
	}

	if len(snapshot.Summary) > 0 {
		cleaned.Summary = make(map[string]int64, len(snapshot.Summary))
		for key, value := range snapshot.Summary {
			cleaned.Summary[truncateHistoryString(key, historyMaxStringBytes)] = value
		}
	}

	if len(snapshot.Order) > 0 {
		limit := len(snapshot.Order)
		if limit > historyMaxWorkflowNodes {
			limit = historyMaxWorkflowNodes
		}
		cleaned.Order = make([]string, 0, limit)
		for i := 0; i < limit; i++ {
			cleaned.Order = append(cleaned.Order, truncateHistoryString(snapshot.Order[i], historyMaxStringBytes))
		}
	}

	if len(snapshot.Nodes) > 0 {
		limit := len(snapshot.Nodes)
		if limit > historyMaxWorkflowNodes {
			limit = historyMaxWorkflowNodes
		}
		cleaned.Nodes = make([]workflow.NodeSnapshot, 0, limit)
		for i := 0; i < limit; i++ {
			node := snapshot.Nodes[i]
			cleaned.Nodes = append(cleaned.Nodes, workflow.NodeSnapshot{
				ID:          truncateHistoryString(node.ID, historyMaxStringBytes),
				Status:      node.Status,
				Input:       previewValueForHistory(node.Input, historyNodeOutputPreviewBytes),
				Output:      previewValueForHistory(node.Output, historyNodeOutputPreviewBytes),
				Error:       truncateHistoryString(node.Error, historyNodeOutputPreviewBytes),
				StartedAt:   node.StartedAt,
				CompletedAt: node.CompletedAt,
				Duration:    node.Duration,
			})
		}
	}

	return cleaned
}

func sanitizeHistoryMessages(messages []ports.Message) []ports.Message {
	if len(messages) == 0 {
		return nil
	}

	limit := len(messages)
	if limit > historyMaxContextMessages {
		limit = historyMaxContextMessages
	}

	cleaned := make([]ports.Message, 0, limit)
	for i := 0; i < limit; i++ {
		message := messages[i]
		sanitized := ports.Message{
			Role:       truncateHistoryString(message.Role, 64),
			Content:    truncateHistoryString(message.Content, historyMaxStringBytes),
			ToolCallID: truncateHistoryString(message.ToolCallID, 256),
			Source:     message.Source,
		}
		if len(message.Attachments) > 0 {
			attachments := make(map[string]ports.Attachment, len(message.Attachments))
			for key, att := range message.Attachments {
				attachments[key] = sanitizeContextAttachmentForHistory(att)
			}
			sanitized.Attachments = attachments
		}
		if len(message.Metadata) > 0 {
			if metadata, ok := stripBinaryPayloadsWithStore(message.Metadata, nil).(map[string]any); ok && len(metadata) > 0 {
				sanitized.Metadata = metadata
			}
		}
		thinkingParts := len(message.Thinking.Parts)
		if len(message.ToolCalls) > 0 || len(message.ToolResults) > 0 || thinkingParts > 0 {
			if sanitized.Metadata == nil {
				sanitized.Metadata = map[string]any{}
			}
			sanitized.Metadata["tool_calls_count"] = len(message.ToolCalls)
			sanitized.Metadata["tool_results_count"] = len(message.ToolResults)
			sanitized.Metadata["thinking_parts_count"] = thinkingParts
		}
		cleaned = append(cleaned, sanitized)
	}

	if len(messages) > limit {
		cleaned = append(cleaned, ports.Message{
			Role:    "system",
			Source:  ports.MessageSourceDebug,
			Content: fmt.Sprintf("[truncated %d history messages]", len(messages)-limit),
		})
	}

	return cleaned
}
