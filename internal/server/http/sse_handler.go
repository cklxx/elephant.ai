package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/security/redaction"
	"alex/internal/server/app"
	"alex/internal/tools/builtin"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

// SSEHandler handles Server-Sent Events connections
type SSEHandler struct {
	broadcaster *app.EventBroadcaster
	logger      *utils.Logger
	formatter   *domain.ToolFormatter
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(broadcaster *app.EventBroadcaster) *SSEHandler {
	return &SSEHandler{
		broadcaster: broadcaster,
		logger:      utils.NewComponentLogger("SSEHandler"),
		formatter:   domain.NewToolFormatter(),
	}
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

	// Create event channel for this client
	clientChan := make(chan ports.AgentEvent, 100)

	// Register client with broadcaster
	h.broadcaster.RegisterClient(sessionID, clientChan)
	defer h.broadcaster.UnregisterClient(sessionID, clientChan)

	// Get flusher for streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial connection message
	if _, err := fmt.Fprintf(
		w,
		"event: connected\ndata: {\"session_id\":\"%s\",\"task_id\":\"%s\",\"parent_task_id\":\"%s\"}\n\n",
		sessionID,
		id.TaskIDFromContext(r.Context()),
		id.ParentTaskIDFromContext(r.Context()),
	); err != nil {
		h.logger.Error("Failed to send connection message: %v", err)
		return
	}
	flusher.Flush()

	sendEvent := func(event ports.AgentEvent) bool {
		data, err := h.serializeEvent(event)
		if err != nil {
			h.logger.Error("Failed to serialize event: %v", err)
			return false
		}

		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.EventType(), data); err != nil {
			h.logger.Error("Failed to send SSE message: %v", err)
			return false
		}

		flusher.Flush()
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
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			// Client disconnected
			h.logger.Info("SSE connection closed for session: %s", sessionID)
			return
		}
	}
}

