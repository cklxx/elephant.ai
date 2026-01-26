package http

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/workflow"
)

var sseJSONBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// serializeEvent converts domain event to JSON
func (h *SSEHandler) serializeEvent(event agent.AgentEvent, sentAttachments *stringLRU, finalAnswerCache *stringLRU) (string, error) {
	data, err := h.buildEventData(event, sentAttachments, finalAnswerCache, true)
	if err != nil {
		return "", err
	}

	return marshalSSEPayload(data)
}

func marshalSSEPayload(data map[string]interface{}) (string, error) {
	buf := sseJSONBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer sseJSONBufferPool.Put(buf)

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(data); err != nil {
		return "", err
	}

	payload := strings.TrimSuffix(buf.String(), "\n")
	return payload, nil
}

// buildEventData is the single source of truth for the SSE event envelope the
// backend emits. It assumes all events have already been translated into
// workflow.* envelopes.
func (h *SSEHandler) buildEventData(event agent.AgentEvent, sentAttachments *stringLRU, finalAnswerCache *stringLRU, streamDeltas bool) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"event_type":     event.EventType(),
		"timestamp":      event.Timestamp().Format(time.RFC3339Nano),
		"agent_level":    event.GetAgentLevel(),
		"session_id":     event.GetSessionID(),
		"task_id":        event.GetTaskID(),
		"parent_task_id": event.GetParentTaskID(),
	}

	// Subtask envelopes are flattened into the base envelope while retaining
	// metadata.
	if subtaskEvent, ok := event.(agent.SubtaskWrapper); ok {
		base, err := h.buildEventData(subtaskEvent.WrappedEvent(), sentAttachments, finalAnswerCache, streamDeltas)
		if err != nil {
			return nil, err
		}
		for k, v := range base {
			data[k] = v
		}
		data["timestamp"] = subtaskEvent.Timestamp().Format(time.RFC3339Nano)
		data["agent_level"] = subtaskEvent.GetAgentLevel()
		data["session_id"] = subtaskEvent.GetSessionID()
		data["task_id"] = subtaskEvent.GetTaskID()
		if parentTaskID := subtaskEvent.GetParentTaskID(); parentTaskID != "" {
			data["parent_task_id"] = parentTaskID
		}
		data["is_subtask"] = true
		meta := subtaskEvent.SubtaskDetails()
		if meta.Index > 0 {
			data["subtask_index"] = meta.Index
		}
		if meta.Total > 0 {
			data["total_subtasks"] = meta.Total
		}
		if meta.Preview != "" {
			data["subtask_preview"] = meta.Preview
		}
		if meta.MaxParallel > 0 {
			data["max_parallel"] = meta.MaxParallel
		}
		return data, nil
	}

	// Allow direct user input events if they have not been wrapped yet.
	if input, ok := event.(*domain.WorkflowInputReceivedEvent); ok {
		if sanitized := sanitizeAttachmentsForStream(input.Attachments, sentAttachments, h.dataCache, h.attachmentStore, false); len(sanitized) > 0 {
			data["attachments"] = sanitized
		}
		data["task"] = input.Task
		return data, nil
	}

	envelope, ok := event.(*domain.WorkflowEventEnvelope)
	if !ok {
		return data, nil
	}

	data["version"] = envelope.Version
	if envelope.WorkflowID != "" {
		data["workflow_id"] = envelope.WorkflowID
	}
	if envelope.RunID != "" {
		data["run_id"] = envelope.RunID
	}
	if envelope.NodeID != "" {
		data["node_id"] = envelope.NodeID
	}
	if envelope.NodeKind != "" {
		data["node_kind"] = envelope.NodeKind
	}
	if envelope.IsSubtask {
		data["is_subtask"] = true
	}
	if envelope.SubtaskIndex > 0 {
		data["subtask_index"] = envelope.SubtaskIndex
	}
	if envelope.TotalSubtasks > 0 {
		data["total_subtasks"] = envelope.TotalSubtasks
	}
	if envelope.SubtaskPreview != "" {
		data["subtask_preview"] = envelope.SubtaskPreview
	}
	if envelope.MaxParallel > 0 {
		data["max_parallel"] = envelope.MaxParallel
	}

	payload := sanitizeWorkflowEnvelopePayload(envelope, sentAttachments, h.dataCache, h.attachmentStore)
	if streamDeltas && envelope.Event == "workflow.result.final" {
		if val, ok := payload["final_answer"].(string); ok {
			key := envelope.GetTaskID()
			delta := val
			if prev, ok := finalAnswerCache.Get(key); ok && strings.HasPrefix(val, prev) {
				delta = strings.TrimPrefix(val, prev)
			}
			if key != "" {
				if isStreaming, ok := payload["is_streaming"].(bool); ok && isStreaming {
					finalAnswerCache.Set(key, val)
				}
				if finished, ok := payload["stream_finished"].(bool); ok && finished {
					finalAnswerCache.Delete(key)
				}
			}
			payload["final_answer"] = delta
		}
	}
	if len(payload) > 0 {
		data["payload"] = payload
	}

	return data, nil
}

