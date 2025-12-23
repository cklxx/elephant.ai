package http

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/logging"
	"alex/internal/observability"
	"alex/internal/server/app"
	"alex/internal/tools/builtin"
	id "alex/internal/utils/id"
	"alex/internal/workflow"

	"go.opentelemetry.io/otel/attribute"
)

const inlineAttachmentRetentionLimit = 128 * 1024 // Keep small text blobs inline for preview fallbacks.

// sseAllowlist enumerates events that are relevant to the product surface. Any
// envelope not present here will be suppressed to keep the frontend stream
// lean and avoid noisy system-level lifecycle spam.
var sseAllowlist = map[string]bool{
	"workflow.node.started":                    true,
	"workflow.node.completed":                  true,
	"workflow.node.failed":                     true,
	"workflow.node.output.delta":               true,
	"workflow.node.output.summary":             true,
	"workflow.tool.started":                    true,
	"workflow.tool.progress":                   true,
	"workflow.tool.completed":                  true,
	"workflow.input.received":                  true,
	"workflow.subflow.progress":                true,
	"workflow.subflow.completed":               true,
	"workflow.result.final":                    true,
	"workflow.result.cancelled":                true,
	"workflow.diagnostic.error":                true,
	"workflow.diagnostic.context_compression":  true,
	"workflow.diagnostic.tool_filtering":       true,
	"workflow.diagnostic.environment_snapshot": true,
}

var blockedNodeIDs = map[string]bool{
	"react:context": true,
}

var blockedNodePrefixes = []string{
	"react:",
}

// SSEHandler handles Server-Sent Events connections
type SSEHandler struct {
	broadcaster *app.EventBroadcaster
	logger      logging.Logger
	formatter   *domain.ToolFormatter
	obs         *observability.Observability
	dataCache   *DataCache
}

// SSEHandlerOption configures optional instrumentation for the SSE handler.
type SSEHandlerOption func(*SSEHandler)

// WithSSEObservability wires the observability provider into the handler.
func WithSSEObservability(obs *observability.Observability) SSEHandlerOption {
	return func(handler *SSEHandler) {
		handler.obs = obs
	}
}

