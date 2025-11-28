package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/observability"
	"alex/internal/security/redaction"
	"alex/internal/server/app"
	"alex/internal/tools/builtin"
	"alex/internal/utils"
	id "alex/internal/utils/id"
	"alex/internal/workflow"

	"go.opentelemetry.io/otel/attribute"
)

// SSEHandler handles Server-Sent Events connections
type SSEHandler struct {
	broadcaster *app.EventBroadcaster
	logger      *utils.Logger
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
		logger:      utils.NewComponentLogger("SSEHandler"),
		formatter:   domain.NewToolFormatter(),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(handler)
	}
	if handler.dataCache == nil {
		handler.dataCache = NewDataCache(128, 30*time.Minute)
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

	contextSnapshotEventType := (&domain.ContextSnapshotEvent{}).EventType()
	shouldStream := func(event ports.AgentEvent) bool {
		if event == nil {
			return false
		}

		// Context snapshots are stored for debugging and analytics but contain
		// sensitive/internal details that don't need to be pushed to clients in
		// real time.
		if event.EventType() == contextSnapshotEventType {
			return false
		}
		return true
	}

	sendEvent := func(event ports.AgentEvent) bool {
		if !shouldStream(event) {
			return true
		}

		data, err := h.serializeEvent(event, sentAttachments)
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

	globalHistory := h.broadcaster.GetGlobalHistory()
	if len(globalHistory) > 0 {
		h.logger.Info("Replaying %d global events", len(globalHistory))
		for _, event := range globalHistory {
			if sendEvent(event) {
				lastHistoryTime = event.Timestamp()
			}
		}
	}

	// Replay historical events for this session
	history := h.broadcaster.GetEventHistory(sessionID)
	if len(history) > 0 {
		h.logger.Info("Replaying %d historical events for session: %s", len(history), sessionID)
		for _, event := range history {
			if sendEvent(event) {
				lastHistoryTime = event.Timestamp()
			}
		}
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
func (h *SSEHandler) serializeEvent(event ports.AgentEvent, sentAttachments map[string]string) (string, error) {
	data, err := h.buildEventData(event, sentAttachments)
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
// backend emits. The current IDL of event_type values (and their primary
// payload fields) is:
//   - user_task: task, attachments
//   - iteration_start: iteration, total_iters
//   - thinking: iteration, message_count
//   - think_complete: iteration, content, tool_call_count
//   - assistant_message: iteration, delta, final, created_at, source_model
//   - tool_call_start: iteration, call_id, tool_name, arguments
//   - tool_call_stream: call_id, chunk, is_complete
//   - tool_call_complete: call_id, tool_name, result, error, duration, metadata, attachments
//   - iteration_complete: iteration, tokens_used, tools_run
//   - task_complete: final_answer, total_iterations, total_tokens, stop_reason, duration, attachments
//   - task_cancelled: reason, requested_by
//   - error: iteration, phase, error, recoverable
//   - context_compression: original_count, compressed_count, compression_rate
//   - tool_filtering: preset_name, original_count, filtered_count, filtered_tools, tool_filter_ratio
//   - browser_info: success, message, user_agent, cdp_url, vnc_url, viewport_width, viewport_height, captured
//   - environment_snapshot: host, sandbox, captured
//   - sandbox_progress: status, stage, message, step, total_steps, error, updated
//
// Subtask-wrapped events reuse the base fields below and add:
//   - agent_level: "subagent" for delegated work
//   - is_subtask: true, subtask_index, total_subtasks, subtask_preview, max_parallel
//   - parent_task_id: identifier of the delegating task
//
// These extra keys allow consumers to recognize delegated streams even when
// event_type stays the same (e.g., assistant_message, tool_call_*). Note:
// subagent_progress and subagent_complete are not generated by the server SSE
// stream; any frontend handling of those types should treat them as client
// side extensions synthesized from the subtask envelopes.
func (h *SSEHandler) buildEventData(event ports.AgentEvent, sentAttachments map[string]string) (map[string]interface{}, error) {
	if subtaskEvent, ok := event.(*builtin.SubtaskEvent); ok {
		base, err := h.buildEventData(subtaskEvent.OriginalEvent, sentAttachments)
		if err != nil {
			return nil, err
		}

		// Clone base map to avoid mutating the original instance
		cloned := make(map[string]interface{}, len(base)+6)
		for key, value := range base {
			cloned[key] = value
		}

		cloned["timestamp"] = subtaskEvent.Timestamp().Format(time.RFC3339Nano)
		cloned["agent_level"] = subtaskEvent.GetAgentLevel()
		cloned["session_id"] = subtaskEvent.GetSessionID()
		cloned["task_id"] = subtaskEvent.GetTaskID()
		if parentTaskID := subtaskEvent.GetParentTaskID(); parentTaskID != "" {
			cloned["parent_task_id"] = parentTaskID
		}

		cloned["event_type"] = subtaskEvent.OriginalEvent.EventType()
		cloned["is_subtask"] = true
		cloned["subtask_index"] = subtaskEvent.SubtaskIndex
		cloned["total_subtasks"] = subtaskEvent.TotalSubtasks
		if subtaskEvent.SubtaskPreview != "" {
			cloned["subtask_preview"] = subtaskEvent.SubtaskPreview
		}
		if subtaskEvent.MaxParallel > 0 {
			cloned["max_parallel"] = subtaskEvent.MaxParallel
		}

		return cloned, nil
	}

	data := map[string]interface{}{
		"event_type":     event.EventType(),
		"timestamp":      event.Timestamp().Format(time.RFC3339Nano),
		"agent_level":    event.GetAgentLevel(),
		"session_id":     event.GetSessionID(),
		"task_id":        event.GetTaskID(),
		"parent_task_id": event.GetParentTaskID(),
	}

	// Add event-specific fields based on type
	switch e := event.(type) {
	case *domain.UserTaskEvent:
		data["task"] = e.Task
		if sanitized := sanitizeAttachmentsForStream(e.Attachments, sentAttachments); len(sanitized) > 0 {
			data["attachments"] = sanitized
		}
	case *domain.WorkflowLifecycleEvent:
		data["workflow_id"] = e.WorkflowID
		data["workflow_event_type"] = string(e.WorkflowEventType)
		if e.Phase != "" {
			data["phase"] = e.Phase
		}
		if e.Node != nil {
			data["node"] = sanitizeWorkflowNode(*e.Node)
		}
		if e.Workflow != nil {
			data["workflow"] = sanitizeWorkflowSnapshot(e.Workflow)
		}
	case *domain.StepStartedEvent:
		data["step_index"] = e.StepIndex
		data["step_description"] = e.StepDescription
		if e.Iteration > 0 {
			data["iteration"] = e.Iteration
		}
		if e.Workflow != nil {
			data["workflow"] = sanitizeWorkflowSnapshot(e.Workflow)
		}
	case *domain.StepCompletedEvent:
		data["step_index"] = e.StepIndex
		data["step_description"] = e.StepDescription
		if e.Status != "" {
			data["status"] = e.Status
		}
		if e.Iteration > 0 {
			data["iteration"] = e.Iteration
		}
		if e.Workflow != nil {
			data["workflow"] = sanitizeWorkflowSnapshot(e.Workflow)
		}
	case *domain.IterationStartEvent:
		data["iteration"] = e.Iteration
		data["total_iters"] = e.TotalIters

	case *domain.ThinkingEvent:
		data["iteration"] = e.Iteration
		data["message_count"] = e.MessageCount

	case *domain.ThinkCompleteEvent:
		data["iteration"] = e.Iteration
		data["content"] = e.Content
		data["tool_call_count"] = e.ToolCallCount

	case *domain.AssistantMessageEvent:
		data["iteration"] = e.Iteration
		data["delta"] = e.Delta
		data["final"] = e.Final
		data["created_at"] = e.CreatedAt.Format(time.RFC3339Nano)
		if e.SourceModel != "" {
			data["source_model"] = e.SourceModel
		}

	case *domain.ToolCallStartEvent:
		data["iteration"] = e.Iteration
		data["call_id"] = e.CallID
		data["tool_name"] = e.ToolName
		presentation := h.formatter.PrepareArgs(e.ToolName, e.Arguments)

		// Always include arguments field, even if empty
		if len(presentation.Args) > 0 {
			sanitizedArgs := make(map[string]interface{}, len(presentation.Args))
			for key, value := range presentation.Args {
				sanitizedArgs[key] = value
			}
			data["arguments"] = sanitizeArguments(sanitizedArgs, h.dataCache)
		} else {
			data["arguments"] = map[string]interface{}{}
		}

		if presentation.InlinePreview != "" {
			data["arguments_preview"] = sanitizeValue(h.dataCache, "preview", presentation.InlinePreview)
		}

	case *domain.ToolCallCompleteEvent:
		data["call_id"] = e.CallID
		data["tool_name"] = e.ToolName
		data["result"] = e.Result
		if e.Error != nil {
			data["error"] = e.Error.Error()
		}
		data["duration"] = e.Duration.Milliseconds()
		if len(e.Metadata) > 0 {
			data["metadata"] = e.Metadata
		}
		if sanitized := sanitizeAttachmentsForStream(e.Attachments, sentAttachments); len(sanitized) > 0 {
			data["attachments"] = sanitized
		}

	case *domain.ToolCallStreamEvent:
		data["call_id"] = e.CallID
		data["chunk"] = e.Chunk
		data["is_complete"] = e.IsComplete

	case *domain.IterationCompleteEvent:
		data["iteration"] = e.Iteration
		data["tokens_used"] = e.TokensUsed
		data["tools_run"] = e.ToolsRun

	case *domain.TaskCompleteEvent:
		key := e.GetTaskID()
		delta := e.FinalAnswer
		if prev, ok := finalAnswerCache[key]; ok && strings.HasPrefix(e.FinalAnswer, prev) {
			delta = strings.TrimPrefix(e.FinalAnswer, prev)
		}
		if key != "" && e.IsStreaming {
			finalAnswerCache[key] = e.FinalAnswer
		}
		data["final_answer"] = delta
		data["total_iterations"] = e.TotalIterations
		data["total_tokens"] = e.TotalTokens
		data["stop_reason"] = e.StopReason
		data["duration"] = e.Duration.Milliseconds()
		data["is_streaming"] = e.IsStreaming
		data["stream_finished"] = e.StreamFinished
		if (!e.IsStreaming || e.StreamFinished) && len(e.Attachments) > 0 {
			if sanitized := sanitizeAttachmentsForStream(e.Attachments, sentAttachments); len(sanitized) > 0 {
				data["attachments"] = sanitized
			}
		}

	case *domain.TaskCancelledEvent:
		if e.Reason != "" {
			data["reason"] = e.Reason
		}
		if e.RequestedBy != "" {
			data["requested_by"] = e.RequestedBy
		}

	case *domain.ErrorEvent:
		data["iteration"] = e.Iteration
		data["phase"] = e.Phase
		if e.Error != nil {
			data["error"] = e.Error.Error()
		}
		data["recoverable"] = e.Recoverable

	case *domain.BrowserInfoEvent:
		if e.Success != nil {
			data["success"] = *e.Success
		}
		if e.Message != "" {
			data["message"] = e.Message
		}
		if e.UserAgent != "" {
			data["user_agent"] = e.UserAgent
		}
		if e.CDPURL != "" {
			data["cdp_url"] = e.CDPURL
		}
		if e.VNCURL != "" {
			data["vnc_url"] = e.VNCURL
		}
		if e.ViewportWidth != 0 {
			data["viewport_width"] = e.ViewportWidth
		}
		if e.ViewportHeight != 0 {
			data["viewport_height"] = e.ViewportHeight
		}
		data["captured"] = e.Captured.Format(time.RFC3339)

	case *domain.EnvironmentSnapshotEvent:
		data["host"] = e.Host
		data["sandbox"] = e.Sandbox
		data["captured"] = e.Captured.Format(time.RFC3339)

	case *domain.SandboxProgressEvent:
		data["status"] = e.Status
		data["stage"] = e.Stage
		if e.Message != "" {
			data["message"] = e.Message
		}
		data["step"] = e.Step
		data["total_steps"] = e.TotalSteps
		if e.Error != "" {
			data["error"] = e.Error
		}
		data["updated"] = e.Updated.Format(time.RFC3339)

	case *domain.ContextCompressionEvent:
		data["original_count"] = e.OriginalCount
		data["compressed_count"] = e.CompressedCount
		data["compression_rate"] = e.CompressionRate

	case *domain.ToolFilteringEvent:
		data["preset_name"] = e.PresetName
		data["original_count"] = e.OriginalCount
		data["filtered_count"] = e.FilteredCount
		data["filtered_tools"] = e.FilteredTools
		data["tool_filter_ratio"] = e.ToolFilterRatio

	case *domain.ContextSnapshotEvent:
		data["iteration"] = e.Iteration
		data["llm_turn_seq"] = e.LLMTurnSeq
		data["request_id"] = e.RequestID
		messages := serializeMessages(e.Messages, sentAttachments)
		if messages == nil {
			messages = []map[string]any{}
		}
		data["messages"] = messages
		if excluded := serializeMessages(e.Excluded, sentAttachments); len(excluded) > 0 {
			data["excluded_messages"] = excluded
		}
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

	nodes := make([]map[string]interface{}, 0, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		nodes = append(nodes, sanitizeWorkflowNode(node))
	}

	sanitized := map[string]interface{}{
		"id":      snapshot.ID,
		"phase":   snapshot.Phase,
		"order":   snapshot.Order,
		"nodes":   nodes,
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

const redactedPlaceholder = redaction.Placeholder

// sanitizeArguments creates a deep copy of the provided arguments map and redacts any values that
// appear to contain sensitive information such as API keys or authorization tokens.
func sanitizeArguments(arguments map[string]interface{}, cache *DataCache) map[string]interface{} {
	if len(arguments) == 0 {
		return nil
	}

	sanitized := make(map[string]interface{}, len(arguments))
	for key, value := range arguments {
		sanitized[key] = sanitizeValue(cache, key, value)
	}

	return sanitized
}

func sanitizeValue(cache *DataCache, parentKey string, value interface{}) interface{} {
	if redaction.IsSensitiveKey(parentKey) {
		return redactedPlaceholder
	}

	if value == nil {
		return nil
	}

	if str, ok := value.(string); ok {
		if redaction.LooksLikeSecret(str) {
			return redactedPlaceholder
		}
		if cache != nil {
			if replaced := cache.MaybeStoreDataURI(str); replaced != nil {
				return replaced
			}
		}
		return str
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
			str := string(bytesCopy)
			if redaction.LooksLikeSecret(str) {
				return redactedPlaceholder
			}
			if cache != nil {
				if replaced := cache.MaybeStoreDataURI(str); replaced != nil {
					return replaced
				}
			}
			return str
		}
		fallthrough
	case reflect.Array:
		sanitizedSlice := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			sanitizedSlice[i] = sanitizeValue(cache, "", rv.Index(i).Interface())
		}
		return sanitizedSlice
	case reflect.String:
		str := rv.String()
		if redaction.LooksLikeSecret(str) {
			return redactedPlaceholder
		}
		if cache != nil {
			if replaced := cache.MaybeStoreDataURI(str); replaced != nil {
				return replaced
			}
		}
		return str
	default:
		return value
	}
}

func sanitizeMap(rv reflect.Value, cache *DataCache) map[string]interface{} {
	sanitized := make(map[string]interface{}, rv.Len())
	for _, key := range rv.MapKeys() {
		keyValue := key.Interface()
		keyString := fmt.Sprint(keyValue)
		sanitized[keyString] = sanitizeValue(cache, keyString, rv.MapIndex(key).Interface())
	}

	return sanitized
}

func sanitizeAttachmentsForStream(attachments map[string]ports.Attachment, sent map[string]string) map[string]ports.Attachment {
	if len(attachments) == 0 {
		return nil
	}

	// Fast-path: when nothing has been sent yet, reuse the original map to
	// avoid duplicating attachment payloads in memory. We still populate the
	// sent registry so duplicates can be skipped on later deliveries.
	if len(sent) == 0 {
		if sent != nil {
			for name, attachment := range attachments {
				sent[name] = attachmentDigest(attachment)
			}
		}
		return attachments
	}

	var unsent map[string]ports.Attachment
	for name, attachment := range attachments {
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

func attachmentDigest(att ports.Attachment) string {
	encoded, err := json.Marshal(att)
	if err != nil {
		return att.Name
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func serializeMessages(messages []ports.Message, sentAttachments map[string]string) []map[string]any {
	if len(messages) == 0 {
		return nil
	}

	serialized := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		if isRAGPreloadMessage(msg) {
			continue
		}

		entry := map[string]any{
			"role":    msg.Role,
			"content": msg.Content,
		}

		if len(msg.ToolCalls) > 0 {
			entry["tool_calls"] = msg.ToolCalls
		}
		if len(msg.ToolResults) > 0 {
			entry["tool_results"] = msg.ToolResults
		}
		if msg.ToolCallID != "" {
			entry["tool_call_id"] = msg.ToolCallID
		}
		if len(msg.Metadata) > 0 {
			entry["metadata"] = msg.Metadata
		}
		if sanitized := sanitizeAttachmentsForStream(msg.Attachments, sentAttachments); len(sanitized) > 0 {
			entry["attachments"] = sanitized
		}
		if msg.Source != ports.MessageSourceUnknown && msg.Source != "" {
			entry["source"] = msg.Source
		}

		serialized = append(serialized, entry)
	}

	if len(serialized) == 0 {
		return nil
	}

	return serialized
}

func isRAGPreloadMessage(msg ports.Message) bool {
	if len(msg.Metadata) == 0 {
		return false
	}
	value, ok := msg.Metadata["rag_preload"]
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		return err == nil && parsed
	case float64:
		return v != 0
	case float32:
		return v != 0
	case int:
		return v != 0
	case int8:
		return v != 0
	case int16:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case uint:
		return v != 0
	case uint8:
		return v != 0
	case uint16:
		return v != 0
	case uint32:
		return v != 0
	case uint64:
		return v != 0
	default:
		return false
	}
}