func sanitizeWorkflowNode(node workflow.NodeSnapshot) map[string]interface{} {
	sanitized := map[string]interface{}{
		"id":     node.ID,
		"status": node.Status,
	}

	if node.Error != "" {
		sanitized["error"] = node.Error
	}
	if !node.StartedAt.IsZero() {
		sanitized["started_at"] = node.StartedAt.Format(time.RFC3339Nano)
	}
	if !node.CompletedAt.IsZero() {
		sanitized["completed_at"] = node.CompletedAt.Format(time.RFC3339Nano)
	}
	if node.Duration > 0 {
		sanitized["duration"] = node.Duration
	}

	return sanitized
}

func sanitizeWorkflowSnapshot(snapshot *workflow.WorkflowSnapshot) map[string]interface{} {
	if snapshot == nil {
		return nil
	}

	sanitized := map[string]interface{}{
		"id":      snapshot.ID,
		"phase":   snapshot.Phase,
		"order":   snapshot.Order,
		"summary": snapshot.Summary,
	}

	if !snapshot.StartedAt.IsZero() {
		sanitized["started_at"] = snapshot.StartedAt.Format(time.RFC3339Nano)
	}
	if !snapshot.CompletedAt.IsZero() {
		sanitized["completed_at"] = snapshot.CompletedAt.Format(time.RFC3339Nano)
	}
	if snapshot.Duration > 0 {
		sanitized["duration"] = snapshot.Duration
	}

	return sanitized
}

func sanitizeValue(cache *DataCache, value interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch typed := value.(type) {
	case string:
		return sanitizeStringValue(cache, typed)
	case []byte:
		return sanitizeStringValue(cache, string(typed))
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for key, val := range typed {
			sanitized[key] = sanitizeValue(cache, val)
		}
		return sanitized
	case []any:
		out := make([]any, len(typed))
		for i, entry := range typed {
			out[i] = sanitizeValue(cache, entry)
		}
		return out
	}

	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Map:
		return sanitizeMap(rv, cache)
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			bytesCopy := make([]byte, rv.Len())
			reflect.Copy(reflect.ValueOf(bytesCopy), rv)
			return sanitizeStringValue(cache, string(bytesCopy))
		}
		fallthrough
	case reflect.Array:
		sanitizedSlice := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			sanitizedSlice[i] = sanitizeValue(cache, rv.Index(i).Interface())
		}
		return sanitizedSlice
	case reflect.String:
		return sanitizeStringValue(cache, rv.String())
	default:
		return value
	}
}

func sanitizeMap(rv reflect.Value, cache *DataCache) map[string]interface{} {
	sanitized := make(map[string]interface{}, rv.Len())
	for _, key := range rv.MapKeys() {
		keyValue := key.Interface()
		keyString := fmt.Sprint(keyValue)
		sanitized[keyString] = sanitizeValue(cache, rv.MapIndex(key).Interface())
	}

	return sanitized
}

func sanitizeStringValue(cache *DataCache, value string) interface{} {
	if cache == nil {
		return value
	}

	if replaced := cache.MaybeStoreDataURI(value); replaced != nil {
		return replaced
	}

	return value
}