// WithSSEDataCache wires a data cache used to offload large inline payloads.
func WithSSEDataCache(cache *DataCache) SSEHandlerOption {
	return func(handler *SSEHandler) {
		handler.dataCache = cache
	}
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(broadcaster *app.EventBroadcaster, opts ...SSEHandlerOption) *SSEHandler {
	handler := &SSEHandler{
		broadcaster: broadcaster,
		logger:      logging.NewComponentLogger("SSEHandler"),
		formatter:   domain.NewToolFormatter(),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(handler)
	}
	return handler
}

// HandleSSEStream handles SSE connection for real-time event streaming
func (h *SSEHandler) HandleSSEStream(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers (CORS headers are handled by middleware)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Get session ID from query parameter
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if err := validateSessionID(sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logger.Info("SSE connection established for session: %s", sessionID)

	ctx := r.Context()
	closeReason := "client_closed"
	var spanEnd func(error)
	if h.obs != nil && h.obs.Tracer != nil {
		ctx, span := h.obs.Tracer.StartSpan(ctx, observability.SpanSSEConnection,
			attribute.String("http.route", "/api/sse"),
		)
		span.SetAttributes(attribute.String(observability.AttrSessionID, sessionID))
		r = r.WithContext(ctx)
		spanEnd = func(err error) {
			if err != nil {
				span.RecordError(err)
			}
			span.SetAttributes(attribute.String("alex.sse.close_reason", closeReason))
			span.End()
		}
	}
	startedAt := time.Now()
	if h.obs != nil {
		h.obs.Metrics.IncrementSSEConnections(ctx)
		defer func() {
			h.obs.Metrics.DecrementSSEConnections(ctx)
			h.obs.Metrics.RecordSSEConnectionDuration(ctx, time.Since(startedAt))
		}()
	}
	if spanEnd != nil {
		defer func() {
			if spanEnd != nil {
				spanEnd(nil)
			}
		}()
	}

	// Create event channel for this client
	clientChan := make(chan ports.AgentEvent, 100)
	sentAttachments := make(map[string]string)
	finalAnswerCache := make(map[string]string)

	// Register client with broadcaster
	h.broadcaster.RegisterClient(sessionID, clientChan)
	defer h.broadcaster.UnregisterClient(sessionID, clientChan)

	// Get flusher for streaming (unwrap middlewares if necessary)
	flusher, ok := resolveHTTPFlusher(w)
	if !ok {
		h.logger.Error("Response writer does not support streaming (type=%T)", w)
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial connection message
	initialPayload := fmt.Sprintf(
		"event: connected\ndata: {\"session_id\":\"%s\",\"task_id\":\"%s\",\"parent_task_id\":\"%s\"}\n\n",
		sessionID,
		id.TaskIDFromContext(r.Context()),
		id.ParentTaskIDFromContext(r.Context()),
	)
	if _, err := io.WriteString(w, initialPayload); err != nil {
		h.logger.Error("Failed to send connection message: %v", err)
		if h.obs != nil {
			h.obs.Metrics.RecordSSEMessage(r.Context(), "connected", "write_error", 0)
		}
		if spanEnd != nil {
			spanEnd(err)
			spanEnd = nil
		}
		return
	}
	flusher.Flush()
	if h.obs != nil {
		h.obs.Metrics.RecordSSEMessage(r.Context(), "connected", "ok", int64(len(initialPayload)))
	}

	contextSnapshotEventType := (&domain.WorkflowDiagnosticContextSnapshotEvent{}).EventType()
	shouldStream := func(event ports.AgentEvent) bool {
		if event == nil {
			return false
		}
		base := app.BaseAgentEvent(event)
		if base == nil {
			return false
		}

		// Context snapshots are stored for debugging and analytics but contain
		// sensitive/internal details that don't need to be pushed to clients in
		// real time.
		if base.EventType() == contextSnapshotEventType {
			return false
		}

		// Only stream events that are meaningful to the frontend experience.
		if !sseAllowlist[base.EventType()] {
			return false
		}

		// Only stream workflow envelopes and explicit user task submissions.
		switch base.(type) {
		case *domain.WorkflowEventEnvelope, *domain.WorkflowInputReceivedEvent:
			if env, ok := event.(*domain.WorkflowEventEnvelope); ok {
				if blockedNodeIDs[env.NodeID] {
					return false
				}
				for _, prefix := range blockedNodePrefixes {
					if strings.HasPrefix(env.NodeID, prefix) {
						return false
					}
				}
			}
			return true
		default:
			return false
		}
	}

	sendEvent := func(event ports.AgentEvent) bool {
		if !shouldStream(event) {
			return true
		}

		if isDelegationToolEvent(event) {
			return true
		}

		data, err := h.serializeEvent(event, sentAttachments, finalAnswerCache)
		if err != nil {
			h.logger.Error("Failed to serialize event: %v", err)
			if h.obs != nil {
				h.obs.Metrics.RecordSSEMessage(r.Context(), event.EventType(), "serialization_error", 0)
			}
			return false
		}

		payload := fmt.Sprintf("event: %s\ndata: %s\n\n", event.EventType(), data)
		if _, err := io.WriteString(w, payload); err != nil {
			h.logger.Error("Failed to send SSE message: %v", err)
			if h.obs != nil {
				h.obs.Metrics.RecordSSEMessage(r.Context(), event.EventType(), "write_error", 0)
			}
			return false
		}

		flusher.Flush()
		if h.obs != nil {
			h.obs.Metrics.RecordSSEMessage(r.Context(), event.EventType(), "ok", int64(len(payload)))
		}
		return true
	}

	var lastHistoryTime time.Time

	if err := h.broadcaster.StreamHistory(ctx, app.EventHistoryFilter{SessionID: ""}, func(event ports.AgentEvent) error {
		if sendEvent(event) {
			lastHistoryTime = event.Timestamp()
		}
		return nil
	}); err != nil {
		h.logger.Warn("Failed to replay global events: %v", err)
	}

	// Replay historical events for this session
	if err := h.broadcaster.StreamHistory(ctx, app.EventHistoryFilter{SessionID: sessionID}, func(event ports.AgentEvent) error {
		if sendEvent(event) {
			lastHistoryTime = event.Timestamp()
		}
		return nil
	}); err != nil {
		h.logger.Warn("Failed to replay historical events for session %s: %v", sessionID, err)
	} else {
		h.logger.Info("Completed replaying historical events for session: %s", sessionID)
	}

	// Drain any duplicates that were queued while replaying history
	for {
		select {
		case event := <-clientChan:
			if lastHistoryTime.IsZero() || event.Timestamp().After(lastHistoryTime) {
				if sendEvent(event) {
					lastHistoryTime = event.Timestamp()
				}
			}
		default:
			goto drainComplete
		}
	}

drainComplete:

	// Heartbeat ticker to keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Stream events until client disconnects or context is cancelled
	for {
		select {
		case event := <-clientChan:
			if !sendEvent(event) {
				continue
			}

		case <-ticker.C:
			// Send heartbeat to keep connection alive
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				h.logger.Error("Failed to send heartbeat: %v", err)
				if h.obs != nil {
					h.obs.Metrics.RecordSSEMessage(r.Context(), "heartbeat", "write_error", 0)
				}
				closeReason = "heartbeat_failed"
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			// Client disconnected
			h.logger.Info("SSE connection closed for session: %s", sessionID)
			closeReason = "context_cancelled"
			return
		}
	}
}

// serializeEvent converts domain event to JSON
func (h *SSEHandler) serializeEvent(event ports.AgentEvent, sentAttachments map[string]string, finalAnswerCache map[string]string) (string, error) {
	data, err := h.buildEventData(event, sentAttachments, finalAnswerCache)
	if err != nil {
		return "", err
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// buildEventData is the single source of truth for the SSE event envelope the
// backend emits. It assumes all events have already been translated into
// workflow.* envelopes.
func (h *SSEHandler) buildEventData(event ports.AgentEvent, sentAttachments map[string]string, finalAnswerCache map[string]string) (map[string]interface{}, error) {
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
	if subtaskEvent, ok := event.(*builtin.SubtaskEvent); ok {
		base, err := h.buildEventData(subtaskEvent.OriginalEvent, sentAttachments, finalAnswerCache)
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
		if subtaskEvent.SubtaskIndex > 0 {
			data["subtask_index"] = subtaskEvent.SubtaskIndex
		}
		if subtaskEvent.TotalSubtasks > 0 {
			data["total_subtasks"] = subtaskEvent.TotalSubtasks
		}
		if subtaskEvent.SubtaskPreview != "" {
			data["subtask_preview"] = subtaskEvent.SubtaskPreview
		}
		if subtaskEvent.MaxParallel > 0 {
			data["max_parallel"] = subtaskEvent.MaxParallel
		}
		return data, nil
	}

	// Allow direct user input events if they have not been wrapped yet.
	if input, ok := event.(*domain.WorkflowInputReceivedEvent); ok {
		if sanitized := sanitizeAttachmentsForStream(input.Attachments, sentAttachments, h.dataCache, false); len(sanitized) > 0 {
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

	payload := sanitizeWorkflowEnvelopePayload(envelope, sentAttachments, h.dataCache)
	if envelope.Event == "workflow.result.final" {
		if val, ok := payload["final_answer"].(string); ok {
			key := envelope.GetTaskID()
			delta := val
			if prev, ok := finalAnswerCache[key]; ok && strings.HasPrefix(val, prev) {
				delta = strings.TrimPrefix(val, prev)
			}
			if key != "" {
				if isStreaming, ok := payload["is_streaming"].(bool); ok && isStreaming {
					finalAnswerCache[key] = val
				}
				if finished, ok := payload["stream_finished"].(bool); ok && finished {
					delete(finalAnswerCache, key)
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

type responseWriterUnwrapper interface {
	Unwrap() http.ResponseWriter
}

// resolveHTTPFlusher unwraps middleware layers until it finds a writer
// that supports http.Flusher so SSE streaming can proceed.
func resolveHTTPFlusher(w http.ResponseWriter) (http.Flusher, bool) {
	checked := 0
	current := w
	for current != nil && checked < 16 {
		if flusher, ok := current.(http.Flusher); ok {
			return flusher, true
		}
		unwrapper, ok := current.(responseWriterUnwrapper)
		if !ok {
			break
		}
		next := unwrapper.Unwrap()
		if next == nil || next == current {
			break
		}
		current = next
		checked++
	}
	return nil, false
}

func sanitizeValue(cache *DataCache, value interface{}) interface{} {
	if value == nil {
		return nil
	}

	if str, ok := value.(string); ok {
		return sanitizeStringValue(cache, str)
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

func sanitizeAttachmentsForStream(attachments map[string]ports.Attachment, sent map[string]string, cache *DataCache, forceInclude bool) map[string]ports.Attachment {
	if len(attachments) == 0 {
		return nil
	}

	sanitized := make(map[string]ports.Attachment, len(attachments))
	for name, attachment := range attachments {
		sanitized[name] = normalizeAttachmentPayload(attachment, cache)
	}

	if forceInclude {
		if sent != nil {
			for name, attachment := range sanitized {
				sent[name] = attachmentDigest(attachment)
			}
		}
		return sanitized
	}

	// Fast-path: when nothing has been sent yet, reuse the original map to
	// avoid duplicating attachment payloads in memory. We still populate the
	// sent registry so duplicates can be skipped on later deliveries.
	if len(sent) == 0 {
		if sent != nil {
			for name, attachment := range sanitized {
				sent[name] = attachmentDigest(attachment)
			}
		}
		return sanitized
	}

	var unsent map[string]ports.Attachment
	for name, attachment := range sanitized {
		digest := attachmentDigest(attachment)
		if prevDigest, alreadySent := sent[name]; alreadySent && prevDigest == digest {
			continue
		}
		if unsent == nil {
			unsent = make(map[string]ports.Attachment)
		}
		unsent[name] = attachment
		sent[name] = digest
	}

	return unsent
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
// so SSE streams do not push large base64 blobs to the client.
func normalizeAttachmentPayload(att ports.Attachment, cache *DataCache) ports.Attachment {
	if cache == nil {
		return att
	}

	// Already points to an external or cached resource.
	if att.Data == "" && att.URI != "" && !strings.HasPrefix(att.URI, "data:") {
		return att
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
				return att
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
			return att
		}
	}

	return att
}

func sanitizeEnvelopePayload(payload map[string]any, sent map[string]string, cache *DataCache) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	sanitized := make(map[string]any, len(payload))
	for key, value := range payload {
		if key == "attachments" {
			sanitized[key] = sanitizeUntypedAttachments(value, sent, cache)
			continue
		}
		if key == "result" && cache != nil {
			clean := sanitizeStepResultValue(value)
			sanitized[key] = sanitizeEnvelopeValue(clean, sent, cache)
			continue
		}
		sanitized[key] = sanitizeEnvelopeValue(value, sent, cache)
	}
	return sanitized
}

func sanitizeEnvelopeValue(value any, sent map[string]string, cache *DataCache) any {
	switch v := value.(type) {
	case nil:
		return nil
	case map[string]ports.Attachment:
		return sanitizeAttachmentsForStream(v, sent, cache, false)
	case ports.Attachment:
		if sanitized := sanitizeAttachmentsForStream(map[string]ports.Attachment{"attachment": v}, sent, cache, false); len(sanitized) > 0 {
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
				sanitized[key] = sanitizeUntypedAttachments(val, sent, cache)
				continue
			}
			if key == "nodes" {
				continue
			}
			if key == "messages" || key == "attachment_iterations" {
				continue
			}
			sanitized[key] = sanitizeEnvelopeValue(val, sent, cache)
		}
		return sanitized
	case []any:
		out := make([]any, len(v))
		for i, entry := range v {
			out[i] = sanitizeEnvelopeValue(entry, sent, cache)
		}
		return out
	default:
		return sanitizeValue(cache, v)
	}
}

func sanitizeWorkflowEnvelopePayload(env *domain.WorkflowEventEnvelope, sent map[string]string, cache *DataCache) map[string]any {
	if env == nil {
		return nil
	}

	payload := env.Payload
	if env.Event == "workflow.node.completed" && env.NodeKind == "step" {
		payload = scrubStepPayload(payload)
	}

	return sanitizeEnvelopePayload(payload, sent, cache)
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

func sanitizeUntypedAttachments(value any, sent map[string]string, cache *DataCache) any {
	raw, ok := value.(map[string]any)
	if !ok {
		return sanitizeEnvelopeValue(value, sent, cache)
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
		return sanitizeEnvelopePayload(raw, sent, cache)
	}

	sanitized := sanitizeAttachmentsForStream(attachments, sent, cache, false)
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

// isDelegationToolEvent identifies subagent delegation tool calls so they can be
// filtered from UX-facing streams (the delegated subflow emits its own events).
func isDelegationToolEvent(event ports.AgentEvent) bool {
	env, ok := app.BaseAgentEvent(event).(*domain.WorkflowEventEnvelope)
	if !ok || env == nil {
		return false
	}

	switch env.Event {
	case "workflow.tool.started", "workflow.tool.progress", "workflow.tool.completed":
	default:
		return false
	}

	if toolName := normalizedToolName(env.Payload); toolName != "" {
		return toolName == "subagent"
	}

	return strings.HasPrefix(strings.ToLower(env.NodeID), "subagent:")
}

func normalizedToolName(payload map[string]any) string {
	if len(payload) == 0 {
		return ""
	}
	for _, key := range []string{"tool_name", "tool"} {
		if raw, ok := payload[key]; ok {
			if name, ok := raw.(string); ok {
				normalized := strings.ToLower(strings.TrimSpace(name))
				if normalized != "" {
					return normalized
				}
			}
		}
	}
	return ""
}