// serializeEvent converts domain event to JSON
func (h *SSEHandler) serializeEvent(event ports.AgentEvent) (string, error) {
	data, err := h.buildEventData(event)
	if err != nil {
		return "", err
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

func (h *SSEHandler) buildEventData(event ports.AgentEvent) (map[string]interface{}, error) {
	if subtaskEvent, ok := event.(*builtin.SubtaskEvent); ok {
		base, err := h.buildEventData(subtaskEvent.OriginalEvent)
		if err != nil {
			return nil, err
		}

		// Clone base map to avoid mutating the original instance
		cloned := make(map[string]interface{}, len(base)+6)
		for key, value := range base {
			cloned[key] = value
		}

		cloned["timestamp"] = subtaskEvent.Timestamp().Format(time.RFC3339)
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
		"timestamp":      event.Timestamp().Format(time.RFC3339),
		"agent_level":    event.GetAgentLevel(),
		"session_id":     event.GetSessionID(),
		"task_id":        event.GetTaskID(),
		"parent_task_id": event.GetParentTaskID(),
	}

	// Add event-specific fields based on type
	switch e := event.(type) {
	case *domain.UserTaskEvent:
		data["task"] = e.Task
		if len(e.Attachments) > 0 {
			data["attachments"] = e.Attachments
		}
	case *domain.TaskAnalysisEvent:
		data["action_name"] = e.ActionName
		data["goal"] = e.Goal
		if strings.TrimSpace(e.Approach) != "" {
			data["approach"] = e.Approach
		}
		if len(e.SuccessCriteria) > 0 {
			data["success_criteria"] = append([]string(nil), e.SuccessCriteria...)
		}
		if len(e.Steps) > 0 {
			data["steps"] = serializeAnalysisSteps(e.Steps)
		}
		if retrieval := serializeRetrievalPlan(e.Retrieval); retrieval != nil {
			data["retrieval_plan"] = retrieval
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
			data["arguments"] = sanitizeArguments(sanitizedArgs)
		} else {
			data["arguments"] = map[string]interface{}{}
		}

		if presentation.InlinePreview != "" {
			data["arguments_preview"] = sanitizeValue("preview", presentation.InlinePreview)
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
		if len(e.Attachments) > 0 {
			data["attachments"] = e.Attachments
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
		data["final_answer"] = e.FinalAnswer
		data["total_iterations"] = e.TotalIterations
		data["total_tokens"] = e.TotalTokens
		data["stop_reason"] = e.StopReason
		data["duration"] = e.Duration.Milliseconds()
		if len(e.Attachments) > 0 {
			data["attachments"] = e.Attachments
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

	case *domain.ConfigurationUpdatedEvent:
		data["version"] = e.Version

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
		messages := serializeMessages(e.Messages)
		if messages == nil {
			messages = []map[string]any{}
		}
		data["messages"] = messages
		if excluded := serializeMessages(e.Excluded); len(excluded) > 0 {
			data["excluded_messages"] = excluded
		}
	}

	return data, nil
}

const redactedPlaceholder = redaction.Placeholder

// sanitizeArguments creates a deep copy of the provided arguments map and redacts any values that
// appear to contain sensitive information such as API keys or authorization tokens.
func sanitizeArguments(arguments map[string]interface{}) map[string]interface{} {
	if len(arguments) == 0 {
		return nil
	}

	sanitized := make(map[string]interface{}, len(arguments))
	for key, value := range arguments {
		sanitized[key] = sanitizeValue(key, value)
	}

	return sanitized
}

func serializeAnalysisSteps(steps []ports.TaskAnalysisStep) []map[string]any {
	if len(steps) == 0 {
		return nil
	}
	serialized := make([]map[string]any, 0, len(steps))
	for _, step := range steps {
		if strings.TrimSpace(step.Description) == "" {
			continue
		}
		entry := map[string]any{
			"description": step.Description,
		}
		if strings.TrimSpace(step.Rationale) != "" {
			entry["rationale"] = step.Rationale
		}
		if step.NeedsExternalContext {
			entry["needs_external_context"] = true
		}
		serialized = append(serialized, entry)
	}
	if len(serialized) == 0 {
		return nil
	}
	return serialized
}

func serializeRetrievalPlan(plan ports.TaskRetrievalPlan) map[string]any {
	hasQueries := len(plan.LocalQueries) > 0 || len(plan.SearchQueries) > 0 || len(plan.CrawlURLs) > 0 || len(plan.KnowledgeGaps) > 0
	if !plan.ShouldRetrieve && !hasQueries && strings.TrimSpace(plan.Notes) == "" {
		return nil
	}

	payload := map[string]any{
		"should_retrieve": plan.ShouldRetrieve,
	}
	if len(plan.LocalQueries) > 0 {
		payload["local_queries"] = append([]string(nil), plan.LocalQueries...)
	}
	if len(plan.SearchQueries) > 0 {
		payload["search_queries"] = append([]string(nil), plan.SearchQueries...)
	}
	if len(plan.CrawlURLs) > 0 {
		payload["crawl_urls"] = append([]string(nil), plan.CrawlURLs...)
	}
	if len(plan.KnowledgeGaps) > 0 {
		payload["knowledge_gaps"] = append([]string(nil), plan.KnowledgeGaps...)
	}
	if strings.TrimSpace(plan.Notes) != "" {
		payload["notes"] = plan.Notes
	}

	if !plan.ShouldRetrieve && hasQueries {
		payload["should_retrieve"] = true
	}

	return payload
}

func sanitizeValue(parentKey string, value interface{}) interface{} {
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
		return sanitizeMap(rv)
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			bytesCopy := make([]byte, rv.Len())
			reflect.Copy(reflect.ValueOf(bytesCopy), rv)
			str := string(bytesCopy)
			if redaction.LooksLikeSecret(str) {
				return redactedPlaceholder
			}
			return str
		}
		fallthrough
	case reflect.Array:
		sanitizedSlice := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			sanitizedSlice[i] = sanitizeValue("", rv.Index(i).Interface())
		}
		return sanitizedSlice
	case reflect.String:
		str := rv.String()
		if redaction.LooksLikeSecret(str) {
			return redactedPlaceholder
		}
		return str
	default:
		return value
	}
}

func sanitizeMap(rv reflect.Value) map[string]interface{} {
	sanitized := make(map[string]interface{}, rv.Len())
	for _, key := range rv.MapKeys() {
		keyValue := key.Interface()
		keyString := fmt.Sprint(keyValue)
		sanitized[keyString] = sanitizeValue(keyString, rv.MapIndex(key).Interface())
	}

	return sanitized
}

func serializeMessages(messages []ports.Message) []map[string]any {
	if len(messages) == 0 {
		return nil
	}

	serialized := make([]map[string]any, len(messages))
	for i, msg := range messages {
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
		if len(msg.Attachments) > 0 {
			entry["attachments"] = msg.Attachments
		}
		if msg.Source != ports.MessageSourceUnknown && msg.Source != "" {
			entry["source"] = msg.Source
		}

		serialized[i] = entry
	}

	return serialized
}