func sanitizeAttachmentsForStream(attachments map[string]ports.Attachment, sent *stringLRU, cache *DataCache, store *AttachmentStore, forceInclude bool) map[string]ports.Attachment {
	if len(attachments) == 0 {
		return nil
	}

	sanitized := make(map[string]ports.Attachment, len(attachments))
	for name, attachment := range attachments {
		sanitized[name] = normalizeAttachmentPayload(attachment, cache, store)
	}

	if forceInclude {
		for name, attachment := range sanitized {
			sent.Set(name, attachmentDigest(attachment))
		}
		return sanitized
	}

	// Fast-path: when nothing has been sent yet, reuse the original map to
	// avoid duplicating attachment payloads in memory. We still populate the
	// sent registry so duplicates can be skipped on later deliveries.
	if sent.Len() == 0 {
		for name, attachment := range sanitized {
			sent.Set(name, attachmentDigest(attachment))
		}
		return sanitized
	}

	var unsent map[string]ports.Attachment
	for name, attachment := range sanitized {
		digest := attachmentDigest(attachment)
		if prevDigest, alreadySent := sent.Get(name); alreadySent && prevDigest == digest {
			continue
		}
		if unsent == nil {
			unsent = make(map[string]ports.Attachment)
		}
		unsent[name] = attachment
		sent.Set(name, digest)
	}

	return unsent
}

func isHTMLAttachment(att ports.Attachment) bool {
	media := strings.ToLower(strings.TrimSpace(att.MediaType))
	format := strings.ToLower(strings.TrimSpace(att.Format))
	profile := strings.ToLower(strings.TrimSpace(att.PreviewProfile))
	return strings.Contains(media, "html") || format == "html" || strings.Contains(profile, "document.html")
}

func shouldPersistHTML(att ports.Attachment) bool {
	if !isHTMLAttachment(att) {
		return false
	}
	if strings.TrimSpace(att.URI) != "" && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(att.URI)), "data:") && strings.TrimSpace(att.Data) == "" {
		return false
	}
	return true
}

func persistHTMLAttachment(att ports.Attachment, store *AttachmentStore) (ports.Attachment, bool) {
	if store == nil || !shouldPersistHTML(att) {
		return att, false
	}

	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "text/html"
	}

	var payload []byte
	switch {
	case att.Data != "":
		if decoded, err := base64.StdEncoding.DecodeString(att.Data); err == nil {
			payload = decoded
		}
	case strings.HasPrefix(att.URI, "data:"):
		if ct, decoded, ok := decodeDataURI(att.URI); ok {
			if ct != "" {
				mediaType = ct
			}
			payload = decoded
		}
	}

	if len(payload) == 0 {
		return att, false
	}

	uri, err := store.StoreBytes(att.Name, mediaType, payload)
	if err != nil || strings.TrimSpace(uri) == "" {
		return att, false
	}

	att.URI = uri
	att.Data = ""
	if att.MediaType == "" {
		att.MediaType = mediaType
	}
	return ensureHTMLPreview(att), true
}

func ensureHTMLPreview(att ports.Attachment) ports.Attachment {
	if !isHTMLAttachment(att) {
		return att
	}

	if att.MediaType == "" {
		att.MediaType = "text/html"
	}
	if att.Format == "" {
		att.Format = "html"
	}
	if att.PreviewProfile == "" {
		att.PreviewProfile = "document.html"
	}

	hasHTMLPreview := false
	for _, asset := range att.PreviewAssets {
		if strings.Contains(strings.ToLower(asset.MimeType), "html") {
			hasHTMLPreview = true
			break
		}
	}

	if !hasHTMLPreview && strings.TrimSpace(att.URI) != "" {
		att.PreviewAssets = append(att.PreviewAssets, ports.AttachmentPreviewAsset{
			AssetID:     fmt.Sprintf("%s-html", strings.TrimSpace(att.Name)),
			Label:       "HTML preview",
			MimeType:    att.MediaType,
			CDNURL:      att.URI,
			PreviewType: "iframe",
		})
	}

	return att
}

func shouldRetainInlinePayload(mediaType string, size int) bool {
	if size <= 0 || size > inlineAttachmentRetentionLimit {
		return false
	}

	media := strings.ToLower(strings.TrimSpace(mediaType))
	if media == "" {
		return false
	}

	if strings.HasPrefix(media, "text/") {
		return true
	}

	return strings.Contains(media, "markdown") || strings.Contains(media, "json")
}

