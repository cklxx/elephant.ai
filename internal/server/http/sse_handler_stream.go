package http

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
	"alex/internal/logging"
	"alex/internal/observability"
	"alex/internal/server/app"
	id "alex/internal/utils/id"

	"go.opentelemetry.io/otel/attribute"
)

// HandleSSEStream handles SSE connection for real-time event streaming
func (h *SSEHandler) HandleSSEStream(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context(), h.logger)
	// Set SSE headers (CORS headers are handled by middleware)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Clear the server-level WriteTimeout for this long-lived SSE connection.
	// Without this, the 30s WriteTimeout would kill streaming connections.
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		logger.Warn("Failed to clear write deadline for SSE: %v (streaming may be affected)", err)
	}

	// Get session ID from query parameter
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if err := validateSessionID(sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	replayMode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("replay")))
	debugMode := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("debug")), "1") ||
		strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("debug")), "true")
	includeSessionHistory := true
	includeGlobalHistory := true
	switch replayMode {
	case "", "full":
		// Preserve existing behavior.
	case "session":
		includeGlobalHistory = false
	case "none":
		includeSessionHistory = false
		includeGlobalHistory = false
	default:
		http.Error(w, "invalid replay mode", http.StatusBadRequest)
		return
	}

	logger.Info("SSE connection established for session: %s", sessionID)

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
	clientChan := make(chan agent.AgentEvent, 100)
	sentAttachments := newStringLRU(sseSentAttachmentCacheSize)
	finalAnswerCache := newStringLRU(sseFinalAnswerCacheSize)

	// Register client with broadcaster
	h.broadcaster.RegisterClient(sessionID, clientChan)
	defer h.broadcaster.UnregisterClient(sessionID, clientChan)

	// Get flusher for streaming (unwrap middlewares if necessary)
	flusher, ok := resolveHTTPFlusher(w)
	if !ok {
		logger.Error("Response writer does not support streaming (type=%T)", w)
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial connection message
	var activeRunID string
	if h.runTracker != nil {
		activeRunID = h.runTracker.GetActiveRunID(sessionID)
	}
	initialPayload := fmt.Sprintf(
		"event: connected\ndata: {\"session_id\":\"%s\",\"run_id\":\"%s\",\"parent_run_id\":\"%s\",\"active_run_id\":\"%s\"}\n\n",
		sessionID,
		id.RunIDFromContext(r.Context()),
		id.ParentRunIDFromContext(r.Context()),
		activeRunID,
	)
	if _, err := io.WriteString(w, initialPayload); err != nil {
		logger.Error("Failed to send connection message: %v", err)
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

	const sseSeenEventIDsCacheSize = 10000
	const sseRunSeqCacheSize = 2048
	seenEventIDs := newStringLRU(sseSeenEventIDsCacheSize)
	lastSeqByRun := newRunSeqLRU(sseRunSeqCacheSize)

	sendEvent := func(event agent.AgentEvent) bool {
		if !h.shouldStreamEvent(event, debugMode) {
			return true
		}

		if isDelegationToolEvent(event) {
			switch event.EventType() {
			case types.EventToolStarted, types.EventToolCompleted:
				// Allow delegation tool anchors to reach the frontend.
			default:
				return true
			}
		}

		if !shouldSendEvent(event, seenEventIDs, lastSeqByRun) {
			return true
		}

		data, err := h.serializeEvent(event, sentAttachments, finalAnswerCache)
		if err != nil {
			logger.Error("Failed to serialize event: %v", err)
			if h.obs != nil {
				h.obs.Metrics.RecordSSEMessage(r.Context(), event.EventType(), "serialization_error", 0)
			}
			return false
		}

		payload := fmt.Sprintf("event: %s\ndata: %s\n\n", event.EventType(), data)
		if _, err := io.WriteString(w, payload); err != nil {
			logger.Error("Failed to send SSE message: %v", err)
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

	if includeGlobalHistory {
		if err := h.broadcaster.StreamHistory(ctx, app.EventHistoryFilter{SessionID: ""}, func(event agent.AgentEvent) error {
			sendEvent(event)
			return nil
		}); err != nil {
			logger.Warn("Failed to replay global events: %v", err)
		}
	}

	// Replay historical events for this session
	if includeSessionHistory {
		if err := h.broadcaster.StreamHistory(ctx, app.EventHistoryFilter{SessionID: sessionID}, func(event agent.AgentEvent) error {
			sendEvent(event)
			return nil
		}); err != nil {
			logger.Warn("Failed to replay historical events for session %s: %v", sessionID, err)
		} else {
			logger.Info("Completed replaying historical events for session: %s", sessionID)
		}
	}

	// Drain any duplicates that were queued while replaying history
	for {
		select {
		case event := <-clientChan:
			sendEvent(event)
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
				logger.Error("Failed to send heartbeat: %v", err)
				if h.obs != nil {
					h.obs.Metrics.RecordSSEMessage(r.Context(), "heartbeat", "write_error", 0)
				}
				closeReason = "heartbeat_failed"
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			// Client disconnected
			logger.Info("SSE connection closed for session: %s", sessionID)
			closeReason = "context_cancelled"
			return
		}
	}
}

func shouldSendEvent(event agent.AgentEvent, seenEventIDs *stringLRU, lastSeqByRun *runSeqLRU) bool {
	if event == nil {
		return false
	}
	if seq := event.GetSeq(); seq > 0 {
		runID := strings.TrimSpace(event.GetRunID())
		if runID != "" {
			if last, ok := lastSeqByRun.Get(runID); ok && seq <= last {
				return false
			}
			lastSeqByRun.Set(runID, seq)
		}
	}
	if eventID := strings.TrimSpace(event.GetEventID()); eventID != "" {
		if _, ok := seenEventIDs.Get(eventID); ok {
			return false
		}
		seenEventIDs.Set(eventID, "")
	}
	return true
}

func (h *SSEHandler) shouldStreamEvent(event agent.AgentEvent, debugMode bool) bool {
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
	if base.EventType() == types.EventDiagnosticContextSnapshot {
		return false
	}

	// Only stream events that are meaningful to the frontend experience.
	if !sseAllowlist[base.EventType()] {
		if !debugMode || !sseDebugAllowlist[base.EventType()] {
			return false
		}
	}

	// Only stream workflow envelopes and explicit user task submissions.
	switch base.(type) {
	case *domain.WorkflowEventEnvelope, *domain.WorkflowInputReceivedEvent:
		if env, ok := event.(*domain.WorkflowEventEnvelope); ok && !debugMode {
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

// isDelegationToolEvent identifies subagent delegation tool calls so streaming
// filters can suppress noisy delegation traffic while preserving anchor events.
func isDelegationToolEvent(event agent.AgentEvent) bool {
	env, ok := app.BaseAgentEvent(event).(*domain.WorkflowEventEnvelope)
	if !ok || env == nil {
		return false
	}

	switch env.Event {
	case types.EventToolStarted, types.EventToolProgress, types.EventToolCompleted:
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