// normalizeAttachmentPayload converts inline payloads (Data or data URIs) into cache-backed URLs
// or persistent attachment store entries so SSE streams do not push large base64 blobs to the client.
func normalizeAttachmentPayload(att ports.Attachment, cache *DataCache, store *AttachmentStore) ports.Attachment {
	if store != nil && shouldPersistHTML(att) {
		if rewritten, ok := persistHTMLAttachment(att, store); ok {
			return rewritten
		}
	}

	if cache == nil {
		return att
	}

	// Already points to an external or cached resource.
	if att.Data == "" && att.URI != "" && !strings.HasPrefix(att.URI, "data:") {
		return ensureHTMLPreview(att)
	}

	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	// Prefer explicit data payloads.
	if att.Data != "" {
		if decoded, err := base64.StdEncoding.DecodeString(att.Data); err == nil && len(decoded) > 0 {
			if url := cache.StoreBytes(mediaType, decoded); url != "" {
				att.URI = url
				att.Data = ""
				if att.MediaType == "" {
					att.MediaType = mediaType
				}
				if shouldRetainInlinePayload(att.MediaType, len(decoded)) {
					att.Data = base64.StdEncoding.EncodeToString(decoded)
				}
				return ensureHTMLPreview(att)
			}
		}
	}

	// Fallback to data URIs when present.
	if strings.HasPrefix(att.URI, "data:") {
		rawURI := att.URI
		if cached := cache.MaybeStoreDataURI(rawURI); cached != nil {
			if url, ok := cached["url"].(string); ok && url != "" {
				att.URI = url
			}
			if ct, ok := cached["content_type"].(string); ok && ct != "" {
				att.MediaType = ct
			} else if att.MediaType == "" {
				att.MediaType = mediaType
			}
			if ct, payload, ok := decodeDataURI(rawURI); ok && shouldRetainInlinePayload(ct, len(payload)) {
				att.Data = base64.StdEncoding.EncodeToString(payload)
				if att.MediaType == "" {
					att.MediaType = ct
				}
			} else {
				att.Data = ""
			}
			return ensureHTMLPreview(att)
		}
	}

	return ensureHTMLPreview(att)
}

func sanitizeEnvelopePayload(payload map[string]any, sent *stringLRU, cache *DataCache, store *AttachmentStore) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	sanitized := make(map[string]any, len(payload))
	for key, value := range payload {
		if key == "attachments" {
			sanitized[key] = sanitizeUntypedAttachments(value, sent, cache, store)
			continue
		}
		if key == "result" && cache != nil {
			clean := sanitizeStepResultValue(value)
			sanitized[key] = sanitizeEnvelopeValue(clean, sent, cache, store)
			continue
		}
		sanitized[key] = sanitizeEnvelopeValue(value, sent, cache, store)
	}
	return sanitized
}

func sanitizeEnvelopeValue(value any, sent *stringLRU, cache *DataCache, store *AttachmentStore) any {
	switch v := value.(type) {
	case nil:
		return nil
	case map[string]ports.Attachment:
		return sanitizeAttachmentsForStream(v, sent, cache, store, false)
	case ports.Attachment:
		if sanitized := sanitizeAttachmentsForStream(map[string]ports.Attachment{"attachment": v}, sent, cache, store, false); len(sanitized) > 0 {
			return sanitized["attachment"]
		}
		return nil
	case workflow.NodeSnapshot:
		return sanitizeWorkflowNode(v)
	case *workflow.NodeSnapshot:
		if v == nil {
			return nil
		}
		return sanitizeWorkflowNode(*v)
	case *workflow.WorkflowSnapshot:
		return sanitizeWorkflowSnapshot(v)
	case workflow.WorkflowSnapshot:
		snap := v
		return sanitizeWorkflowSnapshot(&snap)
	case time.Time:
		if v.IsZero() {
			return nil
		}
		return v.Format(time.RFC3339Nano)
	case map[string]any:
		sanitized := make(map[string]any, len(v))
		for key, val := range v {
			if key == "attachments" {
				sanitized[key] = sanitizeUntypedAttachments(val, sent, cache, store)
				continue
			}
			if key == "nodes" {
				continue
			}
			if key == "messages" || key == "attachment_iterations" {
				continue
			}
			sanitized[key] = sanitizeEnvelopeValue(val, sent, cache, store)
		}
		return sanitized
	case []any:
		out := make([]any, len(v))
		for i, entry := range v {
			out[i] = sanitizeEnvelopeValue(entry, sent, cache, store)
		}
		return out
	default:
		return sanitizeValue(cache, v)
	}
}

func sanitizeWorkflowEnvelopePayload(env *domain.WorkflowEventEnvelope, sent *stringLRU, cache *DataCache, store *AttachmentStore) map[string]any {
	if env == nil {
		return nil
	}

	payload := env.Payload
	if env.Event == "workflow.node.completed" && env.NodeKind == "step" {
		payload = scrubStepPayload(payload)
	}

	return sanitizeEnvelopePayload(payload, sent, cache, store)
}

func sanitizeStepResultValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(v))
		for key, val := range v {
			if key == "messages" || key == "attachment_iterations" {
				continue
			}
			clean[key] = val
		}
		return clean
	case []any:
		return nil
	default:
		return value
	}
}

func scrubStepPayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return payload
	}

	scrubbed := make(map[string]any, len(payload)+1)
	for key, val := range payload {
		scrubbed[key] = val
	}

	if res, ok := scrubbed["result"]; ok {
		clean := sanitizeStepResultValue(res)
		scrubbed["result"] = clean
		if summary := summarizeStepResult(clean); summary != "" {
			scrubbed["step_result"] = summary
		}
	} else if sr, ok := scrubbed["step_result"]; ok {
		if summary := summarizeStepResult(sr); summary != "" {
			scrubbed["step_result"] = summary
		}
	}

	return scrubbed
}

func summarizeStepResult(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case map[string]any:
		if errMsg, ok := v["error"].(string); ok && errMsg != "" {
			return errMsg
		}
		for _, key := range []string{"summary", "content", "output", "text"} {
			if s, ok := v[key].(string); ok && s != "" {
				return s
			}
		}

		clean := make(map[string]any, len(v))
		for key, val := range v {
			if key == "messages" || key == "attachments" || key == "attachment_iterations" {
				continue
			}
			clean[key] = val
		}
		if len(clean) == 0 {
			return ""
		}
		if desc, ok := clean["description"].(string); ok && desc != "" {
			return desc
		}
		return fmt.Sprint(clean)
	default:
		return fmt.Sprint(v)
	}
}

func sanitizeUntypedAttachments(value any, sent *stringLRU, cache *DataCache, store *AttachmentStore) any {
	raw, ok := value.(map[string]any)
	if !ok {
		return sanitizeEnvelopeValue(value, sent, cache, store)
	}

	attachments := make(map[string]ports.Attachment)
	for name, entry := range raw {
		entryMap, ok := entry.(map[string]any)
		if !ok || !isAttachmentRecord(entryMap) {
			continue
		}
		att := attachmentFromMap(entryMap)
		if att.Name == "" {
			att.Name = name
		}
		attachments[name] = att
	}

	if len(attachments) == 0 {
		return sanitizeEnvelopePayload(raw, sent, cache, store)
	}

	sanitized := sanitizeAttachmentsForStream(attachments, sent, cache, store, false)
	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}

func isAttachmentRecord(entry map[string]any) bool {
	if entry == nil {
		return false
	}
	_, hasData := entry["data"]
	_, hasURI := entry["uri"]
	_, hasMediaType := entry["media_type"]
	_, hasName := entry["name"]
	return hasData || hasURI || hasMediaType || hasName
}

func attachmentFromMap(entry map[string]any) ports.Attachment {
	att := ports.Attachment{}

	if v, ok := entry["name"].(string); ok {
		att.Name = v
	}
	if v, ok := entry["media_type"].(string); ok {
		att.MediaType = v
	}
	if v, ok := entry["uri"].(string); ok {
		att.URI = v
	}
	if v, ok := entry["data"].(string); ok {
		att.Data = v
	}
	if v, ok := entry["source"].(string); ok {
		att.Source = v
	}
	if v, ok := entry["description"].(string); ok {
		att.Description = v
	}
	if v, ok := entry["kind"].(string); ok {
		att.Kind = v
	}
	if v, ok := entry["format"].(string); ok {
		att.Format = v
	}

	return att
}

func attachmentDigest(att ports.Attachment) string {
	encoded, err := json.Marshal(att)
	if err != nil {
		return att.Name
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
